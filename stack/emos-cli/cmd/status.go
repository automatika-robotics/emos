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
		if cfg == nil {
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

	// Python package checks
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
		importCmd := fmt.Sprintf("source %s && python3 -c 'import %s' 2>&1", rosSetup, m.module)
		if err := exec.Command("bash", "-c", importCmd).Run(); err != nil {
			ui.Error(fmt.Sprintf("  %s: Not installed", m.display))
		} else {
			ui.Success(fmt.Sprintf("  %s: OK", m.display))
		}
	}

	// ROS package checks
	fmt.Println()
	ui.Info("ROS Packages:")
	listCmd := fmt.Sprintf("source %s && ros2 pkg list 2>/dev/null", rosSetup)
	out, err := exec.Command("bash", "-c", listCmd).Output()
	if err != nil {
		ui.Warn("  Could not list ROS packages")
		return
	}

	pkgList := string(out)
	rosPkgs := []struct {
		name    string
		display string
	}{
		{"automatika_ros_sugar", "automatika_ros_sugar"},
		{"automatika_embodied_agents", "automatika_embodied_agents"},
		{"kompass", "kompass"},
		{"kompass_interfaces", "kompass_interfaces"},
	}

	for _, p := range rosPkgs {
		if strings.Contains(pkgList, p.name) {
			ui.Success(fmt.Sprintf("  %s: OK", p.display))
		} else {
			ui.Error(fmt.Sprintf("  %s: Not found", p.display))
		}
	}
}
