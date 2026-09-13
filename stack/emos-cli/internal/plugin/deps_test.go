package plugin

import (
	"reflect"
	"testing"
)

func TestPixiPackages(t *testing.T) {
	deps := Deps{
		ROS:    []string{"livox_ros_driver2", "realsense2_camera"},
		System: SystemDeps{Conda: []string{"libpcap"}},
	}
	got := pixiPackages(deps, "jazzy")
	want := []string{
		"ros-jazzy-livox-ros-driver2",
		"ros-jazzy-realsense2-camera",
		"libpcap",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("pixiPackages = %v, want %v", got, want)
	}
}

func TestPixiAddArgsResolveOnlyAarch64OnTheRobot(t *testing.T) {
	pkgs := []string{"ros-jazzy-livox-ros-driver2", "libpcap"}

	got := pixiAddArgs("/emos/pixi.toml", pkgs, "arm64")
	want := []string{"add", "--manifest-path", "/emos/pixi.toml",
		"--platform", "linux-aarch64", "ros-jazzy-livox-ros-driver2", "libpcap"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("arm64 = %v, want %v", got, want)
	}

	// An x86 dev machine keeps resolving every platform, aarch64 included.
	got = pixiAddArgs("/emos/pixi.toml", pkgs, "amd64")
	want = []string{"add", "--manifest-path", "/emos/pixi.toml", "ros-jazzy-livox-ros-driver2", "libpcap"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("amd64 = %v, want %v", got, want)
	}
}

func TestPixiPackagesEmpty(t *testing.T) {
	if got := pixiPackages(Deps{}, "jazzy"); len(got) != 0 {
		t.Fatalf("pixiPackages(empty) = %v, want empty", got)
	}
}

func TestDepsEmpty(t *testing.T) {
	if !depsEmpty(Deps{}) {
		t.Error("depsEmpty(zero) = false, want true")
	}
	if depsEmpty(Deps{System: SystemDeps{Apt: []string{"libpcap-dev"}}}) {
		t.Error("depsEmpty(with apt dep) = true, want false")
	}
}

func TestRosDistroPkg(t *testing.T) {
	got := rosDistroPkg("realsense2_camera_msgs", "jazzy")
	if want := "ros-jazzy-realsense2-camera-msgs"; got != want {
		t.Fatalf("rosDistroPkg = %q, want %q", got, want)
	}
}
