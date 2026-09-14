package runner

import (
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/container"
	"github.com/automatika-robotics/emos-cli/internal/ui"
)

// ContainerStrategy handles recipe execution inside a Docker container.
// It supports both oss-container (licensed=false) and licensed (licensed=true) modes.
type ContainerStrategy struct {
	licensed bool
	env      []string // exported in every command run in the container
}

func NewContainerStrategy(licensed bool, env []string) *ContainerStrategy {
	return &ContainerStrategy{licensed: licensed, env: env}
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

// shell prefixes command with the container's ROS environment.
func (s *ContainerStrategy) shell(command string) string {
	parts := []string{"source ros_entrypoint.sh"}
	for _, kv := range s.env {
		parts = append(parts, "export "+shellQuote(kv))
	}
	return strings.Join(append(parts, command), " && ")
}

// Command runs shell in the container through a docker exec on the host.
func (s *ContainerStrategy) Command(shell string) *exec.Cmd {
	return exec.Command("docker", "exec", config.ContainerName, "bash", "-c", s.shell(shell))
}

func (s *ContainerStrategy) RecipesDir() string { return recipesRoot }

func (s *ContainerStrategy) LaunchRobotHardware() error {
	if !s.licensed {
		return nil
	}

	ui.Header("HARDWARE & SENSOR LAUNCH")
	return ui.Spinner("Launching robot base hardware...", func() error {
		return container.ExecDetached(config.ContainerName,
			s.shell("ros2 launch "+emosRoot+"/robot/launch/bringup_robot.py"))
	})
}

// StartRecipe runs the recipe through a docker exec on the host, which is
// what writes its output, so the log lands on the host.
func (s *ContainerStrategy) StartRecipe(recipeName string, out io.Writer) (*RunHandle, error) {
	h, err := startRecipe(s, recipeName, out)
	if err != nil {
		return nil, err
	}
	h.container = config.ContainerName
	h.killTarget = filepath.Join(recipesRoot, recipeName, "recipe.py")
	return h, nil
}

func (s *ContainerStrategy) Cleanup() error {
	ui.Spinner("EMOS container cleanup...", func() error {
		return container.Stop(config.ContainerName)
	})
	return nil
}
