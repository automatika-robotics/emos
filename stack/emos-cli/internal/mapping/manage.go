package mapping

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// ErrNoSuchMap is returned when the named map is not in the store.
type ErrNoSuchMap struct {
	Name  string
	Store string
}

func (e *ErrNoSuchMap) Error() string {
	return fmt.Sprintf("no map named %q in %s", e.Name, e.Store)
}

// ErrNoSuchArchive is returned when an archive to import is not on disk.
type ErrNoSuchArchive struct{ Path string }

func (e *ErrNoSuchArchive) Error() string {
	return fmt.Sprintf("no archive at %s", e.Path)
}

// ErrMapIsActive refuses to delete the map the robot is currently using.
type ErrMapIsActive struct{ Name string }

func (e *ErrMapIsActive) Error() string {
	return fmt.Sprintf("%q is the active map", e.Name)
}

// Remove deletes a map from the store.
//
// The path always comes from List rather than from the caller, and the active
// map is refused outright. As with every vendor command, the store settles
// whether it worked, not the exit code.
func (d *Declaration) Remove(name string, run Runner) error {
	if err := d.checkLocal(); err != nil {
		return err
	}
	target, err := d.Find(name)
	if err != nil {
		return err
	}
	if target.Active {
		return &ErrMapIsActive{Name: name}
	}

	var argv []string
	switch {
	case len(d.Vendor.Remove) > 0:
		argv = render(d.Vendor.Remove, mapVars(target))
	case d.Vendor.RequiresRoot:
		// No vendor delete command, and the store is owned by the robot's own
		// software, so the CLI escalates on its own.
		argv = []string{"sudo", "rm", "-rf", "--", target.Path}
	default:
		return os.RemoveAll(target.Path)
	}
	if err := run(argv); err != nil {
		return err
	}
	var missing *ErrNoSuchMap
	if _, err := d.Find(name); errors.As(err, &missing) {
		return nil
	} else if err != nil {
		return err
	}
	return fmt.Errorf("the remove command ran but %s is still there", target.Path)
}

// Use makes a map the active one by running the declared apply, then
// after_apply.
//
// The exit code is not trusted: the active link is re-read after apply.
func (d *Declaration) Use(name string, run Runner) error {
	if err := d.checkLocal(); err != nil {
		return err
	}
	if len(d.Vendor.Apply) == 0 {
		return fmt.Errorf("this robot's plugin declares no way to make a map active")
	}
	target, err := d.Find(name)
	if err != nil {
		return err
	}

	if err := run(render(d.Vendor.Apply, mapVars(target))); err != nil {
		return fmt.Errorf("apply map: %w", err)
	}
	active, err := d.ActiveName()
	if err != nil {
		return err
	}
	if active != target.Name {
		if active == "" {
			return fmt.Errorf("the apply ran but no map is marked active")
		}
		return fmt.Errorf("the apply ran but the active map is still %q", active)
	}
	if len(d.Vendor.AfterApply) > 0 {
		if err := run(render(d.Vendor.AfterApply, mapVars(target))); err != nil {
			return fmt.Errorf("%q is active, but the follow-up command failed: %w", target.Name, err)
		}
	}
	return nil
}

// Export packages a map for copying off the robot, returning the archive path
// when it can be identified.
//
// Vendors commonly package only the *active* map and take no argument, so a
// name that is not the active one is refused. Vendors also choose where the
// archive lands so it is moved into dest, somewhere predictable and
// provider-independent. Empty dest leaves it. When the move fails, the path
// returned is where the archive still is.
func (d *Declaration) Export(name, dest string, run Runner) (string, error) {
	if err := d.checkLocal(); err != nil {
		return "", err
	}
	if len(d.Vendor.Export) == 0 {
		return "", fmt.Errorf("this robot's plugin declares no way to export a map")
	}
	target, err := d.Find(name)
	if err != nil {
		return "", err
	}
	if !target.Active && !hasPlaceholder(d.Vendor.Export) {
		return "", fmt.Errorf(
			"this robot can only export the active map, and %q is not it", name)
	}

	before := snapshotDir(d.Vendor.ExportDir)
	if err := run(render(d.Vendor.Export, mapVars(target))); err != nil {
		return "", err
	}
	archive := newestNew(d.Vendor.ExportDir, before)
	if archive == "" || dest == "" {
		return archive, nil
	}
	return relocate(archive, dest)
}

