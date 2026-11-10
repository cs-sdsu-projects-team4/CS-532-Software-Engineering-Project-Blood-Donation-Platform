// Package http builds the HTTP router and holds the per-resource handlers.
//
// Business rules do NOT live here — handlers only parse input, call into
// domain/matching/store, and write responses. See CLAUDE.md conventions.
package http

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewRouter wires middleware and routes and returns an http.Handler.
func NewRouter(logger *slog.Logger, db *pgxpool.Pool) http.Handler {
	r := chi.NewRouter()

	// Middleware runs on every request, in order.
	r.Use(middleware.RequestID)                // tag each request with an id
	r.Use(middleware.RealIP)                    // trust proxy headers for client IP
	r.Use(middleware.Logger)                    // log method, path, status, duration
	r.Use(middleware.Recoverer)                 // turn panics into 500s, don't crash
	r.Use(middleware.Timeout(30 * time.Second)) // cap request duration

	h := &healthHandler{db: db}

	// Liveness: is the process up? Readiness: can it serve traffic (DB reachable)?
	r.Get("/healthz", h.live)
	r.Get("/readyz", h.ready)

	// Future feature routes mount under /api/v1 (added in later PRs).
	// r.Route("/api/v1", func(r chi.Router) { ... })

	_ = logger // reserved for request-scoped logging in later PRs
	return r
}
