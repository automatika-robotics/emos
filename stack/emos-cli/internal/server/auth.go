package server

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/automatika-robotics/emos-cli/internal/config"
)

const (
	pairingFileName = "serve.json"
	tokenTTL        = 90 * 24 * time.Hour
	pairingDigits   = 6
)

// authState is the JSON shape persisted to ~/.config/emos/serve.json. Both the
// pairing code and the issued tokens are stored as SHA-256 hashes
type authState struct {
	PairingCodeHash string         `json:"pairing_code_hash"`
	PairingCreated  time.Time      `json:"pairing_created"`
	Tokens          []storedToken  `json:"tokens"`
	UnauthOK        bool           `json:"unauth_ok,omitempty"` // dev-only opt-out
}

type storedToken struct {
	Hash      string    `json:"hash"`
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Label     string    `json:"label,omitempty"`
}

// Auth handles pairing-code -> bearer-token issuance for the dashboard.
type Auth struct {
	mu        sync.Mutex
	state     authState
	path      string
	bypass    bool   // --no-auth for development
	freshCode string // plaintext, only set when newly generated for printing
}

// NewAuthForCLI is a thin wrapper for CLI subcommands that need to mutate
// auth state (revoke, regenerate) without booting the full server.
func NewAuthForCLI() (*Auth, error) { return NewAuth(false) }

// NewAuth loads or initializes the auth state. If no pairing code is on disk,
// a fresh six-digit code is generated, persisted as a hash, and made available
// via FreshPairingCode for one-time display by the caller.
func NewAuth(bypass bool) (*Auth, error) {
	a := &Auth{
		path:   filepath.Join(config.ConfigDir, pairingFileName),
		bypass: bypass,
	}
	if err := a.load(); err != nil {
		return nil, err
	}
	if a.state.PairingCodeHash == "" {
		code, err := generatePairingCode()
		if err != nil {
			return nil, err
		}
		a.state.PairingCodeHash = hashSecret(code)
		a.state.PairingCreated = time.Now()
		a.freshCode = code
		if err := a.save(); err != nil {
			return nil, err
		}
	}
	return a, nil
}

// FreshPairingCode returns the just-generated code if this process generated it
func (a *Auth) FreshPairingCode() string { return a.freshCode }

// RegeneratePairingCode replaces the stored code (revoking any in-flight
// pair attempt). Returns the new plaintext code.
func (a *Auth) RegeneratePairingCode() (string, error) {
	code, err := generatePairingCode()
	if err != nil {
		return "", err
	}
	a.mu.Lock()
	a.state.PairingCodeHash = hashSecret(code)
	a.state.PairingCreated = time.Now()
	err = a.saveLocked()
	a.mu.Unlock()
	if err != nil {
		return "", err
	}
	return code, nil
}

// Pair exchanges a pairing code for a long-lived bearer token. The code is not
// rotated on success; multiple devices can pair until the operator regenerates.
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
	a.state.Tokens = append(a.state.Tokens, storedToken{
		Hash:      hashSecret(token),
		IssuedAt:  time.Now(),
		ExpiresAt: exp,
		Label:     label,
	})
	if err := a.saveLocked(); err != nil {
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
	return a.saveLocked()
}

// AuthRequired enforces auth on protected routes
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

// --- persistence helpers ---

func (a *Auth) load() error {
	data, err := os.ReadFile(a.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(data, &a.state)
}

func (a *Auth) save() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.saveLocked()
}

func (a *Auth) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(a.path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(a.state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(a.path, data, 0600)
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

