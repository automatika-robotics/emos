// Package plugin installs, updates, and introspects a robot plugin; a ROS
// package that adapts a specific robot to the EMOS stack. A robot runs exactly
// one plugin at a time.
package plugin

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/automatika-robotics/emos-cli/internal/api"
	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/container"
	"github.com/automatika-robotics/emos-cli/internal/installer"
)

// colconBuild is the build invocation, run from the workspace root so the
// overlay lands at ~/emos/workspace/install.
const colconBuild = "colcon build --merge-install --symlink-install " +
	"--cmake-args -DCMAKE_BUILD_TYPE=Release"

// Resolve looks up a plugin slug in the support-portal registry.
func Resolve(slug string) (api.Plugin, error) {
	plugins, err := api.ListPlugins()
	if err != nil {
		return api.Plugin{}, err
	}
	for _, p := range plugins {
		if p.Filename == slug {
			return p, nil
		}
	}
	return api.Plugin{}, fmt.Errorf("plugin %q not found in the catalog", slug)
}

// Install fetches, builds, and activates a robot plugin. Any previously
// installed plugin is removed first.
func Install(cfg *config.EMOSConfig, entry api.Plugin, out io.Writer) error {
	if cfg == nil || !cfg.IsInstalled() {
		return fmt.Errorf("EMOS is not installed; run 'emos install' first")
	}
	if entry.Filename == "" {
		return fmt.Errorf("plugin entry has no slug")
	}
	if entry.EntryPoint == "" {
		return fmt.Errorf("plugin %q has no entry_point in the catalog", entry.Filename)
	}

	// One plugin at a time: clear any existing workspace wholesale.
	if err := os.RemoveAll(config.WorkspaceDir); err != nil {
		return fmt.Errorf("clear plugin workspace: %w", err)
	}
	if err := os.MkdirAll(config.PluginSrcDir(), 0755); err != nil {
		return fmt.Errorf("create plugin source dir: %w", err)
	}

	srcDir := filepath.Join(config.PluginSrcDir(), entry.Filename)
	fmt.Fprintf(out, "Cloning %s\n", entry.Repo)
	if err := gitClone(entry.Repo, entry.Ref, srcDir, out); err != nil {
		return fmt.Errorf("git clone: %w", err)
	}

	// Install the plugin's declared sensor-driver dependencies.
	if err := resolveDeps(cfg, config.PluginSrcDir(), out); err != nil {
		return fmt.Errorf("resolve dependencies: %w", err)
	}

	fmt.Fprintf(out, "Building plugin (%s mode)\n", cfg.Mode)
	if err := build(cfg, out); err != nil {
		return fmt.Errorf("colcon build: %w", err)
	}

	describe, err := inspect(cfg, entry.EntryPoint)
	if err != nil {
		return fmt.Errorf("inspect plugin: %w", err)
	}
	if err := os.WriteFile(config.PluginDescribeFile, describe, 0o644); err != nil {
		return fmt.Errorf("cache plugin describe: %w", err)
	}

	cfg.Plugin = &config.PluginInfo{
		Slug:        entry.Filename,
		EntryPoint:  entry.EntryPoint,
		Repo:        entry.Repo,
		Ref:         entry.Ref,
		InstalledAt: time.Now().UTC(),
	}
	if entry.Image != "" {
		cfg.Plugin.ImageURL = config.PluginsEndpoint + "/images/" + entry.Image
	}
	if err := config.SaveConfig(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	return nil
}

// Update pulls a default branch tracking plugin to its latest commit and
// rebuilds it. A ref-pinned plugin is left untouched.
func Update(cfg *config.EMOSConfig, out io.Writer) error {
	if cfg == nil || cfg.Plugin == nil {
		return nil
	}
	p := cfg.Plugin
	if p.Ref != "" {
		fmt.Fprintf(out, "Plugin %s is pinned to %s; nothing to update.\n", p.Slug, p.Ref)
		return nil
	}
	srcDir := filepath.Join(config.PluginSrcDir(), p.Slug)
	if _, err := os.Stat(srcDir); err != nil {
		return fmt.Errorf("plugin source missing at %s; reinstall with 'emos plugin install %s'", srcDir, p.Slug)
	}

	fmt.Fprintf(out, "Updating plugin %s\n", p.Slug)
	if err := runStreaming("git", []string{"pull", "--ff-only"}, srcDir, out); err != nil {
		return fmt.Errorf("git pull: %w", err)
	}
	if err := runStreaming("git", []string{"submodule", "update", "--init", "--depth", "1"}, srcDir, out); err != nil {
		return fmt.Errorf("submodule update: %w", err)
	}
	if err := build(cfg, out); err != nil {
		return fmt.Errorf("colcon build: %w", err)
	}
	describe, err := inspect(cfg, p.EntryPoint)
	if err != nil {
		return fmt.Errorf("inspect plugin: %w", err)
	}
	if err := os.WriteFile(config.PluginDescribeFile, describe, 0o644); err != nil {
		return err
	}
	p.InstalledAt = time.Now().UTC()
	return config.SaveConfig(cfg)
}

// Remove deletes the active plugin's workspace and clears its config record.
func Remove(cfg *config.EMOSConfig) error {
	if err := os.RemoveAll(config.WorkspaceDir); err != nil {
		return fmt.Errorf("remove plugin workspace: %w", err)
	}
	_ = os.Remove(config.PluginDescribeFile)
	if cfg != nil {
		cfg.Plugin = nil
		return config.SaveConfig(cfg)
	}
	return nil
}

// CachedDescribe returns the active plugin's cached describe() JSON.
func CachedDescribe() ([]byte, bool) {
	data, err := os.ReadFile(config.PluginDescribeFile)
	if err != nil || len(data) == 0 {
		return nil, false
	}
	return data, true
}

// gitClone clones repo at ref (empty = default branch) into dest, then pulls
// any submodules.
func gitClone(repo, ref, dest string, out io.Writer) error {
	args := []string{"clone", "--depth", "1"}
	if ref != "" {
		args = append(args, "--branch", ref)
	}
	args = append(args, repo, dest)
	if err := runStreaming("git", args, "", out); err != nil {
		return err
	}
	return runStreaming("git", []string{"submodule", "update", "--init", "--depth", "1"}, dest, out)
}

// build compiles the plugin into the overlay, dispatching on install mode.
func build(cfg *config.EMOSConfig, out io.Writer) error {
	switch cfg.Mode {
	case config.ModeNative:
		shell := fmt.Sprintf("source /opt/ros/%s/setup.bash && cd %s && %s",
			cfg.ROSDistro, config.WorkspaceDir, colconBuild)
		return runStreaming("bash", []string{"-c", shell}, "", out)

	case config.ModePixi:
		pixiBin, err := installer.ResolvePixi()
		if err != nil {
			return err
		}
		shell := fmt.Sprintf("source %s && cd %s && %s",
			filepath.Join(cfg.PixiProjectDir, "install", "setup.sh"),
			config.WorkspaceDir, colconBuild)
		return runStreaming(pixiBin, pixiRunArgs(cfg, shell), "", out)

	case config.ModeOSSContainer, config.ModeLicensed:
		// The image entrypoint sources the ROS stack; the overlay lands in the
		// mounted /emos/workspace, which is ~/emos/workspace on the host.
		shell := "cd /emos/workspace && " + colconBuild
		return container.RunEphemeral(cfg.ImageTag, shell, out)
	}
	return fmt.Errorf("unsupported install mode for plugins: %s", cfg.Mode)
}

// inspect runs `ros_sugar.robot inspect <entryPoint>` in the plugin's runtime
// environment and returns the validated describe() JSON.
func inspect(cfg *config.EMOSConfig, entryPoint string) ([]byte, error) {
	overlayBash := filepath.Join(config.PluginOverlayDir(), "setup.bash")
	overlaySh := filepath.Join(config.PluginOverlayDir(), "setup.sh")
	py := "python3 -m ros_sugar.robot inspect " + entryPoint

	var (
		describe []byte
		err      error
	)
	switch cfg.Mode {
	case config.ModeNative:
		shell := fmt.Sprintf("source /opt/ros/%s/setup.bash && source %s && %s",
			cfg.ROSDistro, overlayBash, py)
		describe, err = captureStdout("bash", []string{"-c", shell}, "")

	case config.ModePixi:
		pixiBin, e := installer.ResolvePixi()
		if e != nil {
			return nil, e
		}
		shell := fmt.Sprintf("source %s && source %s && %s",
			filepath.Join(cfg.PixiProjectDir, "install", "setup.sh"), overlaySh, py)
		describe, err = captureStdout(pixiBin, pixiRunArgs(cfg, shell), "")

	case config.ModeOSSContainer, config.ModeLicensed:
		shell := "source /emos/workspace/install/setup.bash && " + py
		var s string
		s, err = container.RunEphemeralCapture(cfg.ImageTag, shell)
		describe = []byte(strings.TrimSpace(s))

	default:
		return nil, fmt.Errorf("unsupported install mode for plugins: %s", cfg.Mode)
	}
	if err != nil {
		return nil, err
	}
	if !json.Valid(describe) {
		return nil, fmt.Errorf("plugin inspect did not return valid JSON (check the entry_point %q)", entryPoint)
	}
	return describe, nil
}

// pixiRunArgs wraps a shell snippet in `pixi run --manifest-path <toml> bash -c`.
func pixiRunArgs(cfg *config.EMOSConfig, shell string) []string {
	return []string{
		"run", "--manifest-path",
		filepath.Join(cfg.PixiProjectDir, "pixi.toml"),
		"bash", "-c", shell,
	}
}

// runStreaming runs a command, streaming combined output to out.
func runStreaming(name string, args []string, dir string, out io.Writer) error {
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Stdout = out
	cmd.Stderr = out
	return cmd.Run()
}

// captureStdout runs a command and returns trimmed stdout; stderr is folded
// into the error.
func captureStdout(name string, args []string, dir string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s: %s", err, strings.TrimSpace(stderr.String()))
	}
	return []byte(strings.TrimSpace(stdout.String())), nil
}
