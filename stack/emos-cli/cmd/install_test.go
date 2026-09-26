package cmd

import (
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/automatika-robotics/emos-cli/internal/config"
)

func TestPixiCloneArgsFollowTheRefTheBinaryWasBuiltFrom(t *testing.T) {
	url, dir := "https://github.com/automatika-robotics/emos.git", "/home/robot/.local/share/emos"

	// A release binary carries no ref and clones the default branch.
	want := []string{"clone", "--depth", "1", url, dir}
	if got := pixiCloneArgs("", url, dir); !slices.Equal(got, want) {
		t.Errorf("no ref: %v, want %v", got, want)
	}
	// A development build installs the branch it was built from.
	want = []string{"clone", "--depth", "1", "--branch", "release/0.8.0", url, dir}
	if got := pixiCloneArgs("release/0.8.0", url, dir); !slices.Equal(got, want) {
		t.Errorf("with a ref: %v, want %v", got, want)
	}
}

func TestChooseInstallFollowsTheModeFlag(t *testing.T) {
	t.Cleanup(func() { installModeFlag = "" })
	same := func(a, b func() error) bool { return reflect.ValueOf(a).Pointer() == reflect.ValueOf(b).Pointer() }
	for flag, want := range map[string]func() error{
		"container": installOSSContainer, "oss-container": installOSSContainer,
		"native": installNative, "pixi": installPixi,
	} {
		installModeFlag = flag
		got, err := chooseInstall()
		if err != nil || !same(got, want) {
			t.Errorf("--mode %s: wrong installer, err = %v", flag, err)
		}
	}
	// A licence is no longer an install mode
	installModeFlag = "licensed"
	if _, err := chooseInstall(); err == nil {
		t.Error("--mode licensed must be refused")
	}
}

func TestInstallLicenseUsesTheKeptLicenseWithoutThePortal(t *testing.T) {
	origDir, origLic := config.ConfigDir, config.LicenseFile
	t.Cleanup(func() { config.ConfigDir, config.LicenseFile = origDir, origLic })
	config.ConfigDir = filepath.Join(t.TempDir(), ".config", "emos")
	config.LicenseFile = filepath.Join(config.ConfigDir, "license.json")

	// No key, no kept licence and nobody to ask (a test has no terminal): a free install
	if lic, err := installLicense(nil); lic != nil || err != nil {
		t.Fatalf("no licence anywhere = %+v, %v, want a free install", lic, err)
	}

	kept := &config.License{Key: "ABCDE-FGHJK-LMNPQ-RSTUV", PluginSlug: "emos-plugin-lite3", PluginName: "DeepRobotics Lite3"}
	if err := config.SaveLicense(kept); err != nil {
		t.Fatal(err)
	}
	lic, err := installLicense(nil)
	if err != nil || lic == nil || lic.Key != kept.Key || lic.PluginSlug != kept.PluginSlug {
		t.Errorf("kept licence = %+v, %v, want it back unchanged", lic, err)
	}
}
