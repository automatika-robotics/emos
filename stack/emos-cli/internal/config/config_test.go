package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
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
	LicenseFile = filepath.Join(ConfigDir, "license.json")
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
		Mode:      ModeNative,
		Name:      "epic-otter",
		Port:      9000,
		ROSDistro: "jazzy",
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
		got.ROSDistro != want.ROSDistro {
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

func TestSaveConfigLeavesNoTempFiles(t *testing.T) {
	withTempConfig(t)
	for i := 0; i < 3; i++ {
		if err := SaveConfig(&EMOSConfig{Mode: ModeNative, Port: i}); err != nil {
			t.Fatalf("SaveConfig: %v", err)
		}
	}
	entries, err := os.ReadDir(ConfigDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "config.json" {
			t.Errorf("unexpected file left behind: %s", e.Name())
		}
	}
}

func TestUpdateConfigKeepsWhatOthersWrote(t *testing.T) {
	withTempConfig(t)
	if err := SaveConfig(&EMOSConfig{Mode: ModePixi}); err != nil {
		t.Fatal(err)
	}
	// A pairing lands on disk while a long plugin job still holds an old copy.
	stale := LoadConfig()
	if err := UpdateConfig(func(c *EMOSConfig) { c.Auth.PairingCodeHash = "paired" }); err != nil {
		t.Fatal(err)
	}
	stale.Plugin = &PluginInfo{Slug: "m20_plugin"}
	if err := UpdateConfig(func(c *EMOSConfig) { c.Plugin = stale.Plugin }); err != nil {
		t.Fatal(err)
	}
	got := LoadConfig()
	if got.Auth.PairingCodeHash != "paired" || got.Plugin == nil || got.Mode != ModePixi {
		t.Errorf("an update lost another writer's change: %+v", got)
	}
}

func TestUpdateConfigSerialisesConcurrentWriters(t *testing.T) {
	withTempConfig(t)
	const writers = 20
	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			err := UpdateConfig(func(c *EMOSConfig) {
				c.UpsertSensor(PluginInfo{Slug: fmt.Sprintf("sensor_%d", i)})
			})
			if err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	if got := len(LoadConfig().SensorPlugins); got != writers {
		t.Errorf("%d of %d concurrent updates survived", got, writers)
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

func TestChannelFollowsTheVersion(t *testing.T) {
	orig := Version
	t.Cleanup(func() { Version = orig })
	for version, want := range map[string]string{
		"0.8.0": "stable", "dev": "stable", "0.8.0-dev.20260925": "dev", "0.8.1-dev.20261001.2": "dev",
	} {
		Version = version
		if got := Channel(); got != want {
			t.Errorf("Channel() with version %q = %q, want %q", version, got, want)
		}
	}
}

func TestPublicImageTagFollowsTheChannel(t *testing.T) {
	orig := Version
	t.Cleanup(func() { Version = orig })
	Version = "0.8.0"
	if got := PublicImageTag("jazzy"); got != PublicImage+":jazzy-latest" {
		t.Errorf("stable image = %q", got)
	}
	Version = "0.8.0-dev.20260925"
	if got := PublicImageTag("jazzy"); got != PublicImage+":jazzy-dev" {
		t.Errorf("dev image = %q", got)
	}
}

func TestWorkspaceRefIsTheSourceRefOrMain(t *testing.T) {
	orig := SourceRef
	t.Cleanup(func() { SourceRef = orig })
	SourceRef = ""
	if got := WorkspaceRef(); got != "main" {
		t.Errorf("WorkspaceRef() without a source ref = %q", got)
	}
	SourceRef = "v0.8.0-dev.20260925"
	if got := WorkspaceRef(); got != "v0.8.0-dev.20260925" {
		t.Errorf("WorkspaceRef() with a source ref = %q", got)
	}
}
