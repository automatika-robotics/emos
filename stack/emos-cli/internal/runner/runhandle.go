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
	// for container more pidFile holds the pid of the shell that executed the process
	pidFile string

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

// Cancel interrupts the recipe, then kills it if it is still running after
// grace. Returns immediately if the process is already done.
func (h *RunHandle) Cancel(grace time.Duration) error {
	if !h.Running() {
		return nil
	}
	h.Interrupt()
	select {
	case <-h.done:
		return nil
	case <-time.After(grace):
	}
	h.Kill()
	return nil
}

// signal sends sig to the recipe's process group. In a container the group
// is signalled inside it, and only docker exec is killed on the host.
func (h *RunHandle) signal(sig syscall.Signal) {
	if !h.Running() {
		return
	}
	if h.container != "" {
		_, _ = container.Exec(h.container, containerKill(h.pidFile, sig))
		if sig != syscall.SIGKILL {
			return
		}
	}
	// Negative pid = signal the entire process group (set up via Setpgid).
	_ = syscall.Kill(-h.cmd.Process.Pid, sig)
}

// containerKill is the shell line that signals the process group recorded in
// pidFile.
func containerKill(pidFile string, sig syscall.Signal) string {
	return fmt.Sprintf("kill -%d -- -$(cat %s) 2>/dev/null || true", int(sig), shellQuote(pidFile))
}

// withPidFile prefixes shell with a line that records the shell's pid in a
// new file under the container's /tmp.
func withPidFile(shell string) (string, string) {
	pidFile := fmt.Sprintf("/tmp/emos-%d.pid", time.Now().UnixNano())
	return "echo $$ > " + shellQuote(pidFile) + " && " + shell, pidFile
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

// start runs shell in the strategy's environment, in its own process group,
// writing its output to out.
func start(s RuntimeStrategy, shell string, out io.Writer) (*RunHandle, error) {
	var pidFile string
	// In a container the pid of the process is kept in a file and the group is
	// signalled there.
	if _, ok := s.(*ContainerStrategy); ok {
		shell, pidFile = withPidFile(shell)
	}
	cmd := s.Command(shell)
	cmd.Stdout = out
	cmd.Stderr = out
	h, err := StartProcess(cmd)
	if err != nil {
		return nil, err
	}
	if pidFile != "" {
		h.container = config.ContainerName
		h.pidFile = pidFile
		go func() {
			<-h.done
			_, _ = container.Exec(h.container, "rm -f "+shellQuote(pidFile))
		}()
	}
	return h, nil
}

// startRecipe starts a recipe in the strategy's environment, writing its output
// to out.
func startRecipe(s RuntimeStrategy, recipeName string, out io.Writer) (*RunHandle, error) {
	// Run from the recipe's folder, so it finds files next to it by relative path.
	dir := filepath.Join(s.RecipesDir(), recipeName)
	h, err := start(s, fmt.Sprintf("cd %s && exec python3 -u %s",
		shellQuote(dir), shellQuote(filepath.Join(dir, "recipe.py"))), out)
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
