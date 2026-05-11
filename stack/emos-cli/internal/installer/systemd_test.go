package installer

import (
	"strings"
	"testing"

	"github.com/automatika-robotics/emos-cli/internal/config"
)

func TestSystemdUnitRender(t *testing.T) {
	u := SystemdUnit{
		Name:        "test.service",
		Description: "Test unit",
		After:       []string{"network-online.target"},
		Wants:       []string{"network-online.target"},
		ExecStart:   "/usr/local/bin/test --flag",
		Restart:     "on-failure",
		User:        "alice",
		Environment: []string{"FOO=bar", "BAZ=qux"},
	}
	got := u.Render()

	wantParts := []string{
		"[Unit]",
		"Description=Test unit",
		"After=network-online.target",
		"Wants=network-online.target",
		"[Service]",
		"Restart=on-failure",
		"User=alice",
		"Environment=FOO=bar",
		"Environment=BAZ=qux",
		"ExecStart=/usr/local/bin/test --flag",
		"[Install]",
		"WantedBy=multi-user.target",
	}
	for _, want := range wantParts {
		if !strings.Contains(got, want) {
			t.Errorf("Render output missing %q\n--- got ---\n%s", want, got)
		}
	}
}

func TestDashboardUnitDefaultsToConfigPort(t *testing.T) {
	// port=0 must fall back to config.DefaultDashboardPort, not a hardcoded
	// literal. Regression check for the port-consolidation work.
	u := DashboardUnit("/usr/local/bin/emos", "alice", 0)
	want := "/usr/local/bin/emos serve --addr :"
	if !strings.Contains(u.ExecStart, want) {
		t.Fatalf("ExecStart = %q, want prefix %q", u.ExecStart, want)
	}
	if !strings.HasSuffix(u.ExecStart, formatPort(config.DefaultDashboardPort)) {
		t.Fatalf("ExecStart = %q, want suffix port %d", u.ExecStart, config.DefaultDashboardPort)
	}
	if u.Name != config.DashboardServiceName {
		t.Fatalf("Name = %q, want %q", u.Name, config.DashboardServiceName)
	}
	if u.Restart != "on-failure" {
		t.Fatalf("Restart = %q, want on-failure", u.Restart)
	}
	if u.User != "alice" {
		t.Fatalf("User = %q, want alice", u.User)
	}
}

func TestDashboardUnitExplicitPort(t *testing.T) {
	u := DashboardUnit("/bin/emos", "", 9123)
	if !strings.Contains(u.ExecStart, ":9123") {
		t.Fatalf("ExecStart = %q, want port 9123", u.ExecStart)
	}
}

func TestDashboardUnitHardening(t *testing.T) {
	// Sandboxing knobs need to actually render in the unit file. Each one
	// is a one-liner systemd will reject silently if misspelled, so the
	// test guards against typos as much as accidental removal.
	u := DashboardUnit("/usr/local/bin/emos", "alice", 8765)
	body := u.Render()

	wantInService := []string{
		"NoNewPrivileges=true",
		"ProtectSystem=strict",
		"PrivateTmp=true",
	}
	for _, w := range wantInService {
		if !strings.Contains(body, w) {
			t.Errorf("Render output missing %q\n--- got ---\n%s", w, body)
		}
	}

	// ReadWritePaths is conditional on resolving alice's home dir; on a
	// CI runner alice doesn't exist, so we only assert the directive
	// appears when the hardening slice contains it.
	for _, h := range u.Hardening {
		if strings.HasPrefix(h, "ReadWritePaths=") {
			if !strings.Contains(h, "/emos") || !strings.Contains(h, "/.config/emos") {
				t.Fatalf("ReadWritePaths = %q, want both ~/emos and ~/.config/emos", h)
			}
		}
	}
}

