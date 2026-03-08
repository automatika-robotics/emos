package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/runner"
	"github.com/automatika-robotics/emos-cli/internal/ui"
	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:   "info <recipe_name_or_path>",
	Short: "Show sensor and topic requirements for a recipe",
	Long: `Inspect a recipe's Python source to extract Topic declarations via AST analysis.

Accepts either a recipe name (looked up in ~/emos/recipes/) or a direct path to a .py file.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ui.Banner(config.Version)
		return runInfo(args[0])
	},
}

func init() {
	rootCmd.AddCommand(infoCmd)
}

func runInfo(arg string) error {
	var recipePath string
	var recipeName string

	if strings.HasSuffix(arg, ".py") {
		// Direct file path
		if _, err := os.Stat(arg); os.IsNotExist(err) {
			return fmt.Errorf("file not found: %s", arg)
		}
		recipePath = arg
		recipeName = strings.TrimSuffix(filepath.Base(arg), ".py")
	} else {
		// Recipe name lookup
		recipePath = filepath.Join(config.RecipesDir, arg, "recipe.py")
		if _, err := os.Stat(recipePath); os.IsNotExist(err) {
			ui.Error(fmt.Sprintf("Recipe '%s' not found in '%s'", arg, config.RecipesDir))
			ui.Faint("Run 'emos ls' to see available recipes.")
			return fmt.Errorf("recipe not found")
		}
		recipeName = arg
	}

	topics, err := runner.ExtractTopics(recipePath)
	if err != nil {
		return err
	}

	// Determine distro from config or default
	distro := "jazzy"
	if cfg := config.LoadConfig(); cfg != nil && cfg.ROSDistro != "" {
		distro = cfg.ROSDistro
	}

	runner.DisplayTopicInfo(recipeName, topics, distro)
	return nil
}
