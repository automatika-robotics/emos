package runner

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/automatika-robotics/emos-cli/internal/ui"
)

var boldLabel = lipgloss.NewStyle().Bold(true).Foreground(ui.ThemeBlue)

// SensorInfo describes a sensor type identified by its ROS message type short name.
type SensorInfo struct {
	DisplayName    string
	ROSType        string
	CommonPackages []string // apt package names with {distro} placeholder
}

// sensorKnowledge maps msg_type short names to sensor metadata.
var sensorKnowledge = map[string]SensorInfo{
	"Image": {
		DisplayName:    "RGB Camera",
		ROSType:        "sensor_msgs/msg/Image",
		CommonPackages: []string{"ros-{distro}-usb-cam", "ros-{distro}-v4l2-camera", "ros-{distro}-realsense2-camera"},
	},
	"CompressedImage": {
		DisplayName:    "Camera (compressed)",
		ROSType:        "sensor_msgs/msg/CompressedImage",
		CommonPackages: []string{"ros-{distro}-usb-cam", "ros-{distro}-image-transport-plugins"},
	},
	"LaserScan": {
		DisplayName:    "2D Lidar",
		ROSType:        "sensor_msgs/msg/LaserScan",
		CommonPackages: []string{"ros-{distro}-rplidar-ros", "ros-{distro}-urg-node", "ros-{distro}-sllidar-ros"},
	},
	"Audio": {
		DisplayName:    "Microphone",
		ROSType:        "std_msgs/msg/ByteMultiArray",
		CommonPackages: []string{},
	},
	"Odometry": {
		DisplayName:    "Robot base / wheel encoders",
		ROSType:        "nav_msgs/msg/Odometry",
		CommonPackages: []string{},
	},
	"RGBD": {
		DisplayName:    "Depth camera",
		ROSType:        "realsense2_camera_msgs/msg/RGBD",
		CommonPackages: []string{"ros-{distro}-realsense2-camera"},
	},
	"Imu": {
		DisplayName:    "IMU",
		ROSType:        "sensor_msgs/msg/Imu",
		CommonPackages: []string{},
	},
	"PointCloud2": {
		DisplayName:    "3D Lidar / Depth camera",
		ROSType:        "sensor_msgs/msg/PointCloud2",
		CommonPackages: []string{"ros-{distro}-realsense2-camera", "ros-{distro}-velodyne"},
	},
}

// ExtractedTopic represents a Topic(...) call found in a recipe.py via AST parsing.
type ExtractedTopic struct {
	Name     string `json:"name"`
	MsgType  string `json:"msg_type"`
	IsSensor bool   `json:"is_sensor"`
}

const extractScript = `
import ast, json, sys

SENSOR_TYPES = {"Image", "CompressedImage", "LaserScan", "Audio", "Odometry", "RGBD", "Imu", "PointCloud2"}

def extract(path):
    tree = ast.parse(open(path).read())
    topics = []
    for node in ast.walk(tree):
        if isinstance(node, ast.Call):
            f = node.func
            if (isinstance(f, ast.Name) and f.id == "Topic") or \
               (isinstance(f, ast.Attribute) and f.attr == "Topic"):
                t = {}
                for kw in node.keywords:
                    if kw.arg in ("name", "msg_type") and isinstance(kw.value, ast.Constant):
                        t[kw.arg] = kw.value.value
                if "name" in t and "msg_type" in t:
                    t["is_sensor"] = t["msg_type"] in SENSOR_TYPES
                    topics.append(t)
    json.dump(topics, sys.stdout)

extract(sys.argv[1])
`

// ExtractTopics runs the embedded Python AST script on a recipe file and returns the topics.
func ExtractTopics(recipePath string) ([]ExtractedTopic, error) {
	cmd := exec.Command("python3", "-c", extractScript, recipePath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to extract topics from %s: %w\n%s", recipePath, err, string(out))
	}

	var topics []ExtractedTopic
	if err := json.Unmarshal(out, &topics); err != nil {
		return nil, fmt.Errorf("failed to parse extracted topics: %w", err)
	}
	return topics, nil
}

// SensorTopics returns only the sensor topics from the extracted list.
func SensorTopics(topics []ExtractedTopic) []ExtractedTopic {
	var sensors []ExtractedTopic
	for _, t := range topics {
		if t.IsSensor {
			sensors = append(sensors, t)
		}
	}
	return sensors
}