// relocate moves the archive src into the dest directory, copying when the two
// are on different filesystems. An archive of the same name is never replaced.
func relocate(src, dest string) (string, error) {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return src, err
	}
	target := filepath.Join(dest, filepath.Base(src))
	if _, err := os.Lstat(target); err == nil {
		return src, fmt.Errorf("%s already exists", target)
	}
	switch err := os.Rename(src, target); {
	case err == nil:
		return target, nil
	case !errors.Is(err, syscall.EXDEV):
		return src, err
	}
	if err := copyFile(src, target); err != nil {
		return src, err
	}
	// The copy is what matters; an original left behind is harmless.
	_ = os.Remove(src)
	return target, nil
}

// copyFile copies src to a new file dst, removing dst again if the copy fails.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(dst)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(dst)
		return err
	}
	return nil
}

// Find resolves a map name against the store.
func (d *Declaration) Find(name string) (*Map, error) {
	maps, err := d.List()
	if err != nil {
		return nil, err
	}
	for i := range maps {
		if maps[i].Name == name {
			return &maps[i], nil
		}
	}
	return nil, &ErrNoSuchMap{Name: name, Store: d.Store()}
}

// ActiveName returns the name of the active map, or "" when none is marked.
func (d *Declaration) ActiveName() (string, error) {
	maps, err := d.List()
	if err != nil {
		return "", err
	}
	for _, m := range maps {
		if m.Active {
			return m.Name, nil
		}
	}
	return "", nil
}

// mapVars are the substitutions for a command acting on a map in the store.
func mapVars(m *Map) vars {
	return vars{"name": m.Name, "path": m.Path}
}

// hasPlaceholder reports whether a command names the map it acts on.
func hasPlaceholder(argv []string) bool {
	for _, token := range argv {
		if strings.Contains(token, "{name}") || strings.Contains(token, "{path}") {
			return true
		}
	}
	return false
}

// snapshotDir records the entries in dir, so one appearing later can be named.
func snapshotDir(dir string) map[string]bool {
	seen := map[string]bool{}
	if dir == "" {
		return seen
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return seen
	}
	for _, e := range entries {
		seen[e.Name()] = true
	}
	return seen
}

// newestNew returns the most recently modified file in dir that was not in
// before, or "" when the directory is unknown or no file appeared. An archive
// is a file, so directories are skipped.
func newestNew(dir string, before map[string]bool) string {
	if dir == "" {
		return ""
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	best, bestPath := int64(-1), ""
	for _, e := range entries {
		if before[e.Name()] || e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if ts := info.ModTime().UnixNano(); ts > best {
			best, bestPath = ts, filepath.Join(dir, e.Name())
		}
	}
	return bestPath
}

// Import unpacks an archive produced by Export into the store and returns the
// map that appeared.
//
// A bare filename is looked up in dest so an export and an import round-trip
// without the operator retyping a path.
func (d *Declaration) Import(archive, dest string, run Runner) (*Map, error) {
	if err := d.checkLocal(); err != nil {
		return nil, err
	}
	if len(d.Vendor.Import) == 0 {
		return nil, fmt.Errorf("this robot's plugin declares no way to import a map")
	}

	path := archive
	if _, err := os.Stat(path); err != nil && dest != "" && filepath.Base(archive) == archive {
		path = filepath.Join(dest, archive)
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, &ErrNoSuchArchive{Path: archive}
	}
	if info.IsDir() {
		return nil, fmt.Errorf("%s is a directory, not an archive", path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	before, err := d.snapshot()
	if err != nil {
		return nil, err
	}
	if err := run(render(d.Vendor.Import, vars{"path": abs})); err != nil {
		return nil, fmt.Errorf("import map: %w", err)
	}
	if m := d.appeared(before); m != nil {
		return m, nil
	}
	return nil, fmt.Errorf(
		"the import ran but no new map appeared in %s; a map of the same name may already be there",
		d.Store())
}
