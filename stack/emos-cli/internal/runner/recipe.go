package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/container"
	"github.com/automatika-robotics/emos-cli/internal/ui"
)

const (
	emosRoot    = "/emos"
	recipesRoot = "/emos/recipes"
)

type recipeManifest struct {
	AutonomousMode    bool                `json:"autonomous_mode"`
	WebClient         bool                `json:"web_client"`
	Sensors           []string            `json:"sensors"`
	ZenohRouterConfig string              `json:"zenoh_router_config_file"`
	SensorTopics      map[string][]string `json:"sensor_topics,omitempty"`
}

func RunRecipe(recipeName, rmwImpl string, skipSensorCheck bool) error {
	// Validate RMW implementation
	validRMW := map[string]bool{
		"rmw_fastrtps_cpp":   true,
		"rmw_cyclonedds_cpp": true,
		"rmw_zenoh_cpp":      true,
	}
	if !validRMW[rmwImpl] {
		return fmt.Errorf("invalid RMW implementation: %s (allowed: rmw_fastrtps_cpp, rmw_cyclonedds_cpp, rmw_zenoh_cpp)", rmwImpl)
	}

	// Check recipe exists
	recipePath := filepath.Join(config.RecipesDir, recipeName)
	manifestPath := filepath.Join(recipePath, "manifest.json")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		ui.Error(fmt.Sprintf("Recipe '%s' not found in '%s'", recipeName, config.RecipesDir))
		ui.Faint("Run 'emos ls' to see available recipes.")
		return fmt.Errorf("recipe not found")
	}

	// Parse manifest
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest: %w", err)
	}
	var manifest recipeManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("failed to parse manifest: %w", err)
	}

	// Setup logging
	os.MkdirAll(config.LogsDir, 0755)
	timestamp := time.Now().Format("20060102_150405")
	logFile := filepath.Join(config.LogsDir, fmt.Sprintf("%s_%s.log", recipeName, timestamp))

	// Determine strategy from config
	cfg := config.LoadConfig()
	if cfg == nil {
		return fmt.Errorf("no EMOS installation found — run 'emos install' first")
	}
	mode := cfg.Mode

	ui.Header("EMOS - PRE-RECIPE SETUP")
	ui.Info("Recipe Name: " + recipeName)
	ui.Info("Mode: " + string(mode))
	ui.Info("RMW Implementation: " + rmwImpl)
	ui.Info("Required sensors: " + strings.Join(manifest.Sensors, ", "))
	ui.Info("Autonomous mode: " + fmt.Sprintf("%v", manifest.AutonomousMode))

	var strategy RuntimeStrategy
	switch mode {
	case config.ModeOSSContainer:
		strategy = NewContainerStrategy(false)
	case config.ModeLicensed:
		strategy = NewContainerStrategy(true)
	case config.ModeNative:
		strategy = NewNativeStrategy()
	default:
		return fmt.Errorf("unknown install mode: %s", mode)
	}

	// Execute the recipe pipeline
	if err := strategy.PrepareEnvironment(); err != nil {
		return err
	}

	if err := strategy.SetRMWImpl(rmwImpl); err != nil {
		return err
	}

	if rmwImpl == "rmw_zenoh_cpp" {
		if err := strategy.ConfigureZenoh(recipeName, &manifest); err != nil {
			return err
		}
	}

	if err := strategy.LaunchRobotHardware(); err != nil {
		return err
	}

	for _, sensor := range manifest.Sensors {
		configFile := findSensorConfig(recipeName, sensor, mode)
		if err := strategy.LaunchSensor(recipeName, sensor, configFile); err != nil {
			return err
		}
	}

	if !skipSensorCheck {
		if err := strategy.VerifyNodes(manifest.Sensors, &manifest); err != nil {
			strategy.Cleanup()
			return err
		}
	} else {
		ui.Info("Sensor check skipped (--skip-sensor-check)")
	}

	err = strategy.ExecRecipe(recipeName, &manifest, logFile)

	fmt.Println()
	if err != nil {
		ui.Error(fmt.Sprintf("Recipe '%s' exited with an error.", recipeName))
	} else {
		ui.Success(fmt.Sprintf("Recipe '%s' finished successfully.", recipeName))
	}

	strategy.Cleanup()
	return err
}

