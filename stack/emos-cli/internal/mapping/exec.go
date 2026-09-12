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

// command renders an argv template and prefixes sudo when the provider needs
// privilege. Returns nil when the verb is not declared.
func (d *Declaration) command(argv []string, name string) []string {
	if len(argv) == 0 {
		return nil
	}
	out := Render(argv, name)
	if d.Kind == KindVendor && d.Vendor != nil && d.Vendor.RequiresRoot {
		out = append([]string{"sudo"}, out...)
	}
	return out
}

// checkLocal rejects a provider whose commands run on another machine.
func (d *Declaration) checkLocal() error {
	if d.Kind != KindVendor || d.Vendor == nil {
		return nil
	}
	if host := d.Vendor.Host; host != "" && host != "local" {
		return fmt.Errorf("this robot maps on %s; running commands there is not supported yet", host)
	}
	return nil
}
