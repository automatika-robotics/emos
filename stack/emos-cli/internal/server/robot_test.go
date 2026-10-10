package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/automatika-robotics/emos-cli/internal/config"
)

func TestRobotSensorsFromPluginFeedbacks(t *testing.T) {
	s := newTestServer(t, true)
	describe := `{
	  "metadata": {"name": "M20", "vendor": "MOVA"},
	  "feedbacks": [
	    {"key": "odom", "msg_type": "Odometry"},
	    {"key": "front_camera", "msg_type": "Image"},
	    {"key": "lidar_front", "msg_type": "PointCloud2"},
	    {"key": "battery", "msg_type": "BatteryState"}
	  ],
	  "actions": [{"name": "stand"}], "events": []
	}`
	savePlugins(t, &config.PluginInfo{Slug: "m20_plugin", EntryPoint: "m20_plugin.plugin:M20Plugin",
		Role: config.RoleRobot, Describe: json.RawMessage(describe)})

	rec := httpServe(t, s, httptest.NewRequest(http.MethodGet, "/api/v1/robot", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var info RobotInfo
	jsonBody(t, rec, &info)
	if info.Source != "plugin" || info.Model != "M20" || info.Vendor != "MOVA" {
		t.Fatalf("info = %+v", info)
	}
	// Sensor-typed feedbacks, in declaration order; BatteryState is not a sensor feed.
	if got := strings.Join(info.Sensors, ","); got != "odom,front_camera,lidar_front" {
		t.Fatalf("sensors = %q", got)
	}
	if strings.Join(info.Actions, ",") != "stand" {
		t.Fatalf("actions = %v", info.Actions)
	}
}
