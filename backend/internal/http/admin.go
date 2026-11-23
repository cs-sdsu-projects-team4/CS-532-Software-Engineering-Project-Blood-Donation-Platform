package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/blood-donation/backend/internal/auth"
	"github.com/blood-donation/backend/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// createClinicBody is a system_admin's new clinic. latitude/longitude are optional
// and, when supplied, feed the PostGIS location used for request defaults/matching.
type createClinicBody struct {
	Name         string   `json:"name"`
	ContactEmail string   `json:"contact_email"`
	ContactPhone string   `json:"contact_phone"`
	Address      string   `json:"address"`
	Latitude     *float64 `json:"latitude"`
	Longitude    *float64 `json:"longitude"`
}

// createClinic handles POST /api/v1/admin/clinics (system_admin). Provisioning
// clinics is an admin action — clinics are never self-registered.
func (h *apiHandler) createClinic(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.IdentityFromContext(r.Context())
	var body createClinicBody
	if !decodeJSON(w, r, &body) {
		return
	}
	if strings.TrimSpace(body.Name) == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "name is required")
		return
	}
	if body.ContactEmail != "" {
		if _, ok := validEmail(body.ContactEmail); !ok {
			writeError(w, http.StatusBadRequest, "invalid_request", "contact_email is not a valid email")
			return
		}
	}
	if msg, ok := checkLatLng(body.Latitude, body.Longitude); !ok {
		writeError(w, http.StatusBadRequest, "invalid_request", msg)
		return
	}

	clinic, err := h.queries.CreateClinic(r.Context(), store.CreateClinicParams{
		Name:         strings.TrimSpace(body.Name),
		ContactEmail: optText(body.ContactEmail),
		ContactPhone: optText(body.ContactPhone),
		Address:      optText(body.Address),
		Latitude:     optFloat(body.Latitude),
		Longitude:    optFloat(body.Longitude),
	})
	if err != nil {
		h.serverError(w, "create clinic", err)
		return
	}
	h.logActivity(r.Context(), id.UserID, "clinic.created", "clinic", clinic.ID, nil)
	writeJSON(w, http.StatusCreated, toClinicResponse(store.GetClinicRow(clinic)))
}

// listClinics handles GET /api/v1/admin/clinics (system_admin).
func (h *apiHandler) listClinics(w http.ResponseWriter, r *http.Request) {
	rows, err := h.queries.ListClinics(r.Context())
	if err != nil {
		h.serverError(w, "list clinics", err)
		return
	}
	out := make([]clinicResponse, 0, len(rows))
	for _, c := range rows {
		out = append(out, toClinicResponse(store.GetClinicRow(c)))
	}
	writeJSON(w, http.StatusOK, out)
}

// getClinic handles GET /api/v1/admin/clinics/{id} (system_admin).
func (h *apiHandler) getClinic(w http.ResponseWriter, r *http.Request) {
	clinicID, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}
	c, err := h.queries.GetClinic(r.Context(), clinicID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "clinic not found")
			return
		}
		h.serverError(w, "get clinic", err)
		return
	}
	writeJSON(w, http.StatusOK, toClinicResponse(c))
}

// validStaffRoles are the roles a clinic member may hold. donor/system_admin are
// not clinic staff and are rejected here.
var validStaffRoles = map[string]bool{"clinic_employee": true, "clinic_admin": true}

// createMemberBody provisions a clinic-staff account. The clinic/staff accounts
// are created by a system_admin — they never self-register.
type createMemberBody struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	FullName string `json:"full_name"`
	Phone    string `json:"phone"`
	Role     string `json:"role"` // clinic_employee or clinic_admin
}

// createClinicMember handles POST /api/v1/admin/clinics/{id}/members (system_admin).
// It creates the staff user AND the membership in one transaction so a failure
// never leaves an account with no clinic (or vice versa).
func (h *apiHandler) createClinicMember(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.IdentityFromContext(r.Context())
	clinicID, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}
	var body createMemberBody
	if !decodeJSON(w, r, &body) {
		return
	}

	email, ok := validEmail(body.Email)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_request", "a valid email is required")
		return
	}
	if len(body.Password) < minPasswordLen {
		writeError(w, http.StatusBadRequest, "invalid_request", "password must be at least 8 characters")
		return
	}
	if strings.TrimSpace(body.FullName) == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "full_name is required")
		return
	}
	if !validStaffRoles[body.Role] {
		writeError(w, http.StatusBadRequest, "invalid_request", "role must be clinic_employee or clinic_admin")
		return
	}

	// The clinic must exist (clean 404 rather than a foreign-key error).
	if _, err := h.queries.GetClinic(r.Context(), clinicID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "clinic not found")
			return
		}
		h.serverError(w, "get clinic", err)
		return
	}

	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		h.serverError(w, "hash password", err)
		return
	}

	ctx := r.Context()
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

	role := store.UserRole(body.Role)
	user, err := qtx.CreateUser(ctx, store.CreateUserParams{
		Email:        email,
		PasswordHash: hash,
		Role:         role,
		FullName:     strings.TrimSpace(body.FullName),
		Phone:        optText(body.Phone),
	})
	if err != nil {
		h.serverError(w, "create staff user", err)
		return
	}
	member, err := qtx.AddClinicMember(ctx, store.AddClinicMemberParams{
		ClinicID: clinicID,
		UserID:   user.ID,
		Role:     role,
	})
	if err != nil {
		h.serverError(w, "add clinic member", err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		h.serverError(w, "commit tx", err)
		return
	}

	h.logActivity(ctx, actor.UserID, "clinic.member_added", "clinic", clinicID, nil)
	writeJSON(w, http.StatusCreated, clinicMemberResponse{
		ID:        member.ID,
		ClinicID:  member.ClinicID,
		UserID:    user.ID,
		Role:      string(user.Role),
		FullName:  user.FullName,
		Email:     user.Email,
		CreatedAt: tsString(member.CreatedAt),
	})
}

