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

// InstallNativeWorkspace sets up the EMOS ROS 2 workspace at wsPath.
func InstallNativeWorkspace(wsPath, distro string) error {
	srcDir := filepath.Join(wsPath, "src")
	os.MkdirAll(srcDir, 0755)

	// Fetch EMOS source
	emosRepoURL := "https://github.com/" + config.GitHubOrg + "/" + config.GitHubRepo + ".git"
	emosPackages := []string{"sugarcoat", "kompass", "embodied-agents"}
	emosRepo := filepath.Join(srcDir, ".emos-repo")

	if err := ui.Spinner("Fetching EMOS source...", func() error {
		if _, err := os.Stat(emosRepo); err == nil {
			return runCmd(emosRepo, "git", "pull")
		}
		return runCmd(srcDir, "git", "clone", "--depth", "1", emosRepoURL, ".emos-repo")
	}); err != nil {
		return fmt.Errorf("failed to fetch emos source: %w", err)
	}

	// Copy stack packages from the monorepo into the workspace src
	for _, pkg := range emosPackages {
		dest := filepath.Join(srcDir, pkg)
		src := filepath.Join(emosRepo, "stack", pkg)
		if _, err := os.Stat(src); err != nil {
			continue
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

	// System dependencies (matches Dockerfile)
	ui.Info("Installing system dependencies...")
	aptPkgs := []string{
		"portaudio19-dev", "jq", "python3-empy",
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

	if err := ui.Spinner("Fetching Python dependencies...", func() error {
		pipPkgs := []string{
			"numpy", "opencv-python-headless", "attrs>=23.2.0",
			"jinja2", "httpx", "setproctitle",
			"msgpack", "msgpack-numpy", "platformdirs",
			"tqdm", "pyyaml", "toml", "websockets",
			"ollama", "redis[hiredis]", "pyaudio",
			"soundfile", "python-fasthtml", "monsterui",
		}
		args := append([]string{"install", "--no-cache-dir"}, pipPkgs...)
		cmd := exec.Command("pip3", args...)
		cmd.Env = append(os.Environ(), "PIP_BREAK_SYSTEM_PACKAGES=1")
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s: %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	}); err != nil {
		ui.Warn("Some pip packages may have failed: " + err.Error())
	}

	// Install kompass-core with GPU support (also works on non-GPU machines)
	ui.Info("Installing kompass-core (this may take a while)...")
	installGPUCmd := exec.Command("bash", "-c",
		"curl -fsSL https://raw.githubusercontent.com/automatika-robotics/kompass-core/main/build_dependencies/install_gpu.sh | bash")
	installGPUCmd.Stdout = os.Stdout
	installGPUCmd.Stderr = os.Stderr
	if err := installGPUCmd.Run(); err != nil {
		return fmt.Errorf("kompass-core GPU install failed: %w", err)
	}

	// Rosdep
	ui.Info("Running rosdep install...")
	rosSetup := filepath.Join("/opt/ros", distro, "setup.bash")
	rosdepCmd := fmt.Sprintf("source %s && rosdep install --from-paths %s --ignore-src -r -y", rosSetup, srcDir)
	cmd = exec.Command("bash", "-c", rosdepCmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run() // best-effort

	// Colcon build — sanitize environment to avoid venv contamination
	ui.Info("Building workspace with colcon (this may take a while)...")
	buildCmd := fmt.Sprintf("unset VIRTUAL_ENV && source %s && cd %s && colcon build --symlink-install", rosSetup, wsPath)
	cmd = exec.Command("bash", "-c", buildCmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = cleanEnv()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("colcon build failed: %w", err)
	}

	ui.Success("Workspace built successfully.")
	return nil
}

// UpdateNativeWorkspace pulls latest sources and rebuilds.
func UpdateNativeWorkspace(wsPath, distro string) error {
	srcDir := filepath.Join(wsPath, "src")

	// Pull latest emos repo and re-copy stack packages
	emosRepo := filepath.Join(srcDir, ".emos-repo")
	emosPackages := []string{"sugarcoat", "kompass", "embodied-agents"}

	if _, err := os.Stat(filepath.Join(emosRepo, ".git")); err == nil {
		if err := ui.Spinner("Fetching EMOS source...", func() error {
			return runCmd(emosRepo, "git", "pull")
		}); err != nil {
			ui.Warn("Failed to update emos repo: " + err.Error())
		}
		for _, pkg := range emosPackages {
			src := filepath.Join(emosRepo, "stack", pkg)
			dest := filepath.Join(srcDir, pkg)
			if _, err := os.Stat(src); err != nil {
				continue
			}
			os.RemoveAll(dest)
			runCmd("", "cp", "-r", src, dest)
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

	// Update kompass-core via GPU install script
	ui.Info("Updating kompass-core...")
	installGPUCmd := exec.Command("bash", "-c",
		"curl -fsSL https://raw.githubusercontent.com/automatika-robotics/kompass-core/main/build_dependencies/install_gpu.sh | bash")
	installGPUCmd.Stdout = os.Stdout
	installGPUCmd.Stderr = os.Stderr
	if err := installGPUCmd.Run(); err != nil {
		ui.Warn("kompass-core update failed: " + err.Error())
	}

	// Rosdep install before rebuild
	rosSetup := filepath.Join("/opt/ros", distro, "setup.bash")
	rosdepCmd := fmt.Sprintf("source %s && rosdep install --from-paths %s --ignore-src -r -y", rosSetup, srcDir)
	cmd := exec.Command("bash", "-c", rosdepCmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run() // best-effort

	// Rebuild
	buildCmd := fmt.Sprintf("unset VIRTUAL_ENV && source %s && cd %s && colcon build --symlink-install", rosSetup, wsPath)
	ui.Info("Rebuilding workspace...")
	cmd = exec.Command("bash", "-c", buildCmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = cleanEnv()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("colcon build failed: %w", err)
	}

	ui.Success("Native workspace updated.")
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
