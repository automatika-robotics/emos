package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSSETicketIssueConsumeOnce(t *testing.T) {
	st := newSSETicketStore()
	tkt, exp, err := st.Issue()
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if tkt == "" {
		t.Fatalf("ticket empty")
	}
	if !exp.After(time.Now()) {
		t.Fatalf("expiry not in the future: %v", exp)
	}
	if !st.Consume(tkt) {
		t.Fatalf("first Consume should succeed")
	}
	// Second Consume must fail — single use is the whole point.
	if st.Consume(tkt) {
		t.Fatalf("second Consume should fail")
	}
}

func TestSSETicketEmptyOrUnknownRejected(t *testing.T) {
	st := newSSETicketStore()
	if st.Consume("") {
		t.Fatalf("empty ticket should not consume")
	}
	if st.Consume("not-a-real-ticket") {
		t.Fatalf("unknown ticket should not consume")
	}
}

func TestSSETicketExpired(t *testing.T) {
	st := newSSETicketStore()
	st.ttl = 1 * time.Millisecond
	tkt, _, _ := st.Issue()
	time.Sleep(10 * time.Millisecond)
	if st.Consume(tkt) {
		t.Fatalf("expired ticket must not consume")
	}
}

func TestSSETicketMiddlewareGatesSSE(t *testing.T) {
	// Real-mode server (no bypass): SSE endpoint must reject without a
	// ticket, accept with a freshly-minted one, then reject again on
	// reuse of the same ticket.
	s := newTestServer(t, false)
	tok, _, err := s.auth.Pair(s.auth.FreshPairingCode(), "")
	if err != nil {
		t.Fatalf("Pair: %v", err)
	}

	// Plant a finished job so SSE returns immediately.
	j := s.jobs.New("done", "recipe_pull", "demo")
	j.Update(JobStatusFinished, 1.0, "installed")

	srv := httptest.NewServer(s.router)
	defer srv.Close()

	// 1. No ticket → 401.
	resp, err := http.Get(srv.URL + "/api/v1/jobs/done/logs")
	if err != nil {
		t.Fatalf("GET unticketed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unticketed status = %d, want 401", resp.StatusCode)
	}

	// 2. Mint a ticket via the bearer-protected endpoint.
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/auth/sse-ticket", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST ticket: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("ticket-issue status = %d, want 200", resp.StatusCode)
	}
	var tr sseTicketResponse
	jsonReadAll(t, resp, &tr)
	if tr.Ticket == "" {
		t.Fatalf("issued ticket empty")
	}

	// 3. First use of that ticket → 200.
	resp, err = http.Get(srv.URL + "/api/v1/jobs/done/logs?ticket=" + tr.Ticket)
	if err != nil {
		t.Fatalf("GET with ticket: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("ticketed status = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()

	// 4. Replay attempt → 401.
	resp, err = http.Get(srv.URL + "/api/v1/jobs/done/logs?ticket=" + tr.Ticket)
	if err != nil {
		t.Fatalf("GET replay: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("replay status = %d, want 401", resp.StatusCode)
	}
}

// jsonReadAll is a tiny helper for httptest.NewServer responses, since
// jsonBody works against the in-memory recorder, not a live response.
func jsonReadAll(t *testing.T, resp *http.Response, v any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("decode: %v", err)
	}
}
