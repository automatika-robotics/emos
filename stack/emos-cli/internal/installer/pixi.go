package installer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// PixiInstallHint is the one-liner shown to users who need to install pixi.
const PixiInstallHint = "curl -fsSL https://pixi.sh/install.sh | bash"

// ResolvePixi finds the pixi binary in priority order:
//  1. PATH (works in interactive shells).
//  2. ~/.pixi/bin/pixi (the pixi installer's standard target).
//  3. /usr/local/bin/pixi (system-wide installs).
//
// Returns an absolute path, or an error listing the locations checked.
func ResolvePixi() (string, error) {
	if p, err := exec.LookPath("pixi"); err == nil {
		return p, nil
	}
	var checked []string
	if home, err := os.UserHomeDir(); err == nil {
		p := filepath.Join(home, ".pixi", "bin", "pixi")
		checked = append(checked, p)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	checked = append(checked, "/usr/local/bin/pixi")
	if st, err := os.Stat("/usr/local/bin/pixi"); err == nil && !st.IsDir() {
		return "/usr/local/bin/pixi", nil
	}
	return "", fmt.Errorf(
		"pixi not found in PATH or %v -- install it from https://pixi.sh", checked)
}
