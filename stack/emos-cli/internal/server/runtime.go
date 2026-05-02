package server

import (
	"errors"
	"sync"
	"time"

	"github.com/automatika-robotics/emos-cli/internal/runner"
)

// RunStatus is the lifecycle state of a recipe run.
//	preparing -> running -> finished | failed | canceled
//	preparing -> failed | canceled (pre-flight error / abort)
type RunStatus string

const (
	RunStatusPreparing RunStatus = "preparing"
	RunStatusRunning   RunStatus = "running"
	RunStatusFinished  RunStatus = "finished"
	RunStatusFailed    RunStatus = "failed"
	RunStatusCanceled  RunStatus = "canceled"
)

// Run is the daemon-side record of a recipe execution. It is the JSON shape
// returned by /runs and /runs/{id}.
//
// Concurrency invariants:
//   - Status, ExitCode, FinishedAt, Error: only mutated under Runtime.mu.
//   - handle: only set by AttachHandle (under Runtime.mu); after the
//     handleAttached channel is closed, readers can read it safely thanks
//     to the channel-close happens-before guarantee.
//   - cancelCh / handleAttached: closed at most once each.
type Run struct {
	ID         string    `json:"id"`
	Recipe     string    `json:"recipe"`
	Status     RunStatus `json:"status"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at,omitempty"`
	ExitCode   int       `json:"exit_code"`
	LogPath    string    `json:"log_path"`
	RMW        string    `json:"rmw"`
	Error      string    `json:"error,omitempty"`

	handle          *runner.RunHandle `json:"-"`
	cancelCh        chan struct{}     `json:"-"` // closed by CancelPreflight
	handleAttached  chan struct{}     `json:"-"` // closed by AttachHandle
}

// CancelCh returns a channel that closes if the run is cancelled before its
// recipe process is started.
func (r *Run) CancelCh() <-chan struct{} { return r.cancelCh }

// HandleAttached returns a channel that closes once the recipe process has
// been started and r.handle is safe to read.
func (r *Run) HandleAttached() <-chan struct{} { return r.handleAttached }

// Handle returns the live process handle. Only safe to call AFTER
// HandleAttached() has closed; otherwise returns nil.
func (r *Run) Handle() *runner.RunHandle {
	select {
	case <-r.handleAttached:
		return r.handle
	default:
		return nil
	}
}

// isTerminal reports whether the run has already moved into a final state.
// Caller MUST hold Runtime.mu.
func (r *Run) isTerminal() bool {
	switch r.Status {
	case RunStatusFinished, RunStatusFailed, RunStatusCanceled:
		return true
	}
	return false
}

// Runtime owns the at-most-one currently running recipe plus a small
// in-memory ring of recently finished runs
type Runtime struct {
	mu      sync.Mutex
	current *Run
	history []*Run
	maxHist int
}

func NewRuntime() *Runtime {
	return &Runtime{maxHist: 10}
}

// Current returns the active run, or nil.
func (rt *Runtime) Current() *Run {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	return rt.current
}

// List returns running + history (most recent first), in a snapshot copy.
func (rt *Runtime) List() []Run {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	out := make([]Run, 0, len(rt.history)+1)
	if rt.current != nil {
		out = append(out, *rt.current)
	}
	for i := len(rt.history) - 1; i >= 0; i-- {
		out = append(out, *rt.history[i])
	}
	return out
}

// Get looks up a run by id (active or history).
func (rt *Runtime) Get(id string) *Run {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.current != nil && rt.current.ID == id {
		c := *rt.current
		return &c
	}
	for _, r := range rt.history {
		if r.ID == id {
			c := *r
			return &c
		}
	}
	return nil
}

// TryLock atomically registers a run as the active one without starting an
// exit watcher. Used to claim the single-recipe slot at the start of a
// preparing-phase goroutine.
func (rt *Runtime) TryLock(r *Run) error {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.current != nil {
		return runner.ErrAlreadyRunning
	}
	if r.cancelCh == nil {
		r.cancelCh = make(chan struct{})
	}
	if r.handleAttached == nil {
		r.handleAttached = make(chan struct{})
	}
	rt.current = r
	return nil
}

