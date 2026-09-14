package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/automatika-robotics/emos-cli/internal/plugin"
	"github.com/automatika-robotics/emos-cli/internal/runner"
)

type startRunBody struct {
	Recipe string `json:"recipe"`
	RMW    string `json:"rmw,omitempty"`
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
	recipeDir, err := safeRecipeDir(body.Recipe)
	if err != nil {
		writeErr(w, http.StatusBadRequest, codeBadRequest, "invalid recipe name")
		return
	}
	if body.RMW != "" && !runner.ValidRMW(body.RMW) {
		writeErr(w, http.StatusBadRequest, codeBadRequest, "invalid rmw implementation")
		return
	}
	if !s.cfg.IsInstalled() {
		writeErr(w, http.StatusFailedDependency, codeBadRequest,
			"no EMOS installation found — run `emos install` first")
		return
	}

	if _, err := os.Stat(filepath.Join(recipeDir, "recipe.py")); err != nil {
		writeErr(w, http.StatusNotFound, codeNotFound, "recipe not installed")
		return
	}

	logFile := runner.LogFilePath(body.Recipe)
	run := &Run{
		ID:             newID(),
		Recipe:         body.Recipe,
		Status:         RunStatusPreparing,
		StartedAt:      time.Now(), // overwritten when the recipe process actually starts
		LogPath:        logFile,
		RMW:            body.RMW,
		cancelCh:       make(chan struct{}),
		handleAttached: make(chan struct{}),
	}
	// Claimed under workMu, the same as a plugin job, so a recipe and a plugin
	// change can never both start.
	s.workMu.Lock()
	if plugin.Busy() {
		s.workMu.Unlock()
		writeErr(w, http.StatusConflict, codeConflict,
			"plugins are being installed, updated or removed; try again once that finishes")
		return
	}
	err = s.runtime.TryLock(run)
	s.workMu.Unlock()
	if err != nil {
		writeErr(w, http.StatusConflict, codeAlreadyRunning,
			"a recipe is already running; stop it before starting a new one")
		return
	}

	s.goTracked(func() { s.runRecipeAsync(run, recipeDir, body) })

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
	attached := false // a recipe process was started and attached to the run
	defer func() {
		// Close only if we never attached a process; the recipe process
		// inherits the file descriptor when started by StartRecipe.
		if !attached {
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

	step("preparing run: recipe=%s", run.Recipe)

	manifest := runner.LoadManifest(filepath.Join(recipeDir, "manifest.json"))

	strategy, err := runner.NewStrategy(s.cfg, body.RMW)
	if err != nil {
		step("ERROR: %s", err)
		s.runtime.FailPreflight(run, err)
		return
	}

	// Whatever the run starts is cleaned up on every way out of here
	var router *runner.RunHandle
	defer func() {
		if !attached {
			runner.StopZenohRouter(router)
			_ = strategy.Cleanup()
		}
	}()

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

	step("RMW implementation: %s", runner.RMWLabel(body.RMW))
	if body.RMW == runner.ZenohRMW {
		step("starting zenoh router")
		if router, err = runner.StartZenohRouter(strategy, manifest); err != nil {
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
	if check() {
		s.runtime.CancelPreflight(run)
		return
	}

	step("starting recipe process")
	logf.Close() // strategy will reopen for append; avoid two writers
	handle, err := strategy.StartRecipe(run.Recipe, run.LogPath)
	if err != nil {
		s.runtime.FailPreflight(run, err)
		return
	}

	// A cancel can land after the last checkpoint, while the process starts;
	// AttachHandle then stops the process and the deferred cleanup runs.
	if !s.runtime.AttachHandle(run, handle) {
		return
	}
	attached = true
	s.goTracked(func() {
		<-handle.Done()
		runner.StopZenohRouter(router)
		_ = strategy.Cleanup()
	})
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

	// `done` closes when the recipe process exits, or for a run still in
	// preparing, when the pre-flight goroutine attaches a handle and that
	// handle's process eventually finishes.
	done := make(chan struct{})
	if cur := s.runtime.Current(); cur != nil && cur.ID == id {
		go func() {
			defer close(done)
			select {
			case <-cur.HandleAttached():
				if h := cur.Handle(); h != nil {
					<-h.Done()
				}
			case <-cur.CancelCh():
				// Cancelled before a process ever started.
				return
			}
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
