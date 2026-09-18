package mapping

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/automatika-robotics/emos-cli/internal/config"
)

// Exit codes of emos_mapping's session.
const (
	sessionNoMap     = 1 // it ran, but the backend never published a map
	sessionCannotMap = 2 // the plugin could not be mapped with
)

// announcePrefix starts the line the session prints once the map's directory
// exists.
const announcePrefix = "Map directory: "

// validEntryPoint is a plugin's '<package.module>:<ClassName>'. It goes into a
// shell command unquoted.
var validEntryPoint = regexp.MustCompile(
	`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)*:[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)*$`)

// ErrNeedsHostInstall is returned in a container install, whose image does not
// carry the mapping backend.
var ErrNeedsHostInstall = errors.New("EMOS cannot build maps itself in a container install")

// ErrNoMapBuilt is returned when the session ran but the backend never
// published a map.
var ErrNoMapBuilt = errors.New("the mapping backend published no map")

// ErrSessionExited is returned when the session ended without a map for any
// other reason. Its output says why.
type ErrSessionExited struct{ Code int }

func (e *ErrSessionExited) Error() string {
	if e.Code == sessionCannotMap {
		return "the robot's plugin cannot be mapped with"
	}
	return fmt.Sprintf("the mapping session exited with status %d", e.Code)
}

// Process is a started mapping session.
type Process interface {
	// Interrupt asks it to stop and save, as Ctrl+C in its terminal would.
	Interrupt()
	Kill()
	Done() <-chan struct{}
	Wait() (int, error)
}

// Starter starts shell in the environment EMOS is installed in, with its
// output going to out.
type Starter func(shell string, out io.Writer) (Process, error)

// backendPackage is the ROS package of the SLAM backend the session runs.
const backendPackage = "glim_ros"

// BackendInstalled reports whether the SLAM backend is in the environment EMOS
// is installed in.
func BackendInstalled(start Starter) (bool, error) {
	proc, err := start("ros2 pkg prefix "+backendPackage, io.Discard)
	if err != nil {
		return false, err
	}
	code, _ := proc.Wait()
	return code == 0, nil
}

// NativeSupported returns why an install in mode cannot build maps itself, or nil.
func NativeSupported(mode config.InstallMode) error {
	if mode == config.ModeOSSContainer || mode == config.ModeLicensed {
		return ErrNeedsHostInstall
	}
	return nil
}

// NativeSession is a native mapping run in progress.
type NativeSession struct {
	decl *Declaration
	proc Process
	// Dir is the directory the map is being built in.
	Dir string
}

// StartNative begins a mapping session EMOS runs itself, for the plugin at
// entryPoint. It returns once the session has announced the map's directory.
// The session's output goes to out. Cancelling ctx before then kills it.
func (d *Declaration) StartNative(ctx context.Context, entryPoint, name string, start Starter, out io.Writer) (*NativeSession, error) {
	if d.Kind != KindNative {
		return nil, fmt.Errorf("this robot maps with its own software")
	}
	if err := CheckName(name); err != nil {
		return nil, err
	}
	if !validEntryPoint.MatchString(entryPoint) {
		return nil, fmt.Errorf("plugin entry point %q is not '<package.module>:<ClassName>'", entryPoint)
	}

	announced := newAnnouncement(out)
	proc, err := start(sessionShell(entryPoint, name), announced)
	if err != nil {
		return nil, fmt.Errorf("start mapping: %w", err)
	}
	select {
	case dir := <-announced.dir:
		return &NativeSession{decl: d, proc: proc, Dir: dir}, nil
	case <-proc.Done():
		code, _ := proc.Wait()
		return nil, &ErrSessionExited{Code: code}
	case <-ctx.Done():
		proc.Kill()
		return nil, ctx.Err()
	}
}

// sessionShell is the command that runs the session. The module is run
// directly, so an interrupt reaches one process, once.
func sessionShell(entryPoint, name string) string {
	return "python3 -u -m emos_mapping.session --plugin " + entryPoint + " --name " + name
}

// Done closes when the session has ended, whether or not it was asked to.
func (s *NativeSession) Done() <-chan struct{} { return s.proc.Done() }

// Stop asks the session to stop and returns the map it saved. Cancelling ctx
// kills it and gives up with ErrStopInterrupted.
func (s *NativeSession) Stop(ctx context.Context) (*Map, error) {
	s.proc.Interrupt()
	select {
	case <-s.proc.Done():
	case <-ctx.Done():
		s.proc.Kill()
		return nil, ErrStopInterrupted
	}
	return s.Result()
}

// Result returns the map of a session that has ended. The store settles whether
// a map was saved, not the exit code.
func (s *NativeSession) Result() (*Map, error) {
	code, _ := s.proc.Wait()
	built, err := s.decl.Find(filepath.Base(s.Dir))
	var missing *ErrNoSuchMap
	if err != nil && !errors.As(err, &missing) {
		return nil, err
	}
	if built != nil && built.Grid != "" {
		return built, nil
	}
	if code == sessionNoMap {
		return nil, ErrNoMapBuilt
	}
	return nil, &ErrSessionExited{Code: code}
}

// announcement passes the session's output through to next, and reports the
// map directory once the session prints it.
type announcement struct {
	next io.Writer
	dir  chan string

	mu      sync.Mutex
	pending []byte
	found   bool
}

func newAnnouncement(next io.Writer) *announcement {
	return &announcement{next: next, dir: make(chan string, 1)}
}

func (a *announcement) Write(p []byte) (int, error) {
	a.mu.Lock()
	if !a.found {
		a.pending = append(a.pending, p...)
		for {
			end := bytes.IndexByte(a.pending, '\n')
			if end < 0 {
				break
			}
			line := strings.TrimSpace(string(a.pending[:end]))
			a.pending = a.pending[end+1:]
			if strings.HasPrefix(line, announcePrefix) {
				a.found, a.pending = true, nil
				a.dir <- strings.TrimPrefix(line, announcePrefix)
				break
			}
		}
	}
	a.mu.Unlock()
	return a.next.Write(p)
}
