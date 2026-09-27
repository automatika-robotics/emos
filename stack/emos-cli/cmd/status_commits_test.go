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
	sub := gitRepo(t)
	if err := os.Rename(sub, filepath.Join(ws, "stack", "kompass")); err != nil {
		t.Fatal(err)
	}
	rows := workspaceCommits(ws)
	if len(rows) != 1+len(stackPackages) || rows[0][0] != "emos" || len(rows[0][1]) != 7 {
		t.Fatalf("unexpected rows %v", rows)
	}
	for _, row := range rows[1:] {
		switch row[0] {
		case "kompass":
			if len(row[1]) != 7 {
				t.Errorf("kompass commit not read: %v", row)
			}
		default:
			if row[1] != "unknown" {
				t.Errorf("a missing package should read unknown: %v", row)
			}
		}
	}
}

func TestStackLabelParsesIntoRows(t *testing.T) {
	rows := parseStackLabel("sugarcoat@f5ab9cc kompass@5edb164abc embodied-agents@1b56512")
	if len(rows) != 3 || rows[1][0] != "kompass" || rows[1][1] != "5edb164" || rows[2][1] != "1b56512" {
		t.Fatalf("unexpected rows %v", rows)
	}
	if parseStackLabel("") != nil {
		t.Fatal("an empty label should give no rows")
	}
}
