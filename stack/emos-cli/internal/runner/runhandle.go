package runner

import (
	"errors"
	"fmt"
	"os/exec"
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
// in-container processes (container mode), then SIGKILLs after grace.
// Returns immediately if the process is already done.
func (h *RunHandle) Cancel(grace time.Duration) error {
	if !h.Running() {
		return nil
	}
	if h.cmd != nil && h.cmd.Process != nil {
		// Negative pid = signal the entire process group (set up via Setpgid).
		_ = syscall.Kill(-h.cmd.Process.Pid, syscall.SIGTERM)
	}
	if h.container != "" {
		// docker exec doesn't propagate signals; kill recipe processes inside.
		// Worst-case fallback (full container stop) lives one level up in the daemon.
		_, _ = container.Exec(h.container, "pkill -TERM -f 'python3.*recipe.py' || true")
	}
	if grace <= 0 {
		grace = 5 * time.Second
	}
	select {
	case <-h.done:
		return nil
	case <-time.After(grace):
	}
	if h.cmd != nil && h.cmd.Process != nil {
		_ = syscall.Kill(-h.cmd.Process.Pid, syscall.SIGKILL)
	}
	if h.container != "" {
		_, _ = container.Exec(h.container, "pkill -KILL -f 'python3.*recipe.py' || true")
	}
	return nil
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
// The tee inside the container writes to a host-bind-mounted log file
// (~/emos/logs is mounted into the container) so we can tail it from here.
func startContainerExec(containerName, shellCmd, logPath string) (*RunHandle, error) {
	// We use `docker exec` (not detached) but capture its stdout/stderr to a
	// host-side `tee` that writes to logPath. This works regardless of how the
	// container mounts its filesystem.
	full := fmt.Sprintf("%s 2>&1", shellCmd)
	cmd := exec.Command("docker", "exec", containerName, "bash", "-c", full)
	// Pipe to a host-side log file so daemon SSE can tail it.
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
		Pid:       cmd.Process.Pid,
		LogPath:   logPath,
		StartedAt: time.Now(),
		cmd:       cmd,
		container: containerName,
		done:      make(chan struct{}),
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
