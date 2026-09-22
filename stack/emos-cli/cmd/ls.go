package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/automatika-robotics/emos-cli/internal/api"
	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/ui"
	"github.com/spf13/cobra"
)

var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List locally installed recipes",
	RunE:  runLs,
}

func runLs(cmd *cobra.Command, args []string) error {
	entries, err := os.ReadDir(config.RecipesDir)
	if err != nil {
		ui.Faint("No recipes found in " + config.RecipesDir)
		return nil
	}
	cfg := config.LoadConfig()

	var rows [][]string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		description, version := "-", "-"

		manifestPath := filepath.Join(config.RecipesDir, name, "manifest.json")
		if data, err := os.ReadFile(manifestPath); err == nil {
			var manifest struct {
				Name    string            `json:"name"`
				Variant api.RecipeVariant `json:"variant"`
			}
			if json.Unmarshal(data, &manifest) == nil {
				if manifest.Name != "" {
					description = manifest.Name
				}
				if manifest.Variant.ID != "" {
					version = api.VariantLabel(manifest.Variant, cfg)
				}
			}
		}

		rows = append(rows, []string{name, description, version})
	}

	if len(rows) == 0 {
		ui.Faint("No recipes found in " + config.RecipesDir)
		return nil
	}

	fmt.Println()
	ui.PrintTable([]string{"NAME", "DESCRIPTION", "VERSION"}, rows)
	return nil
}
