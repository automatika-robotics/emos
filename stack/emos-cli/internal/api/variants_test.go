package api

import (
	"errors"
	"testing"

	"github.com/automatika-robotics/emos-cli/internal/config"
)

const (
	lite3 = "emos-plugin-lite3"
	m20   = "emos-plugin-m20"
	hik   = "emos-plugin-hikvision"
)

var (
	generic   = RecipeVariant{ID: GenericVariant}
	forLite3  = RecipeVariant{ID: lite3, Robot: lite3}
	forM20    = RecipeVariant{ID: m20, Robot: m20}
	lite3Hik  = RecipeVariant{ID: lite3 + "+" + hik, Robot: lite3, Sensors: []string{hik}}
	everyKind = Recipe{Filename: "vision_follower", Variants: []RecipeVariant{generic, forLite3, forM20, lite3Hik}}
	robotOnly = Recipe{Filename: "patrol", Variants: []RecipeVariant{forLite3, lite3Hik}}
)

func license(robot string) *config.License { return &config.License{Key: "K", PluginSlug: robot} }

func TestChooseVariant(t *testing.T) {
	for name, c := range map[string]struct {
		recipe     Recipe
		robot      string
		sensors    []string
		lic        *config.License
		want       string
		unlicensed string
	}{
		"licensed robot gets its variant":               {everyKind, lite3, nil, license(lite3), lite3, ""},
		"with the sensor installed, the fuller variant": {everyKind, lite3, []string{hik}, license(lite3), lite3 + "+" + hik, ""},
		"no robot plugin gets the generic one":          {everyKind, "", nil, nil, GenericVariant, ""},
		"another robot's variants are ignored":          {Recipe{Variants: []RecipeVariant{generic, forM20}}, lite3, nil, license(lite3), GenericVariant, ""},
		"no license falls back and says so":             {everyKind, lite3, nil, nil, GenericVariant, lite3},
		"a license for another robot falls back too":    {everyKind, lite3, nil, license(m20), GenericVariant, lite3},
	} {
		got, err := ChooseVariant(c.recipe, c.robot, c.sensors, c.lic)
		if err != nil || got.Variant.ID != c.want || got.Unlicensed != c.unlicensed {
			t.Errorf("%s: got %q (unlicensed %q), %v; want %q (unlicensed %q)", name, got.Variant.ID, got.Unlicensed, err, c.want, c.unlicensed)
		}
	}
}

func TestChooseVariantWithNothingToFallBackOn(t *testing.T) {
	// Only robot variants, and the license does not cover the robot
	_, err := ChooseVariant(robotOnly, lite3, nil, nil)
	var needs *NeedsLicenseError
	if !errors.As(err, &needs) || needs.Robot != lite3 {
		t.Errorf("unlicensed robot-only recipe = %v, want NeedsLicenseError for %s", err, lite3)
	}
	// Only robot variants, for a robot that is not this one
	if _, err := ChooseVariant(robotOnly, m20, nil, license(m20)); !errors.Is(err, ErrNotForThisRobot) {
		t.Errorf("recipe for another robot = %v, want ErrNotForThisRobot", err)
	}
	// The only variant for this robot needs a sensor that is not installed
	sensorOnly := Recipe{Variants: []RecipeVariant{lite3Hik}}
	if _, err := ChooseVariant(sensorOnly, lite3, nil, license(lite3)); !errors.Is(err, ErrNotForThisRobot) {
		t.Errorf("variant needing a missing sensor = %v, want ErrNotForThisRobot", err)
	}
}
