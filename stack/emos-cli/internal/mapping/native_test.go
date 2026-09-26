package mapping

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

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
		"lite3_plugin:Lite3Plugin", "office", announcing(t, proc, dir, &shell), &log, nil)
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
	_, err := nativeDecl(t.TempDir()).StartNative(context.Background(), "p:P", "office", start, io.Discard, nil)
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
			"p:P", "office", announcing(t, proc, dir, &shell), io.Discard, nil)
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
		"p:P", "office", announcing(t, proc, dir, &shell), io.Discard, nil)
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
		"p:P", "office", announcing(t, proc, filepath.Join(store, "office-1"), &shell), io.Discard, nil)
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
	_, err := nativeDecl(t.TempDir()).StartNative(ctx, "p:P", "office", start, io.Discard, nil)
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
		if _, err := decl.StartNative(context.Background(), c.entry, c.name, start, io.Discard, nil); err == nil {
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
	if _, err := vendor.StartNative(context.Background(), "p:P", "office", start, io.Discard, nil); err == nil {
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

// nativeStore builds a store with the named maps. Each has the files a session
// writes, and the backend's working data in a subdirectory.
func nativeStore(t *testing.T, names ...string) *Declaration {
	t.Helper()
	store := t.TempDir()
	for _, name := range names {
		dir := filepath.Join(store, name)
		os.MkdirAll(filepath.Join(dir, "glim", "dump"), 0o755)
		os.WriteFile(filepath.Join(dir, "occ_grid.yaml"), []byte("image: occ_grid.pgm\n"), 0o644)
		os.WriteFile(filepath.Join(dir, "occ_grid.pgm"), []byte("P5 1 1 255\n\xfe"), 0o644)
		os.WriteFile(filepath.Join(dir, "map.json"), []byte(`{"name": "`+name+`"}`), 0o644)
		os.WriteFile(filepath.Join(dir, "glim", "dump", "graph.bin"), []byte("working data"), 0o644)
	}
	return nativeDecl(store)
}

func noCommand(t *testing.T) Runner {
	return func([]string) error { t.Fatal("no command should run for a native store"); return nil }
}

func TestNativeUseRepointsTheActiveLink(t *testing.T) {
	decl := nativeStore(t, "office-1", "lab-2")
	for _, name := range []string{"office-1", "lab-2"} { // none active, then a switch
		if err := decl.Use(name, noCommand(t)); err != nil {
			t.Fatalf("Use(%s): %v", name, err)
		}
		if active, _ := decl.ActiveName(); active != name {
			t.Errorf("active = %q, want %q", active, name)
		}
	}
	// Relative, so the store can be moved, and what a recipe follows to the grid.
	if target, _ := os.Readlink(filepath.Join(decl.Store(), "active")); target != "lab-2" {
		t.Errorf("link target = %q", target)
	}
	if _, err := os.Stat(filepath.Join(decl.Store(), "active", "occ_grid.yaml")); err != nil {
		t.Errorf("the grid should be reachable through the link: %v", err)
	}
	if maps, _ := decl.List(); len(maps) != 2 {
		t.Errorf("nothing but the two maps should be listed, got %+v", maps)
	}
}

func TestNativeUseRefusesWhatARecipeCouldNotLoad(t *testing.T) {
	decl := nativeStore(t, "office-1")
	os.MkdirAll(filepath.Join(decl.Store(), "unfinished-3"), 0o755)

	var noGrid *ErrNoGrid
	if err := decl.Use("unfinished-3", noCommand(t)); !errors.As(err, &noGrid) {
		t.Errorf("a map without a grid: want ErrNoGrid, got %v", err)
	}
	var missing *ErrNoSuchMap
	if err := decl.Use("nowhere", noCommand(t)); !errors.As(err, &missing) {
		t.Errorf("an unknown map: want ErrNoSuchMap, got %v", err)
	}
	if active, _ := decl.ActiveName(); active != "" {
		t.Errorf("nothing should have become active, got %q", active)
	}
}

func TestNativeUseLeavesARealDirectoryNamedLikeTheLinkAlone(t *testing.T) {
	decl := nativeStore(t, "office-1")
	os.MkdirAll(filepath.Join(decl.Store(), "active"), 0o755)
	if err := decl.Use("office-1", noCommand(t)); err == nil {
		t.Error("want an error: 'active' is a directory, not a link")
	}
}

func zipEntries(t *testing.T, path string) []string {
	t.Helper()
	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	var names []string
	for _, f := range r.File {
		names = append(names, f.Name)
	}
	return names
}

func TestNativeExportPacksTheMapFilesOnly(t *testing.T) {
	decl := nativeStore(t, "office-1")
	dest := filepath.Join(t.TempDir(), "map-archives")

	archive, err := decl.Export("office-1", dest, noCommand(t))
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if archive != filepath.Join(dest, "office-1.zip") {
		t.Errorf("archive = %q", archive)
	}
	want := []string{"office-1/map.json", "office-1/occ_grid.pgm", "office-1/occ_grid.yaml"}
	if got := zipEntries(t, archive); !slices.Equal(got, want) {
		t.Errorf("entries = %v, want %v", got, want)
	}

	if _, err := decl.Export("office-1", dest, noCommand(t)); err == nil {
		t.Error("an archive of the same name must never be replaced")
	}
	var missing *ErrNoSuchMap
	if _, err := decl.Export("nowhere", dest, noCommand(t)); !errors.As(err, &missing) {
		t.Errorf("an unknown map: want ErrNoSuchMap, got %v", err)
	}
	if left, _ := filepath.Glob(filepath.Join(dest, "*.partial")); len(left) != 0 {
		t.Errorf("partial archives left behind: %v", left)
	}
}

func TestNativeExportAndImportRoundTrip(t *testing.T) {
	robot := nativeStore(t, "office-1")
	archives := filepath.Join(t.TempDir(), "map-archives")
	archive, err := robot.Export("office-1", archives, noCommand(t))
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	other := nativeDecl(filepath.Join(t.TempDir(), "maps"))                    // a store that does not exist yet
	built, err := other.Import(filepath.Base(archive), archives, noCommand(t)) // bare name, looked up
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if built.Name != "office-1" || built.Grid == "" || built.Active {
		t.Errorf("built = %+v", built)
	}
	for _, file := range []string{"occ_grid.yaml", "occ_grid.pgm", "map.json"} {
		want, _ := os.ReadFile(filepath.Join(robot.Store(), "office-1", file))
		got, err := os.ReadFile(filepath.Join(other.Store(), "office-1", file))
		if err != nil || !bytes.Equal(got, want) {
			t.Errorf("%s did not survive the round trip: %v", file, err)
		}
	}
	if err := other.Use("office-1", noCommand(t)); err != nil {
		t.Errorf("an imported map should be usable: %v", err)
	}

	var exists *ErrMapExists
	if _, err := other.Import(archive, archives, noCommand(t)); !errors.As(err, &exists) {
		t.Errorf("importing it again: want ErrMapExists, got %v", err)
	}
	if maps, _ := other.List(); len(maps) != 1 {
		t.Errorf("only the imported map should be listed, got %+v", maps)
	}
}

// writeZip makes an archive with the given entries; a name ending in "/" is a
// directory, and mode 0 means a plain file.
func writeZip(t *testing.T, entries map[string]os.FileMode) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "hostile.zip")
	out, _ := os.Create(path)
	zw := zip.NewWriter(out)
	for name, mode := range entries {
		header := &zip.FileHeader{Name: name}
		if mode != 0 {
			header.SetMode(mode)
		}
		w, err := zw.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(name, "/") {
			io.WriteString(w, "x")
		}
	}
	zw.Close()
	out.Close()
	return path
}

