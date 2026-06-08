package server

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/plugin"
)

// RobotInfo is best-effort identity about the device. The dashboard renders
// a generic device card if /robot returns 404
type RobotInfo struct {
	Name        string   `json:"name,omitempty"`
	Model       string   `json:"model,omitempty"`
	Serial      string   `json:"serial,omitempty"`
	Vendor      string   `json:"vendor,omitempty"`
	Kinematics  string   `json:"kinematics,omitempty"`
	Sensors     []string `json:"sensors,omitempty"`
	Plugin      string   `json:"plugin,omitempty"`      // active plugin entry point (module:Class)
	Description string   `json:"description,omitempty"` // from the plugin's metadata
	ImageURL    string   `json:"image_url,omitempty"`   // portal-served robot picture, if any
	Actions     []string `json:"actions,omitempty"`     // plugin-provided action names
	Events      []string `json:"events,omitempty"`      // plugin-provided event names
	Source      string   `json:"source"`                // "manifest" | "plugin" | "config"
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

// readRobotManifest reads ~/emos/robot/manifest.json
// NOTE: Available in licensed deployments.
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

// detectRobotPlugin reports the active robot plugin recorded in the EMOS config,
// enriched with the plugin's cached describe() metadata.
func detectRobotPlugin() *RobotInfo {
	cfg := config.LoadConfig()
	if cfg == nil || cfg.Plugin == nil {
		return nil
	}
	info := &RobotInfo{
		Plugin:   cfg.Plugin.EntryPoint,
		ImageURL: cfg.Plugin.ImageURL,
		Source:   "plugin",
	}

	if data, ok := plugin.CachedDescribe(); ok {
		var d struct {
			Metadata struct {
				Name        string `json:"name"`
				Vendor      string `json:"vendor"`
				Description string `json:"description"`
			} `json:"metadata"`
			Actions []struct {
				Name string `json:"name"`
			} `json:"actions"`
			Events []struct {
				Name string `json:"name"`
			} `json:"events"`
		}
		if json.Unmarshal(data, &d) == nil {
			// The plugin's metadata name is the robot's model/type
			info.Model = d.Metadata.Name
			info.Vendor = d.Metadata.Vendor
			info.Description = d.Metadata.Description
			for _, a := range d.Actions {
				if a.Name != "" {
					info.Actions = append(info.Actions, a.Name)
				}
			}
			for _, e := range d.Events {
				if e.Name != "" {
					info.Events = append(info.Events, e.Name)
				}
			}
		}
	}
	if info.Model == "" {
		info.Model = cfg.Plugin.Slug
	}
	return info
}
