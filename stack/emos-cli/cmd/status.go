package cmd

import (
	"fmt"
	"os"

	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/container"
	"github.com/automatika-robotics/emos-cli/internal/ui"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Display EMOS container and license status",
	Run: func(cmd *cobra.Command, args []string) {
		ui.Banner(config.Version)
		fmt.Println("  EMOS is a self-contained automation layer for your robot.")
		fmt.Println("  This tool helps you manage its lifecycle on this machine.")
		fmt.Println("  Developed by Automatika Robotics.")
		fmt.Println()

		status := container.Status(config.ContainerName)
		switch status {
		case "running":
			ui.Success(fmt.Sprintf("Container '%s': Running", config.ContainerName))
		case "exited":
			ui.Warn(fmt.Sprintf("Container '%s': Exited", config.ContainerName))
		default:
			ui.Error(fmt.Sprintf("Container '%s': Not Found", config.ContainerName))
		}

		if _, err := os.Stat(config.LicenseFile); err == nil {
			ui.Success("License Key: Found")
		} else {
			ui.Error("License Key: Not Found")
		}
	},
}
