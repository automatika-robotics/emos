package server

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/automatika-robotics/emos-cli/internal/config"
)

// handleHealth is a cheap liveness probe for systemd, the dashboard, and any
// uptime monitor
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"version": config.Version,
		"uptime":  time.Since(s.startedAt).Round(time.Second).String(),
	})
}

// handleInfo describes the install in one structured payload
func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	resp := map[string]any{
		"version":     config.Version,
		"started_at":  s.startedAt,
		"uptime":      time.Since(s.startedAt).Round(time.Second).String(),
		"hostname":    hostname(),
		"platform":    runtime.GOOS + "/" + runtime.GOARCH,
		"home_dir":    config.HomeDir,
		"recipes_dir": config.RecipesDir,
		"logs_dir":    config.LogsDir,
		"installed":   s.cfg != nil,
	}
	if s.cfg != nil {
		resp["mode"] = s.cfg.Mode
		resp["ros_distro"] = s.cfg.ROSDistro
		if s.cfg.ImageTag != "" {
			resp["image_tag"] = s.cfg.ImageTag
		}
		if s.cfg.WorkspacePath != "" {
			resp["workspace_path"] = s.cfg.WorkspacePath
		}
		if s.cfg.PixiProjectDir != "" {
			resp["pixi_project_dir"] = s.cfg.PixiProjectDir
		}
		if s.cfg.LicenseKey != "" {
			resp["license_present"] = true
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleCapabilities exposes feature flags the UI uses to gate buttons
func (s *Server) handleCapabilities(w http.ResponseWriter, r *http.Request) {
	caps := map[string]any{
		"can_run_recipes":    s.cfg != nil,
		"can_pull_recipes":   true, // UI; backend will return offline if needed
		"has_robot_identity": false,
		"docker_available":   commandExists("docker"),
		"pixi_available":     commandExists("pixi"),
	}
	if _, ok := DiscoverRobot(); ok {
		caps["has_robot_identity"] = true
	}
	writeJSON(w, http.StatusOK, caps)
}

// handleConnectivity reports the cached online/offline state
func (s *Server) handleConnectivity(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("refresh") == "1" {
		s.conn.Online(r.Context())
	}
	online, lastChecked, target := s.conn.Snapshot()
	writeJSON(w, http.StatusOK, map[string]any{
		"online":          online,
		"last_checked_at": lastChecked,
		"target":          target,
	})
}

// handleRobot returns identity if discoverable, else 404. The dashboard
// renders a generic device card on 404, not an error.
func (s *Server) handleRobot(w http.ResponseWriter, r *http.Request) {
	info, ok := DiscoverRobot()
	if !ok {
		writeErr(w, http.StatusNotFound, codeNotFound, "robot identity not available")
		return
	}
	writeJSON(w, http.StatusOK, info)
}

// hostname returns the device hostname stripped of any .local suffix.
func hostname() string {
	h, _ := os.Hostname()
	return h
}

func commandExists(name string) bool {
	// cheap PATH probe
	if v := os.Getenv("PATH"); v != "" {
		for _, dir := range filepath.SplitList(v) {
			if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
				return true
			}
		}
	}
	return false
}
