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
			containerPackages(cfg, status == "running")

		case config.ModeNative:
			nativeStatus(cfg)

		case config.ModePixi:
			pixiStatus(cfg)
		}
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

	setupSh := filepath.Join(projectDir, "install", "setup.sh")
	statusPackages(func(script string) (string, error) {
		cmd := exec.Command(pixiBin, "run", "--manifest-path", pixiToml, "bash", "-c", "source "+setupSh+" && "+script)
		cmd.Dir = projectDir
		out, err := cmd.Output()
		return string(out), err
	}, sourceCommits(cfg))
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

	statusPackages(func(script string) (string, error) {
		out, err := exec.Command("bash", "-c", "source "+rosSetup+" && "+script).Output()
		return string(out), err
	}, sourceCommits(cfg))
}

// containerPackages probes the running container, or a throwaway one from
// the image while it is stopped.
func containerPackages(cfg *config.EMOSConfig, running bool) {
	statusPackages(func(script string) (string, error) {
		if running {
			return container.Exec(config.ContainerName, "source /ros_entrypoint.sh && "+script)
		}
		return container.RunEphemeralCapture(cfg.ImageTag, script)
	}, sourceCommits(cfg))
}

// The EMOS ROS packages and the checkout each one is built from.
var emosPackages = []struct{ name, source string }{
	{"automatika_ros_sugar", "sugarcoat"},
	{"automatika_embodied_agents", "embodied-agents"},
	{"kompass", "kompass"},
	{"kompass_interfaces", "kompass"},
	{"emos_mapping", "emos"},
}

// packageProbe reads each ROS package's version off its package.xml on the
// ament prefix path, then kompass-core's from its Python metadata, in one
// shell of the install's environment.
func packageProbe() string {
	names := make([]string, len(emosPackages))
	for i, pkg := range emosPackages {
		names[i] = pkg.name
	}
	return `IFS=: read -ra prefixes <<< "$AMENT_PREFIX_PATH"
for p in ` + strings.Join(names, " ") + `; do
  for d in "${prefixes[@]}"; do
    f="$d/share/$p/package.xml"
    if [ -f "$f" ]; then
      v=$(sed -n 's:.*<version[^>]*>\([^<]*\)</version>.*:\1:p' "$f" | head -1)
      echo "$p=${v:-unknown}"
      break
    fi
  done
done
python3 -c 'import importlib.metadata as m; print("kompass-core=" + m.version("kompass-core"))' 2>/dev/null || true`
}

// statusPackages prints the version of every EMOS package the probe finds.
// On the dev channel each one shows the commit it was built from instead.
func statusPackages(probe func(script string) (string, error), commits map[string]string) {
	fmt.Println()
	ui.Info("EMOS Packages:")
	out, err := probe(packageProbe())
	if err != nil {
		ui.Warn("  Could not list the packages")
		return
	}
	for _, row := range packageRows(out, commits) {
		if !row.found {
			ui.Error(fmt.Sprintf("  %-28s Not found", row.name))
			continue
		}
		ui.Success(fmt.Sprintf("  %-28s %s", row.name, row.value))
	}
}

type packageRow struct {
	name, value string
	found       bool
}

// packageRows reads the probe's output, one "package=version" line per
// installed package. commits is nil off the dev channel.
func packageRows(out string, commits map[string]string) []packageRow {
	versions := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		if name, version, ok := strings.Cut(strings.TrimSpace(line), "="); ok {
			versions[name] = version
		}
	}
	row := func(name, source string) packageRow {
		value, found := versions[name]
		if found && commits != nil && source != "" {
			value = orUnknown(commits[source])
		}
		return packageRow{name, value, found}
	}
	var rows []packageRow
	for _, pkg := range emosPackages {
		rows = append(rows, row(pkg.name, pkg.source))
	}
	return append(rows, row("kompass-core", ""))
}

// sourceCommits is the short commit of every checkout the install was built
// from, keyed by source. Only on the dev channel, where a build is known by
// its commits rather than a version.
func sourceCommits(cfg *config.EMOSConfig) map[string]string {
	if config.Channel() != "dev" {
		return nil
	}
	switch cfg.Mode {
	case config.ModePixi:
		return workspaceCommits(cfg.PixiProjectDir)
	case config.ModeNative:
		return workspaceCommits(filepath.Join(cfg.WorkspacePath, "src", ".emos-repo"))
	case config.ModeOSSContainer:
		return imageCommits(cfg.ImageTag)
	}
	return nil
}

// workspaceCommits reads the EMOS checkout at dir and its stack submodules.
func workspaceCommits(dir string) map[string]string {
	commits := map[string]string{"emos": gitShortHead(dir)}
	for _, src := range []string{"sugarcoat", "kompass", "embodied-agents"} {
		commits[src] = gitShortHead(filepath.Join(dir, "stack", src))
	}
	return commits
}

// imageCommits reads the commits a nightly image was built from off its
// labels. Images built before the labels existed give none.
func imageCommits(image string) map[string]string {
	commits := parseStackLabel(container.ImageLabel(image, "io.emos.stack"))
	if rev := container.ImageLabel(image, "org.opencontainers.image.revision"); rev != "" {
		commits["emos"] = shortCommit(rev)
	}
	return commits
}

// parseStackLabel reads "sugarcoat@f5ab9cc kompass@5edb164 .. ".
func parseStackLabel(label string) map[string]string {
	commits := map[string]string{}
	for _, entry := range strings.Fields(label) {
		if name, sha, ok := strings.Cut(entry, "@"); ok {
			commits[name] = sha
		}
	}
	return commits
}

// gitShortHead is the short commit checked out at dir, empty when there is
// no checkout.
func gitShortHead(dir string) string {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--short=7", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func shortCommit(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

func orUnknown(commit string) string {
	if commit == "" {
		return "unknown"
	}
	return commit
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
// when the plugin gave no name. On the dev channel it carries the commit the
// plugin runs.
func pluginLabel(p config.PluginInfo) string {
	label := p.Slug
	if name := p.DisplayName(); name != p.Slug {
		label = fmt.Sprintf("%s (%s)", name, p.Slug)
	}
	if config.Channel() == "dev" {
		label += " @ " + orUnknown(gitShortHead(filepath.Join(config.PluginSrcDir(), p.Slug)))
	}
	return label
}
