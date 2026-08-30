package plugin

import (
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/automatika-robotics/emos-cli/internal/api"
	"github.com/automatika-robotics/emos-cli/internal/config"
)

// useTempWorkspace points the package's workspace at a temp dir for the test.
func useTempWorkspace(t *testing.T) string {
	t.Helper()
	orig := config.WorkspaceDir
	config.WorkspaceDir = t.TempDir()
	t.Cleanup(func() { config.WorkspaceDir = orig })
	if err := os.MkdirAll(config.PluginSrcDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	return config.PluginSrcDir()
}

func mkdirs(t *testing.T, root string, names ...string) {
	t.Helper()
	for _, n := range names {
		if err := os.MkdirAll(filepath.Join(root, n), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func listDirs(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}

func TestGcSources(t *testing.T) {
	src := useTempWorkspace(t)
	// Workspace: robot m20 (+ its two rslidar sources), sensor hikvision, a
	// source shared by both, and two leftovers from a replaced robot.
	mkdirs(t, src,
		"m20_plugin", "rslidar_sdk", "rslidar_msg",
		"hikmicro_plugin", "shared_msgs",
		"old_robot", "old_robot_driver",
	)
	cfg := &config.EMOSConfig{
		Plugin: &config.PluginInfo{Slug: "m20_plugin", Sources: []string{"rslidar_sdk", "rslidar_msg", "shared_msgs"}},
		SensorPlugins: []config.PluginInfo{
			{Slug: "hikmicro_plugin", Sources: []string{"shared_msgs"}},
		},
	}
	// A stray file must be left alone: only directories are workspace packages.
	if err := os.WriteFile(filepath.Join(src, "README"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := gcSources(keepSet(cfg.Plugins()), io.Discard); err != nil {
		t.Fatal(err)
	}

	want := []string{"hikmicro_plugin", "m20_plugin", "rslidar_msg", "rslidar_sdk", "shared_msgs"}
	if got := listDirs(t, src); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("after gc: %v, want %v", got, want)
	}
	if _, err := os.Stat(filepath.Join(src, "README")); err != nil {
		t.Fatalf("stray file was removed: %v", err)
	}
}

func TestGcSourcesNoWorkspace(t *testing.T) {
	orig := config.WorkspaceDir
	config.WorkspaceDir = filepath.Join(t.TempDir(), "missing")
	t.Cleanup(func() { config.WorkspaceDir = orig })
	if err := gcSources(keepSet(nil), io.Discard); err != nil {
		t.Fatalf("missing src dir should be a no-op, got %v", err)
	}
}

func TestCloneSourcesRejectsMissingGit(t *testing.T) {
	useTempWorkspace(t)
	m := &Manifest{Sources: []Source{{Ref: "v1"}}}
	if _, err := cloneSources(m, io.Discard); err == nil {
		t.Fatal("expected an error for a source with no git URL")
	}
}

func TestCloneSourcesNilManifest(t *testing.T) {
	names, err := cloneSources(nil, io.Discard)
	if err != nil || names != nil {
		t.Fatalf("nil manifest: names=%v err=%v, want nil,nil", names, err)
	}
}

func sortedKeys(m map[string]bool) string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}

func TestInstallKeepSet(t *testing.T) {
	// Installed: robot m20 (rslidar_sdk, shared_msgs) + sensor hikvision (shared_msgs).
	base := func() *config.EMOSConfig {
		return &config.EMOSConfig{
			Plugin: &config.PluginInfo{Slug: "m20_plugin", Sources: []string{"rslidar_sdk", "shared_msgs"}},
			SensorPlugins: []config.PluginInfo{
				{Slug: "hikmicro_plugin", Sources: []string{"shared_msgs"}},
			},
		}
	}
	cases := []struct {
		name    string
		entry   api.Plugin
		sources []string
		want    string
	}{
		{
			name:    "robot replaced: old robot + its unshared source go, shared source stays",
			entry:   api.Plugin{Filename: "lite3_plugin", Role: config.RoleRobot},
			sources: []string{"livox_ros_driver2"},
			want:    "hikmicro_plugin,lite3_plugin,livox_ros_driver2,shared_msgs",
		},
		{
			name:    "robot reinstalled with a smaller manifest: dropped source goes",
			entry:   api.Plugin{Filename: "m20_plugin", Role: config.RoleRobot},
			sources: nil,
			want:    "hikmicro_plugin,m20_plugin,shared_msgs",
		},
		{
			name:    "sensor added: robot and everything it needs stays",
			entry:   api.Plugin{Filename: "thermal_plugin", Role: config.RoleSensor},
			sources: []string{"thermal_msgs"},
			want:    "hikmicro_plugin,m20_plugin,rslidar_sdk,shared_msgs,thermal_msgs,thermal_plugin",
		},
		{
			name:    "sensor reinstalled: its old sources are recomputed from the new manifest",
			entry:   api.Plugin{Filename: "hikmicro_plugin", Role: config.RoleSensor},
			sources: nil,
			want:    "hikmicro_plugin,m20_plugin,rslidar_sdk,shared_msgs",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sortedKeys(installKeepSet(base(), tc.entry, tc.sources)); got != tc.want {
				t.Fatalf("keep = %s\n     want %s", got, tc.want)
			}
		})
	}
}

func TestParseRole(t *testing.T) {
	cases := map[string]string{
		`{"role":"sensor"}`:            "sensor",
		`{"role":"robot"}`:             "robot",
		`{"role":"PluginRole.SENSOR"}`: "sensor", // tolerate an enum-style value
		`{"role":" Robot "}`:           "robot",  // trimmed + lowercased
		`{"metadata":{}}`:              "",
		`not json`:                     "",
	}
	for in, want := range cases {
		if got := parseRole([]byte(in)); got != want {
			t.Errorf("parseRole(%q) = %q, want %q", in, got, want)
		}
	}
}
