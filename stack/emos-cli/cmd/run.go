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
		if plugin.Busy() {
			ui.Error("Plugins are being installed, updated or removed; a recipe would load a half-built overlay.")
			ui.Faint("Try again once that finishes.")
			return fmt.Errorf("plugins are busy")
		}
		return runner.RunRecipe(args[0], rmwFlag)
	},
}

func init() {
	runCmd.Flags().StringVar(&rmwFlag, "rmw", "",
		"RMW implementation (rmw_fastrtps_cpp, rmw_cyclonedds_cpp, rmw_zenoh_cpp); "+
			"unset keeps the environment's, or ROS's default")
}
