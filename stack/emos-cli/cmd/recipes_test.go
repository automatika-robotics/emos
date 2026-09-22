package cmd

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/automatika-robotics/emos-cli/internal/api"
	"github.com/automatika-robotics/emos-cli/internal/config"
)

func TestRecipeRowsShowWhatThisRobotGets(t *testing.T) {
	generic := api.RecipeVariant{ID: api.GenericVariant}
	lite3 := api.RecipeVariant{ID: "emos-plugin-lite3", Robot: "emos-plugin-lite3"}
	m20 := api.RecipeVariant{ID: "emos-plugin-m20", Robot: "emos-plugin-m20"}
	catalog := []api.Recipe{
		{Filename: "hello", Name: "Hello", Variants: []api.RecipeVariant{generic}},
		{Filename: "follow", Name: "Follow", Variants: []api.RecipeVariant{generic, lite3, m20}},
		{Filename: "patrol", Name: "Patrol", Variants: []api.RecipeVariant{lite3}},
		{Filename: "dock", Name: "Dock", Variants: []api.RecipeVariant{m20}},
	}
	lite3Robot := &config.EMOSConfig{Plugin: &config.PluginInfo{Slug: "emos-plugin-lite3",
		Describe: json.RawMessage(`{"metadata": {"name": "Lite3"}}`)}}
	lite3License := &config.License{Key: "K", PluginSlug: "emos-plugin-lite3"}

	// A licensed Lite3 gets its own versions; the M20-only recipe is not for it
	rows, unlicensed := recipeRows(catalog, lite3Robot, lite3License)
	want := [][]string{{"hello", "Hello", "generic"}, {"follow", "Follow", "Lite3"}, {"patrol", "Patrol", "Lite3"}}
	if !reflect.DeepEqual(rows, want) || unlicensed {
		t.Errorf("licensed Lite3: rows = %v (unlicensed %v), want %v", rows, unlicensed, want)
	}

	// A Lite3 without a license sees the Lite3 versions it is missing, marked
	rows, unlicensed = recipeRows(catalog, lite3Robot, nil)
	want = [][]string{{"hello", "Hello", "generic"}, {"follow", "Follow", "generic *"}}
	if !reflect.DeepEqual(rows, want) || !unlicensed {
		t.Errorf("free Lite3: rows = %v (unlicensed %v), want %v", rows, unlicensed, want)
	}

	// A machine with no plugin sees the generic recipes only, and no marks
	rows, unlicensed = recipeRows(catalog, nil, nil)
	want = [][]string{{"hello", "Hello", "generic"}, {"follow", "Follow", "generic"}}
	if !reflect.DeepEqual(rows, want) || unlicensed {
		t.Errorf("no plugin: rows = %v (unlicensed %v), want %v", rows, unlicensed, want)
	}
}
