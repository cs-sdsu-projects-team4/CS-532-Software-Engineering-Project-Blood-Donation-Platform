package http

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/blood-donation/backend/internal/auth"
	"github.com/blood-donation/backend/internal/domain"
	"github.com/blood-donation/backend/internal/matching"
	"github.com/blood-donation/backend/internal/notify"
	"github.com/blood-donation/backend/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// validUrgencies is the fixed set a request may declare.
var validUrgencies = map[string]bool{"routine": true, "urgent": true, "critical": true}

// createRequestBody is a clinic's new blood request (FR-05). latitude/longitude
// are optional and default to the clinic's location; blood_type must be one of
// the 8 concrete types (a request never names "unknown").
type createRequestBody struct {
	BloodType   string   `json:"blood_type"`
	UnitsNeeded int32    `json:"units_needed"`
	Urgency     string   `json:"urgency"`   // optional; defaults to "routine"
	RadiusM     int32    `json:"radius_m"`  // optional; defaults to 10000
	Latitude    *float64 `json:"latitude"`  // optional
	Longitude   *float64 `json:"longitude"` // optional
	Notes       string   `json:"notes"`     // optional
	NeededBy    *string  `json:"needed_by"` // optional RFC3339 timestamp
}

// createRequest handles POST /api/v1/requests. A clinic_employee or clinic_admin
// creates a request for their own clinic; it starts at pending_approval and is
// not matched until a clinic_admin approves it (FR-06).
func (h *apiHandler) createRequest(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.IdentityFromContext(r.Context())
	member, ok := h.callerMembership(w, r)
	if !ok {
		return
	}

	var req createRequestBody
	if !decodeJSON(w, r, &req) {
		return
	}

	if !domain.IsValidBloodType(domain.BloodType(req.BloodType)) {
		writeError(w, http.StatusBadRequest, "invalid_request",
			"blood_type must be one of O-,O+,A-,A+,B-,B+,AB-,AB+")
		return
	}
	if req.UnitsNeeded <= 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "units_needed must be positive")
		return
	}
	urgency := req.Urgency
	if urgency == "" {
		urgency = "routine"
	}
	if !validUrgencies[urgency] {
		writeError(w, http.StatusBadRequest, "invalid_request", "urgency must be routine, urgent, or critical")
		return
	}
	radius := req.RadiusM
	if radius == 0 {
		radius = 10000
	}
	if radius < 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "radius_m must be positive")
		return
	}
	if msg, ok := checkLatLng(req.Latitude, req.Longitude); !ok {
		writeError(w, http.StatusBadRequest, "invalid_request", msg)
		return
	}
	neededBy, ok := parseOptionalTime(w, req.NeededBy)
	if !ok {
		return
	}

	created, err := h.queries.CreateBloodRequest(r.Context(), store.CreateBloodRequestParams{
		ClinicID:    member.ClinicID,
		CreatedBy:   id.UserID,
		BloodType:   req.BloodType,
		UnitsNeeded: req.UnitsNeeded,
		Urgency:     urgency,
		RadiusM:     radius,
		Latitude:    req.Latitude,
		Longitude:   req.Longitude,
		Notes:       optText(req.Notes),
		NeededBy:    neededBy,
	})
	if err != nil {
		h.serverError(w, "create blood request", err)
		return
	}

	h.logActivity(r.Context(), id.UserID, "request.created", "blood_request", created.ID, nil)
	writeJSON(w, http.StatusCreated, toRequestResponse(store.GetBloodRequestRow(created)))
}

// listClinicRequests handles GET /api/v1/requests — all requests for the caller's
// clinic.
func (h *apiHandler) listClinicRequests(w http.ResponseWriter, r *http.Request) {
	member, ok := h.callerMembership(w, r)
	if !ok {
		return
	}
	rows, err := h.queries.ListBloodRequestsByClinic(r.Context(), member.ClinicID)
	if err != nil {
		h.serverError(w, "list blood requests", err)
		return
	}
	out := make([]requestResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, toRequestResponse(store.GetBloodRequestRow(row)))
	}
	writeJSON(w, http.StatusOK, out)
}

// getRequest handles GET /api/v1/requests/{id} — one request the caller's clinic owns.
func (h *apiHandler) getRequest(w http.ResponseWriter, r *http.Request) {
	member, ok := h.callerMembership(w, r)
	if !ok {
		return
	}
	reqID, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}
	row, ok := h.loadClinicRequest(w, r.Context(), reqID, member.ClinicID)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, toRequestResponse(row))
}

