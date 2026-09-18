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
	pixiBin, err := ResolvePixi()
	if err != nil {
		return err
	}
	pkgs := make([]string, len(MappingBackendROSPackages))
	for i, name := range MappingBackendROSPackages {
		pkgs[i] = "ros-" + rosDistro + "-" + name
	}
	steps := [][]string{
		PixiAddArgs(filepath.Join(projectDir, "pixi.toml"), pkgs, runtime.GOARCH),
		{"run", mappingBackendTask},
	}
	for _, args := range steps {
		step := exec.Command(pixiBin, args...)
		step.Dir = projectDir
		step.Env = env
		step.Stdout = os.Stdout
		step.Stderr = os.Stderr
		if err := step.Run(); err != nil {
			return fmt.Errorf("pixi %s failed: %w", strings.Join(args[:2], " "), err)
		}
	}
	return nil
}
