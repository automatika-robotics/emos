package runner

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
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
	if want := "exec python3 -u /emos/recipes/demo/recipe.py"; len(s.shells) != 1 || s.shells[0] != want {
		t.Errorf("shell = %q, want %q", s.shells, want)
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
