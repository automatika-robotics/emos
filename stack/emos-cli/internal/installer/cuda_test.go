package installer

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// fakeNVCC installs a compiler under root/bin that reports the given release,
// or fails when release is "".
func fakeNVCC(t *testing.T, root, release string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts stand in for nvcc")
	}
	script := "#!/bin/sh\nexit 1\n"
	if release != "" {
		script = "#!/bin/sh\necho 'nvcc: NVIDIA (R) Cuda compiler driver'\n" +
			"echo 'Cuda compilation tools, release " + release + ", V" + release + ".68'\n"
	}
	path := filepath.Join(root, "bin", "nvcc")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDetectCUDAWhereTheInstallersPutIt(t *testing.T) {
	local := t.TempDir()
	fakeNVCC(t, filepath.Join(local, "cuda-12.6"), "12.6")
	if err := os.Symlink("cuda-12.6", filepath.Join(local, "cuda")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir()) // nothing on the PATH

	cuda := detectCUDA(filepath.Join(local, "cuda", "bin", "nvcc"))
	want, _ := filepath.EvalSymlinks(filepath.Join(local, "cuda-12.6"))
	if cuda == nil || cuda.Version != "12.6" || cuda.Root != want {
		t.Errorf("got %+v, want CUDA 12.6 at %s, the directory the link stands for", cuda, want)
	}
}

func TestDetectCUDAOnThePath(t *testing.T) {
	toolkit := t.TempDir()
	fakeNVCC(t, toolkit, "11.4")
	t.Setenv("PATH", filepath.Join(toolkit, "bin"))

	cuda := detectCUDA(filepath.Join(t.TempDir(), "cuda", "bin", "nvcc"))
	want, _ := filepath.EvalSymlinks(toolkit)
	if cuda == nil || cuda.Version != "11.4" || cuda.Root != want {
		t.Errorf("got %+v, want CUDA 11.4 at %s", cuda, want)
	}
}

func TestDetectCUDAWithoutAWorkingToolkit(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if cuda := detectCUDA(filepath.Join(t.TempDir(), "cuda", "bin", "nvcc")); cuda != nil {
		t.Errorf("no toolkit: got %+v", cuda)
	}
	broken := t.TempDir()
	if cuda := detectCUDA(fakeNVCC(t, broken, "")); cuda != nil {
		t.Errorf("a compiler that does not run: got %+v", cuda)
	}
}
