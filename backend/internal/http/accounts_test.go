package http

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/blood-donation/backend/internal/auth"
	"github.com/blood-donation/backend/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// testEnv bundles everything a handler test needs.
type testEnv struct {
	srv    *httptest.Server
	tokens *auth.TokenManager
	fake   *fakeStore
}

const testPassword = "password123"

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	tm, err := auth.NewTokenManager("test-secret", "test-issuer", 15*time.Minute, time.Hour)
	if err != nil {
		t.Fatalf("token manager: %v", err)
	}

	fake := newFakeStore()
	hash, err := auth.HashPassword(testPassword)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	// Donor user id 1, with a profile.
	fake.addUser(store.User{
		ID: 1, Email: "donor@example.com", PasswordHash: hash,
		Role: store.UserRoleDonor, FullName: "Dana Donor",
	})
	fake.profiles[1] = store.GetDonorProfileRow{
		UserID: 1, BloodType: "O+", IsAgeEligible: true, IsAvailable: true, TravelRadiusM: 10000,
	}
	// A clinic admin user id 2 and a clinic employee user id 3, both staffing
	// clinic 100 (no donor profiles).
	fake.addUser(store.User{
		ID: 2, Email: "admin@clinic.example", PasswordHash: hash,
		Role: store.UserRoleClinicAdmin, FullName: "Avery Admin",
	})
	fake.addUser(store.User{
		ID: 3, Email: "staff@clinic.example", PasswordHash: hash,
		Role: store.UserRoleClinicEmployee, FullName: "Erin Employee",
	})
	fake.memberships[2] = store.ClinicMember{ID: 1, ClinicID: 100, UserID: 2, Role: store.UserRoleClinicAdmin}
	fake.memberships[3] = store.ClinicMember{ID: 2, ClinicID: 100, UserID: 3, Role: store.UserRoleClinicEmployee}

	api := &apiHandler{queries: fake, pool: nil, tokens: tm, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	mountV1(r, api, tm)

	return &testEnv{srv: httptest.NewServer(r), tokens: tm, fake: fake}
}

func (e *testEnv) close() { e.srv.Close() }

// do issues a request with an optional bearer token and JSON body.
func (e *testEnv) do(t *testing.T, method, path, token string, body any) *http.Response {
	t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, e.srv.URL+path, r)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := e.srv.Client().Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

func (e *testEnv) donorToken(t *testing.T) string {
	t.Helper()
	p, err := e.tokens.IssuePair(1, auth.RoleDonor)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	return p.AccessToken
}

func (e *testEnv) adminToken(t *testing.T) string {
	t.Helper()
	p, err := e.tokens.IssuePair(2, auth.RoleClinicAdmin)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	return p.AccessToken
}

func (e *testEnv) employeeToken(t *testing.T) string {
	t.Helper()
	p, err := e.tokens.IssuePair(3, auth.RoleClinicEmployee)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	return p.AccessToken
}

func decodeBody(t *testing.T, resp *http.Response, dst any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		t.Fatalf("decode body: %v", err)
	}
}

func TestLogin(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	t.Run("success", func(t *testing.T) {
		resp := e.do(t, http.MethodPost, "/api/v1/auth/login", "", loginRequest{Email: "donor@example.com", Password: testPassword})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("got %d, want 200", resp.StatusCode)
		}
		var out authResponse
		decodeBody(t, resp, &out)
		if out.Tokens.AccessToken == "" || out.User.Role != "donor" {
			t.Fatalf("unexpected response: %+v", out)
		}
	})

	t.Run("wrong password", func(t *testing.T) {
		resp := e.do(t, http.MethodPost, "/api/v1/auth/login", "", loginRequest{Email: "donor@example.com", Password: "nope"})
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("got %d, want 401", resp.StatusCode)
		}
		resp.Body.Close()
	})

	t.Run("unknown email", func(t *testing.T) {
		resp := e.do(t, http.MethodPost, "/api/v1/auth/login", "", loginRequest{Email: "ghost@example.com", Password: testPassword})
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("got %d, want 401", resp.StatusCode)
		}
		resp.Body.Close()
	})
}

func TestMe(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	t.Run("with token", func(t *testing.T) {
		resp := e.do(t, http.MethodGet, "/api/v1/auth/me", e.donorToken(t), nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("got %d, want 200", resp.StatusCode)
		}
		var out userResponse
		decodeBody(t, resp, &out)
		if out.ID != 1 || out.Email != "donor@example.com" {
			t.Fatalf("unexpected: %+v", out)
		}
	})

	t.Run("without token", func(t *testing.T) {
		resp := e.do(t, http.MethodGet, "/api/v1/auth/me", "", nil)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("got %d, want 401", resp.StatusCode)
		}
		resp.Body.Close()
	})
}

