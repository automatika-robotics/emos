package installer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/ui"
)

// ROSInstallation holds detected ROS 2 info.
type ROSInstallation struct {
	Distro string
	Path   string
}

// DetectROS scans the system for ROS 2 installations.
func DetectROS() []ROSInstallation {
	var installs []ROSInstallation
	seen := map[string]bool{}

	// Check ROS_DISTRO env var
	if distro := os.Getenv("ROS_DISTRO"); distro != "" {
		p := filepath.Join("/opt/ros", distro)
		if _, err := os.Stat(filepath.Join(p, "setup.bash")); err == nil {
			installs = append(installs, ROSInstallation{Distro: distro, Path: p})
			seen[distro] = true
		}
	}

	// Scan /opt/ros/*/setup.bash
	matches, _ := filepath.Glob("/opt/ros/*/setup.bash")
	for _, m := range matches {
		dir := filepath.Dir(m)
		distro := filepath.Base(dir)
		if !seen[distro] {
			installs = append(installs, ROSInstallation{Distro: distro, Path: dir})
			seen[distro] = true
		}
	}

	return installs
}

// InstallNative builds EMOS packages and installs them into the ROS 2 installation.
// Uses two separate colcon builds (matching the Dockerfile pattern):
//  1. Localization dependencies (angles, geographic_info, robot_localization)
//  2. EMOS packages (sugarcoat, kompass, embodied-agents)
func InstallNative(wsPath, distro string) error {
	rosPath := filepath.Join("/opt/ros", distro)
	rosSetup := filepath.Join(rosPath, "setup.bash")

	// -- Fetch sources --
	srcDir := filepath.Join(wsPath, "src")
	os.MkdirAll(srcDir, 0755)

	emosRepoURL := "https://github.com/" + config.GitHubOrg + "/" + config.GitHubRepo + ".git"
	emosPackages := []string{"sugarcoat", "kompass", "embodied-agents"}
	emosRepo := filepath.Join(srcDir, ".emos-repo")

	if err := ui.Spinner("Fetching EMOS source...", func() error {
		if _, err := os.Stat(emosRepo); err == nil {
			if err := runCmd(emosRepo, "git", "pull"); err != nil {
				return err
			}
			// Update submodules (stack packages are submodules)
			return runCmd(emosRepo, "git", "submodule", "update", "--init", "--depth", "1")
		}
		return runCmd(srcDir, "git", "clone", "--depth", "1", "--recurse-submodules", "--shallow-submodules", emosRepoURL, ".emos-repo")
	}); err != nil {
		return fmt.Errorf("failed to fetch emos source: %w", err)
	}

	// Copy stack packages from the monorepo
	for _, pkg := range emosPackages {
		dest := filepath.Join(srcDir, pkg)
		src := filepath.Join(emosRepo, "stack", pkg)
		// Check that the submodule was actually populated (has files, not just empty dir)
		if _, err := os.Stat(filepath.Join(src, "package.xml")); err != nil {
			// kompass has nested package.xml
			if _, err := os.Stat(filepath.Join(src, "kompass", "package.xml")); err != nil {
				ui.Warn(fmt.Sprintf("Package %s not found in source — skipping", pkg))
				continue
			}
		}
		os.RemoveAll(dest)
		if err := runCmd("", "cp", "-r", src, dest); err != nil {
			return fmt.Errorf("failed to copy %s: %w", pkg, err)
		}
	}

	// Fetch localization dependencies
	if err := ui.Spinner("Fetching dependencies...", func() error {
		locDeps := []struct {
			name   string
			url    string
			branch string
		}{
			{"angles", "https://github.com/ros/angles.git", "ros2"},
			{"geographic_info", "https://github.com/ros-geographic-info/geographic_info.git", geoBranch(distro)},
			{"robot_localization", "https://github.com/cra-ros-pkg/robot_localization.git", distro + "-devel"},
		}

		for _, dep := range locDeps {
			dest := filepath.Join(srcDir, dep.name)
			if _, err := os.Stat(dest); err == nil {
				continue
			}
			if err := runCmd(srcDir, "git", "clone", "--depth", "1", "-b", dep.branch, dep.url); err != nil {
				return fmt.Errorf("failed to clone %s: %w", dep.name, err)
			}
		}

		// Remove unnecessary geographic_info subdirectories (matches Dockerfile)
		for _, sub := range []string{"geographic_info", "geodesy"} {
			os.RemoveAll(filepath.Join(srcDir, "geographic_info", sub))
		}
		return nil
	}); err != nil {
		return err
	}

	// -- System dependencies --
	ui.Info("Installing system dependencies...")
	aptPkgs := []string{
		"portaudio19-dev", "jq", "python3-empy",
		"python3-ament-package", "ros-" + distro + "-rpyutils",
		"ros-" + distro + "-rmw-zenoh-cpp",
	}
	cmd := exec.Command("sudo", append([]string{"apt-get", "install", "-y"}, aptPkgs...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		ui.Warn("Some apt packages may have failed to install: " + err.Error())
	}
	if err := installGeoLib(); err != nil {
		ui.Warn("Failed to install GeographicLib: " + err.Error())
	}

	// -- Install kompass-core with GPU support --
	// This also installs python3-pip, so it must run before the pip step.
	// Download to a temp file instead of curl|bash to avoid stdin corruption.
	ui.Info("Installing kompass-core (this may take a while)...")
	installGPUCmd := exec.Command("bash", "-c",
		`tmpf=$(mktemp /tmp/install_gpu_XXXXXX.sh) && `+
			`curl -fsSL https://raw.githubusercontent.com/automatika-robotics/kompass-core/main/build_dependencies/install_gpu.sh -o "$tmpf" && `+
			`chmod +x "$tmpf" && bash "$tmpf" && rm -f "$tmpf"`)
	installGPUCmd.Stdout = os.Stdout
	installGPUCmd.Stderr = os.Stderr
	if err := installGPUCmd.Run(); err != nil {
		return fmt.Errorf("kompass-core GPU install failed: %w", err)
	}

	// -- Python dependencies (python3-pip now available from install_gpu.sh) --
	if err := ui.Spinner("Fetching Python dependencies...", func() error {
		pipPkgs := []string{
			"numpy", "opencv-python-headless", "attrs>=23.2.0",
			"jinja2", "httpx", "setproctitle",
			"msgpack", "msgpack-numpy", "platformdirs",
			"tqdm", "pyyaml", "toml", "websockets",
			"ollama", "redis[hiredis]", "pyaudio",
			"soundfile", "python-fasthtml", "monsterui",
		}
		args := append([]string{"-m", "pip", "install", "--no-cache-dir"}, pipPkgs...)
		cmd := exec.Command("python3", args...)
		cmd.Env = append(os.Environ(), "PIP_BREAK_SYSTEM_PACKAGES=1")
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s: %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	}); err != nil {
		ui.Warn("Some pip packages may have failed: " + err.Error())
	}

	// -- Ensure rosdep is initialized and up to date --
	if _, err := os.Stat("/etc/ros/rosdep/sources.list.d/20-default.list"); os.IsNotExist(err) {
		ui.Info("Initializing rosdep...")
		initCmd := exec.Command("sudo", "rosdep", "init")
		initCmd.Stdout = os.Stdout
		initCmd.Stderr = os.Stderr
		initCmd.Run()
	}
	ui.Info("Updating rosdep...")
	updateCmd := exec.Command("bash", "-c", "rosdep update")
	updateCmd.Stdout = os.Stdout
	updateCmd.Stderr = os.Stderr
	updateCmd.Run()

	// -- Stage 1: Build localization dependencies --
	// Build only the localization packages first, then install them into
	// /opt/ros/{distro} so they're available as an underlay for EMOS packages.
	ui.Info("Installing dependencies for localization packages...")
	rosdepCmd := fmt.Sprintf("source %s && sudo apt-get update && rosdep install --from-paths %s --ignore-src -y", rosSetup, srcDir)
	cmd = exec.Command("bash", "-c", rosdepCmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		ui.Warn("Some rosdep dependencies may have failed: " + err.Error())
	}

	locPkgs := "angles geographic_msgs robot_localization"
	ui.Info("Building localization packages...")
	buildCmd := fmt.Sprintf(
		"source %s && cd %s && colcon build --merge-install --packages-select %s --cmake-args -DCMAKE_BUILD_TYPE=Release",
		rosSetup, wsPath, locPkgs)
	cmd = exec.Command("bash", "-c", buildCmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = cleanEnv()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("localization build failed: %w", err)
	}

	if err := mergeIntoROS(filepath.Join(wsPath, "install"), rosPath); err != nil {
		return err
	}

	// -- Stage 2: Build EMOS packages --
	// Re-source /opt/ros/{distro} which now includes localization packages,
	// then build only the EMOS packages.
	emosPkgs := "automatika_ros_sugar automatika_embodied_agents kompass kompass_interfaces"
	ui.Info("Building EMOS packages (this may take a while)...")
	buildCmd = fmt.Sprintf(
		"unset VIRTUAL_ENV && source %s && cd %s && colcon build --merge-install --packages-select %s --cmake-args -DCMAKE_BUILD_TYPE=Release",
		rosSetup, wsPath, emosPkgs)
	cmd = exec.Command("bash", "-c", buildCmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = cleanEnv()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("EMOS build failed: %w", err)
	}

	if err := mergeIntoROS(filepath.Join(wsPath, "install"), rosPath); err != nil {
		return err
	}

	// -- Verify installation --
	ui.Info("Verifying installation...")
	if err := VerifyNativeInstall(rosSetup); err != nil {
		return err
	}

	ui.Success("EMOS packages installed into " + rosPath)
	return nil
}

// VerifyNativeInstall checks that EMOS packages are correctly installed and importable.
func VerifyNativeInstall(rosSetup string) error {
	// Check Python imports
	pyChecks := []struct {
		module  string
		display string
	}{
		{"ros_sugar", "Sugarcoat (ros_sugar)"},
		{"agents", "Embodied Agents"},
		{"kompass", "Kompass"},
		{"kompass_core", "Kompass Core"},
	}

	allOK := true
	for _, check := range pyChecks {
		importCmd := fmt.Sprintf("source %s && python3 -c 'import %s'", rosSetup, check.module)
		cmd := exec.Command("bash", "-c", importCmd)
		cmd.Env = cleanEnv()
		if out, err := cmd.CombinedOutput(); err != nil {
			ui.Error(fmt.Sprintf("  %s: FAILED (%s)", check.display, strings.TrimSpace(string(out))))
			allOK = false
		} else {
			ui.Success(fmt.Sprintf("  %s: OK", check.display))
		}
	}

	// Check ROS packages are registered
	rosChecks := []string{
		"automatika_ros_sugar",
		"automatika_embodied_agents",
		"kompass",
		"kompass_interfaces",
	}

	listCmd := fmt.Sprintf("source %s && ros2 pkg list", rosSetup)
	out, err := exec.Command("bash", "-c", listCmd).CombinedOutput()
	if err == nil {
		pkgList := string(out)
		for _, pkg := range rosChecks {
			if strings.Contains(pkgList, pkg) {
				ui.Success(fmt.Sprintf("  ROS package %s: OK", pkg))
			} else {
				ui.Error(fmt.Sprintf("  ROS package %s: NOT FOUND", pkg))
				allOK = false
			}
		}
	} else {
		ui.Warn("Could not list ROS packages (ros2 command not available in this shell)")
	}

	if !allOK {
		return fmt.Errorf("some packages failed verification — check the errors above")
	}
	return nil
}

// UpdateNative pulls latest sources, rebuilds, and re-installs into the ROS 2 installation.
func UpdateNative(wsPath, distro string) error {
	rosPath := filepath.Join("/opt/ros", distro)
	rosSetup := filepath.Join(rosPath, "setup.bash")
	srcDir := filepath.Join(wsPath, "src")

	// Pull latest emos repo and re-copy stack packages
	emosRepo := filepath.Join(srcDir, ".emos-repo")
	emosPackages := []string{"sugarcoat", "kompass", "embodied-agents"}

	if _, err := os.Stat(filepath.Join(emosRepo, ".git")); err == nil {
		if err := ui.Spinner("Fetching EMOS source...", func() error {
			if err := runCmd(emosRepo, "git", "pull"); err != nil {
				return err
			}
			return runCmd(emosRepo, "git", "submodule", "update", "--init", "--depth", "1")
		}); err != nil {
			ui.Warn("Failed to update emos repo: " + err.Error())
		}
		for _, pkg := range emosPackages {
			src := filepath.Join(emosRepo, "stack", pkg)
			dest := filepath.Join(srcDir, pkg)
			os.RemoveAll(dest)
			if err := runCmd("", "cp", "-r", src, dest); err != nil {
				ui.Warn(fmt.Sprintf("Failed to copy %s: %v", pkg, err))
			}
		}
	}

	// Update localization dependency repos
	if err := ui.Spinner("Fetching dependencies...", func() error {
		for _, name := range []string{"angles", "geographic_info", "robot_localization"} {
			repoPath := filepath.Join(srcDir, name)
			if _, err := os.Stat(filepath.Join(repoPath, ".git")); err != nil {
				continue
			}
			if err := runCmd(repoPath, "git", "pull"); err != nil {
				return fmt.Errorf("failed to update %s: %w", name, err)
			}
		}
		return nil
	}); err != nil {
		ui.Warn(err.Error())
	}

	// Update kompass-core via GPU install script (download to file, not curl|bash)
	ui.Info("Updating kompass-core...")
	installGPUCmd := exec.Command("bash", "-c",
		`tmpf=$(mktemp /tmp/install_gpu_XXXXXX.sh) && `+
			`curl -fsSL https://raw.githubusercontent.com/automatika-robotics/kompass-core/main/build_dependencies/install_gpu.sh -o "$tmpf" && `+
			`chmod +x "$tmpf" && bash "$tmpf" && rm -f "$tmpf"`)
	installGPUCmd.Stdout = os.Stdout
	installGPUCmd.Stderr = os.Stderr
	if err := installGPUCmd.Run(); err != nil {
		ui.Warn("kompass-core update failed: " + err.Error())
	}

	// Ensure rosdep is initialized and up to date
	if _, err := os.Stat("/etc/ros/rosdep/sources.list.d/20-default.list"); os.IsNotExist(err) {
		ui.Info("Initializing rosdep...")
		initCmd := exec.Command("sudo", "rosdep", "init")
		initCmd.Stdout = os.Stdout
		initCmd.Stderr = os.Stderr
		initCmd.Run()
	}
	updateCmd := exec.Command("bash", "-c", "rosdep update")
	updateCmd.Stdout = os.Stdout
	updateCmd.Stderr = os.Stderr
	updateCmd.Run()

	// -- Stage 1: Rebuild localization packages --
	rosdepCmd := fmt.Sprintf("source %s && sudo apt-get update && rosdep install --from-paths %s --ignore-src -y", rosSetup, srcDir)
	cmd := exec.Command("bash", "-c", rosdepCmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		ui.Warn("Some rosdep dependencies may have failed: " + err.Error())
	}

	locPkgs := "angles geographic_msgs robot_localization"
	ui.Info("Rebuilding localization packages...")
	buildCmd := fmt.Sprintf(
		"source %s && cd %s && colcon build --merge-install --packages-select %s --cmake-args -DCMAKE_BUILD_TYPE=Release",
		rosSetup, wsPath, locPkgs)
	cmd = exec.Command("bash", "-c", buildCmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = cleanEnv()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("localization build failed: %w", err)
	}
	if err := mergeIntoROS(filepath.Join(wsPath, "install"), rosPath); err != nil {
		return err
	}

	// -- Stage 2: Rebuild EMOS packages --
	emosPkgs := "automatika_ros_sugar automatika_embodied_agents kompass kompass_interfaces"
	ui.Info("Rebuilding EMOS packages...")
	buildCmd = fmt.Sprintf(
		"unset VIRTUAL_ENV && source %s && cd %s && colcon build --merge-install --packages-select %s --cmake-args -DCMAKE_BUILD_TYPE=Release",
		rosSetup, wsPath, emosPkgs)
	cmd = exec.Command("bash", "-c", buildCmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = cleanEnv()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("EMOS build failed: %w", err)
	}
	if err := mergeIntoROS(filepath.Join(wsPath, "install"), rosPath); err != nil {
		return err
	}

	ui.Success("EMOS packages updated in " + rosPath)
	return nil
}

// mergeIntoROS copies the colcon merge-install output into /opt/ros/{distro}/.
// It preserves the original ROS setup files (setup.bash, setup.sh, etc.) because
// colcon's generated versions have different PYTHONPATH behavior that can break
// imports of packages installed in /opt/ros/{distro}/lib/python3.X/site-packages/.
func mergeIntoROS(installDir, rosPath string) error {
	ui.Info("Installing packages into " + rosPath + " (requires sudo)...")

	// Remove colcon-generated setup files from the install dir before copying
	// so they don't overwrite the originals in /opt/ros/{distro}/.
	setupFiles := []string{
		"setup.bash", "setup.sh", "setup.zsh",
		"local_setup.bash", "local_setup.sh", "local_setup.zsh",
		"setup.ps1", "local_setup.ps1",
		"_local_setup_util.py",
		".colcon_install_layout",
		"COLCON_IGNORE",
	}
	for _, f := range setupFiles {
		os.Remove(filepath.Join(installDir, f))
	}

	cmd := exec.Command("sudo", "cp", "-r", "--no-preserve=ownership", installDir+"/.", rosPath+"/")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install packages into %s: %w", rosPath, err)
	}
	return nil
}

