package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/automatika-robotics/emos-cli/internal/api"
	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/ui"
	"github.com/spf13/cobra"
)

var pullCmd = &cobra.Command{
	Use:   "pull <recipe_name>",
	Short: "Download and install a recipe",
	Args:  cobra.ExactArgs(1),
	RunE:  runPull,
}

func runPull(cmd *cobra.Command, args []string) error {
	name := args[0]
	destDir := filepath.Join(config.RecipesDir, name)

	// Check for existing recipe
	if _, err := os.Stat(destDir); err == nil {
		ui.Warn(fmt.Sprintf("A recipe named '%s' already exists.", name))
		if !ui.Confirm("Continuing will overwrite its contents. Are you sure?") {
			ui.Error("Pull operation aborted.")
			return fmt.Errorf("aborted by user")
		}
	}

	os.MkdirAll(destDir, 0755)

	ui.Info("Installing recipe: " + name)

	err := ui.Spinner("Downloading recipe...", func() error {
		return api.DownloadRecipe(cmd.Context(), name, destDir)
	})
	if err != nil {
		return err
	}

	fmt.Println()
	ui.SuccessBox(fmt.Sprintf("Recipe '%s' installed successfully!", name))
	return nil
}
