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
