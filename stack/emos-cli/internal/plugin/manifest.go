package plugin

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// ManifestFile is a plugin's dependency manifest, read from its repo root
// before the build.
const ManifestFile = "emos-plugin.yaml"

// Manifest is a plugin's declared build sources and install dependencies. Every
// field is optional; a plugin with no driver dependencies ships no manifest at all.
type Manifest struct {
	// Sources are repositories cloned into the workspace as sibling packages and
	// built from source alongside the plugin.
	Sources []Source `yaml:"sources"`
	// Deps are packages a fresh environment must install for the plugin and its
	// sources to build and run.
	Deps Deps `yaml:"deps"`
}

// Source is one repository to clone into the workspace and build from source.
type Source struct {
	Git       string `yaml:"git"`       // clone URL
	Ref       string `yaml:"ref"`       // tag or branch; empty = default branch
	Recursive bool   `yaml:"recursive"` // init nested submodules on clone
	Name      string `yaml:"name"`      // workspace dir name; default = repo basename
}

// Deps are a plugin's installable dependencies, grouped by ecosystem.
type Deps struct {
	// ROS names published packages for the distro, installed as ros-<distro>-*
	// on both robostack (pixi add) and apt (native/rosdep).
	ROS []string `yaml:"ros"`
	// System names non-ROS packages per package manager.
	System SystemDeps `yaml:"system"`
}

// SystemDeps names non-ROS packages per package manager.
type SystemDeps struct {
	Conda []string `yaml:"conda"` // robostack / conda-forge names (pixi add)
	Apt   []string `yaml:"apt"`   // Debian package names (native)
}

// PackageName is the workspace directory a source is cloned into.
func (s Source) PackageName() string {
	if s.Name != "" {
		return s.Name
	}
	base := strings.TrimSuffix(s.Git, "/")
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	return strings.TrimSuffix(base, ".git")
}

// LoadManifest reads a plugin's emos-plugin.yaml from dir. A missing file is not
// an error.
func LoadManifest(dir string) (*Manifest, error) {
	data, err := os.ReadFile(filepath.Join(dir, ManifestFile))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	// Reject unknown keys so any mistake in a manifest fails here
	// rather than as a silently missing package at build time.
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var m Manifest
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", ManifestFile, err)
	}
	for i, s := range m.Sources {
		if s.Git == "" {
			return nil, fmt.Errorf("%s: sources[%d] has no git URL", ManifestFile, i)
		}
		if err := checkDirName(s.PackageName()); err != nil {
			return nil, fmt.Errorf("%s: sources[%d]: %w", ManifestFile, i, err)
		}
	}
	return &m, nil
}

// dirName is what a directory created in the workspace may be called.
var dirName = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9._-]*$`)

// checkDirName rejects a name from a manifest or the catalog before it is joined
// onto a workspace path that is later deleted.
func checkDirName(name string) error {
	if !dirName.MatchString(name) {
		return fmt.Errorf("%q is not a usable directory name", name)
	}
	return nil
}
