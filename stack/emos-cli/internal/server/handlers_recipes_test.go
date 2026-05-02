package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/automatika-robotics/emos-cli/internal/config"
)

// forceOffline primes the Connectivity cache so Online() returns false
// without hitting the network. The cache TTL is 30s so the fake stays
// effective for the duration of the test.
func forceOffline(s *Server) {
	s.conn = &Connectivity{target: "test", cacheFor: time.Hour}
	s.conn.online = false
	s.conn.lastChecked = time.Now()
}

func TestValidRecipeName(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"vision_follower", true},
		{"vision-follower", true},
		{"vision.follower", true},
		{"VisionFollower01", true},
		{"", false},
		{".hidden", false},
		{"with space", false},
		{"with/slash", false},
		{"with..dotdot", true}, // disallowed via prefix only; embedded dots are OK
		{"../escape", false},
	}
	for _, tc := range cases {
		got := validRecipeName(tc.in)
		if got != tc.want {
			t.Errorf("validRecipeName(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestHandleRecipesLocalEmpty(t *testing.T) {
	s := newTestServer(t, true)
	// RecipesDir doesn't exist yet; handler must return [] not 500.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/recipes/local", nil)
	rec := httpServe(t, s, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var out []LocalRecipe
	jsonBody(t, rec, &out)
	if len(out) != 0 {
		t.Fatalf("expected empty list, got %d", len(out))
	}
}

func TestHandleRecipesLocalListsDirEntries(t *testing.T) {
	s := newTestServer(t, true)

	// Plant a recipe with manifest + recipe.py and a non-recipe file.
	if err := os.MkdirAll(filepath.Join(config.RecipesDir, "demo"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	must(t, os.WriteFile(filepath.Join(config.RecipesDir, "demo", "recipe.py"), []byte("# noop\n"), 0o644))
	must(t, os.WriteFile(filepath.Join(config.RecipesDir, "demo", "manifest.json"),
		[]byte(`{"name":"Demo Recipe","description":"hello"}`), 0o644))
	must(t, os.WriteFile(filepath.Join(config.RecipesDir, "stray.txt"), []byte("ignore"), 0o644))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/recipes/local", nil)
	rec := httpServe(t, s, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var out []LocalRecipe
	jsonBody(t, rec, &out)
	if len(out) != 1 {
		t.Fatalf("len = %d, want 1 (stray.txt should be ignored)", len(out))
	}
	r := out[0]
	if r.Name != "demo" || !r.HasRecipePy || r.DisplayName != "Demo Recipe" || r.Description != "hello" {
		t.Fatalf("recipe metadata wrong: %+v", r)
	}
}

func TestHandleRecipeDetail(t *testing.T) {
	s := newTestServer(t, true)

	// Bad name → 400.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/recipes/..%2Fescape", nil)
	rec := httpServe(t, s, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad-name status = %d, want 400", rec.Code)
	}

	// Missing → 404.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/recipes/missing", nil)
	rec = httpServe(t, s, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing status = %d, want 404", rec.Code)
	}

	// Existing without recipe.py — manifest present.
	must(t, os.MkdirAll(filepath.Join(config.RecipesDir, "demo"), 0o755))
	must(t, os.WriteFile(filepath.Join(config.RecipesDir, "demo", "manifest.json"),
		[]byte(`{"name":"Demo"}`), 0o644))
	req = httptest.NewRequest(http.MethodGet, "/api/v1/recipes/demo", nil)
	rec = httpServe(t, s, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var got LocalRecipe
	jsonBody(t, rec, &got)
	if got.DisplayName != "Demo" {
		t.Fatalf("DisplayName = %q, want Demo", got.DisplayName)
	}
}

func TestHandleRecipeDelete(t *testing.T) {
	s := newTestServer(t, true)

	dir := filepath.Join(config.RecipesDir, "demo")
	must(t, os.MkdirAll(dir, 0o755))
	must(t, os.WriteFile(filepath.Join(dir, "recipe.py"), []byte("# noop"), 0o644))

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/recipes/demo", nil)
	rec := httpServe(t, s, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("recipe dir still exists after delete: %v", err)
	}

	// Idempotent: deleting again is a 204 (RemoveAll on a missing path is OK).
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/recipes/demo", nil)
	rec = httpServe(t, s, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("idempotent delete status = %d, want 204", rec.Code)
	}
}

func TestHandleRecipesRemoteOfflineReturns503(t *testing.T) {
	s := newTestServer(t, true)
	forceOffline(s)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/recipes/remote", nil)
	rec := httpServe(t, s, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	var apiErr APIError
	jsonBody(t, rec, &apiErr)
	if apiErr.Code != codeOffline {
		t.Fatalf("code = %q, want %q", apiErr.Code, codeOffline)
	}
	if apiErr.Details["target"] == nil {
		t.Fatalf("offline response should include target detail: %+v", apiErr)
	}
}

func TestHandleRecipePullOfflineReturns503(t *testing.T) {
	s := newTestServer(t, true)
	forceOffline(s)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/recipes/demo/pull", nil)
	rec := httpServe(t, s, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	var apiErr APIError
	jsonBody(t, rec, &apiErr)
	if apiErr.Code != codeOffline {
		t.Fatalf("code = %q, want %q", apiErr.Code, codeOffline)
	}
}

func TestHandleRecipePullBadName(t *testing.T) {
	s := newTestServer(t, true)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/recipes/..%2Fescape/pull", nil)
	rec := httpServe(t, s, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
}
