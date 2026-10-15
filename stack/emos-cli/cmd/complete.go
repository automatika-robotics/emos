package cmd

import (
	"os"
	"sort"

	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/spf13/cobra"
)

// completeRecipeNames offers the installed recipes for a command's one argument.
func completeRecipeNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return installedRecipeNames(), cobra.ShellCompDirectiveNoFileComp
}

// completeRecipeNamesOrFiles is completeRecipeNames for a command that also
// takes a path, so the shell keeps completing files.
func completeRecipeNamesOrFiles(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveDefault
	}
	return installedRecipeNames(), cobra.ShellCompDirectiveDefault
}

// installedRecipeNames lists the recipe directories, sorted.
func installedRecipeNames() []string {
	if config.RecipesDir == "" {
		config.Init()
	}
	entries, err := os.ReadDir(config.RecipesDir)
	if err != nil {
		return nil
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() && entry.Name()[0] != '.' {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names
}
