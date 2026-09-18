package runner

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
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

func TestSessionWithoutAManifestStillGetsItsZenohRouter(t *testing.T) {
	// A run that is not a recipe's, such as a mapping session, has none.
	f := &fakeStrategy{listen: listener(t, freeRouterAddr(t))}
	var seen []string
	s, err := prepare(f, zenohRMW, nil, stages(&seen, ""))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if s.router == nil {
		t.Fatalf("stages = %v; the run asked for Zenoh, so a router should be up", seen)
	}
}

func TestStopOnSignalFailsOnceASignalHasArrived(t *testing.T) {
	signals := make(chan os.Signal, 1)
	checkpoint := StopOnSignal(signals, "interrupted before mapping started")
	if err := checkpoint("preparing environment"); err != nil {
		t.Fatalf("no signal yet, got %v", err)
	}
	signals <- os.Interrupt
	if err := checkpoint("starting mapping"); err == nil || err.Error() != "interrupted before mapping started" {
		t.Errorf("want the reason, got %v", err)
	}
}

func TestSessionStartRunsInTheEnvironmentAndHonoursTheCheckpoint(t *testing.T) {
	var shells []string
	f := &shellStrategy{shells: &shells}
	var seen []string
	s, err := prepare(f, "", &recipeManifest{}, stages(&seen, ""))
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	h, err := s.Start("starting mapping", "echo mapped", &out)
	if err != nil {
		t.Fatal(err)
	}
	if code, _ := h.Wait(); code != 0 || strings.TrimSpace(out.String()) != "mapped" {
		t.Errorf("exit %d, output %q", code, out.String())
	}
	if want := []string{"exec echo mapped"}; !slices.Equal(shells, want) {
		t.Errorf("shells = %v, want %v", shells, want)
	}
	if seen[len(seen)-1] != "starting mapping" {
		t.Errorf("stages = %v", seen)
	}

	stopped, _ := prepare(f, "", &recipeManifest{}, stages(&seen, "starting mapping"))
	if _, err := stopped.Start("starting mapping", "echo never", io.Discard); err == nil {
		t.Error("a failing checkpoint should stop the start")
	}
	if len(shells) != 1 {
		t.Errorf("nothing should have run, shells = %v", shells)
	}
}

// shellStrategy runs the shell it is given on the host, and records it.
type shellStrategy struct {
	fakeStrategy
	shells *[]string
}

func (f *shellStrategy) Command(shell string) *exec.Cmd {
	*f.shells = append(*f.shells, shell)
	return exec.Command("bash", "-c", shell)
}
