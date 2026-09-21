package server

import (
	"encoding/json"

	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/runner"
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
	Source      string   `json:"source"`                // "plugin"
}

// DiscoverRobot reports the robot this install is for, if it knows one.
func DiscoverRobot() (*RobotInfo, bool) {
	if info := detectRobotPlugin(); info != nil {
		return info, true
	}
	return nil, false
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

	if data := cfg.Plugin.Describe; len(data) > 0 {
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
			Feedbacks []struct {
				Key     string `json:"key"`
				MsgType string `json:"msg_type"`
			} `json:"feedbacks"`
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
			// The robot's own sensors are its feedbacks of sensor message types.
			for _, f := range d.Feedbacks {
				if f.Key != "" && runner.IsSensorType(f.MsgType) {
					info.Sensors = append(info.Sensors, f.Key)
				}
			}
		}
	}
	if info.Model == "" {
		info.Model = cfg.Plugin.Slug
	}
	return info
}
