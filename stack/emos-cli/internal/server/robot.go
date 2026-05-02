package server

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/automatika-robotics/emos-cli/internal/config"
)

// RobotInfo is best-effort identity about the device. The dashboard renders
// a generic device card if /robot returns 404
// TODO: Populate from installed robot plugins or license info
type RobotInfo struct {
	Name       string   `json:"name,omitempty"`
	Model      string   `json:"model,omitempty"`
	Serial     string   `json:"serial,omitempty"`
	Vendor     string   `json:"vendor,omitempty"`
	Kinematics string   `json:"kinematics,omitempty"`
	Sensors    []string `json:"sensors,omitempty"`
	Plugin     string   `json:"plugin,omitempty"`
	Source     string   `json:"source"` // "manifest" | "plugin" | "config"
}

// DiscoverRobot tries each known source in order and returns the first hit.
func DiscoverRobot() (*RobotInfo, bool) {
	if info := readRobotManifest(); info != nil {
		return info, true
	}
	if info := detectRobotPlugin(); info != nil {
		return info, true
	}
	return nil, false
}

// readRobotManifest reads ~/emos/robot/manifest.json (licensed deployments)
func readRobotManifest() *RobotInfo {
	path := filepath.Join(config.HomeDir, "emos", "robot", "manifest.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	// Manifest may be a flat dict (current shape: {base: [...], lidar: "...", ...})
	// or richer
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	info := &RobotInfo{Source: "manifest"}
	if v, ok := raw["name"].(string); ok {
		info.Name = v
	}
	if v, ok := raw["model"].(string); ok {
		info.Model = v
	}
	if v, ok := raw["serial"].(string); ok {
		info.Serial = v
	}
	if v, ok := raw["vendor"].(string); ok {
		info.Vendor = v
	}
	if v, ok := raw["kinematics"].(string); ok {
		info.Kinematics = v
	}
	for k, v := range raw {
		switch k {
		case "name", "model", "serial", "vendor", "kinematics":
			continue
		}
		if _, ok := v.(string); ok {
			info.Sensors = append(info.Sensors, k)
		}
	}
	if info.Name == "" && info.Model == "" && len(info.Sensors) == 0 {
		return nil
	}
	return info
}

// detectRobotPlugin looks for an installed sugarcoat-style robot plugin in the
// EMOS workspace.
// NOTE: uses a simple filesystem heuristic; future revs can hit `ros2 pkg list`
func detectRobotPlugin() *RobotInfo {
	cfg := config.LoadConfig()
	if cfg == nil {
		return nil
	}
	candidates := []string{}
	if cfg.WorkspacePath != "" {
		candidates = append(candidates, filepath.Join(cfg.WorkspacePath, "src"))
	}
	candidates = append(candidates, filepath.Join(config.HomeDir, "emos", "ros_ws", "src"))

	for _, base := range candidates {
		entries, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			if len(name) >= len("_robot_plugin") &&
				name[len(name)-len("_robot_plugin"):] == "_robot_plugin" {
				return &RobotInfo{
					Plugin: name,
					Source: "plugin",
				}
			}
		}
	}
	return nil
}
