package runner

import (
	"errors"
	"fmt"
	"io"
	"os/exec"

	"github.com/automatika-robotics/emos-cli/internal/config"
)

// RuntimeStrategy defines the interface for mode-specific recipe execution.
type RuntimeStrategy interface {
	PrepareEnvironment() error
	// Command returns a process that runs shell in the mode's ROS environment.
	Command(shell string) *exec.Cmd
	// RecipesDir is the recipes directory as a Command sees it.
	RecipesDir() string
	// StartRecipe starts the recipe in its own process group, writing its
	// output to out, and returns a handle to wait on or stop it.
	StartRecipe(recipeName string, out io.Writer) (*RunHandle, error)
	Cleanup() error
}

var errNotInstalled = errors.New("no EMOS installation found — run 'emos install' first")

// newStrategy returns the strategy for cfg's install mode. rmw is set as the
// RMW implementation of everything the run starts; empty leaves the
// environment's. The recipe UI's state directory and the robot's certificate
// are set in every mode.
func newStrategy(cfg *config.EMOSConfig, rmw string) (RuntimeStrategy, error) {
	if !cfg.IsInstalled() {
		return nil, errNotInstalled
	}
	var env []string
	if rmw != "" {
		env = append(env, "RMW_IMPLEMENTATION="+rmw)
	}
	uiDir := config.UISecurityDir
	if cfg.Mode == config.ModeOSSContainer {
		uiDir = uiSecurityRoot
	}
	uiEnv, err := uiEnv(uiDir)
	if err != nil {
		return nil, err
	}
	env = append(env, uiEnv...)
	switch cfg.Mode {
	case config.ModeOSSContainer:
		return NewContainerStrategy(env), nil
	case config.ModeNative:
		return NewNativeStrategy(env), nil
	case config.ModePixi:
		return NewPixiStrategy(cfg.PixiProjectDir, env), nil
	}
	return nil, fmt.Errorf("unknown install mode: %s", cfg.Mode)
}
