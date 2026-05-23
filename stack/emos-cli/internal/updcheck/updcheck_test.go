package updcheck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/automatika-robotics/emos-cli/internal/config"
)

// withFakeReleases redirects config.ReleasesURL() at a httptest.Server for
// the duration of the test by overriding the GitHub host through
// http.DefaultTransport... actually that's hard. Instead we monkey-patch
// the package-level httpClient with one that rewrites the request URL via
// a custom RoundTripper. This avoids depending on config internals.
func withFakeReleases(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	origClient := httpClient
	t.Cleanup(func() { httpClient = origClient })

	target, _ := url.Parse(srv.URL)
	httpClient = &http.Client{
		Timeout: 5 * time.Second,
		Transport: rewriteRT{target: target},
	}
	return srv
}

// rewriteRT routes every request to the test server, preserving path
// and query so the handler can still gate on them if needed.
type rewriteRT struct{ target *url.URL }

func (r rewriteRT) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.URL.Scheme = r.target.Scheme
	req.URL.Host = r.target.Host
	req.Host = r.target.Host
	return http.DefaultTransport.RoundTrip(req)
}

// withTmpConfigDir redirects config.ConfigDir at a fresh tempdir for the
// duration of the test.
func withTmpConfigDir(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	orig := config.ConfigDir
	config.ConfigDir = tmp
	t.Cleanup(func() { config.ConfigDir = orig })
	return tmp
}

func TestLatest_HappyPath(t *testing.T) {
	withFakeReleases(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v0.7.0"}`))
	})
	got, err := Latest(context.Background())
	if err != nil {
		t.Fatalf("Latest err = %v", err)
	}
	if got != "0.7.0" {
		t.Errorf("Latest = %q, want %q (no leading v)", got, "0.7.0")
	}
}

func TestLatest_404(t *testing.T) {
	withFakeReleases(t, func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	if _, err := Latest(context.Background()); err == nil {
		t.Fatal("expected an error for a 404 response, got nil")
	}
}

func TestLatest_MalformedJSON(t *testing.T) {
	withFakeReleases(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{ not json `))
	})
	if _, err := Latest(context.Background()); err == nil {
		t.Fatal("expected an error for malformed JSON, got nil")
	}
}

func TestLatest_EmptyTagName(t *testing.T) {
	withFakeReleases(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":""}`))
	})
	if _, err := Latest(context.Background()); err == nil {
		t.Fatal("expected an error for empty tag_name, got nil")
	}
}

func TestIsNewer_Table(t *testing.T) {
	cases := []struct {
		current, latest string
		want            bool
	}{
		{"0.6.1", "0.7.0", true},
		{"0.7.0", "0.6.1", false},
		{"0.7.0", "0.7.0", false},
		{"1.2.9", "1.2.10", true}, // numeric, not lexicographic
		{"1.2.10", "1.2.9", false},
		{"0.0.0", "0.0.1", true},
		// dev / empty inputs -> never newer
		{"dev", "9.9.9", false},
		{"", "9.9.9", false},
		{"0.6.1", "", false},
		// malformed -> false (no panic)
		{"abc", "0.7.0", false},
		{"0.6.1", "x.y.z", false},
		{"0.6", "0.7.0", false}, // missing patch
		// leading v on either side is tolerated
		{"v0.6.1", "v0.7.0", true},
	}
	for _, tc := range cases {
		got := IsNewer(tc.current, tc.latest)
		if got != tc.want {
			t.Errorf("IsNewer(%q, %q) = %v, want %v", tc.current, tc.latest, got, tc.want)
		}
	}
}

func TestCachedLatest_RoundTrip(t *testing.T) {
	withTmpConfigDir(t)
	if err := writeCache("0.7.0", time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("writeCache: %v", err)
	}
	latest, checkedAt, ok := CachedLatest()
	if !ok {
		t.Fatal("CachedLatest ok=false after writeCache")
	}
	if latest != "0.7.0" {
		t.Errorf("latest = %q, want 0.7.0", latest)
	}
	if !checkedAt.Equal(time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC)) {
		t.Errorf("checkedAt = %v, want 2026-05-21T12:00:00Z", checkedAt)
	}
}

func TestCachedLatest_MissingFile(t *testing.T) {
	withTmpConfigDir(t)
	_, _, ok := CachedLatest()
	if ok {
		t.Fatal("CachedLatest ok=true on empty config dir")
	}
}

