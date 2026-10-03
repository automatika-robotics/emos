package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/automatika-robotics/emos-cli/internal/config"
)

// pixiInstall points the server at a pixi install whose pixi just runs the
// command it is given, with the recipe "demo" holding source.
func pixiInstall(t *testing.T, s *Server, source string) {
	t.Helper()
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 is needed to run a recipe")
	}
	bin := t.TempDir()
	// pixi run --manifest-path <toml> <command...>
	must(t, os.WriteFile(filepath.Join(bin, "pixi"), []byte("#!/bin/sh\nshift 3\nexec \"$@\"\n"), 0o755))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	project := t.TempDir()
	must(t, os.MkdirAll(filepath.Join(project, "install"), 0o755))
	must(t, os.WriteFile(filepath.Join(project, "pixi.toml"), nil, 0o644))
	must(t, os.WriteFile(filepath.Join(project, "install", "setup.sh"), nil, 0o644))
	s.cfg = &config.EMOSConfig{Mode: config.ModePixi, PixiProjectDir: project}

	dir := filepath.Join(config.RecipesDir, "demo")
	must(t, os.MkdirAll(dir, 0o755))
	must(t, os.WriteFile(filepath.Join(dir, "recipe.py"), []byte(source), 0o644))
}

func startDemoRun(t *testing.T, s *Server) Run {
	t.Helper()
	rec := httpServe(t, s, jsonRequest(t, http.MethodPost, "/api/v1/runs", map[string]string{"recipe": "demo"}))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("start run: status %d: %s", rec.Code, rec.Body.String())
	}
	var run Run
	jsonBody(t, rec, &run)
	return run
}

