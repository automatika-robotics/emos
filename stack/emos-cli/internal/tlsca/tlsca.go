// Package tlsca mints and persists a self-signed TLS certificate for the
// EMOS dashboard. The certificate is created on first boot, stored under
// ~/.config/emos/, and rotated when nearing expiry.
package tlsca

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/netinfo"
)

const (
	certFile = "tls.crt"
	keyFile  = "tls.key"

	certValidity = 2 * 365 * 24 * time.Hour // 2 years
	rotateBefore = 30 * 24 * time.Hour      // re-mint when within 30 days of expiry
)

// Info bundles cert info in displayable format
type Info struct {
	TLSCert     tls.Certificate
	Leaf        *x509.Certificate
	CertPath    string
	KeyPath     string
	Fingerprint string
}

// Ensure returns a valid Info.
func Ensure(deviceName string) (*Info, error) {
	if info, err := load(); err == nil && !shouldRotate(info) {
		return info, nil
	}
	return Generate(deviceName)
}

// Generate mints a fresh self-signed certificate and persists it,
// overwriting any existing files.
func Generate(deviceName string) (*Info, error) {
	if err := os.MkdirAll(config.ConfigDir, 0o700); err != nil {
		return nil, fmt.Errorf("ensure config dir: %w", err)
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("serial: %w", err)
	}

	dnsSANs, ipSANs := buildSANs(deviceName)

	cn := deviceName
	if cn == "" {
		cn = "emos-device"
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: cn, Organization: []string{"EMOS"}},
		NotBefore:    time.Now().Add(-5 * time.Minute),
		NotAfter:     time.Now().Add(certValidity),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     dnsSANs,
		IPAddresses:  ipSANs,
		// Self-signed: this cert is its own CA so the browser trusts the
		// chain after the user accepts the warning.
		IsCA:                  true,
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, fmt.Errorf("create cert: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("marshal key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	certPath, keyPath := paths()
	if err := os.WriteFile(certPath, certPEM, 0o644); err != nil {
		return nil, fmt.Errorf("write cert: %w", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		return nil, fmt.Errorf("write key: %w", err)
	}
	return load()
}

// Load returns the persisted certificate, or an error if no valid keypair
// is on disk. Callers that want auto-generation should use Ensure instead.
func Load() (*Info, error) { return load() }

// Fingerprint returns the colon-separated uppercase SHA-256 of the
// certificate's DER encoding, matching the format browsers display.
func Fingerprint(c *x509.Certificate) string {
	sum := sha256.Sum256(c.Raw)
	hexed := strings.ToUpper(hex.EncodeToString(sum[:]))
	var b strings.Builder
	for i := 0; i < len(hexed); i += 2 {
		if i > 0 {
			b.WriteByte(':')
		}
		b.WriteString(hexed[i : i+2])
	}
	return b.String()
}

// Paths returns the on-disk locations of the cert + key.
func Paths() (certPath, keyPath string) { return paths() }

// --- internals ---

func paths() (string, string) {
	return filepath.Join(config.ConfigDir, certFile), filepath.Join(config.ConfigDir, keyFile)
}

func load() (*Info, error) {
	certPath, keyPath := paths()
	tlsCert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, err
	}
	if len(tlsCert.Certificate) == 0 {
		return nil, errors.New("tls: empty certificate chain")
	}
	leaf, err := x509.ParseCertificate(tlsCert.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("parse leaf: %w", err)
	}
	tlsCert.Leaf = leaf
	return &Info{
		TLSCert:     tlsCert,
		Leaf:        leaf,
		CertPath:    certPath,
		KeyPath:     keyPath,
		Fingerprint: Fingerprint(leaf),
	}, nil
}

func shouldRotate(info *Info) bool {
	return time.Until(info.Leaf.NotAfter) < rotateBefore
}

// buildSANs computes the DNS and IP SANs the cert should cover. Includes
// the per-device <name>.local, the shared emos.local shortcut, localhost
// for browser dev, plus 127.0.0.1, ::1, and every LAN IPv4 we can see at
// mint time.
func buildSANs(deviceName string) (dns []string, ips []net.IP) {
	dns = []string{"localhost"}
	if deviceName != "" {
		dns = append(dns, deviceName+".local")
	}
	dns = append(dns, "emos.local")

	ips = []net.IP{net.IPv4(127, 0, 0, 1), net.ParseIP("::1")}
	for _, s := range netinfo.LocalIPv4Strings() {
		if ip := net.ParseIP(s); ip != nil {
			ips = append(ips, ip)
		}
	}
	return dns, ips
}
