package cmd

import (
	"fmt"
	"os/exec"

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
			if _, err := exec.LookPath("ros2"); err == nil {
				ui.Success("ROS 2: Available")
			} else {
				ui.Error("ROS 2: Not found in PATH")
			}
			if cfg.WorkspacePath != "" {
				ui.Info("Workspace: " + cfg.WorkspacePath)
			}
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
