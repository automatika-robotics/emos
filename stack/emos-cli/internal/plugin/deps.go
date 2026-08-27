package plugin

// Dependency resolution for a plugin's declared sensor-driver packages.
//
// A plugin's package.xml (and those of any driver it vendors as a git
// submodule) declares the ROS packages it needs. To find the ones that still
// require installation we ask the environment what it already provides
// (`ros2 pkg list`) rather than maintaining a hardcoded allow-list, then
// resolve the remainder per install mode:
//
//   - a dep already built from source in the workspace, or already installed in
//     the environment, needs nothing;
//   - packaged drivers are installed from the env (pixi add on pixi, rosdep on
//     native);
//   - container mode cannot persist packaged drivers past the ephemeral build,
//     so it prints exact install instructions instead.

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/container"
	"github.com/automatika-robotics/emos-cli/internal/installer"
)

// pkgManifest is the slice of a ROS package.xml we care about.
type pkgManifest struct {
	Name            string   `xml:"name"`
	ExecDepend      []string `xml:"exec_depend"`
	BuildDepend     []string `xml:"build_depend"`
	BuildtoolDepend []string `xml:"buildtool_depend"`
	Depend          []string `xml:"depend"`
}

// stackPackages are the EMOS packages built from source into the environment.
var stackPackages = []string{
	"automatika_ros_sugar", "kompass", "embodied_agents", "kompass_interfaces",
}

// collectDeps walks every package.xml under srcRoot and returns the union of
// declared dependencies and the set of package names the tree itself provides.
func collectDeps(srcRoot string) (declared []string, provided map[string]bool, err error) {
	provided = map[string]bool{}
	declaredSet := map[string]bool{}

	walkErr := filepath.WalkDir(srcRoot, func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() || d.Name() != "package.xml" {
			return nil
		}
		data, rErr := os.ReadFile(path)
		if rErr != nil {
			return rErr
		}
		var m pkgManifest
		if xml.Unmarshal(data, &m) != nil {
			return nil // skip a malformed package.xml rather than fail the whole scan
		}
		if name := strings.TrimSpace(m.Name); name != "" {
			provided[name] = true
		}
		for _, group := range [][]string{m.ExecDepend, m.BuildDepend, m.BuildtoolDepend, m.Depend} {
			for _, dep := range group {
				if dep = strings.TrimSpace(dep); dep != "" {
					declaredSet[dep] = true
				}
			}
		}
		return nil
	})
	if walkErr != nil {
		return nil, nil, walkErr
	}
	for dep := range declaredSet {
		declared = append(declared, dep)
	}
	sort.Strings(declared)
	return declared, provided, nil
}

// externalDeps returns the packages that still need installing: declared, minus
// what the environment already provides.
func externalDeps(declared []string, provided map[string]bool) []string {
	var ext []string
	for _, dep := range declared {
		switch {
		case provided[dep]: // built from source in the workspace, or already installed
		case isPipDep(dep): // python3-* / python-* -> pip/conda, not a ROS driver
		default:
			ext = append(ext, dep)
		}
	}
	return ext
}

func isPipDep(dep string) bool {
	return strings.HasPrefix(dep, "python3-") || strings.HasPrefix(dep, "python-")
}

// resolveDeps installs a plugin's external driver dependencies, dispatched by
// install mode. Called from Install after the clone and before the build.
func resolveDeps(cfg *config.EMOSConfig, srcRoot string, out io.Writer) error {
	declared, provided, err := collectDeps(srcRoot)
	if err != nil {
		return fmt.Errorf("scan plugin dependencies: %w", err)
	}
	installed, err := installedPackages(cfg)
	if err != nil {
		// Non-fatal: fall back to no env filter.
		fmt.Fprintf(out, "Warning: could not list installed packages (%v); "+
			"treating every declared dependency as a candidate.\n", err)
	}
	for name := range installed {
		provided[name] = true
	}
	ext := externalDeps(declared, provided)
	if len(ext) == 0 {
		fmt.Fprintln(out, "All plugin dependencies are already provided by the environment.")
		return nil
	}
	fmt.Fprintf(out, "External driver dependencies: %s\n", strings.Join(ext, ", "))

	switch cfg.Mode {
	case config.ModePixi:
		return resolveDepsPixi(cfg, ext, out)
	case config.ModeNative:
		return resolveDepsNative(cfg, srcRoot, ext, out)
	case config.ModeOSSContainer, config.ModeLicensed:
		fmt.Fprintln(out, "Container mode can't install driver packages persistently. The "+
			"plugin is installed; add these to the EMOS image, or install in pixi/native mode:")
		printDriverInstructions(cfg, ext, out)
		return nil
	}
	return fmt.Errorf("unsupported install mode for dependency resolution: %s", cfg.Mode)
}

