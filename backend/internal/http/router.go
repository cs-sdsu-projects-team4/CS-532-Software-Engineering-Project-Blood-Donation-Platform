// Package http builds the HTTP router and holds the per-resource handlers.
//
// Business rules do NOT live here — handlers only parse input, call into
// domain/matching/store, and write responses. See CLAUDE.md conventions.
package http

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/blood-donation/backend/internal/auth"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewRouter wires middleware and routes and returns an http.Handler. tokens
// issues/verifies the JWTs used by the auth middleware and the auth endpoints.
func NewRouter(logger *slog.Logger, db *pgxpool.Pool, tokens *auth.TokenManager) http.Handler {
	r := chi.NewRouter()

	// Middleware runs on every request, in order.
	r.Use(middleware.RequestID)                 // tag each request with an id
	r.Use(middleware.RealIP)                    // trust proxy headers for client IP
	r.Use(middleware.Logger)                    // log method, path, status, duration
	r.Use(middleware.Recoverer)                 // turn panics into 500s, don't crash
	r.Use(middleware.Timeout(30 * time.Second)) // cap request duration

	h := &healthHandler{db: db}

	// Liveness: is the process up? Readiness: can it serve traffic (DB reachable)?
	r.Get("/healthz", h.live)
	r.Get("/readyz", h.ready)

	api := newAPIHandler(logger, db, tokens)
	mountV1(r, api, tokens)

	return r
}

// mountV1 registers all /api/v1 feature routes on r. It is separated from
// NewRouter so tests can mount the same routes against a fake-backed apiHandler.
func mountV1(r chi.Router, api *apiHandler, tokens *auth.TokenManager) {
	r.Route("/api/v1", func(r chi.Router) {
		// --- public: no token required ---
		r.Post("/auth/register", api.registerDonor) // donor self-registration (FR-01)
		r.Post("/auth/login", api.login)
		r.Post("/auth/refresh", api.refresh)

		// --- authenticated: valid access token required ---
		r.Group(func(r chi.Router) {
			r.Use(auth.Authenticator(tokens))

			// Any authenticated user can read their own account.
			r.Get("/auth/me", api.me)

			// Donor-only area (FR-02, FR-03, FR-04). RBAC is enforced here in the
			// backend, never in the clients (FR-12).
			r.Group(func(r chi.Router) {
				r.Use(auth.RequireRole(auth.RoleDonor))

				r.Get("/donor/profile", api.getMyProfile)
				r.Patch("/donor/profile", api.updateMyProfile)

				r.Get("/screening/questions", api.getQuestionnaire)
				r.Post("/screening", api.submitScreening)
				r.Get("/screening/me", api.getMyScreening)

				// Matches & scheduling for the donor (FR-10, FR-11).
				r.Get("/donor/matches", api.listMyMatches)
				r.Post("/donor/matches/{id}/respond", api.respondToMatch)
				r.Post("/donor/matches/{id}/appointment", api.bookAppointment)
				r.Get("/donor/appointments", api.listMyAppointments)
				r.Patch("/donor/appointments/{id}", api.cancelMyAppointment)

				// FCM device-token registration for push notifications (FR-09).
				r.Post("/donor/push-tokens", api.registerPushToken)
				r.Delete("/donor/push-tokens", api.unregisterPushToken)
			})

			// Clinic-staff area: creating and viewing blood requests (FR-05).
			// Membership (which clinic) is resolved per-request in the handlers.
			r.Group(func(r chi.Router) {
				r.Use(auth.RequireRole(auth.RoleClinicEmployee, auth.RoleClinicAdmin))

				r.Post("/requests", api.createRequest)
				r.Get("/requests", api.listClinicRequests)
				r.Get("/requests/{id}", api.getRequest)
				r.Get("/requests/{id}/matches", api.listRequestMatches)

				// Clinic-side appointment views and outcomes (FR-11).
				r.Get("/appointments", api.listClinicAppointments)
				r.Patch("/appointments/{id}", api.updateClinicAppointment)

				// The approval gate is clinic_admin-only (FR-06). A request must
				// be approved before any donor is matched/notified.
				r.Group(func(r chi.Router) {
					r.Use(auth.RequireRole(auth.RoleClinicAdmin))
					r.Post("/requests/{id}/approve", api.approveRequest)
				})
			})
		})
	})
}
