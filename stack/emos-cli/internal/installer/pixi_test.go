package installer

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
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

func TestInstallMappingBackendAddsItsROSPackageThenRunsTheTask(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("pixi support is unix-only")
	}
	bin, project := t.TempDir(), t.TempDir()
	record := filepath.Join(t.TempDir(), "record")
	// Records each call, and fails the one the project names in "fail".
	fake := "#!/bin/sh\necho \"$PWD|$*|$EMOS_MAPPING_CUDA\" >> " + record +
		"\n! grep -q \"^$1$\" fail 2>/dev/null\n"
	if err := os.WriteFile(filepath.Join(bin, "pixi"), []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	resolved, _ := filepath.EvalSymlinks(project)
	add := resolved + "|" + strings.Join(PixiAddArgs(filepath.Join(project, "pixi.toml"),
		[]string{"ros-jazzy-image-transport"}, runtime.GOARCH), " ") + "|/usr/local/cuda-12.6\n"
	task := resolved + "|run install-mapping-backend|/usr/local/cuda-12.6\n"

	env := append(os.Environ(), "EMOS_MAPPING_CUDA=/usr/local/cuda-12.6")
	if err := InstallMappingBackend(project, "jazzy", env); err != nil {
		t.Fatalf("InstallMappingBackend: %v", err)
	}
	if got, _ := os.ReadFile(record); string(got) != add+task {
		t.Errorf("pixi was called as\n%s\nwant\n%s", got, add+task)
	}

	// A package that cannot be added ends it before anything is built.
	os.Remove(record)
	os.WriteFile(filepath.Join(project, "fail"), []byte("add\n"), 0o644)
	if err := InstallMappingBackend(project, "jazzy", env); err == nil {
		t.Error("a failed add must be reported")
	}
	if got, _ := os.ReadFile(record); string(got) != add {
		t.Errorf("nothing should be built after a failed add, pixi was called as\n%s", got)
	}

	os.WriteFile(filepath.Join(project, "fail"), []byte("run\n"), 0o644)
	if err := InstallMappingBackend(project, "jazzy", env); err == nil {
		t.Error("a failed build must be reported")
	}
}

func TestPixiAddArgsResolveOnlyAarch64OnTheRobot(t *testing.T) {
	pkgs := []string{"ros-jazzy-livox-ros-driver2", "libpcap"}

	got := PixiAddArgs("/emos/pixi.toml", pkgs, "arm64")
	want := []string{"add", "--manifest-path", "/emos/pixi.toml",
		"--platform", "linux-aarch64", "ros-jazzy-livox-ros-driver2", "libpcap"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("arm64 = %v, want %v", got, want)
	}

	// An x86 dev machine keeps resolving every platform, aarch64 included.
	got = PixiAddArgs("/emos/pixi.toml", pkgs, "amd64")
	want = []string{"add", "--manifest-path", "/emos/pixi.toml", "ros-jazzy-livox-ros-driver2", "libpcap"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("amd64 = %v, want %v", got, want)
	}
}
