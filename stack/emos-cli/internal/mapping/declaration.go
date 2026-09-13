// Package mapping drives building a map of the robot's environment.
//
// Mapping needs the active plugin's *knowledge*. The declaration is read
// from the describe() tree cached at install time.
//
// A plugin declares one of two providers: "vendor", where the robot ships its
// own SLAM tool and EMOS drives it, or "native", where EMOS builds the map
// itself.
package mapping

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/automatika-robotics/emos-cli/internal/config"
)

// Kind discriminates the providers. It is the "kind" field sugarcoat stamps on
// the mapping block.
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
	// ExportDir is where Export leaves its archive when the vendor picks the
	// path itself. Empty means unknown.
	ExportDir string `json:"export_dir"`
	// Import unpacks an archive into the store. {path} is the archive.
	Import []string `json:"import"`
	// Remove deletes a map. Nil where the vendor has no such command, in which
	// case the map directory is removed directly.
	Remove     []string `json:"remove"`
	ActiveLink string   `json:"active_link"`
	// RequiresRoot says some verb escalates, so a caller without a terminal for
	// a password prompt cannot drive this provider.
	RequiresRoot bool    `json:"requires_root"`
	Host         string  `json:"host"`
	AreaLimitM   float64 `json:"area_limit_m"`
}

// Declaration is how the active robot maps its environment. Vendor is set for
// every declaration Resolve returns.
type Declaration struct {
	Kind   Kind
	Vendor *Vendor
}

// UnmarshalJSON decodes a mapping block by its "kind" tag.
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
		return nil
	default:
		return fmt.Errorf("unknown mapping kind %q", probe.Kind)
	}
}

// ErrNoPlugin is returned when no robot plugin is installed, so there is no
// robot to map with.
var ErrNoPlugin = errors.New("no robot plugin is installed")

// ErrNotSupported is returned when a robot plugin is installed but declares no
// mapping capability.
var ErrNotSupported = errors.New("this robot's plugin declares no mapping support")

// ErrNativeNotSupported is returned when the plugin declares native mapping,
// which this version of EMOS does not support.
var ErrNativeNotSupported = errors.New("native mapping is not supported by this version of EMOS")

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
	if decl.Kind == KindNative {
		return nil, ErrNativeNotSupported
	}
	return &decl, nil
}

// vars are the substitutions an argv template may contain, as {key}.
type vars map[string]string

// render substitutes {key} placeholders in an argv template.
//
// One pass per token, so a substituted value is never itself re-expanded: a map
// named "{path}" stays that name rather than turning into the archive path.
func render(argv []string, subs vars) []string {
	keys := make([]string, 0, len(subs))
	for key := range subs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	pairs := make([]string, 0, 2*len(keys))
	for _, key := range keys {
		pairs = append(pairs, "{"+key+"}", subs[key])
	}
	replacer := strings.NewReplacer(pairs...)

	out := make([]string, 0, len(argv))
	for _, token := range argv {
		out = append(out, replacer.Replace(token))
	}
	return out
}
