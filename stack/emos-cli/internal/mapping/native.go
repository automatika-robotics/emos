package mapping

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/automatika-robotics/emos-cli/internal/config"
)

// Exit codes of emos_mapping's session.
const (
	sessionNoMap     = 1 // it ran, but the backend never published a map
	sessionCannotMap = 2 // the plugin could not be mapped with
)

// announcePrefix starts the line the session prints once the map's directory
// exists.
const (
	announcePrefix = "Map directory: "
	// warningPrefix marks a line the session wants the operator to see while
	// driving.
	warningPrefix = "Mapping warning: "
)

// ErrStopTimedOut says the session did not exit after the stop request and
// was killed.
var ErrStopTimedOut = errors.New("the mapping session did not stop in time and was killed")

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
	if mode == config.ModeOSSContainer {
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
// The session's output goes to out, and its warnings for the operator to
// warn, when given. Cancelling ctx before then kills it.
func (d *Declaration) StartNative(ctx context.Context, entryPoint, name string, start Starter, out io.Writer, warn func(string)) (*NativeSession, error) {
	if d.Kind != KindNative {
		return nil, fmt.Errorf("this robot maps with its own software")
	}
	if err := CheckName(name); err != nil {
		return nil, err
	}
	if !validEntryPoint.MatchString(entryPoint) {
		return nil, fmt.Errorf("plugin entry point %q is not '<package.module>:<ClassName>'", entryPoint)
	}

	announced := newAnnouncement(out, warn)
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

// Stop asks the session to stop and returns the map it saved. A session that
// has not exited after stopTimeout, the same grace a vendor's tool gets, is
// killed. Cancelling ctx kills it at once
func (s *NativeSession) Stop(ctx context.Context) (*Map, error) {
	s.proc.Interrupt()
	timeout := time.NewTimer(stopTimeout)
	defer timeout.Stop()
	select {
	case <-s.proc.Done():
		return s.Result()
	case <-timeout.C:
		s.proc.Kill()
		// The map may have been saved before the session hung on its way out
		if built, err := s.decl.Find(filepath.Base(s.Dir)); err == nil && built.Grid != "" {
			return built, nil
		}
		return nil, ErrStopTimedOut
	case <-ctx.Done():
		s.proc.Kill()
		return nil, ErrStopInterrupted
	}
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

// announcement passes the session's output through to next, reports the map
// directory once the session prints it, and forwards the session's warnings.
type announcement struct {
	next io.Writer
	warn func(string)
	dir  chan string

	mu      sync.Mutex
	pending []byte
	found   bool
}

func newAnnouncement(next io.Writer, warn func(string)) *announcement {
	return &announcement{next: next, warn: warn, dir: make(chan string, 1)}
}

func (a *announcement) Write(p []byte) (int, error) {
	a.mu.Lock()
	a.pending = append(a.pending, p...)
	for {
		end := bytes.IndexByte(a.pending, '\n')
		if end < 0 {
			break
		}
		line := strings.TrimSpace(string(a.pending[:end]))
		a.pending = a.pending[end+1:]
		switch {
		case !a.found && strings.HasPrefix(line, announcePrefix):
			a.found = true
			a.dir <- strings.TrimPrefix(line, announcePrefix)
		case a.warn != nil && strings.HasPrefix(line, warningPrefix):
			a.warn(strings.TrimPrefix(line, warningPrefix))
		}
	}
	a.mu.Unlock()
	return a.next.Write(p)
}

// ErrNoGrid refuses to make active a map a recipe could not load.
type ErrNoGrid struct{ Name string }

func (e *ErrNoGrid) Error() string {
	return fmt.Sprintf("%q has no occupancy grid", e.Name)
}

// ErrMapExists refuses an import that would replace a map in the store.
type ErrMapExists struct{ Name string }

func (e *ErrMapExists) Error() string {
	return fmt.Sprintf("a map named %q is already in the store", e.Name)
}

// ErrNotAMapArchive is returned for an archive EMOS did not export.
type ErrNotAMapArchive struct{ Path, Reason string }

func (e *ErrNotAMapArchive) Error() string {
	return fmt.Sprintf("%s is not a map archive: %s", e.Path, e.Reason)
}

// useNative makes a map the active one by repointing the store's active link,
// which is what a recipe follows to its map.
func (d *Declaration) useNative(name string) error {
	target, err := d.Find(name)
	if err != nil {
		return err
	}
	if target.Grid == "" {
		return &ErrNoGrid{Name: name}
	}
	link := filepath.Join(d.Store(), d.activeLink())
	if info, err := os.Lstat(link); err == nil && info.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("%s is not a link, so it cannot be pointed at a map", link)
	}
	// Swapped in by a rename. Its target is relative, so the store can be moved.
	staged := link + ".new"
	_ = os.Remove(staged)
	if err := os.Symlink(target.Name, staged); err != nil {
		return err
	}
	if err := os.Rename(staged, link); err != nil {
		_ = os.Remove(staged)
		return err
	}
	return nil
}

// exportNative packs a map's files into <dest>/<name>.zip and returns its
// path. The backend's working data stays behind.
func (d *Declaration) exportNative(name, dest string) (string, error) {
	target, err := d.Find(name)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return "", err
	}
	archive := filepath.Join(dest, target.Name+".zip")
	if _, err := os.Lstat(archive); err == nil {
		return "", fmt.Errorf("%s already exists", archive)
	}
	partial := archive + ".partial"
	if err := zipMapFiles(target, partial); err != nil {
		_ = os.Remove(partial)
		return "", err
	}
	return archive, os.Rename(partial, archive)
}

// zipMapFiles writes the files of a map directory into a new zip at path, each
// under the map's name.
func zipMapFiles(m *Map, path string) error {
	entries, err := os.ReadDir(m.Path)
	if err != nil {
		return err
	}
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()
	zw := zip.NewWriter(out)
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = m.Name + "/" + entry.Name()
		header.Method = zip.Deflate
		w, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}
		if err := copyInto(w, filepath.Join(m.Path, entry.Name())); err != nil {
			return err
		}
	}
	if err := zw.Close(); err != nil {
		return err
	}
	return out.Close()
}

