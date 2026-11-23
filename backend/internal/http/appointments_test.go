package http

import (
	"net/http"
	"testing"
	"time"

	"github.com/blood-donation/backend/internal/store"
)

func futureTime() string { return time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339) }

func TestBookAppointment(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()
	tok := e.donorToken(t)
	e.fake.requests[10] = store.GetBloodRequestRow{ID: 10, ClinicID: 100, Status: store.RequestStatusApproved}
	e.fake.matches[5] = store.DonorMatch{ID: 5, RequestID: 10, DonorUserID: 1, Status: store.MatchStatusAccepted}
	e.fake.matches[8] = store.DonorMatch{ID: 8, RequestID: 10, DonorUserID: 1, Status: store.MatchStatusNotified}

	t.Run("book an accepted match", func(t *testing.T) {
		resp := e.do(t, http.MethodPost, "/api/v1/donor/matches/5/appointment", tok, map[string]any{"scheduled_at": futureTime()})
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("got %d, want 201", resp.StatusCode)
		}
		var out appointmentResponse
		decodeBody(t, resp, &out)
		if out.Status != "scheduled" || out.ClinicID != 100 {
			t.Fatalf("unexpected appointment: %+v", out)
		}
	})

	t.Run("cannot book a match that is not accepted", func(t *testing.T) {
		resp := e.do(t, http.MethodPost, "/api/v1/donor/matches/8/appointment", tok, map[string]any{"scheduled_at": futureTime()})
		if resp.StatusCode != http.StatusConflict {
			t.Fatalf("got %d, want 409", resp.StatusCode)
		}
		resp.Body.Close()
	})
}

func TestBookAppointmentValidation(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()
	tok := e.donorToken(t)

	t.Run("missing scheduled_at", func(t *testing.T) {
		resp := e.do(t, http.MethodPost, "/api/v1/donor/matches/5/appointment", tok, map[string]any{})
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("got %d, want 400", resp.StatusCode)
		}
		resp.Body.Close()
	})

	t.Run("scheduled_at in the past", func(t *testing.T) {
		resp := e.do(t, http.MethodPost, "/api/v1/donor/matches/5/appointment", tok,
			map[string]any{"scheduled_at": "2020-01-01T09:00:00Z"})
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("got %d, want 400", resp.StatusCode)
		}
		resp.Body.Close()
	})
}

func TestCancelMyAppointment(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()
	e.fake.appointments[7] = store.Appointment{ID: 7, DonorUserID: 1, ClinicID: 100, Status: store.AppointmentStatusScheduled}
	e.fake.appointments[8] = store.Appointment{ID: 8, DonorUserID: 2, ClinicID: 100, Status: store.AppointmentStatusScheduled}

	t.Run("cancel own appointment", func(t *testing.T) {
		resp := e.do(t, http.MethodPatch, "/api/v1/donor/appointments/7", e.donorToken(t), nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("got %d, want 200", resp.StatusCode)
		}
		var out appointmentResponse
		decodeBody(t, resp, &out)
		if out.Status != "cancelled" {
			t.Fatalf("status = %q, want cancelled", out.Status)
		}
	})

	t.Run("cannot cancel another donor's appointment", func(t *testing.T) {
		resp := e.do(t, http.MethodPatch, "/api/v1/donor/appointments/8", e.donorToken(t), nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("got %d, want 404", resp.StatusCode)
		}
		resp.Body.Close()
	})
}

func TestUpdateClinicAppointment(t *testing.T) {
	e := newTestEnv(t)
	defer e.close()
	admin := e.adminToken(t) // clinic_admin at clinic 100
	e.fake.appointments[7] = store.Appointment{ID: 7, DonorUserID: 1, ClinicID: 100, Status: store.AppointmentStatusScheduled}
	e.fake.appointments[9] = store.Appointment{ID: 9, DonorUserID: 1, ClinicID: 200, Status: store.AppointmentStatusScheduled}

	t.Run("record outcome", func(t *testing.T) {
		resp := e.do(t, http.MethodPatch, "/api/v1/appointments/7", admin, map[string]any{"status": "completed"})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("got %d, want 200", resp.StatusCode)
		}
		var out appointmentResponse
		decodeBody(t, resp, &out)
		if out.Status != "completed" {
			t.Fatalf("status = %q, want completed", out.Status)
		}
	})

	t.Run("bad status rejected", func(t *testing.T) {
		resp := e.do(t, http.MethodPatch, "/api/v1/appointments/7", admin, map[string]any{"status": "banana"})
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("got %d, want 400", resp.StatusCode)
		}
		resp.Body.Close()
	})

	t.Run("cannot touch another clinic's appointment", func(t *testing.T) {
		resp := e.do(t, http.MethodPatch, "/api/v1/appointments/9", admin, map[string]any{"status": "completed"})
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("got %d, want 404", resp.StatusCode)
		}
		resp.Body.Close()
	})
}
