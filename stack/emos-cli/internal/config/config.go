package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/automatika-robotics/emos-cli/internal/identity"
)

// Version is set at build time via -ldflags from the Makefile.
// The "dev" fallback only appears when built with plain `go build`
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
	Name           string      `json:"name,omitempty"` // human-friendly device name (e.g. "epic-otter")
	Port           int         `json:"port,omitempty"` // dashboard bind port; 0 means DefaultDashboardPort
	LicenseKey     string      `json:"license_key,omitempty"`
	ROSDistro      string      `json:"ros_distro"`
	ImageTag       string      `json:"image_tag,omitempty"`
	WorkspacePath  string      `json:"workspace_path,omitempty"`
	PixiProjectDir string      `json:"pixi_project_dir,omitempty"`
	Auth           AuthState   `json:"auth"`
}

// DefaultDashboardPort is the bind port used when EMOSConfig.Port is unset.
const DefaultDashboardPort = 8765

// DashboardPort returns the configured dashboard port, falling back to the
// default if none is set or if the config is missing entirely.
func DashboardPort() int {
	if cfg := LoadConfig(); cfg != nil && cfg.Port != 0 {
		return cfg.Port
	}
	return DefaultDashboardPort
}

// AuthState is the dashboard's pairing + bearer-token state. It lives on
// EMOSConfig so the entire device's persistent state fits in one file
// (~/.config/emos/config.json). Plaintext secrets are never written to
// disk:
//   - PairingCodeHash holds a bcrypt hash of the 6-digit code.
//   - Tokens[].Hash holds an HMAC-SHA256 of the bearer token, keyed by
//     TokenKey. Verification recomputes the HMAC and compares.
type AuthState struct {
	PairingCodeHash string      `json:"pairing_code_hash,omitempty"`
	PairingCreated  time.Time   `json:"pairing_created,omitempty"`
	TokenKey        []byte      `json:"token_key,omitempty"` // 32-byte HMAC key, generated once
	Tokens          []AuthToken `json:"tokens,omitempty"`
}

// AuthToken is one issued bearer token's record. The plaintext token is
// returned to the browser once and then discarded; we keep only an
// HMAC-SHA256 (keyed by AuthState.TokenKey) to verify on subsequent
// requests.
type AuthToken struct {
	Hash      string    `json:"hash"`
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Label     string    `json:"label,omitempty"`
}

// PairedDeviceCount returns the number of bearer tokens currently issued
// for this device. Nil-safe so callers can chain off LoadConfig() without a
// guard.
func (c *EMOSConfig) PairedDeviceCount() int {
	if c == nil {
		return 0
	}
	return len(c.Auth.Tokens)
}

// IsInstalled reports whether this config represents a finished EMOS
// install. Mode is set by every install flow; an empty Mode means
// `emos install` has not run.
func (c *EMOSConfig) IsInstalled() bool {
	return c != nil && c.Mode != ""
}

const (
	ContainerName        = "emos"
	ServiceName          = "emos.service"           // container auto-restart unit
	DashboardServiceName = "emos-dashboard.service" // `emos serve` daemon unit
	GitHubOrg            = "automatika-robotics"
	GitHubRepo           = "emos"
	PublicImage          = "ghcr.io/automatika-robotics/emos"

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

// SaveConfig persists the EMOS config to disk. Mode 0600 because the file
// holds license keys and hashed auth tokens.
func SaveConfig(cfg *EMOSConfig) error {
	if err := os.MkdirAll(ConfigDir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ConfigFile, data, 0600)
}

// ResolveDeviceName loads the config (creating an empty one if needed),
// returns its Name if already set, otherwise computes a deterministic name
// from the device's hardware fingerprint, persists it, and returns it.
// Idempotent: subsequent calls return the same value without recomputing.
func ResolveDeviceName() (string, error) {
	cfg := LoadConfig()
	if cfg == nil {
		// No EMOS install yet, still give the daemon an identity by
		// creating a minimal config with just the name. This lets the
		// dashboard work pre-install.
		cfg = &EMOSConfig{}
	}
	if cfg.Name != "" {
		return cfg.Name, nil
	}
	cfg.Name = identity.Compute(cfg.LicenseKey)
	if err := SaveConfig(cfg); err != nil {
		return cfg.Name, err
	}
	return cfg.Name, nil
}

// SetDeviceName validates and persists a customer-chosen device name.
// Replaces any previously stored value (auto-computed or otherwise).
func SetDeviceName(name string) error {
	if err := identity.Validate(name); err != nil {
		return err
	}
	cfg := LoadConfig()
	if cfg == nil {
		cfg = &EMOSConfig{}
	}
	cfg.Name = name
	return SaveConfig(cfg)
}

func InstallerURL() string {
	return "https://raw.githubusercontent.com/" + GitHubOrg + "/" + GitHubRepo + "/main/stack/emos-cli/scripts/install.sh"
}

func ReleasesURL() string {
	return "https://api.github.com/repos/" + GitHubOrg + "/" + GitHubRepo + "/releases/latest"
}
