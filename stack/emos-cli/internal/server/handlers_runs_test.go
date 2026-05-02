package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/automatika-robotics/emos-cli/internal/config"
)

func TestValidRMW(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"rmw_fastrtps_cpp", true},
		{"rmw_cyclonedds_cpp", true},
		{"rmw_zenoh_cpp", true},
		{"", false},
		{"rmw_other", false},
		{"FastRTPS", false},
	}
	for _, tc := range cases {
		if got := validRMW(tc.in); got != tc.want {
			t.Errorf("validRMW(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestHandleRunsListEmpty(t *testing.T) {
	s := newTestServer(t, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/runs", nil)
	rec := httpServe(t, s, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var runs []Run
	jsonBody(t, rec, &runs)
	if len(runs) != 0 {
		t.Fatalf("List = %d entries, want 0", len(runs))
	}
}

func TestHandleRunGetMissing(t *testing.T) {
	s := newTestServer(t, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/runs/nope", nil)
	rec := httpServe(t, s, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestHandleRunsStartBadJSON(t *testing.T) {
	s := newTestServer(t, true)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/runs", nil)
	req.Header.Set("Content-Type", "application/json")
	rec := httpServe(t, s, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHandleRunsStartInvalidRecipe(t *testing.T) {
	s := newTestServer(t, true)
	req := jsonRequest(t, http.MethodPost, "/api/v1/runs", map[string]string{
		"recipe": "../escape",
	})
	rec := httpServe(t, s, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHandleRunsStartInvalidRMW(t *testing.T) {
	s := newTestServer(t, true)
	req := jsonRequest(t, http.MethodPost, "/api/v1/runs", map[string]string{
		"recipe": "demo",
		"rmw":    "rmw_unknown",
	})
	rec := httpServe(t, s, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHandleRunsStartNoInstall(t *testing.T) {
	s := newTestServer(t, true)
	// s.cfg is nil by default in newTestServer.
	req := jsonRequest(t, http.MethodPost, "/api/v1/runs", map[string]string{
		"recipe": "demo",
	})
	rec := httpServe(t, s, req)
	// FailedDependency is the dedicated status for "you need to install first".
	if rec.Code != http.StatusFailedDependency {
		t.Fatalf("status = %d, want 424 (FailedDependency)", rec.Code)
	}
}

func TestHandleRunsStartRecipeNotInstalled(t *testing.T) {
	s := newTestServer(t, true)
	s.cfg = &config.EMOSConfig{Mode: config.ModeNative, ROSDistro: "jazzy"}

	req := jsonRequest(t, http.MethodPost, "/api/v1/runs", map[string]string{
		"recipe": "missing",
	})
	rec := httpServe(t, s, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestHandleRunsStartConflictWhenAnotherRunning(t *testing.T) {
	s := newTestServer(t, true)
	s.cfg = &config.EMOSConfig{Mode: config.ModeNative, ROSDistro: "jazzy"}

	// Plant a recipe so the validation passes through to TryLock.
	dir := filepath.Join(config.RecipesDir, "demo")
	must(t, os.MkdirAll(dir, 0o755))
	must(t, os.WriteFile(filepath.Join(dir, "recipe.py"), []byte("# noop"), 0o644))

	// Pre-occupy the runtime slot.
	occupier := &Run{ID: "earlier", Status: RunStatusPreparing}
	if err := s.runtime.TryLock(occupier); err != nil {
		t.Fatalf("TryLock occupier: %v", err)
	}

	req := jsonRequest(t, http.MethodPost, "/api/v1/runs", map[string]string{
		"recipe": "demo",
	})
	rec := httpServe(t, s, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
	var apiErr APIError
	jsonBody(t, rec, &apiErr)
	if apiErr.Code != codeAlreadyRunning {
		t.Fatalf("code = %q, want %q", apiErr.Code, codeAlreadyRunning)
	}
}

func TestHandleRunCancelMissing(t *testing.T) {
	s := newTestServer(t, true)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/runs/nope", nil)
	rec := httpServe(t, s, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestHandleRunCancelPreparing(t *testing.T) {
	s := newTestServer(t, true)
	r := &Run{ID: "abc", Status: RunStatusPreparing}
	if err := s.runtime.TryLock(r); err != nil {
		t.Fatalf("TryLock: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/runs/abc", nil)
	rec := httpServe(t, s, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", rec.Code)
	}
	if r.Status != RunStatusCanceled {
		t.Fatalf("Status = %q, want canceled", r.Status)
	}
}
