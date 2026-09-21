package config

import (
	"encoding/json"
	"os"
	"time"
)

// License is what the portal said about a licence key when it was last
// verified, which takes a key its holder has activated on the support portal.
// It has its own file, so it outlives an uninstall, and everything that shows
// the licence reads it from there.
type License struct {
	Key          string     `json:"key"`
	PluginSlug   string     `json:"plugin_slug"`           // the robot plugin the licence is for
	PluginName   string     `json:"plugin_name,omitempty"` // the robot, as the catalog names it
	ClientName   string     `json:"client_name,omitempty"`
	SerialNumber string     `json:"serial_number,omitempty"`
	Tier         string     `json:"tier,omitempty"`       // expert, pro or enterprise
	ClaimedAt    *time.Time `json:"claimed_at,omitempty"` // when it was activated on the support portal
	VerifiedAt   time.Time  `json:"verified_at"`
}

// LoadLicense returns the saved licence, or nil when there is none.
func LoadLicense() *License {
	data, err := os.ReadFile(LicenseFile)
	if err != nil {
		return nil
	}
	var lic License
	if json.Unmarshal(data, &lic) != nil || lic.Key == "" {
		return nil
	}
	return &lic
}

// SaveLicense persists the licence. Mode 0600.
func SaveLicense(lic *License) error {
	data, err := json.MarshalIndent(lic, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(LicenseFile, data)
}

// RemoveLicense deletes the saved licence. There being none is not an error.
func RemoveLicense() error {
	if err := os.Remove(LicenseFile); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
