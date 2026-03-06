package cmd

import (
	"fmt"
	"os"

	"github.com/automatika-robotics/emos-cli/internal/api"
	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/container"
	"github.com/automatika-robotics/emos-cli/internal/ui"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update the EMOS container to the latest version",
	RunE:  runUpdate,
}

func runUpdate(cmd *cobra.Command, args []string) error {
	ui.Banner(config.Version)

	// Check for license file
	keyBytes, err := os.ReadFile(config.LicenseFile)
	if err != nil {
		ui.Error("No existing installation found.")
		fmt.Println("  Please run 'emos install <license_key>' first.")
		return fmt.Errorf("no license file")
	}
	licenseKey := string(keyBytes)

	fmt.Println("  Checking for EmbodiedOS container updates...")
	fmt.Println()

	// Validate license
	var creds *api.Credentials
	err = ui.Spinner("Verifying license...", func() error {
		var e error
		creds, e = api.ValidateLicense(licenseKey)
		return e
	})
	if err != nil {
		return err
	}

	// Remove existing container
	if container.Exists(config.ContainerName) {
		err := ui.Spinner("Removing existing container...", func() error {
			return container.Remove(config.ContainerName)
		})
		if err != nil {
			return err
		}
	}

	// Deploy robot files and container
	if err := deployRobotFiles(creds); err != nil {
		return err
	}
	if err := deployContainer(creds); err != nil {
		return err
	}

	fmt.Println()
	ui.SuccessBox("EmbodiedOS container updated successfully!")
	return nil
}