// installedPackages returns the ROS packages already available in the plugin's
// base environment (standard ROS + the built EMOS stack) via `ros2 pkg list`.
func installedPackages(cfg *config.EMOSConfig) (map[string]bool, error) {
	const cmd = "ros2 pkg list"
	var (
		outStr string
		err    error
	)
	switch cfg.Mode {
	case config.ModeNative:
		shell := fmt.Sprintf("source /opt/ros/%s/setup.bash && %s", cfg.ROSDistro, cmd)
		var b []byte
		b, err = captureStdout("bash", []string{"-c", shell}, "")
		outStr = string(b)

	case config.ModePixi:
		pixiBin, e := installer.ResolvePixi()
		if e != nil {
			return nil, e
		}
		shell := fmt.Sprintf("source %s && %s",
			filepath.Join(cfg.PixiProjectDir, "install", "setup.sh"), cmd)
		var b []byte
		b, err = captureStdout(pixiBin, pixiRunArgs(cfg, shell), "")
		outStr = string(b)

	case config.ModeOSSContainer, config.ModeLicensed:
		shell := fmt.Sprintf("source /opt/ros/%s/setup.bash && %s", cfg.ROSDistro, cmd)
		outStr, err = container.RunEphemeralCapture(cfg.ImageTag, shell)

	default:
		return nil, fmt.Errorf("unsupported install mode: %s", cfg.Mode)
	}
	if err != nil {
		return nil, err
	}
	installed := map[string]bool{}
	for _, line := range strings.Split(outStr, "\n") {
		if p := strings.TrimSpace(line); p != "" {
			installed[p] = true
		}
	}
	return installed, nil
}

// resolveDepsPixi maps each external ROS dep to its robostack conda package and
// `pixi add`s the ones not already declared in the pixi project's pixi.toml.
func resolveDepsPixi(cfg *config.EMOSConfig, ext []string, out io.Writer) error {
	pixiBin, err := installer.ResolvePixi()
	if err != nil {
		return err
	}
	manifest := filepath.Join(cfg.PixiProjectDir, "pixi.toml")
	existing, _ := os.ReadFile(manifest) // best-effort; used only to skip already-present deps

	var toAdd []string
	for _, dep := range ext {
		pkg := rosDistroPkg(dep, cfg.ROSDistro)
		if len(existing) > 0 && bytes.Contains(existing, []byte(pkg)) {
			continue // already in pixi.toml
		}
		toAdd = append(toAdd, pkg)
	}
	if len(toAdd) == 0 {
		fmt.Fprintln(out, "  (all driver deps already present in pixi.toml)")
		return nil
	}
	fmt.Fprintf(out, "pixi add %s\n", strings.Join(toAdd, " "))
	args := append([]string{"add", "--manifest-path", manifest}, toAdd...)
	return runStreaming(pixiBin, args, "", out)
}

// resolveDepsNative lets rosdep resolve the plugin's declared deps to apt. It
// skips the source-built EMOS stack packages. rosdep does its
// own installed-vs-missing filtering so it also covers a vendored driver's
// build prerequisites.
func resolveDepsNative(cfg *config.EMOSConfig, srcRoot string, ext []string, out io.Writer) error {
	shell := fmt.Sprintf(
		"source /opt/ros/%s/setup.bash && rosdep install --from-paths %s --ignore-src -y "+
			"--rosdistro %s --skip-keys \"%s\"",
		cfg.ROSDistro, srcRoot, cfg.ROSDistro, strings.Join(stackPackages, " "))
	if err := runStreaming("bash", []string{"-c", shell}, "", out); err != nil {
		fmt.Fprintln(out, "rosdep could not resolve every dependency (a driver may not be "+
			"apt-packaged). Install these manually, or use pixi mode:")
		printDriverInstructions(cfg, ext, out)
	}
	return nil
}

// printDriverInstructions lists each external dep with the ros-<distro>-* package
// that provides it
func printDriverInstructions(cfg *config.EMOSConfig, ext []string, out io.Writer) {
	for _, dep := range ext {
		fmt.Fprintf(out, "  - %s  ->  %s\n", dep, rosDistroPkg(dep, cfg.ROSDistro))
	}
}

// rosDistroPkg is the ros-<distro>-* package name for a ROS package dependency
// -- the same name in robostack conda and in apt
func rosDistroPkg(dep, distro string) string {
	return "ros-" + distro + "-" + strings.ReplaceAll(dep, "_", "-")
}
