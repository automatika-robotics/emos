package config

import (
	"os"
	"reflect"
	"testing"
	"time"
)

func TestLicenseRoundTripIsPrivateAndApartFromTheConfig(t *testing.T) {
	withTempConfig(t)
	if LoadLicense() != nil {
		t.Fatal("a fresh machine has no licence")
	}

	claimed := time.Date(2026, 9, 21, 10, 42, 7, 0, time.UTC)
	want := &License{
		Key: "ABCDE-FGHJK-LMNPQ-RSTUV", PluginSlug: "emos-plugin-lite3", PluginName: "DeepRobotics Lite3",
		ClientName: "ACME Security GmbH", SerialNumber: "L3-2026-00417", Tier: "pro",
		ClaimedAt:  &claimed,
		VerifiedAt: time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC),
	}
	if err := SaveLicense(want); err != nil {
		t.Fatalf("SaveLicense: %v", err)
	}
	if got := LoadLicense(); !reflect.DeepEqual(got, want) {
		t.Errorf("LoadLicense = %+v, want %+v", got, want)
	}

	info, err := os.Stat(LicenseFile)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("licence file mode = %o, want 600: it holds the key", perm)
	}
	// An uninstall removes the config; the licence is a file of its own.
	if _, err := os.Stat(ConfigFile); !os.IsNotExist(err) {
		t.Errorf("saving a licence must not create the config, stat err = %v", err)
	}
}

func TestLoadLicenseIgnoresAFileItCannotUse(t *testing.T) {
	withTempConfig(t)
	if err := os.MkdirAll(ConfigDir, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"not json":      "ABCDE-FGHJK-LMNPQ-RSTUV",
		"without a key": `{"plugin_slug": "emos-plugin-lite3"}`,
	} {
		if err := os.WriteFile(LicenseFile, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if lic := LoadLicense(); lic != nil {
			t.Errorf("%s: LoadLicense = %+v, want nil", name, lic)
		}
	}
}
