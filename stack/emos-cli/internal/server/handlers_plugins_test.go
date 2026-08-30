package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/automatika-robotics/emos-cli/internal/api"
	"github.com/automatika-robotics/emos-cli/internal/config"
)

// savePlugins writes a config with the given robot and sensors into the
// test's temp config dir (newTestServer must have been called first).
func savePlugins(t *testing.T, robot *config.PluginInfo, sensors ...config.PluginInfo) {
	t.Helper()
	cfg := &config.EMOSConfig{Mode: config.ModePixi, ROSDistro: "jazzy", Plugin: robot, SensorPlugins: sensors}
	if err := config.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
}

func TestPluginsInstalledEmpty(t *testing.T) {
	s := newTestServer(t, true)
	rec := httpServe(t, s, httptest.NewRequest(http.MethodGet, "/api/v1/plugins/installed", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (nothing installed is not an error)", rec.Code)
	}
	// Exact wire shape: robot is null, sensors is an empty array, never null.
	if got := rec.Body.String(); got != `{"robot":null,"sensors":[]}`+"\n" {
		t.Fatalf("body = %s", got)
	}
}

func TestPluginsInstalled(t *testing.T) {
	s := newTestServer(t, true)
	savePlugins(t,
		&config.PluginInfo{Slug: "m20_plugin", EntryPoint: "m20_plugin.plugin:M20Plugin", Role: config.RoleRobot,
			Repo: "https://x/m20", Sources: []string{"rslidar_sdk", "rslidar_msg"},
			Describe: json.RawMessage(`{"role":"robot"}`)},
		config.PluginInfo{Slug: "hikmicro_plugin", EntryPoint: "hikmicro_plugin:HikmicroBispectrum", Role: config.RoleSensor},
		config.PluginInfo{Slug: "legacy_plugin", EntryPoint: "legacy:Legacy"}, // pre-role record
	)
	rec := httpServe(t, s, httptest.NewRequest(http.MethodGet, "/api/v1/plugins/installed", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var got InstalledPlugins
	jsonBody(t, rec, &got)
	if got.Robot == nil || got.Robot.Slug != "m20_plugin" || got.Robot.Role != "robot" ||
		len(got.Robot.Sources) != 2 || string(got.Robot.Describe) != `{"role":"robot"}` {
		t.Fatalf("robot = %+v", got.Robot)
	}
	if len(got.Sensors) != 2 || got.Sensors[0].Slug != "hikmicro_plugin" || got.Sensors[0].Role != "sensor" {
		t.Fatalf("sensors = %+v", got.Sensors)
	}
	if got.Sensors[1].Role != "robot" {
		t.Fatalf("a record without a role defaults to robot, got %q", got.Sensors[1].Role)
	}
}

func TestCatalogEntryRole(t *testing.T) {
	if got := catalogEntry(api.Plugin{Filename: "m20"}).Role; got != "robot" {
		t.Fatalf("missing role -> %q, want robot", got)
	}
	if got := catalogEntry(api.Plugin{Filename: "hik", Role: "sensor"}).Role; got != "sensor" {
		t.Fatalf("role = %q, want sensor", got)
	}
}

func TestPluginRemoveSlugNotInstalled(t *testing.T) {
	s := newTestServer(t, true)
	savePlugins(t, &config.PluginInfo{Slug: "m20_plugin", EntryPoint: "m:M"})
	rec := httpServe(t, s, httptest.NewRequest(http.MethodDelete, "/api/v1/plugins/nope", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestPluginRemoveSlug(t *testing.T) {
	s := newTestServer(t, true)
	// A single installed plugin: removing it wipes the (temp) workspace and
	// needs no rebuild, so the job completes without a toolchain.
	savePlugins(t, nil, config.PluginInfo{Slug: "hikmicro_plugin", EntryPoint: "h:H", Role: config.RoleSensor})

	rec := httpServe(t, s, httptest.NewRequest(http.MethodDelete, "/api/v1/plugins/hikmicro_plugin", nil))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	jsonBody(t, rec, &resp)
	job := s.jobs.Get(resp["job_id"])
	if job == nil || job.Kind != "plugin_remove" || job.Target != "hikmicro_plugin" {
		t.Fatalf("job = %+v", job)
	}
	select {
	case <-job.Done():
	case <-time.After(10 * time.Second):
		t.Fatal("remove job did not finish")
	}
	if v := job.Snapshot(); v.Status != JobStatusFinished {
		t.Fatalf("job = %+v", v)
	}
	if cfg := config.LoadConfig(); cfg == nil || len(cfg.Plugins()) != 0 {
		t.Fatalf("plugins after remove = %+v", cfg.Plugins())
	}
}
