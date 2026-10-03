package plugin

// Dependency resolution for a plugin's declared driver dependencies.
//
// Each plugin declares what a fresh environment needs in its emos-plugin.yaml
// manifest. Resolution happens per install mode:
//
//   - pixi: `pixi add` the manifest's ROS packages (as ros-<distro>-*) and its
//     conda system packages;
//   - native: let rosdep resolve everything under the workspace to apt. It
//     reads the plugin's and its sources' package.xml directly, conditions and
//     system libs included;
//   - container: print exact install instructions, since an ephemeral build
//     can't persist packages into the image.

import (
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/installer"
)

// stackPackages are the EMOS packages built from source into the environment;
// rosdep must skip them on native installs.
var stackPackages = []string{
	"automatika_ros_sugar", "kompass", "automatika_embodied_agents", "kompass_interfaces", "emos_mapping",
}

// resolveDeps installs a plugin's declared driver dependencies, dispatched by
// install mode. A nil manifest means the plugin declares none.
func resolveDeps(cfg *config.EMOSConfig, manifest *Manifest, srcRoot string, out io.Writer) error {
	var deps Deps
	if manifest != nil {
		deps = manifest.Deps
	}
	switch cfg.Mode {
	case config.ModePixi:
		return resolveDepsPixi(cfg, deps, out)
	case config.ModeNative:
		return resolveDepsNative(cfg, srcRoot, out)
	case config.ModeOSSContainer:
		if depsEmpty(deps) {
			return nil
		}
		fmt.Fprintln(out, "Container mode can't install driver packages persistently. The "+
			"plugin is installed; add these to the EMOS image:")
		printDriverInstructions(cfg, deps, out)
		return nil
	}
	return fmt.Errorf("unsupported install mode for dependency resolution: %s", cfg.Mode)
}

// pixiPackages is the conda package list for a plugin's declared deps.
func pixiPackages(deps Deps, distro string) []string {
	var pkgs []string
	for _, dep := range deps.ROS {
		pkgs = append(pkgs, rosDistroPkg(dep, distro))
	}
	return append(pkgs, deps.System.Conda...)
}

// resolveDepsPixi `pixi add`s the plugin's declared conda packages. pixi add is
// idempotent for a dependency already in pixi.toml.
func resolveDepsPixi(cfg *config.EMOSConfig, deps Deps, out io.Writer) error {
	pkgs := pixiPackages(deps, cfg.ROSDistro)
	if len(pkgs) == 0 {
		return nil
	}
	pixiBin, err := installer.ResolvePixi()
	if err != nil {
		return err
	}
	args := installer.PixiAddArgs(filepath.Join(cfg.PixiProjectDir, "pixi.toml"), pkgs, runtime.GOARCH)
	fmt.Fprintf(out, "pixi %s\n", strings.Join(args, " "))
	return runStreaming(pixiBin, args, "", out)
}

// resolveDepsNative lets rosdep resolve everything under the workspace to apt.
// Reads package.xml directly.
func resolveDepsNative(cfg *config.EMOSConfig, srcRoot string, out io.Writer) error {
	rosdep := fmt.Sprintf("rosdep install --from-paths %s --ignore-src -r -y --rosdistro %s --skip-keys %s",
		quote(srcRoot), quote(cfg.ROSDistro), quote(strings.Join(stackPackages, " ")))
	shell := "source " + quote(rosSetup(cfg.ROSDistro)) + " && " + rosdep
	if err := runStreaming("bash", []string{"-c", shell}, "", out); err != nil {
		fmt.Fprintln(out, "rosdep could not install every dependency (see above). Once the cause is fixed, run:\n  "+rosdep)
	}
	return nil
}

// printDriverInstructions lists the manifest's declared deps as apt packages.
func printDriverInstructions(cfg *config.EMOSConfig, deps Deps, out io.Writer) {
	for _, dep := range deps.ROS {
		fmt.Fprintf(out, "  - %s  ->  %s\n", dep, rosDistroPkg(dep, cfg.ROSDistro))
	}
	for _, pkg := range deps.System.Apt {
		fmt.Fprintf(out, "  - %s\n", pkg)
	}
}

// depsEmpty reports whether a plugin declares no installable dependencies.
func depsEmpty(deps Deps) bool {
	return len(deps.ROS) == 0 && len(deps.System.Conda) == 0 && len(deps.System.Apt) == 0
}

// rosDistroPkg is the ros-<distro>-* package name for a ROS package dependency.
func rosDistroPkg(dep, distro string) string {
	return "ros-" + distro + "-" + strings.ReplaceAll(dep, "_", "-")
}
