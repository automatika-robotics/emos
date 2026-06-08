package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/container"
	"github.com/automatika-robotics/emos-cli/internal/installer"
	"github.com/automatika-robotics/emos-cli/internal/ui"
)

var (
	uninstallKeepData    bool
	uninstallKeepConfig  bool
	uninstallYes         bool
	uninstallRemoveImage bool
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove EMOS from this device",
	Long: `Removes the EMOS installation set up by 'emos install'.

Always:
  - stops + removes the dashboard systemd service (emos-dashboard.service).

By install mode:
  - container / licensed: removes the Docker container; image is preserved unless
    --remove-image is passed; licensed mode also removes the container auto-restart
    unit and ~/emos/robot.
  - native:               removes the build workspace and uninstalls kompass-core.
                          Files in /opt/ros/<distro>/ are co-mingled with ROS by
                          colcon and cannot be cleanly uninstalled — the command
                          prints the manual rm commands instead of running them.
  - pixi:                 removes .pixi/, build/, install/, log/ under the EMOS
                          repo directory. The cloned repo itself is preserved.

By default, also removes ~/emos/recipes, ~/emos/logs, and ~/.config/emos.
Use --keep-data and --keep-config to preserve these.

The CLI binary at /usr/local/bin/emos is never removed automatically — running
process can't reliably delete itself. The command prints the rm command for you.`,
	RunE: runUninstall,
}

func init() {
	uninstallCmd.Flags().BoolVar(&uninstallKeepData, "keep-data", false,
		"Preserve ~/emos/recipes and ~/emos/logs")
	uninstallCmd.Flags().BoolVar(&uninstallKeepConfig, "keep-config", false,
		"Preserve ~/.config/emos")
	uninstallCmd.Flags().BoolVar(&uninstallRemoveImage, "remove-image", false,
		"Also remove the Docker image (container / licensed modes)")
	uninstallCmd.Flags().BoolVarP(&uninstallYes, "yes", "y", false,
		"Skip the confirmation prompt")
}

func runUninstall(cmd *cobra.Command, args []string) error {
	ui.Banner(config.Version)
	ui.Header("EMOS UNINSTALL")
	warnIfSudo()

	cfg := config.LoadConfig()
	if cfg == nil || !cfg.IsInstalled() {
		ui.Warn("No EMOS installation recorded in " + config.ConfigFile + ".")
		ui.Faint("Continuing in best-effort mode -- stray files will still be cleaned.")
		cfg = &config.EMOSConfig{}
	} else {
		ui.Info("Mode:        " + string(cfg.Mode))
		if cfg.ROSDistro != "" {
			ui.Info("ROS distro:  " + cfg.ROSDistro)
		}
	}

	printRemovalPlan(cfg)

	if !uninstallYes {
		if !ui.Confirm("Proceed with uninstall?") {
			ui.Info("Uninstall canceled.")
			return nil
		}
	}

	// Always: dashboard systemd service.
	if existsUnitFile(config.DashboardServiceName) {
		ui.Info("Removing " + config.DashboardServiceName + "...")
		unit := installer.SystemdUnit{Name: config.DashboardServiceName}
		if err := unit.Uninstall(); err != nil {
			ui.Warn(err.Error())
		} else {
			ui.Success("Dashboard service removed.")
		}
	}

	// Mode-specific cleanup.
	switch cfg.Mode {
	case config.ModeOSSContainer, config.ModeLicensed:
		uninstallContainerMode(cfg)
	case config.ModeNative:
		uninstallNativeMode(cfg)
	case config.ModePixi:
		uninstallPixiMode(cfg)
	}

	// Robot-plugin workspace is not user data, always remove it.
	removePathQuiet("plugin workspace", config.WorkspaceDir)

	// Common cleanup.
	if !uninstallKeepData {
		removePathQuiet("recipes", config.RecipesDir)
		removePathQuiet("logs", config.LogsDir)
	} else {
		ui.Faint("Preserved ~/emos/recipes and ~/emos/logs (--keep-data).")
	}
	if !uninstallKeepConfig {
		removePathQuiet("config", config.ConfigDir)
	} else {
		ui.Faint("Preserved ~/.config/emos (--keep-config).")
	}

	fmt.Println()
	ui.SuccessBox("EMOS uninstalled.")

	if _, err := os.Stat("/usr/local/bin/emos"); err == nil {
		fmt.Println()
		ui.Faint("To remove the CLI binary itself, run:")
		ui.Faint("  sudo rm /usr/local/bin/emos")
	}
	return nil
}

