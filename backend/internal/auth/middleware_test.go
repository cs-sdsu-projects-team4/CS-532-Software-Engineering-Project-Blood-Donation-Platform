package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// okHandler returns 200 and is what the middleware chain protects.
func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestAuthenticator(t *testing.T) {
	tm := newTM(t, time.Minute, time.Hour)
	pair, _ := tm.IssuePair(5, RoleDonor)
	handler := Authenticator(tm)(okHandler())

	cases := []struct {
		name       string
		authHeader string
		wantStatus int
	}{
		{"valid bearer", "Bearer " + pair.AccessToken, http.StatusOK},
		{"case-insensitive scheme", "bearer " + pair.AccessToken, http.StatusOK},
		{"missing header", "", http.StatusUnauthorized},
		{"wrong scheme", "Basic abc", http.StatusUnauthorized},
		{"garbage token", "Bearer not.a.jwt", http.StatusUnauthorized},
		{"refresh token rejected", "Bearer " + pair.RefreshToken, http.StatusUnauthorized},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/x", nil)
			if c.authHeader != "" {
				req.Header.Set("Authorization", c.authHeader)
			}
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			if rr.Code != c.wantStatus {
				t.Errorf("got %d, want %d", rr.Code, c.wantStatus)
			}
		})
	}
}

// FR-12: each role can/can't reach a role-gated endpoint.
func TestRequireRole(t *testing.T) {
	tm := newTM(t, time.Minute, time.Hour)
	// Endpoint allows only clinic_admin.
	chain := Authenticator(tm)(RequireRole(RoleClinicAdmin)(okHandler()))

	cases := []struct {
		role       Role
		wantStatus int
	}{
		{RoleClinicAdmin, http.StatusOK},
		{RoleClinicEmployee, http.StatusForbidden},
		{RoleDonor, http.StatusForbidden},
		{RoleSystemAdmin, http.StatusForbidden}, // not auto-granted; must be listed
	}
	for _, c := range cases {
		t.Run(string(c.role), func(t *testing.T) {
			pair, _ := tm.IssuePair(1, c.role)
			req := httptest.NewRequest(http.MethodPost, "/approve", nil)
			req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
			rr := httptest.NewRecorder()
			chain.ServeHTTP(rr, req)
			if rr.Code != c.wantStatus {
				t.Errorf("role %s: got %d, want %d", c.role, rr.Code, c.wantStatus)
			}
		})
	}
}

func TestRequireRole_WithoutIdentity(t *testing.T) {
	// RequireRole mounted without Authenticator → no identity on context → 401.
	handler := RequireRole(RoleDonor)(okHandler())
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want 401", rr.Code)
	}
}
