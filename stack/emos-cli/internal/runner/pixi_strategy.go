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

// PixiStrategy handles recipe execution inside a pixi-managed environment.
// EMOS packages are built via colcon inside the pixi env; the colcon install/
// setup.sh must be sourced before running ROS commands.
type PixiStrategy struct {
	projectDir string
	extraEnv   []string
}

func NewPixiStrategy(projectDir string) *PixiStrategy {
	return &PixiStrategy{projectDir: projectDir}
}

// pixiRun executes a command inside the pixi environment, with the
// strategy's per-run env additions stamped onto the resulting *exec.Cmd.
func (s *PixiStrategy) pixiRun(shellCmd string) *exec.Cmd {
	cmd := exec.Command("pixi", "run", "--manifest-path",
		filepath.Join(s.projectDir, "pixi.toml"),
		"bash", "-c", shellCmd)
	cmd.Dir = s.projectDir
	cmd.Env = append(os.Environ(), s.extraEnv...)
	return cmd
}

// sourceCmd returns the shell snippet that sources the colcon install overlay.
func (s *PixiStrategy) sourceCmd() string {
	return "source " + filepath.Join(s.projectDir, "install", "setup.sh")
}

func (s *PixiStrategy) PrepareEnvironment() error {
	ui.Header("PIXI ENVIRONMENT SETUP")

	// Check pixi binary
	if _, err := exec.LookPath("pixi"); err != nil {
		return fmt.Errorf("pixi not found in PATH — install it from https://pixi.sh")
	}
	ui.Success("pixi binary found.")

	// Check project dir and pixi.toml
	pixiToml := filepath.Join(s.projectDir, "pixi.toml")
	if _, err := os.Stat(pixiToml); err != nil {
		return fmt.Errorf("pixi.toml not found at %s", pixiToml)
	}
	ui.Success("pixi project: " + s.projectDir)

	// Verify EMOS packages are importable
	cmd := s.pixiRun(s.sourceCmd() + " && python3 -c 'import agents' 2>/dev/null")
	if err := cmd.Run(); err != nil {
		ui.Warn("EMOS packages may not be built. Run 'pixi run setup' in " + s.projectDir)
	} else {
		ui.Success("EMOS packages available.")
	}

	return nil
}

func (s *PixiStrategy) SetRMWImpl(rmw string) error {
	ui.Header("RMW CONFIGURATION")
	ui.Info("Setting RMW_IMPLEMENTATION=" + rmw)
	s.extraEnv = append(s.extraEnv, "RMW_IMPLEMENTATION="+rmw)
	return nil
}

func (s *PixiStrategy) ConfigureZenoh(recipeName string, manifest *recipeManifest) error {
	if manifest.ZenohRouterConfig != "" {
		configPath := filepath.Join(config.RecipesDir, manifest.ZenohRouterConfig)
		if _, err := os.Stat(configPath); err == nil {
			ui.Info("Using Zenoh router config: " + configPath)
			s.extraEnv = append(s.extraEnv, "ZENOH_ROUTER_CONFIG_URI="+configPath)
		} else {
			ui.Warn("Zenoh config file not found — using default")
		}
	}

	ui.Spinner("Starting zenoh router...", func() error {
		cmd := s.pixiRun(s.sourceCmd() + " && ros2 run rmw_zenoh_cpp rmw_zenohd &")
		return cmd.Start()
	})
	time.Sleep(2 * time.Second)
	return nil
}

func (s *PixiStrategy) LaunchRobotHardware() error {
	ui.Header("HARDWARE & SENSOR LAUNCH")

	bringup := filepath.Join(config.HomeDir, "emos", "robot", "launch", "bringup_robot.py")
	if _, err := os.Stat(bringup); err != nil {
		ui.Info("Pixi mode: no robot bringup found. Ensure hardware drivers are running.")
		return nil
	}

	return ui.Spinner("Launching robot base hardware...", func() error {
		cmd := s.pixiRun(s.sourceCmd() + " && ros2 launch " + bringup + " &")
		return cmd.Start()
	})
}

func (s *PixiStrategy) VerifySensorTopics(sensors []ExtractedTopic, distro string) error {
	ui.Header("VERIFYING SENSOR TOPICS")
	time.Sleep(5 * time.Second)

	checker := func() (string, error) {
		cmd := s.pixiRun(s.sourceCmd() + " && ros2 topic list")
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	return verifySensorTopicsAST(sensors, checker, distro)
}

func (s *PixiStrategy) ExecRecipe(recipeName string, manifest *recipeManifest, logFile string) error {
	ui.Header("LAUNCHING RECIPE: " + recipeName)
	ui.Info("All output will be saved to: " + logFile)
	ui.Success("BEGIN RECIPE OUTPUT")
	fmt.Println()

	recipePath := filepath.Join(config.RecipesDir, recipeName, "recipe.py")
	shellCmd := fmt.Sprintf("%s && python3 -u %s 2>&1 | tee %s", s.sourceCmd(), recipePath, logFile)
	cmd := s.pixiRun(shellCmd)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// StartRecipe launches the recipe inside the pixi env non-blocking. Output is
// redirected to the log file; the daemon tails it via SSE.
func (s *PixiStrategy) StartRecipe(recipeName string, manifest *recipeManifest, logFile string) (*RunHandle, error) {
	recipePath := filepath.Join(config.RecipesDir, recipeName, "recipe.py")
	shellCmd := fmt.Sprintf("%s && exec python3 -u %s >> %s 2>&1", s.sourceCmd(), recipePath, logFile)
	cmd := s.pixiRun(shellCmd)
	if err := os.MkdirAll(parentDir(logFile), 0755); err != nil {
		return nil, err
	}
	return startCmd(cmd, logFile)
}

func (s *PixiStrategy) Cleanup() error {
	ui.Info("Pixi mode: no cleanup needed.")
	return nil
}
