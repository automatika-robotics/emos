package mapping

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrNoSuchMap is returned when the named map is not in the store.
type ErrNoSuchMap struct {
	Name  string
	Store string
}

func (e *ErrNoSuchMap) Error() string {
	return fmt.Sprintf("no map named %q in %s", e.Name, e.Store)
}

// ErrMapIsActive refuses to delete the map the robot is currently using.
type ErrMapIsActive struct{ Name string }

func (e *ErrMapIsActive) Error() string {
	return fmt.Sprintf("%q is the active map", e.Name)
}

// Remove deletes a map from the store.
//
// The path always comes from List rather than from the caller, and the active
// map is refused outright.
func (d *Declaration) Remove(name string, run Runner) error {
	if err := d.checkLocal(); err != nil {
		return err
	}
	target, err := d.find(name)
	if err != nil {
		return err
	}
	if target.Active {
		return &ErrMapIsActive{Name: name}
	}

	if d.Kind == KindVendor && d.Vendor != nil && len(d.Vendor.Remove) > 0 {
		return run(d.command(d.Vendor.Remove, name))
	}
	if d.Kind == KindVendor && d.Vendor != nil && d.Vendor.RequiresRoot {
		return run([]string{"sudo", "rm", "-rf", "--", target.Path})
	}
	return os.RemoveAll(target.Path)
}

// Export packages a map for copying off the robot, returning the archive path
// when it can be identified.
//
// Vendors commonly package only the *active* map and take no argument, so a
// name that is not the active one is refused.
func (d *Declaration) Export(name string, run Runner) (string, error) {
	if err := d.checkLocal(); err != nil {
		return "", err
	}
	if d.Kind != KindVendor || d.Vendor == nil || len(d.Vendor.Export) == 0 {
		return "", fmt.Errorf("this robot's plugin declares no way to export a map")
	}
	target, err := d.find(name)
	if err != nil {
		return "", err
	}
	if !target.Active && !hasPlaceholder(d.Vendor.Export) {
		return "", fmt.Errorf(
			"this robot can only export the active map, and %q is not it", name)
	}

	before := snapshotDir(d.Vendor.ExportDir)
	if err := run(d.command(d.Vendor.Export, name)); err != nil {
		return "", err
	}
	return newestNew(d.Vendor.ExportDir, before), nil
}

// find resolves a map name against the store.
func (d *Declaration) find(name string) (*Map, error) {
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

// ActiveName returns the name of the active map, or "".
func (d *Declaration) ActiveName() string {
	maps, err := d.List()
	if err != nil {
		return ""
	}
	for _, m := range maps {
		if m.Active {
			return m.Name
		}
	}
	return ""
}

func hasPlaceholder(argv []string) bool {
	for _, token := range argv {
		if strings.Contains(token, "{name}") {
			return true
		}
	}
	return false
}

// snapshotDir records the files in dir, so one appearing later can be named.
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
// before, or "" when the directory is unknown or nothing appeared.
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
		if before[e.Name()] {
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
