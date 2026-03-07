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
	"github.com/automatika-robotics/emos-cli/internal/installer"
	"github.com/automatika-robotics/emos-cli/internal/ui"
	"github.com/spf13/cobra"
)

var (
	installModeFlag   string
	installDistroFlag string
)

var installCmd = &cobra.Command{
	Use:   "install [license-key]",
	Short: "Install EMOS (container, native, or licensed mode)",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runInstall,
}

func init() {
	installCmd.Flags().StringVar(&installModeFlag, "mode", "",
		"Installation mode: container, native, or licensed")
	installCmd.Flags().StringVar(&installDistroFlag, "distro", "",
		"ROS 2 distribution (jazzy, humble, kilted)")
}

func runInstall(cmd *cobra.Command, args []string) error {
	ui.Banner(config.Version)

	// If a license key is provided directly, go to licensed flow
	if len(args) == 1 {
		return installLicensed(args[0])
	}

	// If --mode flag is set, skip the menu
	switch installModeFlag {
	case "container", "oss-container":
		return installOSSContainer()
	case "native":
		return installNative()
	case "licensed":
		key := ui.Input("Enter your EMOS license key", "")
		if key == "" {
			return fmt.Errorf("license key is required for licensed mode")
		}
		return installLicensed(key)
	case "":
		// Show interactive menu
	default:
		return fmt.Errorf("unknown mode: %s (use container, native, or licensed)", installModeFlag)
	}

	fmt.Println("  Welcome to EMOS - The Embodied Operating System")
	fmt.Println()
	fmt.Println("  How would you like to install EMOS?")

	choice := ui.Select("Select installation mode:", []string{
		"Container Install         (No ROS required - runs in Docker)",
		"Native Install            (Requires existing ROS 2 installation)",
		"I have an EMOS License Key",
	})

	switch choice {
	case 0:
		return installOSSContainer()
	case 1:
		return installNative()
	case 2:
		key := ui.Input("Enter your EMOS license key", "")
		if key == "" {
			return fmt.Errorf("license key is required")
		}
		return installLicensed(key)
	}

	return nil
}

func selectDistro() string {
	if installDistroFlag != "" {
		return installDistroFlag
	}
	choice := ui.Select("Select ROS 2 distribution:", []string{
		"Jazzy (Recommended)",
		"Humble",
		"Kilted",
	})
	switch choice {
	case 0:
		return "jazzy"
	case 1:
		return "humble"
	case 2:
		return "kilted"
	}
	return "jazzy"
}

func installOSSContainer() error {
	// Check Docker is installed
	if _, err := exec.LookPath("docker"); err != nil {
		ui.Error("Docker is not installed or not in PATH.")
		fmt.Println("  Please install Docker first: https://docs.docker.com/get-docker/")
		return fmt.Errorf("docker not found")
	}
	ui.Success("Docker detected.")

	distro := selectDistro()
	image := config.PublicImageTag(distro)

	// Check for existing container
	if container.Exists(config.ContainerName) {
		ui.Warn("An existing EMOS container was found.")
		if !ui.Confirm("This will REMOVE the existing container and perform a fresh installation. Are you sure?") {
			return fmt.Errorf("aborted by user")
		}
		if err := ui.Spinner("Removing existing container...", func() error {
			return container.Remove(config.ContainerName)
		}); err != nil {
			return fmt.Errorf("failed to remove container: %w", err)
		}
	}

	os.MkdirAll(config.ConfigDir, 0755)

	// Pull public image (no login needed)
	fmt.Println()
	ui.Info("Pulling EMOS container image: " + image)
	ui.Faint("This may take several minutes depending on your network connection.")
	if err := container.Pull(image); err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}
	ui.Success("Pulled image successfully.")

	// Create directories
	os.MkdirAll(filepath.Join(config.HomeDir, "emos", "recipes"), 0755)
	os.MkdirAll(filepath.Join(config.HomeDir, "emos", "logs"), 0755)

	// Start container
	if err := ui.Spinner("Starting EMOS container...", func() error {
		return container.Run(config.ContainerName, image)
	}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	// Save config
	cfg := &config.EMOSConfig{
		Mode:      config.ModeOSSContainer,
		ROSDistro: distro,
		ImageTag:  image,
	}
	if err := config.SaveConfig(cfg); err != nil {
		ui.Warn("Failed to save config: " + err.Error())
	}

	fmt.Println()
	ui.SuccessBox("EMOS installed successfully (container mode)!")
	ui.Faint("Run 'emos pull <recipe>' to download a recipe, then 'emos run <recipe>' to execute it.")
	ui.Faint("Ensure your sensor drivers are running externally (host or separate containers).")
	return nil
}

