package mapping

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// How long to wait for a map to appear after asking the provider to stop, and
// how often to look. Finishing a map is post-processing, not instant.
var (
	stopTimeout = 90 * time.Second
	stopPoll    = 2 * time.Second
)

// ErrStopInterrupted is returned when Stop is cancelled before a map appears.
// The provider may still be mapping.
var ErrStopInterrupted = errors.New("stopping mapping was interrupted")

// validName is what a map name may look like. It goes to a vendor tool that
// may run as root, so it must not read as an option or a path.
var validName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// Session is a mapping run in progress.
type Session struct {
	decl   *Declaration
	run    Runner
	name   string
	before map[string]bool
}

// CanStart returns why this robot cannot run a mapping session, or nil.
func (d *Declaration) CanStart() error {
	if err := d.checkLocal(); err != nil {
		return err
	}
	if len(d.Vendor.Start) == 0 || len(d.Vendor.Stop) == 0 {
		return fmt.Errorf("this robot's plugin does not declare how to start and stop mapping")
	}
	return nil
}

// Start begins a mapping session.
//
// name is passed to the provider as given; it may append its own timestamp, so
// the map that appears is not necessarily called this.
func (d *Declaration) Start(name string, run Runner) (*Session, error) {
	if err := d.CanStart(); err != nil {
		return nil, err
	}
	if !validName.MatchString(name) {
		return nil, fmt.Errorf(
			"map name %q: use letters, digits, '.', '_' or '-', starting with a letter or digit", name)
	}

	before, err := d.snapshot()
	if err != nil {
		return nil, err
	}
	if err := run(render(d.Vendor.Start, vars{"name": name})); err != nil {
		return nil, fmt.Errorf("start mapping: %w", err)
	}
	return &Session{decl: d, run: run, name: name, before: before}, nil
}

// Stop ends the session and returns the map that appeared.
//
// A clean exit code from the provider does not mean a map was written, so what
// Stop waits on is the store, not the return code. Re-issuing stop is opt-in
// via the declaration's stop_retries. Cancelling ctx gives up with
// ErrStopInterrupted.
func (s *Session) Stop(ctx context.Context) (*Map, error) {
	d := s.decl
	var lastErr error
	for attempt := 0; attempt <= d.Vendor.StopRetries; attempt++ {
		if err := s.run(render(d.Vendor.Stop, vars{"name": s.name})); err != nil {
			lastErr = err
		}
		m, err := s.await(ctx, stopTimeout)
		if err != nil {
			return nil, ErrStopInterrupted
		}
		if m != nil {
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

// StopCommand is the stop command as it would run, for an operator who has to
// run it by hand.
func (s *Session) StopCommand() string {
	return strings.Join(render(s.decl.Vendor.Stop, vars{"name": s.name}), " ")
}

// await polls the store until a new map appears, the timeout passes (nil, nil)
// or ctx is cancelled.
func (s *Session) await(ctx context.Context, timeout time.Duration) (*Map, error) {
	deadline := time.Now().Add(timeout)
	for {
		if m := s.decl.appeared(s.before); m != nil {
			return m, nil
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !time.Now().Before(deadline) {
			return nil, nil
		}
		select {
		case <-ctx.Done():
		case <-time.After(stopPoll):
		}
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
