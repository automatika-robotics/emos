package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/container"
	"github.com/automatika-robotics/emos-cli/internal/installer"
	"github.com/automatika-robotics/emos-cli/internal/ui"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Display EMOS installation status",
	Run: func(cmd *cobra.Command, args []string) {
		banner()
		ui.StatusCard(config.Version)
		printChannel()
		printUpdateAvailable()

		cfg := config.LoadConfig()
		if !cfg.IsInstalled() {
			fmt.Println()
			ui.Error("No EMOS installation found.")
			ui.Faint("Run 'emos install' to get started.")
			statusLicense()
			return
		}

		fmt.Println()
		ui.Info("Mode: " + string(cfg.Mode))
		ui.Info("ROS Distro: " + cfg.ROSDistro)
		statusPlugins(cfg)

		switch cfg.Mode {
		case config.ModeOSSContainer:
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

		case config.ModePixi:
			pixiStatus(cfg)
		}
		statusCommits(cfg)
		statusLicense()
	},
}

func pixiStatus(cfg *config.EMOSConfig) {
	projectDir := cfg.PixiProjectDir
	if projectDir == "" {
		ui.Error("No pixi project directory configured.")
		return
	}

	// check for pixi binary (PATH, then ~/.pixi/bin, then /usr/local/bin)
	pixiBin, pixiErr := installer.ResolvePixi()
	if pixiErr != nil {
		ui.Error("pixi: Not found")
	} else {
		ui.Success("pixi: Available")
	}

	// Project dir
	pixiToml := filepath.Join(projectDir, "pixi.toml")
	if _, err := os.Stat(pixiToml); err != nil {
		ui.Error("pixi project: Not found at " + projectDir)
		return
	}
	ui.Success("pixi project: " + projectDir)

	// Without a pixi binary we can't probe the environment; skip the package
	// checks
	if pixiErr != nil {
		return
	}

	// Helper to run a shell command inside the pixi environment
	setupSh := filepath.Join(projectDir, "install", "setup.sh")
	pixiShell := func(shellCmd string) *exec.Cmd {
		cmd := exec.Command(pixiBin, "run", "--manifest-path", pixiToml, "bash", "-c", shellCmd)
		cmd.Dir = projectDir
		return cmd
	}

	checkPackages(
		func(module string) error {
			return pixiShell(fmt.Sprintf("source %s && python3 -c 'import %s' 2>&1", setupSh, module)).Run()
		},
		func() (string, error) {
			out, err := pixiShell(fmt.Sprintf("source %s && ros2 pkg list 2>/dev/null", setupSh)).Output()
			return string(out), err
		},
	)
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

	checkPackages(
		func(module string) error {
			return exec.Command("bash", "-c", fmt.Sprintf("source %s && python3 -c 'import %s' 2>&1", rosSetup, module)).Run()
		},
		func() (string, error) {
			out, err := exec.Command("bash", "-c", fmt.Sprintf("source %s && ros2 pkg list 2>/dev/null", rosSetup)).Output()
			return string(out), err
		},
	)
}

// checkPackages verifies EMOS Python and ROS packages using the provided
// import checker and package lister functions.
func checkPackages(tryImport func(module string) error, listROSPkgs func() (string, error)) {
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
		if err := tryImport(m.module); err != nil {
			ui.Error(fmt.Sprintf("  %s: Not installed", m.display))
		} else {
			ui.Success(fmt.Sprintf("  %s: OK", m.display))
		}
	}

	fmt.Println()
	ui.Info("ROS Packages:")
	out, err := listROSPkgs()
	if err != nil {
		ui.Warn("  Could not list ROS packages")
		return
	}

	rosPkgs := []string{
		"automatika_ros_sugar",
		"automatika_embodied_agents",
		"kompass",
		"kompass_interfaces",
		"emos_mapping",
	}

	for _, name := range rosPkgs {
		if strings.Contains(out, name) {
			ui.Success(fmt.Sprintf("  %s: OK", name))
		} else {
			ui.Error(fmt.Sprintf("  %s: Not found", name))
		}
	}
}

