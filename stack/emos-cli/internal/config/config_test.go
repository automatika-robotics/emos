package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// withTempConfig points the package-level path globals at a fresh tmp dir
// and returns the path of the would-be config.json. Restores originals on
// test cleanup so tests don't leak state into one another.
func withTempConfig(t *testing.T) string {
	t.Helper()
	origHome, origDir, origRecipes, origLogs, origLicense, origCfg :=
		HomeDir, ConfigDir, RecipesDir, LogsDir, LicenseFile, ConfigFile

	tmp := t.TempDir()
	HomeDir = tmp
	ConfigDir = filepath.Join(tmp, ".config", "emos")
	RecipesDir = filepath.Join(tmp, "emos", "recipes")
	LogsDir = filepath.Join(tmp, "emos", "logs")
	LicenseFile = filepath.Join(ConfigDir, "license.key")
	ConfigFile = filepath.Join(ConfigDir, "config.json")

	t.Cleanup(func() {
		HomeDir, ConfigDir, RecipesDir, LogsDir, LicenseFile, ConfigFile =
			origHome, origDir, origRecipes, origLogs, origLicense, origCfg
	})
	return ConfigFile
}

func TestSaveLoadRoundTrip(t *testing.T) {
	withTempConfig(t)

	want := &EMOSConfig{
		Mode:       ModeNative,
		Name:       "epic-otter",
		Port:       9000,
		LicenseKey: "lic-123",
		ROSDistro:  "jazzy",
		Auth: AuthState{
			PairingCodeHash: "abc",
			PairingCreated:  time.Now().UTC().Truncate(time.Second),
			Tokens: []AuthToken{
				{Hash: "h1", IssuedAt: time.Now().UTC().Truncate(time.Second), ExpiresAt: time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second), Label: "phone"},
			},
		},
	}
	if err := SaveConfig(want); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	got := LoadConfig()
	if got == nil {
		t.Fatalf("LoadConfig: got nil")
	}
	if got.Mode != want.Mode || got.Name != want.Name || got.Port != want.Port ||
		got.LicenseKey != want.LicenseKey || got.ROSDistro != want.ROSDistro {
		t.Fatalf("scalar fields mismatch: got=%+v want=%+v", got, want)
	}
	if got.Auth.PairingCodeHash != want.Auth.PairingCodeHash {
		t.Fatalf("auth pairing hash mismatch: got=%q want=%q", got.Auth.PairingCodeHash, want.Auth.PairingCodeHash)
	}
	if len(got.Auth.Tokens) != 1 || got.Auth.Tokens[0].Hash != "h1" || got.Auth.Tokens[0].Label != "phone" {
		t.Fatalf("tokens not preserved: got=%+v", got.Auth.Tokens)
	}
}

func TestSaveConfigUsesRestrictivePermissions(t *testing.T) {
	withTempConfig(t)
	if err := SaveConfig(&EMOSConfig{Mode: ModeNative}); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	st, err := os.Stat(ConfigFile)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	// The file holds license keys + hashed tokens; world-readable would be a
	// regression worth catching.
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("config.json mode = %v, want 0600", st.Mode().Perm())
	}
}

func TestLoadConfigLegacyLicenseMigration(t *testing.T) {
	withTempConfig(t)

	if err := os.MkdirAll(ConfigDir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(LicenseFile, []byte("legacy-key"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg := LoadConfig()
	if cfg == nil {
		t.Fatalf("LoadConfig: nil after legacy license migration")
	}
	if cfg.Mode != ModeLicensed {
		t.Fatalf("Mode = %q, want %q", cfg.Mode, ModeLicensed)
	}
	if cfg.LicenseKey != "legacy-key" {
		t.Fatalf("LicenseKey = %q, want %q", cfg.LicenseKey, "legacy-key")
	}
	// Migration must persist a config.json so subsequent loads short-circuit.
	if _, err := os.Stat(ConfigFile); err != nil {
		t.Fatalf("expected config.json to be written by migration: %v", err)
	}
}

func TestLoadConfigMissingReturnsNil(t *testing.T) {
	withTempConfig(t)
	if got := LoadConfig(); got != nil {
		t.Fatalf("LoadConfig with no files = %+v, want nil", got)
	}
}

func TestDashboardPort(t *testing.T) {
	withTempConfig(t)

	// Nil config → default.
	if got := DashboardPort(); got != DefaultDashboardPort {
		t.Fatalf("DashboardPort with no config = %d, want %d", got, DefaultDashboardPort)
	}

	// Config with Port=0 → default.
	if err := SaveConfig(&EMOSConfig{Mode: ModeNative}); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	if got := DashboardPort(); got != DefaultDashboardPort {
		t.Fatalf("DashboardPort with Port=0 = %d, want %d", got, DefaultDashboardPort)
	}

	// Configured port wins.
	if err := SaveConfig(&EMOSConfig{Mode: ModeNative, Port: 9123}); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	if got := DashboardPort(); got != 9123 {
		t.Fatalf("DashboardPort = %d, want 9123", got)
	}
}

func TestResolveDeviceNameGeneratesAndPersists(t *testing.T) {
	withTempConfig(t)

	got, err := ResolveDeviceName()
	if err != nil {
		t.Fatalf("ResolveDeviceName: %v", err)
	}
	if got == "" {
		t.Fatalf("ResolveDeviceName: empty name")
	}

	// Subsequent calls must return the same name without re-rolling.
	again, err := ResolveDeviceName()
	if err != nil {
		t.Fatalf("ResolveDeviceName (2nd): %v", err)
	}
	if again != got {
		t.Fatalf("ResolveDeviceName not idempotent: first=%q second=%q", got, again)
	}

	// And it must be persisted to disk.
	cfg := LoadConfig()
	if cfg == nil || cfg.Name != got {
		t.Fatalf("name not persisted: cfg=%+v want Name=%q", cfg, got)
	}
}

func TestResolveDeviceNameReadsExistingName(t *testing.T) {
	withTempConfig(t)

	if err := SaveConfig(&EMOSConfig{Mode: ModeNative, Name: "preset-name"}); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	got, err := ResolveDeviceName()
	if err != nil {
		t.Fatalf("ResolveDeviceName: %v", err)
	}
	if got != "preset-name" {
		t.Fatalf("ResolveDeviceName = %q, want %q", got, "preset-name")
	}
}

func TestSetDeviceNameValidatesAndPersists(t *testing.T) {
	withTempConfig(t)

	if err := SetDeviceName("NotValid Name"); err == nil {
		t.Fatalf("SetDeviceName accepted invalid input")
	}

	if err := SetDeviceName("happy-robot"); err != nil {
		t.Fatalf("SetDeviceName(valid): %v", err)
	}
	cfg := LoadConfig()
	if cfg == nil || cfg.Name != "happy-robot" {
		t.Fatalf("SetDeviceName did not persist: cfg=%+v", cfg)
	}
}

func TestPairedDeviceCount(t *testing.T) {
	var nilCfg *EMOSConfig
	if got := nilCfg.PairedDeviceCount(); got != 0 {
		t.Fatalf("nil PairedDeviceCount = %d, want 0", got)
	}
	cfg := &EMOSConfig{}
	if got := cfg.PairedDeviceCount(); got != 0 {
		t.Fatalf("empty PairedDeviceCount = %d, want 0", got)
	}
	cfg.Auth.Tokens = []AuthToken{{Hash: "a"}, {Hash: "b"}}
	if got := cfg.PairedDeviceCount(); got != 2 {
		t.Fatalf("PairedDeviceCount = %d, want 2", got)
	}
}
