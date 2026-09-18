package mapping

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/runner"
)

// fakeProcess is a mapping session under the test's control.
type fakeProcess struct {
	done        chan struct{}
	code        int
	onInterrupt func(*fakeProcess)
	killed      bool
}

func newFakeProcess() *fakeProcess { return &fakeProcess{done: make(chan struct{})} }

func (p *fakeProcess) exit(code int) {
	p.code = code
	close(p.done)
}
func (p *fakeProcess) Interrupt() {
	if p.onInterrupt != nil {
		p.onInterrupt(p)
	}
}
func (p *fakeProcess) Kill()                 { p.killed = true }
func (p *fakeProcess) Done() <-chan struct{} { return p.done }
func (p *fakeProcess) Wait() (int, error)    { <-p.done; return p.code, nil }

func nativeDecl(store string) *Declaration {
	return &Declaration{Kind: KindNative, Native: &Native{Store: store, ActiveLink: "active"}}
}

// announcing returns a Starter whose session announces dir, as emos_mapping's
// does, after creating it.
func announcing(t *testing.T, proc *fakeProcess, dir string, shell *string) Starter {
	return func(sh string, out io.Writer) (Process, error) {
		*shell = sh
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		io.WriteString(out, "[INFO] loading the plugin\nMap direct")
		io.WriteString(out, "ory: "+dir+"\n[INFO] glim started\n")
		return proc, nil
	}
}

func TestNativeSessionSavesTheMap(t *testing.T) {
	store := t.TempDir()
	dir := filepath.Join(store, "office-20260918-101500")
	proc := newFakeProcess()
	proc.onInterrupt = func(p *fakeProcess) {
		os.WriteFile(filepath.Join(dir, "occ_grid.yaml"), []byte("image: occ_grid.pgm\n"), 0o644)
		p.exit(0)
	}
	var shell string
	var log bytes.Buffer

	session, err := nativeDecl(store).StartNative(context.Background(),
		"lite3_plugin:Lite3Plugin", "office", announcing(t, proc, dir, &shell), &log)
	if err != nil {
		t.Fatalf("StartNative: %v", err)
	}
	if want := "python3 -u -m emos_mapping.session --plugin lite3_plugin:Lite3Plugin --name office"; shell != want {
		t.Errorf("shell = %q, want %q", shell, want)
	}
	if session.Dir != dir {
		t.Errorf("Dir = %q, want %q", session.Dir, dir)
	}
	if !strings.Contains(log.String(), "glim started") {
		t.Errorf("the session's output should pass through to the log, got %q", log.String())
	}

	built, err := session.Stop(context.Background())
	if err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if built.Name != "office-20260918-101500" || built.Grid != filepath.Join(dir, "occ_grid.yaml") {
		t.Errorf("built = %+v", built)
	}
}

func TestNativeSessionThatEndsBeforeAnnouncing(t *testing.T) {
	proc := newFakeProcess()
	proc.exit(2)
	start := func(string, io.Writer) (Process, error) { return proc, nil }
	_, err := nativeDecl(t.TempDir()).StartNative(context.Background(), "p:P", "office", start, io.Discard)
	var exited *ErrSessionExited
	if !errors.As(err, &exited) || exited.Code != 2 {
		t.Errorf("want ErrSessionExited{2}, got %v", err)
	}
}

func TestNativeSessionWithoutAMap(t *testing.T) {
	store := t.TempDir()
	dir := filepath.Join(store, "office-20260918-101500")
	for code, want := range map[int]error{1: ErrNoMapBuilt, 3: &ErrSessionExited{Code: 3}} {
		proc := newFakeProcess()
		proc.onInterrupt = func(p *fakeProcess) { p.exit(code) }
		var shell string
		session, err := nativeDecl(store).StartNative(context.Background(),
			"p:P", "office", announcing(t, proc, dir, &shell), io.Discard)
		if err != nil {
			t.Fatalf("StartNative: %v", err)
		}
		_, err = session.Stop(context.Background())
		var exited *ErrSessionExited
		switch {
		case want == ErrNoMapBuilt && !errors.Is(err, ErrNoMapBuilt):
			t.Errorf("exit %d: want ErrNoMapBuilt, got %v", code, err)
		case want != ErrNoMapBuilt && (!errors.As(err, &exited) || exited.Code != code):
			t.Errorf("exit %d: want ErrSessionExited, got %v", code, err)
		}
	}
}

