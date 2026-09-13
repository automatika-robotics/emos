package mapping

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/automatika-robotics/emos-cli/internal/config"
)

// A vendor block exactly as sugarcoat's describe() emits it.
const vendorDescribe = `{
  "role": "robot",
  "mapping": {
    "kind": "vendor",
    "start": ["drmap", "mapping", "-b", "-n", "{name}"],
    "stop": ["drmap", "stop_mapping"],
    "store": "/var/opt/robot/data/maps",
    "grid": "occ_grid.yaml",
    "cloud": "full_cloud.pcd",
    "apply": ["drmap", "apply", "{name}"],
    "after_apply": ["systemctl", "restart", "localization.service"],
    "export": null,
    "active_link": "active",
    "requires_root": true,
    "host": "local",
    "area_limit_m": 50.0
  }
}`

const nativeDescribe = `{
  "role": "robot",
  "mapping": {
    "kind": "native",
    "cloud": "lidar",
    "imu": "lidar_imu",
    "z_min": 0.15,
    "z_max": 0.8,
    "resolution": 0.05
  }
}`

func cfgWith(describe string) *config.EMOSConfig {
	return &config.EMOSConfig{
		Plugin: &config.PluginInfo{
			Slug:     "test_plugin",
			Role:     config.RoleRobot,
			Describe: json.RawMessage(describe),
		},
	}
}

func TestResolveVendor(t *testing.T) {
	decl, err := Resolve(cfgWith(vendorDescribe))
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if decl.Kind != KindVendor || decl.Vendor == nil {
		t.Fatalf("want vendor declaration, got kind=%q vendor=%v", decl.Kind, decl.Vendor)
	}
	v := decl.Vendor
	if got := len(v.Start); got != 5 {
		t.Errorf("start argv: want 5 tokens, got %d (%v)", got, v.Start)
	}
	if !v.RequiresRoot {
		t.Error("requires_root should have decoded as true")
	}
	if v.Store != "/var/opt/robot/data/maps" {
		t.Errorf("store = %q", v.Store)
	}
	if v.AreaLimitM != 50 {
		t.Errorf("area_limit_m = %v", v.AreaLimitM)
	}
	if v.Export != nil {
		t.Errorf("a null verb should decode to nil, got %v", v.Export)
	}
}

func TestResolveRefusesNative(t *testing.T) {
	// Plugins already declare native mapping; this version must say it cannot
	// do it rather than treat the robot as mappable.
	if _, err := Resolve(cfgWith(nativeDescribe)); !errors.Is(err, ErrNativeNotSupported) {
		t.Errorf("want ErrNativeNotSupported, got %v", err)
	}
}

func TestResolveWithoutPluginOrSupport(t *testing.T) {
	if _, err := Resolve(nil); !errors.Is(err, ErrNoPlugin) {
		t.Errorf("nil config: want ErrNoPlugin, got %v", err)
	}
	if _, err := Resolve(&config.EMOSConfig{}); !errors.Is(err, ErrNoPlugin) {
		t.Errorf("no robot plugin: want ErrNoPlugin, got %v", err)
	}
	// A plugin that declares no mapping at all.
	if _, err := Resolve(cfgWith(`{"role":"robot","mapping":null}`)); !errors.Is(err, ErrNotSupported) {
		t.Errorf("null mapping: want ErrNotSupported, got %v", err)
	}
	// A plugin installed before mapping declarations existed: no key at all,
	// which must read the same as declaring none rather than crashing.
	if _, err := Resolve(cfgWith(`{"role":"robot"}`)); !errors.Is(err, ErrNotSupported) {
		t.Errorf("absent mapping key: want ErrNotSupported, got %v", err)
	}
}