func copyInto(w io.Writer, path string) error {
	in, err := os.Open(path)
	if err != nil {
		return err
	}
	defer in.Close()
	_, err = io.Copy(w, in)
	return err
}

// importNative unpacks an archive made by exportNative into the store and
// returns the map.
func (d *Declaration) importNative(path string) (*Map, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, &ErrNotAMapArchive{Path: path, Reason: "it is not a zip file"}
	}
	defer r.Close()
	name, err := archivedMapName(r.File)
	if err != nil {
		return nil, &ErrNotAMapArchive{Path: path, Reason: err.Error()}
	}
	store := d.Store()
	final := filepath.Join(store, name)
	if _, err := os.Lstat(final); err == nil || name == d.activeLink() {
		return nil, &ErrMapExists{Name: name}
	}
	if err := os.MkdirAll(store, 0o755); err != nil {
		return nil, err
	}

	staging := filepath.Join(store, ".importing-"+name)
	if err := os.RemoveAll(staging); err != nil {
		return nil, err
	}
	if err := os.Mkdir(staging, 0o755); err != nil {
		return nil, err
	}
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if err := unzipFile(f, filepath.Join(staging, pathpkg.Base(f.Name))); err != nil {
			_ = os.RemoveAll(staging)
			return nil, err
		}
	}
	if err := os.Rename(staging, final); err != nil {
		_ = os.RemoveAll(staging)
		return nil, err
	}
	return d.Find(name)
}

// archivedMapName returns the map an archive holds. Every entry has to be a
// plain file directly under one directory, named as a map may be.
func archivedMapName(files []*zip.File) (string, error) {
	name := ""
	for _, f := range files {
		dir, file, _ := strings.Cut(f.Name, "/")
		if CheckName(dir) != nil || (name != "" && dir != name) {
			return "", fmt.Errorf("entry %q is not inside one map directory", f.Name)
		}
		name = dir
		if file == "" && f.FileInfo().IsDir() {
			continue // the map directory's own entry
		}
		if file == "" || file == "." || file == ".." || strings.Contains(file, "/") || !f.Mode().IsRegular() {
			return "", fmt.Errorf("entry %q is not a plain file in the map directory", f.Name)
		}
	}
	if name == "" {
		return "", errors.New("it is empty")
	}
	return name, nil
}

func unzipFile(f *zip.File, path string) error {
	in, err := f.Open()
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
