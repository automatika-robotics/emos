package cmd

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/automatika-robotics/emos-cli/internal/config"
)

// withLicenseDir points the licence file at a temporary directory.
func withLicenseDir(t *testing.T) {
	t.Helper()
	origDir, origLic := config.ConfigDir, config.LicenseFile
	t.Cleanup(func() { config.ConfigDir, config.LicenseFile = origDir, origLic })
	config.ConfigDir = filepath.Join(t.TempDir(), ".config", "emos")
	config.LicenseFile = filepath.Join(config.ConfigDir, "license.json")
}

// captureStdout returns what fn prints. The pipe it prints into is not a terminal.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = orig
	out, _ := io.ReadAll(r)
	return string(out)
}

func TestLicensedLineNamesTheHolderAndTheRobot(t *testing.T) {
	lic := &config.License{PluginSlug: "emos-plugin-lite3", PluginName: "DeepRobotics Lite3", ClientName: "ACME Security GmbH"}
	if got, want := licensedLine(lic), "Licensed to ACME Security GmbH for DeepRobotics Lite3"; got != want {
		t.Errorf("licensedLine = %q, want %q", got, want)
	}
	// A licence without a client name, and a robot the catalog gave no name for
	lic = &config.License{PluginSlug: "emos-plugin-m20"}
	if got, want := licensedLine(lic), "Licensed for emos-plugin-m20"; got != want {
		t.Errorf("licensedLine = %q, want %q", got, want)
	}
}

// The nudge is for a person at a terminal. Piped output and service logs stay clean.
func TestLicenseNudgeStaysOutOfPipedOutput(t *testing.T) {
	withLicenseDir(t)
	if out := captureStdout(t, licenseNudge); out != "" {
		t.Errorf("nudge printed into a pipe: %q", out)
	}
	out := captureStdout(t, statusLicense)
	if !strings.Contains(out, "License: none") || strings.Contains(out, config.SalesEmail) {
		t.Errorf("status without a terminal = %q, want the fact and no nudge", out)
	}
}

func TestStatusShowsTheKeptLicenseWithoutTheKey(t *testing.T) {
	withLicenseDir(t)
	lic := &config.License{Key: "ABCDE-FGHJK-LMNPQ-RSTUV", PluginSlug: "emos-plugin-lite3", PluginName: "DeepRobotics Lite3", ClientName: "ACME Security GmbH", Tier: "pro"}
	if err := config.SaveLicense(lic); err != nil {
		t.Fatal(err)
	}
	out := captureStdout(t, statusLicense)
	for _, want := range []string{"ACME Security GmbH", "DeepRobotics Lite3", "Pro", config.SupportURL, "****STUV"} {
		if !strings.Contains(out, want) {
			t.Errorf("status is missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "ABCDE") || strings.Contains(out, config.SalesEmail) {
		t.Errorf("status must not show the full key or a nudge to a licence holder:\n%s", out)
	}
}
