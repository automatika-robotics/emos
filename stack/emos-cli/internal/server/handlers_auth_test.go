package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHandleAuthPairValidCode(t *testing.T) {
	s := newTestServer(t, false)
	code := s.auth.FreshPairingCode()
	if code == "" {
		t.Fatalf("expected fresh pairing code")
	}

	req := jsonRequest(t, http.MethodPost, "/api/v1/auth/pair", map[string]string{
		"code":  code,
		"label": "phone",
	})
	rec := httpServe(t, s, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var resp pairResponse
	jsonBody(t, rec, &resp)
	if resp.Token == "" {
		t.Fatalf("Token = empty")
	}
	if !resp.ExpiresAt.After(time.Now()) {
		t.Fatalf("ExpiresAt %v not in the future", resp.ExpiresAt)
	}
}

func TestHandleAuthPairInvalidCode(t *testing.T) {
	s := newTestServer(t, false)
	_ = s.auth.FreshPairingCode()

	req := jsonRequest(t, http.MethodPost, "/api/v1/auth/pair", map[string]string{
		"code": "000000",
	})
	rec := httpServe(t, s, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body=%s", rec.Code, rec.Body)
	}
	var apiErr APIError
	jsonBody(t, rec, &apiErr)
	if apiErr.Code != codeUnauthorized {
		t.Fatalf("error code = %q, want %q", apiErr.Code, codeUnauthorized)
	}
}

func TestHandleAuthPairBadJSON(t *testing.T) {
	s := newTestServer(t, false)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/pair", nil)
	req.Header.Set("Content-Type", "application/json")
	rec := httpServe(t, s, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHandleAuthMeBearer(t *testing.T) {
	s := newTestServer(t, false)
	tok, _, err := s.auth.Pair(s.auth.FreshPairingCode(), "")
	if err != nil {
		t.Fatalf("Pair: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httpServe(t, s, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]any
	jsonBody(t, rec, &body)
	if body["authenticated"] != true {
		t.Fatalf("authenticated = %v, want true", body["authenticated"])
	}
}

func TestHandleAuthMeRejectsQueryToken(t *testing.T) {
	// Tokens in URLs leak through reverse-proxy access logs and Referer
	// headers — bearerToken() must NOT fall back to ?token=. SSE clients
	// use the dedicated single-use ticket flow instead.
	s := newTestServer(t, false)
	tok, _, _ := s.auth.Pair(s.auth.FreshPairingCode(), "")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me?token="+tok, nil)
	rec := httpServe(t, s, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (query-string token must be rejected)", rec.Code)
	}
}

func TestHandleAuthMeMissingToken(t *testing.T) {
	s := newTestServer(t, false)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	rec := httpServe(t, s, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestAuthRequiredBlocksUnauthenticated(t *testing.T) {
	s := newTestServer(t, false)

	// /robot is in the authenticated group; without a token it must 401.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/robot", nil)
	rec := httpServe(t, s, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestAuthBypassMode(t *testing.T) {
	// In --no-auth mode, the protected surface is open. Useful for dev but a
	// regression in default behaviour would be a security incident.
	s := newTestServer(t, true)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/robot", nil)
	rec := httpServe(t, s, req)
	// 404 (no robot identity) is the correct authenticated-but-no-data
	// answer; 401 would mean bypass didn't take effect.
	if rec.Code == http.StatusUnauthorized {
		t.Fatalf("auth bypass not effective: got 401")
	}
}