// printChannel says when this binary follows the nightly builds, and how to
// leave them.
func printChannel() {
	if config.Channel() != "dev" {
		return
	}
	ui.Faint("Channel: dev (nightly builds of unreleased EMOS)")
	ui.Faint("Back to stable: curl -fsSL " + config.InstallerURL() + " | sudo bash, then 'emos update'")
}

// statusPlugins prints the installed robot plugin and sensor plugins.
func statusPlugins(cfg *config.EMOSConfig) {
	if cfg.Plugin == nil {
		ui.Info("Robot plugin: none")
	} else {
		ui.Info("Robot plugin: " + pluginLabel(*cfg.Plugin))
	}
	if len(cfg.SensorPlugins) == 0 {
		return
	}
	names := make([]string, 0, len(cfg.SensorPlugins))
	for _, p := range cfg.SensorPlugins {
		names = append(names, pluginLabel(p))
	}
	ui.Info("Sensor plugins: " + strings.Join(names, ", "))
}

// pluginLabel is the hardware's name with the catalog slug, or the slug alone
// when the plugin gave no name.
func pluginLabel(p config.PluginInfo) string {
	if name := p.DisplayName(); name != p.Slug {
		return fmt.Sprintf("%s (%s)", name, p.Slug)
	}
	return p.Slug
}

// The stack packages checked out under the workspace's stack/ directory.
var stackPackages = []string{"sugarcoat", "kompass", "embodied-agents"}

// statusCommits lists the commit every part of the install runs. Only on the
// dev channel, where a build is known by its commits.
func statusCommits(cfg *config.EMOSConfig) {
	if config.Channel() != "dev" {
		return
	}
	var rows [][]string
	switch cfg.Mode {
	case config.ModePixi:
		rows = workspaceCommits(cfg.PixiProjectDir)
	case config.ModeNative:
		rows = workspaceCommits(filepath.Join(cfg.WorkspacePath, "src", ".emos-repo"))
	case config.ModeOSSContainer:
		rows = imageCommits(cfg.ImageTag)
	}
	for _, p := range cfg.Plugins() {
		rows = append(rows, []string{p.Slug, gitShortHead(filepath.Join(config.PluginSrcDir(), p.Slug))})
	}
	if len(rows) == 0 {
		return
	}
	fmt.Println()
	ui.Info("Commits:")
	for _, row := range rows {
		ui.Faint(fmt.Sprintf("  %-20s %s", row[0], row[1]))
	}
}

// workspaceCommits reads the EMOS checkout at dir and its stack submodules.
func workspaceCommits(dir string) [][]string {
	rows := [][]string{{"emos", gitShortHead(dir)}}
	for _, pkg := range stackPackages {
		rows = append(rows, []string{pkg, gitShortHead(filepath.Join(dir, "stack", pkg))})
	}
	return rows
}

// imageCommits reads the commits the nightly image was built from off its
// labels. Images built before the labels existed report unknown.
func imageCommits(image string) [][]string {
	if image == "" {
		return nil
	}
	revision := container.ImageLabel(image, "org.opencontainers.image.revision")
	if revision == "" {
		return [][]string{{"emos", "unknown (the image carries no commit labels)"}}
	}
	return append([][]string{{"emos", shortCommit(revision)}}, parseStackLabel(container.ImageLabel(image, "io.emos.stack"))...)
}

// parseStackLabel turns "sugarcoat@f5ab9cc kompass@5edb164" into rows.
func parseStackLabel(label string) [][]string {
	var rows [][]string
	for _, entry := range strings.Fields(label) {
		if name, sha, ok := strings.Cut(entry, "@"); ok {
			rows = append(rows, []string{name, shortCommit(sha)})
		}
	}
	return rows
}

// gitShortHead is the short commit checked out at dir, or unknown.
func gitShortHead(dir string) string {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--short=7", "HEAD").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

func shortCommit(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
