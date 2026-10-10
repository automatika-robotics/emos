package mapping

import (
	"archive/zip"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// debugMap is a map from a debug session, with the log its session left among
// others in logs.
func debugMap(t *testing.T, logs string) *Declaration {
	t.Helper()
	decl := nativeStore(t, "office-20261004-193928")
	dir := filepath.Join(decl.Store(), "office-20261004-193928")
	os.MkdirAll(filepath.Join(dir, "glim", "config"), 0o755)
	os.WriteFile(filepath.Join(dir, "glim", "config", "config.json"), []byte("{}"), 0o644)
	os.MkdirAll(filepath.Join(dir, DebugBag), 0o755)
	os.WriteFile(filepath.Join(dir, DebugBag, "bag_0.mcap"), make([]byte, 4096), 0o644)
	os.MkdirAll(logs, 0o755)
	for _, log := range []string{
		"map-office_20261004_193911.log", // this session: opened 17 s before the map
		"map-office_20261004_180002.log", // an earlier session of the same name
		"map-office_20261004_194500.log", // a later one
		"map-office_lab_20261004_193911.log",
		"map-lab_20261004_193911.log",
	} {
		os.WriteFile(filepath.Join(logs, log), []byte(log), 0o644)
	}
	return decl
}

func TestDebugExportPacksTheWholeMapAndItsSessionLog(t *testing.T) {
	logs := filepath.Join(t.TempDir(), "logs")
	decl := debugMap(t, logs)
	dest := filepath.Join(t.TempDir(), "map-archives")

	export, err := decl.ExportDebug("office-20261004-193928", dest, logs)
	if err != nil {
		t.Fatalf("ExportDebug: %v", err)
	}
	if export.Archive != filepath.Join(dest, "office-20261004-193928-debug.zip") {
		t.Errorf("archive = %q", export.Archive)
	}
	if export.Log != filepath.Join(logs, "map-office_20261004_193911.log") || !export.RawData {
		t.Errorf("export = %+v", export)
	}
	want := []string{
		"office-20261004-193928/debug/bag/bag_0.mcap",
		"office-20261004-193928/glim/config/config.json",
		"office-20261004-193928/glim/dump/graph.bin",
		"office-20261004-193928/logs/map-office_20261004_193911.log",
		"office-20261004-193928/map.json",
		"office-20261004-193928/occ_grid.pgm",
		"office-20261004-193928/occ_grid.yaml",
	}
	got := zipEntries(t, export.Archive)
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("entries = %v, want %v", got, want)
	}

	// The recording is stored as it is, the rest deflated
	r, err := zip.OpenReader(export.Archive)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range r.File {
		want := zip.Deflate
		if strings.HasSuffix(f.Name, ".mcap") {
			want = zip.Store
		}
		if f.Method != want {
			t.Errorf("%s packed with method %d, want %d", f.Name, f.Method, want)
		}
	}
	r.Close()

	if _, err := decl.ExportDebug("office-20261004-193928", dest, logs); err == nil {
		t.Error("an archive of the same name must never be replaced")
	}
	// Not a map to import, even into another store: only the map files are
	var notArchive *ErrNotAMapArchive
	if _, err := nativeStore(t).Import(export.Archive, "", noCommand(t)); !errors.As(err, &notArchive) {
		t.Errorf("a debug archive should not import as a map, got %v", err)
	}
}

func TestDebugExportOfAMapWithoutLogOrRecording(t *testing.T) {
	decl := nativeStore(t, "imported")
	export, err := decl.ExportDebug("imported", t.TempDir(), filepath.Join(t.TempDir(), "logs"))
	if err != nil {
		t.Fatalf("ExportDebug: %v", err)
	}
	if export.Log != "" || export.RawData {
		t.Errorf("export = %+v, want no log and no raw data", export)
	}
}

func TestDebugExportIsForMapsEMOSBuilds(t *testing.T) {
	vendor := &Declaration{Kind: KindVendor, Vendor: &Vendor{}}
	if _, err := vendor.ExportDebug("office", t.TempDir(), t.TempDir()); err == nil {
		t.Error("a vendor map has no session to debug")
	}
}

func TestTheSessionLogIsTheOneOpenedJustBeforeTheMap(t *testing.T) {
	logs := t.TempDir()
	for _, log := range []string{"map-casa-salon_20261004_193920.log", "map-casa-salon_20261004_192500.log"} {
		os.WriteFile(filepath.Join(logs, log), nil, 0o644)
	}
	if got := sessionLog(logs, "casa-salon-20261004-193928"); got != filepath.Join(logs, "map-casa-salon_20261004_193920.log") {
		t.Errorf("sessionLog = %q", got)
	}
	for _, dir := range []string{"casa-salon", "casa-salon-2026", "casa-salon-20261004-999999", "casa-salon-20261005-120000"} {
		if got := sessionLog(logs, dir); got != "" {
			t.Errorf("sessionLog(%q) = %q, want none", dir, got)
		}
	}
}
