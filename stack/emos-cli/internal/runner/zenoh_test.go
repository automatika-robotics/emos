package runner

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/automatika-robotics/emos-cli/internal/config"
)

func TestValidRMW(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"rmw_fastrtps_cpp", true},
		{"rmw_cyclonedds_cpp", true},
		{"rmw_zenoh_cpp", true},
		{"", false},
		{"rmw_other", false},
		{"FastRTPS", false},
	}
	for _, tc := range cases {
		if got := ValidRMW(tc.in); got != tc.want {
			t.Errorf("ValidRMW(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestNewStrategySetsTheRMWOnlyWhenAsked(t *testing.T) {
	native := &config.EMOSConfig{Mode: config.ModeNative}
	s, err := newStrategy(native, "rmw_zenoh_cpp")
	if err != nil {
		t.Fatal(err)
	}
	if env := s.Command("true").Env; !slices.Contains(env, "RMW_IMPLEMENTATION=rmw_zenoh_cpp") {
		t.Error("the RMW asked for should be in the command's environment")
	}
	s, _ = newStrategy(native, "")
	if got, want := len(s.Command("true").Env), len(os.Environ()); got != want {
		t.Errorf("with none asked for the environment should be left as is: %d vars, want %d", got, want)
	}

	container := &config.EMOSConfig{Mode: config.ModeOSSContainer}
	s, _ = newStrategy(container, "rmw_zenoh_cpp")
	if got, want := lastArg(s.Command("run")), "source ros_entrypoint.sh && export 'RMW_IMPLEMENTATION=rmw_zenoh_cpp' && run"; got != want {
		t.Errorf("container command = %q, want %q", got, want)
	}
	s, _ = newStrategy(container, "")
	if got, want := lastArg(s.Command("run")), "source ros_entrypoint.sh && run"; got != want {
		t.Errorf("container command = %q, want %q", got, want)
	}

	if _, err := newStrategy(nil, ""); err == nil {
		t.Error("no install config must be an error")
	}
}

func lastArg(cmd *exec.Cmd) string { return cmd.Args[len(cmd.Args)-1] }

// routerStrategy stands in for an install mode whose commands the test builds.
type routerStrategy struct {
	RuntimeStrategy // only Command and RecipesDir are used
	command         func() *exec.Cmd
	shells          []string
}

func (r *routerStrategy) Command(shell string) *exec.Cmd {
	r.shells = append(r.shells, shell)
	return r.command()
}

func (r *routerStrategy) RecipesDir() string { return "/emos/recipes" }

// freeRouterAddr points the router checks at a port nothing listens on.
func freeRouterAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close()
	saved := zenohRouterAddr
	zenohRouterAddr = addr
	t.Cleanup(func() { zenohRouterAddr = saved })
	return addr
}

// listener returns a stand-in router: a process that listens on addr.
func listener(t *testing.T, addr string) func() *exec.Cmd {
	t.Helper()
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 is needed for a stand-in router")
	}
	host, port, _ := net.SplitHostPort(addr)
	script := fmt.Sprintf("import socket, time\n"+
		"s = socket.socket()\n"+
		"s.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)\n"+
		"s.bind((%q, %s))\n"+
		"s.listen()\n"+
		"time.sleep(60)\n", host, port)
	return func() *exec.Cmd { return exec.Command("python3", "-c", script) }
}

func TestZenohRouterAlreadyRunningIsLeftAlone(t *testing.T) {
	l, err := net.Listen("tcp", freeRouterAddr(t))
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	s := &routerStrategy{command: func() *exec.Cmd { return exec.Command("false") }}
	router, err := startZenohRouter(s, &recipeManifest{})
	if err != nil || router != nil || len(s.shells) != 0 {
		t.Errorf("startZenohRouter = %v, %v; want the running router used and nothing started", router, err)
	}
}

func TestZenohRouterStartedByTheRunIsStopped(t *testing.T) {
	s := &routerStrategy{command: listener(t, freeRouterAddr(t))}
	router, err := startZenohRouter(s, &recipeManifest{})
	if err != nil {
		t.Fatalf("startZenohRouter: %v", err)
	}
	if router == nil || !zenohRouterUp() {
		t.Fatal("the router should be running and listening")
	}
	stopZenohRouter(router)
	select {
	case <-router.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("the router is still running")
	}
}

func TestZenohRouterGetsTheRecipesConfigAsTheModeSeesIt(t *testing.T) {
	saved := config.RecipesDir
	config.RecipesDir = t.TempDir()
	t.Cleanup(func() { config.RecipesDir = saved })
	os.MkdirAll(filepath.Join(config.RecipesDir, "demo"), 0o755)
	os.WriteFile(filepath.Join(config.RecipesDir, "demo", "zenoh.json5"), []byte("{}"), 0o644)

	s := &routerStrategy{command: listener(t, freeRouterAddr(t))}
	router, err := startZenohRouter(s, &recipeManifest{ZenohRouterConfig: "demo/zenoh.json5"})
	if err != nil {
		t.Fatalf("startZenohRouter: %v", err)
	}
	defer stopZenohRouter(router)
	if len(s.shells) != 1 || !strings.HasPrefix(s.shells[0], "export ZENOH_ROUTER_CONFIG_URI='/emos/recipes/demo/zenoh.json5' && ") {
		t.Errorf("router shell = %q", s.shells)
	}
}

func TestZenohRouterThatExitsIsReported(t *testing.T) {
	freeRouterAddr(t)
	s := &routerStrategy{command: func() *exec.Cmd { return exec.Command("sh", "-c", "exit 1") }}
	if _, err := startZenohRouter(s, &recipeManifest{}); err == nil {
		t.Error("a router that exits while starting must fail the run")
	}
}

func TestZenohRouterThatNeverListensIsStopped(t *testing.T) {
	freeRouterAddr(t)
	saved := zenohRouterStartTimeout
	zenohRouterStartTimeout = 300 * time.Millisecond
	t.Cleanup(func() { zenohRouterStartTimeout = saved })

	cmd := exec.Command("sleep", "30")
	s := &routerStrategy{command: func() *exec.Cmd { return cmd }}
	if _, err := startZenohRouter(s, &recipeManifest{}); err == nil {
		t.Fatal("a router that never listens must fail the run")
	}
	deadline := time.Now().Add(3 * time.Second)
	for cmd.Process.Signal(syscall.Signal(0)) == nil {
		if time.Now().After(deadline) {
			t.Fatal("the router that never listened was left running")
		}
		time.Sleep(50 * time.Millisecond)
	}
}