func TestNativeImportRefusesWhatCouldUnpackOutsideTheMap(t *testing.T) {
	for label, entries := range map[string]map[string]os.FileMode{
		"a path that climbs out":   {"../evil.yaml": 0},
		"a climb inside the map":   {"office-1/../../evil.yaml": 0},
		"an absolute path":         {"/etc/cron.d/evil": 0},
		"a file outside a map dir": {"occ_grid.yaml": 0},
		"a nested directory":       {"office-1/glim/dump.bin": 0},
		"two maps":                 {"office-1/occ_grid.yaml": 0, "lab-2/occ_grid.yaml": 0},
		"a symlink":                {"office-1/occ_grid.yaml": os.ModeSymlink | 0o777},
		"a name that is an option": {"-rf/occ_grid.yaml": 0},
		"nothing":                  {},
	} {
		decl := nativeStore(t)
		var notArchive *ErrNotAMapArchive
		if _, err := decl.Import(writeZip(t, entries), "", noCommand(t)); !errors.As(err, &notArchive) {
			t.Errorf("%s: want ErrNotAMapArchive, got %v", label, err)
		}
		if left, _ := os.ReadDir(decl.Store()); len(left) != 0 {
			t.Errorf("%s: the store should be untouched, got %v", label, left)
		}
	}
}

