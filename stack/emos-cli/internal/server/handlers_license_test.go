package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/automatika-robotics/emos-cli/internal/config"
)

func TestLicenseOfAFreeInstallPointsAtSales(t *testing.T) {
	s := newTestServer(t, true)
	rec := httpServe(t, s, httptest.NewRequest(http.MethodGet, "/api/v1/license", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var got licenseInfo
	jsonBody(t, rec, &got)
	if got.Licensed || got.SalesEmail != config.SalesEmail || got.SupportURL != "" || got.Holder != "" {
		t.Errorf("free install = %+v", got)
	}
}

func TestLicenseShowsTheKeptLicenseAndNeverItsKey(t *testing.T) {
	s := newTestServer(t, true)
	claimed := time.Date(2026, 9, 21, 10, 42, 7, 0, time.UTC)
	if err := config.SaveLicense(&config.License{
		Key: "ABCDE-FGHJK-LMNPQ-RSTUV", PluginSlug: "emos-plugin-lite3", PluginName: "DeepRobotics Lite3",
		ClientName: "ACME Security GmbH", SerialNumber: "L3-2026-00417", Tier: "pro",
		ClaimedAt: &claimed, VerifiedAt: time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}

	rec := httpServe(t, s, httptest.NewRequest(http.MethodGet, "/api/v1/license", nil))
	var got licenseInfo
	jsonBody(t, rec, &got)
	if !got.Licensed || got.Holder != "ACME Security GmbH" || got.Robot != "DeepRobotics Lite3" ||
		got.PluginSlug != "emos-plugin-lite3" || got.Tier != "pro" || got.SerialNumber != "L3-2026-00417" ||
		got.ActivatedAt == nil || !got.ActivatedAt.Equal(claimed) || got.VerifiedAt == nil ||
		got.SupportURL != config.SupportURL || got.SalesEmail != "" {
		t.Errorf("licence = %+v", got)
	}
	for _, part := range []string{"ABCDE", "RSTUV", "key"} {
		if strings.Contains(rec.Body.String(), part) {
			t.Errorf("the response must not carry the key, found %q in %s", part, rec.Body.String())
		}
	}
}

// The licence names the holder and the robot's serial number, so it is not on
// the public surface that /info is on.
func TestLicenseNeedsPairing(t *testing.T) {
	s := newTestServer(t, false)
	rec := httpServe(t, s, httptest.NewRequest(http.MethodGet, "/api/v1/license", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("unpaired request = %d, want 401", rec.Code)
	}
}
