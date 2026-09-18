package cmd

import (
	"fmt"

	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/plugin"
	"github.com/automatika-robotics/emos-cli/internal/runner"
	"github.com/automatika-robotics/emos-cli/internal/ui"
	"github.com/spf13/cobra"
)

var rmwFlag string

var runCmd = &cobra.Command{
	Use:   "run <recipe_name>",
	Short: "Execute an automation recipe",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ui.Banner(config.Version)
		if err := refuseWhilePluginsBusy("a recipe"); err != nil {
			return err
		}
		return runner.RunRecipe(args[0], rmwFlag)
	},
}

// refuseWhilePluginsBusy says why what cannot start while a plugin operation is
// rebuilding the overlay it would load.
func refuseWhilePluginsBusy(what string) error {
	if !plugin.Busy() {
		return nil
	}
	ui.Error(fmt.Sprintf("Plugins are being installed, updated or removed; %s would load a half-built overlay.", what))
	ui.Faint("Try again once that finishes.")
	return fmt.Errorf("plugins are busy")
}

func init() {
	runCmd.Flags().StringVar(&rmwFlag, "rmw", "",
		"RMW implementation ("+runner.RMWChoices+"); "+
			"unset keeps the environment's, or ROS's default")
}
