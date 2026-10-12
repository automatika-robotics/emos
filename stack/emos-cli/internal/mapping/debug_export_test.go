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

// debugMap is a map from a debug session, with its recording and its log.
func debugMap(t *testing.T) *Declaration {
	t.Helper()
	decl := nativeStore(t, "office-20261004-193928")
	dir := filepath.Join(decl.Store(), "office-20261004-193928")
	os.MkdirAll(filepath.Join(dir, "glim", "config"), 0o755)
	os.WriteFile(filepath.Join(dir, "glim", "config", "config.json"), []byte("{}"), 0o644)
	os.MkdirAll(filepath.Join(dir, DebugBag), 0o755)
	os.WriteFile(filepath.Join(dir, DebugBag, "bag_0.mcap"), make([]byte, 4096), 0o644)
	os.WriteFile(filepath.Join(dir, DebugLog), []byte("Mapping ready: office\n"), 0o644)
	return decl
}

func TestADebugSessionKeepsItsLogWithTheMap(t *testing.T) {
	log := filepath.Join(t.TempDir(), "map-office_20261004_193911.log")
	os.WriteFile(log, []byte("Mapping ready: office\n"), 0o644)
	session := &NativeSession{Dir: t.TempDir()}
	if err := session.SaveLog(log); err != nil {
		t.Fatalf("SaveLog: %v", err)
	}
	if got, _ := os.ReadFile(filepath.Join(session.Dir, DebugLog)); string(got) != "Mapping ready: office\n" {
		t.Errorf("kept log = %q", got)
	}
}

func TestDebugExportPacksTheWholeMap(t *testing.T) {
	decl := debugMap(t)
	dest := filepath.Join(t.TempDir(), "map-archives")

	export, err := decl.ExportDebug("office-20261004-193928", dest)
	if err != nil {
		t.Fatalf("ExportDebug: %v", err)
	}
	if export.Archive != filepath.Join(dest, "office-20261004-193928-debug.zip") || !export.RawData {
		t.Errorf("export = %+v", export)
	}
	want := []string{
		"office-20261004-193928/debug/bag/bag_0.mcap",
		"office-20261004-193928/debug/session.log",
		"office-20261004-193928/glim/config/config.json",
		"office-20261004-193928/glim/dump/graph.bin",
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

	if _, err := decl.ExportDebug("office-20261004-193928", dest); err == nil {
		t.Error("an archive of the same name must never be replaced")
	}
	// Not a map to import, even into another store: only the map files are
	var notArchive *ErrNotAMapArchive
	if _, err := nativeStore(t).Import(export.Archive, "", noCommand(t)); !errors.As(err, &notArchive) {
		t.Errorf("a debug archive should not import as a map, got %v", err)
	}
}

func TestDebugExportOfAMapWithoutRecording(t *testing.T) {
	decl := nativeStore(t, "imported")
	export, err := decl.ExportDebug("imported", t.TempDir())
	if err != nil {
		t.Fatalf("ExportDebug: %v", err)
	}
	if export.RawData {
		t.Errorf("export = %+v, want no raw data", export)
	}
}

func TestDebugExportIsForMapsEMOSBuilds(t *testing.T) {
	vendor := &Declaration{Kind: KindVendor, Vendor: &Vendor{}}
	if _, err := vendor.ExportDebug("office", t.TempDir()); err == nil {
		t.Error("a vendor map has no session to debug")
	}
}
