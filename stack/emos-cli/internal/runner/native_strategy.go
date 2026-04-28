package runner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/ui"
)

// NativeStrategy handles recipe execution directly on the host (no container).
// EMOS packages are installed directly into /opt/ros/{distro}/, so only the
// ROS setup.bash needs to be sourced.
type NativeStrategy struct {
	rosDistro string
}

func NewNativeStrategy() *NativeStrategy {
	distro := "jazzy"
	if cfg := config.LoadConfig(); cfg != nil && cfg.ROSDistro != "" {
		distro = cfg.ROSDistro
	}
	return &NativeStrategy{rosDistro: distro}
}

func (s *NativeStrategy) sourceCmd() string {
	return "source " + filepath.Join("/opt/ros", s.rosDistro, "setup.bash")
}

func (s *NativeStrategy) PrepareEnvironment() error {
	ui.Header("HOST ENVIRONMENT SETUP")

	rosSetup := filepath.Join("/opt/ros", s.rosDistro, "setup.bash")
	if _, err := os.Stat(rosSetup); err != nil {
		return fmt.Errorf("ROS 2 %s not found at /opt/ros/%s — is it installed?", s.rosDistro, s.rosDistro)
	}
	ui.Success(fmt.Sprintf("ROS 2 %s found.", s.rosDistro))

	// Quick check that EMOS packages are importable
	checkCmd := exec.Command("bash", "-c", s.sourceCmd()+" && python3 -c 'import agents' 2>/dev/null")
	if err := checkCmd.Run(); err != nil {
		ui.Warn("EMOS packages may not be installed. Run 'emos install --mode native' first.")
	} else {
		ui.Success("EMOS packages available.")
	}

	return nil
}

func (s *NativeStrategy) SetRMWImpl(rmw string) error {
	ui.Header("RMW CONFIGURATION")
	ui.Info("Setting RMW_IMPLEMENTATION=" + rmw)
	os.Setenv("RMW_IMPLEMENTATION", rmw)
	return nil
}

func (s *NativeStrategy) ConfigureZenoh(recipeName string, manifest *recipeManifest) error {
	if manifest.ZenohRouterConfig != "" {
		configPath := filepath.Join(config.RecipesDir, manifest.ZenohRouterConfig)
		if _, err := os.Stat(configPath); err == nil {
			ui.Info("Using Zenoh router config: " + configPath)
			os.Setenv("ZENOH_ROUTER_CONFIG_URI", configPath)
		} else {
			ui.Warn("Zenoh config file not found — using default")
		}
	}

	ui.Spinner("Starting zenoh router...", func() error {
		cmd := exec.Command("bash", "-c", s.sourceCmd()+" && ros2 run rmw_zenoh_cpp rmw_zenohd &")
		return cmd.Start()
	})
	time.Sleep(2 * time.Second)
	return nil
}

func (s *NativeStrategy) LaunchRobotHardware() error {
	ui.Header("HARDWARE & SENSOR LAUNCH")

	bringup := filepath.Join(config.HomeDir, "emos", "robot", "launch", "bringup_robot.py")
	if _, err := os.Stat(bringup); err != nil {
		ui.Info("Native mode: no robot bringup found. Ensure hardware drivers are running.")
		return nil
	}

	return ui.Spinner("Launching robot base hardware...", func() error {
		cmd := exec.Command("bash", "-c", s.sourceCmd()+" && ros2 launch "+bringup+" &")
		return cmd.Start()
	})
}

func (s *NativeStrategy) VerifySensorTopics(sensors []ExtractedTopic, distro string) error {
	ui.Header("VERIFYING SENSOR TOPICS")
	time.Sleep(5 * time.Second)

	src := s.sourceCmd()
	checker := func() (string, error) {
		out, err := exec.Command("bash", "-c", src+" && ros2 topic list").CombinedOutput()
		return string(out), err
	}
	return verifySensorTopicsAST(sensors, checker, distro)
}

func (s *NativeStrategy) ExecRecipe(recipeName string, manifest *recipeManifest, logFile string) error {
	ui.Header("LAUNCHING RECIPE: " + recipeName)
	ui.Info("All output will be saved to: " + logFile)
	ui.Success("BEGIN RECIPE OUTPUT")
	fmt.Println()

	recipePath := filepath.Join(config.RecipesDir, recipeName, "recipe.py")
	shellCmd := fmt.Sprintf("%s && python3 -u %s 2>&1 | tee %s", s.sourceCmd(), recipePath, logFile)
	cmd := exec.Command("bash", "-c", shellCmd)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// StartRecipe launches the recipe non-blocking and writes output only to the
// log file (no terminal binding). The returned handle is what the daemon
// stores to track + cancel the run.
func (s *NativeStrategy) StartRecipe(recipeName string, manifest *recipeManifest, logFile string) (*RunHandle, error) {
	recipePath := filepath.Join(config.RecipesDir, recipeName, "recipe.py")
	shellCmd := fmt.Sprintf("%s && exec python3 -u %s >> %s 2>&1", s.sourceCmd(), recipePath, logFile)
	cmd := exec.Command("bash", "-c", shellCmd)
	if err := os.MkdirAll(parentDir(logFile), 0755); err != nil {
		return nil, err
	}
	return startCmd(cmd, logFile)
}

func (s *NativeStrategy) Cleanup() error {
	ui.Info("Native mode: no container cleanup needed.")
	return nil
}
