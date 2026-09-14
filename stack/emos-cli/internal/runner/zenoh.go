package runner

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/ui"
)

// zenohRouterAddr is where rmw_zenoh sessions look for a router by default.
var zenohRouterAddr = "127.0.0.1:7447"

// zenohRouterStartTimeout bounds how long a new router has to start listening.
var zenohRouterStartTimeout = 10 * time.Second

func zenohRouterUp() bool {
	conn, err := net.DialTimeout("tcp", zenohRouterAddr, 300*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// StartZenohRouter starts a Zenoh router in the strategy's environment and
// waits until it listens. A router already running is used as it is, and the
// handle is nil.
func StartZenohRouter(s RuntimeStrategy, manifest *recipeManifest) (*RunHandle, error) {
	if zenohRouterUp() {
		ui.Info("Using the Zenoh router already running on this machine.")
		if manifest.ZenohRouterConfig != "" {
			ui.Warn(manifest.ZenohRouterConfig + " is not applied to it.")
		}
		return nil, nil
	}

	shell := "exec ros2 run rmw_zenoh_cpp rmw_zenohd"
	if path := zenohRouterConfig(s, manifest.ZenohRouterConfig); path != "" {
		shell = "export ZENOH_ROUTER_CONFIG_URI=" + shellQuote(path) + " && " + shell
	}
	router, err := StartProcess(s.Command(shell), "")
	if err != nil {
		return nil, fmt.Errorf("start zenoh router: %w", err)
	}
	deadline := time.After(zenohRouterStartTimeout)
	for !zenohRouterUp() {
		select {
		case <-router.Done():
			return nil, fmt.Errorf("the zenoh router exited while starting")
		case <-deadline:
			router.Cancel(time.Second)
			return nil, fmt.Errorf("the zenoh router did not start listening on %s", zenohRouterAddr)
		case <-time.After(200 * time.Millisecond):
		}
	}
	ui.Success("Zenoh router started.")
	return router, nil
}

// zenohRouterConfig returns a recipe's router config file as the strategy's
// commands see it, or "" to use the default configuration.
func zenohRouterConfig(s RuntimeStrategy, file string) string {
	if file == "" {
		return ""
	}
	if !strings.HasSuffix(file, ".json5") {
		ui.Warn("Zenoh config must be .json5 — using default")
		return ""
	}
	// Checked on the host, where a container's recipes are mounted from.
	if _, err := os.Stat(filepath.Join(config.RecipesDir, file)); err != nil {
		ui.Warn("Zenoh config file not found — using default")
		return ""
	}
	path := s.RecipesDir() + "/" + file
	ui.Info("Using Zenoh router config: " + path)
	return path
}

// StopZenohRouter stops a router the run started. Nil is a no-op. In a
// container the router goes with the container.
func StopZenohRouter(router *RunHandle) {
	if router != nil {
		router.Cancel(2 * time.Second)
	}
}
