package installer

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestSyncWorkspaceMovesBetweenATagAndABranch(t *testing.T) {
	origin := filepath.Join(t.TempDir(), "origin")
	git(t, t.TempDir(), "init", "-q", "-b", "main", origin)
	git(t, origin, "commit", "-q", "--allow-empty", "-m", "release")
	git(t, origin, "tag", "v0.8.0-dev.20260925")
	nightly := git(t, origin, "rev-parse", "HEAD")
	git(t, origin, "commit", "-q", "--allow-empty", "-m", "after")
	tip := git(t, origin, "rev-parse", "HEAD")

	ws := filepath.Join(t.TempDir(), "ws")
	git(t, t.TempDir(), "clone", "-q", "--depth", "1", "-b", "main", "file://"+origin, ws)

	if err := SyncWorkspace(ws, "v0.8.0-dev.20260925"); err != nil {
		t.Fatalf("sync to the tag: %v", err)
	}
	if got := git(t, ws, "rev-parse", "HEAD"); got != nightly {
		t.Errorf("after the tag HEAD = %s, want %s", got, nightly)
	}
	if err := SyncWorkspace(ws, "main"); err != nil {
		t.Fatalf("sync back to main: %v", err)
	}
	if got := git(t, ws, "rev-parse", "HEAD"); got != tip {
		t.Errorf("after main HEAD = %s, want %s", got, tip)
	}
}
