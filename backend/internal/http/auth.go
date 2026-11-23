package http

import (
	"errors"
	"net/http"
	"strings"

	"github.com/blood-donation/backend/internal/auth"
	"github.com/blood-donation/backend/internal/domain"
	"github.com/blood-donation/backend/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// registerRequest is the donor self-registration body (FR-01). Only donors can
// self-register; clinic/admin accounts are provisioned by a system_admin.
type registerRequest struct {
	Email         string   `json:"email"`
	Password      string   `json:"password"`
	FullName      string   `json:"full_name"`
	Phone         string   `json:"phone"`           // optional
	BloodType     string   `json:"blood_type"`      // optional; defaults to "unknown"
	Age18OrOlder  bool     `json:"age_18_or_older"` // required true
	TravelRadiusM int32    `json:"travel_radius_m"` // optional; defaults to 10000
	Latitude      *float64 `json:"latitude"`        // optional
	Longitude     *float64 `json:"longitude"`       // optional
}

// authResponse is returned by register/login: the token pair plus a safe view of
// the user (never the password hash).
type authResponse struct {
	User   userResponse   `json:"user"`
	Tokens auth.TokenPair `json:"tokens"`
}

// registerDonor handles POST /api/v1/auth/register.
func (h *apiHandler) registerDonor(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	// --- validate (server-side; never trust the client) ---
	email, ok := validEmail(req.Email)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_request", "a valid email is required")
		return
	}
	if len(req.Password) < minPasswordLen {
		writeError(w, http.StatusBadRequest, "invalid_request", "password must be at least 8 characters")
		return
	}
	if strings.TrimSpace(req.FullName) == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "full_name is required")
		return
	}
	// FR-01: registration requires an 18-or-older self declaration.
	if !req.Age18OrOlder {
		writeError(w, http.StatusBadRequest, "invalid_request",
			"you must confirm you are 18 or older to register as a donor")
		return
	}
	bloodType := domain.BloodUnknown
	if req.BloodType != "" {
		bloodType = domain.BloodType(req.BloodType)
		if !domain.IsValidDonorBloodType(bloodType) {
			writeError(w, http.StatusBadRequest, "invalid_request",
				"blood_type must be one of O-,O+,A-,A+,B-,B+,AB-,AB+ or 'unknown'")
			return
		}
	}
	radius := req.TravelRadiusM
	if radius == 0 {
		radius = 10000
	}
	if radius < 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "travel_radius_m must be positive")
		return
	}
	if msg, ok := checkLatLng(req.Latitude, req.Longitude); !ok {
		writeError(w, http.StatusBadRequest, "invalid_request", msg)
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		h.serverError(w, "hash password", err)
		return
	}

	ctx := r.Context()

	// Donor creation touches two tables (users + donor_profiles); do it in one
	// transaction so a failure never leaves an orphaned user.
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		h.serverError(w, "begin tx", err)
		return
	}
	defer tx.Rollback(ctx) // no-op after a successful commit
	qtx := store.New(tx)

	exists, err := qtx.EmailExists(ctx, email)
	if err != nil {
		h.serverError(w, "check email", err)
		return
	}
	if exists {
		writeError(w, http.StatusConflict, "email_taken", "an account with this email already exists")
		return
	}

	user, err := qtx.CreateUser(ctx, store.CreateUserParams{
		Email:        email,
		PasswordHash: hash,
		Role:         store.UserRoleDonor,
		FullName:     strings.TrimSpace(req.FullName),
		Phone:        optText(req.Phone),
	})
	if err != nil {
		h.serverError(w, "create user", err)
		return
	}

	if _, err := qtx.CreateDonorProfile(ctx, store.CreateDonorProfileParams{
		UserID:        user.ID,
		BloodType:     string(bloodType),
		IsAgeEligible: true,
		IsAvailable:   true,
		TravelRadiusM: radius,
		Longitude:     optFloat(req.Longitude),
		Latitude:      optFloat(req.Latitude),
	}); err != nil {
		h.serverError(w, "create donor profile", err)
		return
	}

	if err := tx.Commit(ctx); err != nil {
		h.serverError(w, "commit tx", err)
		return
	}

	tokens, err := h.tokens.IssuePair(user.ID, auth.RoleDonor)
	if err != nil {
		h.serverError(w, "issue tokens", err)
		return
	}

	writeJSON(w, http.StatusCreated, authResponse{
		User: userResponse{
			ID:       user.ID,
			Email:    user.Email,
			Role:     string(user.Role),
			FullName: user.FullName,
			Phone:    textPtr(user.Phone),
		},
		Tokens: tokens,
	})
}

