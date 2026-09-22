package runner

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/automatika-robotics/emos-cli/internal/config"
	"time"
)

func TestStartRecipeWritesItsOutputAndExitCode(t *testing.T) {
	s := &routerStrategy{command: func() *exec.Cmd {
		return exec.Command("sh", "-c", "echo out; echo err >&2; exit 3")
	}}
	var out bytes.Buffer
	h, err := startRecipe(s, "demo", &out)
	if err != nil {
		t.Fatal(err)
	}
	if code, _ := h.Wait(); code != 3 {
		t.Errorf("exit code = %d, want the recipe's 3", code)
	}
	if got := out.String(); !strings.Contains(got, "out") || !strings.Contains(got, "err") {
		t.Errorf("output = %q, want stdout and stderr", got)
	}
	want := "cd '/emos/recipes/demo' && exec python3 -u '/emos/recipes/demo/recipe.py'"
	if len(s.shells) != 1 || s.shells[0] != want {
		t.Errorf("shell = %q, want %q", s.shells, want)
	}
}

// hostStrategy runs commands in a plain shell, with recipes in dir.
type hostStrategy struct {
	RuntimeStrategy // only Command and RecipesDir are used
	dir             string
}

func (h hostStrategy) Command(shell string) *exec.Cmd { return exec.Command("bash", "-c", shell) }
func (h hostStrategy) RecipesDir() string             { return h.dir }

func TestRecipeRunsFromItsOwnFolder(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 is needed to run a recipe")
	}
	recipes := t.TempDir()
	dir := filepath.Join(recipes, "demo")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "robot.toml"), []byte("config next to the recipe"), 0o644)
	os.WriteFile(filepath.Join(dir, "recipe.py"), []byte("print(open('robot.toml').read())\n"), 0o644)

	var out bytes.Buffer
	h, err := startRecipe(hostStrategy{dir: recipes}, "demo", &out)
	if err != nil {
		t.Fatal(err)
	}
	if code, _ := h.Wait(); code != 0 {
		t.Fatalf("exit code = %d: %s", code, out.String())
	}
	if !strings.Contains(out.String(), "config next to the recipe") {
		t.Errorf("the recipe could not open a file next to it: %s", out.String())
	}
}

func startShell(t *testing.T, script string) *RunHandle {
	t.Helper()
	h, err := StartProcess(exec.Command("sh", "-c", script))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Kill)
	time.Sleep(200 * time.Millisecond) // let the trap install
	return h
}

func TestCancelInterruptsTheRecipeAndWaitsForIt(t *testing.T) {
	h := startShell(t, `trap "exit 7" INT; while :; do sleep 0.1; done`)
	if err := h.Cancel(5 * time.Second); err != nil {
		t.Fatal(err)
	}
	if code, _ := h.Wait(); code != 7 {
		t.Errorf("exit code = %d; want 7, from the recipe's own shutdown on SIGINT", code)
	}
}

func TestWaitForRecipeInterruptsOnTheFirstSignal(t *testing.T) {
	h := startShell(t, `trap "exit 0" INT; while :; do sleep 0.1; done`)
	signals := make(chan os.Signal, 2)
	signals <- syscall.SIGINT

	stopped, err := waitForRecipe(h, signals)
	if !stopped || err != nil {
		t.Errorf("waitForRecipe = %v, %v; want a clean stop", stopped, err)
	}
}

func TestWaitForRecipeKillsOnTheSecondSignal(t *testing.T) {
	h := startShell(t, `trap "" INT; while :; do sleep 0.1; done`)
	signals := make(chan os.Signal, 2)
	signals <- syscall.SIGINT
	signals <- syscall.SIGINT

	done := make(chan error, 1)
	go func() {
		_, err := waitForRecipe(h, signals)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Error("a killed recipe must report an error")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the recipe ignoring the interrupt was not killed")
	}
}

func TestWrongRobotNamesTheRobotARecipeWasMadeFor(t *testing.T) {
	lite3 := &config.EMOSConfig{Plugin: &config.PluginInfo{Slug: "emos-plugin-lite3"}}
	generic, forLite3, forM20 := &recipeManifest{}, &recipeManifest{}, &recipeManifest{}
	forLite3.Variant.Robot = "emos-plugin-lite3"
	forM20.Variant.Robot = "emos-plugin-m20"

	if got := generic.WrongRobot(lite3); got != "" {
		t.Errorf("a generic recipe is for every robot, got %q", got)
	}
	if got := forLite3.WrongRobot(lite3); got != "" {
		t.Errorf("the recipe's robot is the installed one, got %q", got)
	}
	if got := forM20.WrongRobot(lite3); got != "emos-plugin-m20" {
		t.Errorf("a recipe for another robot = %q, want its plugin", got)
	}
	// No robot plugin installed at all
	if got := forM20.WrongRobot(&config.EMOSConfig{}); got != "emos-plugin-m20" {
		t.Errorf("no robot installed = %q, want the recipe's plugin", got)
	}
}
