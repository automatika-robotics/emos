package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/runner"
)

type startRunBody struct {
	Recipe          string `json:"recipe"`
	RMW             string `json:"rmw,omitempty"`
	SkipSensorCheck bool   `json:"skip_sensor_check,omitempty"`
}

func (s *Server) handleRunsList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.runtime.List())
}

func (s *Server) handleRunGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	run := s.runtime.Get(id)
	if run == nil {
		writeErr(w, http.StatusNotFound, codeNotFound, "run not found")
		return
	}
	writeJSON(w, http.StatusOK, run)
}

// handleRunsStart kicks off a recipe asynchronously. Validates inputs
// synchronously, registers a Run in `preparing` state, and returns 202 with
// the Run record. All slow pre-flight runs in a goroutine.
func (s *Server) handleRunsStart(w http.ResponseWriter, r *http.Request) {
	var body startRunBody
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, codeBadRequest, err.Error())
		return
	}
	if !validRecipeName(body.Recipe) {
		writeErr(w, http.StatusBadRequest, codeBadRequest, "invalid recipe name")
		return
	}
	if body.RMW == "" {
		body.RMW = "rmw_zenoh_cpp"
	}
	if !validRMW(body.RMW) {
		writeErr(w, http.StatusBadRequest, codeBadRequest, "invalid rmw implementation")
		return
	}
	if s.cfg == nil {
		writeErr(w, http.StatusFailedDependency, codeBadRequest,
			"no EMOS installation found — run `emos install` first")
		return
	}

	recipeDir := filepath.Join(config.RecipesDir, body.Recipe)
	if _, err := os.Stat(filepath.Join(recipeDir, "recipe.py")); err != nil {
		writeErr(w, http.StatusNotFound, codeNotFound, "recipe not installed")
		return
	}

	logFile := runner.LogFilePath(body.Recipe)
	run := &Run{
		ID:        newID(),
		Recipe:    body.Recipe,
		Status:    RunStatusPreparing,
		StartedAt: time.Now(), // overwritten when the recipe process actually starts
		LogPath:   logFile,
		RMW:       body.RMW,
		cancelCh:  make(chan struct{}),
	}
	if err := s.runtime.TryLock(run); err != nil {
		writeErr(w, http.StatusConflict, codeAlreadyRunning,
			"a recipe is already running; stop it before starting a new one")
		return
	}

	go s.runRecipeAsync(run, recipeDir, body)

	writeJSON(w, http.StatusAccepted, run)
}

// runRecipeAsync owns the full lifecycle of a single run from preparing
// through process exit. Errors are written to the run's log file and the
// Run record's Error field. Cancellation during preparing flips the run to
// canceled at the next checkpoint.
func (s *Server) runRecipeAsync(run *Run, recipeDir string, body startRunBody) {
	logf, err := openSetupLog(run.LogPath)
	if err != nil {
		s.runtime.FailPreflight(run, fmt.Errorf("open log file: %w", err))
		return
	}
	defer func() {
		// Close only if we never attached a process; the recipe process
		// inherits the file descriptor when started by StartRecipe.
		if run.handle == nil {
			logf.Close()
		}
	}()

	step := func(format string, a ...any) {
		fmt.Fprintf(logf, "[setup] "+format+"\n", a...)
	}
	check := func() bool {
		select {
		case <-run.CancelCh():
			step("cancelled by user")
			return true
		default:
			return false
		}
	}

	step("preparing run: recipe=%s rmw=%s", run.Recipe, run.RMW)

	manifest := runner.LoadManifest(filepath.Join(recipeDir, "manifest.json"))

	strategy, err := s.buildStrategy()
	if err != nil {
		step("ERROR: %s", err)
		s.runtime.FailPreflight(run, err)
		return
	}

	// Hand the strategy a fresh run for cleanup-after-exit.
	deferStrategyCleanup := func() {
		go func() {
			if run.handle != nil {
				<-run.handle.Done()
			}
			_ = strategy.Cleanup()
		}()
	}

	step("preparing environment")
	if err := strategy.PrepareEnvironment(); err != nil {
		step("ERROR: environment preparation failed: %s", err)
		s.runtime.FailPreflight(run, err)
		return
	}
	if check() {
		s.runtime.CancelPreflight(run)
		return
	}

	step("setting RMW implementation: %s", body.RMW)
	if err := strategy.SetRMWImpl(body.RMW); err != nil {
		step("ERROR: %s", err)
		s.runtime.FailPreflight(run, err)
		return
	}

	if body.RMW == "rmw_zenoh_cpp" {
		step("starting zenoh router")
		if err := strategy.ConfigureZenoh(run.Recipe, manifest); err != nil {
			step("ERROR: %s", err)
			s.runtime.FailPreflight(run, err)
			return
		}
	}
	if check() {
		s.runtime.CancelPreflight(run)
		return
	}

	step("launching robot hardware (if configured)")
	if err := strategy.LaunchRobotHardware(); err != nil {
		step("ERROR: %s", err)
		s.runtime.FailPreflight(run, err)
		return
	}

	if !body.SkipSensorCheck {
		step("verifying required sensor topics")
		topics, _ := runner.ExtractTopics(filepath.Join(recipeDir, "recipe.py"))
		sensors := runner.SensorTopics(topics)
		distro := s.cfg.ROSDistro
		if distro == "" {
			distro = "jazzy"
		}
		if err := strategy.VerifySensorTopics(sensors, distro); err != nil {
			step("ERROR: %s", err)
			_ = strategy.Cleanup()
			s.runtime.FailPreflight(run, err)
			return
		}
	} else {
		step("sensor verification skipped (--skip-sensor-check)")
	}
	if check() {
		s.runtime.CancelPreflight(run)
		_ = strategy.Cleanup()
		return
	}

	step("starting recipe process")
	logf.Close() // strategy will reopen for append; avoid two writers
	handle, err := strategy.StartRecipe(run.Recipe, manifest, run.LogPath)
	if err != nil {
		s.runtime.FailPreflight(run, err)
		_ = strategy.Cleanup()
		return
	}

	s.runtime.AttachHandle(run, handle)
	deferStrategyCleanup()
}

