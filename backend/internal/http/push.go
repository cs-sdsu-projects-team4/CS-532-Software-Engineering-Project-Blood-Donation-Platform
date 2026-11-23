package http

import (
	"context"
	"net/http"
	"strings"

	"github.com/blood-donation/backend/internal/auth"
	"github.com/blood-donation/backend/internal/notify"
	"github.com/blood-donation/backend/internal/store"
)

var validPlatforms = map[string]bool{"android": true, "ios": true, "web": true}

// pushTokenBody registers an FCM device token for the caller (FR-09).
type pushTokenBody struct {
	Token    string `json:"token"`
	Platform string `json:"platform"` // android, ios, or web
}

// registerPushToken handles POST /api/v1/donor/push-tokens. Re-registering an
// existing token just re-points it at the current user/platform (upsert).
func (h *apiHandler) registerPushToken(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.IdentityFromContext(r.Context())
	var body pushTokenBody
	if !decodeJSON(w, r, &body) {
		return
	}
	token := strings.TrimSpace(body.Token)
	if token == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "token is required")
		return
	}
	if !validPlatforms[body.Platform] {
		writeError(w, http.StatusBadRequest, "invalid_request", "platform must be android, ios, or web")
		return
	}
	saved, err := h.queries.UpsertPushToken(r.Context(), store.UpsertPushTokenParams{
		UserID:   id.UserID,
		Token:    token,
		Platform: body.Platform,
	})
	if err != nil {
		h.serverError(w, "upsert push token", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": saved.ID, "platform": saved.Platform})
}

// deleteTokenBody identifies a token to unregister (e.g. on logout).
type deleteTokenBody struct {
	Token string `json:"token"`
}

// unregisterPushToken handles DELETE /api/v1/donor/push-tokens.
func (h *apiHandler) unregisterPushToken(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.IdentityFromContext(r.Context())
	var body deleteTokenBody
	if !decodeJSON(w, r, &body) {
		return
	}
	token := strings.TrimSpace(body.Token)
	if token == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "token is required")
		return
	}
	if err := h.queries.DeletePushToken(r.Context(), store.DeletePushTokenParams{
		Token:  token,
		UserID: id.UserID,
	}); err != nil {
		h.serverError(w, "delete push token", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// notifyMatchedDonors sends a push to each matched donor's registered devices
// (FR-09). It is best-effort: a notification failure is logged but never fails
// the request that triggered it, and a donor with no tokens is simply skipped.
func (h *apiHandler) notifyMatchedDonors(ctx context.Context, matches []store.DonorMatch, n notify.Notification) {
	if h.notifier == nil {
		return
	}
	for _, m := range matches {
		tokens, err := h.queries.ListPushTokensForUser(ctx, m.DonorUserID)
		if err != nil {
			h.logger.Error("load push tokens", "donor", m.DonorUserID, "err", err)
			continue
		}
		if len(tokens) == 0 {
			continue
		}
		strs := make([]string, len(tokens))
		for i, t := range tokens {
			strs[i] = t.Token
		}
		if err := h.notifier.Notify(ctx, strs, n); err != nil {
			h.logger.Error("notify donor", "donor", m.DonorUserID, "err", err)
		}
	}
}