// findSensorConfig looks for a sensor-specific config file in the recipe directory.
func findSensorConfig(recipeName, sensor string, mode config.InstallMode) string {
	if mode == config.ModeNative {
		for _, ext := range []string{"yaml", "json", "toml"} {
			candidate := filepath.Join(config.RecipesDir, recipeName, sensor+"_config."+ext)
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
		return ""
	}
	// Container modes: check inside container
	for _, ext := range []string{"yaml", "json", "toml"} {
		candidate := fmt.Sprintf("%s/%s/%s_config.%s", recipesRoot, recipeName, sensor, ext)
		if container.FileExists(config.ContainerName, candidate) {
			return candidate
		}
	}
	return ""
}

func killROSProcesses() {
	ui.Info("Killing host ROS processes...")
	for _, proc := range []string{"roslaunch", "roscore", "ros2"} {
		runQuiet("sudo", "pkill", "-f", proc)
	}
	time.Sleep(time.Second)
	ui.Success("Terminated host ROS processes.")
}

func setRMWImpl(rmwImpl string) {
	script := fmt.Sprintf(`
if grep -q '^export RMW_IMPLEMENTATION=' /ros_entrypoint.sh; then
  sed -i 's|^export RMW_IMPLEMENTATION=.*|export RMW_IMPLEMENTATION=%s|' /ros_entrypoint.sh
else
  sed -i '1a export RMW_IMPLEMENTATION=%s' /ros_entrypoint.sh
fi`, rmwImpl, rmwImpl)
	container.Exec(config.ContainerName, script)
}

func configureZenoh(recipeName string, manifest *recipeManifest) error {
	zenohConfig := manifest.ZenohRouterConfig
	zenohConfigURI := ""

	if zenohConfig != "" {
		uri := recipesRoot + "/" + zenohConfig
		if strings.HasSuffix(uri, ".json5") {
			if !container.FileExists(config.ContainerName, uri) {
				ui.Warn("Zenoh config file not found — using default")
			} else {
				zenohConfigURI = uri
				ui.Info("Using Zenoh router config: " + zenohConfigURI)
			}
		} else {
			ui.Warn("Zenoh config must be .json5 — using default")
		}
	} else {
		ui.Info("Using default Zenoh router configuration.")
	}

	if err := ui.Spinner("Starting zenoh router...", func() error {
		return container.ExecDetached(config.ContainerName,
			"source ros_entrypoint.sh && ros2 run rmw_zenoh_cpp rmw_zenohd")
	}); err != nil {
		return err
	}
	time.Sleep(2 * time.Second)

	if zenohConfigURI != "" {
		container.Exec(config.ContainerName, fmt.Sprintf(`
if grep -q '^export ZENOH_ROUTER_CONFIG_URI=' /ros_entrypoint.sh; then
  sed -i 's|^export ZENOH_ROUTER_CONFIG_URI=.*|export ZENOH_ROUTER_CONFIG_URI=%s|' /ros_entrypoint.sh
else
  sed -i '1a export ZENOH_ROUTER_CONFIG_URI=%s' /ros_entrypoint.sh
fi`, zenohConfigURI, zenohConfigURI))
	} else {
		container.Exec(config.ContainerName,
			"sed -i '/^export ZENOH_ROUTER_CONFIG_URI=/d' /ros_entrypoint.sh")
	}

	return nil
}

// runQuiet runs a system command, ignoring errors (used for pkill etc.)
func runQuiet(name string, args ...string) {
	execCommand(name, args...).Run()
}

// topicChecker abstracts how to run `ros2 topic list` for different modes.
type topicChecker func() (string, error)

// expectedSensorTopics builds the map of sensor->topics from the manifest or defaults.
func expectedSensorTopics(sensors []string, manifest *recipeManifest) map[string][]string {
	topics := map[string][]string{}
	for _, sensor := range sensors {
		if t, ok := manifest.SensorTopics[sensor]; ok {
			topics[sensor] = t
		} else {
			switch sensor {
			case "camera":
				topics[sensor] = []string{"/camera/image_raw"}
			case "lidar":
				topics[sensor] = []string{"/scan"}
			default:
				topics[sensor] = []string{"/" + sensor}
			}
		}
	}
	return topics
}

// verifySensorTopics checks that expected sensor topics are published.
func verifySensorTopics(sensors []string, manifest *recipeManifest, check topicChecker) error {
	if len(sensors) == 0 && len(manifest.SensorTopics) == 0 {
		ui.Info("No sensor verification required.")
		return nil
	}

	expected := expectedSensorTopics(sensors, manifest)
	ui.Info("Verifying sensor topics are available...")
	allPresent := true
	for sensor, topics := range expected {
		for _, topic := range topics {
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
				ui.Success(fmt.Sprintf("Sensor '%s': topic '%s' found.", sensor, topic))
			} else {
				ui.Error(fmt.Sprintf("Sensor '%s': topic '%s' not found within 10s.", sensor, topic))
				allPresent = false
			}
		}
	}
	if !allPresent {
		return fmt.Errorf("required sensor topics are missing")
	}
	return nil
}