func printRemovalPlan(cfg *config.EMOSConfig) {
	fmt.Println()
	ui.Faint("The following will be removed:")
	fmt.Println()

	if existsUnitFile(config.DashboardServiceName) {
		ui.Faint("  - dashboard systemd service (" + config.DashboardServiceName + ")")
	}

	switch cfg.Mode {
	case config.ModeOSSContainer:
		ui.Faint("  - Docker container '" + config.ContainerName + "'")
		img := config.PublicImageTag(distroOr(cfg, "jazzy"))
		if uninstallRemoveImage {
			ui.Faint("  - Docker image " + img)
		} else {
			ui.Faint("    (image " + img + " preserved; pass --remove-image to also drop it)")
		}
	case config.ModeLicensed:
		ui.Faint("  - Docker container '" + config.ContainerName + "'")
		if existsUnitFile(config.ServiceName) {
			ui.Faint("  - container restart unit (" + config.ServiceName + ")")
		}
		if uninstallRemoveImage && cfg.ImageTag != "" {
			ui.Faint("  - Docker image " + cfg.ImageTag)
		}
		robotDir := filepath.Join(config.HomeDir, "emos", "robot")
		if _, err := os.Stat(robotDir); err == nil {
			ui.Faint("  - " + robotDir)
		}
	case config.ModeNative:
		if cfg.WorkspacePath != "" {
			ui.Faint("  - native build workspace: " + cfg.WorkspacePath)
		}
		ui.Faint("  - kompass-core (via pip uninstall)")
		ui.Faint("  - /opt/ros/" + distroOr(cfg, "jazzy") + "/ -- best-effort, manual commands printed")
	case config.ModePixi:
		if cfg.PixiProjectDir != "" {
			if samePath(cfg.PixiProjectDir, config.PixiDir) {
				ui.Faint("  - pixi workspace (cloned repo + env + build): " + cfg.PixiProjectDir)
			} else {
				ui.Faint("  - pixi env, build, install, log under: " + cfg.PixiProjectDir)
			}
		}
	}

	if cfg.Plugin != nil {
		ui.Faint("  - robot plugin workspace: " + config.WorkspaceDir)
	}
	if !uninstallKeepData {
		ui.Faint("  - " + config.RecipesDir)
		ui.Faint("  - " + config.LogsDir)
	}
	if !uninstallKeepConfig {
		ui.Faint("  - " + config.ConfigDir)
	}
	fmt.Println()
}

func uninstallContainerMode(cfg *config.EMOSConfig) {
	if container.Exists(config.ContainerName) {
		ui.Info("Stopping container '" + config.ContainerName + "'...")
		_ = container.Stop(config.ContainerName)
		ui.Info("Removing container '" + config.ContainerName + "'...")
		if err := container.Remove(config.ContainerName); err != nil {
			ui.Warn("docker rm: " + err.Error())
		} else {
			ui.Success("Container removed.")
		}
	}

	// Container auto-restart unit (licensed flow installs this).
	if existsUnitFile(config.ServiceName) {
		ui.Info("Removing " + config.ServiceName + "...")
		unit := installer.SystemdUnit{Name: config.ServiceName}
		if err := unit.Uninstall(); err != nil {
			ui.Warn(err.Error())
		} else {
			ui.Success("Container restart unit removed.")
		}
	}

	if uninstallRemoveImage {
		var img string
		switch cfg.Mode {
		case config.ModeOSSContainer:
			img = config.PublicImageTag(distroOr(cfg, "jazzy"))
		case config.ModeLicensed:
			img = cfg.ImageTag
		}
		if img != "" {
			ui.Info("Removing image " + img + "...")
			c := exec.Command("docker", "rmi", img)
			if out, err := c.CombinedOutput(); err != nil {
				ui.Warn("docker rmi: " + strings.TrimSpace(string(out)))
			} else {
				ui.Success("Image removed.")
			}
		}
	}

	if cfg.Mode == config.ModeLicensed {
		robotDir := filepath.Join(config.HomeDir, "emos", "robot")
		if _, err := os.Stat(robotDir); err == nil {
			removePathQuiet("licensed deployment files", robotDir)
		}
	}
}

func uninstallNativeMode(cfg *config.EMOSConfig) {
	if cfg.WorkspacePath != "" {
		removePathQuiet("native workspace", cfg.WorkspacePath)
	}

	ui.Info("Uninstalling kompass-core via pip...")
	if err := tryPipUninstall("kompass-core"); err != nil {
		ui.Warn("kompass-core uninstall: " + err.Error())
	} else {
		ui.Success("kompass-core uninstalled.")
	}

	distro := distroOr(cfg, "jazzy")
	rosPath := filepath.Join("/opt/ros", distro)
	fmt.Println()
	ui.Warn("EMOS package files in " + rosPath + " cannot be cleanly uninstalled --")
	ui.Warn("colcon merge-install co-mingles them with ROS itself.")
	ui.Faint("To remove them manually (review carefully before running):")
	ui.Faint("  sudo rm -rf " + rosPath + "/share/{automatika_ros_sugar,automatika_embodied_agents,kompass,kompass_interfaces}")
	ui.Faint("  sudo rm -rf " + rosPath + "/lib/python*/site-packages/{agents,kompass,ros_sugar,kompass_interfaces}")
	ui.Faint("  sudo rm -rf " + rosPath + "/lib/{kompass,kompass_interfaces}")
	ui.Faint("  sudo rm -rf " + rosPath + "/include/{kompass,kompass_interfaces}")
}

