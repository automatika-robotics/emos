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
	got := RosDistroPkg("realsense2_camera_msgs", "jazzy")
	if want := "ros-jazzy-realsense2-camera-msgs"; got != want {
		t.Fatalf("RosDistroPkg = %q, want %q", got, want)
	}
}
