package runner

import "os/exec"

// execCommand wraps os/exec.Command for use in runner functions.
func execCommand(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}
