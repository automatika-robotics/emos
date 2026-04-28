// Package identity assigns each EMOS device a stable, human-friendly name
// derived from a hardware fingerprint. Used by the dashboard banner, mDNS
// publishing, and the dashboard UI so users can identify which robot
// they're talking to without memorising serial numbers or hostnames.
//
// The name is deterministic: the same robot always computes the same name
// from the same fingerprint, so it survives reinstalls and config wipes.
// The user can override it via `emos config set name <new>`.
package identity

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"github.com/automatika-robotics/emos-cli/internal/netinfo"
)

// Compute returns a stable adjective-noun pair for this device.
//
// Fingerprint priority:
//   1. license key, if present (most stable across hardware swaps)
//   2. primary network MAC (survives reinstalls but not NIC swaps)
//   3. random one-shot fallback (used only when the host can't expose
//      either of the above; logs the situation upstream)
//
// The returned name is lowercased ASCII separated by '-' and is always a
// valid mDNS hostname segment.
func Compute(licenseKey string) string {
	if seed := fingerprint(licenseKey); seed != "" {
		return nameFromSeed(seed)
	}
	return fallbackName()
}

// Validate checks whether `name` conforms to mDNS hostname rules:
//   - 3..32 chars
//   - lowercase ASCII letters, digits, hyphens
//   - cannot start/end with a hyphen
//   - cannot have consecutive hyphens
//
// Returns an error message suitable for surfacing to the user, or "" on success.
var validNamePattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$`)

func Validate(name string) error {
	if len(name) < 3 || len(name) > 32 {
		return fmt.Errorf("name must be 3-32 characters")
	}
	if !validNamePattern.MatchString(name) {
		return fmt.Errorf("name may contain only lowercase letters, digits, and hyphens, and cannot start or end with a hyphen")
	}
	if strings.Contains(name, "--") {
		return fmt.Errorf("name cannot contain consecutive hyphens")
	}
	return nil
}

func fingerprint(licenseKey string) string {
	if licenseKey != "" {
		return "license:" + licenseKey
	}
	if mac := netinfo.PrimaryMAC(); mac != "" {
		return "mac:" + mac
	}
	return ""
}

// nameFromSeed maps a deterministic seed to an adjective-noun pair via a
// SHA-256 hash split into two 32-bit ints, indexed into each wordlist.
func nameFromSeed(seed string) string {
	h := sha256.Sum256([]byte(seed))
	a := adjectives[binary.BigEndian.Uint32(h[0:4])%uint32(len(adjectives))]
	n := nouns[binary.BigEndian.Uint32(h[4:8])%uint32(len(nouns))]
	return a + "-" + n
}

// fallbackName is used only when no fingerprint is available. Six random
// hex chars guarantee uniqueness without relying on hardware.
func fallbackName() string {
	var b [3]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "emos-device"
	}
	return "emos-" + hex.EncodeToString(b[:])
}
