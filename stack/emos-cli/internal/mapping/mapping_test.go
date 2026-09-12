package mapping

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

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
	if decl.Native != nil {
		t.Error("native must stay nil for a vendor declaration")
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

func TestResolveNative(t *testing.T) {
	decl, err := Resolve(cfgWith(nativeDescribe))
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if decl.Kind != KindNative || decl.Native == nil {
		t.Fatalf("want native declaration, got kind=%q", decl.Kind)
	}
	if decl.Vendor != nil {
		t.Error("vendor must stay nil for a native declaration")
	}
	// Feedback keys, deliberately not topic names.
	if decl.Native.Cloud != "lidar" || decl.Native.IMU != "lidar_imu" {
		t.Errorf("inputs = %q / %q", decl.Native.Cloud, decl.Native.IMU)
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
	got := Render([]string{"drmap", "mapping", "-n", "{name}"}, "warehouse")
	want := []string{"drmap", "mapping", "-n", "warehouse"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Render = %v, want %v", got, want)
		}
	}
	// The template must not be mutated -- it is read from the plugin once and
	// reused for every later call.
	tmpl := []string{"drmap", "apply", "{name}"}
	Render(tmpl, "a")
	if tmpl[2] != "{name}" {
		t.Errorf("Render mutated its input: %v", tmpl)
	}
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

func TestNativeStoreIsEmosOwned(t *testing.T) {
	config.Init()
	d := &Declaration{Kind: KindNative, Native: &Native{Cloud: "lidar"}}
	if d.Store() != config.MapsDir {
		t.Errorf("native store = %q, want %q", d.Store(), config.MapsDir)
	}
}


func TestCommandRendersAndEscalates(t *testing.T) {
	d := vendorDecl("/var/opt/robot/data/maps")
	d.Vendor.RequiresRoot = true
	got := d.command([]string{"drmap", "mapping", "-n", "{name}"}, "warehouse")
	want := []string{"sudo", "drmap", "mapping", "-n", "warehouse"}
	if len(got) != len(want) {
		t.Fatalf("command = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("command = %v, want %v", got, want)
		}
	}

	d.Vendor.RequiresRoot = false
	if got := d.command([]string{"drmap", "stop_mapping"}, ""); got[0] == "sudo" {
		t.Errorf("should not escalate when requires_root is false: %v", got)
	}

	// An undeclared verb yields no command rather than an empty argv to run.
	if got := d.command(nil, "x"); got != nil {
		t.Errorf("undeclared verb = %v, want nil", got)
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
