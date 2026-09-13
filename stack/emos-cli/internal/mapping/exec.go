package mapping

import (
	"fmt"
	"os"
	"os/exec"
)

// Runner executes one rendered argv.
type Runner func(argv []string) error

// SystemRunner runs argv with stdio inherited, so a sudo password prompt
// reaches the terminal and the vendor tool's output stays visible.
func SystemRunner(argv []string) error {
	if len(argv) == 0 {
		return fmt.Errorf("empty command")
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Escalates reports whether a declared argv will ask for a password, so a
// caller can warn before the prompt appears.
func Escalates(argv []string) bool {
	return len(argv) > 0 && argv[0] == "sudo"
}

// checkLocal rejects a provider whose commands run on another machine.
func (d *Declaration) checkLocal() error {
	if host := d.Vendor.Host; host != "" && host != "local" {
		return fmt.Errorf("this robot maps on %s; running commands there is not supported yet", host)
	}
	return nil
}
