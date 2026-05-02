package server

import (
	"encoding/json"
	"net/http"
)

// APIError is the wire shape returned by every JSON error response.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	// Optional details payload, currently only used by 503 offline responses.
	Details map[string]any `json:"details,omitempty"`
}

const (
	codeBadRequest      = "bad_request"
	codeUnauthorized    = "unauthorized"
	codeForbidden       = "forbidden"
	codeNotFound        = "not_found"
	codeConflict        = "conflict"
	codeInternal        = "internal"
	codeOffline         = "offline"
	codeAlreadyRunning  = "already_running"
	codeUpstreamFailure = "upstream_failure"
)

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeErr(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, APIError{Code: code, Message: message})
}

func writeErrDetails(w http.ResponseWriter, status int, code, message string, details map[string]any) {
	writeJSON(w, status, APIError{Code: code, Message: message, Details: details})
}

func decodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}
