package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/automatika-robotics/emos-cli/internal/config"
)

func TestSafeRecipeDirRejectsSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks differ on windows")
	}
	withTempConfig(t)

	// Construct: <recipesDir>/escaper -> /etc
	// safeRecipeDir() must refuse it because the resolved path leaves
	// RecipesDir.
	if err := os.MkdirAll(config.RecipesDir, 0o755); err != nil {
		t.Fatalf("mkdir RecipesDir: %v", err)
	}
	target := "/etc"
	link := filepath.Join(config.RecipesDir, "escaper")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	if _, err := safeRecipeDir("escaper"); err == nil {
		t.Fatalf("safeRecipeDir(escaper) should reject the escape")
	}
}

func TestSafeRecipeDirAcceptsRealRecipe(t *testing.T) {
	withTempConfig(t)

	dir := filepath.Join(config.RecipesDir, "demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	got, err := safeRecipeDir("demo")
	if err != nil {
		t.Fatalf("safeRecipeDir(demo): %v", err)
	}
	// EvalSymlinks may canonicalise (e.g. /tmp -> /private/tmp on macOS).
	resolvedDir, _ := filepath.EvalSymlinks(dir)
	if got != resolvedDir {
		t.Fatalf("safeRecipeDir = %q, want %q", got, resolvedDir)
	}
}

func TestSafeRecipeDirAcceptsMissingPath(t *testing.T) {
	// `emos pull` calls into safeRecipeDir-equivalent paths before the
	// recipe directory exists. The lexical join is safe because the name
	// passes validRecipeName.
	withTempConfig(t)
	if err := os.MkdirAll(config.RecipesDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	got, err := safeRecipeDir("not-yet-pulled")
	if err != nil {
		t.Fatalf("safeRecipeDir(not-yet-pulled): %v", err)
	}
	if got == "" {
		t.Fatalf("safeRecipeDir returned empty path")
	}
}

func TestHandleRecipeDetailRejectsSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks differ on windows")
	}
	s := newTestServer(t, true)

	// Plant the same escape and verify the handler returns 400, not 200
	// with a peek at /etc.
	if err := os.MkdirAll(config.RecipesDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Symlink("/etc", filepath.Join(config.RecipesDir, "escaper")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/recipes/escaper", nil)
	rec := httpServe(t, s, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (escape must be rejected)", rec.Code)
	}
}
