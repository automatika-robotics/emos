package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/automatika-robotics/emos-cli/internal/config"
)

// adminTestSetup redirects config storage at a tempdir and returns a fresh
// Auth + the writable config object. The caller can mutate the on-disk hash
// to simulate `emos config rotate-pairing` and then exercise reload paths.
func adminTestSetup(t *testing.T) *Auth {
	t.Helper()

	tmp := t.TempDir()
	origConfigDir := config.ConfigDir
	origConfigFile := config.ConfigFile
	origLicenseFile := config.LicenseFile
	t.Cleanup(func() {
		config.ConfigDir = origConfigDir
		config.ConfigFile = origConfigFile
		config.LicenseFile = origLicenseFile
	})
	config.ConfigDir = tmp
	config.ConfigFile = filepath.Join(tmp, "config.json")
	config.LicenseFile = filepath.Join(tmp, "license.key")

	// Lower bcrypt cost so RegeneratePairingCode is fast under -race.
	origCost := pairingHashCost
	pairingHashCost = bcryptMinCost(t)
	t.Cleanup(func() { pairingHashCost = origCost })

	auth, err := NewAuth(false)
	if err != nil {
		t.Fatalf("NewAuth: %v", err)
	}
	return auth
}

// bcryptMinCost returns 4 (the bcrypt minimum) so tests don't burn CPU on
// real-cost hashes. Wrapped in a helper for clarity at call sites.
func bcryptMinCost(t *testing.T) int { t.Helper(); return 4 }

func TestHandleAdminReloadAuth_RejectsNonLoopback(t *testing.T) {
	auth := adminTestSetup(t)
	s := &Server{auth: auth}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/reload-auth", nil)
	req.RemoteAddr = "192.168.1.42:54321"
	rr := httptest.NewRecorder()

	s.handleAdminReloadAuth(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 Forbidden", rr.Code)
	}
}

func TestHandleAdminReloadAuth_LoopbackReloadsHash(t *testing.T) {
	auth := adminTestSetup(t)

	originalHash := auth.state.PairingCodeHash
	if originalHash == "" {
		t.Fatalf("setup didn't initialise pairing hash")
	}

	// Mint a new code via the same path the rotate-pairing CLI uses; this
	// updates the on-disk file via persistLocked, but ALSO updates the
	// in-memory state on this same Auth object. Build a *separate* Auth
	// to simulate the daemon (in-memory state pre-rotation), then call
	// Reload on it and confirm it picks up the new on-disk hash.
	if _, err := auth.RegeneratePairingCode(); err != nil {
		t.Fatalf("RegeneratePairingCode: %v", err)
	}
	rotatedHash := auth.state.PairingCodeHash
	if rotatedHash == originalHash {
		t.Fatalf("RegeneratePairingCode did not change the hash")
	}

	// Build a second Auth that reflects the daemon's state BEFORE the
	// rotate. We fake this by stuffing the original hash back into a
	// fresh Auth struct whose disk-bound config now has the rotated hash.
	daemonAuth := &Auth{state: config.AuthState{PairingCodeHash: originalHash}}
	s := &Server{auth: daemonAuth}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/reload-auth", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	rr := httptest.NewRecorder()

	s.handleAdminReloadAuth(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 No Content; body=%q", rr.Code, rr.Body.String())
	}
	if daemonAuth.state.PairingCodeHash != rotatedHash {
		t.Fatalf("hash after reload = %q, want %q", daemonAuth.state.PairingCodeHash, rotatedHash)
	}
}

func TestAuthReload_PreservesTokens(t *testing.T) {
	// Reload is called from `rotate-pairing`'s hot-reload path. It must
	// refresh the pairing-code hash without disturbing TokenKey or Tokens
	// -- otherwise every paired browser would silently 401 after a rotate,
	// undoing the whole point of "existing browsers stay paired".
	auth := adminTestSetup(t)
	auth.state.Tokens = []config.AuthToken{{Hash: "deadbeef", Label: "phone"}}
	originalKey := append([]byte(nil), auth.state.TokenKey...)

	// Rotate via the disk-writing path.
	if _, err := auth.RegeneratePairingCode(); err != nil {
		t.Fatal(err)
	}

	// Strip in-memory tokens to simulate "they were lost on a restart"
	// and confirm Reload doesn't restore them either -- only the pairing
	// hash field is in scope.
	preReloadHash := auth.state.PairingCodeHash
	auth.state.PairingCodeHash = "" // pretend the daemon didn't have it

	if err := auth.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if auth.state.PairingCodeHash != preReloadHash {
		t.Errorf("pairing hash not refreshed: got %q, want %q",
			auth.state.PairingCodeHash, preReloadHash)
	}
	if string(auth.state.TokenKey) != string(originalKey) {
		t.Errorf("Reload changed TokenKey")
	}
	if len(auth.state.Tokens) != 1 || auth.state.Tokens[0].Label != "phone" {
		t.Errorf("Reload disturbed Tokens: %+v", auth.state.Tokens)
	}
}

func TestIsLoopbackRequest(t *testing.T) {
	cases := []struct {
		remote string
		want   bool
	}{
		{"127.0.0.1:54321", true},
		{"127.0.0.42:54321", true}, // anywhere in 127.0.0.0/8 is loopback
		{"[::1]:54321", true},
		{"192.168.1.42:80", false},
		{"10.0.0.1:443", false},
		{"203.0.113.42:8080", false},
		{"", false},
		{"not-a-host", false},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodPost, "/x", nil)
		req.RemoteAddr = tc.remote
		if got := isLoopbackRequest(req); got != tc.want {
			t.Errorf("isLoopbackRequest(%q) = %v, want %v", tc.remote, got, tc.want)
		}
	}
}

func TestIsLoopbackRequest_RejectsSpoofedForwardedHeaders(t *testing.T) {
	// Regression for the X-Forwarded-For bypass. chi's middleware.RealIP
	// (registered globally) rewrites r.RemoteAddr from forwarded headers
	// when present; without a reverse proxy in front of us, those headers
	// are necessarily attacker-supplied. Even if RemoteAddr LOOKS like
	// loopback (because RealIP already rewrote it), the presence of any
	// forwarded header means the request didn't actually originate on
	// this host.
	for _, h := range []string{
		"X-Forwarded-For",
		"X-Real-IP",
		"True-Client-IP",
		"X-Cluster-Client-IP",
		"Forwarded",
	} {
		req := httptest.NewRequest(http.MethodPost, "/x", nil)
		req.RemoteAddr = "127.0.0.1:54321" // exactly what RealIP would set
		req.Header.Set(h, "127.0.0.1")
		if isLoopbackRequest(req) {
			t.Errorf("isLoopbackRequest accepted request with %s header set", h)
		}
	}
}
