package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/automatika-robotics/emos-cli/internal/api"
	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/runner"
)

// LocalRecipe is the per-recipe wire shape returned by /recipes/local
// and (with `Topics`/`SensorTopics` populated) by /recipes/{name}.
type LocalRecipe struct {
	Name         string                 `json:"name"`
	DisplayName  string                 `json:"display_name,omitempty"`
	Description  string                 `json:"description,omitempty"`
	Path         string                 `json:"path"`
	HasRecipePy  bool                   `json:"has_recipe_py"`
	Manifest     map[string]any         `json:"manifest,omitempty"`
	Topics       []runner.ExtractedTopic `json:"topics,omitempty"`
	SensorTopics []runner.ExtractedTopic `json:"sensor_topics,omitempty"`
}

// handleRecipesLocal lists everything in ~/emos/recipes/
func (s *Server) handleRecipesLocal(w http.ResponseWriter, r *http.Request) {
	entries, err := os.ReadDir(config.RecipesDir)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, http.StatusOK, []LocalRecipe{})
			return
		}
		writeErr(w, http.StatusInternalServerError, codeInternal, err.Error())
		return
	}
	out := make([]LocalRecipe, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		out = append(out, readLocalRecipe(e.Name()))
	}
	writeJSON(w, http.StatusOK, out)
}

func readLocalRecipe(name string) LocalRecipe {
	dir := filepath.Join(config.RecipesDir, name)
	rec := LocalRecipe{Name: name, Path: dir}
	if _, err := os.Stat(filepath.Join(dir, "recipe.py")); err == nil {
		rec.HasRecipePy = true
	}
	if data, err := os.ReadFile(filepath.Join(dir, "manifest.json")); err == nil {
		var m map[string]any
		if json.Unmarshal(data, &m) == nil {
			rec.Manifest = m
			if v, ok := m["name"].(string); ok {
				rec.DisplayName = v
			}
			if v, ok := m["description"].(string); ok {
				rec.Description = v
			}
		}
	}
	return rec
}

// CatalogRecipe is the wire shape for /recipes/remote
type CatalogRecipe struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
}

// handleRecipesRemote proxies the Automatika catalog. Returns 503 with
// code:"offline" if the upstream is unreachable so the UI can render its
// dedicated empty state instead of a generic error.
func (s *Server) handleRecipesRemote(w http.ResponseWriter, r *http.Request) {
	if !s.conn.Online(r.Context()) {
		writeErrDetails(w, http.StatusServiceUnavailable, codeOffline,
			"recipe catalog unavailable while offline",
			map[string]any{"target": "support-api.automatikarobotics.com"})
		return
	}
	upstream, err := api.ListRecipes()
	if err != nil {
		s.conn.Invalidate() // network may have just dropped
		writeErrDetails(w, http.StatusServiceUnavailable, codeUpstreamFailure,
			"recipe catalog upstream error",
			map[string]any{"error": err.Error()})
		return
	}
	out := make([]CatalogRecipe, 0, len(upstream))
	for _, r := range upstream {
		out = append(out, CatalogRecipe{Name: r.Filename, DisplayName: r.Name})
	}
	writeJSON(w, http.StatusOK, out)
}

// handleRecipeDetail returns recipe metadata + extracted topics.
func (s *Server) handleRecipeDetail(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if !validRecipeName(name) {
		writeErr(w, http.StatusBadRequest, codeBadRequest, "invalid recipe name")
		return
	}
	dir := filepath.Join(config.RecipesDir, name)
	if _, err := os.Stat(dir); err != nil {
		writeErr(w, http.StatusNotFound, codeNotFound, "recipe not installed")
		return
	}
	rec := readLocalRecipe(name)
	if rec.HasRecipePy {
		if topics, err := runner.ExtractTopics(filepath.Join(dir, "recipe.py")); err == nil {
			rec.Topics = topics
			rec.SensorTopics = runner.SensorTopics(topics)
		}
	}
	writeJSON(w, http.StatusOK, rec)
}

// handleRecipeDelete removes a recipe directory. No-op if it doesn't exist.
func (s *Server) handleRecipeDelete(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if !validRecipeName(name) {
		writeErr(w, http.StatusBadRequest, codeBadRequest, "invalid recipe name")
		return
	}
	dir := filepath.Join(config.RecipesDir, name)
	if err := os.RemoveAll(dir); err != nil {
		writeErr(w, http.StatusInternalServerError, codeInternal, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleRecipePull starts a download job and returns the job id immediately.
// The UI streams progress via /jobs/{id}/logs (SSE).
func (s *Server) handleRecipePull(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if !validRecipeName(name) {
		writeErr(w, http.StatusBadRequest, codeBadRequest, "invalid recipe name")
		return
	}
	if !s.conn.Online(r.Context()) {
		writeErrDetails(w, http.StatusServiceUnavailable, codeOffline,
			"cannot pull recipes while offline",
			map[string]any{"target": "support-api.automatikarobotics.com"})
		return
	}

	id := newID()
	job := s.jobs.New(id, "recipe_pull", name)
	go func() {
		job.Update(JobStatusRunning, 0.05, "starting download")
		if err := os.MkdirAll(config.RecipesDir, 0755); err != nil {
			job.Update(JobStatusFailed, 0, "create recipes dir: "+err.Error())
			s.conn.Invalidate()
			return
		}
		job.Update(JobStatusRunning, 0.20, "downloading recipe archive")
		if err := api.DownloadRecipe(name, config.RecipesDir); err != nil {
			job.Update(JobStatusFailed, 0, err.Error())
			s.conn.Invalidate()
			return
		}
		job.Update(JobStatusFinished, 1.0, "installed")
	}()
	writeJSON(w, http.StatusAccepted, map[string]string{"job_id": id})
}

// validRecipeName guards against path traversal and weird names; matches
// what `emos pull` accepts in practice
func validRecipeName(name string) bool {
	if name == "" || len(name) > 128 {
		return false
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '-', r == '_', r == '.':
			continue
		default:
			return false
		}
	}
	if strings.HasPrefix(name, ".") {
		return false
	}
	return true
}

func newID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "id-fallback"
	}
	return hex.EncodeToString(b[:])
}