func TestRenderSubstitutesName(t *testing.T) {
	got := render([]string{"sudo", "drmap", "mapping", "-n", "{name}"}, vars{"name": "warehouse"})
	want := []string{"sudo", "drmap", "mapping", "-n", "warehouse"}
	if !equal(got, want) {
		t.Fatalf("render = %v, want %v", got, want)
	}
	// The template must not be mutated -- it is read from the plugin once and
	// reused for every later call.
	tmpl := []string{"drmap", "apply", "{name}"}
	render(tmpl, vars{"name": "a"})
	if tmpl[2] != "{name}" {
		t.Errorf("render mutated its input: %v", tmpl)
	}
}

func TestRenderDoesNotReexpandSubstitutedValues(t *testing.T) {
	got := render([]string{"{name}", "{path}"}, vars{"name": "{path}", "path": "/a.zip"})
	if got[0] != "{path}" || got[1] != "/a.zip" {
		t.Errorf("render = %v, want [{path} /a.zip]", got)
	}
}

func TestEscalatesReadsTheDeclaredArgv(t *testing.T) {
	if !Escalates([]string{"sudo", "drmap", "apply", "{name}"}) {
		t.Error("an argv starting with sudo escalates")
	}
	if Escalates([]string{"drmap", "pack"}) || Escalates(nil) {
		t.Error("only an argv starting with sudo escalates")
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// buildStore lays out a map store the way a vendor tool would: map directories
// plus an "active" symlink pointing at one of them.
func buildStore(t *testing.T, names []string, active string, withGrid bool) string {
	t.Helper()
	store := t.TempDir()
	for _, n := range names {
		dir := filepath.Join(store, n)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if withGrid {
			if err := os.WriteFile(filepath.Join(dir, "occ_grid.yaml"), []byte("image: x\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	if active != "" {
		if err := os.Symlink(filepath.Join(store, active), filepath.Join(store, "active")); err != nil {
			t.Fatal(err)
		}
	}
	return store
}

func vendorDecl(store string) *Declaration {
	return &Declaration{
		Kind: KindVendor,
		Vendor: &Vendor{
			Store: store, Grid: "occ_grid.yaml", ActiveLink: "active", Host: "local",
		},
	}
}

func TestListMarksActiveAndSkipsTheLink(t *testing.T) {
	store := buildStore(t, []string{"warehouse-20260101-100000", "yard-20260102-110000"},
		"yard-20260102-110000", true)
	maps, err := vendorDecl(store).List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(maps) != 2 {
		t.Fatalf("want 2 maps (the 'active' symlink is not one), got %d: %+v", len(maps), maps)
	}
	var activeCount int
	for _, m := range maps {
		if m.Active {
			activeCount++
			if m.Name != "yard-20260102-110000" {
				t.Errorf("wrong map marked active: %s", m.Name)
			}
		}
		if m.Grid == "" {
			t.Errorf("%s: grid path should have been found", m.Name)
		}
	}
	if activeCount != 1 {
		t.Errorf("want exactly 1 active map, got %d", activeCount)
	}
}

func TestListReportsAMapWithNoGrid(t *testing.T) {
	// A half-finished map: the directory exists but the vendor never wrote the
	// grid. Listing it is right; claiming a recipe could load it is not.
	store := buildStore(t, []string{"aborted-20260101-100000"}, "", false)
	maps, err := vendorDecl(store).List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(maps) != 1 || maps[0].Grid != "" {
		t.Fatalf("want one map with no grid, got %+v", maps)
	}
}

func TestListOnAnAbsentStoreIsEmptyNotAnError(t *testing.T) {
	// Every robot starts here; it is not a failure.
	maps, err := vendorDecl(filepath.Join(t.TempDir(), "never-created")).List()
	if err != nil {
		t.Fatalf("want no error for an absent store, got %v", err)
	}
	if len(maps) != 0 {
		t.Errorf("want no maps, got %+v", maps)
	}
}

func TestListRefusesARemoteStore(t *testing.T) {
	d := vendorDecl("/var/opt/robot/data/maps")
	d.Vendor.Host = "ssh://user@10.21.31.106"
	if _, err := d.List(); err == nil {
		t.Error("a remote store should say so rather than listing the local filesystem")
	}
}

func TestListFindsTheGridTheWayARecipeDoes(t *testing.T) {
	// An imported map may carry office.yaml rather than the declared name; a
	// recipe's active_grid_path() still finds it, so the CLI must too.
	store := buildStore(t, []string{"imported", "ambiguous"}, "", false)
	os.WriteFile(filepath.Join(store, "imported", "office.yaml"), []byte("image: office.pgm\n"), 0o644)
	os.WriteFile(filepath.Join(store, "ambiguous", "a.yaml"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(store, "ambiguous", "b.yaml"), []byte("x"), 0o644)

	maps, err := vendorDecl(store).List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, m := range maps {
		switch m.Name {
		case "imported":
			if filepath.Base(m.Grid) != "office.yaml" {
				t.Errorf("imported grid = %q, want office.yaml", m.Grid)
			}
		case "ambiguous":
			if m.Grid != "" {
				t.Errorf("two candidate grids must not be guessed between, got %q", m.Grid)
			}
		}
	}
}

func TestListResolvesARelativeActiveLink(t *testing.T) {
	store := buildStore(t, []string{"a", "b"}, "", true)
	if err := os.Symlink("b", filepath.Join(store, "active")); err != nil {
		t.Fatal(err)
	}
	active, err := vendorDecl(store).ActiveName()
	if err != nil || active != "b" {
		t.Errorf("ActiveName = %q, %v; want b", active, err)
	}
}

func TestCheckLocalRejectsRemoteHosts(t *testing.T) {
	d := vendorDecl("/maps")
	if err := d.checkLocal(); err != nil {
		t.Errorf("local host should be fine: %v", err)
	}
	d.Vendor.Host = "ssh://user@10.21.31.106"
	if err := d.checkLocal(); err == nil {
		t.Error("remote host should be refused")
	}
}

// recordingRunner captures argv instead of executing, and can create the map
// directory a real vendor tool would have written.
type recorder struct {
	ran     [][]string
	onStop  func()
	after   func()
	failFor string // argv[1] (after sudo) that should fail
}

func (r *recorder) run(argv []string) error {
	r.ran = append(r.ran, argv)
	verb := argv
	if verb[0] == "sudo" {
		verb = verb[1:]
	}
	if r.failFor != "" && len(verb) > 1 && verb[1] == r.failFor {
		return fmt.Errorf("vendor tool failed")
	}
	if len(verb) > 1 && verb[1] == "stop_mapping" && r.onStop != nil {
		r.onStop()
	}
	if r.after != nil {
		r.after()
	}
	return nil
}

// fastStop shrinks the wait-for-map poll so tests do not sleep; restored so the
// real values stand for anything added later.
func fastStop(t *testing.T, timeout time.Duration) {
	t.Helper()
	savedT, savedP := stopTimeout, stopPoll
	stopTimeout, stopPoll = timeout, time.Millisecond
	t.Cleanup(func() { stopTimeout, stopPoll = savedT, savedP })
}

func sessionDecl(store string) *Declaration {
	d := vendorDecl(store)
	d.Vendor.RequiresRoot = true
	d.Vendor.Start = []string{"sudo", "drmap", "mapping", "-b", "-n", "{name}"}
	d.Vendor.Stop = []string{"sudo", "drmap", "stop_mapping"}
	return d
}

func TestSessionIdentifiesTheMapTheVendorNamed(t *testing.T) {
	fastStop(t, 50*time.Millisecond)
	store := buildStore(t, []string{"old-20260101-100000"}, "", true)
	d := sessionDecl(store)

	// The vendor appends its own timestamp, so the directory that appears is
	// not the name we asked for.
	rec := &recorder{onStop: func() {
		dir := filepath.Join(store, "warehouse-20260912-143002")
		os.MkdirAll(dir, 0o755)
		os.WriteFile(filepath.Join(dir, "occ_grid.yaml"), []byte("image: x\n"), 0o644)
	}}

	s, err := d.Start("warehouse", rec.run)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if got := rec.ran[0]; got[0] != "sudo" || got[len(got)-1] != "warehouse" {
		t.Errorf("start argv = %v", got)
	}

	built, err := s.Stop(context.Background())
	if err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if built.Name != "warehouse-20260912-143002" {
		t.Errorf("found map %q, want the one the vendor created", built.Name)
	}
	if built.Grid == "" {
		t.Error("the new map's grid should have been located")
	}
}

func TestSessionRetriesStopUntilTheMapAppears(t *testing.T) {
	fastStop(t, 50*time.Millisecond)
	store := buildStore(t, nil, "", false)
	d := sessionDecl(store)

	// Re-issuing stop is opt-in: the vendor declares it, the framework does not
	// assume it. DEEP Robotics documents it as a conditional remedy ("if
	// stuttering occurs ... you may run it several more times").
	d.Vendor.StopRetries = 2
	calls := 0
	rec := &recorder{}
	rec.onStop = func() {
		calls++
		if calls < 3 {
			return
		}
		os.MkdirAll(filepath.Join(store, "late-20260912-143002"), 0o755)
	}

	s, err := d.Start("late", rec.run)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	built, err := s.Stop(context.Background())
	if err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if built.Name != "late-20260912-143002" {
		t.Errorf("built = %q", built.Name)
	}
	if calls != 3 {
		t.Errorf("stop ran %d times, want 3 (1 + 2 declared retries)", calls)
	}
}

func TestSessionReportsWhenNoMapEverAppears(t *testing.T) {
	fastStop(t, 50*time.Millisecond)
	store := buildStore(t, nil, "", false)
	d := sessionDecl(store)
	rec := &recorder{}

	s, err := d.Start("doomed", rec.run)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := s.Stop(context.Background()); err == nil {
		t.Error("a session that produced nothing must not report success")
	}
}

func TestSessionIgnoresMapsThatExistedBefore(t *testing.T) {
	fastStop(t, 50*time.Millisecond)
	// A pre-existing map must not be mistaken for the one just built.
	store := buildStore(t, []string{"previous-20260101-100000"}, "", true)
	d := sessionDecl(store)
	rec := &recorder{}

	s, err := d.Start("new", rec.run)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := s.Stop(context.Background()); err == nil {
		t.Error("the pre-existing map should not count as the session's output")
	}
}

func TestStartNeedsAStopCommand(t *testing.T) {
	// Starting a session nothing can end would leave the robot mapping.
	d := sessionDecl(buildStore(t, nil, "", false))
	d.Vendor.Stop = nil
	rec := &recorder{}
	if _, err := d.Start("x", rec.run); err == nil {
		t.Error("a declaration without stop must not start")
	}
	if len(rec.ran) != 0 {
		t.Errorf("nothing should run: %v", rec.ran)
	}
}

func TestStartRejectsNamesThatReadAsOptionsOrPaths(t *testing.T) {
	d := sessionDecl(buildStore(t, nil, "", false))
	for _, name := range []string{"", "--force", "../etc", "a/b", "-n"} {
		rec := &recorder{}
		if _, err := d.Start(name, rec.run); err == nil {
			t.Errorf("name %q should be refused", name)
		}
		if len(rec.ran) != 0 {
			t.Errorf("name %q: nothing should run, ran %v", name, rec.ran)
		}
	}
	if _, err := d.Start("warehouse_2.b-1", (&recorder{}).run); err != nil {
		t.Errorf("an ordinary name should start: %v", err)
	}
}

func TestStopGivesUpWhenInterrupted(t *testing.T) {
	fastStop(t, time.Minute)
	d := sessionDecl(buildStore(t, nil, "", false))
	d.Vendor.StopRetries = 2
	calls := 0
	rec := &recorder{}
	rec.onStop = func() { calls++ }

	s, err := d.Start("interrupted", rec.run)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Stop(ctx); !errors.Is(err, ErrStopInterrupted) {
		t.Fatalf("want ErrStopInterrupted, got %v", err)
	}
	if calls != 1 {
		t.Errorf("stop ran %d times; an interrupt must not wait out or retry", calls)
	}
	if got := s.StopCommand(); got != "sudo drmap stop_mapping" {
		t.Errorf("StopCommand = %q", got)
	}
}

func TestStopIssuedOnceWhenNoRetriesDeclared(t *testing.T) {
	// The default must not repeat a vendor command that may not be idempotent.
	fastStop(t, 20*time.Millisecond)
	store := buildStore(t, nil, "", false)
	d := sessionDecl(store) // StopRetries left at 0
	calls := 0
	rec := &recorder{}
	rec.onStop = func() { calls++ }

	s, err := d.Start("once", rec.run)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := s.Stop(context.Background()); err == nil {
		t.Fatal("no map appeared, so Stop must report failure")
	}
	if calls != 1 {
		t.Errorf("stop ran %d times, want 1", calls)
	}
}

func TestRemoveRefusesTheActiveMap(t *testing.T) {
	// Deleting the active map would leave localization with nothing to
	// localize against, and the vendor's docs warn against touching it.
	store := buildStore(t, []string{"keep-20260101-100000", "gone-20260102-100000"},
		"keep-20260101-100000", true)
	d := vendorDecl(store)
	rec := &recorder{}

	err := d.Remove("keep-20260101-100000", rec.run)
	var active *ErrMapIsActive
	if !errors.As(err, &active) {
		t.Fatalf("want ErrMapIsActive, got %v", err)
	}
	if len(rec.ran) != 0 {
		t.Errorf("nothing should run when the delete is refused: %v", rec.ran)
	}
	if _, err := os.Stat(filepath.Join(store, "keep-20260101-100000")); err != nil {
		t.Error("the active map must still be there")
	}
}

func TestRemoveRefusesAnUnknownMap(t *testing.T) {
	store := buildStore(t, []string{"a-20260101-100000"}, "", true)
	d := vendorDecl(store)
	rec := &recorder{}
	var missing *ErrNoSuchMap
	if err := d.Remove("typo", rec.run); !errors.As(err, &missing) {
		t.Fatalf("want ErrNoSuchMap, got %v", err)
	}
	if len(rec.ran) != 0 {
		t.Errorf("nothing should run for a map that is not there: %v", rec.ran)
	}
}

func TestRemoveUsesThePathFromTheStoreNotTheCaller(t *testing.T) {
	// The argv must name the resolved map directory, never a caller-supplied
	// string joined onto the store.
	store := buildStore(t, []string{"gone-20260102-100000"}, "", true)
	d := vendorDecl(store)
	d.Vendor.RequiresRoot = true
	rec := &recorder{after: deletes(filepath.Join(store, "gone-20260102-100000"))}
	if err := d.Remove("gone-20260102-100000", rec.run); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if len(rec.ran) != 1 {
		t.Fatalf("want one command, got %v", rec.ran)
	}
	got := rec.ran[0]
	want := []string{"sudo", "rm", "-rf", "--", filepath.Join(store, "gone-20260102-100000")}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("argv = %v, want %v", got, want)
		}
	}
}

func TestRemovePrefersADeclaredCommand(t *testing.T) {
	store := buildStore(t, []string{"gone-20260102-100000"}, "", true)
	d := vendorDecl(store)
	d.Vendor.RequiresRoot = true
	d.Vendor.Remove = []string{"sudo", "vendortool", "delete", "{name}"}
	rec := &recorder{after: deletes(filepath.Join(store, "gone-20260102-100000"))}
	if err := d.Remove("gone-20260102-100000", rec.run); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if rec.ran[0][1] != "vendortool" {
		t.Errorf("a declared delete command should win over rm: %v", rec.ran[0])
	}
}

// deletes stands in for a vendor delete command that works.
func deletes(dir string) func() {
	return func() { os.RemoveAll(dir) }
}

func TestRemoveWithoutRootDeletesDirectly(t *testing.T) {
	store := buildStore(t, []string{"gone-20260102-100000"}, "", true)
	d := vendorDecl(store)
	d.Vendor.RequiresRoot = false
	rec := &recorder{}
	if err := d.Remove("gone-20260102-100000", rec.run); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if len(rec.ran) != 0 {
		t.Errorf("no command should run: %v", rec.ran)
	}
	if _, err := os.Stat(filepath.Join(store, "gone-20260102-100000")); !os.IsNotExist(err) {
		t.Error("the map directory should be gone")
	}
}

func TestExportRefusesANonActiveMapWhenTheToolTakesNoName(t *testing.T) {
	// drmap pack packages whatever is active; exporting "b" while "a" is
	// active would hand back the wrong map under the right name.
	store := buildStore(t, []string{"a-20260101-100000", "b-20260102-100000"},
		"a-20260101-100000", true)
	d := vendorDecl(store)
	d.Vendor.Export = []string{"drmap", "pack"}
	rec := &recorder{}
	if _, err := d.Export("b-20260102-100000", "", rec.run); err == nil {
		t.Error("exporting a non-active map should be refused, not silently wrong")
	}
	if len(rec.ran) != 0 {
		t.Errorf("nothing should run: %v", rec.ran)
	}
}

func TestExportReportsTheArchiveThatAppeared(t *testing.T) {
	store := buildStore(t, []string{"a-20260101-100000"}, "a-20260101-100000", true)
	out := t.TempDir()
	os.WriteFile(filepath.Join(out, "old.zip"), []byte("x"), 0o644)

	d := vendorDecl(store)
	d.Vendor.Export = []string{"drmap", "pack"}
	d.Vendor.ExportDir = out
	rec := &recorder{}
	rec.onStop = nil
	rec.after = func() {
		os.WriteFile(filepath.Join(out, "map-new.zip"), []byte("y"), 0o644)
	}

	dest := t.TempDir()
	archive, err := d.Export("a-20260101-100000", dest, rec.run)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if filepath.Dir(archive) != dest {
		t.Errorf("archive = %q, want it moved into %q", archive, dest)
	}
	if _, err := os.Stat(archive); err != nil {
		t.Errorf("archive should exist at its new path: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "map-new.zip")); !os.IsNotExist(err) {
		t.Error("the vendor's copy should have been moved, not duplicated")
	}
}

func TestExportRunsTheDeclaredCommandAsIs(t *testing.T) {
	// The vendor documents pack without sudo, and requires_root is true for the
	// provider as a whole; the CLI must not escalate on its behalf.
	store := buildStore(t, []string{"a-20260101-100000"}, "a-20260101-100000", true)
	d := vendorDecl(store)
	d.Vendor.RequiresRoot = true
	d.Vendor.Export = []string{"drmap", "pack"}
	rec := &recorder{}
	if _, err := d.Export("a-20260101-100000", "", rec.run); err != nil {
		t.Fatalf("Export: %v", err)
	}
	if !equal(rec.ran[0], d.Vendor.Export) {
		t.Errorf("ran %v, want exactly %v", rec.ran[0], d.Vendor.Export)
	}
}

func TestExportNeverOverwritesAnArchive(t *testing.T) {
	store := buildStore(t, []string{"a"}, "a", true)
	out, dest := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(dest, "map.zip"), []byte("earlier"), 0o644)

	d := vendorDecl(store)
	d.Vendor.Export = []string{"drmap", "pack"}
	d.Vendor.ExportDir = out
	rec := &recorder{after: func() {
		os.WriteFile(filepath.Join(out, "map.zip"), []byte("new"), 0o644)
	}}

	archive, err := d.Export("a", dest, rec.run)
	if err == nil {
		t.Fatal("an archive of the same name in dest must not be replaced")
	}
	if archive != filepath.Join(out, "map.zip") {
		t.Errorf("archive = %q, want where it still is", archive)
	}
	if data, _ := os.ReadFile(filepath.Join(dest, "map.zip")); string(data) != "earlier" {
		t.Error("the existing archive was overwritten")
	}
}

func TestExportIgnoresDirectoriesInTheExportDir(t *testing.T) {
	store := buildStore(t, []string{"a"}, "a", true)
	out := t.TempDir()
	d := vendorDecl(store)
	d.Vendor.Export = []string{"drmap", "pack"}
	d.Vendor.ExportDir = out
	rec := &recorder{after: func() {
		os.MkdirAll(filepath.Join(out, "unrelated"), 0o755)
	}}
	archive, err := d.Export("a", "", rec.run)
	if err != nil || archive != "" {
		t.Errorf("Export = %q, %v; a new directory is not an archive", archive, err)
	}
}

func TestCopyFileStreamsAndRefusesAnExistingTarget(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "src.zip"), filepath.Join(dir, "dst.zip")
	os.WriteFile(src, []byte("archive"), 0o644)
	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile: %v", err)
	}
	if data, _ := os.ReadFile(dst); string(data) != "archive" {
		t.Errorf("copied %q", data)
	}
	if err := copyFile(src, dst); err == nil {
		t.Error("an existing target must be refused")
	}
}

func TestRemoveReportsAMapTheCommandLeftBehind(t *testing.T) {
	store := buildStore(t, []string{"stubborn"}, "", true)
	d := vendorDecl(store)
	d.Vendor.RequiresRoot = true
	d.Vendor.Remove = []string{"vendortool", "delete", "{name}"}
	if err := d.Remove("stubborn", (&recorder{}).run); err == nil {
		t.Error("a clean exit with the map still in the store must not report success")
	}
}

func importDecl(store string) *Declaration {
	d := vendorDecl(store)
	d.Vendor.RequiresRoot = true
	d.Vendor.Import = []string{"sudo", "drmap", "unpack", "{path}"}
	return d
}

func TestImportRunsWithTheAbsoluteArchivePathAndReportsTheNewMap(t *testing.T) {
	store := buildStore(t, []string{"old-20260101-100000"}, "", true)
	archive := filepath.Join(t.TempDir(), "yard.zip")
	os.WriteFile(archive, []byte("zip"), 0o644)

	d := importDecl(store)
	rec := &recorder{after: func() {
		os.MkdirAll(filepath.Join(store, "yard-20260912-143002"), 0o755)
	}}
	built, err := d.Import(archive, "", rec.run)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	want := []string{"sudo", "drmap", "unpack", archive}
	got := rec.ran[0]
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("argv = %v, want %v", got, want)
		}
	}
	if built.Name != "yard-20260912-143002" {
		t.Errorf("imported = %q", built.Name)
	}
}

