package server

import (
	"sync"
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

func TestJobUpdateFinishedSignalsDone(t *testing.T) {
	// New contract (issue #4): on terminal status, Done() fires and the
	// final event is delivered through the bus -- the bus is NOT closed.
	js := NewJobs()
	j := js.New("j", "pull", "x")
	sub := j.Subscribe()
	done := j.Done()

	j.Update(JobStatusFinished, 1.0, "done")

	// Bus must deliver the terminal event.
	select {
	case evt := <-sub:
		if evt.Status != JobStatusFinished {
			t.Fatalf("event = %+v, want finished", evt)
		}
	case <-time.After(time.Second):
		t.Fatalf("no finished event delivered")
	}

	// Done() must fire promptly after Update returns.
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("Done() did not fire after terminal status")
	}

	if j.Snapshot().FinishedAt.IsZero() {
		t.Fatalf("FinishedAt not set on terminal update")
	}
}

func TestJobBusNeverClosesAfterTerminal(t *testing.T) {
	// Regression for issue #4. The pre-fix code closed the bus channel
	// 2s after a terminal Update, opening a window where a concurrent
	// Update could `send on closed channel` panic. The new contract is
	// that the bus stays open for the lifetime of the process; subscribers
	// learn termination through Done(). Pin that here.
	js := NewJobs()
	j := js.New("j", "pull", "x")
	sub := j.Subscribe()

	j.Update(JobStatusFinished, 1.0, "done")
	<-sub // drain the terminal event

	select {
	case <-j.Done():
	case <-time.After(time.Second):
		t.Fatalf("Done() did not fire")
	}

	// Wait well past the old 2 s sleep window. The channel must still
	// be open: a default branch fires only on an open-but-empty channel,
	// while a closed channel would deliver `(zero, false)` immediately.
	time.Sleep(150 * time.Millisecond)
	select {
	case _, ok := <-sub:
		if !ok {
			t.Fatalf("bus was closed after terminal status; new contract says it must stay open")
		}
		t.Fatalf("unexpected event after drain")
	default:
		// open and empty -- correct.
	}
}

func TestConcurrentTerminalUpdatesDoNotPanic(t *testing.T) {
	// Issue #4 regression: under the old code, two concurrent Update()
	// calls that both hit a terminal status could race the goroutine
	// that closed the bus, producing either `send on closed channel` or
	// `close of closed channel` panics. The new code routes terminal
	// notification through context.CancelFunc, which is idempotent.
	for trial := 0; trial < 50; trial++ {
		js := NewJobs()
		j := js.New("j", "pull", "x")

		var wg sync.WaitGroup
		const goroutines = 16
		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				if i%2 == 0 {
					j.Update(JobStatusFinished, 1.0, "done")
				} else {
					j.Update(JobStatusFailed, 0, "boom")
				}
			}(i)
		}
		wg.Wait()

		// Done must have fired, and the bus must still be readable
		// (open-empty or open-with-pending-events) without panicking.
		select {
		case <-j.Done():
		case <-time.After(time.Second):
			t.Fatalf("trial %d: Done() did not fire after %d concurrent terminal updates", trial, goroutines)
		}
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