func TestNativeImportRefusesWhatIsNotAnEMOSArchive(t *testing.T) {
	decl := nativeStore(t)
	tarball := filepath.Join(t.TempDir(), "vendor-export.tar.gz")
	os.WriteFile(tarball, []byte("\x1f\x8b not a zip"), 0o644)
	var notArchive *ErrNotAMapArchive
	if _, err := decl.Import(tarball, "", noCommand(t)); !errors.As(err, &notArchive) {
		t.Errorf("want ErrNotAMapArchive, got %v", err)
	}
	var noArchive *ErrNoSuchArchive
	if _, err := decl.Import("nowhere.zip", t.TempDir(), noCommand(t)); !errors.As(err, &noArchive) {
		t.Errorf("want ErrNoSuchArchive, got %v", err)
	}
	// A map may not take the name of the link that marks the active one.
	if _, err := decl.Import(writeZip(t, map[string]os.FileMode{"active/occ_grid.yaml": 0}), "", noCommand(t)); err == nil {
		t.Error("a map named like the active link should be refused")
	}
	// The map directory's own entry, which some zip tools add, is fine.
	ok := writeZip(t, map[string]os.FileMode{"office-1/": os.ModeDir | 0o755, "office-1/occ_grid.yaml": 0})
	if built, err := decl.Import(ok, "", noCommand(t)); err != nil || built.Grid == "" {
		t.Errorf("want the map, got %+v, %v", built, err)
	}
}

func TestNativeNeedsAHostInstall(t *testing.T) {
	for mode, supported := range map[config.InstallMode]bool{
		config.ModePixi: true, config.ModeNative: true,
		config.ModeOSSContainer: false,
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
	session, err := nativeDecl(store).StartNative(context.Background(), "lite3_plugin:Lite3Plugin", "office", start, &log, nil)
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

func TestAHangingNativeSessionIsKilledOnStop(t *testing.T) {
	old := stopTimeout
	stopTimeout = 20 * time.Millisecond
	defer func() { stopTimeout = old }()

	store := t.TempDir()
	dir := filepath.Join(store, "office-1")
	for _, saved := range []bool{false, true} {
		proc := newFakeProcess() // ignores the interrupt and never exits
		var shell string
		session, err := nativeDecl(store).StartNative(context.Background(),
			"p:P", "office", announcing(t, proc, dir, &shell), io.Discard, nil)
		if err != nil {
			t.Fatalf("StartNative: %v", err)
		}
		if saved {
			os.WriteFile(filepath.Join(dir, "occ_grid.yaml"), []byte("image: occ_grid.pgm\n"), 0o644)
		}
		built, err := session.Stop(context.Background())
		if !proc.killed {
			t.Errorf("saved=%v: the hanging session was not killed", saved)
		}
		switch {
		case saved && (err != nil || built == nil || built.Grid == ""):
			t.Errorf("saved map: want it returned, got %+v, %v", built, err)
		case !saved && !errors.Is(err, ErrStopTimedOut):
			t.Errorf("no map: want ErrStopTimedOut, got %v", err)
		}
	}
}

func TestSessionWarningsReachTheOperator(t *testing.T) {
	store := t.TempDir()
	dir := filepath.Join(store, "office-1")
	proc := newFakeProcess()
	var warnings []string
	var out io.Writer
	start := func(sh string, w io.Writer) (Process, error) {
		os.MkdirAll(dir, 0o755)
		out = w
		io.WriteString(w, "Map directory: "+dir+"\n")
		return proc, nil
	}
	if _, err := nativeDecl(store).StartNative(context.Background(), "p:P", "office", start, io.Discard,
		func(m string) { warnings = append(warnings, m) }); err != nil {
		t.Fatalf("StartNative: %v", err)
	}
	io.WriteString(out, "[INFO] driver started\nMapping warning: no point cloud on /lidar yet\n[INFO] more\n")
	if len(warnings) != 1 || warnings[0] != "no point cloud on /lidar yet" {
		t.Errorf("want the warning forwarded, got %q", warnings)
	}
}
