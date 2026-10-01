package installer

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// makeFakePixi creates an executable file named "pixi" at the given path
// (parent dirs created) and returns its absolute path. Skips on Windows since
// pixi support is *nix-only.
func makeFakePixi(t *testing.T, path string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("pixi support is unix-only")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho fake-pixi\n"), 0755); err != nil {
		t.Fatal(err)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

func TestResolvePixi_FromPath(t *testing.T) {
	tmp := t.TempDir()
	binDir := filepath.Join(tmp, "bin")
	fakeBin := makeFakePixi(t, filepath.Join(binDir, "pixi"))

	t.Setenv("PATH", binDir)
	t.Setenv("HOME", tmp) // make sure ~/.pixi/bin doesn't accidentally win

	got, err := ResolvePixi()
	if err != nil {
		t.Fatalf("ResolvePixi err = %v, want nil", err)
	}
	if got != fakeBin {
		t.Errorf("ResolvePixi = %q, want %q (from PATH)", got, fakeBin)
	}
}

func TestResolvePixi_FallsBackToHome(t *testing.T) {
	// PATH has nothing useful; the binary lives at ~/.pixi/bin/pixi (where the
	// pixi installer puts it). This is the systemd-mode case where PATH is bare.
	tmp := t.TempDir()
	emptyPath := filepath.Join(tmp, "empty-path")
	_ = os.MkdirAll(emptyPath, 0755)

	homeDir := filepath.Join(tmp, "home")
	expected := makeFakePixi(t, filepath.Join(homeDir, ".pixi", "bin", "pixi"))

	t.Setenv("PATH", emptyPath)
	t.Setenv("HOME", homeDir)

	got, err := ResolvePixi()
	if err != nil {
		t.Fatalf("ResolvePixi err = %v, want nil", err)
	}
	if got != expected {
		t.Errorf("ResolvePixi = %q, want %q (from ~/.pixi/bin)", got, expected)
	}
}

func TestResolvePixi_NotFound(t *testing.T) {
	tmp := t.TempDir()
	emptyPath := filepath.Join(tmp, "empty-path")
	_ = os.MkdirAll(emptyPath, 0755)

	t.Setenv("PATH", emptyPath)
	t.Setenv("HOME", filepath.Join(tmp, "no-pixi-here"))

	if _, err := ResolvePixi(); err == nil {
		t.Fatalf("ResolvePixi err = nil, want failure when pixi missing everywhere")
	}
}

// recordingPixi puts a pixi on the PATH that appends each call to the returned
// file, as "<directory>|<arguments>|<CUDA toolkit>", and fails the verb the
// project names in "fail". Otherwise it builds the named wheels when asked to
// run a task, and keeps what it is asked to add in the manifest until asked to
// remove it.
func recordingPixi(t *testing.T, wheels ...string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("pixi support is unix-only")
	}
	bin := t.TempDir()
	record := filepath.Join(t.TempDir(), "record")
	script := "#!/bin/sh\necho \"$PWD|$*|$EMOS_MAPPING_CUDA$EMOS_CUDA\" >> " + record + "\n" +
		"if grep -q \"^$1$\" fail 2>/dev/null; then exit 1; fi\n" +
		"case \"$1\" in\n" +
		"run) mkdir -p cuda_wheels; for w in " + strings.Join(wheels, " ") + "; do : > cuda_wheels/$w; done ;;\n" +
		"add) echo \"$*\" >> pixi.toml ;;\n" +
		"remove) : > pixi.toml ;;\n" +
		"esac\n"
	if err := os.WriteFile(filepath.Join(bin, "pixi"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return record
}

func TestInstallMappingBackendAddsItsROSPackageThenRunsTheTask(t *testing.T) {
	project := t.TempDir()
	record := recordingPixi(t)
	resolved, _ := filepath.EvalSymlinks(project)
	add := resolved + "|" + strings.Join(PixiAddArgs(filepath.Join(project, "pixi.toml"),
		[]string{"ros-jazzy-image-transport"}, runtime.GOARCH), " ") + "|/usr/local/cuda-12.6\n"
	task := resolved + "|run install-mapping-backend|/usr/local/cuda-12.6\n"

	env := append(os.Environ(), "EMOS_MAPPING_CUDA=/usr/local/cuda-12.6")
	if err := InstallMappingBackend(project, "jazzy", env); err != nil {
		t.Fatalf("InstallMappingBackend: %v", err)
	}
	if got, _ := os.ReadFile(record); string(got) != add+task {
		t.Errorf("pixi was called as\n%s\nwant\n%s", got, add+task)
	}

	// A package that cannot be added ends it before anything is built.
	os.Remove(record)
	os.WriteFile(filepath.Join(project, "fail"), []byte("add\n"), 0o644)
	if err := InstallMappingBackend(project, "jazzy", env); err == nil || !strings.Contains(err.Error(), "pixi add failed") {
		t.Errorf("a failed add must be reported, got %v", err)
	}
	if got, _ := os.ReadFile(record); string(got) != add {
		t.Errorf("nothing should be built after a failed add, pixi was called as\n%s", got)
	}

	os.WriteFile(filepath.Join(project, "fail"), []byte("run\n"), 0o644)
	if err := InstallMappingBackend(project, "jazzy", env); err == nil || !strings.Contains(err.Error(), "pixi run install-mapping-backend failed") {
		t.Errorf("a failed build must be reported by its task, got %v", err)
	}
}

func TestPixiAddArgsResolveOnlyAarch64OnTheRobot(t *testing.T) {
	pkgs := []string{"ros-jazzy-livox-ros-driver2", "libpcap"}

	got := PixiAddArgs("/emos/pixi.toml", pkgs, "arm64")
	want := []string{"add", "--manifest-path", "/emos/pixi.toml",
		"--platform", "linux-aarch64", "ros-jazzy-livox-ros-driver2", "libpcap"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("arm64 = %v, want %v", got, want)
	}

	// An x86 dev machine keeps resolving every platform, aarch64 included.
	got = PixiAddArgs("/emos/pixi.toml", pkgs, "amd64")
	want = []string{"add", "--manifest-path", "/emos/pixi.toml", "ros-jazzy-livox-ros-driver2", "libpcap"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("amd64 = %v, want %v", got, want)
	}
}

const (
	sherpaWheel = "sherpa_onnx-1.13.8+cuda-cp312-cp312-linux_aarch64.whl"
	llamaWheel  = "llama_cpp_python-0.3.35-py3-none-linux_aarch64.whl"
)

func TestInstallCUDAPackagesBuildsThenHandsTheWheelsToPixi(t *testing.T) {
	project := t.TempDir()
	record := recordingPixi(t, sherpaWheel, llamaWheel)
	if HasCUDAPackages(project) {
		t.Fatal("a fresh workspace has no CUDA packages")
	}

	if err := InstallCUDAPackages(project, "/usr/local/cuda-12.6", os.Environ()); err != nil {
		t.Fatalf("InstallCUDAPackages: %v", err)
	}
	resolved, _ := filepath.EvalSymlinks(project)
	wheels := filepath.Join(project, "cuda_wheels")
	specs := []string{
		"sherpa-onnx @ file://" + filepath.Join(wheels, sherpaWheel),
		"llama-cpp-python @ file://" + filepath.Join(wheels, llamaWheel),
	}
	want := resolved + "|run install-cuda-packages|/usr/local/cuda-12.6\n" +
		resolved + "|" + strings.Join(cudaPackagesArgs("add", specs, runtime.GOARCH), " ") + "|/usr/local/cuda-12.6\n"
	if got, _ := os.ReadFile(record); string(got) != want {
		t.Errorf("pixi was called as\n%s\nwant\n%s", got, want)
	}
	if !HasCUDAPackages(project) {
		t.Error("the workspace should now report its CUDA packages")
	}

	// Removing them takes the platform's entries out, and the wheels with them.
	os.Remove(record)
	if err := RemoveCUDAPackages(project, os.Environ()); err != nil {
		t.Fatalf("RemoveCUDAPackages: %v", err)
	}
	want = resolved + "|" + strings.Join(cudaPackagesArgs("remove", CUDAPackages, runtime.GOARCH), " ") + "|\n"
	if got, _ := os.ReadFile(record); string(got) != want {
		t.Errorf("pixi was called as\n%s\nwant\n%s", got, want)
	}
	if HasCUDAPackages(project) {
		t.Error("the manifest should no longer name the wheels")
	}
	if _, err := os.Stat(wheels); !os.IsNotExist(err) {
		t.Error("the wheels should be gone")
	}
}

// pixi remove fails on entries that are not in the manifest, which would end
// every later update.
func TestWheelsPixiRefusedAreNotReportedAsInstalled(t *testing.T) {
	project := t.TempDir()
	recordingPixi(t, sherpaWheel, llamaWheel)
	os.WriteFile(filepath.Join(project, "fail"), []byte("add\n"), 0o644)
	if err := InstallCUDAPackages(project, "/usr/local/cuda-12.6", os.Environ()); err == nil {
		t.Fatal("a refused add must be reported")
	}
	if HasCUDAPackages(project) {
		t.Error("wheels that never reached the manifest are not installed")
	}
}

func TestInstallCUDAPackagesNeedsAWheelForEachPackage(t *testing.T) {
	project := t.TempDir()
	record := recordingPixi(t, sherpaWheel) // the llama-cpp-python build left nothing
	err := InstallCUDAPackages(project, "/usr/local/cuda-12.6", os.Environ())
	if err == nil || !strings.Contains(err.Error(), "llama-cpp-python") {
		t.Errorf("want an error naming the missing wheel, got %v", err)
	}
	if got, _ := os.ReadFile(record); strings.Contains(string(got), "add ") {
		t.Errorf("nothing should be handed to pixi, it was called as\n%s", got)
	}

	// A build that fails ends it there too.
	project = t.TempDir()
	record = recordingPixi(t, sherpaWheel, llamaWheel)
	os.WriteFile(filepath.Join(project, "fail"), []byte("run\n"), 0o644)
	if err := InstallCUDAPackages(project, "/usr/local/cuda-12.6", os.Environ()); err == nil {
		t.Error("a failed build must be reported")
	}
	if got, _ := os.ReadFile(record); strings.Contains(string(got), "add ") {
		t.Errorf("nothing should be handed to pixi after a failed build, it was called as\n%s", got)
	}
}

func TestCUDAPackagesArgsNameThePlatformOfTheHost(t *testing.T) {
	got := cudaPackagesArgs("add", []string{"sherpa-onnx @ file:///w/s.whl"}, "arm64")
	want := []string{"add", "--pypi", "--platform", "linux-aarch64", "sherpa-onnx @ file:///w/s.whl"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("arm64 = %v, want %v", got, want)
	}

	// A wheel is for one architecture, so an x86 host names its platform too.
	got = cudaPackagesArgs("remove", CUDAPackages, "amd64")
	want = []string{"remove", "--pypi", "--platform", "linux-64", "sherpa-onnx", "llama-cpp-python"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("amd64 = %v, want %v", got, want)
	}
}

func TestAnUpdateKeepsTheCUDAWheelsAndPutsThemBack(t *testing.T) {
	project := t.TempDir()
	record := recordingPixi(t, sherpaWheel, llamaWheel)
	if err := InstallCUDAPackages(project, "/usr/local/cuda-12.6", os.Environ()); err != nil {
		t.Fatalf("InstallCUDAPackages: %v", err)
	}
	wheels := filepath.Join(project, "cuda_wheels")

	if err := SetAsideCUDAPackages(project, os.Environ()); err != nil {
		t.Fatalf("SetAsideCUDAPackages: %v", err)
	}
	if HasCUDAPackages(project) || !KeptCUDAWheels(project) {
		t.Fatal("the manifest should let go of the wheels, and the wheels should stay")
	}
	// Set aside twice is fine: pixi is not asked to remove what is not there
	os.Remove(record)
	if err := SetAsideCUDAPackages(project, os.Environ()); err != nil {
		t.Fatalf("a second SetAsideCUDAPackages: %v", err)
	}
	if got, _ := os.ReadFile(record); len(got) != 0 {
		t.Errorf("pixi should not have been called, it was called as\n%s", got)
	}

	if err := RestoreCUDAPackages(project, os.Environ()); err != nil {
		t.Fatalf("RestoreCUDAPackages: %v", err)
	}
	if !HasCUDAPackages(project) {
		t.Error("the manifest should name the wheels again")
	}
	calls, _ := os.ReadFile(record)
	add := strings.Index(string(calls), "|add ")
	probe := strings.Index(string(calls), "llama_supports_gpu_offload")
	if add < 0 || probe < add || !strings.Contains(string(calls), "sherpa-onnx llama-cpp-python|") {
		t.Errorf("want the add, then the probe of both packages, pixi was called as\n%s", calls)
	}

	// Wheels that fail the probe are reported, for the caller to remove
	os.WriteFile(filepath.Join(project, "fail"), []byte("run\n"), 0o644)
	if err := RestoreCUDAPackages(project, os.Environ()); err == nil || err.Error() != "they no longer load" {
		t.Errorf("a failed probe must be reported, got %v", err)
	}
	os.Remove(filepath.Join(project, "fail"))
	if err := RemoveCUDAPackages(project, os.Environ()); err != nil {
		t.Fatalf("RemoveCUDAPackages: %v", err)
	}
	if _, err := os.Stat(wheels); !os.IsNotExist(err) || KeptCUDAWheels(project) {
		t.Error("the wheels should be gone")
	}
}

func TestTheCUDAProbeChecksVersionsAndTheGPU(t *testing.T) {
	if exec.Command("python3", "-c", "import tomllib, packaging").Run() != nil {
		t.Skip("python3 with tomllib and packaging is not available")
	}
	anyVersion := "[pypi-dependencies]\nsherpa-onnx = \"*\"\nllama-cpp-python = { version = \"*\", index = \"https://x\" }\n"
	cases := []struct {
		name, manifest string
		offload, keep  bool
		reason         string
	}{
		{"any version", anyVersion, true, true, ""},
		{"not named at all", "[dependencies]\nnumpy = \"*\"\n", true, true, ""},
		{"a minimum the build meets", "[pypi-dependencies]\nsherpa-onnx = \">=1.13\"\n", true, true, ""},
		{"a minimum past the build", "[pypi-dependencies]\nsherpa-onnx = \">=1.14\"\n", true, false, "sherpa-onnx 1.13.8 does not satisfy >=1.14"},
		{"a pin in a table", "[pypi-dependencies]\nllama-cpp-python = { version = \"==0.3.40\", index = \"https://x\" }\n", true, false, "llama-cpp-python 0.3.35 does not satisfy ==0.3.40"},
		{"a build without the GPU", anyVersion, false, false, "llama-cpp-python does not use the GPU"},
	}
	for _, c := range cases {
		project, site := t.TempDir(), t.TempDir()
		os.WriteFile(filepath.Join(project, "pixi.toml"), []byte(c.manifest), 0o644)
		offload := "False"
		if c.offload {
			offload = "True"
		}
		os.WriteFile(filepath.Join(site, "sherpa_onnx.py"), nil, 0o644)
		os.WriteFile(filepath.Join(site, "llama_cpp.py"), []byte("def llama_supports_gpu_offload(): return "+offload+"\n"), 0o644)
		for name, version := range map[string]string{"sherpa-onnx": "1.13.8+cuda", "llama-cpp-python": "0.3.35"} {
			info := filepath.Join(site, strings.ReplaceAll(name, "-", "_")+"-"+version+".dist-info")
			os.MkdirAll(info, 0o755)
			os.WriteFile(filepath.Join(info, "METADATA"), []byte("Metadata-Version: 2.1\nName: "+name+"\nVersion: "+version+"\n"), 0o644)
		}
		cmd := exec.Command("bash", "-c", cudaProbe+" "+strings.Join(CUDAPackages, " "))
		cmd.Dir = project
		cmd.Env = append(os.Environ(), "PYTHONPATH="+site)
		out, err := cmd.Output()
		if (err == nil) != c.keep || lastLine(string(out)) != c.reason {
			t.Errorf("%s: keep=%v reason=%q", c.name, err == nil, lastLine(string(out)))
		}
	}

	// Packages that are not there at all end the probe without a reason
	cmd := exec.Command("bash", "-c", cudaProbe+" "+strings.Join(CUDAPackages, " "))
	cmd.Dir = t.TempDir()
	cmd.Env = append(os.Environ(), "PYTHONPATH="+t.TempDir())
	if out, err := cmd.Output(); err == nil || lastLine(string(out)) != "" {
		t.Errorf("missing packages: err=%v out=%q", err, out)
	}
}

func TestWithoutSourcedROSDropsOnlyWhatTheSetupFileAdded(t *testing.T) {
	env := []string{
		"HOME=/home/u",
		"AMENT_PREFIX_PATH=/opt/ros/jazzy:/home/u/ros2_ws/install",
		"COLCON_PREFIX_PATH=/home/u/ros2_ws/install",
		"PATH=/home/u/ros2_ws/install/bin:/opt/ros/jazzy/bin:/usr/local/cuda/bin:/usr/bin",
		"LD_LIBRARY_PATH=/opt/ros/jazzy/lib:/opt/ros/jazzy/opt/gz_sim_vendor/lib:/usr/local/cuda/lib64:/home/u/ros2_ws/install/lib",
		"PYTHONPATH=/opt/ros/jazzy/lib/python3.12/site-packages:/home/u/tools",
		"CMAKE_PREFIX_PATH=/opt/ros/jazzy",
		"ROS_DISTRO=jazzy",
		"ROS_VERSION=2",
		"ROS_PYTHON_VERSION=3",
		"ROS_DOMAIN_ID=5",
		"ROS_AUTOMATIC_DISCOVERY_RANGE=SUBNET",
		"RMW_IMPLEMENTATION=rmw_cyclonedds_cpp",
		"HIKMICRO_PASS=secret",
	}
	want := []string{
		"HOME=/home/u",
		"PATH=/usr/local/cuda/bin:/usr/bin",
		"LD_LIBRARY_PATH=/usr/local/cuda/lib64",
		"PYTHONPATH=/home/u/tools",
		"ROS_DOMAIN_ID=5",
		"ROS_AUTOMATIC_DISCOVERY_RANGE=SUBNET",
		"RMW_IMPLEMENTATION=rmw_cyclonedds_cpp",
		"HIKMICRO_PASS=secret",
	}
	if got := WithoutSourcedROS(env); !reflect.DeepEqual(got, want) {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	// A shell that never sourced ROS is passed through as it is, /opt/ros aside.
	plain := []string{"PATH=/usr/bin:/opt/ros/humble/bin", "LD_LIBRARY_PATH=/usr/local/cuda/lib64", "ROS_DOMAIN_ID=3"}
	if got := WithoutSourcedROS(plain); !reflect.DeepEqual(got, []string{"PATH=/usr/bin", "LD_LIBRARY_PATH=/usr/local/cuda/lib64", "ROS_DOMAIN_ID=3"}) {
		t.Errorf("plain shell: got %v", got)
	}
}
