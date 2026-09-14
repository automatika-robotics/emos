package runner

import (
	"fmt"
	"io"
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
	return nil
}

func (s *NativeStrategy) StartRecipe(recipeName string, out io.Writer) (*RunHandle, error) {
	return startRecipe(s, recipeName, out)
}

func (s *NativeStrategy) Cleanup() error {
	return nil
}
