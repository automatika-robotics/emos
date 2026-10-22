package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/automatika-robotics/emos-cli/internal/api"
	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/ui"
	"github.com/spf13/cobra"
)

var pullGeneric bool

var pullCmd = &cobra.Command{
	Use:   "pull <recipe_name>",
	Short: "Download and install a recipe",
	Long: "Download a recipe from the catalog and install it.\n\n" +
		"Some recipes have a version made for a specific robot. You get that version when\n" +
		"you have a license for that robot.",
	Args: cobra.ExactArgs(1),
	RunE: runPull,
}

func init() {
	pullCmd.Flags().BoolVar(&pullGeneric, "generic", false, "Install the generic version, whatever robot is installed")
}

func runPull(cmd *cobra.Command, args []string) error {
	name := args[0]
	cfg, lic := config.LoadConfig(), config.LoadLicense()

	recipe, err := findRecipe(name)
	if err != nil {
		return err
	}
	choice, err := chooseVariant(recipe, cfg, lic)
	if err != nil {
		return err
	}
	version := api.VariantLabel(choice.Variant, cfg) + " version"

	// Check for existing recipe
	if _, err := os.Stat(filepath.Join(config.RecipesDir, name)); err == nil {
		ui.Warn(fmt.Sprintf("A recipe named '%s' already exists.", name))
		if !ui.Confirm("Continuing will overwrite its contents. Are you sure?") {
			ui.Error("Pull operation aborted.")
			return fmt.Errorf("aborted by user")
		}
	}

	ui.Info("Installing recipe: " + name + " (" + version + ")")
	if choice.Unlicensed != "" && lic == nil {
		ui.Faint("A version made for the " + cfg.PluginLabel(choice.Unlicensed) + " is available with a license: " + config.SalesEmail)
	}

	var key string
	if choice.Variant.ID != api.GenericVariant {
		key = lic.Key
	}
	err = ui.Spinner("Downloading recipe...", func() error {
		return api.DownloadRecipe(cmd.Context(), name, choice.Variant.ID, key, config.RecipesDir)
	})
	if err != nil {
		return explainPull(err)
	}

	fmt.Println()
	ui.SuccessBox(fmt.Sprintf("Recipe '%s' installed successfully (%s)!", name, version))
	return nil
}

// findRecipe looks name up in the portal's catalog.
func findRecipe(name string) (api.Recipe, error) {
	var recipes []api.Recipe
	err := ui.Spinner("Looking up the recipe in the catalog...", func() error {
		var e error
		recipes, e = api.ListRecipes()
		return e
	})
	if err != nil {
		return api.Recipe{}, err
	}
	for _, r := range recipes {
		if r.Filename == name {
			return r, nil
		}
	}
	ui.Error(fmt.Sprintf("The catalog has no recipe named '%s'.", name))
	ui.Faint("'emos recipes' lists what it has.")
	return api.Recipe{}, fmt.Errorf("no such recipe")
}

// chooseVariant settles which version of recipe this machine gets, and
// explains it to the operator when it gets none.
func chooseVariant(recipe api.Recipe, cfg *config.EMOSConfig, lic *config.License) (api.VariantChoice, error) {
	if pullGeneric {
		for _, v := range recipe.Variants {
			if v.ID == api.GenericVariant {
				return api.VariantChoice{Variant: v}, nil
			}
		}
		ui.Error("This recipe has no generic version.")
		return api.VariantChoice{}, fmt.Errorf("no generic version")
	}

	choice, err := api.ChooseVariantFor(recipe, cfg, lic)
	var needs *api.NeedsLicenseError
	switch {
	case errors.As(err, &needs):
		ui.Error("This recipe comes in a version for the " + cfg.PluginLabel(needs.Robot) + " only, which needs a license for that robot.")
		if lic != nil {
			ui.Faint("The license on this machine is for the " + licensedRobot(lic) + ".")
		} else {
			ui.Faint("Activate one with 'emos license activate <key>'.")
			licenseNudge()
		}
		return choice, fmt.Errorf("license needed")
	case errors.Is(err, api.ErrNotForThisRobot):
		ui.Error("This recipe is not available for your robot.")
		ui.Faint("It has no generic version, and none for the robot and sensor plugins installed here.")
		return choice, fmt.Errorf("not available for this robot")
	}
	return choice, err
}

// explainPull tells the operator why the portal would not give the recipe.
func explainPull(err error) error {
	var refused *api.RecipeRefusedError
	switch {
	case errors.Is(err, api.ErrInvalidLicense):
		ui.Error("The portal no longer accepts the license kept on this machine.")
		ui.Faint("Ask about it at " + config.SupportURL + ", or install the generic version with --generic.")
		return fmt.Errorf("license not accepted")
	case errors.As(err, &refused):
		ui.Error(refused.Reason)
		ui.Faint("'emos pull --generic' installs the generic version, if the recipe has one.")
		return fmt.Errorf("license does not cover this recipe")
	}
	return err
}
