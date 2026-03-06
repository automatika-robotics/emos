package config

import (
	"os"
	"path/filepath"
)

// Version is set at build time via -ldflags.
var Version = "0.4.0"

const (
	ContainerName = "emos"
	ServiceName   = "emos.service"
	GitHubOrg     = "automatika-robotics"
	GitHubRepo    = "emos"

	// API endpoints
	APIBaseURL          = "https://support-api.automatikarobotics.com/api"
	CredentialsEndpoint = APIBaseURL + "/registrations/credentials"
	RecipesEndpoint     = APIBaseURL + "/recipes"
)

var (
	HomeDir    string
	ConfigDir  string
	RecipesDir string
	LicenseFile string
)

func Init() {
	HomeDir, _ = os.UserHomeDir()
	ConfigDir = filepath.Join(HomeDir, ".config", "emos")
	RecipesDir = filepath.Join(HomeDir, "emos", "recipes")
	LicenseFile = filepath.Join(ConfigDir, "license.key")
}

func InstallerURL() string {
	return "https://raw.githubusercontent.com/" + GitHubOrg + "/" + GitHubRepo + "/main/stack/emos-cli/scripts/install.sh"
}

func ReleasesURL() string {
	return "https://api.github.com/repos/" + GitHubOrg + "/" + GitHubRepo + "/releases/latest"
}
