package http

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// healthHandler serves liveness/readiness probes. Handlers in Go are methods on
// a struct that holds their dependencies (here, the DB pool) — this is how we
// do dependency injection without a framework.
type healthHandler struct {
	db *pgxpool.Pool
}

// live reports that the process is running. It does NOT touch the database, so
// orchestrators can tell "process up" apart from "dependencies healthy".
func (h *healthHandler) live(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ready reports whether the service can handle traffic, which requires a
// reachable database.
func (h *healthHandler) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := h.db.Ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "unavailable",
			"db":     "down",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "db": "up"})
}