// approveResponse returns the approved request plus how many donors were matched.
type approveResponse struct {
	Request      requestResponse `json:"request"`
	MatchedCount int             `json:"matched_count"`
}

// approveRequest handles POST /api/v1/requests/{id}/approve. clinic_admin only
// (FR-06): it flips the request to approved and runs the matching pipeline. The
// status guard in the query makes a double-approve a no-op (409).
func (h *apiHandler) approveRequest(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.IdentityFromContext(r.Context())
	member, ok := h.callerMembership(w, r)
	if !ok {
		return
	}
	reqID, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}
	// Ownership: the request must belong to the admin's clinic.
	if _, ok := h.loadClinicRequest(w, r.Context(), reqID, member.ClinicID); !ok {
		return
	}

	approved, err := h.queries.ApproveBloodRequest(r.Context(), store.ApproveBloodRequestParams{
		ApprovedBy: pgtype.Int8{Int64: id.UserID, Valid: true},
		ID:         reqID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusConflict, "invalid_state", "request is not pending approval")
			return
		}
		h.serverError(w, "approve request", err)
		return
	}

	// Run the deterministic matching pipeline now that the request is approved.
	engine := matching.NewEngine(h.queries)
	matches, err := engine.Run(r.Context(), matching.Request{ID: approved.ID, BloodType: approved.BloodType})
	if err != nil {
		h.serverError(w, "run matching", err)
		return
	}

	h.logActivity(r.Context(), id.UserID, "request.approved", "blood_request", approved.ID, nil)

	// Notify matched donors (FR-09), best-effort — never fails the approval.
	h.notifyMatchedDonors(r.Context(), matches, notify.Notification{
		Title: "Blood donation request",
		Body:  "A nearby clinic needs " + approved.BloodType + " blood. Open the app to respond.",
		Data:  map[string]string{"request_id": strconv.FormatInt(approved.ID, 10)},
	})

	writeJSON(w, http.StatusOK, approveResponse{
		Request:      toRequestResponse(store.GetBloodRequestRow(approved)),
		MatchedCount: len(matches),
	})
}

// listRequestMatches handles GET /api/v1/requests/{id}/matches — the matched
// donors (with contact details) for a request the caller's clinic owns.
func (h *apiHandler) listRequestMatches(w http.ResponseWriter, r *http.Request) {
	member, ok := h.callerMembership(w, r)
	if !ok {
		return
	}
	reqID, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}
	if _, ok := h.loadClinicRequest(w, r.Context(), reqID, member.ClinicID); !ok {
		return
	}
	rows, err := h.queries.ListMatchesForRequest(r.Context(), reqID)
	if err != nil {
		h.serverError(w, "list request matches", err)
		return
	}
	out := make([]requestMatchResponse, 0, len(rows))
	for _, m := range rows {
		out = append(out, requestMatchResponse{
			ID:          m.ID,
			DonorUserID: m.DonorUserID,
			DistanceM:   floatPtr(m.DistanceM),
			Status:      string(m.Status),
			NotifiedAt:  tsPtr(m.NotifiedAt),
			RespondedAt: tsPtr(m.RespondedAt),
			FullName:    m.FullName,
			Email:       m.Email,
			Phone:       textPtr(m.Phone),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// --- shared helpers for the clinic/request handlers ---

// callerMembership returns the clinic the authenticated staff user belongs to.
// It writes a 403 and returns ok=false when the user staffs no clinic.
func (h *apiHandler) callerMembership(w http.ResponseWriter, r *http.Request) (store.ClinicMember, bool) {
	id, _ := auth.IdentityFromContext(r.Context())
	m, err := h.queries.GetClinicMembershipByUser(r.Context(), id.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusForbidden, "forbidden", "your account is not attached to a clinic")
			return store.ClinicMember{}, false
		}
		h.serverError(w, "get clinic membership", err)
		return store.ClinicMember{}, false
	}
	return m, true
}

// loadClinicRequest fetches a request and verifies it belongs to clinicID,
// writing a 404 otherwise (we don't reveal other clinics' requests).
func (h *apiHandler) loadClinicRequest(w http.ResponseWriter, ctx context.Context, reqID, clinicID int64) (store.GetBloodRequestRow, bool) {
	row, err := h.queries.GetBloodRequest(ctx, reqID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "request not found")
			return store.GetBloodRequestRow{}, false
		}
		h.serverError(w, "get blood request", err)
		return store.GetBloodRequestRow{}, false
	}
	if row.ClinicID != clinicID {
		writeError(w, http.StatusNotFound, "not_found", "request not found")
		return store.GetBloodRequestRow{}, false
	}
	return row, true
}