// FR-12: donor-only endpoints reject other roles and anonymous callers.
func TestRBAC_DonorOnly(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	cases := []struct {
		name, token string
		want        int
	}{
		{"donor allowed", e.donorToken(t), http.StatusOK},
		{"clinic admin forbidden", e.adminToken(t), http.StatusForbidden},
		{"anonymous unauthorized", "", http.StatusUnauthorized},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resp := e.do(t, http.MethodGet, "/api/v1/donor/profile", c.token, nil)
			if resp.StatusCode != c.want {
				t.Fatalf("got %d, want %d", resp.StatusCode, c.want)
			}
			resp.Body.Close()
		})
	}
}

func TestProfileUpdate(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()
	tok := e.donorToken(t)

	t.Run("patch availability and radius", func(t *testing.T) {
		avail := false
		radius := int32(5000)
		resp := e.do(t, http.MethodPatch, "/api/v1/donor/profile", tok, updateProfileRequest{
			IsAvailable: &avail, TravelRadiusM: &radius,
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("got %d, want 200", resp.StatusCode)
		}
		var out profileResponse
		decodeBody(t, resp, &out)
		if out.IsAvailable != false || out.TravelRadiusM != 5000 {
			t.Fatalf("patch not applied: %+v", out)
		}
	})

	t.Run("invalid blood type", func(t *testing.T) {
		bt := "XY"
		resp := e.do(t, http.MethodPatch, "/api/v1/donor/profile", tok, updateProfileRequest{BloodType: &bt})
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("got %d, want 400", resp.StatusCode)
		}
		resp.Body.Close()
	})

	t.Run("latitude without longitude", func(t *testing.T) {
		lat := 41.7
		resp := e.do(t, http.MethodPatch, "/api/v1/donor/profile", tok, updateProfileRequest{Latitude: &lat})
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("got %d, want 400", resp.StatusCode)
		}
		resp.Body.Close()
	})
}

func TestScreeningFlow(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()
	tok := e.donorToken(t)

	// Questionnaire is readable.
	resp := e.do(t, http.MethodGet, "/api/v1/screening/questions", tok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("questions: got %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()

	// Build a complete flagged answer set: answer each question with its flag value.
	answers := map[string]bool{}
	for _, q := range []struct {
		id   string
		flag bool
	}{
		{"age_18_or_older", false}, {"feeling_well_today", false},
		{"donated_last_8_weeks", true}, {"recent_tattoo_or_piercing", false},
		{"recent_illness_or_fever", false}, {"pregnant_or_recent_birth", false},
	} {
		answers[q.id] = q.flag
	}

	t.Run("submit complete", func(t *testing.T) {
		resp := e.do(t, http.MethodPost, "/api/v1/screening", tok, screeningRequest{Answers: answers})
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("got %d, want 201", resp.StatusCode)
		}
		var out screeningResult
		decodeBody(t, resp, &out)
		if !out.HasFlags {
			t.Fatal("expected flags for a recent donation")
		}
	})

	t.Run("latest after submit", func(t *testing.T) {
		resp := e.do(t, http.MethodGet, "/api/v1/screening/me", tok, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("got %d, want 200", resp.StatusCode)
		}
		resp.Body.Close()
	})

	t.Run("incomplete rejected", func(t *testing.T) {
		resp := e.do(t, http.MethodPost, "/api/v1/screening", tok, screeningRequest{
			Answers: map[string]bool{"age_18_or_older": false},
		})
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("got %d, want 400", resp.StatusCode)
		}
		resp.Body.Close()
	})

	t.Run("unknown question rejected", func(t *testing.T) {
		resp := e.do(t, http.MethodPost, "/api/v1/screening", tok, screeningRequest{
			Answers: map[string]bool{"made_up_question": true},
		})
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("got %d, want 400", resp.StatusCode)
		}
		resp.Body.Close()
	})
}

// Register validation runs before any DB/transaction work, so these cases are
// exercised with a nil pool.
func TestRegisterValidation(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()

	base := func() map[string]any {
		return map[string]any{
			"email": "new@example.com", "password": "password123",
			"full_name": "New Donor", "age_18_or_older": true,
		}
	}

	cases := []struct {
		name   string
		mutate func(m map[string]any)
	}{
		{"missing age declaration", func(m map[string]any) { m["age_18_or_older"] = false }},
		{"bad email", func(m map[string]any) { m["email"] = "not-an-email" }},
		{"short password", func(m map[string]any) { m["password"] = "short" }},
		{"invalid blood type", func(m map[string]any) { m["blood_type"] = "ZZ" }},
		{"latitude without longitude", func(m map[string]any) { m["latitude"] = 41.7 }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := base()
			c.mutate(m)
			resp := e.do(t, http.MethodPost, "/api/v1/auth/register", "", m)
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("got %d, want 400", resp.StatusCode)
			}
			resp.Body.Close()
		})
	}
}
