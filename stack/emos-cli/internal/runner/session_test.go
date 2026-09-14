package runner

import (
	"errors"
	"io"
	"os/exec"
	"slices"
	"testing"
	"time"
)

// fakeStrategy records the calls a session makes. Its commands are listeners,
// so a Zenoh router it is asked for comes up.
type fakeStrategy struct {
	calls      []string
	prepareErr error
	listen     func() *exec.Cmd
}

func (f *fakeStrategy) PrepareEnvironment() error {
	f.calls = append(f.calls, "prepare")
	return f.prepareErr
}
func (f *fakeStrategy) Command(string) *exec.Cmd { return f.listen() }
func (f *fakeStrategy) RecipesDir() string       { return "/recipes" }
func (f *fakeStrategy) LaunchRobotHardware() error {
	f.calls = append(f.calls, "hardware")
	return nil
}
func (f *fakeStrategy) StartRecipe(string, io.Writer) (*RunHandle, error) {
	f.calls = append(f.calls, "start")
	return nil, nil
}
func (f *fakeStrategy) Cleanup() error {
	f.calls = append(f.calls, "cleanup")
	return nil
}

// stages returns a checkpoint that records each stage, and fails at stopAt.
func stages(seen *[]string, stopAt string) func(string) error {
	return func(stage string) error {
		*seen = append(*seen, stage)
		if stage == stopAt {
			return errors.New("stopped")
		}
		return nil
	}
}

func TestSessionRunsTheStagesInOrder(t *testing.T) {
	f := &fakeStrategy{}
	var seen []string
	s, err := prepare(f, "", &recipeManifest{}, stages(&seen, ""))
	if err != nil {
		t.Fatal(err)
	}
	s.StartRecipe("demo", io.Discard)
	s.Close()

	if want := []string{"preparing environment", "launching robot hardware", "starting recipe"}; !slices.Equal(seen, want) {
		t.Errorf("stages = %v, want %v", seen, want)
	}
	if want := []string{"prepare", "hardware", "start", "cleanup"}; !slices.Equal(f.calls, want) {
		t.Errorf("calls = %v, want %v", f.calls, want)
	}
}

func TestSessionStoppedAtACheckpointCleansUp(t *testing.T) {
	f := &fakeStrategy{}
	var seen []string
	if _, err := prepare(f, "", &recipeManifest{}, stages(&seen, "launching robot hardware")); err == nil {
		t.Fatal("a checkpoint error must end the setup")
	}
	if want := []string{"prepare", "cleanup"}; !slices.Equal(f.calls, want) {
		t.Errorf("calls = %v, want %v", f.calls, want)
	}
}

func TestSessionWithAFailedStageCleansUp(t *testing.T) {
	f := &fakeStrategy{prepareErr: errors.New("no ROS")}
	var seen []string
	if _, err := prepare(f, "", &recipeManifest{}, stages(&seen, "")); err == nil {
		t.Fatal("a failed stage must end the setup")
	}
	if want := []string{"prepare", "cleanup"}; !slices.Equal(f.calls, want) {
		t.Errorf("calls = %v, want %v", f.calls, want)
	}
}

func TestSessionStopsTheZenohRouterItStarted(t *testing.T) {
	f := &fakeStrategy{listen: listener(t, freeRouterAddr(t))}
	var seen []string
	s, err := prepare(f, zenohRMW, &recipeManifest{}, stages(&seen, ""))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(seen, "starting zenoh router") || s.router == nil {
		t.Fatalf("stages = %v; the run asked for Zenoh, so a router should be up", seen)
	}
	s.Close()
	select {
	case <-s.router.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("Close left the router running")
	}
}
