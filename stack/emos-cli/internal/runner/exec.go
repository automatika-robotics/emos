package runner

import (
	"os"
	"os/exec"
)

// execCommand wraps os/exec.Command for use in runner functions.
func execCommand(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}

// openLogFile opens a recipe log file for append-write, creating parents as
// needed. Used by daemon-style strategy launches that don't go through `tee`.
func openLogFile(path string) (*os.File, error) {
	if err := os.MkdirAll(parentDir(path), 0755); err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
}

func parentDir(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return p[:i]
		}
	}
	return "."
}
