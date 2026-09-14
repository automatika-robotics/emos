package runner

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/container"
)

// RunHandle is a started recipe process that callers can Wait() on, stop, or
// read state from. Strategies return one from StartRecipe.
type RunHandle struct {
	Pid       int
	StartedAt time.Time

	cmd       *exec.Cmd
	container string
	// killTarget is the exact full-path string the recipe's python process
	// was invoked with. Used to scope `pkill -f` to the recipe
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

// Interrupt asks the recipe to shut down, as Ctrl+C in its own terminal would.
func (h *RunHandle) Interrupt() { h.signal(syscall.SIGINT) }

// Kill ends the recipe at once.
func (h *RunHandle) Kill() { h.signal(syscall.SIGKILL) }

// Cancel sends SIGTERM to the recipe, then SIGKILLs after grace. Returns
// immediately if the process is already done.
func (h *RunHandle) Cancel(grace time.Duration) error {
	if !h.Running() {
		return nil
	}
	h.signal(syscall.SIGTERM)
	select {
	case <-h.done:
		return nil
	case <-time.After(grace):
	}
	h.Kill()
	return nil
}

// signal sends sig to the recipe's process group. In a container the recipe is
// signalled inside it and only docker exec is killed.
func (h *RunHandle) signal(sig syscall.Signal) {
	if !h.Running() {
		return
	}
	if h.container != "" {
		_, _ = container.Exec(h.container, fmt.Sprintf(
			"pkill -%d -f %s || true", sig, shellQuote(h.killTarget)))
		if sig != syscall.SIGKILL {
			return
		}
	}
	// Negative pid = signal the entire process group (set up via Setpgid).
	_ = syscall.Kill(-h.cmd.Process.Pid, sig)
}

// shellQuote wraps `s` in single quotes for safe inclusion in a shell command.
// An embedded single quote ends the quoted string, is added escaped as \', and
// a new quoted string starts.
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

// StartProcess starts cmd in its own process group, and returns a handle that
// records its exit status.
func StartProcess(cmd *exec.Cmd) (*RunHandle, error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	h := &RunHandle{
		Pid:       cmd.Process.Pid,
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

// startRecipe starts a recipe in the strategy's environment, writing its output
// to out.
func startRecipe(s RuntimeStrategy, recipeName string, out io.Writer) (*RunHandle, error) {
	cmd := s.Command("exec python3 -u " + filepath.Join(s.RecipesDir(), recipeName, "recipe.py"))
	cmd.Stdout = out
	cmd.Stderr = out
	h, err := StartProcess(cmd)
	if err != nil {
		return nil, fmt.Errorf("start recipe: %w", err)
	}
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
