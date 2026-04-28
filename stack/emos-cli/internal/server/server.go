package server

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/grandcat/zeroconf"

	"github.com/automatika-robotics/emos-cli/internal/config"
)

// Options configures the daemon at boot.
type Options struct {
	Addr          string  // bind address, e.g. ":8765"
	DisableMDNS   bool    // skip zeroconf publication
	DisableAuth   bool    // dev only: accept all requests
	UI            fs.FS   // embedded SPA; nil disables the UI
	Logger        *slog.Logger
}

// Server bundles every subsystem the daemon needs. Handlers are methods on
// Server so they can read `s.runtime`, `s.jobs`, `s.auth` without DI plumbing.
type Server struct {
	cfg    *config.EMOSConfig
	opts   Options
	log    *slog.Logger
	router http.Handler

	auth    *Auth
	conn    *Connectivity
	runtime *Runtime
	jobs    *Jobs

	startedAt time.Time

	httpServer *http.Server
	mdns       *zeroconf.Server
}

// New constructs a Server with all subsystems initialised. The pairing code,
// if freshly generated, is available via s.PairingCode().
func New(opts Options) (*Server, error) {
	if opts.Addr == "" {
		opts.Addr = ":8765"
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}

	auth, err := NewAuth(opts.DisableAuth)
	if err != nil {
		return nil, err
	}

	s := &Server{
		cfg:       config.LoadConfig(),
		opts:      opts,
		log:       opts.Logger,
		auth:      auth,
		conn:      NewConnectivity(),
		runtime:   NewRuntime(),
		jobs:      NewJobs(),
		startedAt: time.Now(),
	}
	s.router = s.buildRouter()
	return s, nil
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
		}
		mdnsSrv, err := announceMDNS(port, txt, s.log)
		if err != nil {
			s.log.Warn("mDNS register failed", "err", err)
		}
		s.mdns = mdnsSrv
	}

	s.httpServer = &http.Server{
		Addr:              s.opts.Addr,
		Handler:           s.router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		s.log.Info("dashboard listening", "addr", s.opts.Addr)
		if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
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

	if s.mdns != nil {
		s.mdns.Shutdown()
	}
	return s.httpServer.Shutdown(shutdownCtx)
}

// PairingCode returns the freshly-generated pairing code (one-time per process)
// for the CLI to display, or "" if pairing was already configured.
func (s *Server) PairingCode() string { return s.auth.FreshPairingCode() }

// modeOrUnknown returns the install mode for diagnostics, or "unknown" when
// no config is on disk
func (s *Server) modeOrUnknown() config.InstallMode {
	if s.cfg == nil {
		return "unknown"
	}
	return s.cfg.Mode
}

func portFromAddr(addr string) (int, error) {
	_, p, err := net.SplitHostPort(addr)
	if err != nil {
		return 0, err
	}
	if p == "" {
		return 80, nil
	}
	if strings.HasPrefix(p, ":") {
		p = p[1:]
	}
	return strconv.Atoi(p)
}
