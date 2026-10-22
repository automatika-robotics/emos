package server

import (
	"net/http"
	"time"

	"github.com/automatika-robotics/emos-cli/internal/config"
)

// licenseInfo is the license as the dashboard shows it, without the key.
// Licensed installs get SupportURL, unlicensed ones SalesEmail.
type licenseInfo struct {
	Licensed     bool       `json:"licensed"`
	Holder       string     `json:"holder,omitempty"`
	Robot        string     `json:"robot,omitempty"`
	PluginSlug   string     `json:"plugin_slug,omitempty"`
	Tier         string     `json:"tier,omitempty"`
	SerialNumber string     `json:"serial_number,omitempty"`
	ActivatedAt  *time.Time `json:"activated_at,omitempty"`
	VerifiedAt   *time.Time `json:"verified_at,omitempty"`
	SupportURL   string     `json:"support_url,omitempty"`
	SalesEmail   string     `json:"sales_email,omitempty"`
}

// handleLicense returns the license stored on this machine. It requires
// pairing, since it includes the holder's name and the robot's serial number.
func (s *Server) handleLicense(w http.ResponseWriter, r *http.Request) {
	lic := config.LoadLicense()
	if lic == nil {
		writeJSON(w, http.StatusOK, licenseInfo{SalesEmail: config.SalesEmail})
		return
	}
	robot := lic.PluginName
	if robot == "" {
		robot = lic.PluginSlug
	}
	writeJSON(w, http.StatusOK, licenseInfo{
		Licensed:     true,
		Holder:       lic.ClientName,
		Robot:        robot,
		PluginSlug:   lic.PluginSlug,
		Tier:         lic.Tier,
		SerialNumber: lic.SerialNumber,
		ActivatedAt:  lic.ClaimedAt,
		VerifiedAt:   &lic.VerifiedAt,
		SupportURL:   config.SupportURL,
	})
}