func TestImportFindsABareNameAmongExportedArchives(t *testing.T) {
	store := buildStore(t, nil, "", false)
	archives := t.TempDir()
	os.WriteFile(filepath.Join(archives, "yard.zip"), []byte("zip"), 0o644)

	d := importDecl(store)
	rec := &recorder{after: func() {
		os.MkdirAll(filepath.Join(store, "yard"), 0o755)
	}}
	if _, err := d.Import("yard.zip", archives, rec.run); err != nil {
		t.Fatalf("Import: %v", err)
	}
	if got := rec.ran[0][3]; got != filepath.Join(archives, "yard.zip") {
		t.Errorf("archive path = %q, want it resolved in the archives dir", got)
	}
}

func TestImportRefusesAMissingArchiveBeforeRunningAnything(t *testing.T) {
	d := importDecl(buildStore(t, nil, "", false))
	rec := &recorder{}
	_, err := d.Import("typo.zip", t.TempDir(), rec.run)
	var missing *ErrNoSuchArchive
	if !errors.As(err, &missing) {
		t.Fatalf("want ErrNoSuchArchive, got %v", err)
	}
	if len(rec.ran) != 0 {
		t.Errorf("nothing should run for an archive that is not there: %v", rec.ran)
	}
}

func TestImportReportsWhenNoMapAppears(t *testing.T) {
	store := buildStore(t, []string{"yard"}, "", true)
	archive := filepath.Join(t.TempDir(), "yard.zip")
	os.WriteFile(archive, []byte("zip"), 0o644)
	// Unpacking over an existing map leaves nothing new to point at.
	if _, err := importDecl(store).Import(archive, "", (&recorder{}).run); err == nil {
		t.Error("an import that produced no new map must not report success")
	}
}

