package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

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
	Role        string   `json:"role"` // "robot" | "sensor"
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	EntryPoint  string   `json:"entry_point"`
}

// catalogEntry maps a registry entry to the wire shape. An entry without a
// role is considered a robot plugin.
func catalogEntry(p api.Plugin) CatalogPlugin {
	role := p.Role
	if role == "" {
		role = config.RoleRobot
	}
	return CatalogPlugin{
		Slug:        p.Filename,
		Name:        p.Name,
		Vendor:      p.Vendor,
		Role:        role,
		Description: p.Description,
		Tags:        p.Tags,
		EntryPoint:  p.EntryPoint,
	}
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
		out = append(out, catalogEntry(p))
	}
	writeJSON(w, http.StatusOK, out)
}

// InstalledPlugin is the wire shape of one installed plugin in
// /plugins/installed. The config record plus its cached describe() tree.
type InstalledPlugin struct {
	Slug        string          `json:"slug"`
	EntryPoint  string          `json:"entry_point"`
	Role        string          `json:"role"`
	Repo        string          `json:"repo"`
	Ref         string          `json:"ref,omitempty"`
	ImageURL    string          `json:"image_url,omitempty"`
	Sources     []string        `json:"sources,omitempty"`
	Describe    json.RawMessage `json:"describe,omitempty"`
	InstalledAt time.Time       `json:"installed_at"`
}

// InstalledPlugins is the wire shape for /plugins/installed. The one robot
// plugin (or null) and the sensor plugins mounted alongside it.
type InstalledPlugins struct {
	Robot   *InstalledPlugin  `json:"robot"`
	Sensors []InstalledPlugin `json:"sensors"`
}

func installedView(pi config.PluginInfo) InstalledPlugin {
	role := pi.Role
	if role == "" {
		role = config.RoleRobot
	}
	return InstalledPlugin{
		Slug:        pi.Slug,
		EntryPoint:  pi.EntryPoint,
		Role:        role,
		Repo:        pi.Repo,
		Ref:         pi.Ref,
		ImageURL:    pi.ImageURL,
		Sources:     pi.Sources,
		Describe:    pi.Describe,
		InstalledAt: pi.InstalledAt,
	}
}

// handlePluginsInstalled returns every installed plugin, robot and sensors
// apart.
func (s *Server) handlePluginsInstalled(w http.ResponseWriter, r *http.Request) {
	resp := InstalledPlugins{Sensors: []InstalledPlugin{}}
	if cfg := config.LoadConfig(); cfg != nil {
		if cfg.Plugin != nil {
			robot := installedView(*cfg.Plugin)
			resp.Robot = &robot
		}
		for _, pi := range cfg.SensorPlugins {
			resp.Sensors = append(resp.Sensors, installedView(pi))
		}
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

// handlePluginRemoveSlug removes one installed plugin as a background job.
func (s *Server) handlePluginRemoveSlug(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	cfg := config.LoadConfig()
	if cfg == nil || cfg.FindPlugin(slug) == nil {
		writeErr(w, http.StatusNotFound, codeNotFound, "plugin not installed: "+slug)
		return
	}
	id := newID()
	job := s.jobs.New(id, "plugin_remove", slug)
	go func() {
		job.Update(JobStatusRunning, 0.1, "removing and rebuilding remaining plugins")
		if err := plugin.Remove(cfg, slug, jobLogWriter{job: job}); err != nil {
			job.Update(JobStatusFailed, 0, err.Error())
			return
		}
		job.Update(JobStatusFinished, 1.0, "removed")
	}()
	writeJSON(w, http.StatusAccepted, map[string]string{"job_id": id})
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
