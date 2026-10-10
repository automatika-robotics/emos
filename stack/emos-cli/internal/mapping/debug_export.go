package mapping

import (
	"archive/zip"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DebugExport is what a debug archive packed.
type DebugExport struct {
	// Archive is the path of the archive.
	Archive string
	// Log is the session log packed with the map, empty when none was found,
	// as for a map imported from another robot.
	Log string
	// RawData says whether the archive holds a debug session's recording.
	RawData bool
}

// ExportDebug packs everything support needs to look into a map EMOS built
// itself: the whole map directory, with GLIM's configuration and dump and a
// debug session's raw sensor data, and the session's log from logsDir. It is
// '<name>-debug.zip' in dest, which 'emos map import' does not take: only the
// map files are a map.
func (d *Declaration) ExportDebug(name, dest, logsDir string) (*DebugExport, error) {
	if d.Kind != KindNative {
		return nil, fmt.Errorf("a debug export is for maps EMOS builds itself; this robot maps with its own software")
	}
	target, err := d.Find(name)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return nil, err
	}
	export := &DebugExport{
		Archive: filepath.Join(dest, target.Name+"-debug.zip"),
		Log:     sessionLog(logsDir, target.Name),
	}
	if _, err := os.Lstat(export.Archive); err == nil {
		return nil, fmt.Errorf("%s already exists", export.Archive)
	}
	if info, err := os.Stat(filepath.Join(target.Path, DebugBag)); err == nil && info.IsDir() {
		export.RawData = true
	}
	partial := export.Archive + ".partial"
	if err := zipMapTree(target, export.Log, partial); err != nil {
		_ = os.Remove(partial)
		return nil, err
	}
	return export, os.Rename(partial, export.Archive)
}

// zipMapTree writes every regular file under a map directory into a new zip
// at path, under the map's name, and the session log under its logs/.
func zipMapTree(m *Map, log, path string) error {
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()
	zw := zip.NewWriter(out)
	add := func(file, entry string, info fs.FileInfo) error {
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = entry
		header.Method = zipMethod(entry)
		w, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}
		return copyInto(w, file)
	}
	err = filepath.WalkDir(m.Path, func(file string, entry fs.DirEntry, err error) error {
		if err != nil || !entry.Type().IsRegular() {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(m.Path, file)
		if err != nil {
			return err
		}
		return add(file, m.Name+"/"+filepath.ToSlash(rel), info)
	})
	if err != nil {
		return err
	}
	if log != "" {
		info, err := os.Stat(log)
		if err != nil {
			return err
		}
		if err := add(log, m.Name+"/logs/"+filepath.Base(log), info); err != nil {
			return err
		}
	}
	if err := zw.Close(); err != nil {
		return err
	}
	return out.Close()
}

// zipMethod is how an entry is packed. Recorded sensor data is stored as it
// is: raw point clouds hardly deflate (by some 7 %), and deflating gigabytes
// would only keep the CPU busy.
func zipMethod(entry string) uint16 {
	if strings.HasSuffix(entry, ".mcap") || strings.HasSuffix(entry, ".db3") {
		return zip.Store
	}
	return zip.Deflate
}

// The stamps on a map directory, '<name>-20060102-150405', and on the log the
// CLI opens for its session just before, 'map-<name>_20060102_150405.log'.
const (
	mapStamp = "20060102-150405"
	logStamp = "20060102_150405"
	// The longest a session takes from opening its log to making the map's
	// directory: the backend is checked in between
	logLead = 10 * time.Minute
)

// sessionLog is the log of the session that built the map in mapDir, or empty
// when logsDir holds none: an imported map was built elsewhere.
func sessionLog(logsDir, mapDir string) string {
	cut := len(mapDir) - len(mapStamp) - 1
	if cut < 1 || mapDir[cut] != '-' {
		return ""
	}
	name := mapDir[:cut]
	made, err := time.ParseInLocation(mapStamp, mapDir[cut+1:], time.Local)
	if err != nil {
		return ""
	}
	prefix := "map-" + name + "_"
	logs, _ := filepath.Glob(filepath.Join(logsDir, prefix+"*.log"))
	best, bestLead := "", logLead+1
	for _, log := range logs {
		stamp := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(log), prefix), ".log")
		opened, err := time.ParseInLocation(logStamp, stamp, time.Local)
		if err != nil {
			continue // another map whose name starts with this one's
		}
		// Stamps are to the second: a log opened in the same second may read later
		lead := made.Sub(opened)
		if lead >= -time.Second && lead <= logLead && lead < bestLead {
			best, bestLead = log, lead
		}
	}
	return best
}
