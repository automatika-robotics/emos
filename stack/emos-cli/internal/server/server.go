package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io/fs"
	stdlog "log"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/tlsca"
)

// Options configures the daemon at boot.
type Options struct {
	Addr        string // bind address (host:port); empty = config.DefaultDashboardPort
	DeviceName  string // human-friendly device name, used by mDNS + dashboard UI
	DisableMDNS bool   // skip zeroconf publication
	DisableAuth bool   // dev only: accept all requests
	EnableTLS   bool   // opt-in HTTPS via a self-signed cert; off by default
	UI          fs.FS  // embedded SPA; nil disables the UI
	Logger      *slog.Logger
}

// Server bundles every subsystem the daemon needs. Handlers are methods on
// Server so they can read `s.runtime`, `s.jobs`, `s.auth` without DI plumbing.
type Server struct {
	cfg    *config.EMOSConfig
	opts   Options
	log    *slog.Logger
	router http.Handler

	auth       *Auth
	conn       *Connectivity
	runtime    *Runtime
	jobs       *Jobs
	sseTickets *sseTicketStore

	startedAt time.Time

	httpServer *http.Server
	mdns       *mdnsRegistrations
	tlsInfo    *tlsca.Info // nil when serving plain HTTP
}

// New constructs a Server with all subsystems initialised. The pairing code,
// if freshly generated, is available via s.PairingCode().
func New(opts Options) (*Server, error) {
	if opts.Addr == "" {
		opts.Addr = fmt.Sprintf(":%d", config.DefaultDashboardPort)
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}

	auth, err := NewAuth(opts.DisableAuth)
	if err != nil {
		return nil, err
	}

	s := &Server{
		cfg:        config.LoadConfig(),
		opts:       opts,
		log:        opts.Logger,
		auth:       auth,
		conn:       NewConnectivity(),
		runtime:    NewRuntime(),
		jobs:       NewJobs(),
		sseTickets: newSSETicketStore(),
		startedAt:  time.Now(),
	}
	if opts.EnableTLS {
		info, err := tlsca.Ensure(opts.DeviceName)
		if err != nil {
			return nil, fmt.Errorf("tls: %w", err)
		}
		s.tlsInfo = info
	}
	s.router = s.buildRouter()
	return s, nil
}

// TLSInfo returns the active TLS certificate info, or nil when running
// in --no-tls mode. Used by the CLI to print the cert fingerprint.
func (s *Server) TLSInfo() *tlsca.Info { return s.tlsInfo }

// Scheme returns "https" or "http" depending on whether TLS is active.
func (s *Server) Scheme() string {
	if s.tlsInfo != nil {
		return "https"
	}
	return "http"
}

// Run blocks until ctx is canceled or the listener errors out.
func (s *Server) Run(ctx context.Context) error {
	port, err := portFromAddr(s.opts.Addr)
	if err != nil {
		return err
	}

	if !s.opts.DisableMDNS {
		txt := []string{
			"version=" + config.Version,
			"mode=" + string(s.modeOrUnknown()),
			"name=" + s.opts.DeviceName,
			"scheme=" + s.Scheme(),
		}
		mdnsRegs, err := announceMDNS(port, s.opts.DeviceName, txt, s.log)
		if err != nil {
			s.log.Warn("mDNS register failed", "err", err)
		}
		s.mdns = mdnsRegs
	}

	s.httpServer = &http.Server{
		Addr:              s.opts.Addr,
		Handler:           s.router,
		ReadHeaderTimeout: 10 * time.Second,
		// Route the stdlib's internal log output (TLS handshake errors,
		// "URL query contains semicolon", etc.) through slog so noisy
		// LAN probes don't flood stderr at INFO.
		ErrorLog: stdlog.New(&slogErrorWriter{log: s.log}, "", 0),
	}
	if s.tlsInfo != nil {
		s.httpServer.TLSConfig = &tls.Config{
			Certificates: []tls.Certificate{s.tlsInfo.TLSCert},
			MinVersion:   tls.VersionTLS12,
		}
	}

	errCh := make(chan error, 1)
	go func() {
		s.log.Info("dashboard listening", "addr", s.opts.Addr, "scheme", s.Scheme())
		var err error
		if s.tlsInfo != nil {
			// Cert + key are already in TLSConfig, so the file paths can be empty.
			err = s.httpServer.ListenAndServeTLS("", "")
		} else {
			err = s.httpServer.ListenAndServe()
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
	case err := <-errCh:
		if err != nil {
			return err
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.mdns.Shutdown()
	return s.httpServer.Shutdown(shutdownCtx)
}

// PairingCode returns the freshly-generated pairing code (one-time per process)
// for the CLI to display, or "" if pairing was already configured.
func (s *Server) PairingCode() string { return s.auth.FreshPairingCode() }

// modeOrUnknown returns the install mode for diagnostics, or "unknown"
// when there's no install (no config or a pre-install stub).
func (s *Server) modeOrUnknown() config.InstallMode {
	if !s.cfg.IsInstalled() {
		return "unknown"
	}
	return s.cfg.Mode
}

func portFromAddr(addr string) (int, error) {
	_, p, err := net.SplitHostPort(addr)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(p)
}

// slogErrorWriter routes net/http's internal Logger output through slog.
// On a LAN device, anything probing the dashboard port can generate logs
// which get downgraded to debug.
type slogErrorWriter struct{ log *slog.Logger }

func (w *slogErrorWriter) Write(p []byte) (int, error) {
	msg := strings.TrimRight(string(p), "\n")
	if isHTTPNoise(msg) {
		w.log.Debug("http server", "msg", msg)
	} else {
		w.log.Warn("http server", "msg", msg)
	}
	return len(p), nil
}

// isHTTPNoise classifies a stdlib http error log line as benign LAN
// chatter rather than something the operator should see at INFO
func isHTTPNoise(msg string) bool {
	switch {
	case strings.Contains(msg, "TLS handshake error"),
		strings.Contains(msg, "tls: first record does not look like a TLS handshake"),
		strings.Contains(msg, "tls: unknown certificate"),
		strings.Contains(msg, "tls: bad certificate"),
		strings.Contains(msg, "URL query contains semicolon"):
		return true
	}
	return false
}
