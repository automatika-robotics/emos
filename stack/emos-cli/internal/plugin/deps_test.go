package plugin

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// pkgXML builds a minimal package.xml (format 3) with a name and exec_depends.
func pkgXML(name string, deps ...string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0"?>` + "\n")
	b.WriteString(`<package format="3">` + "\n")
	b.WriteString("  <name>" + name + "</name>\n")
	for _, d := range deps {
		b.WriteString("  <exec_depend>" + d + "</exec_depend>\n")
	}
	b.WriteString("</package>\n")
	return b.String()
}

// writePkg drops a package.xml into root/<name>/.
func writePkg(t *testing.T, root, name, body string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "package.xml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestExternalDeps(t *testing.T) {
	// What the env probe (ros2 pkg list) would report as installed: standard
	// ROS + the source-built EMOS stack.
	envInstalled := []string{
		"automatika_ros_sugar", "kompass", "embodied_agents",
		"sensor_msgs", "std_msgs", "nav_msgs", "geometry_msgs", "tf2_ros",
		"ament_index_python", "launch", "launch_ros",
	}
	cases := []struct {
		name     string
		declared []string
		srcExtra []string // packages provided by vendored source in the workspace
		want     []string
	}{
		{
			name: "hikvision: only baseline + pip",
			declared: []string{"automatika_ros_sugar", "python3-numpy", "python3-opencv",
				"python3-httpx", "sensor_msgs", "std_msgs", "geometry_msgs", "tf2_ros"},
			want: nil,
		},
		{
			name: "m20 without vendoring: rslidar is external",
			declared: []string{"automatika_ros_sugar", "python3-numpy", "nav_msgs",
				"sensor_msgs", "tf2_ros", "ament_index_python", "rslidar_sdk"},
			want: []string{"rslidar_sdk"},
		},
		{
			name:     "m20 with vendored rslidar: nothing external",
			declared: []string{"automatika_ros_sugar", "rslidar_sdk"},
			srcExtra: []string{"rslidar_sdk", "rslidar_msg"},
			want:     nil,
		},
		{
			name: "lite3: livox + realsense external, standard ROS filtered out",
			declared: []string{"automatika_ros_sugar", "python3-numpy", "nav_msgs",
				"sensor_msgs", "tf2_ros", "ament_index_python",
				"livox_ros_driver2", "realsense2_camera", "realsense2_camera_msgs"},
			want: []string{"livox_ros_driver2", "realsense2_camera", "realsense2_camera_msgs"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			provided := map[string]bool{}
			for _, p := range envInstalled {
				provided[p] = true
			}
			for _, p := range tc.srcExtra {
				provided[p] = true
			}
			got := externalDeps(tc.declared, provided)
			sort.Strings(got)
			sort.Strings(tc.want)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("externalDeps = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCollectDeps(t *testing.T) {
	root := t.TempDir()
	writePkg(t, root, "lite3_plugin", pkgXML("lite3_plugin", "sensor_msgs", "livox_ros_driver2"))
	writePkg(t, root, "livox_ros_driver2", pkgXML("livox_ros_driver2", "pcl_ros"))

	declared, provided, err := collectDeps(root)
	if err != nil {
		t.Fatal(err)
	}
	// The workspace provides both package names.
	if !provided["lite3_plugin"] || !provided["livox_ros_driver2"] {
		t.Fatalf("provided = %v, want lite3_plugin + livox_ros_driver2", provided)
	}
	// Declared is the union across every package.xml in the tree.
	for _, want := range []string{"sensor_msgs", "livox_ros_driver2", "pcl_ros"} {
		found := false
		for _, d := range declared {
			if d == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("declared %v missing %q", declared, want)
		}
	}
}

func TestRosDistroPkg(t *testing.T) {
	got := rosDistroPkg("realsense2_camera_msgs", "jazzy")
	if want := "ros-jazzy-realsense2-camera-msgs"; got != want {
		t.Fatalf("rosDistroPkg = %q, want %q", got, want)
	}
}
