package runner

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/ui"
)

const (
	emosRoot       = "/emos"
	recipesRoot    = "/emos/recipes"
	uiSecurityRoot = "/emos/.ui-security"
)

type recipeManifest struct {
	ZenohRouterConfig string `json:"zenoh_router_config_file"`
	// The version of the recipe that was installed, from the portal
	Variant struct {
		ID      string   `json:"id"`
		Robot   string   `json:"robot"`
		Sensors []string `json:"sensors"`
	} `json:"variant"`
}

// WrongRobot is the robot plugin a recipe was made for, when it is not the one
// installed. Empty for a generic recipe or a matching one.
func (m *recipeManifest) WrongRobot(cfg *config.EMOSConfig) string {
	robot := m.Variant.Robot
	if robot == "" || (cfg != nil && cfg.Plugin != nil && cfg.Plugin.Slug == robot) {
		return ""
	}
	return robot
}

// LoadManifest reads a recipe manifest file. Always returns a non-nil pointer
// — a missing or malformed manifest yields an empty manifest, since the file
// is optional.
func LoadManifest(path string) *recipeManifest {
	m := &recipeManifest{}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, m)
	}
	return m
}

// zenohRMW is the RMW implementation that needs a Zenoh router running.
const zenohRMW = "rmw_zenoh_cpp"

// RMWChoices are the RMW implementations an EMOS install carries.
const RMWChoices = "rmw_fastrtps_cpp, rmw_zenoh_cpp"

// ValidRMW reports whether rmw is one of RMWChoices.
func ValidRMW(rmw string) bool {
	switch rmw {
	case "rmw_fastrtps_cpp", zenohRMW:
		return true
	}
	return false
}

// CheckRMW rejects an RMW implementation a run cannot ask for. Empty asks for
// none, which is allowed.
func CheckRMW(rmw string) error {
	if rmw == "" || ValidRMW(rmw) {
		return nil
	}
	return fmt.Errorf("invalid RMW implementation: %s (allowed: %s)", rmw, RMWChoices)
}

// RMWLabel names the RMW a run asked for, where empty means none was.
func RMWLabel(rmw string) string {
	if rmw == "" {
		return "environment default"
	}
	return rmw
}

// RunRecipe runs a recipe in the foreground. An empty rmwImpl sets no RMW
// implementation, leaving the environment's.
func RunRecipe(recipeName, rmwImpl string) error {
	if err := CheckRMW(rmwImpl); err != nil {
		return err
	}

	// Check recipe exists
	recipePath := filepath.Join(config.RecipesDir, recipeName)
	recipeFile := filepath.Join(recipePath, "recipe.py")
	if _, err := os.Stat(recipeFile); os.IsNotExist(err) {
		ui.Error(fmt.Sprintf("Recipe '%s' not found in '%s'", recipeName, config.RecipesDir))
		ui.Faint("Run 'emos ls' to see available recipes.")
		return fmt.Errorf("recipe not found")
	}

	manifest := LoadManifest(filepath.Join(recipePath, "manifest.json"))

	cfg := config.LoadConfig()
	if !cfg.IsInstalled() {
		return errNotInstalled
	}
	if robot := manifest.WrongRobot(cfg); robot != "" {
		ui.Warn("This recipe was installed for the " + cfg.PluginLabel(robot) + ", which is not the robot installed now.")
		ui.Faint("'emos pull " + recipeName + "' installs the version for this robot.")
	}

	logFile := LogFilePath(recipeName)
	log, err := OpenLog(logFile)
	if err != nil {
		return err
	}
	defer log.Close()

	ui.Header("EMOS - PRE-RECIPE SETUP")
	ui.Info("Recipe Name: " + recipeName)
	ui.Info("Mode: " + string(cfg.Mode))
	ui.Info("RMW Implementation: " + RMWLabel(rmwImpl))

	// From here a Ctrl+C stops the run, and what it started is cleaned up.
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(signals)

	session, err := Prepare(cfg, rmwImpl, manifest,
		StopOnSignal(signals, "interrupted before the recipe started"))
	if err != nil {
		return err
	}
	defer session.Close()

	ui.Header("LAUNCHING RECIPE: " + recipeName)
	ui.Info("All output will be saved to: " + logFile)
	ui.Success("BEGIN RECIPE OUTPUT")
	fmt.Println()

	handle, err := session.StartRecipe(recipeName, io.MultiWriter(os.Stdout, log))
	if err != nil {
		return err
	}
	stopped, err := waitForRecipe(handle, signals)

	fmt.Println()
	switch {
	case err != nil:
		ui.Error(fmt.Sprintf("Recipe '%s' exited with an error: %v", recipeName, err))
	case stopped:
		ui.Success(fmt.Sprintf("Recipe '%s' stopped.", recipeName))
	default:
		ui.Success(fmt.Sprintf("Recipe '%s' finished successfully.", recipeName))
	}
	return err
}

// waitForRecipe waits for the recipe to exit. The first signal asks it to shut
// down, and another kills it.
func waitForRecipe(h *RunHandle, signals <-chan os.Signal) (stopped bool, err error) {
	for {
		select {
		case <-h.Done():
			_, err := h.Wait()
			return stopped, err
		case <-signals:
			if stopped {
				ui.Warn("Killing the recipe.")
				h.Kill()
				continue
			}
			stopped = true
			fmt.Println()
			ui.Info("Stopping the recipe; press Ctrl+C again to force it.")
			h.Interrupt()
		}
	}
}

// killROSProcesses pkills any ROS processes the current user owns.
func killROSProcesses() {
	ui.Info("Killing host ROS processes...")
	for _, proc := range []string{"roslaunch", "roscore", "ros2"} {
		runQuiet("pkill", "-f", proc)
	}
	time.Sleep(time.Second)
	ui.Success("Terminated host ROS processes.")
}

// runQuiet runs a system command, ignoring errors (used for pkill etc.)
func runQuiet(name string, args ...string) {
	execCommand(name, args...).Run()
}
