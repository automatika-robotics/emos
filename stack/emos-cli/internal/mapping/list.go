package mapping

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/automatika-robotics/emos-cli/internal/config"
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

// Store returns the directory holding this provider's maps.
func (d *Declaration) Store() string {
	if d.Kind == KindVendor && d.Vendor != nil {
		return d.Vendor.Store
	}
	return config.MapsDir
}

// gridName is the occupancy-grid filename to look for inside a map directory.
func (d *Declaration) gridName() string {
	if d.Kind == KindVendor && d.Vendor != nil && d.Vendor.Grid != "" {
		return d.Vendor.Grid
	}
	return "occ_grid.yaml"
}

// activeLink is the symlink naming the active map, if the provider uses one.
func (d *Declaration) activeLink() string {
	if d.Kind == KindVendor && d.Vendor != nil {
		return d.Vendor.ActiveLink
	}
	return "active"
}

// List returns every map in the provider's store, oldest first.
func (d *Declaration) List() ([]Map, error) {
	store := d.Store()
	if store == "" {
		return nil, fmt.Errorf("the plugin's mapping declaration names no map store")
	}
	if d.Kind == KindVendor && d.Vendor != nil && d.Vendor.Host != "" &&
		d.Vendor.Host != "local" {
		return nil, fmt.Errorf(
			"the map store lives on %s; reading a remote store is not supported yet",
			d.Vendor.Host)
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
	grid := d.gridName()

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
		m := Map{
			Name:     name,
			Path:     full,
			Active:   name == active,
			Modified: info.ModTime(),
		}
		if gridPath := filepath.Join(full, grid); fileExists(gridPath) {
			m.Grid = gridPath
		}
		maps = append(maps, m)
	}
	sort.Slice(maps, func(i, j int) bool {
		return maps[i].Modified.Before(maps[j].Modified)
	})
	return maps, nil
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
