package runner

import (
	"errors"
	"io"
	"os"

	"github.com/automatika-robotics/emos-cli/internal/config"
)

// Session is the environment a recipe run needs, and what the run started to
// provide it.
type Session struct {
	strategy   RuntimeStrategy
	router     *RunHandle
	checkpoint func(stage string) error
}

// Prepare sets up a run for the install mode in cfg. checkpoint is called
// before each stage; an error from it or from a stage ends the setup, and
// whatever had started is cleaned up. manifest is nil for a run that is not a
// recipe's.
func Prepare(cfg *config.EMOSConfig, rmw string, manifest *recipeManifest, checkpoint func(stage string) error) (*Session, error) {
	strategy, err := newStrategy(cfg, rmw)
	if err != nil {
		return nil, err
	}
	return prepare(strategy, rmw, manifest, checkpoint)
}

// StopOnSignal returns a checkpoint that fails with reason once a signal has
// arrived on signals.
func StopOnSignal(signals <-chan os.Signal, reason string) func(string) error {
	return func(string) error {
		select {
		case <-signals:
			return errors.New(reason)
		default:
			return nil
		}
	}
}

func prepare(strategy RuntimeStrategy, rmw string, manifest *recipeManifest, checkpoint func(string) error) (*Session, error) {
	if manifest == nil {
		manifest = &recipeManifest{}
	}
	s := &Session{strategy: strategy, checkpoint: checkpoint}
	if err := s.setUp(rmw, manifest); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

func (s *Session) setUp(rmw string, manifest *recipeManifest) error {
	if err := s.checkpoint("preparing environment"); err != nil {
		return err
	}
	if err := s.strategy.PrepareEnvironment(); err != nil {
		return err
	}
	if rmw == zenohRMW {
		if err := s.checkpoint("starting zenoh router"); err != nil {
			return err
		}
		router, err := startZenohRouter(s.strategy, manifest)
		if err != nil {
			return err
		}
		s.router = router
	}
	return nil
}

// StartRecipe starts the recipe, writing its output to out.
func (s *Session) StartRecipe(recipeName string, out io.Writer) (*RunHandle, error) {
	if err := s.checkpoint("starting recipe"); err != nil {
		return nil, err
	}
	return s.strategy.StartRecipe(recipeName, out)
}

// Start runs shell in the run's environment, in its own process group, writing
// its output to out.
func (s *Session) Start(stage, shell string, out io.Writer) (*RunHandle, error) {
	if err := s.checkpoint(stage); err != nil {
		return nil, err
	}
	cmd := s.strategy.Command("exec " + shell)
	cmd.Stdout = out
	cmd.Stderr = out
	return StartProcess(cmd)
}

// Close stops the Zenoh router the run started, then cleans up the strategy.
func (s *Session) Close() {
	stopZenohRouter(s.router)
	_ = s.strategy.Cleanup()
}
