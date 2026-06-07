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
	Short: "Manage the robot plugin",
	Long: "Manage the robot plugin -- a ROS package that adapts a specific robot to\n" +
		"the EMOS stack. A robot runs one plugin at a time.",
}

func init() {
	pluginCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List robot plugins available in the catalog",
		RunE:  runPluginList,
	})
	pluginCmd.AddCommand(&cobra.Command{
		Use:   "install <plugin>",
		Short: "Install and activate a robot plugin",
		Args:  cobra.ExactArgs(1),
		RunE:  runPluginInstall,
	})
	pluginCmd.AddCommand(&cobra.Command{
		Use:   "inspect",
		Short: "Show the active plugin's interface (feedbacks, commands, actions, events)",
		RunE:  runPluginInspect,
	})
	pluginCmd.AddCommand(&cobra.Command{
		Use:   "remove",
		Short: "Remove the active robot plugin",
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

	active := ""
	if cfg := config.LoadConfig(); cfg != nil && cfg.Plugin != nil {
		active = cfg.Plugin.Slug
	}

	fmt.Println()
	var rows [][]string
	for _, p := range plugins {
		marker := ""
		if p.Filename == active {
			marker = "● active"
		}
		rows = append(rows, []string{p.Filename, p.Name, p.Vendor, marker})
	}
	ui.PrintTable([]string{"PLUGIN", "NAME", "VENDOR", ""}, rows)
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

	if cfg.Plugin != nil && cfg.Plugin.Slug != slug {
		ui.Warn(fmt.Sprintf("A robot runs one plugin at a time. This replaces the active plugin '%s'.", cfg.Plugin.Slug))
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
	ui.SuccessBox(fmt.Sprintf("Plugin '%s' installed and activated.", entry.Name))
	if module, class, ok := strings.Cut(entry.EntryPoint, ":"); ok {
		ui.Faint(fmt.Sprintf("In a recipe: from %s import %s; Launcher(robot_plugin=%s())", module, class, class))
	}
	return nil
}

// runPluginInspect pretty-prints the active plugin's cached describe() tree
// (feedbacks, commands, actions, events).
func runPluginInspect(cmd *cobra.Command, args []string) error {
	cfg := config.LoadConfig()
	if cfg == nil || cfg.Plugin == nil {
		ui.Warn("No plugin is installed. Use 'emos plugin install <plugin>'.")
		return nil
	}
	data, ok := plugin.CachedDescribe()
	if !ok {
		ui.Warn("No cached plugin description found. Try reinstalling the plugin.")
		return nil
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, data, "", "  "); err != nil {
		fmt.Println(string(data))
		return nil
	}
	fmt.Println(pretty.String())
	return nil
}

// runPluginRemove deletes the active plugin and its workspace after
// confirmation.
func runPluginRemove(cmd *cobra.Command, args []string) error {
	cfg := config.LoadConfig()
	if cfg == nil || cfg.Plugin == nil {
		ui.Info("No plugin is installed.")
		return nil
	}
	slug := cfg.Plugin.Slug
	if !ui.Confirm(fmt.Sprintf("Remove the active plugin '%s'?", slug)) {
		return fmt.Errorf("aborted by user")
	}
	if err := plugin.Remove(cfg); err != nil {
		return err
	}
	ui.Success("Plugin '" + slug + "' removed.")
	return nil
}
