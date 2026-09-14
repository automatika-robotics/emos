package runner

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/installer"
	"github.com/automatika-robotics/emos-cli/internal/ui"
)

// PixiStrategy handles recipe execution inside a pixi-managed environment.
// EMOS packages are built via colcon inside the pixi env; the colcon install/
// setup.sh must be sourced before running ROS commands.
type PixiStrategy struct {
	projectDir string
	pixiBin    string   // resolved absolute path to the pixi binary;
	env        []string // added to the environment of every command
}

func NewPixiStrategy(projectDir string, env []string) *PixiStrategy {
	return &PixiStrategy{projectDir: projectDir, env: env}
}

// ensurePixi populates s.pixiBin once on first use. Idempotent. Pixi
// resolution lives in internal/installer so the install/update/run paths
// agree on where the binary is.
func (s *PixiStrategy) ensurePixi() error {
	if s.pixiBin != "" {
		return nil
	}
	bin, err := installer.ResolvePixi()
	if err != nil {
		return err
	}
	s.pixiBin = bin
	return nil
}

// Command runs shell inside the pixi environment, with the colcon overlays
// sourced.
func (s *PixiStrategy) Command(shell string) *exec.Cmd {
	_ = s.ensurePixi() // resolution errors are surfaced by PrepareEnvironment
	bin := s.pixiBin
	if bin == "" {
		// Fall through with the bare name so exec produces a clear
		// "executable file not found" rather than a panic. Should be
		// unreachable in normal flow because PrepareEnvironment runs first.
		bin = "pixi"
	}
	cmd := exec.Command(bin, "run", "--manifest-path",
		filepath.Join(s.projectDir, "pixi.toml"),
		"bash", "-c", s.sourceCmd()+" && "+shell)
	cmd.Dir = s.projectDir
	cmd.Env = append(os.Environ(), s.env...)
	return cmd
}

func (s *PixiStrategy) RecipesDir() string { return config.RecipesDir }

// sourceCmd returns the shell snippet that sources the colcon install overlay,
// plus the robot-plugin overlay when a plugin is installed.
func (s *PixiStrategy) sourceCmd() string {
	cmd := "source " + filepath.Join(s.projectDir, "install", "setup.sh")
	overlay := filepath.Join(config.PluginOverlayDir(), "setup.sh")
	if _, err := os.Stat(overlay); err == nil {
		cmd += " && source " + overlay
	}
	return cmd
}

func (s *PixiStrategy) PrepareEnvironment() error {
	ui.Header("PIXI ENVIRONMENT SETUP")

	if err := s.ensurePixi(); err != nil {
		return err
	}
	ui.Success("pixi binary: " + s.pixiBin)

	// Check project dir and pixi.toml
	pixiToml := filepath.Join(s.projectDir, "pixi.toml")
	if _, err := os.Stat(pixiToml); err != nil {
		return fmt.Errorf("pixi.toml not found at %s", pixiToml)
	}
	ui.Success("pixi project: " + s.projectDir)

	// Verify EMOS packages are importable
	if err := s.Command("python3 -c 'import agents' 2>/dev/null").Run(); err != nil {
		ui.Warn("EMOS packages may not be built. Run 'pixi run setup' in " + s.projectDir)
	} else {
		ui.Success("EMOS packages available.")
	}

	return nil
}

func (s *PixiStrategy) LaunchRobotHardware() error {
	return nil
}

func (s *PixiStrategy) StartRecipe(recipeName string, out io.Writer) (*RunHandle, error) {
	return startRecipe(s, recipeName, out)
}

func (s *PixiStrategy) Cleanup() error {
	return nil
}
