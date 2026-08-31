package server

import (
	"testing"
	"time"
)

// preflightRun locks a preparing run into the runtime and registers a tracked
// goroutine that behaves like runRecipeAsync's checkpoints: it waits for the
// cancel signal, transitions the run, and runs its cleanup.
func preflightRun(t *testing.T, s *Server, id string) (run *Run, cleaned chan struct{}) {
	t.Helper()
	run = &Run{ID: id, Recipe: "r", Status: RunStatusPreparing,
		cancelCh: make(chan struct{}), handleAttached: make(chan struct{})}
	if err := s.runtime.TryLock(run); err != nil {
		t.Fatal(err)
	}
	cleaned = make(chan struct{})
	s.goTracked(func() {
		<-run.CancelCh()
		s.runtime.CancelPreflight(run)
		close(cleaned) // stands in for strategy.Cleanup()
	})
	return run, cleaned
}

func TestDrainRunsStopsActiveRun(t *testing.T) {
	s := newTestServer(t, true)
	run, cleaned := preflightRun(t, s, "r1")

	start := time.Now()
	s.drainRuns(3 * time.Second)

	select {
	case <-cleaned:
	default:
		t.Fatal("cleanup did not run before drainRuns returned")
	}
	if got := s.runtime.Get(run.ID); got == nil || got.Status != RunStatusCanceled {
		t.Fatalf("run status = %+v, want canceled", got)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("drain took %v; should return as soon as goroutines finish", elapsed)
	}
}

func TestDrainRunsNoActiveRun(t *testing.T) {
	s := newTestServer(t, true)
	start := time.Now()
	s.drainRuns(3 * time.Second)
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("drain with nothing to do took %v", elapsed)
	}
}

func TestDrainRunsBoundedByTimeout(t *testing.T) {
	s := newTestServer(t, true)
	release := make(chan struct{})
	s.goTracked(func() { <-release }) // a goroutine stuck past its checkpoint
	defer close(release)

	start := time.Now()
	s.drainRuns(150 * time.Millisecond)
	elapsed := time.Since(start)
	if elapsed < 150*time.Millisecond || elapsed > 2*time.Second {
		t.Fatalf("drain elapsed %v, want ~the 150ms bound", elapsed)
	}
}
