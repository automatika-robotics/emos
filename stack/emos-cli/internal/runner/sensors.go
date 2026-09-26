package runner

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/ui"
)

var boldLabel = lipgloss.NewStyle().Bold(true).Foreground(ui.ThemeBlue)

// sensorTypes are the message types EMOS treats as sensor feeds.
var sensorTypes = map[string]bool{
	"Image":           true,
	"CompressedImage": true,
	"LaserScan":       true,
	"Audio":           true,
	"Odometry":        true,
	"RGBD":            true,
	"Imu":             true,
	"PointCloud2":     true,
}

// IsSensorType reports whether a message type short name is one EMOS treats
// as a sensor feed.
func IsSensorType(msgType string) bool {
	return sensorTypes[msgType]
}

// ExtractedTopic represents a Topic(...) call found in a recipe.py via AST parsing.
type ExtractedTopic struct {
	Name     string `json:"name"`
	MsgType  string `json:"msg_type"`
	IsSensor bool   `json:"is_sensor"`
	// UsePlugin is set when a plugin carries the topic rather than ROS.
	UsePlugin bool   `json:"use_plugin,omitempty"`
	PluginID  string `json:"plugin_id,omitempty"`
}

// Via names what carries the topic: a plugin, or ROS.
func (t ExtractedTopic) Via() string {
	switch {
	case !t.UsePlugin:
		return "ROS topic"
	case t.PluginID == "":
		return "robot plugin"
	default:
		return "plugin " + t.PluginID
	}
}

// extractScript prints the Topic(...) calls in a recipe (Python3.8 compatible).
const extractScript = `
import ast, json, sys

def source(node):
    if isinstance(node, ast.Name):
        return node.id
    if isinstance(node, ast.Attribute):
        inner = source(node.value)
        return inner + "." + node.attr if inner else ""
    return ""

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
                    elif kw.arg == "use_plugin":
                        value = kw.value
                        if isinstance(value, ast.Constant):
                            if value.value is True:
                                t["use_plugin"] = True
                            elif isinstance(value.value, str) and value.value:
                                t["use_plugin"] = True
                                t["plugin_id"] = value.value
                        else:
                            t["use_plugin"] = True
                            t["plugin_id"] = source(value)
                if "name" in t and "msg_type" in t:
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
	for i := range topics {
		topics[i].IsSensor = IsSensorType(topics[i].MsgType)
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

// DisplayTopicInfo prints the topic info table to the terminal, then what the
// topics need to carry data, given the plugins installed in cfg (may be nil).
func DisplayTopicInfo(recipeName string, topics []ExtractedTopic, cfg *config.EMOSConfig) {
	fmt.Println()
	ui.Header("Recipe: " + recipeName)

	sensors := SensorTopics(topics)
	if len(sensors) == 0 {
		fmt.Println()
		ui.Info("No sensor topics required.")
	} else {
		printTopics("Sensors:", sensors)
	}
	if other := OtherTopics(topics); len(other) > 0 {
		printTopics("Other Topics:", other)
	}
	printTopicNeeds(topics, cfg)
	fmt.Println()
}

func printTopicNeeds(topics []ExtractedTopic, cfg *config.EMOSConfig) {
	var robot, named, drivers bool
	for _, t := range topics {
		switch {
		case t.UsePlugin && t.PluginID == "":
			robot = true
		case t.UsePlugin:
			named = true
		case t.IsSensor:
			drivers = true
		}
	}
	if !robot && !named && !drivers {
		return
	}
	fmt.Println()
	if robot {
		if cfg != nil && cfg.Plugin != nil {
			ui.Info("Robot plugin topics come from the installed robot plugin, " + cfg.Plugin.Slug + ".")
		} else {
			ui.Warn("Robot plugin topics need a robot plugin, and none is installed. " +
				"Install one with 'emos plugin install <plugin>'.")
		}
	}
	if named {
		if cfg != nil && len(cfg.SensorPlugins) > 0 {
			ui.Info("Topics via a named plugin come from the sensor plugin the recipe attaches with that id.")
		} else {
			ui.Warn("Topics via a named plugin need a sensor plugin, and none is installed. " +
				"Install one with 'emos plugin install <plugin>'.")
		}
	}
	if drivers {
		ui.Info("ROS sensor topics have to be published by a ROS driver for the sensor, or by a node the recipe starts.")
	}
}

func printTopics(label string, topics []ExtractedTopic) {
	fmt.Println()
	fmt.Println(boldLabel.Render("  " + label))
	var rows [][]string
	for _, t := range topics {
		rows = append(rows, []string{topicName(t), t.MsgType, t.Via()})
	}
	ui.PrintTable([]string{"Topic", "Type", "Via"}, rows)
}

// topicName is how a topic is shown. ROS topics with leading slash, and
// plugin feeds by the name the plugin knows it by.
func topicName(t ExtractedTopic) string {
	if t.UsePlugin || strings.HasPrefix(t.Name, "/") {
		return t.Name
	}
	return "/" + t.Name
}
