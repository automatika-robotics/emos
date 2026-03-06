package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/automatika-robotics/emos-cli/internal/api"
	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/container"
	"github.com/automatika-robotics/emos-cli/internal/ui"
	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install <license-key>",
	Short: "Install and start EMOS using a license key",
	Args:  cobra.ExactArgs(1),
	RunE:  runInstall,
}

func runInstall(cmd *cobra.Command, args []string) error {
	licenseKey := args[0]

	// Check for existing installation
	if container.Exists(config.ContainerName) {
		ui.Warn("An existing EmbodiedOS container was found.")
		if !ui.Confirm("This will REMOVE the existing container and perform a fresh installation. Are you sure?") {
			ui.Error("Installation aborted.")
			return fmt.Errorf("aborted by user")
		}
	}

	ui.Banner(config.Version)
	fmt.Println("  Starting EmbodiedOS installation...")
	fmt.Println()

	os.MkdirAll(config.ConfigDir, 0755)

	// Validate license
	var creds *api.Credentials
	err := ui.Spinner("Validating license key...", func() error {
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

	// Save license key
	os.WriteFile(config.LicenseFile, []byte(licenseKey), 0600)

	// Ensure emos directory exists
	os.MkdirAll(filepath.Join(config.HomeDir, "emos"), 0755)

	// Deploy robot files
	if err := deployRobotFiles(creds); err != nil {
		return err
	}

	// Deploy container
	if err := deployContainer(creds); err != nil {
		return err
	}

	// Create systemd service
	if ui.Confirm("Create systemd service for auto-restart?") {
		if err := createSystemdService(); err != nil {
			ui.Warn("Failed to create systemd service: " + err.Error())
		}
	}

	fmt.Println()
	ui.SuccessBox("EmbodiedOS installed successfully!")
	return nil
}

func deployRobotFiles(creds *api.Credentials) error {
	ui.Header("DEPLOYING ROBOT FILES")

	return ui.Spinner("Cloning deployment repository...", func() error {
		tmpDir, err := os.MkdirTemp("", "emos-deploy-*")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmpDir)

		gitURL := fmt.Sprintf("https://%s:%s@github.com/%s/%s.git",
			creds.Username, creds.Password, config.GitHubOrg, creds.DeploymentRepo)

		cmd := exec.Command("git", "clone", "--depth", "1", gitURL, tmpDir)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git clone failed: %s", string(out))
		}

		robotSrc := filepath.Join(tmpDir, "robot")
		if _, err := os.Stat(robotSrc); os.IsNotExist(err) {
			return fmt.Errorf("cloned repository does not contain a 'robot' directory")
		}

		robotDest := filepath.Join(config.HomeDir, "emos", "robot")
		os.RemoveAll(robotDest)

		return exec.Command("cp", "-r", robotSrc, robotDest).Run()
	})
}

func deployContainer(creds *api.Credentials) error {
	ui.Header("DEPLOYING CONTAINER")

	err := ui.Spinner("Logging into Docker registry...", func() error {
		return container.Login(creds.Registry, creds.Username, creds.Password)
	})
	if err != nil {
		return err
	}

	fmt.Println()
	ui.Info("Pulling EmbodiedOS container image...")
	ui.Faint("This may take several minutes depending on your network connection.")
	if err := container.Pull(creds.FullImage()); err != nil {
		return fmt.Errorf("failed to pull Docker image: %w", err)
	}
	ui.Success("Pulled latest image.")

	return ui.Spinner("Starting EmbodiedOS container...", func() error {
		return container.Run(config.ContainerName, creds.FullImage())
	})
}

func createSystemdService() error {
	serviceContent := fmt.Sprintf(`[Unit]
Description=EmbodiedOS Container
After=docker.service
Requires=docker.service
[Service]
Restart=always
ExecStart=/usr/bin/docker start -a %s
ExecStop=/usr/bin/docker stop -t 2 %s
[Install]
WantedBy=multi-user.target
`, config.ContainerName, config.ContainerName)

	servicePath := "/etc/systemd/system/" + config.ServiceName

	// Write service file
	cmd := exec.Command("sudo", "tee", servicePath)
	cmd.Stdin = strings.NewReader(serviceContent)
	if err := cmd.Run(); err != nil {
		return err
	}

	// Reload and enable
	exec.Command("sudo", "systemctl", "daemon-reload").Run()
	exec.Command("sudo", "systemctl", "enable", config.ServiceName).Run()
	exec.Command("sudo", "systemctl", "start", config.ServiceName).Run()

	ui.Success("Systemd service created and started.")
	return nil
}
