package cmd

import (
	"errors"
	"fmt"

	"github.com/automatika-robotics/emos-cli/internal/api"
	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/ui"
	"github.com/spf13/cobra"
)

var recipesCmd = &cobra.Command{
	Use:   "recipes",
	Short: "List the recipes in the catalog that this robot can use",
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

	cfg, lic := config.LoadConfig(), config.LoadLicense()
	rows, unlicensed := recipeRows(recipes, cfg, lic)

	fmt.Println()
	ui.PrintTable([]string{"RECIPE NAME", "DESCRIPTION", "VERSION"}, rows)
	fmt.Println()
	ui.Faint("Use 'emos pull <recipe_name>' to install a recipe.")
	if unlicensed {
		ui.Faint("* made for your robot, available with an EMOS license: " + config.SalesEmail)
	}
	return nil
}

// recipeRows lists the recipes this machine can use, with the version it
// would get.
func recipeRows(recipes []api.Recipe, cfg *config.EMOSConfig, lic *config.License) (rows [][]string, unlicensed bool) {
	for _, r := range recipes {
		choice, err := api.ChooseVariantFor(r, cfg, lic)
		var needs *api.NeedsLicenseError
		if errors.As(err, &needs) || errors.Is(err, api.ErrNotForThisRobot) {
			continue
		}
		version := api.VariantLabel(choice.Variant, cfg)
		if choice.Unlicensed != "" && lic == nil {
			version += " *"
			unlicensed = true
		}
		rows = append(rows, []string{r.Filename, r.Name, version})
	}
	return rows, unlicensed
}
