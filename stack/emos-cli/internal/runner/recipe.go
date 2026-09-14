package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/ui"
)

const (
	emosRoot    = "/emos"
	recipesRoot = "/emos/recipes"
)

type recipeManifest struct {
	ZenohRouterConfig string `json:"zenoh_router_config_file"`
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

// ZenohRMW is the RMW implementation that needs a Zenoh router running.
const ZenohRMW = "rmw_zenoh_cpp"

// ValidRMW reports whether rmw is an RMW implementation a run may ask for.
func ValidRMW(rmw string) bool {
	switch rmw {
	case "rmw_fastrtps_cpp", "rmw_cyclonedds_cpp", ZenohRMW:
		return true
	}
	return false
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
	if rmwImpl != "" && !ValidRMW(rmwImpl) {
		return fmt.Errorf("invalid RMW implementation: %s (allowed: rmw_fastrtps_cpp, rmw_cyclonedds_cpp, rmw_zenoh_cpp)", rmwImpl)
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

	// Setup logging
	os.MkdirAll(config.LogsDir, 0755)
	timestamp := time.Now().Format("20060102_150405")
	logFile := filepath.Join(config.LogsDir, fmt.Sprintf("%s_%s.log", recipeName, timestamp))

	cfg := config.LoadConfig()
	strategy, err := NewStrategy(cfg, rmwImpl)
	if err != nil {
		return err
	}

	ui.Header("EMOS - PRE-RECIPE SETUP")
	ui.Info("Recipe Name: " + recipeName)
	ui.Info("Mode: " + string(cfg.Mode))
	ui.Info("RMW Implementation: " + RMWLabel(rmwImpl))

	// Execute the recipe pipeline
	if err := strategy.PrepareEnvironment(); err != nil {
		return err
	}
	defer strategy.Cleanup()

	if rmwImpl == ZenohRMW {
		router, err := StartZenohRouter(strategy, manifest)
		if err != nil {
			return err
		}
		defer StopZenohRouter(router)
	}

	if err := strategy.LaunchRobotHardware(); err != nil {
		return err
	}

	err = strategy.ExecRecipe(recipeName, logFile)

	fmt.Println()
	if err != nil {
		ui.Error(fmt.Sprintf("Recipe '%s' exited with an error.", recipeName))
	} else {
		ui.Success(fmt.Sprintf("Recipe '%s' finished successfully.", recipeName))
	}
	return err
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