func TestImportNeedsADeclaredCommand(t *testing.T) {
	d := vendorDecl(buildStore(t, nil, "", false))
	archive := filepath.Join(t.TempDir(), "yard.zip")
	os.WriteFile(archive, []byte("zip"), 0o644)
	if _, err := d.Import(archive, "", (&recorder{}).run); err == nil {
		t.Error("a plugin with no import verb should say so")
	}
}

func useDecl(store string) *Declaration {
	d := vendorDecl(store)
	d.Vendor.Apply = []string{"sudo", "drmap", "apply", "{name}"}
	d.Vendor.AfterApply = []string{"systemctl", "restart", "localization.service"}
	return d
}

// relink points the store's active link at name, as a vendor apply would.
func relink(t *testing.T, store, name string) {
	t.Helper()
	link := filepath.Join(store, "active")
	os.Remove(link)
	if err := os.Symlink(filepath.Join(store, name), link); err != nil {
		t.Fatal(err)
	}
}

func TestUseRunsApplyThenAfterApply(t *testing.T) {
	store := buildStore(t, []string{"a", "b"}, "a", true)
	d := useDecl(store)
	rec := &recorder{after: func() { relink(t, store, "b") }}
	if err := d.Use("b", rec.run); err != nil {
		t.Fatalf("Use: %v", err)
	}
	if len(rec.ran) != 2 {
		t.Fatalf("want apply then after_apply, got %v", rec.ran)
	}
	if got := rec.ran[0]; got[2] != "apply" || got[3] != "b" {
		t.Errorf("apply argv = %v", got)
	}
	if got := rec.ran[1]; got[0] != "systemctl" {
		t.Errorf("after_apply argv = %v", got)
	}
}