func TestDashboardUnitBootHardening(t *testing.T) {
	// The boot-time hardening (StartLimit*/RestartSec) is what makes the
	// unit tolerant to a slow-coming-up network at reboot. Regression check:
	// StartLimitBurst and StartLimitIntervalSec MUST be in [Unit] (systemd
	// rejects them silently in [Service] for new units), RestartSec MUST be
	// in [Service] alongside Restart=.
	u := DashboardUnit("/usr/local/bin/emos", "", 8765)
	body := u.Render()

	unitIdx := strings.Index(body, "[Unit]")
	serviceIdx := strings.Index(body, "[Service]")
	if unitIdx < 0 || serviceIdx < 0 || unitIdx > serviceIdx {
		t.Fatalf("missing or misordered [Unit]/[Service] sections; got body=\n%s", body)
	}
	unitSection := body[unitIdx:serviceIdx]
	serviceSection := body[serviceIdx:]

	for _, want := range []string{"StartLimitBurst=10", "StartLimitIntervalSec=300"} {
		if !strings.Contains(unitSection, want) {
			t.Errorf("[Unit] missing %q\n--- got [Unit] ---\n%s", want, unitSection)
		}
		// Must NOT also be in [Service] (systemd rejects them there in
		// modern versions).
		if strings.Contains(serviceSection, want) {
			t.Errorf("%q must live only in [Unit]; also found in [Service]", want)
		}
	}
	if !strings.Contains(serviceSection, "RestartSec=5") {
		t.Errorf("[Service] missing RestartSec=5\n--- got [Service] ---\n%s", serviceSection)
	}
}

func TestSystemdUnitOmitsZeroLimits(t *testing.T) {
	// Other units (e.g. ContainerUnit) leave the boot-hardening fields at
	// their zero value. Render() must not emit StartLimit*/RestartSec when
	// they're 0 -- otherwise systemd parses garbage like "RestartSec=0" as
	// "restart immediately" and we'd accidentally regress existing units.
	u := SystemdUnit{
		Name:        "test.service",
		Description: "Test",
		ExecStart:   "/bin/true",
		Restart:     "always",
		// RestartSec, StartLimitBurst, StartLimitIntervalSec all 0.
	}
	body := u.Render()
	for _, forbidden := range []string{"RestartSec=", "StartLimitBurst=", "StartLimitIntervalSec="} {
		if strings.Contains(body, forbidden) {
			t.Errorf("unit with zero-valued limits emitted %q\n--- got ---\n%s", forbidden, body)
		}
	}
}

func TestContainerUnitHardening(t *testing.T) {
	u := ContainerUnit("emos")
	body := u.Render()
	for _, w := range []string{"NoNewPrivileges=true", "ProtectSystem=strict", "PrivateTmp=true"} {
		if !strings.Contains(body, w) {
			t.Errorf("Render output missing %q\n--- got ---\n%s", w, body)
		}
	}
}

func TestContainerUnit(t *testing.T) {
	u := ContainerUnit("emos")
	if u.Name != config.ServiceName {
		t.Fatalf("Name = %q, want %q", u.Name, config.ServiceName)
	}
	if u.Restart != "always" {
		t.Fatalf("Restart = %q, want always", u.Restart)
	}
	if !strings.Contains(u.ExecStart, "docker start -a emos") {
		t.Fatalf("ExecStart = %q, want docker start", u.ExecStart)
	}
	if !strings.Contains(u.ExecStop, "docker stop -t 2 emos") {
		t.Fatalf("ExecStop = %q, want docker stop", u.ExecStop)
	}
	if !contains(u.Requires, "docker.service") {
		t.Fatalf("Requires = %v, want docker.service", u.Requires)
	}
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

// formatPort exists only so the test reads naturally. We don't import "fmt"
// at the top to keep the test file's dependency surface small.
func formatPort(p int) string {
	if p == 0 {
		return ""
	}
	out := ""
	for p > 0 {
		out = string(rune('0'+p%10)) + out
		p /= 10
	}
	return out
}
