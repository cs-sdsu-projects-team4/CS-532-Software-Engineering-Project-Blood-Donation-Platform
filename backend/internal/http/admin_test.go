package http

import (
	"net/http"
	"testing"
)

// FR-12: the /admin area is system_admin-only.
func TestAdminRBAC(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	cases := []struct {
		name, token string
		want        int
	}{
		{"system admin allowed", e.systemAdminToken(t), http.StatusOK},
		{"clinic admin forbidden", e.adminToken(t), http.StatusForbidden},
		{"donor forbidden", e.donorToken(t), http.StatusForbidden},
		{"anonymous unauthorized", "", http.StatusUnauthorized},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resp := e.do(t, http.MethodGet, "/api/v1/admin/clinics", c.token, nil)
			if resp.StatusCode != c.want {
				t.Fatalf("got %d, want %d", resp.StatusCode, c.want)
			}
			resp.Body.Close()
		})
	}
}

func TestCreateClinic(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()
	tok := e.systemAdminToken(t)

	t.Run("success", func(t *testing.T) {
		resp := e.do(t, http.MethodPost, "/api/v1/admin/clinics", tok,
			map[string]any{"name": "Central Blood Bank", "contact_email": "ops@cbb.example"})
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("got %d, want 201", resp.StatusCode)
		}
		var out clinicResponse
		decodeBody(t, resp, &out)
		if out.ID == 0 || out.Name != "Central Blood Bank" {
			t.Fatalf("unexpected clinic: %+v", out)
		}
	})

	t.Run("missing name", func(t *testing.T) {
		resp := e.do(t, http.MethodPost, "/api/v1/admin/clinics", tok, map[string]any{"name": ""})
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("got %d, want 400", resp.StatusCode)
		}
		resp.Body.Close()
	})

	t.Run("invalid contact email", func(t *testing.T) {
		resp := e.do(t, http.MethodPost, "/api/v1/admin/clinics", tok,
			map[string]any{"name": "X", "contact_email": "not-an-email"})
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("got %d, want 400", resp.StatusCode)
		}
		resp.Body.Close()
	})
}

func TestCreateClinicMemberValidation(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()
	tok := e.systemAdminToken(t)

	// Seed a clinic (id 1) so the not-found path is distinguishable from bad input.
	resp := e.do(t, http.MethodPost, "/api/v1/admin/clinics", tok, map[string]any{"name": "Clinic A"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("seed clinic: got %d", resp.StatusCode)
	}
	resp.Body.Close()

	valid := func() map[string]any {
		return map[string]any{
			"email": "staff1@clinic.example", "password": "password123",
			"full_name": "Staff One", "role": "clinic_employee",
		}
	}

	t.Run("bad role rejected", func(t *testing.T) {
		m := valid()
		m["role"] = "donor"
		resp := e.do(t, http.MethodPost, "/api/v1/admin/clinics/1/members", tok, m)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("got %d, want 400", resp.StatusCode)
		}
		resp.Body.Close()
	})

	t.Run("short password rejected", func(t *testing.T) {
		m := valid()
		m["password"] = "short"
		resp := e.do(t, http.MethodPost, "/api/v1/admin/clinics/1/members", tok, m)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("got %d, want 400", resp.StatusCode)
		}
		resp.Body.Close()
	})

	t.Run("unknown clinic 404", func(t *testing.T) {
		resp := e.do(t, http.MethodPost, "/api/v1/admin/clinics/999/members", tok, valid())
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("got %d, want 404", resp.StatusCode)
		}
		resp.Body.Close()
	})
}
