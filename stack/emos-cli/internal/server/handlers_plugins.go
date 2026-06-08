package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/automatika-robotics/emos-cli/internal/api"
	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/plugin"
)

// CatalogPlugin is the wire shape for /plugins/remote.
type CatalogPlugin struct {
	Slug        string   `json:"slug"`
	Name        string   `json:"name"`
	Vendor      string   `json:"vendor"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	EntryPoint  string   `json:"entry_point"`
}

// handlePluginsRemote proxies the support-portal plugin registry. Mirrors
// handleRecipesRemote.
func (s *Server) handlePluginsRemote(w http.ResponseWriter, r *http.Request) {
	if !s.conn.Online(r.Context()) {
		writeErrDetails(w, http.StatusServiceUnavailable, codeOffline,
			"plugin catalog unavailable while offline",
			map[string]any{"target": "support-api.automatikarobotics.com"})
		return
	}
	upstream, err := api.ListPlugins()
	if err != nil {
		s.conn.Invalidate()
		writeErrDetails(w, http.StatusServiceUnavailable, codeUpstreamFailure,
			"plugin catalog upstream error",
			map[string]any{"error": err.Error()})
		return
	}
	out := make([]CatalogPlugin, 0, len(upstream))
	for _, p := range upstream {
		out = append(out, CatalogPlugin{
			Slug:        p.Filename,
			Name:        p.Name,
			Vendor:      p.Vendor,
			Description: p.Description,
			Tags:        p.Tags,
			EntryPoint:  p.EntryPoint,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// ActivePlugin is the wire shape for /plugins/active.
type ActivePlugin struct {
	Slug       string          `json:"slug"`
	EntryPoint string          `json:"entry_point"`
	Repo       string          `json:"repo"`
	Ref        string          `json:"ref,omitempty"`
	Describe   json.RawMessage `json:"describe,omitempty"`
}

// handlePluginActive returns the currently active plugin (plus its cached
// describe() tree).
func (s *Server) handlePluginActive(w http.ResponseWriter, r *http.Request) {
	cfg := config.LoadConfig()
	if cfg == nil || cfg.Plugin == nil {
		writeErr(w, http.StatusNotFound, codeNotFound, "no plugin installed")
		return
	}
	resp := ActivePlugin{
		Slug:       cfg.Plugin.Slug,
		EntryPoint: cfg.Plugin.EntryPoint,
		Repo:       cfg.Plugin.Repo,
		Ref:        cfg.Plugin.Ref,
	}
	if data, ok := plugin.CachedDescribe(); ok {
		resp.Describe = json.RawMessage(data)
	}
	writeJSON(w, http.StatusOK, resp)
}

// handlePluginInstall starts a plugin install job and returns its id. The
// latest build line streams to the UI as the job's progress message via
// /jobs/{id}/logs.
func (s *Server) handlePluginInstall(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if slug == "" {
		writeErr(w, http.StatusBadRequest, codeBadRequest, "missing plugin slug")
		return
	}
	if !s.conn.Online(r.Context()) {
		writeErrDetails(w, http.StatusServiceUnavailable, codeOffline,
			"cannot install plugins while offline",
			map[string]any{"target": "support-api.automatikarobotics.com"})
		return
	}
	cfg := config.LoadConfig()
	if !cfg.IsInstalled() {
		writeErr(w, http.StatusBadRequest, codeBadRequest, "EMOS is not installed")
		return
	}

	id := newID()
	job := s.jobs.New(id, "plugin_install", slug)
	ctx, cancel := context.WithCancel(context.Background())
	job.SetCancel(cancel)
	go func() {
		defer cancel()
		job.Update(JobStatusRunning, 0.05, "resolving plugin")
		entry, err := plugin.Resolve(slug)
		if err != nil {
			job.Update(JobStatusFailed, 0, err.Error())
			s.conn.Invalidate()
			return
		}
		job.Update(JobStatusRunning, 0.15, "cloning and building")
		if err := plugin.Install(cfg, entry, jobLogWriter{job: job}); err != nil {
			if ctx.Err() != nil {
				job.Update(JobStatusFailed, 0, "cancelled")
				return
			}
			job.Update(JobStatusFailed, 0, err.Error())
			return
		}
		job.Update(JobStatusFinished, 1.0, "installed")
	}()
	writeJSON(w, http.StatusAccepted, map[string]string{"job_id": id})
}

// handlePluginRemove removes the active plugin. Idempotent.
func (s *Server) handlePluginRemove(w http.ResponseWriter, r *http.Request) {
	cfg := config.LoadConfig()
	if cfg == nil || cfg.Plugin == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err := plugin.Remove(cfg); err != nil {
		writeErr(w, http.StatusInternalServerError, codeInternal, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// jobLogWriter forwards subprocess output to a job's progress message; the
// dashboard renders job.message live, so the latest non-empty line shows the
// build's current activity.
type jobLogWriter struct{ job *Job }

func (w jobLogWriter) Write(p []byte) (int, error) {
	if line := lastNonEmptyLine(string(p)); line != "" {
		w.job.Update(JobStatusRunning, -1, line)
	}
	return len(p), nil
}

func lastNonEmptyLine(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if t := strings.TrimSpace(lines[i]); t != "" {
			return t
		}
	}
	return ""
}
