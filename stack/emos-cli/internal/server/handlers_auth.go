package server

import (
	"net/http"
	"time"
)

type pairBody struct {
	Code  string `json:"code"`
	Label string `json:"label,omitempty"`
}

type pairResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (s *Server) handleAuthPair(w http.ResponseWriter, r *http.Request) {
	var body pairBody
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, codeBadRequest, err.Error())
		return
	}
	tok, exp, err := s.auth.Pair(body.Code, body.Label)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, codeUnauthorized, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, pairResponse{Token: tok, ExpiresAt: exp})
}

// handleAuthMe tells a holder of the bearer token that it's still valid
func (s *Server) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	tok := bearerToken(r)
	if err := s.auth.Verify(tok); err != nil {
		writeErr(w, http.StatusUnauthorized, codeUnauthorized, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": true})
}

type sseTicketResponse struct {
	Ticket    string    `json:"ticket"`
	ExpiresAt time.Time `json:"expires_at"`
}

// handleAuthSSETicket mints a single-use, short-lived ticket for the SSE
// endpoints. The caller must hold a valid bearer token (this route lives
// under the AuthRequired group)
func (s *Server) handleAuthSSETicket(w http.ResponseWriter, r *http.Request) {
	ticket, exp, err := s.sseTickets.Issue()
	if err != nil {
		writeErr(w, http.StatusServiceUnavailable, codeInternal, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, sseTicketResponse{Ticket: ticket, ExpiresAt: exp})
}
