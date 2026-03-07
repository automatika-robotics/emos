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

// ContainerStrategy handles recipe execution inside a Docker container.
// It supports both oss-container (licensed=false) and licensed (licensed=true) modes.
type ContainerStrategy struct {
	licensed bool
}

func NewContainerStrategy(licensed bool) *ContainerStrategy {
	return &ContainerStrategy{licensed: licensed}
}

func (s *ContainerStrategy) PrepareEnvironment() error {
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

	return nil
}

func (s *ContainerStrategy) SetRMWImpl(rmw string) error {
	ui.Header("RMW CONFIGURATION")
	ui.Info("Setting RMW_IMPLEMENTATION=" + rmw)
	setRMWImpl(rmw)
	return nil
}

func (s *ContainerStrategy) ConfigureZenoh(recipeName string, manifest *recipeManifest) error {
	return configureZenoh(recipeName, manifest)
}

func (s *ContainerStrategy) LaunchRobotHardware() error {
	ui.Header("HARDWARE & SENSOR LAUNCH")

	if !s.licensed {
		ui.Info("OSS container mode: skipping robot hardware launch.")
		ui.Faint("Ensure your robot hardware drivers are running externally.")
		return nil
	}

	return ui.Spinner("Launching robot base hardware...", func() error {
		return container.ExecDetached(config.ContainerName,
			"source ros_entrypoint.sh && ros2 launch "+emosRoot+"/robot/launch/bringup_robot.py")
	})
}

func (s *ContainerStrategy) LaunchSensor(recipeName, sensor, configFile string) error {
	if !s.licensed {
		ui.Info(fmt.Sprintf("OSS container mode: skipping sensor '%s' launch. Ensure it is running externally.", sensor))
		return nil
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

func (s *ContainerStrategy) VerifyNodes(sensors []string, manifest *recipeManifest) error {
	ui.Header("VERIFYING ROS2 NODES")
	time.Sleep(5 * time.Second)

	if s.licensed {
		return verifyNodes(sensors)
	}

	// OSS mode: verify by topic presence
	checker := func() (string, error) {
		return container.Exec(config.ContainerName,
			"source ros_entrypoint.sh && ros2 topic list")
	}
	return verifySensorTopics(sensors, manifest, checker)
}

func (s *ContainerStrategy) ExecRecipe(recipeName string, manifest *recipeManifest, logFile string) error {
	// Final configuration
	ui.Header("FINAL CONFIGURATION")

	if s.licensed {
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
	}

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

	recipeCmd := fmt.Sprintf("source ros_entrypoint.sh && python3 %s/%s/recipe.py | tee %s",
		recipesRoot, recipeName, logFile)
	return container.ExecInteractive(config.ContainerName, recipeCmd)
}

func (s *ContainerStrategy) Cleanup() error {
	ui.Spinner("EMOS container cleanup...", func() error {
		return container.Stop(config.ContainerName)
	})
	return nil
}

// verifyNodes checks for expected ROS nodes using the robot manifest (licensed mode).
func verifyNodes(sensors []string) error {
	robotManifest := filepath.Join(config.HomeDir, "emos", "robot", "manifest.json")
	data, err := os.ReadFile(robotManifest)
	if err != nil {
		return fmt.Errorf("robot manifest not found: %w", err)
	}

	var robotConfig map[string]json.RawMessage
	json.Unmarshal(data, &robotConfig)

	var baseNodes []string
	if raw, ok := robotConfig["base"]; ok {
		json.Unmarshal(raw, &baseNodes)
	}

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
