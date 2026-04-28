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
	ZenohRouterConfig string `json:"zenoh_router_config_file"`
}

// LoadManifest reads a recipe manifest file. Always returns a non-nil pointer
// — a missing or malformed manifest yields an empty manifest, since the file
// is optional.
func LoadManifest(path string) *recipeManifest {
	m := &recipeManifest{}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, m)
	}
	return m
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
	recipeFile := filepath.Join(recipePath, "recipe.py")
	if _, err := os.Stat(recipeFile); os.IsNotExist(err) {
		ui.Error(fmt.Sprintf("Recipe '%s' not found in '%s'", recipeName, config.RecipesDir))
		ui.Faint("Run 'emos ls' to see available recipes.")
		return fmt.Errorf("recipe not found")
	}

	// Parse manifest (optional — only needed for zenoh config)
	var manifest recipeManifest
	manifestPath := filepath.Join(recipePath, "manifest.json")
	if data, err := os.ReadFile(manifestPath); err == nil {
		json.Unmarshal(data, &manifest)
	}

	// Extract topics from recipe.py via AST
	topics, err := ExtractTopics(recipeFile)
	if err != nil {
		ui.Warn(fmt.Sprintf("Could not extract topics: %v", err))
	}
	sensorTopics := SensorTopics(topics)

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
	distro := cfg.ROSDistro
	if distro == "" {
		distro = "jazzy"
	}

	ui.Header("EMOS - PRE-RECIPE SETUP")
	ui.Info("Recipe Name: " + recipeName)
	ui.Info("Mode: " + string(mode))
	ui.Info("RMW Implementation: " + rmwImpl)
	if len(sensorTopics) > 0 {
		names := make([]string, len(sensorTopics))
		for i, t := range sensorTopics {
			names[i] = topicName(t.Name)
		}
		ui.Info("Sensor topics: " + strings.Join(names, ", "))
	}

	var strategy RuntimeStrategy
	switch mode {
	case config.ModeOSSContainer:
		strategy = NewContainerStrategy(false)
	case config.ModeLicensed:
		strategy = NewContainerStrategy(true)
	case config.ModeNative:
		strategy = NewNativeStrategy()
	case config.ModePixi:
		strategy = NewPixiStrategy(cfg.PixiProjectDir)
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

	if !skipSensorCheck {
		if err := strategy.VerifySensorTopics(sensorTopics, distro); err != nil {
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

