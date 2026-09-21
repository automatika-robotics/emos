package plugin

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/automatika-robotics/emos-cli/internal/api"
	"github.com/automatika-robotics/emos-cli/internal/config"
)

// fakePixi stands in for pixi: colcon builds write an overlay marker (or fail
// when FAKE_BUILD_FAIL is set), inspect prints a describe() with FAKE_ROLE, and
// pixi add records its arguments in FAKE_PIXI_ADDS.
const fakePixi = `#!/bin/sh
case "$*" in
*"colcon build"*)
	[ -n "$FAKE_BUILD_FAIL" ] && { echo "build failed"; exit 1; }
	mkdir -p "$FAKE_WS/install" && echo new > "$FAKE_WS/install/marker"
	exit 0 ;;
*"ros_sugar.robot inspect"*)
	echo "{\"role\":\"${FAKE_ROLE:-robot}\"}"
	exit 0 ;;
"add "*)
	echo "$*" >> "$FAKE_PIXI_ADDS"
	exit 0 ;;
esac
exit 0
`

// useTempInstall isolates config and workspace in a temp dir, with a pixi-mode
// install whose builds run through fakePixi. The workspace starts with a
// working robot plugin, old_robot, and its overlay.
func useTempInstall(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	origDir, origFile, origLicense, origWs := config.ConfigDir, config.ConfigFile, config.LicenseFile, config.WorkspaceDir
	config.ConfigDir = filepath.Join(root, "config")
	config.ConfigFile = filepath.Join(config.ConfigDir, "config.json")
	config.LicenseFile = filepath.Join(config.ConfigDir, "license.json")
	config.WorkspaceDir = filepath.Join(root, "emos", "workspace")
	t.Cleanup(func() {
		config.ConfigDir, config.ConfigFile, config.LicenseFile, config.WorkspaceDir = origDir, origFile, origLicense, origWs
	})

	bin := filepath.Join(root, "bin")
	writeFile(t, filepath.Join(bin, "pixi"), fakePixi)
	if err := os.Chmod(filepath.Join(bin, "pixi"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_WS", config.WorkspaceDir)
	t.Setenv("FAKE_PIXI_ADDS", filepath.Join(root, "pixi-adds.log"))

	writeFile(t, filepath.Join(config.PluginSrcDir(), "old_robot", "package.xml"), "old")
	writeFile(t, filepath.Join(config.PluginOverlayDir(), "marker"), "old\n")
	err := config.SaveConfig(&config.EMOSConfig{
		Mode:           config.ModePixi,
		PixiProjectDir: filepath.Join(root, "pixi"),
		Plugin:         &config.PluginInfo{Slug: "old_robot", EntryPoint: "old:Old", Role: config.RoleRobot},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// gitRepo makes a local repository with one commit, cloneable by URL.
func gitRepo(t *testing.T, name string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	writeFile(t, filepath.Join(dir, "package.xml"), name)
	for _, args := range [][]string{
		{"init", "-q"},
		{"add", "."},
		{"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return "file://" + dir
}

func robotEntry(t *testing.T, slug string) api.Plugin {
	return api.Plugin{Filename: slug, EntryPoint: slug + ":Robot", Role: config.RoleRobot, Repo: gitRepo(t, slug)}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func overlayMarker(t *testing.T) string {
	t.Helper()
	data, _ := os.ReadFile(filepath.Join(config.PluginOverlayDir(), "marker"))
	return string(data)
}

// assertOldRobotIntact checks the working setup useTempInstall created is
// exactly as it was.
func assertOldRobotIntact(t *testing.T) {
	t.Helper()
	if !exists(filepath.Join(config.PluginSrcDir(), "old_robot", "package.xml")) {
		t.Error("the installed robot's source is gone")
	}
	if got := overlayMarker(t); got != "old\n" {
		t.Errorf("overlay marker = %q; the previous overlay was not kept", got)
	}
	if cfg := config.LoadConfig(); cfg.Plugin == nil || cfg.Plugin.Slug != "old_robot" {
		t.Errorf("config robot = %+v, want old_robot", cfg.Plugin)
	}
	if exists(txnDir()) {
		t.Error("the staging dir was left behind")
	}
}

func TestInstallFailedBuildKeepsTheWorkingSetup(t *testing.T) {
	useTempInstall(t)
	t.Setenv("FAKE_BUILD_FAIL", "1")
	if err := Install(config.LoadConfig(), robotEntry(t, "new_robot"), io.Discard); err == nil {
		t.Fatal("a failed build must fail the install")
	}
	assertOldRobotIntact(t)
	if exists(filepath.Join(config.PluginSrcDir(), "new_robot")) {
		t.Error("the new plugin's clone was left in the workspace")
	}
}

func TestInstallRoleMismatchKeepsTheWorkingSetup(t *testing.T) {
	// The build succeeds, but the plugin turns out to be a sensor.
	useTempInstall(t)
	t.Setenv("FAKE_ROLE", "sensor")
	if err := Install(config.LoadConfig(), robotEntry(t, "new_robot"), io.Discard); err == nil {
		t.Fatal("a role mismatch must fail the install")
	}
	assertOldRobotIntact(t)
}

func TestInstallFailedCloneKeepsTheSourceBeingReinstalled(t *testing.T) {
	useTempInstall(t)
	entry := api.Plugin{Filename: "old_robot", EntryPoint: "old:Old", Role: config.RoleRobot,
		Repo: "file://" + filepath.Join(t.TempDir(), "unreachable")}
	if err := Install(config.LoadConfig(), entry, io.Discard); err == nil {
		t.Fatal("an unreachable repo must fail the install")
	}
	assertOldRobotIntact(t)
}

func TestInstallReplacesTheRobotAndKeepsOtherConfig(t *testing.T) {
	useTempInstall(t)
	// Loaded before the install; a pairing is saved while the build runs.
	stale := config.LoadConfig()
	if err := config.UpdateConfig(func(c *config.EMOSConfig) { c.Auth.PairingCodeHash = "paired" }); err != nil {
		t.Fatal(err)
	}

	if err := Install(stale, robotEntry(t, "new_robot"), io.Discard); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if exists(filepath.Join(config.PluginSrcDir(), "old_robot")) {
		t.Error("the replaced robot's source should be gone")
	}
	if !exists(filepath.Join(config.PluginSrcDir(), "new_robot", "package.xml")) {
		t.Error("the new robot was not placed in the workspace")
	}
	if got := overlayMarker(t); got != "new\n" {
		t.Errorf("overlay marker = %q, want the new build", got)
	}
	cfg := config.LoadConfig()
	if cfg.Plugin == nil || cfg.Plugin.Slug != "new_robot" {
		t.Errorf("config robot = %+v, want new_robot", cfg.Plugin)
	}
	if cfg.Auth.PairingCodeHash != "paired" {
		t.Error("the install overwrote a pairing saved while it ran")
	}
	if exists(txnDir()) {
		t.Error("the staging dir was left behind")
	}
}

func TestInstallRejectsAnUnsafeSlugBeforeTouchingAnything(t *testing.T) {
	useTempInstall(t)
	entry := robotEntry(t, "new_robot")
	entry.Filename = "../.."
	if err := Install(config.LoadConfig(), entry, io.Discard); err == nil {
		t.Fatal("an unsafe slug must be rejected")
	}
	assertOldRobotIntact(t)
}

func TestRemoveIsRecordedEvenWhenTheRebuildFails(t *testing.T) {
	useTempInstall(t)
	writeFile(t, filepath.Join(config.PluginSrcDir(), "sensor", "package.xml"), "sensor")
	err := config.UpdateConfig(func(c *config.EMOSConfig) {
		c.UpsertSensor(config.PluginInfo{Slug: "sensor", EntryPoint: "s:S", Role: config.RoleSensor})
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("FAKE_BUILD_FAIL", "1")
	if err := Remove(config.LoadConfig(), "sensor", io.Discard); err == nil {
		t.Fatal("a failed rebuild must be reported")
	}
	if config.LoadConfig().FindPlugin("sensor") != nil {
		t.Error("the removal was not recorded")
	}
	// The remaining robot keeps its previous, working overlay.
	assertOldRobotIntact(t)
}

func TestUpdateFailedBuildReturnsToThePreviousCommit(t *testing.T) {
	useTempInstall(t)
	entry := robotEntry(t, "new_robot")
	if err := Install(config.LoadConfig(), entry, io.Discard); err != nil {
		t.Fatalf("Install: %v", err)
	}

	// Upstream moves on, and the new commit does not build.
	origin := entry.Repo[len("file://"):]
	writeFile(t, filepath.Join(origin, "package.xml"), "v2")
	commit := exec.Command("git", "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qam", "v2")
	commit.Dir = origin
	if out, err := commit.CombinedOutput(); err != nil {
		t.Fatalf("commit: %v\n%s", err, out)
	}
	t.Setenv("FAKE_BUILD_FAIL", "1")
	if err := Update(config.LoadConfig(), io.Discard); err == nil {
		t.Fatal("a failed build must fail the update")
	}

	data, _ := os.ReadFile(filepath.Join(config.PluginSrcDir(), "new_robot", "package.xml"))
	if string(data) != "new_robot" {
		t.Errorf("source = %q; the overlay would run code it was not built from", data)
	}
	if got := overlayMarker(t); got != "new\n" {
		t.Errorf("overlay marker = %q; the previous overlay was not kept", got)
	}
}

func TestUpdateInstallsWhatTheUpdatedManifestDeclares(t *testing.T) {
	useTempInstall(t)
	entry := robotEntry(t, "new_robot")
	if err := Install(config.LoadConfig(), entry, io.Discard); err != nil {
		t.Fatalf("Install: %v", err)
	}

	// Upstream adds a dependency to the plugin's manifest.
	origin := entry.Repo[len("file://"):]
	writeFile(t, filepath.Join(origin, ManifestFile), "deps:\n  system:\n    conda: [libfoo]\n")
	for _, args := range [][]string{
		{"add", ManifestFile},
		{"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "needs libfoo"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = origin
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	if err := Update(config.LoadConfig(), io.Discard); err != nil {
		t.Fatalf("Update: %v", err)
	}
	adds, _ := os.ReadFile(os.Getenv("FAKE_PIXI_ADDS"))
	if !strings.Contains(string(adds), "libfoo") {
		t.Errorf("pixi add calls = %q; the new dependency was not installed", adds)
	}
}

func TestLockIsExclusive(t *testing.T) {
	useTempInstall(t)
	unlock, err := Lock()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Lock(); !errors.Is(err, ErrBusy) {
		t.Errorf("second Lock = %v, want ErrBusy", err)
	}
	if !Busy() {
		t.Error("Busy should report the held lock")
	}
	unlock()
	if Busy() {
		t.Error("Busy after unlock")
	}
}
