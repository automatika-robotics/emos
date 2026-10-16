package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/automatika-robotics/emos-cli/internal/api"
	"github.com/automatika-robotics/emos-cli/internal/config"
)

var lite3Only = api.Recipe{Filename: "patrol", Variants: []api.RecipeVariant{{ID: "emos-plugin-lite3", Robot: "emos-plugin-lite3"}}}

func lite3Config() *config.EMOSConfig {
	return &config.EMOSConfig{Plugin: &config.PluginInfo{Slug: "emos-plugin-lite3",
		Describe: json.RawMessage(`{"metadata": {"name": "Lite3"}}`)}}
}

func TestChooseVariantExplainsWhyThereIsNone(t *testing.T) {
	t.Cleanup(func() { pullGeneric = false })
	withLicenseDir(t)

	var err error
	out := captureStdout(t, func() { _, err = chooseVariant(lite3Only, lite3Config(), nil) })
	if err == nil || !strings.Contains(out, "version for the Lite3 only") || !strings.Contains(out, "emos license activate") {
		t.Errorf("robot-only recipe without a license: err = %v, said:\n%s", err, out)
	}

	m20License := &config.License{Key: "K", PluginSlug: "emos-plugin-m20", PluginName: "DeepRobotics M20"}
	out = captureStdout(t, func() { _, err = chooseVariant(lite3Only, lite3Config(), m20License) })
	if err == nil || !strings.Contains(out, "is for the DeepRobotics M20") || strings.Contains(out, "activate") {
		t.Errorf("license for another robot: err = %v, said:\n%s", err, out)
	}

	out = captureStdout(t, func() { _, err = chooseVariant(lite3Only, nil, nil) })
	if err == nil || !strings.Contains(out, "not available for your robot") {
		t.Errorf("no robot installed: err = %v, said:\n%s", err, out)
	}
}

func TestPullGenericTakesTheGenericVersionOrSaysThereIsNone(t *testing.T) {
	t.Cleanup(func() { pullGeneric = false })
	pullGeneric = true

	both := api.Recipe{Variants: []api.RecipeVariant{{ID: api.GenericVariant}, {ID: "emos-plugin-lite3", Robot: "emos-plugin-lite3"}}}
	lic := &config.License{Key: "K", PluginSlug: "emos-plugin-lite3"}
	choice, err := chooseVariant(both, lite3Config(), lic)
	if err != nil || choice.Variant.ID != api.GenericVariant {
		t.Errorf("--generic on a licensed robot = %q, %v", choice.Variant.ID, err)
	}

	var out string
	out = captureStdout(t, func() { _, err = chooseVariant(lite3Only, lite3Config(), lic) })
	if err == nil || !strings.Contains(out, "no generic version") {
		t.Errorf("--generic without a generic version: err = %v, said:\n%s", err, out)
	}
}