// waitForLog waits until the run's log holds want, and returns the log.
func waitForLog(t *testing.T, path, want string) string {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for {
		data, _ := os.ReadFile(path)
		if strings.Contains(string(data), want) {
			return string(data)
		}
		if time.Now().After(deadline) {
			t.Fatalf("log never showed %q:\n%s", want, data)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// waitForRunEnd waits for the run and everything it started to be done.
func waitForRunEnd(t *testing.T, s *Server, id string) *Run {
	t.Helper()
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	deadline := time.After(20 * time.Second)
	select {
	case <-done:
	case <-deadline:
		t.Fatal("the run did not finish")
	}
	// The exit is recorded by the runtime's own watcher.
	for s.runtime.Current() != nil {
		select {
		case <-deadline:
			t.Fatal("the run's exit was never recorded")
		case <-time.After(20 * time.Millisecond):
		}
	}
	return s.runtime.Get(id)
}

func TestDashboardRunRecordsTheRecipesOutputAndExit(t *testing.T) {
	s := newTestServer(t, true)
	pixiInstall(t, s, "import os\nprint('recipe ran in', os.environ['SUGARCOAT_UI_DATA_DIR'], flush=True)\nraise SystemExit(3)\n")

	run := startDemoRun(t, s)
	got := waitForRunEnd(t, s, run.ID)
	if got.Status != RunStatusFailed || got.ExitCode != 3 {
		t.Errorf("run = %s with exit %d, want failed with the recipe's 3", got.Status, got.ExitCode)
	}
	log := waitForLog(t, run.LogPath, "recipe ran in "+config.UISecurityDir)
	for _, stage := range []string{"preparing environment", "starting recipe"} {
		if !strings.Contains(log, "[setup] "+stage) {
			t.Errorf("log is missing the %q stage:\n%s", stage, log)
		}
	}
}

func TestDashboardStopInterruptsTheRecipe(t *testing.T) {
	s := newTestServer(t, true)
	pixiInstall(t, s, "import signal, sys, time\n"+
		"def stop(*_):\n    print('recipe stopping', flush=True)\n    sys.exit(0)\n"+
		"signal.signal(signal.SIGINT, stop)\n"+
		"print('recipe running', flush=True)\n"+
		"while True:\n    time.sleep(0.1)\n")

	run := startDemoRun(t, s)
	waitForLog(t, run.LogPath, "recipe running")
	rec := httpServe(t, s, httptest.NewRequest(http.MethodDelete, "/api/v1/runs/"+run.ID, nil))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("stop run: status %d", rec.Code)
	}
	if got := waitForRunEnd(t, s, run.ID); got.Status != RunStatusCanceled {
		t.Errorf("run = %s, want canceled", got.Status)
	}
	waitForLog(t, run.LogPath, "recipe stopping")
}

func TestHandleRunsListEmpty(t *testing.T) {
	s := newTestServer(t, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/runs", nil)
	rec := httpServe(t, s, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var runs []Run
	jsonBody(t, rec, &runs)
	if len(runs) != 0 {
		t.Fatalf("List = %d entries, want 0", len(runs))
	}
}

func TestHandleRunGetMissing(t *testing.T) {
	s := newTestServer(t, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/runs/nope", nil)
	rec := httpServe(t, s, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestHandleRunsStartBadJSON(t *testing.T) {
	s := newTestServer(t, true)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/runs", nil)
	req.Header.Set("Content-Type", "application/json")
	rec := httpServe(t, s, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHandleRunsStartInvalidRecipe(t *testing.T) {
	s := newTestServer(t, true)
	req := jsonRequest(t, http.MethodPost, "/api/v1/runs", map[string]string{
		"recipe": "../escape",
	})
	rec := httpServe(t, s, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHandleRunsStartInvalidRMW(t *testing.T) {
	s := newTestServer(t, true)
	req := jsonRequest(t, http.MethodPost, "/api/v1/runs", map[string]string{
		"recipe": "demo",
		"rmw":    "rmw_unknown",
	})
	rec := httpServe(t, s, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHandleRunsStartNoInstall(t *testing.T) {
	s := newTestServer(t, true)
	// s.cfg is nil by default in newTestServer.
	req := jsonRequest(t, http.MethodPost, "/api/v1/runs", map[string]string{
		"recipe": "demo",
	})
	rec := httpServe(t, s, req)
	// FailedDependency is the dedicated status for "you need to install first".
	if rec.Code != http.StatusFailedDependency {
		t.Fatalf("status = %d, want 424 (FailedDependency)", rec.Code)
	}
}

func TestHandleRunsStartRecipeNotInstalled(t *testing.T) {
	s := newTestServer(t, true)
	s.cfg = &config.EMOSConfig{Mode: config.ModeNative, ROSDistro: "jazzy"}

	req := jsonRequest(t, http.MethodPost, "/api/v1/runs", map[string]string{
		"recipe": "missing",
	})
	rec := httpServe(t, s, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestHandleRunsStartConflictWhenAnotherRunning(t *testing.T) {
	s := newTestServer(t, true)
	s.cfg = &config.EMOSConfig{Mode: config.ModeNative, ROSDistro: "jazzy"}

	// Plant a recipe so the validation passes through to TryLock.
	dir := filepath.Join(config.RecipesDir, "demo")
	must(t, os.MkdirAll(dir, 0o755))
	must(t, os.WriteFile(filepath.Join(dir, "recipe.py"), []byte("# noop"), 0o644))

	// Pre-occupy the runtime slot.
	occupier := &Run{ID: "earlier", Status: RunStatusPreparing}
	if err := s.runtime.TryLock(occupier); err != nil {
		t.Fatalf("TryLock occupier: %v", err)
	}

	req := jsonRequest(t, http.MethodPost, "/api/v1/runs", map[string]string{
		"recipe": "demo",
	})
	rec := httpServe(t, s, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
	var apiErr APIError
	jsonBody(t, rec, &apiErr)
	if apiErr.Code != codeAlreadyRunning {
		t.Fatalf("code = %q, want %q", apiErr.Code, codeAlreadyRunning)
	}
}

func TestHandleRunCancelMissing(t *testing.T) {
	s := newTestServer(t, true)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/runs/nope", nil)
	rec := httpServe(t, s, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestHandleRunCancelPreparing(t *testing.T) {
	s := newTestServer(t, true)
	r := &Run{ID: "abc", Status: RunStatusPreparing}
	if err := s.runtime.TryLock(r); err != nil {
		t.Fatalf("TryLock: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/runs/abc", nil)
	rec := httpServe(t, s, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", rec.Code)
	}
	if r.Status != RunStatusCanceled {
		t.Fatalf("Status = %q, want canceled", r.Status)
	}
}