func uninstallPixiMode(cfg *config.EMOSConfig) {
	dir := cfg.PixiProjectDir
	if dir == "" {
		ui.Warn("No pixi project directory recorded in config; skipping pixi cleanup.")
		return
	}
	// Remove workspace in pixi installation dir
	if samePath(dir, config.PixiDir) {
		removePathQuiet("pixi workspace", dir)
		return
	}
	for _, sub := range []string{".pixi", "build", "install", "log"} {
		p := filepath.Join(dir, sub)
		if _, err := os.Stat(p); err == nil {
			removePathQuiet(sub, p)
		}
	}
	ui.Faint("The cloned EMOS repo at " + dir + " was preserved.")
}

// samePath reports whether two paths resolve to the same absolute location.
func samePath(a, b string) bool {
	ca, err := filepath.Abs(a)
	if err != nil {
		return false
	}
	cb, err := filepath.Abs(b)
	if err != nil {
		return false
	}
	return filepath.Clean(ca) == filepath.Clean(cb)
}

// existsUnitFile reports whether a systemd unit file is present on disk.
func existsUnitFile(name string) bool {
	_, err := os.Stat(filepath.Join("/etc/systemd/system", name))
	return err == nil
}

// removePathQuiet rm -rf's a path with a one-line user-facing log. Errors are
// surfaced as warnings; missing paths are silent (uninstall is idempotent).
// Paths flagged by safeToDelete are skipped with a warning.
func removePathQuiet(label, path string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return
	}
	if err := safeToDelete(path); err != nil {
		ui.Warn("skip " + label + ": " + err.Error())
		return
	}
	ui.Info("Removing " + label + ": " + path)
	if err := os.RemoveAll(path); err != nil {
		ui.Warn(label + " removal: " + err.Error())
	}
}

// protectedRoots are absolute paths that uninstall always refuses to rm -rf,
// regardless of what the config file claims.
var protectedRoots = map[string]struct{}{
	"/": {}, "/bin": {}, "/boot": {}, "/dev": {}, "/etc": {},
	"/home": {}, "/lib": {}, "/lib32": {}, "/lib64": {}, "/opt": {},
	"/proc": {}, "/root": {}, "/run": {}, "/sbin": {}, "/srv": {},
	"/sys": {}, "/tmp": {}, "/usr": {}, "/var": {},
}

// safeToDelete returns an error if the path looks dangerous to remove --
// empty, very short, or one of the well-known system roots. The guard catches
// accidents (config corruption, mistyped paths) rather than enforcing a sandbox.
func safeToDelete(path string) error {
	if path == "" {
		return errors.New("empty path")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("cannot resolve %q: %w", path, err)
	}
	abs = filepath.Clean(abs)
	if len(abs) < 4 {
		return fmt.Errorf("path too short: %q", abs)
	}
	if _, isProtected := protectedRoots[abs]; isProtected {
		return fmt.Errorf("refusing to remove protected path %q", abs)
	}
	return nil
}

// tryPipUninstall calls `pip uninstall -y <pkg>`, falling back to `pip3` and
// then to `sudo pip` for installs that landed in system Python. Treats "not
// installed" as success (idempotency).
func tryPipUninstall(pkg string) error {
	for _, args := range [][]string{
		{"pip", "uninstall", "-y", pkg},
		{"pip3", "uninstall", "-y", pkg},
		{"sudo", "pip", "uninstall", "-y", pkg},
		{"sudo", "pip3", "uninstall", "-y", pkg},
	} {
		c := exec.Command(args[0], args[1:]...)
		out, err := c.CombinedOutput()
		text := string(out)
		if err == nil {
			return nil
		}
		// Treat "not installed" as success.
		if strings.Contains(text, "not installed") || strings.Contains(text, "Skipping") {
			return nil
		}
	}
	return fmt.Errorf("all pip variants failed; uninstall manually if needed")
}

func distroOr(cfg *config.EMOSConfig, def string) string {
	if cfg != nil && cfg.ROSDistro != "" {
		return cfg.ROSDistro
	}
	return def
}
