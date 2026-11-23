package http

import (
	"net/http"
	"testing"
)

// FR-05/06: only clinic staff may create requests, and only a clinic_admin may
// approve them. This is the approval gate enforced in the backend (FR-12).
func TestRequestRBAC(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	validBody := map[string]any{"blood_type": "A+", "units_needed": 2}

	t.Run("donor cannot create requests", func(t *testing.T) {
		resp := e.do(t, http.MethodPost, "/api/v1/requests", e.donorToken(t), validBody)
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("got %d, want 403", resp.StatusCode)
		}
		resp.Body.Close()
	})

	t.Run("employee can create requests", func(t *testing.T) {
		resp := e.do(t, http.MethodPost, "/api/v1/requests", e.employeeToken(t), validBody)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("got %d, want 201", resp.StatusCode)
		}
		resp.Body.Close()
	})

	t.Run("employee cannot approve (admin only)", func(t *testing.T) {
		resp := e.do(t, http.MethodPost, "/api/v1/requests/1/approve", e.employeeToken(t), nil)
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("got %d, want 403", resp.StatusCode)
		}
		resp.Body.Close()
	})
}

func TestCreateRequestValidation(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()
	tok := e.employeeToken(t)

	cases := []struct {
		name string
		body map[string]any
	}{
		{"invalid blood type", map[string]any{"blood_type": "ZZ", "units_needed": 1}},
		{"unknown not allowed for requests", map[string]any{"blood_type": "unknown", "units_needed": 1}},
		{"non-positive units", map[string]any{"blood_type": "O+", "units_needed": 0}},
		{"bad urgency", map[string]any{"blood_type": "O+", "units_needed": 1, "urgency": "whenever"}},
		{"latitude without longitude", map[string]any{"blood_type": "O+", "units_needed": 1, "latitude": 41.7}},
		{"units too large", map[string]any{"blood_type": "O+", "units_needed": 1000}},
		{"radius too large", map[string]any{"blood_type": "O+", "units_needed": 1, "radius_m": 999999}},
		{"needed_by in the past", map[string]any{"blood_type": "O+", "units_needed": 1, "needed_by": "2020-01-01T00:00:00Z"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resp := e.do(t, http.MethodPost, "/api/v1/requests", tok, c.body)
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("got %d, want 400", resp.StatusCode)
			}
			resp.Body.Close()
		})
	}
}

// The full gate: create (pending) -> approve (runs matching) -> re-approve 409.
func TestApproveFlowAndMatching(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()
	e.fake.matchCount = 3 // the fake pipeline "finds" 3 donors

	// Employee creates a request; it starts pending_approval.
	resp := e.do(t, http.MethodPost, "/api/v1/requests", e.employeeToken(t),
		map[string]any{"blood_type": "A+", "units_needed": 2})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: got %d, want 201", resp.StatusCode)
	}
	var created requestResponse
	decodeBody(t, resp, &created)
	if created.Status != "pending_approval" {
		t.Fatalf("new request status = %q, want pending_approval", created.Status)
	}

	// Admin approves -> matching runs and returns the matched count.
	resp = e.do(t, http.MethodPost, "/api/v1/requests/1/approve", e.adminToken(t), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("approve: got %d, want 200", resp.StatusCode)
	}
	var approved approveResponse
	decodeBody(t, resp, &approved)
	if approved.Request.Status != "approved" {
		t.Fatalf("approved status = %q, want approved", approved.Request.Status)
	}
	if approved.MatchedCount != 3 {
		t.Fatalf("matched_count = %d, want 3", approved.MatchedCount)
	}

	// Re-approving is a no-op guarded by status -> 409.
	resp = e.do(t, http.MethodPost, "/api/v1/requests/1/approve", e.adminToken(t), nil)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("re-approve: got %d, want 409", resp.StatusCode)
	}
	resp.Body.Close()
}

// Lifecycle transitions: a pending request can be cancelled but not fulfilled.
func TestRequestCancelFulfillTransitions(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()
	tok := e.employeeToken(t)

	// Create a pending request (id 1).
	resp := e.do(t, http.MethodPost, "/api/v1/requests", tok,
		map[string]any{"blood_type": "O+", "units_needed": 1})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: got %d", resp.StatusCode)
	}
	resp.Body.Close()

	t.Run("cannot fulfill a pending request", func(t *testing.T) {
		resp := e.do(t, http.MethodPost, "/api/v1/requests/1/fulfill", tok, nil)
		if resp.StatusCode != http.StatusConflict {
			t.Fatalf("got %d, want 409", resp.StatusCode)
		}
		resp.Body.Close()
	})

	t.Run("cancel a pending request", func(t *testing.T) {
		resp := e.do(t, http.MethodPost, "/api/v1/requests/1/cancel", tok, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("got %d, want 200", resp.StatusCode)
		}
		var out requestResponse
		decodeBody(t, resp, &out)
		if out.Status != "cancelled" {
			t.Fatalf("status = %q, want cancelled", out.Status)
		}
	})

	t.Run("cannot cancel an already-cancelled request", func(t *testing.T) {
		resp := e.do(t, http.MethodPost, "/api/v1/requests/1/cancel", tok, nil)
		if resp.StatusCode != http.StatusConflict {
			t.Fatalf("got %d, want 409", resp.StatusCode)
		}
		resp.Body.Close()
	})
}

func TestRespondToMatchValidation(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	resp := e.do(t, http.MethodPost, "/api/v1/donor/matches/5/respond", e.donorToken(t),
		map[string]any{"decision": "maybe"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("got %d, want 400", resp.StatusCode)
	}
	resp.Body.Close()
}
