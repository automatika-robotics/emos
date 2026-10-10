package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/automatika-robotics/emos-cli/internal/mapping"
)

func TestFreeSpaceIsReadWhereTheStoreWillBe(t *testing.T) {
	// The store of a first map does not exist yet: its filesystem still counts
	free, err := freeSpace(filepath.Join(t.TempDir(), "emos", "maps"))
	if err != nil || free == 0 {
		t.Errorf("freeSpace = %d, %v; want the temp filesystem's free space", free, err)
	}
}

func TestSizesReadAsTheOperatorWouldSayThem(t *testing.T) {
	for n, want := range map[uint64]string{
		300 << 20:           "300 MB",
		1 << 30:             "1.0 GB",
		(5 << 30) + 512<<20: "5.5 GB",
	} {
		if got := formatBytes(n); got != want {
			t.Errorf("formatBytes(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestTheDebugBagIsMeasuredWhereTheSessionRecordsIt(t *testing.T) {
	dir := t.TempDir()
	bag := filepath.Join(dir, mapping.DebugBag)
	if err := os.MkdirAll(bag, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(bag, "bag_0.mcap"), make([]byte, 3000), 0o644)
	os.WriteFile(filepath.Join(bag, "metadata.yaml"), []byte(strings.Repeat("x", 100)), 0o644)
	if size, err := dirSize(bag); err != nil || size != 3100 {
		t.Errorf("dirSize = %d, %v; want 3100", size, err)
	}
	if _, err := dirSize(filepath.Join(dir, "missing")); err == nil {
		t.Error("a missing bag should be an error, not an empty recording")
	}
}
