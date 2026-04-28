package server

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/automatika-robotics/emos-cli/internal/config"
)

const (
	tokenTTL      = 90 * 24 * time.Hour
	pairingDigits = 6
)

// Auth handles pairing-code -> bearer-token issuance for the dashboard.
// The mutex serialises in-memory mutations; load-modify-save goes through
// config.LoadConfig/SaveConfig under that mutex.
type Auth struct {
	mu        sync.Mutex
	state     config.AuthState
	bypass    bool   // --no-auth for development
	freshCode string // plaintext, only set when newly generated for printing
}

// NewAuthForCLI is a thin wrapper for CLI subcommands that need to mutate
// auth state (revoke, regenerate) without booting the full server.
func NewAuthForCLI() (*Auth, error) { return NewAuth(false) }

// NewAuth loads or initialises the auth state. If no pairing code is on
// disk, a fresh six-digit code is generated, persisted as a hash, and made
// available via FreshPairingCode for one-time display by the caller.
func NewAuth(bypass bool) (*Auth, error) {
	a := &Auth{bypass: bypass}
	cfg := config.LoadConfig()
	if cfg != nil {
		a.state = cfg.Auth
	}
	if a.state.PairingCodeHash != "" {
		return a, nil
	}
	code, err := generatePairingCode()
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state.PairingCodeHash = hashSecret(code)
	a.state.PairingCreated = time.Now()
	a.freshCode = code
	if err := a.persistLocked(); err != nil {
		return nil, err
	}
	return a, nil
}

// FreshPairingCode returns the just-generated code if this process generated it.
func (a *Auth) FreshPairingCode() string { return a.freshCode }

// RegeneratePairingCode replaces the stored code (revoking any in-flight
// pair attempt). Returns the new plaintext code.
func (a *Auth) RegeneratePairingCode() (string, error) {
	code, err := generatePairingCode()
	if err != nil {
		return "", err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state.PairingCodeHash = hashSecret(code)
	a.state.PairingCreated = time.Now()
	if err := a.persistLocked(); err != nil {
		return "", err
	}
	return code, nil
}

// Pair exchanges a pairing code for a long-lived bearer token. Multiple
// devices can pair until the operator regenerates the code.
func (a *Auth) Pair(code, label string) (string, time.Time, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.state.PairingCodeHash == "" {
		return "", time.Time{}, errors.New("pairing not configured")
	}
	if !constantTimeEqual(hashSecret(strings.TrimSpace(code)), a.state.PairingCodeHash) {
		return "", time.Time{}, errors.New("invalid pairing code")
	}
	token, err := generateToken()
	if err != nil {
		return "", time.Time{}, err
	}
	exp := time.Now().Add(tokenTTL)
	a.state.Tokens = append(a.state.Tokens, config.AuthToken{
		Hash:      hashSecret(token),
		IssuedAt:  time.Now(),
		ExpiresAt: exp,
		Label:     label,
	})
	if err := a.persistLocked(); err != nil {
		return "", time.Time{}, err
	}
	return token, exp, nil
}

// Verify returns nil if the bearer token is valid (not expired, in store).
func (a *Auth) Verify(token string) error {
	if a.bypass {
		return nil
	}
	if token == "" {
		return errors.New("missing token")
	}
	hash := hashSecret(token)
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	for _, t := range a.state.Tokens {
		if t.Hash == hash {
			if now.After(t.ExpiresAt) {
				return errors.New("token expired")
			}
			return nil
		}
	}
	return errors.New("invalid token")
}

// RevokeAll wipes all tokens (CLI use).
func (a *Auth) RevokeAll() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state.Tokens = nil
	return a.persistLocked()
}

// TokenView is the safe-to-print metadata for a single issued token.
// `ID` is a short prefix of the SHA-256 hash, suitable for matching in
// `emos config revoke-token <id>` without exposing the hash itself.
type TokenView struct {
	ID        string
	Label     string
	IssuedAt  time.Time
	ExpiresAt time.Time
}

// ListTokens returns metadata for every issued token. Hashes are not
// surfaced — only an unambiguous short ID derived from each.
func (a *Auth) ListTokens() []TokenView {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]TokenView, 0, len(a.state.Tokens))
	for _, t := range a.state.Tokens {
		out = append(out, TokenView{
			ID:        shortID(t.Hash),
			Label:     t.Label,
			IssuedAt:  t.IssuedAt,
			ExpiresAt: t.ExpiresAt,
		})
	}
	return out
}

// RevokeMatching removes every token whose short ID matches `idOrLabel` (a
// prefix of the hash) OR whose Label exactly equals `idOrLabel`.
func (a *Auth) RevokeMatching(idOrLabel string) (int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	kept := a.state.Tokens[:0]
	revoked := 0
	for _, t := range a.state.Tokens {
		if shortID(t.Hash) == idOrLabel || (t.Label != "" && t.Label == idOrLabel) {
			revoked++
			continue
		}
		kept = append(kept, t)
	}
	if revoked == 0 {
		return 0, nil
	}
	a.state.Tokens = kept
	return revoked, a.persistLocked()
}

// shortID returns the first 8 hex chars of a token hash. Long enough that
// 100+ tokens are extremely unlikely to collide.
func shortID(hash string) string {
	if len(hash) >= 8 {
		return hash[:8]
	}
	return hash
}

// AuthRequired enforces auth on protected routes.
func (a *Auth) AuthRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.bypass {
			next.ServeHTTP(w, r)
			return
		}
		token := bearerToken(r)
		if err := a.Verify(token); err != nil {
			writeErr(w, http.StatusUnauthorized, codeUnauthorized, err.Error())
			return
		}
		next.ServeHTTP(w, r)
	})
}

func bearerToken(r *http.Request) string {
	v := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(v), "bearer ") {
		return strings.TrimSpace(v[len("Bearer "):])
	}
	// Allow ?token= for SSE clients that can't set Authorization.
	if t := r.URL.Query().Get("token"); t != "" {
		return t
	}
	return ""
}

// persistLocked writes the auth state back to ~/.config/emos/config.json.
// Caller must hold a.mu.
func (a *Auth) persistLocked() error {
	cfg := config.LoadConfig()
	if cfg == nil {
		cfg = &config.EMOSConfig{}
	}
	cfg.Auth = a.state
	return config.SaveConfig(cfg)
}

// --- crypto helpers ---

func generatePairingCode() (string, error) {
	max := big.NewInt(1)
	for i := 0; i < pairingDigits; i++ {
		max.Mul(max, big.NewInt(10))
	}
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%0*d", pairingDigits, n.Int64()), nil
}

func generateToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func hashSecret(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func constantTimeEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := 0; i < len(a); i++ {
		v |= a[i] ^ b[i]
	}
	return v == 0
}