func installNative() error {
	// Detect ROS 2 installations
	installs := installer.DetectROS()

	if len(installs) == 0 {
		ui.Error("No ROS 2 installation detected.")
		fmt.Println()
		fmt.Println("  Native install requires ROS 2. Options:")
		fmt.Println("    1. Install ROS 2 first: https://docs.ros.org/")
		fmt.Println("    2. Use container install instead (no ROS needed)")
		fmt.Println()
		if ui.Confirm("Switch to container install?") {
			return installOSSContainer()
		}
		return fmt.Errorf("no ROS 2 installation found")
	}

	var chosen installer.ROSInstallation
	if len(installs) == 1 {
		chosen = installs[0]
		ui.Success(fmt.Sprintf("ROS 2 %s detected at %s", capitalize(chosen.Distro), chosen.Path))
		if !ui.Confirm(fmt.Sprintf("Proceed with native EMOS installation for %s?", capitalize(chosen.Distro))) {
			return fmt.Errorf("aborted by user")
		}
	} else {
		fmt.Println()
		ui.Info("Multiple ROS 2 installations detected:")
		options := make([]string, len(installs))
		for i, inst := range installs {
			options[i] = fmt.Sprintf("%s  (%s)", capitalize(inst.Distro), inst.Path)
		}
		idx := ui.Select("Select ROS 2 distribution:", options)
		chosen = installs[idx]
	}

	os.MkdirAll(config.ConfigDir, 0755)

	// Create directories
	wsPath := filepath.Join(config.HomeDir, "emos", "ros_ws")
	os.MkdirAll(filepath.Join(config.HomeDir, "emos", "recipes"), 0755)
	os.MkdirAll(filepath.Join(config.HomeDir, "emos", "logs"), 0755)

	ui.Header("INSTALLING EMOS PACKAGES")

	if err := installer.InstallNativeWorkspace(wsPath, chosen.Distro); err != nil {
		return err
	}

	// Save config
	cfg := &config.EMOSConfig{
		Mode:          config.ModeNative,
		ROSDistro:     chosen.Distro,
		WorkspacePath: wsPath,
	}
	if err := config.SaveConfig(cfg); err != nil {
		ui.Warn("Failed to save config: " + err.Error())
	}

	fmt.Println()
	ui.SuccessBox("EMOS installed successfully (native mode)!")
	ui.Faint("Run 'emos pull <recipe>' to download a recipe, then 'emos run <recipe>' to execute it.")
	ui.Faint("Ensure your sensor drivers are running on the host before running recipes.")
	return nil
}

func installLicensed(licenseKey string) error {
	// Check for existing installation
	if container.Exists(config.ContainerName) {
		ui.Warn("An existing EmbodiedOS container was found.")
		if !ui.Confirm("This will REMOVE the existing container and perform a fresh installation. Are you sure?") {
			ui.Error("Installation aborted.")
			return fmt.Errorf("aborted by user")
		}
	}

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
	if err := os.WriteFile(config.LicenseFile, []byte(licenseKey), 0600); err != nil {
		ui.Warn("Failed to save license file: " + err.Error())
	}

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

	// Save config
	cfg := &config.EMOSConfig{
		Mode:       config.ModeLicensed,
		LicenseKey: licenseKey,
		ROSDistro:  "jazzy",
		ImageTag:   creds.FullImage(),
	}
	if err := config.SaveConfig(cfg); err != nil {
		ui.Warn("Failed to save config: " + err.Error())
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

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
