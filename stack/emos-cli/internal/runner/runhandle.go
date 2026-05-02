package runner

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/container"
)

// RunHandle is a started recipe process that callers can Wait() on, Cancel(),
// or read state from. Strategies return one from StartRecipe so the daemon can
// track the run; the synchronous CLI path just calls Wait() immediately.
type RunHandle struct {
	Pid       int
	LogPath   string
	StartedAt time.Time

	cmd       *exec.Cmd
	container string
	// killTarget is the exact full-path string the recipe's python process
	// was invoked with. Used by Cancel to scope `pkill -f` to recipe
	killTarget string

	once     sync.Once
	done     chan struct{}
	mu       sync.Mutex
	exitCode int
	exitErr  error
}

// Wait blocks until the process exits. Safe to call from multiple goroutines.
func (h *RunHandle) Wait() (int, error) {
	<-h.done
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.exitCode, h.exitErr
}

// Done returns a channel that closes when the process exits.
func (h *RunHandle) Done() <-chan struct{} { return h.done }

// Running reports whether the process is still active (best-effort, may race).
func (h *RunHandle) Running() bool {
	select {
	case <-h.done:
		return false
	default:
		return true
	}
}

// ExitCode returns the exit code if the process has finished. Zero if still running.
func (h *RunHandle) ExitCode() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.exitCode
}

// Cancel sends SIGTERM to the recipe process group (native/pixi) or kills the
// in-container recipe process (container mode), then SIGKILLs after grace.
// Returns immediately if the process is already done.
func (h *RunHandle) Cancel(grace time.Duration) error {
	if !h.Running() {
		return nil
	}
	if h.cmd != nil && h.cmd.Process != nil {
		// Negative pid = signal the entire process group (set up via Setpgid).
		_ = syscall.Kill(-h.cmd.Process.Pid, syscall.SIGTERM)
	}
	if h.container != "" && h.killTarget != "" {
		_, _ = container.Exec(h.container, fmt.Sprintf(
			"pkill -TERM -f %s || true", shellQuote(h.killTarget)))
	}
	if grace <= 0 {
		// Caller asked for "no grace" — honour it and SIGKILL immediately.
		grace = 0
	}
	if grace > 0 {
		select {
		case <-h.done:
			return nil
		case <-time.After(grace):
		}
	}
	if h.cmd != nil && h.cmd.Process != nil {
		_ = syscall.Kill(-h.cmd.Process.Pid, syscall.SIGKILL)
	}
	if h.container != "" && h.killTarget != "" {
		_, _ = container.Exec(h.container, fmt.Sprintf(
			"pkill -KILL -f %s || true", shellQuote(h.killTarget)))
	}
	return nil
}

// shellQuote wraps `s` in single quotes for safe inclusion in a shell command,
// escaping any embedded single quotes via the standard '\'' trick.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// finish marks the handle as done with the given exit code/error. Idempotent.
func (h *RunHandle) finish(code int, err error) {
	h.once.Do(func() {
		h.mu.Lock()
		h.exitCode = code
		h.exitErr = err
		h.mu.Unlock()
		close(h.done)
	})
}

// startCmd is a small helper used by native/pixi strategies: it sets up a
// process group, starts the bash subprocess, and spawns a goroutine that
// captures the exit status into the handle.
func startCmd(cmd *exec.Cmd, logPath string) (*RunHandle, error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start recipe: %w", err)
	}
	h := &RunHandle{
		Pid:       cmd.Process.Pid,
		LogPath:   logPath,
		StartedAt: time.Now(),
		cmd:       cmd,
		done:      make(chan struct{}),
	}
	go func() {
		err := cmd.Wait()
		code := 0
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				code = exitErr.ExitCode()
			} else {
				code = -1
			}
		}
		h.finish(code, err)
	}()
	return h, nil
}

// startContainerExec starts a `docker exec` in detached-but-tracked form.
// `killTarget` is the in-container recipe.py absolute path
func startContainerExec(containerName, shellCmd, logPath, killTarget string) (*RunHandle, error) {
	// We use `docker exec` (not detached) but capture its stdout/stderr to a
	// host-side log file. This works regardless of how the container mounts.
	full := fmt.Sprintf("%s 2>&1", shellCmd)
	cmd := exec.Command("docker", "exec", containerName, "bash", "-c", full)
	logF, err := openLogFile(logPath)
	if err != nil {
		return nil, err
	}
	cmd.Stdout = logF
	cmd.Stderr = logF
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		logF.Close()
		return nil, fmt.Errorf("start container exec: %w", err)
	}
	h := &RunHandle{
		Pid:        cmd.Process.Pid,
		LogPath:    logPath,
		StartedAt:  time.Now(),
		cmd:        cmd,
		container:  containerName,
		killTarget: killTarget,
		done:       make(chan struct{}),
	}
	go func() {
		err := cmd.Wait()
		_ = logF.Close()
		code := 0
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				code = exitErr.ExitCode()
			} else {
				code = -1
			}
		}
		h.finish(code, err)
	}()
	return h, nil
}

// ErrAlreadyRunning is returned when StartRecipe is called while a previous run
// is still active and the strategy enforces single-recipe semantics. Currently
// this is enforced at the daemon level, not strategy level.
var ErrAlreadyRunning = errors.New("a recipe is already running")

// LogFilePath returns the canonical log path for a recipe run.
func LogFilePath(recipeName string) string {
	return fmt.Sprintf("%s/%s_%s.log", config.LogsDir, recipeName, time.Now().Format("20060102_150405"))
}