// AttachHandle moves a preparing run to running, attaches the live process
// handle, and starts the exit watcher. After this returns, r.handle is
// safely readable by anyone who waits on r.HandleAttached().
func (rt *Runtime) AttachHandle(r *Run, h *runner.RunHandle) {
	rt.mu.Lock()
	if r.isTerminal() {
		// Cancelled or failed during preparing; don't attach.
		rt.mu.Unlock()
		_ = h.Cancel(2 * time.Second)
		return
	}
	r.handle = h
	r.Status = RunStatusRunning
	r.StartedAt = h.StartedAt
	rt.mu.Unlock()
	closeOnce(r.handleAttached)
	go rt.watch(r)
}

// FailPreflight transitions a preparing run to failed and rotates it into
// history. Idempotent — second calls on an already-terminal run no-op.
func (rt *Runtime) FailPreflight(r *Run, err error) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if r.isTerminal() {
		return
	}
	r.Status = RunStatusFailed
	r.Error = err.Error()
	r.FinishedAt = time.Now()
	rt.rotateLocked(r)
}

// CancelPreflight transitions a preparing run to canceled. Idempotent.
// Closes the cancel channel so the pre-flight goroutine bails at its next
// checkpoint; no SIGTERM needed because no process exists yet.
func (rt *Runtime) CancelPreflight(r *Run) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if r.isTerminal() {
		return
	}
	closeOnce(r.cancelCh)
	r.Status = RunStatusCanceled
	r.FinishedAt = time.Now()
	rt.rotateLocked(r)
}

// Cancel terminates the active run if its id matches. Routes to either the
// preparing-phase abort or the running-phase signal-kill.
func (rt *Runtime) Cancel(id string) error {
	rt.mu.Lock()
	cur := rt.current
	if cur == nil || cur.ID != id {
		rt.mu.Unlock()
		return errors.New("not running")
	}
	status := cur.Status
	handle := cur.handle
	if status == RunStatusPreparing || handle == nil {
		// Inline the CancelPreflight body to avoid releasing+re-acquiring.
		if cur.isTerminal() {
			rt.mu.Unlock()
			return nil
		}
		closeOnce(cur.cancelCh)
		cur.Status = RunStatusCanceled
		cur.FinishedAt = time.Now()
		rt.rotateLocked(cur)
		rt.mu.Unlock()
		return nil
	}
	// Running case: mark canceled first so the watcher classifies the
	// SIGTERM-induced exit correctly, then release the lock and signal.
	cur.Status = RunStatusCanceled
	rt.mu.Unlock()
	return handle.Cancel(5 * time.Second)
}

// closeOnce closes ch if not already closed. Goroutine-safe via the
// select-default trick; cheap.
func closeOnce(ch chan struct{}) {
	if ch == nil {
		return
	}
	select {
	case <-ch:
	default:
		close(ch)
	}
}

// rotateLocked moves r out of "current" and into history. Caller holds rt.mu.
func (rt *Runtime) rotateLocked(r *Run) {
	if rt.current == r {
		rt.current = nil
	}
	rt.history = append(rt.history, r)
	if len(rt.history) > rt.maxHist {
		rt.history = rt.history[len(rt.history)-rt.maxHist:]
	}
}

// watch waits on the run handle and rotates the run into history on exit.
// Idempotent against parallel CancelPreflight / FailPreflight via isTerminal.
func (rt *Runtime) watch(r *Run) {
	code, err := r.handle.Wait()
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if r.isTerminal() && r.Status != RunStatusCanceled {
		// Already moved to terminal state by another path; nothing to do.
		return
	}
	r.FinishedAt = time.Now()
	r.ExitCode = code
	switch {
	case err != nil && code == -1:
		r.Status = RunStatusFailed
		r.Error = err.Error()
	case err != nil:
		// exec.ExitError after a SIGTERM-from-Cancel = canceled; otherwise failed
		if r.Status != RunStatusCanceled {
			r.Status = RunStatusFailed
			r.Error = err.Error()
		}
	default:
		r.Status = RunStatusFinished
	}
	rt.rotateLocked(r)
}

