package mapping

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// How long to wait for a map to appear after asking the provider to stop, and
// how often to look. Finishing a map is post-processing, not instant.
var (
	stopTimeout = 90 * time.Second
	stopPoll    = 2 * time.Second
)

// Runner executes one rendered argv.
type Runner func(argv []string) error

// SystemRunner runs argv with stdio inherited, so a sudo password prompt
// reaches the terminal and the vendor tool's output stays visible.
func SystemRunner(argv []string) error {
	if len(argv) == 0 {
		return fmt.Errorf("empty command")
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Escalates reports whether a declared argv will ask for a password, so a
// caller can warn before the prompt appears.
func Escalates(argv []string) bool {
	return len(argv) > 0 && argv[0] == "sudo"
}

// VendorSession is a mapping run in progress on the robot's own software.
type VendorSession struct {
	decl   *Declaration
	run    Runner
	name   string
	before map[string]bool
}

// StartVendor begins a mapping session run by the robot's own software.
//
// name is passed to the provider as given; it may append its own timestamp, so
// the map that appears is not necessarily called this.
func (d *Declaration) StartVendor(name string, run Runner) (*VendorSession, error) {
	if d.Kind != KindVendor {
		return nil, fmt.Errorf("this robot does not map with its own software")
	}
	if err := d.CanStart(); err != nil {
		return nil, err
	}
	if err := CheckName(name); err != nil {
		return nil, err
	}

	before, err := d.snapshot()
	if err != nil {
		return nil, err
	}
	if err := run(render(d.Vendor.Start, vars{"name": name})); err != nil {
		return nil, fmt.Errorf("start mapping: %w", err)
	}
	return &VendorSession{decl: d, run: run, name: name, before: before}, nil
}

// Stop ends the session and returns the map that appeared.
//
// A clean exit code from the provider does not mean a map was written, so what
// Stop waits on is the store, not the return code. Re-issuing stop is opt-in
// via the declaration's stop_retries. Cancelling ctx gives up with
// ErrStopInterrupted.
func (s *VendorSession) Stop(ctx context.Context) (*Map, error) {
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
func (s *VendorSession) StopCommand() string {
	return strings.Join(render(s.decl.Vendor.Stop, vars{"name": s.name}), " ")
}

// await polls the store until a new map appears, the timeout passes (nil, nil)
// or ctx is cancelled.
func (s *VendorSession) await(ctx context.Context, timeout time.Duration) (*Map, error) {
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
