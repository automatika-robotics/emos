package cmd

import (
	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/ui"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "emos",
	Short: "EmbodiedOS Management CLI",
	Long:  "EMOS CLI manages the EmbodiedOS container, recipes, and mapping on your robot.",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		config.Init()
	},
	Run: func(cmd *cobra.Command, args []string) {
		ui.Banner(config.Version)
		cmd.Help()
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show the current CLI version",
	Run: func(cmd *cobra.Command, args []string) {
		ui.Banner(config.Version)
		ui.StatusCard(config.Version)
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(lsCmd)
	rootCmd.AddCommand(recipesCmd)
	rootCmd.AddCommand(pullCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(mapCmd)
	// serveCmd registers in init() within serve.go
}
