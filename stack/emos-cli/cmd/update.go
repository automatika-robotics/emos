package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"

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

	// Self-update the CLI binary first
	updated, err := selfUpdateCLI()
	if err != nil {
		ui.Warn("CLI self-update failed: " + err.Error())
		ui.Info("Continuing with current version...")
		fmt.Println()
	}
	if updated {
		fmt.Println()
		ui.Info("Please run 'emos update' again to update your installation.")
		return nil
	}

	cfg := config.LoadConfig()
	if !cfg.IsInstalled() {
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
	case config.ModePixi:
		return updatePixi(cfg)
	default:
		return fmt.Errorf("unknown mode: %s", cfg.Mode)
	}
}

// selfUpdateCLI checks for a newer CLI release and replaces the current binary.
// Returns true if the binary was updated and the caller should exit.
func selfUpdateCLI() (bool, error) {
	if config.Version == "dev" {
		ui.Info("Development build, skipping CLI update check.")
		return false, nil
	}

	ui.Info("Checking for CLI updates...")

	// Fetch latest release info
	resp, err := http.Get(config.ReleasesURL())
	if err != nil {
		return false, fmt.Errorf("failed to check releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return false, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return false, fmt.Errorf("failed to parse release: %w", err)
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")
	if latestVersion == config.Version {
		ui.Success("CLI is already up to date (v" + config.Version + ")")
		return false, nil
	}

	// Find the binary for current architecture
	arch := runtime.GOARCH
	binaryName := fmt.Sprintf("emos-linux-%s", arch)
	var downloadURL string
	for _, a := range release.Assets {
		if a.Name == binaryName {
			downloadURL = a.BrowserDownloadURL
			break
		}
	}
	if downloadURL == "" {
		return false, fmt.Errorf("no binary found for linux-%s in release %s", arch, release.TagName)
	}

	ui.Info(fmt.Sprintf("Updating CLI: v%s -> v%s", config.Version, latestVersion))

	// Download to temp file
	tmpFile, err := os.CreateTemp("", "emos-update-*")
	if err != nil {
		return false, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	dlResp, err := http.Get(downloadURL)
	if err != nil {
		tmpFile.Close()
		return false, fmt.Errorf("failed to download: %w", err)
	}
	defer dlResp.Body.Close()

	if _, err := io.Copy(tmpFile, dlResp.Body); err != nil {
		tmpFile.Close()
		return false, fmt.Errorf("failed to write binary: %w", err)
	}
	tmpFile.Close()

	if err := os.Chmod(tmpFile.Name(), 0755); err != nil {
		return false, fmt.Errorf("failed to chmod: %w", err)
	}

	// Find where the current binary lives
	selfPath, err := os.Executable()
	if err != nil {
		return false, fmt.Errorf("cannot determine binary path: %w", err)
	}

	// Replace binary (may need sudo)
	if err := os.Rename(tmpFile.Name(), selfPath); err != nil {
		// Rename failed (cross-device or permissions), try sudo cp
		cpCmd := exec.Command("sudo", "cp", tmpFile.Name(), selfPath)
		if err := cpCmd.Run(); err != nil {
			return false, fmt.Errorf("failed to replace binary (try running with sudo): %w", err)
		}
	}

	ui.Success("CLI updated to v" + latestVersion)
	return true, nil
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

func updatePixi(cfg *config.EMOSConfig) error {
	projectDir := cfg.PixiProjectDir
	if projectDir == "" {
		ui.Error("No pixi project directory configured.")
		return fmt.Errorf("pixi project dir not set")
	}

	fmt.Println("  Updating EMOS pixi workspace...")
	fmt.Println()

	// Pull latest source
	if err := ui.Spinner("Pulling latest source...", func() error {
		cmd := exec.Command("git", "pull")
		cmd.Dir = projectDir
		return cmd.Run()
	}); err != nil {
		return fmt.Errorf("git pull failed: %w", err)
	}

	// Update submodules
	if err := ui.Spinner("Updating submodules...", func() error {
		cmd := exec.Command("git", "submodule", "update", "--init", "--depth", "1")
		cmd.Dir = projectDir
		return cmd.Run()
	}); err != nil {
		return fmt.Errorf("submodule update failed: %w", err)
	}

	// Reinstall dependencies
	ui.Info("Updating pixi environment...")
	pixiInstall := exec.Command("pixi", "install")
	pixiInstall.Dir = projectDir
	pixiInstall.Stdout = os.Stdout
	pixiInstall.Stderr = os.Stderr
	if err := pixiInstall.Run(); err != nil {
		return fmt.Errorf("pixi install failed: %w", err)
	}

	// Rebuild
	ui.Info("Rebuilding EMOS packages...")
	pixiSetup := exec.Command("pixi", "run", "setup")
	pixiSetup.Dir = projectDir
	pixiSetup.Stdout = os.Stdout
	pixiSetup.Stderr = os.Stderr
	if err := pixiSetup.Run(); err != nil {
		return fmt.Errorf("pixi run setup failed: %w", err)
	}

	fmt.Println()
	ui.SuccessBox("EMOS pixi workspace updated successfully!")
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

	if err := installer.UpdateNative(wsPath, cfg.ROSDistro); err != nil {
		return err
	}

	fmt.Println()
	ui.SuccessBox("EMOS packages updated successfully!")
	return nil
}
