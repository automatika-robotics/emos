// Package mapping drives building a map of the robot's environment.
//
// Mapping needs the active plugin's *knowledge*. The declaration is read
// from the describe() tree cached at install time.
//
// Two providers, one command surface. Which one a robot has is the plugin's to
// say, and a robot may have neither.
package mapping

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/automatika-robotics/emos-cli/internal/config"
)

// Kind discriminates the two providers. It is the "kind" field sugarcoat stamps
// on the mapping block.
type Kind string

const (
	// KindVendor -- the robot ships its own SLAM and we run its tool.
	KindVendor Kind = "vendor"
	// KindNative -- EMOS builds the map from the plugin's sensor feedbacks.
	KindNative Kind = "native"
)

// Vendor is mapping performed by the robot's own software.
//
// The command fields are argv templates. `{name}` is substituted with the map
// name.
type Vendor struct {
	Start []string `json:"start"`
	Stop  []string `json:"stop"`
	// StopRetries is how many extra times to re-issue Stop if no map appears.
	StopRetries int      `json:"stop_retries"`
	Store       string   `json:"store"`
	Grid        string   `json:"grid"`
	Cloud       string   `json:"cloud"`
	Apply       []string `json:"apply"`
	AfterApply  []string `json:"after_apply"`
	Export      []string `json:"export"`
	ActiveLink  string   `json:"active_link"`
	// RequiresRoot gates the dashboard. It must send the operator to the CLI
	RequiresRoot bool    `json:"requires_root"`
	Host         string  `json:"host"`
	AreaLimitM   float64 `json:"area_limit_m"`
}

// Native is mapping performed by EMOS from the plugin's own feedbacks.
//
// Cloud and IMU name *feedback keys*, not topics. The plugin stays the single
// source of truth for the topic behind each.
type Native struct {
	Cloud      string  `json:"cloud"`
	IMU        string  `json:"imu"`
	ZMin       float64 `json:"z_min"`
	ZMax       float64 `json:"z_max"`
	Resolution float64 `json:"resolution"`
}

// Declaration is how the active robot maps its environment. Exactly one of
// Vendor or Native is set, matching Kind.
type Declaration struct {
	Kind   Kind
	Vendor *Vendor
	Native *Native
}

// UnmarshalJSON decodes a mapping block by its "kind" tag.
//
// The two providers share field *names* with different meanings. A vendor's
// "cloud" is a filename inside a map directory, a native one's is a feedback
// key.
func (d *Declaration) UnmarshalJSON(data []byte) error {
	var probe struct {
		Kind Kind `json:"kind"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return err
	}
	d.Kind = probe.Kind
	switch probe.Kind {
	case KindVendor:
		d.Vendor = &Vendor{}
		return json.Unmarshal(data, d.Vendor)
	case KindNative:
		d.Native = &Native{}
		return json.Unmarshal(data, d.Native)
	default:
		return fmt.Errorf("unknown mapping kind %q", probe.Kind)
	}
}

// ErrNoPlugin is returned when no robot plugin is installed, so there is no
// robot to map with.
var ErrNoPlugin = fmt.Errorf("no robot plugin is installed")

// ErrNotSupported is returned when a robot plugin is installed but declares no
// mapping capability.
var ErrNotSupported = fmt.Errorf("this robot's plugin declares no mapping support")

// Resolve returns how the installed robot maps, read from the describe() tree
// cached when the plugin was installed.
//
// A plugin installed before mapping declarations existed has no "mapping" key
// at all, `emos plugin update` refreshes the cache.
func Resolve(cfg *config.EMOSConfig) (*Declaration, error) {
	if cfg == nil || cfg.Plugin == nil {
		return nil, ErrNoPlugin
	}
	if len(cfg.Plugin.Describe) == 0 {
		return nil, ErrNotSupported
	}
	var tree struct {
		Mapping json.RawMessage `json:"mapping"`
	}
	if err := json.Unmarshal(cfg.Plugin.Describe, &tree); err != nil {
		return nil, fmt.Errorf("read plugin description: %w", err)
	}
	if len(tree.Mapping) == 0 || string(tree.Mapping) == "null" {
		return nil, ErrNotSupported
	}
	var decl Declaration
	if err := json.Unmarshal(tree.Mapping, &decl); err != nil {
		return nil, fmt.Errorf("read mapping declaration: %w", err)
	}
	return &decl, nil
}

// Render substitutes the map name into an argv template.
//
// Templates are argv lists rather than shell strings.
func Render(argv []string, name string) []string {
	out := make([]string, 0, len(argv))
	for _, token := range argv {
		out = append(out, strings.ReplaceAll(token, "{name}", name))
	}
	return out
}
