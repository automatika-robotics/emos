package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORSAllowOriginAcceptsLAN(t *testing.T) {
	s := newTestServer(t, false)
	s.opts.DeviceName = "epic-otter"

	cases := []string{
		"http://localhost:8765",
		"https://localhost",
		"http://127.0.0.1:8765",
		"http://[::1]:8765",
		"http://emos.local:8765",
		"http://epic-otter.local:8765",
		"http://epic-otter.local",
	}
	for _, origin := range cases {
		req, _ := http.NewRequest(http.MethodGet, "/", nil)
		if !s.corsAllowOrigin(req, origin) {
			t.Errorf("origin %q should be allowed", origin)
		}
	}
}

func TestCORSAllowOriginRejectsArbitrary(t *testing.T) {
	s := newTestServer(t, false)
	s.opts.DeviceName = "epic-otter"

	cases := []string{
		"http://evil.example",
		"https://printer.local:631",      // sibling LAN admin UI
		"http://192.168.1.50:3000",       // arbitrary LAN dev server
		"http://attacker.localhost.evil", // suffix trickery
		"http://lookalike-otter.local",   // not the actual device name
		"",
	}
	for _, origin := range cases {
		req, _ := http.NewRequest(http.MethodGet, "/", nil)
		if s.corsAllowOrigin(req, origin) {
			t.Errorf("origin %q should be blocked", origin)
		}
	}
}

func TestCORSAllowOriginEMOSDevWildcards(t *testing.T) {
	t.Setenv("EMOS_DEV", "1")

	s := newTestServer(t, false)
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	if !s.corsAllowOrigin(req, "http://anything.example") {
		t.Fatalf("EMOS_DEV=1 should accept arbitrary origins")
	}
}

// End-to-end: the chi cors middleware should drop the
// Access-Control-Allow-Origin response header for blocked origins, which
// is the wire-level signal browsers act on.
func TestCORSPreflightFromBlockedOriginIsBlocked(t *testing.T) {
	s := newTestServer(t, false)

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/runs", nil)
	req.Header.Set("Origin", "http://evil.example")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec := httpServe(t, s, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("blocked origin received Access-Control-Allow-Origin = %q", got)
	}
}
