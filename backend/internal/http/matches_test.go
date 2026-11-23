package http

import (
	"net/http"
	"testing"

	"github.com/blood-donation/backend/internal/store"
)

func TestRespondToMatch(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()
	tok := e.donorToken(t)
	// A notified match owned by donor (user 1).
	e.fake.matches[6] = store.DonorMatch{ID: 6, RequestID: 10, DonorUserID: 1, Status: store.MatchStatusNotified}

	t.Run("accept a notified match", func(t *testing.T) {
		resp := e.do(t, http.MethodPost, "/api/v1/donor/matches/6/respond", tok, map[string]any{"decision": "accept"})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("got %d, want 200", resp.StatusCode)
		}
		var out struct {
			Status string `json:"status"`
		}
		decodeBody(t, resp, &out)
		if out.Status != "accepted" {
			t.Fatalf("status = %q, want accepted", out.Status)
		}
	})

	t.Run("cannot respond twice", func(t *testing.T) {
		resp := e.do(t, http.MethodPost, "/api/v1/donor/matches/6/respond", tok, map[string]any{"decision": "decline"})
		if resp.StatusCode != http.StatusConflict {
			t.Fatalf("got %d, want 409", resp.StatusCode)
		}
		resp.Body.Close()
	})
}

func TestRespondToMatch_NotOwned(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()
	// Match belongs to a different donor (user 2); donor 1 may not respond.
	e.fake.matches[9] = store.DonorMatch{ID: 9, RequestID: 10, DonorUserID: 2, Status: store.MatchStatusNotified}
	resp := e.do(t, http.MethodPost, "/api/v1/donor/matches/9/respond", e.donorToken(t), map[string]any{"decision": "accept"})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("got %d, want 409", resp.StatusCode)
	}
	resp.Body.Close()
}
