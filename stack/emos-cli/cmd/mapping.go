package cmd

import (
	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/runner"
	"github.com/automatika-robotics/emos-cli/internal/ui"
	"github.com/spf13/cobra"
)

var mapCmd = &cobra.Command{
	Use:   "map",
	Short: "Manage mapping functions",
}

var mapRecordCmd = &cobra.Command{
	Use:   "record",
	Short: "Start recording a new map (run on-board the robot)",
	RunE: func(cmd *cobra.Command, args []string) error {
		ui.Banner(config.Version)
		return runner.RunMapping()
	},
}

var mapInstallEditorCmd = &cobra.Command{
	Use:   "install-editor",
	Short: "Download and install the map editor container",
	RunE: func(cmd *cobra.Command, args []string) error {
		ui.Banner(config.Version)
		return runner.InstallMapEditor()
	},
}

var mapEditCmd = &cobra.Command{
	Use:   "edit <path_to_bag_file.tar.gz>",
	Short: "Process a ROS bag file to generate a PCD map",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ui.Banner(config.Version)
		return runner.EditMap(args[0])
	},
}

func init() {
	mapCmd.AddCommand(mapRecordCmd)
	mapCmd.AddCommand(mapInstallEditorCmd)
	mapCmd.AddCommand(mapEditCmd)
	_ = config.Version // ensure config is referenced
}
