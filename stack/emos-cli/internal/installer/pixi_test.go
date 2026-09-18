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

func TestInstallMappingBackendRunsTheTaskInTheProject(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("pixi support is unix-only")
	}
	bin, project := t.TempDir(), t.TempDir()
	record := filepath.Join(t.TempDir(), "record")
	// Records how it was called, then fails when the project asks it to.
	fake := "#!/bin/sh\necho \"$PWD|$*|$SKBUILD_STRICT_CONFIG\" > " + record + "\ntest ! -e fail\n"
	if err := os.WriteFile(filepath.Join(bin, "pixi"), []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	env := append(os.Environ(), "SKBUILD_STRICT_CONFIG=false")
	if err := InstallMappingBackend(project, env); err != nil {
		t.Fatalf("InstallMappingBackend: %v", err)
	}
	got, _ := os.ReadFile(record)
	resolved, _ := filepath.EvalSymlinks(project)
	if want := resolved + "|run install-mapping-backend|false\n"; string(got) != want {
		t.Errorf("pixi was called as %q, want %q", got, want)
	}

	os.WriteFile(filepath.Join(project, "fail"), nil, 0o644)
	if err := InstallMappingBackend(project, env); err == nil {
		t.Error("a failed build must be reported")
	}
}
