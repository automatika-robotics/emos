package identity

import (
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"min length boundary", "abc", false},
		{"max length boundary", strings.Repeat("a", 32), false},
		{"too short", "ab", true},
		{"too long", strings.Repeat("a", 33), true},
		{"alphanumeric and hyphens", "epic-otter-7", false},
		{"uppercase rejected", "Epic-Otter", true},
		{"space rejected", "epic otter", true},
		{"underscore rejected", "epic_otter", true},
		{"leading hyphen rejected", "-epic", true},
		{"trailing hyphen rejected", "epic-", true},
		{"consecutive hyphens rejected", "epic--otter", true},
		{"empty rejected", "", true},
		{"unicode rejected", "épic", true},
		{"only digits accepted", "123", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(tc.input)
			if tc.wantErr && err == nil {
				t.Fatalf("Validate(%q) = nil, want error", tc.input)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("Validate(%q) = %v, want nil", tc.input, err)
			}
		})
	}
}

func TestComputeDeterministic(t *testing.T) {
	const license = "AAAA-BBBB-CCCC-DDDD"
	first := Compute(license)
	for i := 0; i < 5; i++ {
		got := Compute(license)
		if got != first {
			t.Fatalf("Compute(%q) not deterministic: %q vs %q", license, first, got)
		}
	}
	if err := Validate(first); err != nil {
		t.Fatalf("Compute(%q) = %q, fails Validate: %v", license, first, err)
	}
}

func TestComputeDifferentLicensesDifferentNames(t *testing.T) {
	a := Compute("AAAA")
	b := Compute("BBBB")
	if a == b {
		t.Fatalf("expected distinct names for distinct license keys, both = %q", a)
	}
}

func TestComputeNoLicenseProducesValidName(t *testing.T) {
	// With no license key, fingerprint falls back to MAC (if any) or random
	// hex. Either way the result must pass Validate.
	got := Compute("")
	if err := Validate(got); err != nil {
		t.Fatalf("Compute(\"\") = %q, fails Validate: %v", got, err)
	}
}

func TestNameFromSeedShape(t *testing.T) {
	// Seed-derived names are always "<adj>-<noun>" with both parts drawn from
	// the wordlists. Verifies the indexing math doesn't accidentally produce
	// an empty segment.
	got := nameFromSeed("license:probe")
	parts := strings.Split(got, "-")
	if len(parts) != 2 {
		t.Fatalf("nameFromSeed produced %q, want exactly one hyphen", got)
	}
	if !contains(adjectives, parts[0]) {
		t.Fatalf("adjective %q not in wordlist", parts[0])
	}
	if !contains(nouns, parts[1]) {
		t.Fatalf("noun %q not in wordlist", parts[1])
	}
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}
