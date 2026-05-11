package runner

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// withEnv saves env vars, sets new values for the duration of the test, and
// restores on cleanup. Avoids leaking PATH/HOME mutations across tests.
func withEnv(t *testing.T, kv map[string]string) {
	t.Helper()
	saved := map[string]string{}
	for k := range kv {
		saved[k] = os.Getenv(k)
	}
	for k, v := range kv {
		t.Setenv(k, v)
	}
	_ = saved // t.Setenv handles cleanup on test exit
}

// makeFakePixi creates an executable file named "pixi" at the given path
// (parent dirs created). Returns the absolute path. Skips on Windows since
// the rest of the strategy is *nix-only.
func makeFakePixi(t *testing.T, path string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("pixi strategy is unix-only")
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

	withEnv(t, map[string]string{
		"PATH": binDir,
		"HOME": tmp, // make sure ~/.pixi/bin doesn't accidentally win
	})

	got, err := resolvePixi()
	if err != nil {
		t.Fatalf("resolvePixi err = %v, want nil", err)
	}
	if got != fakeBin {
		t.Errorf("resolvePixi = %q, want %q (from PATH)", got, fakeBin)
	}
}

func TestResolvePixi_FallsBackToHome(t *testing.T) {
	// PATH has nothing useful; the binary lives at ~/.pixi/bin/pixi (where
	// the pixi installer puts it). This is the systemd-mode case the
	// reviewer hit.
	tmp := t.TempDir()
	emptyPath := filepath.Join(tmp, "empty-path")
	_ = os.MkdirAll(emptyPath, 0755)

	homeDir := filepath.Join(tmp, "home")
	expected := makeFakePixi(t, filepath.Join(homeDir, ".pixi", "bin", "pixi"))

	withEnv(t, map[string]string{
		"PATH": emptyPath,
		"HOME": homeDir,
	})

	got, err := resolvePixi()
	if err != nil {
		t.Fatalf("resolvePixi err = %v, want nil", err)
	}
	if got != expected {
		t.Errorf("resolvePixi = %q, want %q (from ~/.pixi/bin)", got, expected)
	}
}

func TestResolvePixi_NotFound(t *testing.T) {
	tmp := t.TempDir()
	emptyPath := filepath.Join(tmp, "empty-path")
	_ = os.MkdirAll(emptyPath, 0755)

	withEnv(t, map[string]string{
		"PATH": emptyPath,
		"HOME": filepath.Join(tmp, "no-pixi-here"),
	})

	if _, err := resolvePixi(); err == nil {
		t.Fatalf("resolvePixi err = nil, want failure when pixi missing everywhere")
	}
}

func TestEnsurePixi_CachesResult(t *testing.T) {
	tmp := t.TempDir()
	binDir := filepath.Join(tmp, "bin")
	fakeBin := makeFakePixi(t, filepath.Join(binDir, "pixi"))

	withEnv(t, map[string]string{
		"PATH": binDir,
		"HOME": tmp,
	})

	s := NewPixiStrategy(tmp)
	if err := s.ensurePixi(); err != nil {
		t.Fatalf("first ensurePixi: %v", err)
	}
	if s.pixiBin != fakeBin {
		t.Fatalf("pixiBin = %q, want %q", s.pixiBin, fakeBin)
	}

	// Now break PATH; the cached value must still serve.
	withEnv(t, map[string]string{
		"PATH": filepath.Join(tmp, "broken"),
		"HOME": filepath.Join(tmp, "no-pixi-here"),
	})
	if err := s.ensurePixi(); err != nil {
		t.Fatalf("second ensurePixi (should hit cache): %v", err)
	}
	if s.pixiBin != fakeBin {
		t.Fatalf("pixiBin changed after caching: %q, want %q", s.pixiBin, fakeBin)
	}
}
