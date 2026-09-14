package runner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/ui"
)

// NativeStrategy handles recipe execution directly on the host (no container).
// EMOS packages are installed directly into /opt/ros/{distro}/, so only the
// ROS setup.bash needs to be sourced.
type NativeStrategy struct {
	rosDistro string
	env       []string // added to the environment of every command
}

func NewNativeStrategy(env []string) *NativeStrategy {
	distro := "jazzy"
	if cfg := config.LoadConfig(); cfg != nil && cfg.ROSDistro != "" {
		distro = cfg.ROSDistro
	}
	return &NativeStrategy{rosDistro: distro, env: env}
}

func (s *NativeStrategy) sourceCmd() string {
	cmd := "source " + filepath.Join("/opt/ros", s.rosDistro, "setup.bash")
	// Source the robot-plugin overlay after the stack so recipes can import
	// the active plugin. Absent until a plugin is installed.
	overlay := filepath.Join(config.PluginOverlayDir(), "setup.bash")
	if _, err := os.Stat(overlay); err == nil {
		cmd += " && source " + overlay
	}
	return cmd
}

func (s *NativeStrategy) Command(shell string) *exec.Cmd {
	cmd := exec.Command("bash", "-c", s.sourceCmd()+" && "+shell)
	cmd.Env = append(os.Environ(), s.env...)
	return cmd
}

func (s *NativeStrategy) RecipesDir() string { return config.RecipesDir }

func (s *NativeStrategy) PrepareEnvironment() error {
	ui.Header("HOST ENVIRONMENT SETUP")

	rosSetup := filepath.Join("/opt/ros", s.rosDistro, "setup.bash")
	if _, err := os.Stat(rosSetup); err != nil {
		return fmt.Errorf("ROS 2 %s not found at /opt/ros/%s — is it installed?", s.rosDistro, s.rosDistro)
	}
	ui.Success(fmt.Sprintf("ROS 2 %s found.", s.rosDistro))

	// Quick check that EMOS packages are importable
	if err := s.Command("python3 -c 'import agents' 2>/dev/null").Run(); err != nil {
		ui.Warn("EMOS packages may not be installed. Run 'emos install --mode native' first.")
	} else {
		ui.Success("EMOS packages available.")
	}

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
		return s.Command("ros2 launch " + bringup + " &").Start()
	})
}

func (s *NativeStrategy) ExecRecipe(recipeName string, logFile string) error {
	ui.Header("LAUNCHING RECIPE: " + recipeName)
	ui.Info("All output will be saved to: " + logFile)
	ui.Success("BEGIN RECIPE OUTPUT")
	fmt.Println()

	recipePath := filepath.Join(config.RecipesDir, recipeName, "recipe.py")
	cmd := s.Command(fmt.Sprintf("python3 -u %s 2>&1 | tee %s", recipePath, logFile))
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// StartRecipe launches the recipe non-blocking and writes output only to the
// log file (no terminal binding). The returned handle is what the daemon
// stores to track + cancel the run.
func (s *NativeStrategy) StartRecipe(recipeName string, logFile string) (*RunHandle, error) {
	recipePath := filepath.Join(config.RecipesDir, recipeName, "recipe.py")
	cmd := s.Command(fmt.Sprintf("exec python3 -u %s >> %s 2>&1", recipePath, logFile))
	if err := os.MkdirAll(parentDir(logFile), 0755); err != nil {
		return nil, err
	}
	h, err := StartProcess(cmd, logFile)
	if err != nil {
		return nil, fmt.Errorf("start recipe: %w", err)
	}
	return h, nil
}

func (s *NativeStrategy) Cleanup() error {
	return nil
}
