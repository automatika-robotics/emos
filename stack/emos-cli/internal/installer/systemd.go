package installer

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/automatika-robotics/emos-cli/internal/config"
)

// SystemdUnit captures everything we need to write a service file. The
// dashboard and the container-restart units differ only in name, exec, and
// after/requires — all the rest is uniform.
type SystemdUnit struct {
	Name        string   // "emos.service"
	Description string   // human-readable
	After       []string // e.g. {"docker.service", "network-online.target"}
	Requires    []string
	Wants       []string
	ExecStart   string
	ExecStop    string
	Restart     string // "always", "on-failure", ""
	User        string // "" → run as root
	Environment []string
}

// Render returns the .service file body.
func (u SystemdUnit) Render() string {
	var b strings.Builder
	b.WriteString("[Unit]\n")
	b.WriteString("Description=" + u.Description + "\n")
	if len(u.After) > 0 {
		b.WriteString("After=" + strings.Join(u.After, " ") + "\n")
	}
	if len(u.Requires) > 0 {
		b.WriteString("Requires=" + strings.Join(u.Requires, " ") + "\n")
	}
	if len(u.Wants) > 0 {
		b.WriteString("Wants=" + strings.Join(u.Wants, " ") + "\n")
	}
	b.WriteString("\n[Service]\n")
	if u.Restart != "" {
		b.WriteString("Restart=" + u.Restart + "\n")
	}
	if u.User != "" {
		b.WriteString("User=" + u.User + "\n")
	}
	for _, e := range u.Environment {
		b.WriteString("Environment=" + e + "\n")
	}
	b.WriteString("ExecStart=" + u.ExecStart + "\n")
	if u.ExecStop != "" {
		b.WriteString("ExecStop=" + u.ExecStop + "\n")
	}
	b.WriteString("\n[Install]\n")
	b.WriteString("WantedBy=multi-user.target\n")
	return b.String()
}

// Install writes the unit to /etc/systemd/system/<name>, reloads systemd, and
// optionally enables + starts it. All disk/systemctl writes go through `sudo`
// because the CLI runs unprivileged on the dev path.
func (u SystemdUnit) Install(enable, start bool) error {
	if !u.IsSupported() {
		return fmt.Errorf("systemd not detected")
	}
	path := "/etc/systemd/system/" + u.Name
	cmd := exec.Command("sudo", "tee", path)
	cmd.Stdin = strings.NewReader(u.Render())
	cmd.Stdout = os.Stdout // tee's echo of the file body
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	_ = exec.Command("sudo", "systemctl", "daemon-reload").Run()
	if enable {
		if err := exec.Command("sudo", "systemctl", "enable", u.Name).Run(); err != nil {
			return fmt.Errorf("enable %s: %w", u.Name, err)
		}
	}
	if start {
		if err := exec.Command("sudo", "systemctl", "start", u.Name).Run(); err != nil {
			return fmt.Errorf("start %s: %w", u.Name, err)
		}
	}
	return nil
}

// Uninstall stops, disables, removes, and reloads.
func (u SystemdUnit) Uninstall() error {
	if !u.IsSupported() {
		return nil
	}
	_ = exec.Command("sudo", "systemctl", "stop", u.Name).Run()
	_ = exec.Command("sudo", "systemctl", "disable", u.Name).Run()
	_ = exec.Command("sudo", "rm", "-f", "/etc/systemd/system/"+u.Name).Run()
	_ = exec.Command("sudo", "systemctl", "daemon-reload").Run()
	return nil
}

// IsSupported reports whether systemctl is on this host. We don't fail loudly
// on non-systemd boxes (containers, slim distros) — the CLI is still useful.
func (u SystemdUnit) IsSupported() bool {
	_, err := exec.LookPath("systemctl")
	return err == nil
}

// IsActive returns true if `systemctl is-active <unit>` reports active.
// Returns false on non-systemd hosts or for any other status (inactive,
// failed, etc.). Free function so callers can check by unit name without
// constructing a full SystemdUnit value.
func IsActive(unitName string) bool {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return false
	}
	out, _ := exec.Command("systemctl", "is-active", unitName).Output()
	return strings.TrimSpace(string(out)) == "active"
}

// --- preset constructors ---

// DashboardUnit is the unit that runs `emos serve` at boot. The binary path
// is computed at install time so the unit follows the binary the user is
// actually running (`/usr/local/bin/emos` if installed via the script,
// or whatever os.Executable resolves to in dev).
func DashboardUnit(binaryPath, user string, port int) SystemdUnit {
	if port == 0 {
		port = config.DefaultDashboardPort
	}
	return SystemdUnit{
		Name:        config.DashboardServiceName,
		Description: "EMOS onboarding dashboard",
		After:       []string{"network-online.target"},
		Wants:       []string{"network-online.target"},
		ExecStart:   fmt.Sprintf("%s serve --addr :%d", binaryPath, port),
		Restart:     "on-failure",
		User:        user,
	}
}

// ContainerUnit auto-restarts the EMOS Docker container at boot. Used by
// the licensed install flow.
func ContainerUnit(containerName string) SystemdUnit {
	return SystemdUnit{
		Name:        config.ServiceName,
		Description: "EmbodiedOS Container",
		After:       []string{"docker.service"},
		Requires:    []string{"docker.service"},
		ExecStart:   "/usr/bin/docker start -a " + containerName,
		ExecStop:    "/usr/bin/docker stop -t 2 " + containerName,
		Restart:     "always",
	}
}
