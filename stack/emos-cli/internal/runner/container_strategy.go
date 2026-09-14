package runner

import (
	"fmt"
	"os"
	"os/exec"
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
	ui.Header("HARDWARE & SENSOR LAUNCH")

	if !s.licensed {
		ui.Info("OSS container mode: skipping robot hardware launch.")
		ui.Faint("Ensure your robot hardware drivers are running externally.")
		return nil
	}

	return ui.Spinner("Launching robot base hardware...", func() error {
		return container.ExecDetached(config.ContainerName,
			s.shell("ros2 launch "+emosRoot+"/robot/launch/bringup_robot.py"))
	})
}

func (s *ContainerStrategy) ExecRecipe(recipeName string, logFile string) error {
	ui.Header("LAUNCHING RECIPE: " + recipeName)
	ui.Info("All output will be saved to: " + logFile)
	ui.Success("BEGIN RECIPE OUTPUT")
	fmt.Println()

	recipeCmd := s.shell(fmt.Sprintf("python3 -u %s/%s/recipe.py | tee %s",
		recipesRoot, recipeName, logFile))
	return container.ExecInteractive(config.ContainerName, recipeCmd)
}

// StartRecipe runs the recipe inside the container non-blocking. The bash
// process we spawn here is host-side (`docker exec`); its captured stdout
// goes to the host log file so the SSE log tail works without bind mounts.
func (s *ContainerStrategy) StartRecipe(recipeName string, logFile string) (*RunHandle, error) {
	recipePath := fmt.Sprintf("%s/%s/recipe.py", recipesRoot, recipeName)
	recipeCmd := s.shell("exec python3 -u " + recipePath)
	if err := os.MkdirAll(parentDir(logFile), 0755); err != nil {
		return nil, err
	}
	return startContainerExec(config.ContainerName, recipeCmd, logFile, recipePath)
}

func (s *ContainerStrategy) Cleanup() error {
	ui.Spinner("EMOS container cleanup...", func() error {
		return container.Stop(config.ContainerName)
	})
	return nil
}
