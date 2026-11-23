// Package bootstrap performs idempotent first-run setup. Currently it ensures a
// system_admin account exists so the /admin API is reachable — nothing self-
// registers as system_admin, so without this the admin endpoints would be
// unusable on a fresh database.
package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"net/mail"
	"strings"

	"github.com/blood-donation/backend/internal/auth"
	"github.com/blood-donation/backend/internal/store"
	"github.com/jackc/pgx/v5/pgtype"
)

const minAdminPasswordLen = 8

// AdminStore is the slice of the data layer EnsureSystemAdmin needs. store.Querier
// satisfies it; tests supply a fake.
type AdminStore interface {
	EmailExists(ctx context.Context, email string) (bool, error)
	CreateUser(ctx context.Context, arg store.CreateUserParams) (store.CreateUserRow, error)
}

// EnsureSystemAdmin makes sure a system_admin with the given email exists. It is
// idempotent and safe to run on every startup:
//   - empty email or password: the feature is disabled, nothing happens;
//   - the email already exists: it is left untouched;
//   - otherwise: a system_admin is created with the given password.
//
// It returns an error only for misconfiguration (bad email / weak password) or a
// real data-layer failure, so the caller can decide whether to fail fast.
func EnsureSystemAdmin(ctx context.Context, q AdminStore, email, password string, logger *slog.Logger) error {
	if email == "" || password == "" {
		logger.Info("bootstrap admin not configured; skipping")
		return nil
	}
	email = strings.TrimSpace(strings.ToLower(email))
	if _, err := mail.ParseAddress(email); err != nil {
		return fmt.Errorf("bootstrap: BOOTSTRAP_ADMIN_EMAIL is not a valid email")
	}
	if len(password) < minAdminPasswordLen {
		return fmt.Errorf("bootstrap: BOOTSTRAP_ADMIN_PASSWORD must be at least %d characters", minAdminPasswordLen)
	}

	exists, err := q.EmailExists(ctx, email)
	if err != nil {
		return fmt.Errorf("bootstrap: check admin email: %w", err)
	}
	if exists {
		logger.Info("bootstrap admin already exists; leaving as-is", "email", email)
		return nil
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return fmt.Errorf("bootstrap: hash admin password: %w", err)
	}
	if _, err := q.CreateUser(ctx, store.CreateUserParams{
		Email:        email,
		PasswordHash: hash,
		Role:         store.UserRoleSystemAdmin,
		FullName:     "System Administrator",
		Phone:        pgtype.Text{Valid: false},
	}); err != nil {
		return fmt.Errorf("bootstrap: create admin: %w", err)
	}
	logger.Info("created bootstrap system_admin", "email", email)
	return nil
}
