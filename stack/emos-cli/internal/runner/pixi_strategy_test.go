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

// Pixi binary resolution moved to internal/installer; the resolver's own
// tests live there (installer/pixi_test.go). This file keeps the
// strategy-level caching test, which exercises ensurePixi.

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
