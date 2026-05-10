package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"syscall"
	"testing"
)

// captureStdoutStderr runs fn with both stdout and stderr replaced by a
// pipe; returns whatever fn wrote to either. Goroutine-safe-enough for
// tests since the cmd package's ui helpers funnel through fmt.Printf,
// fmt.Fprintln, and friends.
func captureStdoutStderr(t *testing.T, fn func()) string {
	t.Helper()
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	origOut, origErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = wOut, wErr
	t.Cleanup(func() { os.Stdout, os.Stderr = origOut, origErr })

	done := make(chan struct{})
	var out, errb bytes.Buffer
	go func() {
		_, _ = io.Copy(&out, rOut)
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(&errb, rErr)
		done <- struct{}{}
	}()

	fn()

	wOut.Close()
	wErr.Close()
	<-done
	<-done
	return out.String() + errb.String()
}

func TestWarnIfSudo_NoSudoNoOp(t *testing.T) {
	// Whatever the actual EUID is, when SUDO_USER isn't set the helper
	// must produce no output. (This covers the typical user-shell case.)
	t.Setenv("SUDO_USER", "")
	got := captureStdoutStderr(t, warnIfSudo)
	if got != "" {
		t.Errorf("warnIfSudo emitted output without SUDO_USER: %q", got)
	}
}

func TestWarnIfSudo_RootWithSudoUserWarns(t *testing.T) {
	// We can't change EUID inside a test process, so this case only
	// fires when the test happens to be run as root with SUDO_USER set
	// (CI rarely does that). On a non-root run it's vacuously true.
	if syscall.Geteuid() != 0 {
		t.Skip("test requires EUID=0 to exercise the sudo path; skipping")
	}
	t.Setenv("SUDO_USER", "alice")
	got := captureStdoutStderr(t, warnIfSudo)
	if !strings.Contains(got, "sudo") {
		t.Errorf("expected warning mentioning sudo; got: %q", got)
	}
	if !strings.Contains(got, "alice") {
		t.Errorf("expected warning to mention SUDO_USER value; got: %q", got)
	}
}

func TestWarnIfSudo_NonRootIsNoOp(t *testing.T) {
	// EUID 0 is required to enter the warn branch; on a non-root test
	// run, even with SUDO_USER set, warnIfSudo must do nothing.
	if syscall.Geteuid() == 0 {
		t.Skip("test asserts non-root behaviour; skipping when EUID=0")
	}
	t.Setenv("SUDO_USER", "alice")
	got := captureStdoutStderr(t, warnIfSudo)
	if got != "" {
		t.Errorf("warnIfSudo emitted output as non-root: %q", got)
	}
}