func TestNativeSessionThatDiesIsStillJudgedByTheStore(t *testing.T) {
	store := t.TempDir()
	dir := filepath.Join(store, "office-20260918-101500")
	proc := newFakeProcess()
	var shell string
	session, err := nativeDecl(store).StartNative(context.Background(),
		"p:P", "office", announcing(t, proc, dir, &shell), io.Discard)
	if err != nil {
		t.Fatalf("StartNative: %v", err)
	}
	os.WriteFile(filepath.Join(dir, "occ_grid.yaml"), []byte("image: occ_grid.pgm\n"), 0o644)
	proc.exit(-1) // ended by a signal, after saving
	<-session.Done()
	if built, err := session.Result(); err != nil || built.Grid == "" {
		t.Errorf("want the saved map, got %+v, %v", built, err)
	}
}

func TestStoppingANativeSessionCanBeGivenUpOn(t *testing.T) {
	store := t.TempDir()
	proc := newFakeProcess() // ignores the interrupt
	var shell string
	session, err := nativeDecl(store).StartNative(context.Background(),
		"p:P", "office", announcing(t, proc, filepath.Join(store, "office-1"), &shell), io.Discard)
	if err != nil {
		t.Fatalf("StartNative: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := session.Stop(ctx); !errors.Is(err, ErrStopInterrupted) || !proc.killed {
		t.Errorf("want ErrStopInterrupted and a kill, got %v (killed=%v)", err, proc.killed)
	}
}

func TestStartingANativeSessionCanBeGivenUpOn(t *testing.T) {
	proc := newFakeProcess() // never announces
	start := func(string, io.Writer) (Process, error) { return proc, nil }
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := nativeDecl(t.TempDir()).StartNative(ctx, "p:P", "office", start, io.Discard)
	if !errors.Is(err, context.Canceled) || !proc.killed {
		t.Errorf("want context.Canceled and a kill, got %v (killed=%v)", err, proc.killed)
	}
}

func TestStartNativeRefusesWhatCannotGoIntoAShellCommand(t *testing.T) {
	started := false
	start := func(string, io.Writer) (Process, error) { started = true; return newFakeProcess(), nil }
	decl := nativeDecl(t.TempDir())
	for _, c := range []struct{ entry, name string }{
		{"lite3_plugin:Lite3Plugin", "office; rm -rf ~"},
		{"lite3_plugin:Lite3Plugin", "-office"},
		{"lite3_plugin:Lite3Plugin; ls", "office"},
		{"lite3_plugin", "office"},
		{"$(id):Plugin", "office"},
	} {
		if _, err := decl.StartNative(context.Background(), c.entry, c.name, start, io.Discard); err == nil {
			t.Errorf("entry %q, name %q: want an error", c.entry, c.name)
		}
	}
	if started {
		t.Error("nothing should have been started")
	}
	for _, entry := range []string{"lite3_plugin:Lite3Plugin", "robots.lite3.plugin:Outer.Inner", "_p:_P1"} {
		if !validEntryPoint.MatchString(entry) {
			t.Errorf("%q should be a valid entry point", entry)
		}
	}
	vendor := &Declaration{Kind: KindVendor, Vendor: &Vendor{}}
	if _, err := vendor.StartNative(context.Background(), "p:P", "office", start, io.Discard); err == nil {
		t.Error("a vendor declaration should not start a native session")
	}
}

func TestNativeStore(t *testing.T) {
	home := t.TempDir()
	oldHome := config.HomeDir
	config.HomeDir = home
	defer func() { config.HomeDir = oldHome }()

	for store, want := range map[string]string{
		"~/emos/maps": filepath.Join(home, "emos", "maps"),
		"~":           home,
		"/data/maps":  "/data/maps",
	} {
		if got := nativeDecl(store).Store(); got != want {
			t.Errorf("Store(%q) = %q, want %q", store, got, want)
		}
	}
}

func TestNativeMapsAreListedAndRemovedWithoutTheVendor(t *testing.T) {
	store := t.TempDir()
	for _, name := range []string{"office-1", "lab-2"} {
		os.MkdirAll(filepath.Join(store, name), 0o755)
	}
	os.WriteFile(filepath.Join(store, "office-1", "occ_grid.yaml"), []byte("image: occ_grid.pgm\n"), 0o644)
	os.Symlink("office-1", filepath.Join(store, "active"))
	decl := nativeDecl(store)

	maps, err := decl.List()
	if err != nil || len(maps) != 2 {
		t.Fatalf("List: %v, %+v", err, maps)
	}
	office, _ := decl.Find("office-1")
	lab, _ := decl.Find("lab-2")
	if !office.Active || office.Grid == "" || lab.Active || lab.Grid != "" {
		t.Errorf("office = %+v, lab = %+v", office, lab)
	}

	noRunner := func([]string) error { t.Fatal("no command should run for a native store"); return nil }
	var active *ErrMapIsActive
	if err := decl.Remove("office-1", noRunner); !errors.As(err, &active) {
		t.Errorf("removing the active map: want ErrMapIsActive, got %v", err)
	}
	if err := decl.Remove("lab-2", noRunner); err != nil {
		t.Errorf("Remove: %v", err)
	}
	if _, err := os.Stat(filepath.Join(store, "lab-2")); !os.IsNotExist(err) {
		t.Error("lab-2 should be gone")
	}
}

func TestNativeNeedsAHostInstall(t *testing.T) {
	for mode, supported := range map[config.InstallMode]bool{
		config.ModePixi: true, config.ModeNative: true,
		config.ModeOSSContainer: false, config.ModeLicensed: false,
	} {
		if err := NativeSupported(mode); (err == nil) != supported {
			t.Errorf("NativeSupported(%s) = %v", mode, err)
		}
	}
}

func TestBackendInstalledAsksTheEnvironment(t *testing.T) {
	for code, want := range map[int]bool{0: true, 1: false} {
		var shell string
		start := func(sh string, _ io.Writer) (Process, error) {
			shell = sh
			proc := newFakeProcess()
			proc.exit(code)
			return proc, nil
		}
		if got, err := BackendInstalled(start); err != nil || got != want {
			t.Errorf("exit %d: BackendInstalled = %v, %v", code, got, err)
		}
		if shell != "ros2 pkg prefix glim_ros" {
			t.Errorf("shell = %q", shell)
		}
	}
}

// A stand-in for emos_mapping's session: it announces the map directory, and
// saves the grid when it is interrupted.
const fakeSessionModule = `import argparse, os, sys, time
parser = argparse.ArgumentParser()
parser.add_argument("--plugin")
parser.add_argument("--name")
args = parser.parse_args()
directory = os.path.join(os.environ["FAKE_STORE"], args.name + "-20260918-101500")
os.makedirs(directory)
print("Map directory: " + directory, flush=True)
try:
    while True:
        time.sleep(0.05)
except KeyboardInterrupt:
    with open(os.path.join(directory, "occ_grid.yaml"), "w") as f:
        f.write("image: occ_grid.pgm\n")
    print("Map saved: " + directory, flush=True)
    sys.exit(0)
`

func TestNativeSessionAgainstARealProcess(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 is not installed")
	}
	store, modules := t.TempDir(), t.TempDir()
	pkg := filepath.Join(modules, "emos_mapping")
	os.MkdirAll(pkg, 0o755)
	os.WriteFile(filepath.Join(pkg, "__init__.py"), nil, 0o644)
	os.WriteFile(filepath.Join(pkg, "session.py"), []byte(fakeSessionModule), 0o644)

	// As a run session starts it: through a shell, in its own process group.
	start := func(shell string, out io.Writer) (Process, error) {
		cmd := exec.Command("bash", "-c", "exec "+shell)
		cmd.Env = append(os.Environ(), "PYTHONPATH="+modules, "FAKE_STORE="+store)
		cmd.Stdout, cmd.Stderr = out, out
		return runner.StartProcess(cmd)
	}
	var log bytes.Buffer
	session, err := nativeDecl(store).StartNative(context.Background(), "lite3_plugin:Lite3Plugin", "office", start, &log)
	if err != nil {
		t.Fatalf("StartNative: %v\n%s", err, log.String())
	}
	built, err := session.Stop(context.Background())
	if err != nil {
		t.Fatalf("Stop: %v\n%s", err, log.String())
	}
	if built.Name != "office-20260918-101500" || built.Grid == "" {
		t.Errorf("built = %+v", built)
	}
	if !strings.Contains(log.String(), "Map saved: ") {
		t.Errorf("log = %q", log.String())
	}
}
