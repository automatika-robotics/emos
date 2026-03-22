package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Version is set at build time via -ldflags from the Makefile.
// The "dev" fallback only appears when built with plain `go build` (without make).
var Version = "dev"

type InstallMode string

const (
	ModeOSSContainer InstallMode = "oss-container"
	ModeLicensed     InstallMode = "licensed"
	ModeNative       InstallMode = "native"
	ModePixi         InstallMode = "pixi"
)

type EMOSConfig struct {
	Mode           InstallMode `json:"mode"`
	LicenseKey     string      `json:"license_key,omitempty"`
	ROSDistro      string      `json:"ros_distro"`
	ImageTag       string      `json:"image_tag,omitempty"`
	WorkspacePath  string      `json:"workspace_path,omitempty"`
	PixiProjectDir string      `json:"pixi_project_dir,omitempty"`
}

const (
	ContainerName = "emos"
	ServiceName   = "emos.service"
	GitHubOrg     = "automatika-robotics"
	GitHubRepo    = "emos"
	PublicImage   = "ghcr.io/automatika-robotics/emos"

	// API endpoints
	APIBaseURL          = "https://support-api.automatikarobotics.com/api"
	CredentialsEndpoint = APIBaseURL + "/registrations/credentials"
	RecipesEndpoint     = APIBaseURL + "/recipes"
)

var (
	HomeDir     string
	ConfigDir   string
	RecipesDir  string
	LogsDir     string
	LicenseFile string
	ConfigFile  string
)

func Init() {
	HomeDir, _ = os.UserHomeDir()
	ConfigDir = filepath.Join(HomeDir, ".config", "emos")
	RecipesDir = filepath.Join(HomeDir, "emos", "recipes")
	LogsDir = filepath.Join(HomeDir, "emos", "logs")
	LicenseFile = filepath.Join(ConfigDir, "license.key")
	ConfigFile = filepath.Join(ConfigDir, "config.json")
}

// PublicImageTag returns the full public image reference for a given ROS distro.
func PublicImageTag(distro string) string {
	return PublicImage + ":" + distro + "-latest"
}

// LoadConfig loads the persistent EMOS config. If config.json is missing but
// license.key exists, it infers licensed mode and migrates.
func LoadConfig() *EMOSConfig {
	data, err := os.ReadFile(ConfigFile)
	if err == nil {
		var cfg EMOSConfig
		if json.Unmarshal(data, &cfg) == nil {
			return &cfg
		}
	}

	// Backward compat: if license.key exists, infer licensed mode
	if keyBytes, err := os.ReadFile(LicenseFile); err == nil && len(keyBytes) > 0 {
		cfg := &EMOSConfig{
			Mode:       ModeLicensed,
			LicenseKey: string(keyBytes),
			ROSDistro:  "jazzy",
		}
		SaveConfig(cfg)
		return cfg
	}

	return nil
}

// SaveConfig persists the EMOS config to disk.
func SaveConfig(cfg *EMOSConfig) error {
	os.MkdirAll(ConfigDir, 0755)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ConfigFile, data, 0644)
}

func InstallerURL() string {
	return "https://raw.githubusercontent.com/" + GitHubOrg + "/" + GitHubRepo + "/main/stack/emos-cli/scripts/install.sh"
}

func ReleasesURL() string {
	return "https://api.github.com/repos/" + GitHubOrg + "/" + GitHubRepo + "/releases/latest"
}
