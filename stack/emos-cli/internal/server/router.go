package server

import (
	"io/fs"
	"log/slog"
	"net/http"
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

	// CORS, robots are typically accessed from a single LAN browser, but during
	// development the SvelteKit dev server runs on a different port.
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
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

		// --- Authenticated surface ---
		r.Group(func(r chi.Router) {
			r.Use(s.auth.AuthRequired)

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
			r.Get("/runs/{id}/logs", s.handleRunLogs)

			r.Get("/jobs", s.handleJobsList)
			r.Get("/jobs/{id}", s.handleJobGet)
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
