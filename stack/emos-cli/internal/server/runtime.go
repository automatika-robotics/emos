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

	handle   *runner.RunHandle `json:"-"`
	cancelCh chan struct{}     `json:"-"` // closed by Cancel during preparing
}

// CancelCh returns a channel that closes if the run is cancelled before its
// recipe process is started.
func (r *Run) CancelCh() <-chan struct{} { return r.cancelCh }

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

// Adopt registers a freshly-started run as the active one. Caller is
// responsible for ensuring no other run is active (use TryAdopt instead).
func (rt *Runtime) Adopt(r *Run) {
	rt.mu.Lock()
	rt.current = r
	rt.mu.Unlock()
	go rt.watch(r)
}

// TryLock atomically registers a run as the active one without starting an
// exit watcher. Used to claim the single-recipe slot at the start of a
// preparing-phase goroutine
func (rt *Runtime) TryLock(r *Run) error {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.current != nil {
		return runner.ErrAlreadyRunning
	}
	if r.cancelCh == nil {
		r.cancelCh = make(chan struct{})
	}
	rt.current = r
	return nil
}

// AttachHandle moves a preparing run to running, attaches the live process
// handle, and starts the exit watcher
func (rt *Runtime) AttachHandle(r *Run, h *runner.RunHandle) {
	rt.mu.Lock()
	r.handle = h
	r.Status = RunStatusRunning
	r.StartedAt = h.StartedAt
	rt.mu.Unlock()
	go rt.watch(r)
}

// FailPreflight transitions a preparing run to failed and rotates it into
// history
func (rt *Runtime) FailPreflight(r *Run, err error) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	r.Status = RunStatusFailed
	r.Error = err.Error()
	r.FinishedAt = time.Now()
	rt.rotateLocked(r)
}

// CancelPreflight transitions a preparing run to canceled (no handle exists
// yet, so nothing to SIGTERM, we just close the cancel channel and let the
// goroutine bail at its next checkpoint).
func (rt *Runtime) CancelPreflight(r *Run) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if r.cancelCh != nil {
		select {
		case <-r.cancelCh:
		default:
			close(r.cancelCh)
		}
	}
	r.Status = RunStatusCanceled
	r.FinishedAt = time.Now()
	rt.rotateLocked(r)
}

// Cancel terminates the active run if its id matches. Routes to either the
// preparing-phase abort or the running-phase signal-kill. For the running
// case we mark the status as canceled BEFORE issuing SIGTERM so the
// watcher classifies the resulting non-zero exit correctly.
func (rt *Runtime) Cancel(id string) error {
	rt.mu.Lock()
	cur := rt.current
	rt.mu.Unlock()
	if cur == nil || cur.ID != id {
		return errors.New("not running")
	}
	if cur.Status == RunStatusPreparing || cur.handle == nil {
		rt.CancelPreflight(cur)
		return nil
	}
	rt.mu.Lock()
	cur.Status = RunStatusCanceled
	rt.mu.Unlock()
	return cur.handle.Cancel(5 * time.Second)
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
func (rt *Runtime) watch(r *Run) {
	code, err := r.handle.Wait()
	rt.mu.Lock()
	defer rt.mu.Unlock()

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