func TestUseReportsWhenTheActiveMapDidNotChange(t *testing.T) {
	store := buildStore(t, []string{"a", "b"}, "a", true)
	rec := &recorder{}
	if err := useDecl(store).Use("b", rec.run); err == nil {
		t.Error("a clean exit with the active link unchanged must not report success")
	}
	if len(rec.ran) != 1 {
		t.Errorf("after_apply should not run when apply did not take: %v", rec.ran)
	}
}

func TestUseStopsWhenApplyFails(t *testing.T) {
	store := buildStore(t, []string{"a", "b"}, "a", true)
	rec := &recorder{failFor: "apply"}
	if err := useDecl(store).Use("b", rec.run); err == nil {
		t.Error("a failed apply must be reported")
	}
	if len(rec.ran) != 1 {
		t.Errorf("nothing should run after a failed apply: %v", rec.ran)
	}
}

func TestUseRefusesAnUnknownMap(t *testing.T) {
	store := buildStore(t, []string{"a"}, "a", true)
	rec := &recorder{}
	var missing *ErrNoSuchMap
	if err := useDecl(store).Use("typo", rec.run); !errors.As(err, &missing) {
		t.Fatalf("want ErrNoSuchMap, got %v", err)
	}
	if len(rec.ran) != 0 {
		t.Errorf("nothing should run for a map that is not there: %v", rec.ran)
	}
}

func TestUseNeedsADeclaredCommand(t *testing.T) {
	store := buildStore(t, []string{"a", "b"}, "a", true)
	if err := vendorDecl(store).Use("b", (&recorder{}).run); err == nil {
		t.Error("a plugin with no apply verb should say so")
	}
}