// openSetupLog opens (and creates) the run log file in append mode. Used by
// the pre-flight goroutine to stream "[setup] ..." progress before the
// recipe process takes over the file.
func openSetupLog(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
}

func (s *Server) handleRunCancel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	cur := s.runtime.Current()
	if cur == nil || cur.ID != id {
		writeErr(w, http.StatusNotFound, codeNotFound, "run not active")
		return
	}
	if err := s.runtime.Cancel(id); err != nil {
		writeErr(w, http.StatusInternalServerError, codeInternal, err.Error())
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// handleRunLogs streams the run's log file as SSE.
//
// Behaviour:
//   - replays the file from byte 0
//   - for active runs, switches to live tail
//   - for finished runs, ends the stream after replay
//   - 15s heartbeat keeps proxies/load balancers happy
func (s *Server) handleRunLogs(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	run := s.runtime.Get(id)
	if run == nil {
		writeErr(w, http.StatusNotFound, codeNotFound, "run not found")
		return
	}
	stream := NewSSEStream(w)
	if stream == nil {
		writeErr(w, http.StatusInternalServerError, codeInternal, "streaming unsupported")
		return
	}

	// `done` closes when the recipe process exits or for a run still in
	// preparing, when the pre-flight goroutine attaches a handle and that
	// handle's process eventually finishes. We poll briefly for the handle
	// in that case so log streaming works seamlessly across the
	// preparing->running transition.
	done := make(chan struct{})
	if cur := s.runtime.Current(); cur != nil && cur.ID == id {
		go func() {
			for cur.handle == nil {
				if cur.Status != RunStatusPreparing {
					close(done)
					return
				}
				time.Sleep(150 * time.Millisecond)
			}
			<-cur.handle.Done()
			close(done)
		}()
	} else {
		close(done)
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	lines := make(chan string, 64)
	tailErr := make(chan error, 1)
	go func() {
		tailErr <- tailLog(ctx, run.LogPath, done, lines)
		close(lines)
	}()

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			if err := stream.Heartbeat(); err != nil {
				return
			}
		case line, ok := <-lines:
			if !ok {
				// Send a final status event so the client can close cleanly.
				snapshot := s.runtime.Get(id)
				_ = stream.SendNamed("end", snapshot)
				return
			}
			if err := stream.SendRaw("log", line); err != nil {
				return
			}
		}
	}
}

func validRMW(rmw string) bool {
	switch rmw {
	case "rmw_fastrtps_cpp", "rmw_cyclonedds_cpp", "rmw_zenoh_cpp":
		return true
	}
	return false
}

// --- strategy factory (mirrors runner.RunRecipe's switch) ---

func (s *Server) buildStrategy() (runner.RuntimeStrategy, error) {
	if s.cfg == nil {
		return nil, errors.New("no install config")
	}
	switch s.cfg.Mode {
	case config.ModeOSSContainer:
		return runner.NewContainerStrategy(false), nil
	case config.ModeLicensed:
		return runner.NewContainerStrategy(true), nil
	case config.ModeNative:
		return runner.NewNativeStrategy(), nil
	case config.ModePixi:
		return runner.NewPixiStrategy(s.cfg.PixiProjectDir), nil
	}
	return nil, fmt.Errorf("unknown install mode: %s", s.cfg.Mode)
}
