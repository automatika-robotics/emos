package server

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/automatika-robotics/emos-cli/internal/config"
)

// withTempConfig points the config package's path globals at a fresh tmp
// dir so each test has an isolated ~/.config/emos. Restored on cleanup.
func withTempConfig(t *testing.T) {
	t.Helper()
	origHome, origDir, origRecipes, origLogs, origLicense, origCfg :=
		config.HomeDir, config.ConfigDir, config.RecipesDir, config.LogsDir, config.LicenseFile, config.ConfigFile

	tmp := t.TempDir()
	config.HomeDir = tmp
	config.ConfigDir = filepath.Join(tmp, ".config", "emos")
	config.RecipesDir = filepath.Join(tmp, "emos", "recipes")
	config.LogsDir = filepath.Join(tmp, "emos", "logs")
	config.LicenseFile = filepath.Join(config.ConfigDir, "license.key")
	config.ConfigFile = filepath.Join(config.ConfigDir, "config.json")

	t.Cleanup(func() {
		config.HomeDir = origHome
		config.ConfigDir = origDir
		config.RecipesDir = origRecipes
		config.LogsDir = origLogs
		config.LicenseFile = origLicense
		config.ConfigFile = origCfg
	})
}

func TestNewAuthFreshFirstBoot(t *testing.T) {
	withTempConfig(t)

	a, err := NewAuth(false)
	if err != nil {
		t.Fatalf("NewAuth: %v", err)
	}
	code := a.FreshPairingCode()
	if code == "" {
		t.Fatalf("expected fresh pairing code on first boot, got empty")
	}
	if len(code) != pairingDigits {
		t.Fatalf("FreshPairingCode length = %d, want %d", len(code), pairingDigits)
	}
	// Hash, not plaintext, on disk.
	cfg := config.LoadConfig()
	if cfg == nil || cfg.Auth.PairingCodeHash == "" {
		t.Fatalf("pairing hash not persisted: %+v", cfg)
	}
	if cfg.Auth.PairingCodeHash == code {
		t.Fatalf("plaintext pairing code leaked into disk")
	}
}

func TestNewAuthDoesNotRegenerateExistingHash(t *testing.T) {
	withTempConfig(t)

	a1, err := NewAuth(false)
	if err != nil {
		t.Fatalf("NewAuth #1: %v", err)
	}
	hash1 := config.LoadConfig().Auth.PairingCodeHash

	a2, err := NewAuth(false)
	if err != nil {
		t.Fatalf("NewAuth #2: %v", err)
	}
	if a2.FreshPairingCode() != "" {
		t.Fatalf("second NewAuth should not produce a fresh code")
	}
	hash2 := config.LoadConfig().Auth.PairingCodeHash
	if hash1 != hash2 {
		t.Fatalf("pairing hash regenerated unexpectedly: %q -> %q", hash1, hash2)
	}
	_ = a1
}

func TestPairValidCode(t *testing.T) {
	withTempConfig(t)

	a, err := NewAuth(false)
	if err != nil {
		t.Fatalf("NewAuth: %v", err)
	}
	code := a.FreshPairingCode()

	tok, exp, err := a.Pair(code, "phone")
	if err != nil {
		t.Fatalf("Pair: %v", err)
	}
	if tok == "" {
		t.Fatalf("Pair returned empty token")
	}
	if !exp.After(time.Now()) {
		t.Fatalf("Pair expiry %v not in the future", exp)
	}
	if err := a.Verify(tok); err != nil {
		t.Fatalf("Verify(fresh token): %v", err)
	}
}

func TestPairInvalidCode(t *testing.T) {
	withTempConfig(t)

	a, err := NewAuth(false)
	if err != nil {
		t.Fatalf("NewAuth: %v", err)
	}
	if _, _, err := a.Pair("000000", ""); err == nil {
		t.Fatalf("Pair with wrong code should error")
	}
}

func TestVerify(t *testing.T) {
	withTempConfig(t)

	a, err := NewAuth(false)
	if err != nil {
		t.Fatalf("NewAuth: %v", err)
	}
	code := a.FreshPairingCode()
	tok, _, err := a.Pair(code, "")
	if err != nil {
		t.Fatalf("Pair: %v", err)
	}

	if err := a.Verify(""); err == nil {
		t.Fatalf("Verify(\"\") should error")
	}
	if err := a.Verify("not-a-real-token"); err == nil {
		t.Fatalf("Verify(garbage) should error")
	}

	// Force expiry by editing on-disk config.
	cfg := config.LoadConfig()
	cfg.Auth.Tokens[0].ExpiresAt = time.Now().Add(-time.Minute)
	if err := config.SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	a2, err := NewAuth(false)
	if err != nil {
		t.Fatalf("NewAuth: %v", err)
	}
	if err := a2.Verify(tok); err == nil {
		t.Fatalf("Verify(expired) should error")
	}
}

func TestVerifyBypass(t *testing.T) {
	withTempConfig(t)

	a, err := NewAuth(true) // bypass
	if err != nil {
		t.Fatalf("NewAuth: %v", err)
	}
	if err := a.Verify(""); err != nil {
		t.Fatalf("Verify in bypass mode should accept empty: %v", err)
	}
	if err := a.Verify("anything"); err != nil {
		t.Fatalf("Verify in bypass mode should accept arbitrary input: %v", err)
	}
}

