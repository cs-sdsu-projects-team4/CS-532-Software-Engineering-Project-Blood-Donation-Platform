package http

import (
	"errors"
	"net/http"

	"github.com/blood-donation/backend/internal/auth"
	"github.com/blood-donation/backend/internal/store"
	"github.com/jackc/pgx/v5"
)

// listMyMatches handles GET /api/v1/donor/matches — the caller's matches with the
// request context they need to decide (FR-10).
func (h *apiHandler) listMyMatches(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.IdentityFromContext(r.Context())
	limit, offset := parsePagination(r)
	rows, err := h.queries.ListMatchesForDonor(r.Context(), store.ListMatchesForDonorParams{
		DonorUserID: id.UserID,
		RowLimit:    limit,
		RowOffset:   offset,
	})
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

// donorMatchResponse is a match from the donor's perspective (with request context).
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
