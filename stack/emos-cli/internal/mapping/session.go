package mapping

import (
	"fmt"
	"time"
)

// How long to wait for a map to appear after asking the provider to stop, and
// how often to look. Finishing a map is post-processing, not instant.
var (
	stopTimeout = 90 * time.Second
	stopPoll    = 2 * time.Second
)

// Session is a mapping run in progress.
type Session struct {
	decl   *Declaration
	run    Runner
	name   string
	before map[string]bool
}

// Start begins a mapping session.
//
// name is passed to the provider as given; it may append its own timestamp, so
// the map that appears is not necessarily called this.
func (d *Declaration) Start(name string, run Runner) (*Session, error) {
	if d.Kind != KindVendor || d.Vendor == nil {
		return nil, fmt.Errorf("only vendor mapping can be started this way")
	}
	if err := d.checkLocal(); err != nil {
		return nil, err
	}
	if len(d.Vendor.Start) == 0 {
		return nil, fmt.Errorf("this robot's plugin declares no way to start mapping")
	}

	before, err := d.snapshot()
	if err != nil {
		return nil, err
	}
	if err := run(d.command(d.Vendor.Start, vars{"name": name})); err != nil {
		return nil, fmt.Errorf("start mapping: %w", err)
	}
	return &Session{decl: d, run: run, name: name, before: before}, nil
}

// Stop ends the session and returns the map that appeared.
//
// A clean exit code from the provider does not mean a map was written, so what
// Stop waits on is the store, not the return code. Re-issuing stop is opt-in
// via the declaration's stop_retries.
func (s *Session) Stop() (*Map, error) {
	d := s.decl
	var lastErr error
	for attempt := 0; attempt <= d.Vendor.StopRetries; attempt++ {
		if err := s.run(d.command(d.Vendor.Stop, vars{"name": s.name})); err != nil {
			lastErr = err
		}
		if m := s.await(stopTimeout); m != nil {
			return m, nil
		}
	}
	if lastErr != nil {
		return nil, fmt.Errorf("stop mapping: %w", lastErr)
	}
	return nil, fmt.Errorf(
		"mapping stopped but no map appeared in %s within %s; the robot may still be processing",
		d.Store(), stopTimeout)
}

// Name is the name the session was started with.
func (s *Session) Name() string { return s.name }

// await polls the store until a new map appears or the deadline passes.
func (s *Session) await(timeout time.Duration) *Map {
	deadline := time.Now().Add(timeout)
	for {
		if m := s.decl.appeared(s.before); m != nil {
			return m
		}
		if !time.Now().Before(deadline) {
			return nil
		}
		time.Sleep(stopPoll)
	}
}

// snapshot records which maps the store held, so the one that appears later can
// be identified even though the provider chose its name.
func (d *Declaration) snapshot() (map[string]bool, error) {
	maps, err := d.List()
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(maps))
	for _, m := range maps {
		seen[m.Name] = true
	}
	return seen, nil
}

// appeared returns the map that is in the store now but not in before, newest
// first when more than one is.
func (d *Declaration) appeared(before map[string]bool) *Map {
	maps, err := d.List()
	if err != nil {
		return nil
	}
	var found *Map
	for i := range maps {
		m := maps[i]
		if before[m.Name] {
			continue
		}
		if found == nil || m.Modified.After(found.Modified) {
			found = &m
		}
	}
	return found
}
