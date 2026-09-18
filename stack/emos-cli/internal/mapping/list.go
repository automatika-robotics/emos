package mapping

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Map is one map in the store.
type Map struct {
	Name     string
	Path     string
	Active   bool
	Modified time.Time
	// Occupancy-grid YAML a recipe hands to Kompass's MapServer,
	// empty when the map directory does not contain one.
	Grid string
}

// ErrStoreUnreadable reports a map store that exists but this user cannot read.
type ErrStoreUnreadable struct {
	Store string
	Err   error
}

func (e *ErrStoreUnreadable) Error() string {
	return fmt.Sprintf("map store %s is not readable by this user: %v", e.Store, e.Err)
}

func (e *ErrStoreUnreadable) Unwrap() error { return e.Err }

// List returns every map in the provider's store, oldest first.
func (d *Declaration) List() ([]Map, error) {
	store := d.Store()
	if store == "" {
		return nil, fmt.Errorf("the plugin's mapping declaration names no map store")
	}
	if err := d.checkLocal(); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(store)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if errors.Is(err, fs.ErrPermission) {
		return nil, &ErrStoreUnreadable{Store: store, Err: err}
	}
	if err != nil {
		return nil, err
	}

	active := d.resolveActive(store)

	var maps []Map
	for _, entry := range entries {
		name := entry.Name()
		if name == d.activeLink() {
			// The active marker is a link to one of the entries below.
			continue
		}
		full := filepath.Join(store, name)
		// Follow links so a store of symlinked maps still lists as directories.
		info, err := os.Stat(full)
		if err != nil || !info.IsDir() {
			continue
		}
		maps = append(maps, Map{
			Name:     name,
			Path:     full,
			Active:   name == active,
			Modified: info.ModTime(),
			Grid:     d.findGrid(full),
		})
	}
	sort.Slice(maps, func(i, j int) bool {
		return maps[i].Modified.Before(maps[j].Modified)
	})
	return maps, nil
}

// findGrid returns the occupancy-grid YAML in a map directory: the declared grid
// file when present, else the only .yaml there, else "". Filenames vary between
// maps in one store, and this matches how a recipe finds the grid through
// sugarcoat's active_grid_path().
func (d *Declaration) findGrid(dir string) string {
	if grid := d.gridFile(); grid != "" {
		if path := filepath.Join(dir, grid); fileExists(path) {
			return path
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	found := ""
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		if found != "" {
			return ""
		}
		found = filepath.Join(dir, entry.Name())
	}
	return found
}

// resolveActive returns the name of the map the active link points at, or "".
func (d *Declaration) resolveActive(store string) string {
	link := d.activeLink()
	if link == "" {
		return ""
	}
	target, err := os.Readlink(filepath.Join(store, link))
	if err != nil {
		return ""
	}
	// The link may be absolute or relative to the store.
	return filepath.Base(target)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
