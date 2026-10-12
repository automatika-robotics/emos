package mapping

import (
	"fmt"
	"os"
	"path/filepath"
)

// DebugExport is what a debug archive packed.
type DebugExport struct {
	// Archive is the path of the archive.
	Archive string
	// RawData says whether the archive holds a debug session's recording.
	RawData bool
}

// ExportDebug packs everything support needs to look into a map EMOS built
// itself: the whole map directory, with GLIM's configuration and dump and,
// from a debug session, the raw sensor data and the session's log.
func (d *Declaration) ExportDebug(name, dest string) (*DebugExport, error) {
	if d.Kind != KindNative {
		return nil, fmt.Errorf("a debug export is for maps EMOS builds itself; this robot maps with its own software")
	}
	target, err := d.Find(name)
	if err != nil {
		return nil, err
	}
	export := &DebugExport{}
	if info, err := os.Stat(filepath.Join(target.Path, DebugBag)); err == nil && info.IsDir() {
		export.RawData = true
	}
	if export.Archive, err = d.exportNative(name, dest, true); err != nil {
		return nil, err
	}
	return export, nil
}
