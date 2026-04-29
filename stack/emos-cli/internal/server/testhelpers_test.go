package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// TestMain lowers bcrypt's pairing-code cost from the production value
// to bcrypt.MinCost so the test suite (especially the 50-way concurrent
// Pair test under -race) finishes in seconds, not minutes. The
// production cost is still what ships in the binary.
func TestMain(m *testing.M) {
	pairingHashCost = bcrypt.MinCost
	os.Exit(m.Run())
}

// newTestServer returns a Server wired up enough that buildRouter()'s
// handlers can be exercised end-to-end. It uses a tmp-dir-backed config
// (via withTempConfig) so each test starts from a clean ~/.config/emos.
//
// `bypassAuth` short-circuits AuthRequired so tests don't have to mint a
// token before hitting an authenticated endpoint.
func newTestServer(t *testing.T, bypassAuth bool) *Server {
	t.Helper()
	withTempConfig(t)

	auth, err := NewAuth(bypassAuth)
	if err != nil {
		t.Fatalf("NewAuth: %v", err)
	}
	s := &Server{
		opts:       Options{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))},
		log:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		auth:       auth,
		conn:       NewConnectivity(),
		runtime:    NewRuntime(),
		jobs:       NewJobs(),
		sseTickets: newSSETicketStore(),
		startedAt:  time.Now(),
	}
	s.router = s.buildRouter()
	return s
}

// httpServe serves a single request through the test server's router and
// returns the recorded response.
func httpServe(t *testing.T, s *Server, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)
	return rec
}

// jsonBody decodes a response body into the supplied struct.
func jsonBody(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
		t.Fatalf("decode body: %v\n--- body ---\n%s", err, rec.Body.String())
	}
}

// jsonRequest builds a POST request with a JSON-encoded body.
func jsonRequest(t *testing.T, method, path string, body any) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	return req
}
