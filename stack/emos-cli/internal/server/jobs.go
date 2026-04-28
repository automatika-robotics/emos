package server

import (
	"sync"
	"time"
)

// JobStatus is the lifecycle state of a generic background job
type JobStatus string

const (
	JobStatusRunning  JobStatus = "running"
	JobStatusFinished JobStatus = "finished"
	JobStatusFailed   JobStatus = "failed"
)

// JobView is the data-only shape returned to API consumers
type JobView struct {
	ID         string         `json:"id"`
	Kind       string         `json:"kind"`
	Target     string         `json:"target"`
	Status     JobStatus      `json:"status"`
	StartedAt  time.Time      `json:"started_at"`
	FinishedAt time.Time      `json:"finished_at,omitempty"`
	Progress   float64        `json:"progress"`
	Message    string         `json:"message"`
	Error      string         `json:"error,omitempty"`
	Meta       map[string]any `json:"meta,omitempty"`
}

// Job is a generic unit of background work surfaced to the UI as a card.
// TODO: Only used by recipe pull, add install/update with same shape
//
// Job embeds JobView for the user-visible state and adds the synchronisation
// primitives that must never leave the package by value.
type Job struct {
	JobView

	mu  sync.Mutex
	bus chan JobEvent
}

// JobEvent is a single update on the job's progress channel; subscribers receive
// a copy via the events bus.
type JobEvent struct {
	JobID    string    `json:"job_id"`
	At       time.Time `json:"at"`
	Status   JobStatus `json:"status"`
	Progress float64   `json:"progress"`
	Message  string    `json:"message"`
}

// Update mutates the job state under the job's lock and broadcasts to subscribers.
// `progress` and `message` are pass-through (caller owns 0..1 semantics).
func (j *Job) Update(status JobStatus, progress float64, message string) {
	j.mu.Lock()
	j.Status = status
	if progress >= 0 {
		j.Progress = progress
	}
	if message != "" {
		j.Message = message
	}
	if status == JobStatusFinished || status == JobStatusFailed {
		j.FinishedAt = time.Now()
	}
	evt := JobEvent{
		JobID:    j.ID,
		At:       time.Now(),
		Status:   j.Status,
		Progress: j.Progress,
		Message:  j.Message,
	}
	bus := j.bus
	j.mu.Unlock()
	// non-blocking broadcast — drop if buffer full so a slow client never
	// stalls the worker.
	select {
	case bus <- evt:
	default:
	}
	if status == JobStatusFinished || status == JobStatusFailed {
		// Allow the channel to drain, then close.
		go func() {
			time.Sleep(2 * time.Second)
			j.mu.Lock()
			defer j.mu.Unlock()
			select {
			case <-j.bus:
			default:
			}
			close(j.bus)
		}()
	}
}

// Subscribe returns the events channel; closes when the job is done.
func (j *Job) Subscribe() <-chan JobEvent {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.bus
}

// Snapshot returns a lock-free copy of the user-visible state for JSON
// encoding
func (j *Job) Snapshot() JobView {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.JobView
}

// Jobs tracks active + recent jobs in memory.
type Jobs struct {
	mu      sync.Mutex
	all     map[string]*Job
	order   []string
	maxKeep int
}

func NewJobs() *Jobs {
	return &Jobs{all: map[string]*Job{}, maxKeep: 50}
}

// New starts tracking a job and returns it; caller drives Updates from a goroutine.
func (js *Jobs) New(id, kind, target string) *Job {
	j := &Job{
		JobView: JobView{
			ID:        id,
			Kind:      kind,
			Target:    target,
			Status:    JobStatusRunning,
			StartedAt: time.Now(),
		},
		bus: make(chan JobEvent, 16),
	}
	js.mu.Lock()
	js.all[id] = j
	js.order = append(js.order, id)
	if len(js.order) > js.maxKeep {
		drop := js.order[0]
		js.order = js.order[1:]
		delete(js.all, drop)
	}
	js.mu.Unlock()
	return j
}

func (js *Jobs) Get(id string) *Job {
	js.mu.Lock()
	defer js.mu.Unlock()
	return js.all[id]
}

func (js *Jobs) List() []JobView {
	js.mu.Lock()
	defer js.mu.Unlock()
	out := make([]JobView, 0, len(js.order))
	for i := len(js.order) - 1; i >= 0; i-- {
		j := js.all[js.order[i]]
		if j != nil {
			out = append(out, j.Snapshot())
		}
	}
	return out
}
