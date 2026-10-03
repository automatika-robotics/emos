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
	"path/filepath"
	"regexp"
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

// Native is mapping performed by EMOS from the plugin's own sensors. The
// mapping session reads the rest of the declaration from the plugin itself.
type Native struct {
	Store      string `json:"store"`
	ActiveLink string `json:"active_link"`
}

// Declaration is how the active robot maps its environment. Kind says which of
// Vendor and Native is set.
type Declaration struct {
	Kind   Kind
	Vendor *Vendor
	Native *Native
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
		d.Native = &Native{}
		return json.Unmarshal(data, d.Native)
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

// ErrStopInterrupted is returned when stopping a session is cancelled before a
// map appears. A vendor's software may still be mapping; a native session has
// been killed.
var ErrStopInterrupted = errors.New("stopping mapping was interrupted")

// validName is what a map name may look like. It goes to a vendor tool that
// may run as root, so it must not read as an option or a path.
var validName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// CheckName rejects a map name the provider should not be handed.
func CheckName(name string) error {
	if !validName.MatchString(name) {
		return fmt.Errorf(
			"map name %q: use letters, digits, '.', '_' or '-', starting with a letter or digit", name)
	}
	return nil
}

// CanStart returns why this robot cannot run a mapping session, or nil.
func (d *Declaration) CanStart() error {
	if d.Kind == KindNative {
		return nil
	}
	if err := d.checkLocal(); err != nil {
		return err
	}
	if len(d.Vendor.Start) == 0 || len(d.Vendor.Stop) == 0 {
		return fmt.Errorf("this robot's plugin does not declare how to start and stop mapping")
	}
	return nil
}

// checkLocal rejects a provider whose commands run on another machine.
func (d *Declaration) checkLocal() error {
	if d.Kind == KindNative {
		return nil
	}
	if host := d.Vendor.Host; host != "" && host != "local" {
		return fmt.Errorf("this robot maps on %s; running commands there is not supported yet", host)
	}
	return nil
}

// Resolve returns how the installed robot maps, read from the describe() tree
// cached when the plugin was installed.
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

// Store returns the directory holding this provider's maps.
func (d *Declaration) Store() string {
	if d.Kind != KindNative {
		return d.Vendor.Store
	}
	// Declared relative to the home of whoever runs EMOS
	store := d.Native.Store
	switch {
	case store == "~":
		return config.HomeDir
	case strings.HasPrefix(store, "~/"):
		return filepath.Join(config.HomeDir, store[2:])
	}
	return store
}

// activeLink is the name of the link in the store that marks the active map.
func (d *Declaration) activeLink() string {
	if d.Kind != KindNative {
		return d.Vendor.ActiveLink
	}
	return d.Native.ActiveLink
}

// gridFile is the occupancy-grid YAML's filename in a map directory, "" when
// the provider names none.
func (d *Declaration) gridFile() string {
	if d.Kind != KindNative {
		return d.Vendor.Grid
	}
	return "occ_grid.yaml"
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