func TestRegeneratePairingCode(t *testing.T) {
	withTempConfig(t)

	a, err := NewAuth(false)
	if err != nil {
		t.Fatalf("NewAuth: %v", err)
	}
	code1 := a.FreshPairingCode()
	tok, _, err := a.Pair(code1, "phone")
	if err != nil {
		t.Fatalf("Pair: %v", err)
	}

	code2, err := a.RegeneratePairingCode()
	if err != nil {
		t.Fatalf("RegeneratePairingCode: %v", err)
	}
	if code2 == code1 {
		t.Fatalf("regenerated code equal to old code (extremely unlikely; suspect bug)")
	}
	// Existing token must still verify — rotation is a code rotation, not a
	// blanket revoke.
	if err := a.Verify(tok); err != nil {
		t.Fatalf("token revoked by rotation: %v", err)
	}
	// Old code must no longer pair.
	if _, _, err := a.Pair(code1, ""); err == nil {
		t.Fatalf("old pairing code still works after rotation")
	}
}

func TestRevokeAll(t *testing.T) {
	withTempConfig(t)

	a, err := NewAuth(false)
	if err != nil {
		t.Fatalf("NewAuth: %v", err)
	}
	tok, _, _ := a.Pair(a.FreshPairingCode(), "phone")
	if err := a.RevokeAll(); err != nil {
		t.Fatalf("RevokeAll: %v", err)
	}
	if err := a.Verify(tok); err == nil {
		t.Fatalf("Verify after RevokeAll should error")
	}
	if cfg := config.LoadConfig(); cfg == nil || len(cfg.Auth.Tokens) != 0 {
		t.Fatalf("tokens not cleared on disk: %+v", cfg)
	}
}

func TestListTokensNoHashLeak(t *testing.T) {
	withTempConfig(t)

	a, err := NewAuth(false)
	if err != nil {
		t.Fatalf("NewAuth: %v", err)
	}
	if _, _, err := a.Pair(a.FreshPairingCode(), "phone"); err != nil {
		t.Fatalf("Pair: %v", err)
	}
	views := a.ListTokens()
	if len(views) != 1 {
		t.Fatalf("ListTokens count = %d, want 1", len(views))
	}
	v := views[0]
	if v.Label != "phone" {
		t.Fatalf("Label = %q, want %q", v.Label, "phone")
	}
	if v.ID == "" || len(v.ID) != 8 {
		t.Fatalf("ID = %q, want 8-char short id", v.ID)
	}
	// The TokenView shape must not have a field that could carry the hash;
	// the short ID is a SHA-256 prefix, never the raw token.
	cfg := config.LoadConfig()
	if v.ID == cfg.Auth.Tokens[0].Hash {
		t.Fatalf("ListTokens leaked full hash as ID")
	}
}

func TestRevokeMatching(t *testing.T) {
	withTempConfig(t)

	a, err := NewAuth(false)
	if err != nil {
		t.Fatalf("NewAuth: %v", err)
	}
	code := a.FreshPairingCode()
	_, _, _ = a.Pair(code, "phone")
	_, _, _ = a.Pair(code, "tablet")
	_, _, _ = a.Pair(code, "phone") // duplicate label

	views := a.ListTokens()
	if len(views) != 3 {
		t.Fatalf("setup: ListTokens = %d, want 3", len(views))
	}

	// By label: revokes both "phone" tokens, leaves "tablet".
	n, err := a.RevokeMatching("phone")
	if err != nil {
		t.Fatalf("RevokeMatching(phone): %v", err)
	}
	if n != 2 {
		t.Fatalf("RevokeMatching(phone) = %d, want 2", n)
	}
	if got := a.ListTokens(); len(got) != 1 || got[0].Label != "tablet" {
		t.Fatalf("after label revoke: %+v", got)
	}

	// By short ID: pinpoints the remaining token.
	id := a.ListTokens()[0].ID
	n, err = a.RevokeMatching(id)
	if err != nil {
		t.Fatalf("RevokeMatching(id): %v", err)
	}
	if n != 1 {
		t.Fatalf("RevokeMatching(id) = %d, want 1", n)
	}

	// No match: returns 0, no error.
	n, err = a.RevokeMatching("does-not-exist")
	if err != nil {
		t.Fatalf("RevokeMatching(nomatch): %v", err)
	}
	if n != 0 {
		t.Fatalf("RevokeMatching(nomatch) = %d, want 0", n)
	}
}

func TestPairConcurrent(t *testing.T) {
	withTempConfig(t)

	a, err := NewAuth(false)
	if err != nil {
		t.Fatalf("NewAuth: %v", err)
	}
	code := a.FreshPairingCode()

	const N = 50
	tokens := make(chan string, N)
	var wg sync.WaitGroup
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tok, _, err := a.Pair(code, "")
			if err != nil {
				t.Errorf("Pair: %v", err)
				return
			}
			tokens <- tok
		}()
	}
	wg.Wait()
	close(tokens)

	seen := map[string]bool{}
	for tok := range tokens {
		if seen[tok] {
			t.Fatalf("duplicate token issued under concurrent Pair: %s", tok)
		}
		seen[tok] = true
	}
	if len(seen) != N {
		t.Fatalf("concurrent Pair issued %d distinct tokens, want %d", len(seen), N)
	}
	if got := len(a.ListTokens()); got != N {
		t.Fatalf("ListTokens = %d, want %d", got, N)
	}
}
