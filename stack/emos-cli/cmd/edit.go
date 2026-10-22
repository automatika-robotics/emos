package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/ui"
	"github.com/spf13/cobra"
)

var editCmd = &cobra.Command{
	Use:               "edit <recipe_name>",
	Short:             "Open a recipe in your editor",
	Long:              "Opens ~/emos/recipes/<name>/recipe.py in $VISUAL, then $EDITOR, then nano, then vi.",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeRecipeNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runEdit(args[0])
	},
}

func init() {
	rootCmd.AddCommand(editCmd)
}

func runEdit(name string) error {
	path, err := recipeFile(name)
	if err != nil {
		return err
	}
	editor := editorCommand(os.Getenv("VISUAL"), os.Getenv("EDITOR"))
	c := exec.Command(editor[0], append(editor[1:], path)...)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	return c.Run()
}

// recipeFile is the recipe.py of an installed recipe.
func recipeFile(name string) (string, error) {
	path := filepath.Join(config.RecipesDir, name, "recipe.py")
	if _, err := os.Stat(path); err != nil || name != filepath.Base(name) {
		ui.Error(fmt.Sprintf("Recipe '%s' not found in '%s'", name, config.RecipesDir))
		ui.Faint("Run 'emos ls' to see available recipes.")
		return "", fmt.Errorf("recipe not found")
	}
	return path, nil
}

// editorCommand is the editor and its arguments: $VISUAL, then $EDITOR, then
// nano, then vi. A value may carry arguments, such as "code --wait".
func editorCommand(visual, editor string) []string {
	for _, value := range []string{visual, editor} {
		if fields := strings.Fields(value); len(fields) > 0 {
			return fields
		}
	}
	if _, err := exec.LookPath("nano"); err == nil {
		return []string{"nano"}
	}
	return []string{"vi"}
}
