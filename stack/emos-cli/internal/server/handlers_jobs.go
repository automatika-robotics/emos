package server

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

func (s *Server) handleJobsList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.jobs.List())
}

func (s *Server) handleJobGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	j := s.jobs.Get(id)
	if j == nil {
		writeErr(w, http.StatusNotFound, codeNotFound, "job not found")
		return
	}
	snap := j.Snapshot()
	writeJSON(w, http.StatusOK, snap)
}

// handleJobLogs streams JobEvent updates via SSE. Closes when the job finishes.
func (s *Server) handleJobLogs(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	j := s.jobs.Get(id)
	if j == nil {
		writeErr(w, http.StatusNotFound, codeNotFound, "job not found")
		return
	}
	stream := NewSSEStream(w)
	if stream == nil {
		writeErr(w, http.StatusInternalServerError, codeInternal, "streaming unsupported")
		return
	}

	// Replay the current snapshot as the first event so a client that
	// connects after the job already finished gets a final state.
	_ = stream.SendNamed("status", j.Snapshot())

	if j.Status != JobStatusRunning {
		_ = stream.SendNamed("end", j.Snapshot())
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	bus := j.Subscribe()
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			if err := stream.Heartbeat(); err != nil {
				return
			}
		case evt, ok := <-bus:
			if !ok {
				_ = stream.SendNamed("end", j.Snapshot())
				return
			}
			if err := stream.SendNamed("status", evt); err != nil {
				return
			}
			if evt.Status != JobStatusRunning {
				_ = stream.SendNamed("end", j.Snapshot())
				return
			}
		}
	}
}
