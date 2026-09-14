package runner

import (
	"errors"
	"fmt"
	"os/exec"

	"github.com/automatika-robotics/emos-cli/internal/config"
)

// RuntimeStrategy defines the interface for mode-specific recipe execution.
//
// ExecRecipe runs the recipe synchronously (used by the CLI `emos run` path).
// StartRecipe starts the recipe in the background and returns a handle the
// caller can Wait on or Cancel (used by the `emos serve` daemon).
type RuntimeStrategy interface {
	PrepareEnvironment() error
	// Command returns a process that runs shell in the mode's ROS environment.
	Command(shell string) *exec.Cmd
	// RecipesDir is the recipes directory as a Command sees it.
	RecipesDir() string
	LaunchRobotHardware() error
	ExecRecipe(recipeName string, logFile string) error
	StartRecipe(recipeName string, logFile string) (*RunHandle, error)
	Cleanup() error
}

// NewStrategy returns the strategy for cfg's install mode. rmw is set as the
// RMW implementation of everything the run starts; empty leaves the
// environment's.
func NewStrategy(cfg *config.EMOSConfig, rmw string) (RuntimeStrategy, error) {
	if !cfg.IsInstalled() {
		return nil, errors.New("no EMOS installation found — run 'emos install' first")
	}
	var env []string
	if rmw != "" {
		env = append(env, "RMW_IMPLEMENTATION="+rmw)
	}
	switch cfg.Mode {
	case config.ModeOSSContainer:
		return NewContainerStrategy(false, env), nil
	case config.ModeLicensed:
		return NewContainerStrategy(true, env), nil
	case config.ModeNative:
		return NewNativeStrategy(env), nil
	case config.ModePixi:
		return NewPixiStrategy(cfg.PixiProjectDir, env), nil
	}
	return nil, fmt.Errorf("unknown install mode: %s", cfg.Mode)
}