// loginRequest is the credentials body.
type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// login handles POST /api/v1/auth/login.
func (h *apiHandler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	email, ok := validEmail(req.Email)
	if !ok || req.Password == "" {
		// Same generic error as a bad password: don't reveal which field was wrong.
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
		return
	}

	user, err := h.queries.GetUserByEmail(r.Context(), email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
			return
		}
		h.serverError(w, "get user", err)
		return
	}

	if err := auth.VerifyPassword(req.Password, user.PasswordHash); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
		return
	}

	tokens, err := h.tokens.IssuePair(user.ID, auth.Role(user.Role))
	if err != nil {
		h.serverError(w, "issue tokens", err)
		return
	}
	writeJSON(w, http.StatusOK, authResponse{
		User: userResponse{
			ID:       user.ID,
			Email:    user.Email,
			Role:     string(user.Role),
			FullName: user.FullName,
			Phone:    textPtr(user.Phone),
		},
		Tokens: tokens,
	})
}

// refreshRequest carries a previously issued refresh token.
type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// refresh handles POST /api/v1/auth/refresh. It verifies the refresh token and
// re-reads the user so the new access token reflects the current role.
func (h *apiHandler) refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	userID, _, err := h.tokens.Parse(req.RefreshToken, auth.RefreshToken)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_token", "invalid or expired refresh token")
		return
	}

	user, err := h.queries.GetUserByID(r.Context(), userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusUnauthorized, "invalid_token", "account no longer exists")
			return
		}
		h.serverError(w, "get user", err)
		return
	}

	tokens, err := h.tokens.IssuePair(user.ID, auth.Role(user.Role))
	if err != nil {
		h.serverError(w, "issue tokens", err)
		return
	}
	writeJSON(w, http.StatusOK, tokens)
}

// me handles GET /api/v1/auth/me — the authenticated caller's own account.
func (h *apiHandler) me(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.IdentityFromContext(r.Context()) // guaranteed present by Authenticator
	user, err := h.queries.GetUserByID(r.Context(), id.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "account not found")
			return
		}
		h.serverError(w, "get user", err)
		return
	}
	writeJSON(w, http.StatusOK, userResponse{
		ID:       user.ID,
		Email:    user.Email,
		Role:     string(user.Role),
		FullName: user.FullName,
		Phone:    textPtr(user.Phone),
	})
}

// --- small shared helpers ---

// userResponse is the public view of a user. No password hash, ever.
type userResponse struct {
	ID       int64   `json:"id"`
	Email    string  `json:"email"`
	Role     string  `json:"role"`
	FullName string  `json:"full_name"`
	Phone    *string `json:"phone"`
}

// optText maps an optional string to a nullable pg text value.
func optText(s string) pgtype.Text {
	s = strings.TrimSpace(s)
	if s == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: s, Valid: true}
}

// textPtr maps a nullable pg text value back to *string for JSON.
func textPtr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	s := t.String
	return &s
}

// optFloat maps an optional float pointer to a nullable pg float8 value.
func optFloat(f *float64) pgtype.Float8 {
	if f == nil {
		return pgtype.Float8{Valid: false}
	}
	return pgtype.Float8{Float64: *f, Valid: true}
}

// serverError logs the internal detail and returns a generic 500 (we never leak
// internal errors to clients).
func (h *apiHandler) serverError(w http.ResponseWriter, context string, err error) {
	h.logger.Error("request failed", "at", context, "err", err)
	writeError(w, http.StatusInternalServerError, "internal_error", "something went wrong")
}
