package server

import (
	"bufio"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHandleJobsListEmpty(t *testing.T) {
	s := newTestServer(t, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
	rec := httpServe(t, s, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != "[]" {
		t.Fatalf("body = %q, want []", got)
	}
}

func TestHandleJobGetMissing(t *testing.T) {
	s := newTestServer(t, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/nope", nil)
	rec := httpServe(t, s, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestHandleJobGetExisting(t *testing.T) {
	s := newTestServer(t, true)
	j := s.jobs.New("abc", "recipe_pull", "demo")
	j.Update(JobStatusRunning, 0.42, "downloading")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/abc", nil)
	rec := httpServe(t, s, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var view JobView
	jsonBody(t, rec, &view)
	if view.ID != "abc" || view.Progress != 0.42 || view.Message != "downloading" {
		t.Fatalf("view = %+v", view)
	}
}

func TestHandleJobCancelRoutesToJob(t *testing.T) {
	s := newTestServer(t, true)
	j := s.jobs.New("abc", "recipe_pull", "demo")

	called := false
	j.SetCancel(func() { called = true })

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/jobs/abc", nil)
	rec := httpServe(t, s, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", rec.Code)
	}
	if !called {
		t.Fatalf("cancel func not invoked")
	}
}

func TestHandleJobCancelMissing(t *testing.T) {
	s := newTestServer(t, true)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/jobs/nope", nil)
	rec := httpServe(t, s, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

// SSE tests use httptest.NewServer because http.ResponseRecorder doesn't
// implement http.Flusher, which NewSSEStream requires.
func TestHandleJobLogsTerminalReplaysAndCloses(t *testing.T) {
	s := newTestServer(t, true)
	j := s.jobs.New("done-job", "recipe_pull", "demo")
	j.Update(JobStatusFinished, 1.0, "installed")

	srv := httptest.NewServer(s.router)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/jobs/done-job/logs")
	if err != nil {
		t.Fatalf("GET logs: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body := readWithTimeout(t, resp.Body, 2*time.Second)
	if !strings.Contains(body, "event: status") {
		t.Fatalf("expected initial status event, got:\n%s", body)
	}
	if !strings.Contains(body, "event: end") {
		t.Fatalf("expected end event for terminal job, got:\n%s", body)
	}
}

// readWithTimeout reads from r until it closes or the deadline elapses.
// SSE responses don't have a body length, so a plain io.ReadAll would block.
func readWithTimeout(t *testing.T, r io.Reader, d time.Duration) string {
	t.Helper()
	done := make(chan struct{})
	var sb strings.Builder
	go func() {
		defer close(done)
		br := bufio.NewReader(r)
		for {
			line, err := br.ReadString('\n')
			sb.WriteString(line)
			if err != nil {
				return
			}
		}
	}()
	select {
	case <-done:
	case <-time.After(d):
	}
	return sb.String()
}
