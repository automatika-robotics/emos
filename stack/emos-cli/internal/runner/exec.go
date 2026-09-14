package runner

import (
	"os"
	"os/exec"
	"path/filepath"
)

// execCommand wraps os/exec.Command for use in runner functions.
func execCommand(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}

// OpenLog opens a recipe run's log for appending, creating it and its directory
// as needed.
func OpenLog(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
}
