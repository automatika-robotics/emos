package cmd

import (
	"fmt"
	"os"

	"github.com/automatika-robotics/emos-cli/internal/api"
	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/container"
	"github.com/automatika-robotics/emos-cli/internal/installer"
	"github.com/automatika-robotics/emos-cli/internal/ui"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update EMOS to the latest version",
	RunE:  runUpdate,
}

func runUpdate(cmd *cobra.Command, args []string) error {
	ui.Banner(config.Version)

	cfg := config.LoadConfig()
	if cfg == nil {
		ui.Error("No existing installation found.")
		fmt.Println("  Please run 'emos install' first.")
		return fmt.Errorf("no installation config")
	}

	ui.Info("Current mode: " + string(cfg.Mode))
	fmt.Println()

	switch cfg.Mode {
	case config.ModeOSSContainer:
		return updateOSSContainer(cfg)
	case config.ModeLicensed:
		return updateLicensed(cfg)
	case config.ModeNative:
		return updateNative(cfg)
	default:
		return fmt.Errorf("unknown mode: %s", cfg.Mode)
	}
}

func updateOSSContainer(cfg *config.EMOSConfig) error {
	image := config.PublicImageTag(cfg.ROSDistro)
	if cfg.ImageTag != "" {
		image = cfg.ImageTag
	}

	fmt.Println("  Checking for EMOS container updates...")
	fmt.Println()

	// Pull latest image
	ui.Info("Pulling latest image: " + image)
	if err := container.Pull(image); err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}
	ui.Success("Pulled latest image.")

	// Recreate container
	if container.Exists(config.ContainerName) {
		if err := ui.Spinner("Removing existing container...", func() error {
			return container.Remove(config.ContainerName)
		}); err != nil {
			return fmt.Errorf("failed to remove container: %w", err)
		}
	}

	if err := ui.Spinner("Starting updated EMOS container...", func() error {
		return container.Run(config.ContainerName, image)
	}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	fmt.Println()
	ui.SuccessBox("EMOS container updated successfully!")
	return nil
}

func updateLicensed(cfg *config.EMOSConfig) error {
	licenseKey := cfg.LicenseKey
	if licenseKey == "" {
		// Fallback to license file
		keyBytes, err := os.ReadFile(config.LicenseFile)
		if err != nil {
			ui.Error("No license key found.")
			return fmt.Errorf("no license key")
		}
		licenseKey = string(keyBytes)
	}

	fmt.Println("  Checking for EmbodiedOS container updates...")
	fmt.Println()

	var creds *api.Credentials
	err := ui.Spinner("Verifying license...", func() error {
		var e error
		creds, e = api.ValidateLicense(licenseKey)
		return e
	})
	if err != nil {
		return err
	}

	if container.Exists(config.ContainerName) {
		if err := ui.Spinner("Removing existing container...", func() error {
			return container.Remove(config.ContainerName)
		}); err != nil {
			return fmt.Errorf("failed to remove container: %w", err)
		}
	}

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

func updateNative(cfg *config.EMOSConfig) error {
	wsPath := cfg.WorkspacePath
	if wsPath == "" {
		ui.Error("No workspace path configured.")
		return fmt.Errorf("workspace path not set")
	}

	fmt.Println("  Updating EMOS native workspace...")
	fmt.Println()

	if err := installer.UpdateNativeWorkspace(wsPath, cfg.ROSDistro); err != nil {
		return err
	}

	fmt.Println()
	ui.SuccessBox("EMOS native workspace updated successfully!")
	return nil
}
