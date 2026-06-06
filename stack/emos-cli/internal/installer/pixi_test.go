package installer

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// makeFakePixi creates an executable file named "pixi" at the given path
// (parent dirs created) and returns its absolute path. Skips on Windows since
// pixi support is *nix-only.
func makeFakePixi(t *testing.T, path string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("pixi support is unix-only")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho fake-pixi\n"), 0755); err != nil {
		t.Fatal(err)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

func TestResolvePixi_FromPath(t *testing.T) {
	tmp := t.TempDir()
	binDir := filepath.Join(tmp, "bin")
	fakeBin := makeFakePixi(t, filepath.Join(binDir, "pixi"))

	t.Setenv("PATH", binDir)
	t.Setenv("HOME", tmp) // make sure ~/.pixi/bin doesn't accidentally win

	got, err := ResolvePixi()
	if err != nil {
		t.Fatalf("ResolvePixi err = %v, want nil", err)
	}
	if got != fakeBin {
		t.Errorf("ResolvePixi = %q, want %q (from PATH)", got, fakeBin)
	}
}

func TestResolvePixi_FallsBackToHome(t *testing.T) {
	// PATH has nothing useful; the binary lives at ~/.pixi/bin/pixi (where the
	// pixi installer puts it). This is the systemd-mode case where PATH is bare.
	tmp := t.TempDir()
	emptyPath := filepath.Join(tmp, "empty-path")
	_ = os.MkdirAll(emptyPath, 0755)

	homeDir := filepath.Join(tmp, "home")
	expected := makeFakePixi(t, filepath.Join(homeDir, ".pixi", "bin", "pixi"))

	t.Setenv("PATH", emptyPath)
	t.Setenv("HOME", homeDir)

	got, err := ResolvePixi()
	if err != nil {
		t.Fatalf("ResolvePixi err = %v, want nil", err)
	}
	if got != expected {
		t.Errorf("ResolvePixi = %q, want %q (from ~/.pixi/bin)", got, expected)
	}
}

func TestResolvePixi_NotFound(t *testing.T) {
	tmp := t.TempDir()
	emptyPath := filepath.Join(tmp, "empty-path")
	_ = os.MkdirAll(emptyPath, 0755)

	t.Setenv("PATH", emptyPath)
	t.Setenv("HOME", filepath.Join(tmp, "no-pixi-here"))

	if _, err := ResolvePixi(); err == nil {
		t.Fatalf("ResolvePixi err = nil, want failure when pixi missing everywhere")
	}
}