// logActivity records an audit entry (FR-13) best-effort: a logging failure must
// never fail the underlying action, so we log the error and move on.
func (h *apiHandler) logActivity(ctx context.Context, actorID int64, action, entityType string, entityID int64, metadata []byte) {
	_, err := h.queries.CreateActivityLog(ctx, store.CreateActivityLogParams{
		ActorUserID: pgtype.Int8{Int64: actorID, Valid: true},
		Action:      action,
		EntityType:  pgtype.Text{String: entityType, Valid: true},
		EntityID:    pgtype.Int8{Int64: entityID, Valid: true},
		Metadata:    metadata,
	})
	if err != nil {
		h.logger.Error("activity log failed", "action", action, "entity_id", entityID, "err", err)
	}
}

// parseIDParam reads a positive int64 path parameter, writing a 400 on failure.
func parseIDParam(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	v, err := strconv.ParseInt(chi.URLParam(r, name), 10, 64)
	if err != nil || v <= 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", name+" must be a positive integer")
		return 0, false
	}
	return v, true
}

// parseOptionalTime parses an optional RFC3339 timestamp, writing a 400 on a
// malformed value. A nil input yields an unset (NULL) value.
func parseOptionalTime(w http.ResponseWriter, s *string) (pgtype.Timestamptz, bool) {
	if s == nil {
		return pgtype.Timestamptz{Valid: false}, true
	}
	t, err := time.Parse(time.RFC3339, *s)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "needed_by must be an RFC3339 timestamp")
		return pgtype.Timestamptz{}, false
	}
	return pgtype.Timestamptz{Time: t, Valid: true}, true
}

// --- response shapes ---

type requestResponse struct {
	ID          int64    `json:"id"`
	ClinicID    int64    `json:"clinic_id"`
	CreatedBy   int64    `json:"created_by"`
	BloodType   string   `json:"blood_type"`
	UnitsNeeded int32    `json:"units_needed"`
	Urgency     string   `json:"urgency"`
	Status      string   `json:"status"`
	ApprovedBy  *int64   `json:"approved_by"`
	ApprovedAt  *string  `json:"approved_at"`
	RadiusM     int32    `json:"radius_m"`
	Latitude    *float64 `json:"latitude"`
	Longitude   *float64 `json:"longitude"`
	Notes       *string  `json:"notes"`
	NeededBy    *string  `json:"needed_by"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

type requestMatchResponse struct {
	ID          int64    `json:"id"`
	DonorUserID int64    `json:"donor_user_id"`
	DistanceM   *float64 `json:"distance_m"`
	Status      string   `json:"status"`
	NotifiedAt  *string  `json:"notified_at"`
	RespondedAt *string  `json:"responded_at"`
	FullName    string   `json:"full_name"`
	Email       string   `json:"email"`
	Phone       *string  `json:"phone"`
}

func toRequestResponse(r store.GetBloodRequestRow) requestResponse {
	return requestResponse{
		ID:          r.ID,
		ClinicID:    r.ClinicID,
		CreatedBy:   r.CreatedBy,
		BloodType:   r.BloodType,
		UnitsNeeded: r.UnitsNeeded,
		Urgency:     r.Urgency,
		Status:      string(r.Status),
		ApprovedBy:  int8Ptr(r.ApprovedBy),
		ApprovedAt:  tsPtr(r.ApprovedAt),
		RadiusM:     r.RadiusM,
		Latitude:    floatPtr(r.Latitude),
		Longitude:   floatPtr(r.Longitude),
		Notes:       textPtr(r.Notes),
		NeededBy:    tsPtr(r.NeededBy),
		CreatedAt:   tsString(r.CreatedAt),
		UpdatedAt:   tsString(r.UpdatedAt),
	}
}

// int8Ptr maps a nullable pg int8 to *int64 for JSON.
func int8Ptr(v pgtype.Int8) *int64 {
	if !v.Valid {
		return nil
	}
	x := v.Int64
	return &x
}

// tsString formats a non-null timestamp as RFC3339 (empty when unset).
func tsString(v pgtype.Timestamptz) string {
	if !v.Valid {
		return ""
	}
	return v.Time.Format(time.RFC3339)
}

// tsPtr maps a nullable timestamp to *string (RFC3339) for JSON.
func tsPtr(v pgtype.Timestamptz) *string {
	if !v.Valid {
		return nil
	}
	s := v.Time.Format(time.RFC3339)
	return &s
}
