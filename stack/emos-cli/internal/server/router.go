package server

import (
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

func (s *Server) buildRouter() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(s.requestLogger())
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// Restrict to local-loopback, the device's mDNS name, and the
	// shared `emos.local` shortcut.
	r.Use(cors.Handler(cors.Options{
		AllowOriginFunc:  s.corsAllowOrigin,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Last-Event-ID"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	r.Route("/api/v1", func(r chi.Router) {
		// --- Public surface (no auth) ---
		r.Get("/health", s.handleHealth)
		r.Get("/info", s.handleInfo)
		r.Get("/capabilities", s.handleCapabilities)
		r.Get("/connectivity", s.handleConnectivity)
		r.Get("/openapi.yaml", s.handleOpenAPI)

		r.Post("/auth/pair", s.handleAuthPair)
		r.Get("/auth/me", s.handleAuthMe)

		// --- Authenticated surface (Bearer header) ---
		r.Group(func(r chi.Router) {
			r.Use(s.auth.AuthRequired)

			// SSE-ticket issue: requires a valid bearer header to mint.
			r.Post("/auth/sse-ticket", s.handleAuthSSETicket)

			r.Get("/robot", s.handleRobot)

			r.Get("/recipes/local", s.handleRecipesLocal)
			r.Get("/recipes/remote", s.handleRecipesRemote)
			r.Get("/recipes/{name}", s.handleRecipeDetail)
			r.Delete("/recipes/{name}", s.handleRecipeDelete)
			r.Post("/recipes/{name}/pull", s.handleRecipePull)

			r.Get("/runs", s.handleRunsList)
			r.Post("/runs", s.handleRunsStart)
			r.Get("/runs/{id}", s.handleRunGet)
			r.Delete("/runs/{id}", s.handleRunCancel)

			r.Get("/jobs", s.handleJobsList)
			r.Get("/jobs/{id}", s.handleJobGet)
			r.Delete("/jobs/{id}", s.handleJobCancel)
		})

		// --- SSE surface (single-use ticket via ?ticket=) ---
		// EventSource can't carry the Authorization header; these routes
		// instead require a fresh ticket from POST /auth/sse-ticket.
		r.Group(func(r chi.Router) {
			r.Use(s.sseTicketRequired)
			r.Get("/runs/{id}/logs", s.handleRunLogs)
			r.Get("/jobs/{id}/logs", s.handleJobLogs)
		})
	})

	// --- Static SPA (no auth — the SPA itself bootstraps via /info, then
	//     stores a token after pairing). ---
	if s.opts.UI != nil {
		r.Handle("/*", spaHandler(s.opts.UI))
	} else {
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte("EMOS dashboard binary built without embedded UI.\n"))
		})
	}

	return r
}

// spaHandler serves the embedded SPA. Any path that doesn't resolve to a
// real file gets the fallback HTML so client-side routing works.
//
// When no production index.html is embedded (fresh clone before `make web`),
// the handler serves placeholder.html instead.
func spaHandler(uiFS fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(uiFS))

	fallbackName := "index.html"
	if _, err := fs.Stat(uiFS, "index.html"); err != nil {
		fallbackName = "placeholder.html"
	}
	fallbackBytes, _ := fs.ReadFile(uiFS, fallbackName)

	serveFallback := func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fallbackBytes)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			serveFallback(w)
			return
		}
		if _, err := fs.Stat(uiFS, path); err != nil {
			serveFallback(w)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}

// requestLogger writes one line per HTTP request
//	5xx                              -> ERROR
//	4xx                              -> WARN
//	Mutating verbs (POST/PUT/...)    -> INFO
//	GET / HEAD with 2xx-3xx          -> DEBUG (suppressed at default level)
//
// State-changing actions and failures stay visible; routine reads are silent
// unless the operator runs the daemon at debug verbosity.
func (s *Server) requestLogger() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)

			status := ww.Status()
			level := slog.LevelDebug
			switch {
			case status >= 500:
				level = slog.LevelError
			case status >= 400:
				level = slog.LevelWarn
			case r.Method != http.MethodGet && r.Method != http.MethodHead:
				level = slog.LevelInfo
			}
			s.log.Log(r.Context(), level,
				"http",
				"method", r.Method,
				"path", r.URL.Path,
				"status", status,
				"bytes", ww.BytesWritten(),
				"dur", time.Since(start),
			)
		})
	}
}

// corsAllowOrigin decides whether a CORS preflight should be honoured.
// Returning false makes the chi cors middleware drop the request's
// Access-Control-* response headers, which forces the browser to block
// the cross-origin call.
// Allowed at rest:
//   - localhost / 127.0.0.1 / [::1] on any port (browser dev tools, mobile
//     simulators bridged via SSH port-forward).
//   - <device-name>.local — the per-device mDNS name resolved by Bonjour /
//     Avahi.
//   - emos.local — the shared shortcut.
func (s *Server) corsAllowOrigin(_ *http.Request, origin string) bool {
	if os.Getenv("EMOS_DEV") == "1" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host := u.Hostname()
	switch host {
	case "localhost", "127.0.0.1", "::1", "emos.local":
		return true
	}
	if name := s.opts.DeviceName; name != "" && host == name+".local" {
		return true
	}
	return false
}
