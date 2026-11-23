package http

import (
	"errors"
	"net/http"
	"strings"

	"github.com/blood-donation/backend/internal/auth"
	"github.com/blood-donation/backend/internal/store"
	"github.com/jackc/pgx/v5"
)

// createClinicBody is a system_admin's new clinic. latitude/longitude are optional
// and, when supplied, feed the PostGIS location used for request defaults/matching.
type createClinicBody struct {
	Name         string   `json:"name"`
	ContactEmail string   `json:"contact_email"`
	ContactPhone string   `json:"contact_phone"`
	Address      string   `json:"address"`
	Latitude     *float64 `json:"latitude"`
	Longitude    *float64 `json:"longitude"`
}

// createClinic handles POST /api/v1/admin/clinics (system_admin). Provisioning
// clinics is an admin action — clinics are never self-registered.
func (h *apiHandler) createClinic(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.IdentityFromContext(r.Context())
	var body createClinicBody
	if !decodeJSON(w, r, &body) {
		return
	}
	if strings.TrimSpace(body.Name) == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "name is required")
		return
	}
	if body.ContactEmail != "" {
		if _, ok := validEmail(body.ContactEmail); !ok {
			writeError(w, http.StatusBadRequest, "invalid_request", "contact_email is not a valid email")
			return
		}
	}
	if msg, ok := checkLatLng(body.Latitude, body.Longitude); !ok {
		writeError(w, http.StatusBadRequest, "invalid_request", msg)
		return
	}

	clinic, err := h.queries.CreateClinic(r.Context(), store.CreateClinicParams{
		Name:         strings.TrimSpace(body.Name),
		ContactEmail: optText(body.ContactEmail),
		ContactPhone: optText(body.ContactPhone),
		Address:      optText(body.Address),
		Latitude:     optFloat(body.Latitude),
		Longitude:    optFloat(body.Longitude),
	})
	if err != nil {
		h.serverError(w, "create clinic", err)
		return
	}
	h.logActivity(r.Context(), id.UserID, "clinic.created", "clinic", clinic.ID, nil)
	writeJSON(w, http.StatusCreated, toClinicResponse(store.GetClinicRow(clinic)))
}

// listClinics handles GET /api/v1/admin/clinics (system_admin).
func (h *apiHandler) listClinics(w http.ResponseWriter, r *http.Request) {
	rows, err := h.queries.ListClinics(r.Context())
	if err != nil {
		h.serverError(w, "list clinics", err)
		return
	}
	out := make([]clinicResponse, 0, len(rows))
	for _, c := range rows {
		out = append(out, toClinicResponse(store.GetClinicRow(c)))
	}
	writeJSON(w, http.StatusOK, out)
}

// getClinic handles GET /api/v1/admin/clinics/{id} (system_admin).
func (h *apiHandler) getClinic(w http.ResponseWriter, r *http.Request) {
	clinicID, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}
	c, err := h.queries.GetClinic(r.Context(), clinicID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "clinic not found")
			return
		}
		h.serverError(w, "get clinic", err)
		return
	}
	writeJSON(w, http.StatusOK, toClinicResponse(c))
}

// --- response shapes ---

type clinicResponse struct {
	ID           int64    `json:"id"`
	Name         string   `json:"name"`
	ContactEmail *string  `json:"contact_email"`
	ContactPhone *string  `json:"contact_phone"`
	Address      *string  `json:"address"`
	Latitude     *float64 `json:"latitude"`
	Longitude    *float64 `json:"longitude"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at"`
}

func toClinicResponse(c store.GetClinicRow) clinicResponse {
	return clinicResponse{
		ID:           c.ID,
		Name:         c.Name,
		ContactEmail: textPtr(c.ContactEmail),
		ContactPhone: textPtr(c.ContactPhone),
		Address:      textPtr(c.Address),
		Latitude:     floatPtr(c.Latitude),
		Longitude:    floatPtr(c.Longitude),
		CreatedAt:    tsString(c.CreatedAt),
		UpdatedAt:    tsString(c.UpdatedAt),
	}
}
