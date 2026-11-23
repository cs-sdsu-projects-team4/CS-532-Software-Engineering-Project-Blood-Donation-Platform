package http

import (
	"errors"
	"net/http"

	"github.com/blood-donation/backend/internal/auth"
	"github.com/blood-donation/backend/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// listMyMatches handles GET /api/v1/donor/matches — the caller's matches with the
// request context they need to decide (FR-10).
func (h *apiHandler) listMyMatches(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.IdentityFromContext(r.Context())
	rows, err := h.queries.ListMatchesForDonor(r.Context(), id.UserID)
	if err != nil {
		h.serverError(w, "list donor matches", err)
		return
	}
	out := make([]donorMatchResponse, 0, len(rows))
	for _, m := range rows {
		out = append(out, donorMatchResponse{
			ID:          m.ID,
			RequestID:   m.RequestID,
			DistanceM:   floatPtr(m.DistanceM),
			Status:      string(m.Status),
			NotifiedAt:  tsPtr(m.NotifiedAt),
			RespondedAt: tsPtr(m.RespondedAt),
			BloodType:   m.BloodType,
			Urgency:     m.Urgency,
			NeededBy:    tsPtr(m.NeededBy),
			ClinicID:    m.ClinicID,
			ClinicName:  m.ClinicName,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// respondBody is a donor's accept/decline decision on a match.
type respondBody struct {
	Decision string `json:"decision"` // "accept" or "decline"
}

// respondToMatch handles POST /api/v1/donor/matches/{id}/respond (FR-10). The
// query's guards ensure only the owning donor can respond, and only once (from
// the 'notified' state), so a decision can't be forged or replayed.
func (h *apiHandler) respondToMatch(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.IdentityFromContext(r.Context())
	matchID, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}
	var body respondBody
	if !decodeJSON(w, r, &body) {
		return
	}
	var status store.MatchStatus
	switch body.Decision {
	case "accept":
		status = store.MatchStatusAccepted
	case "decline":
		status = store.MatchStatusDeclined
	default:
		writeError(w, http.StatusBadRequest, "invalid_request", "decision must be accept or decline")
		return
	}

	updated, err := h.queries.RespondToMatch(r.Context(), store.RespondToMatchParams{
		Status:      status,
		ID:          matchID,
		DonorUserID: id.UserID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Either not this donor's match, or it's already been responded to.
			writeError(w, http.StatusConflict, "invalid_state", "this match is not awaiting your response")
			return
		}
		h.serverError(w, "respond to match", err)
		return
	}

	h.logActivity(r.Context(), id.UserID, "match."+body.Decision+"ed", "donor_match", updated.ID, nil)
	writeJSON(w, http.StatusOK, map[string]any{
		"id":           updated.ID,
		"request_id":   updated.RequestID,
		"status":       string(updated.Status),
		"responded_at": tsPtr(updated.RespondedAt),
	})
}

// bookBody is the chosen appointment slot.
type bookBody struct {
	ScheduledAt string `json:"scheduled_at"` // RFC3339
}

// bookAppointment handles POST /api/v1/donor/matches/{id}/appointment (FR-11).
// The donor must have accepted the match first. The DB's partial unique indexes
// reject a double-booking (same donor or same clinic slot) → 409.
func (h *apiHandler) bookAppointment(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.IdentityFromContext(r.Context())
	matchID, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}
	var body bookBody
	if !decodeJSON(w, r, &body) {
		return
	}
	scheduledAt, ok := parseOptionalTime(w, &body.ScheduledAt)
	if !ok {
		return
	}
	if !scheduledAt.Valid {
		writeError(w, http.StatusBadRequest, "invalid_request", "scheduled_at is required")
		return
	}

	match, err := h.queries.GetDonorMatch(r.Context(), matchID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "match not found")
			return
		}
		h.serverError(w, "get donor match", err)
		return
	}
	if match.DonorUserID != id.UserID {
		writeError(w, http.StatusNotFound, "not_found", "match not found")
		return
	}
	if match.Status != store.MatchStatusAccepted {
		writeError(w, http.StatusConflict, "invalid_state", "accept the match before booking an appointment")
		return
	}

	// The match's request tells us which clinic the slot is at.
	req, err := h.queries.GetBloodRequest(r.Context(), match.RequestID)
	if err != nil {
		h.serverError(w, "get request for appointment", err)
		return
	}

	appt, err := h.queries.CreateAppointment(r.Context(), store.CreateAppointmentParams{
		RequestID:   match.RequestID,
		DonorUserID: id.UserID,
		ClinicID:    req.ClinicID,
		ScheduledAt: scheduledAt,
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "slot_taken", "that time is already booked")
			return
		}
		h.serverError(w, "create appointment", err)
		return
	}

	h.logActivity(r.Context(), id.UserID, "appointment.booked", "appointment", appt.ID, nil)
	writeJSON(w, http.StatusCreated, toAppointmentResponse(appt))
}

// listMyAppointments handles GET /api/v1/donor/appointments.
func (h *apiHandler) listMyAppointments(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.IdentityFromContext(r.Context())
	rows, err := h.queries.ListAppointmentsForDonor(r.Context(), id.UserID)
	if err != nil {
		h.serverError(w, "list donor appointments", err)
		return
	}
	writeAppointments(w, rows)
}

