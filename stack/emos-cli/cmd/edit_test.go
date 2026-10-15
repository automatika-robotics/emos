package cmd

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/automatika-robotics/emos-cli/internal/config"
)

func TestEditorCommandPrecedence(t *testing.T) {
	if got := editorCommand("code --wait", "vim"); !reflect.DeepEqual(got, []string{"code", "--wait"}) {
		t.Errorf("$VISUAL with arguments: got %v", got)
	}
	if got := editorCommand("", "vim"); !reflect.DeepEqual(got, []string{"vim"}) {
		t.Errorf("$EDITOR when $VISUAL is empty: got %v", got)
	}
	if got := editorCommand("  ", ""); len(got) != 1 || (got[0] != "nano" && got[0] != "vi") {
		t.Errorf("fallback with neither set: got %v", got)
	}
}

func TestRecipeFileFindsOnlyInstalledRecipes(t *testing.T) {
	config.RecipesDir = t.TempDir()
	if err := os.MkdirAll(filepath.Join(config.RecipesDir, "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(config.RecipesDir, "demo", "recipe.py"), []byte("pass\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(config.RecipesDir, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}

	path, err := recipeFile("demo")
	if err != nil || path != filepath.Join(config.RecipesDir, "demo", "recipe.py") {
		t.Errorf("installed recipe: path %q, err %v", path, err)
	}
	for _, name := range []string{"missing", "empty", "../demo", "demo/recipe.py"} {
		if _, err := recipeFile(name); err == nil {
			t.Errorf("%q: expected an error", name)
		}
	}
}

func TestInstalledRecipeNamesAreTheDirectories(t *testing.T) {
	config.RecipesDir = t.TempDir()
	for _, dir := range []string{"patrol", "follow", ".hidden"} {
		if err := os.Mkdir(filepath.Join(config.RecipesDir, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(config.RecipesDir, "notes.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := installedRecipeNames(); !reflect.DeepEqual(got, []string{"follow", "patrol"}) {
		t.Errorf("got %v", got)
	}
	config.RecipesDir = filepath.Join(t.TempDir(), "absent")
	if got := installedRecipeNames(); got != nil {
		t.Errorf("no recipes directory: got %v", got)
	}
}