func TestCachedLatest_CorruptedJSON(t *testing.T) {
	tmp := withTmpConfigDir(t)
	if err := os.WriteFile(filepath.Join(tmp, cacheFileName), []byte(`{not json`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, ok := CachedLatest()
	if ok {
		t.Fatal("CachedLatest ok=true on corrupted JSON")
	}
}

func TestRefreshIfStale_FreshCacheNoFetch(t *testing.T) {
	withTmpConfigDir(t)
	// Pre-seed a fresh cache; the fake server would fail the test if hit.
	if err := writeCache("0.7.0", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	withFakeReleases(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("Latest was called even though the cache is fresh")
	})

	latest, ok := RefreshIfStale(6 * time.Hour)
	if !ok || latest != "0.7.0" {
		t.Errorf("RefreshIfStale = (%q, %v), want (0.7.0, true)", latest, ok)
	}
	// Give any erroneous goroutine a moment to fire.
	time.Sleep(50 * time.Millisecond)
}

func TestRefreshIfStale_StaleCacheRefreshesInBackground(t *testing.T) {
	tmp := withTmpConfigDir(t)
	// Pre-seed a stale cache.
	stale := time.Now().Add(-24 * time.Hour).UTC()
	if err := writeCache("0.6.0", stale); err != nil {
		t.Fatal(err)
	}

	fetched := make(chan struct{})
	withFakeReleases(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v0.7.0"}`))
		close(fetched)
	})

	latest, ok := RefreshIfStale(6 * time.Hour)
	// Returns the stale value immediately; background goroutine refreshes.
	if !ok || latest != "0.6.0" {
		t.Errorf("RefreshIfStale = (%q, %v), want (0.6.0, true) on first call", latest, ok)
	}

	// Wait for the background fetch to complete.
	select {
	case <-fetched:
	case <-time.After(2 * time.Second):
		t.Fatal("background refresh never hit the fake server")
	}

	// Give the goroutine a moment to write the cache file after the
	// HTTP handler returned.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		latest, _, _ := CachedLatest()
		if latest == "0.7.0" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	updated, _, ok := CachedLatest()
	if !ok || updated != "0.7.0" {
		// One more diagnostic: dump the cache file so the failure is
		// debuggable.
		data, _ := os.ReadFile(filepath.Join(tmp, cacheFileName))
		t.Fatalf("cache not refreshed: latest=%q ok=%v file=%q", updated, ok, string(data))
	}
}

func TestRefreshIfStale_CacheMissBlocksAndPopulates(t *testing.T) {
	// First-ever invocation: no cache file on disk. The function MUST
	// block-and-fetch so a CLI one-shot has something to show. The
	// goroutine-only path would never write the cache before the CLI
	// process exits.
	withTmpConfigDir(t)
	withFakeReleases(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v0.7.0"}`))
	})

	latest, ok := RefreshIfStale(6 * time.Hour)
	if !ok || latest != "0.7.0" {
		t.Fatalf("RefreshIfStale on cache miss = (%q, %v), want (0.7.0, true)", latest, ok)
	}
	// And the cache file is on disk: subsequent invocations skip the fetch.
	if cached, _, ok := CachedLatest(); !ok || cached != "0.7.0" {
		t.Errorf("cache not persisted after blocking fetch: cached=%q ok=%v", cached, ok)
	}
}

func TestRefreshIfStale_CacheMissFetchFailureReturnsUnknown(t *testing.T) {
	// Cache miss + offline / 5xx -> ok=false, no panic, no cache write.
	withTmpConfigDir(t)
	withFakeReleases(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	// Shrink the first-fetch timeout so the test stays snappy.
	origTimeout := firstFetchTimeout
	firstFetchTimeout = 500 * time.Millisecond
	t.Cleanup(func() { firstFetchTimeout = origTimeout })

	_, ok := RefreshIfStale(6 * time.Hour)
	if ok {
		t.Errorf("RefreshIfStale on cache miss + 5xx returned ok=true; want false")
	}
	if _, _, cachedOK := CachedLatest(); cachedOK {
		t.Errorf("failed first-fetch wrote a cache file; should leave disk untouched")
	}
}

func TestRefreshIfStale_FetchFailureKeepsCache(t *testing.T) {
	tmp := withTmpConfigDir(t)
	stale := time.Now().Add(-24 * time.Hour).UTC()
	if err := writeCache("0.6.0", stale); err != nil {
		t.Fatal(err)
	}
	withFakeReleases(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})

	latest, ok := RefreshIfStale(6 * time.Hour)
	if !ok || latest != "0.6.0" {
		t.Errorf("RefreshIfStale = (%q, %v), want stale (0.6.0, true)", latest, ok)
	}

	// Even after the failed refresh settles, the cache stays at 0.6.0.
	time.Sleep(200 * time.Millisecond)
	updated, _, ok := CachedLatest()
	if !ok || updated != "0.6.0" {
		data, _ := os.ReadFile(filepath.Join(tmp, cacheFileName))
		t.Fatalf("failed refresh clobbered the cache: latest=%q ok=%v file=%q", updated, ok, string(data))
	}
}
