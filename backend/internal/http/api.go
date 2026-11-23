package http

import (
	"log/slog"

	"github.com/blood-donation/backend/internal/auth"
	"github.com/blood-donation/backend/internal/notify"
	"github.com/blood-donation/backend/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
)

// apiHandler holds the dependencies every feature handler needs. Handlers stay
// thin: they parse/validate input, call the store + domain logic, and write JSON.
//
//   - queries: single-statement, type-safe DB access (sqlc Querier interface, so
//     handlers can be unit-tested against a fake).
//   - pool:    used only where a handler needs a multi-statement transaction
//     (donor registration inserts a user AND a profile atomically).
//   - tokens:  issues/verifies JWTs for login and refresh.
type apiHandler struct {
	queries  store.Querier
	pool     *pgxpool.Pool
	tokens   *auth.TokenManager
	notifier notify.Notifier
	logger   *slog.Logger
}

// newAPIHandler wires the handler dependencies. The pool satisfies store.DBTX, so
// store.New(pool) gives us the query methods for non-transactional calls. The
// notifier defaults to a log-only implementation (no Firebase needed in dev/test);
// production swaps in an FCM-backed Notifier.
func newAPIHandler(logger *slog.Logger, pool *pgxpool.Pool, tokens *auth.TokenManager) *apiHandler {
	return &apiHandler{
		queries:  store.New(pool),
		pool:     pool,
		tokens:   tokens,
		notifier: notify.NewLogNotifier(logger),
		logger:   logger,
	}
}
