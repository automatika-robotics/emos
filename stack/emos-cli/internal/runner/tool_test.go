package runner

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/automatika-robotics/emos-cli/internal/config"
)

func TestUISecurityRunsInTheRecipesEnvironment(t *testing.T) {
	withTempDirs(t)
	bin := t.TempDir()
	// pixi run --manifest-path <toml> bash -c <shell>: just runs the shell.
	os.WriteFile(filepath.Join(bin, "pixi"), []byte("#!/bin/sh\nshift 3\nexec \"$@\"\n"), 0o755)
	// ros2 run <pkg> <exe> <args...>: shows what it was asked to run, and where.
	os.WriteFile(filepath.Join(bin, "ros2"), []byte("#!/bin/sh\necho \"$@\"\necho \"data dir: $SUGARCOAT_UI_DATA_DIR\"\n"), 0o755)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	project := t.TempDir()
	os.MkdirAll(filepath.Join(project, "install"), 0o755)
	os.WriteFile(filepath.Join(project, "pixi.toml"), nil, 0o644)
	os.WriteFile(filepath.Join(project, "install", "setup.sh"), nil, 0o644)
	cfg := &config.EMOSConfig{Mode: config.ModePixi, PixiProjectDir: project}

	var out bytes.Buffer
	if err := RunUISecurity(cfg, &out, "keys", "create", "--name", "mission control", "--scopes", "read,command"); err != nil {
		t.Fatal(err)
	}
	if want := "run automatika_ros_sugar ui_security keys create --name mission control --scopes read,command\n"; !strings.Contains(out.String(), want) {
		t.Errorf("ros2 was asked for %q, want %q", out.String(), want)
	}
	if want := "data dir: " + config.UISecurityDir; !strings.Contains(out.String(), want) {
		t.Errorf("output %q lacks %q; the tool must act on the recipes' state directory", out.String(), want)
	}
}

func TestUISecurityReportsTheToolsFailureQuietly(t *testing.T) {
	withTempDirs(t)
	bin := t.TempDir()
	os.WriteFile(filepath.Join(bin, "pixi"), []byte("#!/bin/sh\nshift 3\nexec \"$@\"\n"), 0o755)
	os.WriteFile(filepath.Join(bin, "ros2"), []byte("#!/bin/sh\nexit 1\n"), 0o755)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	project := t.TempDir()
	os.MkdirAll(filepath.Join(project, "install"), 0o755)
	os.WriteFile(filepath.Join(project, "pixi.toml"), nil, 0o644)
	os.WriteFile(filepath.Join(project, "install", "setup.sh"), nil, 0o644)
	cfg := &config.EMOSConfig{Mode: config.ModePixi, PixiProjectDir: project}

	err := RunUISecurity(cfg, &bytes.Buffer{}, "keys", "list")
	if err == nil || !strings.Contains(err.Error(), "status 1") {
		t.Errorf("err = %v, want the tool's exit status", err)
	}
}
