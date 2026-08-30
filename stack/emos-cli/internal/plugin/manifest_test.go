package plugin

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadManifest(t *testing.T) {
	dir := t.TempDir()
	body := `
sources:
  - git: https://github.com/RoboSense-LiDAR/rslidar_sdk
    ref: v1.5.9
    recursive: true
  - git: https://github.com/RoboSense-LiDAR/rslidar_msg
    ref: v1.5.9
deps:
  ros:
    - livox_ros_driver2
  system:
    conda: [libpcap]
    apt: [libpcap-dev]
`
	if err := os.WriteFile(filepath.Join(dir, ManifestFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := LoadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m == nil {
		t.Fatal("manifest is nil")
	}
	if len(m.Sources) != 2 {
		t.Fatalf("sources = %d, want 2", len(m.Sources))
	}
	if m.Sources[0].Git != "https://github.com/RoboSense-LiDAR/rslidar_sdk" ||
		m.Sources[0].Ref != "v1.5.9" || !m.Sources[0].Recursive {
		t.Fatalf("source[0] = %+v", m.Sources[0])
	}
	if m.Sources[1].Recursive {
		t.Errorf("source[1] recursive = true, want false (default)")
	}
	if !reflect.DeepEqual(m.Deps.ROS, []string{"livox_ros_driver2"}) {
		t.Errorf("deps.ros = %v", m.Deps.ROS)
	}
	if !reflect.DeepEqual(m.Deps.System.Conda, []string{"libpcap"}) {
		t.Errorf("deps.system.conda = %v", m.Deps.System.Conda)
	}
	if !reflect.DeepEqual(m.Deps.System.Apt, []string{"libpcap-dev"}) {
		t.Errorf("deps.system.apt = %v", m.Deps.System.Apt)
	}
}

func TestLoadManifestAbsent(t *testing.T) {
	m, err := LoadManifest(t.TempDir())
	if err != nil {
		t.Fatalf("absent manifest should not error, got %v", err)
	}
	if m != nil {
		t.Fatalf("absent manifest should be nil, got %+v", m)
	}
}

func TestSourcePackageName(t *testing.T) {
	cases := []struct {
		src  Source
		want string
	}{
		{Source{Git: "https://github.com/RoboSense-LiDAR/rslidar_sdk"}, "rslidar_sdk"},
		{Source{Git: "https://github.com/RoboSense-LiDAR/rslidar_sdk.git"}, "rslidar_sdk"},
		{Source{Git: "https://github.com/RoboSense-LiDAR/rslidar_sdk/"}, "rslidar_sdk"},
		{Source{Git: "git@github.com:RoboSense-LiDAR/rslidar_msg.git"}, "rslidar_msg"},
		{Source{Git: "https://example.com/foo/bar", Name: "custom_dir"}, "custom_dir"},
	}
	for _, tc := range cases {
		if got := tc.src.PackageName(); got != tc.want {
			t.Errorf("PackageName(%q, name=%q) = %q, want %q", tc.src.Git, tc.src.Name, got, tc.want)
		}
	}
}

func TestLoadManifestRejectsUnknownKeys(t *testing.T) {
	dir := t.TempDir()
	// "source" (singular) is a typo for "sources": it must fail loudly rather
	// than parse as a manifest with no sources.
	body := "source:\n  - git: https://example.com/foo\n"
	if err := os.WriteFile(filepath.Join(dir, ManifestFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadManifest(dir); err == nil {
		t.Fatal("expected an error for an unknown top-level key")
	}
}
