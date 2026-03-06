package cmd

import (
	"fmt"

	"github.com/automatika-robotics/emos-cli/internal/api"
	"github.com/automatika-robotics/emos-cli/internal/ui"
	"github.com/spf13/cobra"
)

var recipesCmd = &cobra.Command{
	Use:   "recipes",
	Short: "List available recipes for download",
	RunE:  runRecipes,
}

func runRecipes(cmd *cobra.Command, args []string) error {
	var recipes []api.Recipe
	err := ui.Spinner("Fetching available recipes from server...", func() error {
		var e error
		recipes, e = api.ListRecipes()
		return e
	})
	if err != nil {
		return err
	}

	fmt.Println()
	var rows [][]string
	for _, r := range recipes {
		rows = append(rows, []string{r.Filename, r.Name})
	}

	ui.PrintTable([]string{"RECIPE NAME", "DESCRIPTION"}, rows)
	fmt.Println()
	ui.Faint("Use 'emos pull <recipe_name>' to install a recipe.")
	return nil
}
