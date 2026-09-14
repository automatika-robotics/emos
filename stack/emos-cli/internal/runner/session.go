package runner

import (
	"io"

	"github.com/automatika-robotics/emos-cli/internal/config"
)

// Session is the environment a recipe run needs, and what the run started to
// provide it.
type Session struct {
	strategy   RuntimeStrategy
	router     *RunHandle
	checkpoint func(stage string) error
}

// Prepare sets up a recipe run for the install mode in cfg. checkpoint is
// called before each stage; an error from it or from a stage ends the setup,
// and whatever had started is cleaned up.
func Prepare(cfg *config.EMOSConfig, rmw string, manifest *recipeManifest, checkpoint func(stage string) error) (*Session, error) {
	strategy, err := newStrategy(cfg, rmw)
	if err != nil {
		return nil, err
	}
	return prepare(strategy, rmw, manifest, checkpoint)
}

func prepare(strategy RuntimeStrategy, rmw string, manifest *recipeManifest, checkpoint func(string) error) (*Session, error) {
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
	if err := s.checkpoint("launching robot hardware"); err != nil {
		return err
	}
	return s.strategy.LaunchRobotHardware()
}

// StartRecipe starts the recipe, writing its output to out.
func (s *Session) StartRecipe(recipeName string, out io.Writer) (*RunHandle, error) {
	if err := s.checkpoint("starting recipe"); err != nil {
		return nil, err
	}
	return s.strategy.StartRecipe(recipeName, out)
}

// Close stops the Zenoh router the run started, then cleans up the strategy.
func (s *Session) Close() {
	stopZenohRouter(s.router)
	_ = s.strategy.Cleanup()
}