// listClinicMembers handles GET /api/v1/admin/clinics/{id}/members (system_admin).
func (h *apiHandler) listClinicMembers(w http.ResponseWriter, r *http.Request) {
	clinicID, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}
	rows, err := h.queries.ListClinicMembers(r.Context(), clinicID)
	if err != nil {
		h.serverError(w, "list clinic members", err)
		return
	}
	out := make([]clinicMemberResponse, 0, len(rows))
	for _, m := range rows {
		out = append(out, clinicMemberResponse{
			ID:        m.ID,
			ClinicID:  m.ClinicID,
			UserID:    m.UserID,
			Role:      string(m.Role),
			FullName:  m.FullName,
			Email:     m.Email,
			CreatedAt: tsString(m.CreatedAt),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// listActivity handles GET /api/v1/admin/activity (system_admin) — the audit feed
// (FR-13). With ?entity_type=&entity_id= it returns that entity's history; without
// them it returns the most recent entries (?limit=, default 50, max 200).
func (h *apiHandler) listActivity(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	entityType := strings.TrimSpace(q.Get("entity_type"))
	entityIDStr := strings.TrimSpace(q.Get("entity_id"))

	var rows []store.ActivityLog
	var err error
	if entityType != "" && entityIDStr != "" {
		eid, perr := strconv.ParseInt(entityIDStr, 10, 64)
		if perr != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "entity_id must be an integer")
			return
		}
		rows, err = h.queries.ListActivityLogsForEntity(r.Context(), store.ListActivityLogsForEntityParams{
			EntityType: pgtype.Text{String: entityType, Valid: true},
			EntityID:   pgtype.Int8{Int64: eid, Valid: true},
		})
	} else {
		rows, err = h.queries.ListRecentActivityLogs(r.Context(), parseLimit(q.Get("limit"), 50, 200))
	}
	if err != nil {
		h.serverError(w, "list activity", err)
		return
	}

	out := make([]activityLogResponse, 0, len(rows))
	for _, a := range rows {
		out = append(out, toActivityLogResponse(a))
	}
	writeJSON(w, http.StatusOK, out)
}

// parseLimit parses a positive limit query value, clamping to [1, max] and
// falling back to def when absent or unparseable.
func parseLimit(s string, def, max int32) int32 {
	if s == "" {
		return def
	}
	v, err := strconv.ParseInt(s, 10, 32)
	if err != nil || v <= 0 {
		return def
	}
	if int32(v) > max {
		return max
	}
	return int32(v)
}

// --- response shapes ---

type activityLogResponse struct {
	ID          int64           `json:"id"`
	ActorUserID *int64          `json:"actor_user_id"`
	Action      string          `json:"action"`
	EntityType  *string         `json:"entity_type"`
	EntityID    *int64          `json:"entity_id"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
	CreatedAt   string          `json:"created_at"`
}

func toActivityLogResponse(a store.ActivityLog) activityLogResponse {
	return activityLogResponse{
		ID:          a.ID,
		ActorUserID: int8Ptr(a.ActorUserID),
		Action:      a.Action,
		EntityType:  textPtr(a.EntityType),
		EntityID:    int8Ptr(a.EntityID),
		Metadata:    json.RawMessage(a.Metadata),
		CreatedAt:   tsString(a.CreatedAt),
	}
}

type clinicMemberResponse struct {
	ID        int64  `json:"id"`
	ClinicID  int64  `json:"clinic_id"`
	UserID    int64  `json:"user_id"`
	Role      string `json:"role"`
	FullName  string `json:"full_name"`
	Email     string `json:"email"`
	CreatedAt string `json:"created_at"`
}

type clinicResponse struct {
	ID           int64    `json:"id"`
	Name         string   `json:"name"`
	ContactEmail *string  `json:"contact_email"`
	ContactPhone *string  `json:"contact_phone"`
	Address      *string  `json:"address"`
	Latitude     *float64 `json:"latitude"`
	Longitude    *float64 `json:"longitude"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at"`
}

func toClinicResponse(c store.GetClinicRow) clinicResponse {
	return clinicResponse{
		ID:           c.ID,
		Name:         c.Name,
		ContactEmail: textPtr(c.ContactEmail),
		ContactPhone: textPtr(c.ContactPhone),
		Address:      textPtr(c.Address),
		Latitude:     floatPtr(c.Latitude),
		Longitude:    floatPtr(c.Longitude),
		CreatedAt:    tsString(c.CreatedAt),
		UpdatedAt:    tsString(c.UpdatedAt),
	}
}
