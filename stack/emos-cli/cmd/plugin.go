package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/automatika-robotics/emos-cli/internal/api"
	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/plugin"
	"github.com/automatika-robotics/emos-cli/internal/ui"
	"github.com/spf13/cobra"
)

var pluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "Manage robot and sensor plugins",
	Long: "Manage plugins -- ROS packages that adapt hardware to the EMOS stack. A\n" +
		"robot runs one robot plugin plus any number of sensor plugins.",
}

func init() {
	pluginCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List plugins available in the catalog",
		RunE:  runPluginList,
	})
	pluginCmd.AddCommand(&cobra.Command{
		Use:   "install <plugin>",
		Short: "Install a plugin (a robot replaces the current robot; a sensor is added)",
		Args:  cobra.ExactArgs(1),
		RunE:  runPluginInstall,
	})
	pluginCmd.AddCommand(&cobra.Command{
		Use:   "inspect [slug]",
		Short: "Show an installed plugin's interface (feedbacks, commands, actions, events)",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runPluginInspect,
	})
	pluginCmd.AddCommand(&cobra.Command{
		Use:   "remove [slug]",
		Short: "Remove an installed plugin, or all of them if no slug is given",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runPluginRemove,
	})
}

// runPluginList prints the plugin catalog from the portal, flagging the
// currently active plugin.
func runPluginList(cmd *cobra.Command, args []string) error {
	var plugins []api.Plugin
	err := ui.Spinner("Fetching plugins from the catalog...", func() error {
		var e error
		plugins, e = api.ListPlugins()
		return e
	})
	if err != nil {
		return err
	}

	installed := map[string]bool{}
	if cfg := config.LoadConfig(); cfg != nil {
		for _, p := range cfg.Plugins() {
			installed[p.Slug] = true
		}
	}

	fmt.Println()
	var rows [][]string
	for _, p := range plugins {
		marker := ""
		if installed[p.Filename] {
			marker = "● installed"
		}
		role := p.Role
		if role == "" {
			role = config.RoleRobot
		}
		rows = append(rows, []string{p.Filename, p.Name, p.Vendor, role, marker})
	}
	ui.PrintTable([]string{"PLUGIN", "NAME", "VENDOR", "ROLE", ""}, rows)
	fmt.Println()
	ui.Faint("Install with: emos plugin install <plugin>")
	return nil
}

// runPluginInstall resolves a plugin slug from the catalog, then clones,
// builds, and activates it, replacing any currently active plugin.
func runPluginInstall(cmd *cobra.Command, args []string) error {
	slug := args[0]

	cfg := config.LoadConfig()
	if !cfg.IsInstalled() {
		ui.Error("EMOS is not installed. Run 'emos install' first.")
		return fmt.Errorf("not installed")
	}

	var entry api.Plugin
	err := ui.Spinner("Looking up plugin in the catalog...", func() error {
		var e error
		entry, e = plugin.Resolve(slug)
		return e
	})
	if err != nil {
		return err
	}

	if entry.Role == config.RoleRobot && cfg.Plugin != nil && cfg.Plugin.Slug != slug {
		ui.Warn(fmt.Sprintf("A robot runs one robot plugin at a time. This replaces the current robot '%s'.", cfg.Plugin.Slug))
		if !ui.Confirm("Continue?") {
			return fmt.Errorf("aborted by user")
		}
	}

	ui.Header("INSTALLING PLUGIN: " + entry.Name)
	ui.Faint("Source: " + entry.Repo)
	ui.Faint("This clones and builds the plugin; it can take a few minutes.")
	fmt.Println()

	if err := plugin.Install(cfg, entry, os.Stdout); err != nil {
		return err
	}

	fmt.Println()
	ui.SuccessBox(fmt.Sprintf("Plugin '%s' installed.", entry.Name))
	if module, class, ok := strings.Cut(entry.EntryPoint, ":"); ok {
		if entry.Role == config.RoleSensor {
			ui.Faint(fmt.Sprintf("In a recipe: from %s import %s; Launcher(..., sensor_plugins=[%s()])", module, class, class))
		} else {
			ui.Faint(fmt.Sprintf("In a recipe: from %s import %s; Launcher(robot_plugin=%s())", module, class, class))
		}
	}
	return nil
}

// runPluginInspect pretty-prints an installed plugin's cached describe() tree.
// With no slug it shows the robot (or the first sensor when there is no robot).
func runPluginInspect(cmd *cobra.Command, args []string) error {
	cfg := config.LoadConfig()
	if cfg == nil || len(cfg.Plugins()) == 0 {
		ui.Warn("No plugin is installed. Use 'emos plugin install <plugin>'.")
		return nil
	}
	var pi *config.PluginInfo
	switch {
	case len(args) == 1:
		if pi = cfg.FindPlugin(args[0]); pi == nil {
			ui.Warn(fmt.Sprintf("Plugin '%s' is not installed.", args[0]))
			return nil
		}
	case cfg.Plugin != nil:
		pi = cfg.Plugin
	case len(cfg.SensorPlugins) > 0:
		pi = &cfg.SensorPlugins[0]
	}
	if pi == nil || len(pi.Describe) == 0 {
		ui.Warn("No cached description found. Try reinstalling the plugin.")
		return nil
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, pi.Describe, "", "  "); err != nil {
		fmt.Println(string(pi.Describe))
		return nil
	}
	fmt.Println(pretty.String())
	return nil
}

// runPluginRemove removes one installed plugin by slug, or all of them when no
// slug is given, after confirmation.
func runPluginRemove(cmd *cobra.Command, args []string) error {
	cfg := config.LoadConfig()
	if cfg == nil || len(cfg.Plugins()) == 0 {
		ui.Info("No plugin is installed.")
		return nil
	}

	if len(args) == 1 {
		slug := args[0]
		if cfg.FindPlugin(slug) == nil {
			ui.Warn(fmt.Sprintf("Plugin '%s' is not installed.", slug))
			return nil
		}
		if !ui.Confirm(fmt.Sprintf("Remove plugin '%s'?", slug)) {
			return fmt.Errorf("aborted by user")
		}
		if err := plugin.Remove(cfg, slug, os.Stdout); err != nil {
			return err
		}
		ui.Success("Plugin '" + slug + "' removed.")
		return nil
	}

	var slugs []string
	for _, p := range cfg.Plugins() {
		slugs = append(slugs, p.Slug)
	}
	if !ui.Confirm(fmt.Sprintf("Remove all %d plugins (%s)?", len(slugs), strings.Join(slugs, ", "))) {
		return fmt.Errorf("aborted by user")
	}
	if err := plugin.RemoveAll(cfg); err != nil {
		return err
	}
	ui.Success("All plugins removed.")
	return nil
}