// geoBranch returns the git branch for geographic_info.
// Only jazzy has a dedicated branch; all others use "ros2".
func geoBranch(distro string) string {
	if distro == "jazzy" {
		return "jazzy"
	}
	return "ros2"
}

// installGeoLib installs the correct GeographicLib apt package.
// Ubuntu 24.04+ renamed it to libgeographiclib-dev; older uses libgeographic-dev.
// Matches the Dockerfile's fallback pattern.
func installGeoLib() error {
	cmd := exec.Command("sudo", "apt-get", "install", "-y", "libgeographiclib-dev")
	if err := cmd.Run(); err != nil {
		cmd = exec.Command("sudo", "apt-get", "install", "-y", "libgeographic-dev")
		return cmd.Run()
	}
	return nil
}

// cleanEnv returns the current environment with VIRTUAL_ENV removed and PATH
// cleaned of any venv bin directories, preventing stale venvs from interfering
// with colcon builds.
func cleanEnv() []string {
	var env []string
	venvPath := os.Getenv("VIRTUAL_ENV")
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "VIRTUAL_ENV=") {
			continue
		}
		if strings.HasPrefix(e, "PATH=") && venvPath != "" {
			// Remove the venv's bin dir from PATH
			parts := strings.Split(e[5:], ":")
			var cleaned []string
			for _, p := range parts {
				if !strings.HasPrefix(p, venvPath) {
					cleaned = append(cleaned, p)
				}
			}
			env = append(env, "PATH="+strings.Join(cleaned, ":"))
			continue
		}
		env = append(env, e)
	}
	return env
}

func runCmd(dir string, name string, args ...string) error {
	// Flatten any string slices passed as args
	var flatArgs []string
	flatArgs = append(flatArgs, args...)

	cmd := exec.Command(name, flatArgs...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
