package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// SSEStream wraps an http.ResponseWriter for Server-Sent Events. It writes
// the standard headers, sends a keep-alive comment every heartbeat interval,
// and exposes Send / SendNamed helpers. All methods are safe to call from a
// single goroutine; clients should not share a stream across goroutines.
type SSEStream struct {
	w       http.ResponseWriter
	flusher http.Flusher
	id      int
}

// NewSSEStream initialises SSE response headers. Returns nil if the response
// writer doesn't support flushing
func NewSSEStream(w http.ResponseWriter) *SSEStream {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil
	}
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache, no-transform")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no") // disable nginx buffering if proxied
	flusher.Flush()
	return &SSEStream{w: w, flusher: flusher}
}

// Send emits a default-event message with a JSON-encoded payload.
func (s *SSEStream) Send(payload any) error {
	return s.SendNamed("message", payload)
}

// SendNamed emits a typed SSE event. The frontend listens with
// `es.addEventListener('log', ...)` etc.
func (s *SSEStream) SendNamed(event string, payload any) error {
	s.id++
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(s.w, "id: %d\nevent: %s\ndata: %s\n\n", s.id, event, body)
	if err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

// SendRaw emits a raw text payload as a `log` event without JSON encoding.
// Used for log lines so we don't double-escape ANSI sequences.
func (s *SSEStream) SendRaw(event, line string) error {
	s.id++
	// Spec: each \n in data must be a separate "data:" line; one log line is fine
	// as-is, but defend against embedded newlines by replacing them.
	for i, c := range line {
		if c == '\n' && i != len(line)-1 {
			// rare; bail to JSON path
			return s.SendNamed(event, map[string]string{"line": line})
		}
	}
	_, err := fmt.Fprintf(s.w, "id: %d\nevent: %s\ndata: %s\n\n", s.id, event, line)
	if err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

// Heartbeat sends a comment line that keeps proxies from timing out the
// connection but is invisible to the EventSource client.
func (s *SSEStream) Heartbeat() error {
	_, err := fmt.Fprintf(s.w, ": ping %d\n\n", time.Now().Unix())
	if err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}
