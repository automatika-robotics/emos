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
	"sync"
	"time"

	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/tlsca"
	"github.com/automatika-robotics/emos-cli/internal/updcheck"
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
	// wg tracks the run goroutines (preflight + cleanup) so shutdown can
	// join them.
	wg sync.WaitGroup

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

	// updates caches the latest release tag for /info to surface. Empty
	// until the first successful refresh; refreshed by a goroutine spawned
	// in Run().
	updates updateState
}

// updateState is the daemon's in-memory copy of the latest release tag.
// Mutated only from the refresh goroutine; read by /info via Snapshot().
type updateState struct {
	mu        sync.RWMutex
	latest    string // GitHub tag with the leading "v" stripped; "" before first success
	checkedAt time.Time
}

// Snapshot returns a copy of the current update state.
func (u *updateState) Snapshot() (latest string, checkedAt time.Time) {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.latest, u.checkedAt
}

// set is called by the refresh goroutine on a successful Latest() lookup.
func (u *updateState) set(latest string) {
	u.mu.Lock()
	u.latest = latest
	u.checkedAt = time.Now()
	u.mu.Unlock()
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

	// Background loop that refreshes the cached "latest release" tag.
	// Tied to ctx so it exits cleanly with the rest of the daemon.
	go s.refreshUpdatesLoop(ctx)

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
	httpErr := s.httpServer.Shutdown(shutdownCtx)
	// When the HTTP server is quiet, stop what we started. (recipes etc)
	s.drainRuns(runDrainTimeout)
	return httpErr
}

// runDrainTimeout bounds the shutdown wait for the run goroutines. Cancel
// SIGKILLs the recipe after a 5 s grace, so the join completes just after.
const runDrainTimeout = 8 * time.Second

// goTracked runs fn on a goroutine registered with the shutdown WaitGroup.
func (s *Server) goTracked(fn func()) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		fn()
	}()
}

// drainRuns cancels the active run, if any, and joins the tracked run
// goroutines so strategy.Cleanup() gets to execute before the process
// exits. A goroutine stuck mid-preflight past its next cancel checkpoint is logged
// and abandoned rather than hanging the stop.
func (s *Server) drainRuns(timeout time.Duration) {
	if cur := s.runtime.Current(); cur != nil {
		s.log.Info("shutdown: stopping active run", "id", cur.ID, "recipe", cur.Recipe)
		if err := s.runtime.Cancel(cur.ID); err != nil {
			s.log.Warn("shutdown: cancel run", "err", err)
		}
	}
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		s.log.Warn("shutdown: run goroutines did not finish in time; exiting anyway")
	}
}

// PairingCode returns the freshly-generated pairing code (one-time per process)
// for the CLI to display, or "" if pairing was already configured.
func (s *Server) PairingCode() string { return s.auth.FreshPairingCode() }

// refreshUpdatesLoop calls updcheck.Latest() at startup and on a ticker.
// Exits when ctx is cancelled, which happens at daemon shutdown.
func (s *Server) refreshUpdatesLoop(ctx context.Context) {
	s.tryRefreshUpdates(ctx)

	ticker := time.NewTicker(updcheck.DefaultRefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tryRefreshUpdates(ctx)
		}
	}
}

// tryRefreshUpdates performs a single best-effort GitHub releases lookup.
// Skipped entirely when connectivity is known-offline.
func (s *Server) tryRefreshUpdates(ctx context.Context) {
	if !s.conn.Online(ctx) {
		s.log.Debug("update check skipped: offline")
		return
	}
	fetchCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	latest, err := updcheck.Latest(fetchCtx)
	if err != nil {
		s.log.Debug("update check failed", "err", err)
		return
	}
	s.updates.set(latest)
}

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
