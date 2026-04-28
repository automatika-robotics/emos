package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/automatika-robotics/emos-cli/internal/config"
)

func TestHandleHealth(t *testing.T) {
	s := newTestServer(t, false)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httpServe(t, s, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var body map[string]any
	jsonBody(t, rec, &body)
	if body["status"] != "ok" {
		t.Fatalf("status field = %v, want ok", body["status"])
	}
	if body["version"] == nil {
		t.Fatalf("version missing in /health body: %+v", body)
	}
}

func TestHandleInfoNoInstall(t *testing.T) {
	s := newTestServer(t, false)
	s.opts.DeviceName = "epic-otter"

	req := httptest.NewRequest(http.MethodGet, "/api/v1/info", nil)
	rec := httpServe(t, s, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]any
	jsonBody(t, rec, &body)
	if body["name"] != "epic-otter" {
		t.Fatalf("name = %v, want epic-otter", body["name"])
	}
	if body["installed"] != false {
		t.Fatalf("installed = %v, want false (no config persisted)", body["installed"])
	}
	// Mode-related fields must be absent when no config is installed; the
	// dashboard relies on `installed:false` to render the install prompt.
	if _, ok := body["mode"]; ok {
		t.Fatalf("mode should be absent when not installed: %+v", body)
	}
}

func TestHandleInfoWithInstall(t *testing.T) {
	s := newTestServer(t, false)
	s.opts.DeviceName = "swift-eagle"
	s.cfg = &config.EMOSConfig{
		Mode:       config.ModeNative,
		ROSDistro:  "jazzy",
		LicenseKey: "secret",
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/info", nil)
	rec := httpServe(t, s, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]any
	jsonBody(t, rec, &body)
	if body["installed"] != true {
		t.Fatalf("installed = %v, want true", body["installed"])
	}
	if body["mode"] != string(config.ModeNative) {
		t.Fatalf("mode = %v, want native", body["mode"])
	}
	if body["ros_distro"] != "jazzy" {
		t.Fatalf("ros_distro = %v, want jazzy", body["ros_distro"])
	}
	if body["license_present"] != true {
		t.Fatalf("license_present = %v, want true", body["license_present"])
	}
	// The raw license must never be returned over the wire.
	if v, ok := body["license_key"]; ok {
		t.Fatalf("license_key leaked in /info body: %v", v)
	}
}

func TestHandleCapabilities(t *testing.T) {
	s := newTestServer(t, false)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/capabilities", nil)
	rec := httpServe(t, s, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]any
	jsonBody(t, rec, &body)
	for _, key := range []string{"can_run_recipes", "can_pull_recipes", "has_robot_identity", "docker_available", "pixi_available"} {
		if _, ok := body[key]; !ok {
			t.Fatalf("capability %q missing in body: %+v", key, body)
		}
	}
	// Without a config, can_run_recipes must be false.
	if body["can_run_recipes"] != false {
		t.Fatalf("can_run_recipes = %v, want false (no install)", body["can_run_recipes"])
	}
}

func TestHandleConnectivitySnapshot(t *testing.T) {
	s := newTestServer(t, false)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/connectivity", nil)
	rec := httpServe(t, s, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]any
	jsonBody(t, rec, &body)
	if _, ok := body["online"]; !ok {
		t.Fatalf("online missing")
	}
	if _, ok := body["target"]; !ok {
		t.Fatalf("target missing")
	}
}
