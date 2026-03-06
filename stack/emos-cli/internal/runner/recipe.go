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
	AutonomousMode       bool     `json:"autonomous_mode"`
	WebClient            bool     `json:"web_client"`
	Sensors              []string `json:"sensors"`
	ZenohRouterConfig    string   `json:"zenoh_router_config_file"`
}

func RunRecipe(recipeName, rmwImpl string) error {
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
	logDir := filepath.Join(config.HomeDir, "emos", "logs")
	os.MkdirAll(logDir, 0755)
	timestamp := time.Now().Format("20060102_150405")
	logFile := filepath.Join(logDir, fmt.Sprintf("%s_%s.log", recipeName, timestamp))

	ui.Header("EMOS - PRE-RECIPE SETUP")
	ui.Info("Recipe Name: " + recipeName)
	ui.Info("RMW Implementation: " + rmwImpl)
	ui.Info("Container: " + config.ContainerName)
	ui.Info("Required sensors: " + strings.Join(manifest.Sensors, ", "))
	ui.Info("Autonomous mode: " + fmt.Sprintf("%v", manifest.AutonomousMode))

	// Container management
	ui.Header("HOST & CONTAINER MANAGEMENT")

	killROSProcesses()

	if !container.Exists(config.ContainerName) {
		return fmt.Errorf("container '%s' does not exist — run 'emos install' first", config.ContainerName)
	}

	if container.IsRunning(config.ContainerName) {
		ui.Spinner("Stopping existing EMOS container...", func() error {
			return container.Stop(config.ContainerName)
		})
	}

	if err := ui.Spinner("Starting EMOS container...", func() error {
		return container.Start(config.ContainerName)
	}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	// RMW Configuration
	ui.Header("RMW CONFIGURATION")
	ui.Info("Setting RMW_IMPLEMENTATION=" + rmwImpl)
	setRMWImpl(rmwImpl)

	if rmwImpl == "rmw_zenoh_cpp" {
		if err := configureZenoh(recipeName, &manifest); err != nil {
			return err
		}
	}

	// Hardware & Sensor Launch
	ui.Header("HARDWARE & SENSOR LAUNCH")

	if err := ui.Spinner("Launching robot base hardware...", func() error {
		return container.ExecDetached(config.ContainerName,
			"source ros_entrypoint.sh && ros2 launch "+emosRoot+"/robot/launch/bringup_robot.py")
	}); err != nil {
		return err
	}

	for _, sensor := range manifest.Sensors {
		if err := launchSensor(recipeName, sensor); err != nil {
			return err
		}
	}

	// Node Verification
	ui.Header("VERIFYING ROS2 NODES")
	time.Sleep(5 * time.Second)

	if err := verifyNodes(manifest.Sensors); err != nil {
		container.Stop(config.ContainerName)
		return err
	}

	// Final Configuration
	ui.Header("FINAL CONFIGURATION")

	if manifest.AutonomousMode {
		ui.Spinner("Activating autonomous mode...", func() error {
			return container.ExecDetached(config.ContainerName,
				"source ros_entrypoint.sh && ./"+emosRoot+"/robot/scripts/activate_autonomous_mode.sh")
		})
		ui.Warn("ATTENTION: Autonomous Mode is now ON.")
	} else {
		ui.Spinner("Deactivating autonomous mode...", func() error {
			return container.ExecDetached(config.ContainerName,
				"source ros_entrypoint.sh && ./"+emosRoot+"/robot/scripts/deactivate_autonomous_mode.sh")
		})
	}

	// Recipe Execution
	ui.Header("LAUNCHING RECIPE: " + recipeName)
	ui.Info("All output will be saved to: " + logFile)

	if manifest.WebClient {
		ui.Spinner("Starting web client in background...", func() error {
			return container.ExecDetached(config.ContainerName,
				"source ros_entrypoint.sh && ros2 run automatika_embodied_agents tiny_web_client")
		})
		ui.Info("Web client should be available at http://<ROBOT_IP>:8080")
	}

	ui.Success("BEGIN RECIPE OUTPUT")
	fmt.Println()

	// Execute recipe interactively with tee to log file
	recipeCmd := fmt.Sprintf("source ros_entrypoint.sh && python3 %s/%s/recipe.py | tee %s",
		recipesRoot, recipeName, logFile)
	err = container.ExecInteractive(config.ContainerName, recipeCmd)

	fmt.Println()
	if err != nil {
		ui.Error(fmt.Sprintf("Recipe '%s' exited with an error.", recipeName))
	} else {
		ui.Success(fmt.Sprintf("Recipe '%s' finished successfully.", recipeName))
	}

	// Cleanup
	ui.Spinner("EMOS container cleanup...", func() error {
		return container.Stop(config.ContainerName)
	})

	return err
}

func killROSProcesses() {
	ui.Info("Killing host ROS processes...")
	for _, proc := range []string{"roslaunch", "roscore", "ros2"} {
		exec_command("sudo", "pkill", "-f", proc)
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

	// Set or remove ZENOH_ROUTER_CONFIG_URI
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

func launchSensor(recipeName, sensor string) error {
	// Look for custom config file
	configFile := ""
	for _, ext := range []string{"yaml", "json", "toml"} {
		candidate := fmt.Sprintf("%s/%s/%s_config.%s", recipesRoot, recipeName, sensor, ext)
		if container.FileExists(config.ContainerName, candidate) {
			configFile = candidate
			break
		}
	}

	title := "Launching sensor: " + sensor
	return ui.Spinner(title, func() error {
		cmd := "source ros_entrypoint.sh && ros2 launch " + emosRoot + "/robot/launch/bringup_" + sensor + ".py"
		if configFile != "" {
			cmd += " config_file:=" + configFile
		}
		return container.ExecDetached(config.ContainerName, cmd)
	})
}

func verifyNodes(sensors []string) error {
	robotManifest := filepath.Join(config.HomeDir, emosRoot, "robot", "manifest.json")
	data, err := os.ReadFile(robotManifest)
	if err != nil {
		return fmt.Errorf("robot manifest not found: %w", err)
	}

	var robotConfig map[string]json.RawMessage
	json.Unmarshal(data, &robotConfig)

	// Get base nodes
	var baseNodes []string
	if raw, ok := robotConfig["base"]; ok {
		json.Unmarshal(raw, &baseNodes)
	}

	// Get sensor nodes
	for _, sensor := range sensors {
		if raw, ok := robotConfig[sensor]; ok {
			var node string
			if json.Unmarshal(raw, &node) == nil && node != "" {
				baseNodes = append(baseNodes, node)
			}
		}
	}

	ui.Info("Verifying required ROS2 nodes are active...")
	allPresent := true
	for _, node := range baseNodes {
		node = strings.TrimSpace(node)
		found := false
		for i := 0; i < 10; i++ {
			out, err := container.Exec(config.ContainerName,
				"source ros_entrypoint.sh && ros2 node list")
			if err == nil && strings.Contains(out, node) {
				found = true
				break
			}
			time.Sleep(time.Second)
		}
		if found {
			ui.Success(fmt.Sprintf("Node '%s' is active.", node))
		} else {
			ui.Error(fmt.Sprintf("Node '%s' did not appear within 10 seconds!", node))
			allPresent = false
		}
	}

	if !allPresent {
		return fmt.Errorf("required nodes are missing")
	}
	return nil
}

// exec_command runs a system command, ignoring errors (used for pkill etc.)
func exec_command(name string, args ...string) {
	cmd := execCommand(name, args...)
	cmd.Run()
}
