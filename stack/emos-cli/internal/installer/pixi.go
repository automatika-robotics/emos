package installer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// PixiInstallHint is the one-liner shown to users who need to install pixi.
const PixiInstallHint = "curl -fsSL https://pixi.sh/install.sh | bash"

// ResolvePixi finds the pixi binary in priority order:
//  1. PATH (works in interactive shells).
//  2. ~/.pixi/bin/pixi (the pixi installer's standard target).
//  3. /usr/local/bin/pixi (system-wide installs).
//
// Returns an absolute path, or an error listing the locations checked.
func ResolvePixi() (string, error) {
	if p, err := exec.LookPath("pixi"); err == nil {
		return p, nil
	}
	var checked []string
	if home, err := os.UserHomeDir(); err == nil {
		p := filepath.Join(home, ".pixi", "bin", "pixi")
		checked = append(checked, p)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	checked = append(checked, "/usr/local/bin/pixi")
	if st, err := os.Stat("/usr/local/bin/pixi"); err == nil && !st.IsDir() {
		return "/usr/local/bin/pixi", nil
	}
	return "", fmt.Errorf(
		"pixi not found in PATH or %v -- install it from https://pixi.sh", checked)
}

// PixiAddArgs is the pixi add invocation for pkgs on a goarch host.
//
// On an aarch64 robot the packages are resolved for aarch64 alone, so one
// missing only on x86 cannot block the install. An x86 host still resolves both
func PixiAddArgs(manifest string, pkgs []string, goarch string) []string {
	args := []string{"add", "--manifest-path", manifest}
	if goarch == "arm64" {
		args = append(args, "--platform", "linux-aarch64")
	}
	return append(args, pkgs...)
}

// RunPixi runs pixi with args in the workspace at projectDir, with the output on
// the terminal.
func RunPixi(projectDir string, env []string, args ...string) error {
	pixiBin, err := ResolvePixi()
	if err != nil {
		return err
	}
	cmd := exec.Command(pixiBin, args...)
	cmd.Dir = projectDir
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		step := args[0]
		if step == "run" {
			step += " " + args[1]
		}
		return fmt.Errorf("pixi %s failed: %w", step, err)
	}
	return nil
}

// mappingBackendTask is the pixi task that builds the native mapping backend.
const mappingBackendTask = "install-mapping-backend"

// MappingBackendROSPackages are the ROS packages the backend needs that are not
// in a stock EMOS environment, named without the "ros-<distro>-" prefix.
var MappingBackendROSPackages = []string{
	"image-transport",
}

// InstallMappingBackend builds the native mapping backend into the pixi
// workspace at projectDir, with the output on the terminal.
func InstallMappingBackend(projectDir, rosDistro string, env []string) error {
	pkgs := make([]string, len(MappingBackendROSPackages))
	for i, name := range MappingBackendROSPackages {
		pkgs[i] = "ros-" + rosDistro + "-" + name
	}
	add := PixiAddArgs(filepath.Join(projectDir, "pixi.toml"), pkgs, runtime.GOARCH)
	if err := RunPixi(projectDir, env, add...); err != nil {
		return err
	}
	return RunPixi(projectDir, env, "run", mappingBackendTask)
}

// cudaPackagesTask is the pixi task that builds CUDAPackages into cudaWheelsDir.
const cudaPackagesTask = "install-cuda-packages"

// cudaWheelsDir holds the CUDA wheels in the pixi workspace. pixi installs them
// from there, so they have to stay.
const cudaWheelsDir = "cuda_wheels"

// CUDAPackages are the PyPI packages EMOS rebuilds from source to use CUDA.
var CUDAPackages = []string{"sherpa-onnx", "llama-cpp-python"}

// cudaWheelSpecs returns "<package> @ file://<wheel>" for each of CUDAPackages,
// from the wheels the build left.
func cudaWheelSpecs(projectDir string) ([]string, error) {
	built, err := filepath.Glob(filepath.Join(projectDir, cudaWheelsDir, "*.whl"))
	if err != nil {
		return nil, err
	}
	wheels := map[string]string{}
	for _, wheel := range built {
		// <distribution>-<version>-...whl, with the package's dashes as underscores
		dist, _, _ := strings.Cut(filepath.Base(wheel), "-")
		wheels[strings.ReplaceAll(dist, "_", "-")] = wheel
	}
	specs := make([]string, len(CUDAPackages))
	for i, pkg := range CUDAPackages {
		if wheels[pkg] == "" {
			return nil, fmt.Errorf("the build left no wheel for %s in %s", pkg, filepath.Join(projectDir, cudaWheelsDir))
		}
		specs[i] = pkg + " @ file://" + wheels[pkg]
	}
	return specs, nil
}

// HasCUDAPackages reports whether the manifest of the workspace at projectDir
// names the CUDA wheels.
func HasCUDAPackages(projectDir string) bool {
	manifest, err := os.ReadFile(filepath.Join(projectDir, "pixi.toml"))
	return err == nil && strings.Contains(string(manifest), "/"+cudaWheelsDir+"/")
}

// InstallCUDAPackages builds CUDAPackages with the CUDA toolkit at cudaRoot in
// the pixi workspace at projectDir, with the output on the terminal, and hands
// the wheels to pixi.
func InstallCUDAPackages(projectDir, cudaRoot string, env []string) error {
	env = append(env, "EMOS_CUDA="+cudaRoot)
	if err := RunPixi(projectDir, env, "run", cudaPackagesTask); err != nil {
		return err
	}
	specs, err := cudaWheelSpecs(projectDir)
	if err != nil {
		return err
	}
	return RunPixi(projectDir, env, cudaPackagesArgs("add", specs, runtime.GOARCH)...)
}

// RemoveCUDAPackages puts the workspace at projectDir back on the repository's
// packages, and forgets the wheels. An update does this before it pulls.
func RemoveCUDAPackages(projectDir string, env []string) error {
	if err := RunPixi(projectDir, env, cudaPackagesArgs("remove", CUDAPackages, runtime.GOARCH)...); err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(projectDir, cudaWheelsDir))
}

// cudaPackagesArgs is the pixi add or remove of PyPI pkgs for the platform of a
// goarch host alone.
func cudaPackagesArgs(verb string, pkgs []string, goarch string) []string {
	platform := "linux-64"
	if goarch == "arm64" {
		platform = "linux-aarch64"
	}
	return append([]string{verb, "--pypi", "--platform", platform}, pkgs...)
}
