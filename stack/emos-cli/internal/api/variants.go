package api

import (
	"errors"
	"slices"

	"github.com/automatika-robotics/emos-cli/internal/config"
)

// VariantChoice is the variant of a recipe a machine gets.
type VariantChoice struct {
	Variant RecipeVariant
	// Unlicensed is the robot a fitting variant exists for, when the generic
	// variant was chosen because the license does not cover that robot.
	Unlicensed string
}

// ErrNotForThisRobot means a recipe has neither a generic variant nor one for
// the installed robot and its sensors.
var ErrNotForThisRobot = errors.New("this recipe is not available for your robot")

// NeedsLicenseError means a recipe only comes in a variant for Robot, and the
// license on this machine does not cover that robot.
type NeedsLicenseError struct{ Robot string }

func (e *NeedsLicenseError) Error() string { return "this recipe needs a license for " + e.Robot }

// ChooseVariant picks the variant of r for a machine with the robot plugin
// robot, the sensor plugins sensors and the license lic; robot may be empty
// and lic nil.
//
// The robot's variant with the most sensors wins, if its sensors are all
// installed and the license is for that robot. Otherwise the generic one.
func ChooseVariant(r Recipe, robot string, sensors []string, lic *config.License) (VariantChoice, error) {
	var fitting, generic *RecipeVariant
	for i := range r.Variants {
		v := &r.Variants[i]
		switch {
		case v.ID == GenericVariant:
			generic = v
		case robot == "" || v.Robot != robot || !containsAll(sensors, v.Sensors):
		case fitting == nil || len(v.Sensors) > len(fitting.Sensors):
			fitting = v
		}
	}

	licensed := lic != nil && lic.PluginSlug == robot
	switch {
	case fitting != nil && licensed:
		return VariantChoice{Variant: *fitting}, nil
	case generic != nil && fitting != nil:
		return VariantChoice{Variant: *generic, Unlicensed: robot}, nil
	case generic != nil:
		return VariantChoice{Variant: *generic}, nil
	case fitting != nil:
		return VariantChoice{}, &NeedsLicenseError{Robot: robot}
	}
	return VariantChoice{}, ErrNotForThisRobot
}

func containsAll(have, want []string) bool {
	for _, w := range want {
		if !slices.Contains(have, w) {
			return false
		}
	}
	return true
}
