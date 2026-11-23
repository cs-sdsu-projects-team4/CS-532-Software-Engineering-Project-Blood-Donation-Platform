package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/blood-donation/backend/internal/auth"
	"github.com/blood-donation/backend/internal/domain"
	"github.com/blood-donation/backend/internal/store"
	"github.com/jackc/pgx/v5"
)

// getQuestionnaire handles GET /api/v1/screening/questions — the preliminary
// questionnaire for the client to render (FR-03).
func (h *apiHandler) getQuestionnaire(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"questions": domain.Questionnaire()})
}

// screeningRequest is a submitted questionnaire: a yes/no answer per question id.
type screeningRequest struct {
	Answers map[string]bool `json:"answers"`
}

// screeningResult is returned after a submission or when reading the latest one.
// It carries the non-diagnostic guidance (FR-04); it never states a decision.
type screeningResult struct {
	HasFlags           bool     `json:"has_flags"`
	Guidance           string   `json:"guidance"`
	FlaggedQuestionIDs []string `json:"flagged_question_ids"`
	SubmittedAt        string   `json:"submitted_at"`
}

// submitScreening handles POST /api/v1/screening — store a donor's answers and
// return non-diagnostic guidance (FR-03, FR-04).
func (h *apiHandler) submitScreening(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.IdentityFromContext(r.Context())

	var req screeningRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	// Reject unknown question ids so a client can't smuggle arbitrary keys.
	for key := range req.Answers {
		if !domain.IsScreeningQuestionID(key) {
			writeError(w, http.StatusBadRequest, "invalid_request", "unknown screening question: "+key)
			return
		}
	}

	result, missing, ok := domain.EvaluateScreening(req.Answers)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": map[string]any{
				"code":              "incomplete_screening",
				"message":           "all screening questions must be answered",
				"missing_questions": missing,
			},
		})
		return
	}

	answersJSON, err := json.Marshal(req.Answers)
	if err != nil {
		h.serverError(w, "marshal answers", err)
		return
	}

	row, err := h.queries.CreateScreeningResponse(r.Context(), store.CreateScreeningResponseParams{
		DonorUserID: id.UserID,
		Answers:     answersJSON,
		HasFlags:    result.HasFlags,
	})
	if err != nil {
		h.serverError(w, "store screening", err)
		return
	}

	writeJSON(w, http.StatusCreated, screeningResult{
		HasFlags:           result.HasFlags,
		Guidance:           result.Guidance,
		FlaggedQuestionIDs: result.FlaggedQuestionIDs,
		SubmittedAt:        row.SubmittedAt.Time.Format(time.RFC3339),
	})
}

// getMyScreening handles GET /api/v1/screening/me — the donor's current (latest)
// screening state, with guidance recomputed from the stored answers.
func (h *apiHandler) getMyScreening(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.IdentityFromContext(r.Context())

	row, err := h.queries.GetLatestScreeningResponse(r.Context(), id.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "no screening submitted yet")
			return
		}
		h.serverError(w, "get screening", err)
		return
	}

	var answers map[string]bool
	if err := json.Unmarshal(row.Answers, &answers); err != nil {
		h.serverError(w, "unmarshal answers", err)
		return
	}
	// Recompute guidance from the stored answers so the message stays consistent
	// with the current questionnaire wording.
	result, _, _ := domain.EvaluateScreening(answers)

	writeJSON(w, http.StatusOK, screeningResult{
		HasFlags:           row.HasFlags,
		Guidance:           result.Guidance,
		FlaggedQuestionIDs: result.FlaggedQuestionIDs,
		SubmittedAt:        row.SubmittedAt.Time.Format(time.RFC3339),
	})
}
