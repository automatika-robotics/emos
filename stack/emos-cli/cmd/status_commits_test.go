package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func gitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"},
		{"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "first"},
	} {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	return dir
}

func TestWorkspaceCommitsReadTheCheckoutAndItsStack(t *testing.T) {
	ws := gitRepo(t)
	if err := os.MkdirAll(filepath.Join(ws, "stack"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(gitRepo(t), filepath.Join(ws, "stack", "kompass")); err != nil {
		t.Fatal(err)
	}
	commits := workspaceCommits(ws)
	if len(commits["emos"]) != 7 || len(commits["kompass"]) != 7 {
		t.Fatalf("checkouts not read: %v", commits)
	}
	if commits["sugarcoat"] != "" || commits["embodied-agents"] != "" {
		t.Fatalf("a missing checkout should read empty: %v", commits)
	}
}

func TestStackLabelParses(t *testing.T) {
	commits := parseStackLabel("sugarcoat@f5ab9cc kompass@5edb164 embodied-agents@1b56512")
	if len(commits) != 3 || commits["kompass"] != "5edb164" || commits["embodied-agents"] != "1b56512" {
		t.Fatalf("unexpected commits %v", commits)
	}
	if len(parseStackLabel("")) != 0 {
		t.Fatal("an empty label should give no commits")
	}
}

const probeOutput = `automatika_ros_sugar=0.8.0
kompass_interfaces=0.8.0
kompass=0.8.0
kompass-core=0.8.1
`

func TestPackageProbeReadsVersionsOffThePrefixPath(t *testing.T) {
	prefix := t.TempDir()
	share := filepath.Join(prefix, "share", "kompass")
	if err := os.MkdirAll(share, 0o755); err != nil {
		t.Fatal(err)
	}
	xml := "<?xml version=\"1.0\"?>\n<package format=\"3\">\n  <name>kompass</name>\n  <version>0.8.0</version>\n</package>\n"
	if err := os.WriteFile(filepath.Join(share, "package.xml"), []byte(xml), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", "-c", packageProbe())
	cmd.Env = append(os.Environ(), "AMENT_PREFIX_PATH="+t.TempDir()+":"+prefix)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("probe failed: %v", err)
	}
	rows := packageRows(string(out), nil)
	if rows[2].name != "kompass" || rows[2].value != "0.8.0" || !rows[2].found || rows[0].found {
		t.Fatalf("unexpected rows %v", rows)
	}
}

// checkRows compares the rows with the wanted values, "" meaning not found.
func checkRows(t *testing.T, rows []packageRow, want map[string]string) {
	t.Helper()
	if len(rows) != len(want) {
		t.Fatalf("unexpected rows %v", rows)
	}
	for _, row := range rows {
		if row.value != want[row.name] || row.found != (want[row.name] != "") {
			t.Errorf("%s: got %q found=%v", row.name, row.value, row.found)
		}
	}
}

func TestPackageRowsOffTheDevChannelShowVersions(t *testing.T) {
	checkRows(t, packageRows(probeOutput, nil), map[string]string{
		"automatika_ros_sugar":       "0.8.0",
		"automatika_embodied_agents": "",
		"kompass":                    "0.8.0",
		"kompass_interfaces":         "0.8.0",
		"emos_mapping":               "",
		"kompass-core":               "0.8.1",
	})
}

func TestPackageRowsOnTheDevChannelShowCommits(t *testing.T) {
	checkRows(t, packageRows(probeOutput, map[string]string{"sugarcoat": "a389fb2", "kompass": "0a4b9b2"}), map[string]string{
		"automatika_ros_sugar":       "a389fb2",
		"automatika_embodied_agents": "",
		"kompass":                    "0a4b9b2",
		"kompass_interfaces":         "0a4b9b2",
		"emos_mapping":               "",
		"kompass-core":               "0.8.1",
	})
	checkRows(t, packageRows("kompass=0.8.0\n", map[string]string{}), map[string]string{
		"automatika_ros_sugar":       "",
		"automatika_embodied_agents": "",
		"kompass":                    "unknown",
		"kompass_interfaces":         "",
		"emos_mapping":               "",
		"kompass-core":               "",
	})
}