// cancelMyAppointment handles PATCH /api/v1/donor/appointments/{id} — a donor
// cancels their own slot, freeing it for rebooking.
func (h *apiHandler) cancelMyAppointment(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.IdentityFromContext(r.Context())
	apptID, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}
	appt, err := h.queries.GetAppointment(r.Context(), apptID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "appointment not found")
			return
		}
		h.serverError(w, "get appointment", err)
		return
	}
	if appt.DonorUserID != id.UserID {
		writeError(w, http.StatusNotFound, "not_found", "appointment not found")
		return
	}
	updated, err := h.queries.SetAppointmentStatus(r.Context(), store.SetAppointmentStatusParams{
		Status: store.AppointmentStatusCancelled,
		ID:     apptID,
	})
	if err != nil {
		h.serverError(w, "cancel appointment", err)
		return
	}
	h.logActivity(r.Context(), id.UserID, "appointment.cancelled", "appointment", updated.ID, nil)
	writeJSON(w, http.StatusOK, toAppointmentResponse(updated))
}

// listClinicAppointments handles GET /api/v1/appointments — all appointments at
// the caller's clinic (clinic staff).
func (h *apiHandler) listClinicAppointments(w http.ResponseWriter, r *http.Request) {
	member, ok := h.callerMembership(w, r)
	if !ok {
		return
	}
	rows, err := h.queries.ListAppointmentsForClinic(r.Context(), member.ClinicID)
	if err != nil {
		h.serverError(w, "list clinic appointments", err)
		return
	}
	writeAppointments(w, rows)
}

// apptStatusBody is a clinic staff update to an appointment's outcome.
type apptStatusBody struct {
	Status string `json:"status"` // completed, no_show, or cancelled
}

var validApptStatusUpdates = map[string]bool{"completed": true, "no_show": true, "cancelled": true}

// updateClinicAppointment handles PATCH /api/v1/appointments/{id} — clinic staff
// record an appointment's outcome.
func (h *apiHandler) updateClinicAppointment(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.IdentityFromContext(r.Context())
	member, ok := h.callerMembership(w, r)
	if !ok {
		return
	}
	apptID, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}
	var body apptStatusBody
	if !decodeJSON(w, r, &body) {
		return
	}
	if !validApptStatusUpdates[body.Status] {
		writeError(w, http.StatusBadRequest, "invalid_request", "status must be completed, no_show, or cancelled")
		return
	}

	appt, err := h.queries.GetAppointment(r.Context(), apptID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "appointment not found")
			return
		}
		h.serverError(w, "get appointment", err)
		return
	}
	if appt.ClinicID != member.ClinicID {
		writeError(w, http.StatusNotFound, "not_found", "appointment not found")
		return
	}

	updated, err := h.queries.SetAppointmentStatus(r.Context(), store.SetAppointmentStatusParams{
		Status: store.AppointmentStatus(body.Status),
		ID:     apptID,
	})
	if err != nil {
		h.serverError(w, "update appointment", err)
		return
	}
	h.logActivity(r.Context(), id.UserID, "appointment."+body.Status, "appointment", updated.ID, nil)
	writeJSON(w, http.StatusOK, toAppointmentResponse(updated))
}

// --- response shapes & helpers ---

type donorMatchResponse struct {
	ID          int64    `json:"id"`
	RequestID   int64    `json:"request_id"`
	DistanceM   *float64 `json:"distance_m"`
	Status      string   `json:"status"`
	NotifiedAt  *string  `json:"notified_at"`
	RespondedAt *string  `json:"responded_at"`
	BloodType   string   `json:"blood_type"`
	Urgency     string   `json:"urgency"`
	NeededBy    *string  `json:"needed_by"`
	ClinicID    int64    `json:"clinic_id"`
	ClinicName  string   `json:"clinic_name"`
}

type appointmentResponse struct {
	ID          int64  `json:"id"`
	RequestID   int64  `json:"request_id"`
	DonorUserID int64  `json:"donor_user_id"`
	ClinicID    int64  `json:"clinic_id"`
	ScheduledAt string `json:"scheduled_at"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func toAppointmentResponse(a store.Appointment) appointmentResponse {
	return appointmentResponse{
		ID:          a.ID,
		RequestID:   a.RequestID,
		DonorUserID: a.DonorUserID,
		ClinicID:    a.ClinicID,
		ScheduledAt: tsString(a.ScheduledAt),
		Status:      string(a.Status),
		CreatedAt:   tsString(a.CreatedAt),
		UpdatedAt:   tsString(a.UpdatedAt),
	}
}

func writeAppointments(w http.ResponseWriter, rows []store.Appointment) {
	out := make([]appointmentResponse, 0, len(rows))
	for _, a := range rows {
		out = append(out, toAppointmentResponse(a))
	}
	writeJSON(w, http.StatusOK, out)
}

// isUniqueViolation reports whether err is a Postgres unique-constraint violation
// (SQLSTATE 23505) — used to turn a no-double-booking conflict into a 409.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
