package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/container"
	"github.com/automatika-robotics/emos-cli/internal/ui"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Display EMOS installation status",
	Run: func(cmd *cobra.Command, args []string) {
		ui.Banner(config.Version)
		ui.StatusCard(config.Version)

		cfg := config.LoadConfig()
		if !cfg.IsInstalled() {
			fmt.Println()
			ui.Error("No EMOS installation found.")
			ui.Faint("Run 'emos install' to get started.")
			return
		}

		fmt.Println()
		ui.Info("Mode: " + string(cfg.Mode))
		ui.Info("ROS Distro: " + cfg.ROSDistro)

		switch cfg.Mode {
		case config.ModeOSSContainer, config.ModeLicensed:
			status := container.Status(config.ContainerName)
			switch status {
			case "running":
				ui.Success(fmt.Sprintf("Container '%s': Running", config.ContainerName))
			case "exited":
				ui.Warn(fmt.Sprintf("Container '%s': Exited", config.ContainerName))
			default:
				ui.Error(fmt.Sprintf("Container '%s': Not Found", config.ContainerName))
			}
			if cfg.ImageTag != "" {
				ui.Info("Image: " + cfg.ImageTag)
			}

		case config.ModeNative:
			nativeStatus(cfg)

		case config.ModePixi:
			pixiStatus(cfg)
		}

		if cfg.Mode == config.ModeLicensed {
			if cfg.LicenseKey != "" {
				ui.Success("License Key: Configured")
			} else {
				ui.Warn("License Key: Not set")
			}
		}
	},
}

func pixiStatus(cfg *config.EMOSConfig) {
	projectDir := cfg.PixiProjectDir
	if projectDir == "" {
		ui.Error("No pixi project directory configured.")
		return
	}

	// pixi binary
	if _, err := exec.LookPath("pixi"); err != nil {
		ui.Error("pixi: Not found in PATH")
	} else {
		ui.Success("pixi: Available")
	}

	// Project dir
	pixiToml := filepath.Join(projectDir, "pixi.toml")
	if _, err := os.Stat(pixiToml); err != nil {
		ui.Error("pixi project: Not found at " + projectDir)
		return
	}
	ui.Success("pixi project: " + projectDir)

	// Helper to run a shell command inside the pixi environment
	setupSh := filepath.Join(projectDir, "install", "setup.sh")
	pixiShell := func(shellCmd string) *exec.Cmd {
		cmd := exec.Command("pixi", "run", "--manifest-path", pixiToml, "bash", "-c", shellCmd)
		cmd.Dir = projectDir
		return cmd
	}

	checkPackages(
		func(module string) error {
			return pixiShell(fmt.Sprintf("source %s && python3 -c 'import %s' 2>&1", setupSh, module)).Run()
		},
		func() (string, error) {
			out, err := pixiShell(fmt.Sprintf("source %s && ros2 pkg list 2>/dev/null", setupSh)).Output()
			return string(out), err
		},
	)
}

func nativeStatus(cfg *config.EMOSConfig) {
	rosPath := filepath.Join("/opt/ros", cfg.ROSDistro)
	rosSetup := filepath.Join(rosPath, "setup.bash")

	// ROS 2 availability
	if _, err := os.Stat(rosSetup); err == nil {
		ui.Success(fmt.Sprintf("ROS 2 %s: Installed at %s", capitalize(cfg.ROSDistro), rosPath))
	} else {
		ui.Error(fmt.Sprintf("ROS 2 %s: Not found at %s", capitalize(cfg.ROSDistro), rosPath))
		return
	}

	checkPackages(
		func(module string) error {
			return exec.Command("bash", "-c", fmt.Sprintf("source %s && python3 -c 'import %s' 2>&1", rosSetup, module)).Run()
		},
		func() (string, error) {
			out, err := exec.Command("bash", "-c", fmt.Sprintf("source %s && ros2 pkg list 2>/dev/null", rosSetup)).Output()
			return string(out), err
		},
	)
}

// checkPackages verifies EMOS Python and ROS packages using the provided
// import checker and package lister functions.
func checkPackages(tryImport func(module string) error, listROSPkgs func() (string, error)) {
	fmt.Println()
	ui.Info("Python Packages:")
	pyModules := []struct {
		module  string
		display string
	}{
		{"ros_sugar", "ros_sugar (Sugarcoat)"},
		{"agents", "agents (Embodied Agents)"},
		{"kompass", "kompass"},
		{"kompass_core", "kompass_core"},
	}

	for _, m := range pyModules {
		if err := tryImport(m.module); err != nil {
			ui.Error(fmt.Sprintf("  %s: Not installed", m.display))
		} else {
			ui.Success(fmt.Sprintf("  %s: OK", m.display))
		}
	}

	fmt.Println()
	ui.Info("ROS Packages:")
	out, err := listROSPkgs()
	if err != nil {
		ui.Warn("  Could not list ROS packages")
		return
	}

	rosPkgs := []string{
		"automatika_ros_sugar",
		"automatika_embodied_agents",
		"kompass",
		"kompass_interfaces",
	}

	for _, name := range rosPkgs {
		if strings.Contains(out, name) {
			ui.Success(fmt.Sprintf("  %s: OK", name))
		} else {
			ui.Error(fmt.Sprintf("  %s: Not found", name))
		}
	}
}
