package server

import (
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/automatika-robotics/emos-cli/internal/runner"
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

func TestAttachHandleStopsTheProcessOfACancelledRun(t *testing.T) {
	s := newTestServer(t, true)
	run := &Run{ID: "r1", Recipe: "r", Status: RunStatusPreparing}
	if err := s.runtime.TryLock(run); err != nil {
		t.Fatal(err)
	}
	// The cancel lands after the last checkpoint, while the process starts.
	if err := s.runtime.Cancel(run.ID); err != nil {
		t.Fatal(err)
	}
	h, err := runner.StartProcess(exec.Command("sleep", "30"), filepath.Join(t.TempDir(), "run.log"))
	if err != nil {
		t.Fatal(err)
	}
	if s.runtime.AttachHandle(run, h) {
		t.Fatal("a cancelled run must not take the process")
	}
	select {
	case <-run.HandleAttached():
	default:
		t.Fatal("HandleAttached must close, or whatever waits on it never finishes")
	}
	select {
	case <-h.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("the process of a cancelled run was left running")
	}
	if got := s.runtime.Get(run.ID); got == nil || got.Status != RunStatusCanceled {
		t.Fatalf("run = %+v, want canceled", got)
	}
}

func TestStopEndsOpenLogStreamsPromptly(t *testing.T) {
	s := newTestServer(t, true)
	logPath := filepath.Join(t.TempDir(), "run.log")
	if err := os.WriteFile(logPath, []byte("[setup] preparing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run := &Run{ID: "r1", Recipe: "r", Status: RunStatusPreparing, LogPath: logPath}
	if err := s.runtime.TryLock(run); err != nil {
		t.Fatal(err)
	}

	s.httpServer = s.newHTTPServer()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go s.httpServer.Serve(ln)

	// A dashboard tab following the run's log.
	resp, err := http.Get("http://" + ln.Addr().String() + "/api/v1/runs/r1/logs")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("log stream status = %d", resp.StatusCode)
	}
	go io.Copy(io.Discard, resp.Body)

	start := time.Now()
	if err := s.stop(); err != nil {
		t.Fatalf("stop = %v; a clean stop must not fail the unit", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("stop took %v; the open log stream held it up", elapsed)
	}
	if got := s.runtime.Get(run.ID); got == nil || got.Status != RunStatusCanceled {
		t.Fatalf("run = %+v, want canceled", got)
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