// OtherTopics returns only the non-sensor topics from the extracted list.
func OtherTopics(topics []ExtractedTopic) []ExtractedTopic {
	var other []ExtractedTopic
	for _, t := range topics {
		if !t.IsSensor {
			other = append(other, t)
		}
	}
	return other
}

// capitalize returns the string with its first letter uppercased.
func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// DisplayTopicInfo prints the topic info table to the terminal.
func DisplayTopicInfo(recipeName string, topics []ExtractedTopic, distro string) {
	sensors := SensorTopics(topics)
	other := OtherTopics(topics)

	fmt.Println()
	ui.Header("Recipe: " + recipeName)

	if len(sensors) == 0 {
		fmt.Println()
		ui.Info("No sensor topics required.")
	} else {
		fmt.Println()
		fmt.Println(boldLabel.Render("  Required Sensors:"))

		var rows [][]string
		for _, t := range sensors {
			hw := t.MsgType
			if info, ok := sensorKnowledge[t.MsgType]; ok {
				hw = info.DisplayName
			}
			rows = append(rows, []string{topicName(t.Name), t.MsgType, hw})
		}
		ui.PrintTable([]string{"Topic", "Type", "Hardware"}, rows)

		// Package suggestions
		seen := map[string]bool{}
		type suggestion struct {
			hardware string
			packages []string
		}
		var suggestions []suggestion

		for _, t := range sensors {
			info, ok := sensorKnowledge[t.MsgType]
			if !ok || len(info.CommonPackages) == 0 || seen[t.MsgType] {
				continue
			}
			seen[t.MsgType] = true
			var pkgs []string
			for _, p := range info.CommonPackages {
				pkgs = append(pkgs, strings.ReplaceAll(p, "{distro}", distro))
			}
			suggestions = append(suggestions, suggestion{hardware: info.DisplayName, packages: pkgs})
		}

		if len(suggestions) > 0 {
			fmt.Println()
			fmt.Printf("  Suggested packages (for ROS 2 %s):\n", capitalize(distro))
			for _, s := range suggestions {
				fmt.Printf("    %-22s %s\n", s.hardware+":", strings.Join(s.packages, ", "))
			}
		}
	}

	if len(other) > 0 {
		fmt.Println()
		fmt.Println(boldLabel.Render("  Other Topics:"))
		var rows [][]string
		for _, t := range other {
			rows = append(rows, []string{topicName(t.Name), t.MsgType})
		}
		ui.PrintTable([]string{"Topic", "Type"}, rows)
	}

	fmt.Println()
}

// topicName ensures the topic name has a leading slash for display.
func topicName(name string) string {
	if !strings.HasPrefix(name, "/") {
		return "/" + name
	}
	return name
}

// verifySensorTopicsAST checks that extracted sensor topics are published.
func verifySensorTopicsAST(sensors []ExtractedTopic, check topicChecker, distro string) error {
	if len(sensors) == 0 {
		ui.Info("No sensor verification required.")
		return nil
	}

	ui.Info("Verifying sensor topics are available...")
	var missing []ExtractedTopic

	for _, t := range sensors {
		topic := topicName(t.Name)
		found := false
		for i := 0; i < 10; i++ {
			out, err := check()
			if err == nil && strings.Contains(out, topic) {
				found = true
				break
			}
			time.Sleep(time.Second)
		}
		if found {
			ui.Success(fmt.Sprintf("Topic '%s' (%s) found.", topic, t.MsgType))
		} else {
			ui.Error(fmt.Sprintf("Topic '%s' (%s) not found within 10s.", topic, t.MsgType))
			missing = append(missing, t)
		}
	}

	if len(missing) > 0 {
		fmt.Println()
		ui.Warn("Missing sensor topics. Ensure the following hardware is connected and publishing:")
		for _, t := range missing {
			hw := t.MsgType
			if info, ok := sensorKnowledge[t.MsgType]; ok {
				hw = info.DisplayName
			}
			fmt.Printf("    %s  <-  %s\n", topicName(t.Name), hw)
		}
		fmt.Println()
		return fmt.Errorf("required sensor topics are missing")
	}
	return nil
}
