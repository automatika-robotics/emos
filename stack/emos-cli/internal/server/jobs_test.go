package server

import (
	"testing"
	"time"
)

func TestJobsNewAndGet(t *testing.T) {
	js := NewJobs()
	j := js.New("j1", "pull", "vision_follower")
	if j == nil {
		t.Fatalf("New returned nil")
	}
	if j.ID != "j1" || j.Kind != "pull" || j.Target != "vision_follower" {
		t.Fatalf("Job fields wrong: %+v", j.JobView)
	}
	if j.Status != JobStatusRunning {
		t.Fatalf("initial Status = %q, want running", j.Status)
	}
	if got := js.Get("j1"); got == nil || got.ID != "j1" {
		t.Fatalf("Get returned %v", got)
	}
	if got := js.Get("nope"); got != nil {
		t.Fatalf("Get(missing) = %v, want nil", got)
	}
}

func TestJobsListNewestFirst(t *testing.T) {
	js := NewJobs()
	_ = js.New("a", "pull", "one")
	_ = js.New("b", "pull", "two")
	_ = js.New("c", "pull", "three")

	views := js.List()
	if len(views) != 3 {
		t.Fatalf("List len = %d, want 3", len(views))
	}
	wantOrder := []string{"c", "b", "a"}
	for i, v := range views {
		if v.ID != wantOrder[i] {
			t.Fatalf("List[%d].ID = %q, want %q", i, v.ID, wantOrder[i])
		}
	}
}

func TestJobsRingBufferEvicts(t *testing.T) {
	js := NewJobs()
	js.maxKeep = 3
	for i := 0; i < 5; i++ {
		js.New(string(rune('a'+i)), "pull", "x")
	}
	views := js.List()
	if len(views) != 3 {
		t.Fatalf("List len = %d, want 3 after eviction", len(views))
	}
	// Oldest two should be gone.
	if got := js.Get("a"); got != nil {
		t.Fatalf("Get(a) should be evicted, got %+v", got.JobView)
	}
	if got := js.Get("b"); got != nil {
		t.Fatalf("Get(b) should be evicted, got %+v", got.JobView)
	}
	for _, id := range []string{"c", "d", "e"} {
		if js.Get(id) == nil {
			t.Fatalf("Get(%s) = nil, expected present", id)
		}
	}
}

func TestJobUpdateBroadcasts(t *testing.T) {
	js := NewJobs()
	j := js.New("j", "pull", "x")
	sub := j.Subscribe()

	j.Update(JobStatusRunning, 0.5, "halfway")

	select {
	case evt := <-sub:
		if evt.Status != JobStatusRunning || evt.Progress != 0.5 || evt.Message != "halfway" {
			t.Fatalf("event = %+v, want running/0.5/halfway", evt)
		}
	case <-time.After(time.Second):
		t.Fatalf("no event delivered")
	}

	if snap := j.Snapshot(); snap.Progress != 0.5 || snap.Message != "halfway" {
		t.Fatalf("Snapshot = %+v", snap)
	}
}

func TestJobUpdateFinishedClosesBus(t *testing.T) {
	js := NewJobs()
	j := js.New("j", "pull", "x")
	sub := j.Subscribe()

	j.Update(JobStatusFinished, 1.0, "done")

	// First event: the finished update.
	select {
	case evt := <-sub:
		if evt.Status != JobStatusFinished {
			t.Fatalf("first event = %+v, want finished", evt)
		}
	case <-time.After(time.Second):
		t.Fatalf("no finished event delivered")
	}

	// The bus closes ~2s after the terminal event. Wait for it.
	closed := false
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case _, ok := <-sub:
			if !ok {
				closed = true
			}
		default:
			time.Sleep(50 * time.Millisecond)
		}
		if closed {
			break
		}
	}
	if !closed {
		t.Fatalf("bus channel not closed after terminal status")
	}
	// FinishedAt must be set.
	if j.Snapshot().FinishedAt.IsZero() {
		t.Fatalf("FinishedAt not set on terminal update")
	}
}

func TestJobUpdateProgressMonotonicGuard(t *testing.T) {
	// Negative progress means "don't change". This is a contract callers
	// rely on to bump status without touching progress.
	js := NewJobs()
	j := js.New("j", "pull", "x")
	j.Update(JobStatusRunning, 0.7, "")
	j.Update(JobStatusRunning, -1, "still going")
	if got := j.Snapshot().Progress; got != 0.7 {
		t.Fatalf("Progress = %v after negative update, want unchanged 0.7", got)
	}
	if got := j.Snapshot().Message; got != "still going" {
		t.Fatalf("Message = %q, want still going", got)
	}
}

func TestJobCancelRoutesToContext(t *testing.T) {
	js := NewJobs()
	j := js.New("j", "pull", "x")

	// No cancel registered yet — Cancel must be a no-op, not a panic.
	j.Cancel()

	called := false
	j.SetCancel(func() { called = true })
	j.Cancel()
	if !called {
		t.Fatalf("SetCancel func not invoked by Cancel()")
	}
}
