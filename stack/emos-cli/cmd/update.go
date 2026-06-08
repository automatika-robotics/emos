package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/automatika-robotics/emos-cli/internal/api"
	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/container"
	"github.com/automatika-robotics/emos-cli/internal/installer"
	"github.com/automatika-robotics/emos-cli/internal/plugin"
	"github.com/automatika-robotics/emos-cli/internal/ui"
	"github.com/automatika-robotics/emos-cli/internal/updcheck"
	"github.com/spf13/cobra"
)

// printUpdateAvailable consults the cached latest-release tag and, if it
// names a strictly newer version than this binary, prints a one-line
// nudge. Silent on dev builds, or when no value is cached yet or on parse
// failures. Called by `emos status` and `emos version`.
func printUpdateAvailable() {
	latest, ok := updcheck.RefreshIfStale(updcheck.DefaultRefreshInterval)
	if !ok || !updcheck.IsNewer(config.Version, latest) {
		return
	}
	ui.Faint(fmt.Sprintf("→ Update available: v%s (current v%s) -- run 'emos update'",
		latest, config.Version))
}

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

	var modeErr error
	switch cfg.Mode {
	case config.ModeOSSContainer:
		modeErr = updateOSSContainer(cfg)
	case config.ModeLicensed:
		modeErr = updateLicensed(cfg)
	case config.ModeNative:
		modeErr = updateNative(cfg)
	case config.ModePixi:
		modeErr = updatePixi(cfg)
	default:
		return fmt.Errorf("unknown mode: %s", cfg.Mode)
	}
	if modeErr != nil {
		return modeErr
	}

	// Pull the active robot plugin to its latest commit and rebuild it.
	if cfg.Plugin != nil {
		fmt.Println()
		ui.Header("UPDATING ROBOT PLUGIN")
		if err := plugin.Update(cfg, os.Stdout); err != nil {
			ui.Warn("Plugin update failed: " + err.Error())
		}
	}
	return nil
}

// selfUpdateCLI checks for a newer CLI release and replaces the current binary.
// Returns true if the binary was updated and the caller should exit.
func selfUpdateCLI() (bool, error) {
	if config.Version == "dev" {
		ui.Info("Development build, skipping CLI update check.")
		return false, nil
	}

	ui.Info("Checking for CLI updates...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	latestVersion, err := updcheck.Latest(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to check releases: %w", err)
	}
	if latestVersion == config.Version {
		ui.Success("CLI is already up to date (v" + config.Version + ")")
		return false, nil
	}

	// We still need the asset list to find the right per-arch binary, so
	// hit /releases/latest a second time
	resp, err := http.Get(config.ReleasesURL())
	if err != nil {
		return false, fmt.Errorf("failed to fetch release assets: %w", err)
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

	pixiBin, err := installer.ResolvePixi()
	if err != nil {
		ui.Error("pixi is required to update a pixi-mode install but was not found.")
		fmt.Println("  Install it with: " + installer.PixiInstallHint)
		return err
	}

	fmt.Println("  Updating EMOS pixi workspace...")
	fmt.Println()

	// Preserve user-added pixi dependencies across the git pull: stash the
	// manifest, pull, then reapply.
	stashed := gitTreeDirty(projectDir, "pixi.toml", "pixi.lock")
	if stashed {
		if err := runGit(projectDir, "stash", "push", "-m",
			"emos-update: preserve local pixi deps", "--", "pixi.toml", "pixi.lock"); err != nil {
			return fmt.Errorf("could not preserve local pixi dependencies: %w", err)
		}
		ui.Info("Preserved local pixi dependencies for the update.")
	}

	// Pull latest source
	if err := ui.Spinner("Pulling latest source...", func() error {
		return runGit(projectDir, "pull")
	}); err != nil {
		return fmt.Errorf("git pull failed: %w", err)
	}

	// Update submodules
	if err := ui.Spinner("Updating submodules...", func() error {
		return runGit(projectDir, "submodule", "update", "--init", "--depth", "1")
	}); err != nil {
		return fmt.Errorf("submodule update failed: %w", err)
	}

	// Reapply the preserved dependencies.
	if stashed {
		if err := runGit(projectDir, "stash", "pop"); err != nil {
			ui.Error("Your local pixi dependencies could not be reapplied -- this release changed pixi.toml.")
			fmt.Println("  Resolve the conflict in " + projectDir + "/pixi.toml" +
				" (your changes are saved via `git stash`), then run 'emos update' again.")
			return fmt.Errorf("pixi manifest conflict during update")
		}
		ui.Success("Reapplied local pixi dependencies.")
	}

	// Reinstall dependencies
	ui.Info("Updating pixi environment...")
	pixiInstall := exec.Command(pixiBin, "install")
	pixiInstall.Dir = projectDir
	pixiInstall.Env = pixiBuildEnv()
	pixiInstall.Stdout = os.Stdout
	pixiInstall.Stderr = os.Stderr
	if err := pixiInstall.Run(); err != nil {
		return fmt.Errorf("pixi install failed: %w", err)
	}

	// Rebuild
	ui.Info("Rebuilding EMOS packages...")
	pixiSetup := exec.Command(pixiBin, "run", "setup")
	pixiSetup.Dir = projectDir
	pixiSetup.Env = pixiBuildEnv()
	pixiSetup.Stdout = os.Stdout
	pixiSetup.Stderr = os.Stderr
	if err := pixiSetup.Run(); err != nil {
		return fmt.Errorf("pixi run setup failed: %w", err)
	}

	fmt.Println()
	ui.SuccessBox("EMOS pixi workspace updated successfully!")
	return nil
}

// gitTreeDirty reports whether any of the given paths have uncommitted changes
// in the repository at dir.
func gitTreeDirty(dir string, paths ...string) bool {
	args := append([]string{"status", "--porcelain", "--"}, paths...)
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(out))) > 0
}

// runGit runs a git command in dir, folding stderr into the returned error.
func runGit(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
	}
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
