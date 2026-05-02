// Package webui embeds the compiled Svelte SPA so the daemon ships as a
// single binary.
//
// Only `dist/placeholder.html` is checked into git; the real production
// build (index.html + assets/) is produced by `make web` and embedded at
// compile time.
package webui

import (
	"embed"
	"io/fs"
)

//go:embed dist
var distFS embed.FS

// FS returns the dist/ subtree rooted so paths look like "index.html",
// "_app/...", etc. — what http.FileServer expects.
func FS() fs.FS {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		return distFS
	}
	return sub
}

// HasProductionBuild reports whether a real Vite build is embedded. False on
// a fresh clone or pre-`make web` build — the SPA handler falls back to the
// placeholder in that case.
func HasProductionBuild() bool {
	f := FS()
	if _, err := fs.Stat(f, "index.html"); err == nil {
		return true
	}
	return false
}

// PlaceholderName is the path that exists when no production build is present.
const PlaceholderName = "placeholder.html"
