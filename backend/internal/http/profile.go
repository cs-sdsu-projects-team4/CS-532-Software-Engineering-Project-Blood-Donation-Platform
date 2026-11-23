package http

import (
	"errors"
	"net/http"
	"time"

	"github.com/blood-donation/backend/internal/auth"
	"github.com/blood-donation/backend/internal/domain"
	"github.com/blood-donation/backend/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const dateLayout = "2006-01-02"

// profileResponse is the public view of a donor profile (FR-02).
type profileResponse struct {
	UserID           int64    `json:"user_id"`
	BloodType        string   `json:"blood_type"`
	IsAgeEligible    bool     `json:"is_age_eligible"`
	IsAvailable      bool     `json:"is_available"`
	TravelRadiusM    int32    `json:"travel_radius_m"`
	Latitude         *float64 `json:"latitude"`
	Longitude        *float64 `json:"longitude"`
	LastDonationDate *string  `json:"last_donation_date"`
}

// getMyProfile handles GET /api/v1/donor/profile — the caller's own donor profile.
func (h *apiHandler) getMyProfile(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.IdentityFromContext(r.Context())
	p, err := h.queries.GetDonorProfile(r.Context(), id.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "no donor profile for this account")
			return
		}
		h.serverError(w, "get donor profile", err)
		return
	}
	writeJSON(w, http.StatusOK, profileResponse{
		UserID:           p.UserID,
		BloodType:        p.BloodType,
		IsAgeEligible:    p.IsAgeEligible,
		IsAvailable:      p.IsAvailable,
		TravelRadiusM:    p.TravelRadiusM,
		Latitude:         floatPtr(p.Latitude),
		Longitude:        floatPtr(p.Longitude),
		LastDonationDate: datePtr(p.LastDonationDate),
	})
}

// updateProfileRequest is a patch: every field is optional, and only the fields
// present are changed (FR-02). latitude+longitude must be sent together.
type updateProfileRequest struct {
	BloodType        *string  `json:"blood_type"`
	IsAvailable      *bool    `json:"is_available"`
	TravelRadiusM    *int32   `json:"travel_radius_m"`
	Latitude         *float64 `json:"latitude"`
	Longitude        *float64 `json:"longitude"`
	LastDonationDate *string  `json:"last_donation_date"` // "YYYY-MM-DD"
}

// updateMyProfile handles PATCH /api/v1/donor/profile. It loads the current row,
// overlays the provided fields, and writes the complete new state back.
func (h *apiHandler) updateMyProfile(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.IdentityFromContext(r.Context())

	var req updateProfileRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	cur, err := h.queries.GetDonorProfile(r.Context(), id.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "no donor profile for this account")
			return
		}
		h.serverError(w, "get donor profile", err)
		return
	}

	// Start from the current state, then apply the patch.
	params := store.UpdateDonorProfileParams{
		UserID:           id.UserID,
		BloodType:        cur.BloodType,
		IsAvailable:      cur.IsAvailable,
		TravelRadiusM:    cur.TravelRadiusM,
		Latitude:         cur.Latitude,
		Longitude:        cur.Longitude,
		LastDonationDate: cur.LastDonationDate,
	}

	if req.BloodType != nil {
		bt := domain.BloodType(*req.BloodType)
		if !domain.IsValidDonorBloodType(bt) {
			writeError(w, http.StatusBadRequest, "invalid_request",
				"blood_type must be one of O-,O+,A-,A+,B-,B+,AB-,AB+ or 'unknown'")
			return
		}
		params.BloodType = string(bt)
	}
	if req.IsAvailable != nil {
		params.IsAvailable = *req.IsAvailable
	}
	if req.TravelRadiusM != nil {
		if *req.TravelRadiusM <= 0 {
			writeError(w, http.StatusBadRequest, "invalid_request", "travel_radius_m must be positive")
			return
		}
		params.TravelRadiusM = *req.TravelRadiusM
	}
	if req.Latitude != nil || req.Longitude != nil {
		if msg, ok := checkLatLng(req.Latitude, req.Longitude); !ok {
			writeError(w, http.StatusBadRequest, "invalid_request", msg)
			return
		}
		params.Latitude = optFloat(req.Latitude)
		params.Longitude = optFloat(req.Longitude)
	}
	if req.LastDonationDate != nil {
		t, perr := time.Parse(dateLayout, *req.LastDonationDate)
		if perr != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "last_donation_date must be YYYY-MM-DD")
			return
		}
		params.LastDonationDate = pgtype.Date{Time: t, Valid: true}
	}

	updated, err := h.queries.UpdateDonorProfile(r.Context(), params)
	if err != nil {
		h.serverError(w, "update donor profile", err)
		return
	}
	writeJSON(w, http.StatusOK, profileResponse{
		UserID:           updated.UserID,
		BloodType:        updated.BloodType,
		IsAgeEligible:    updated.IsAgeEligible,
		IsAvailable:      updated.IsAvailable,
		TravelRadiusM:    updated.TravelRadiusM,
		Latitude:         floatPtr(updated.Latitude),
		Longitude:        floatPtr(updated.Longitude),
		LastDonationDate: datePtr(updated.LastDonationDate),
	})
}

// floatPtr maps a nullable pg float8 to *float64 for JSON.
func floatPtr(f pgtype.Float8) *float64 {
	if !f.Valid {
		return nil
	}
	v := f.Float64
	return &v
}

// datePtr maps a nullable pg date to a *string ("YYYY-MM-DD") for JSON.
func datePtr(d pgtype.Date) *string {
	if !d.Valid {
		return nil
	}
	s := d.Time.Format(dateLayout)
	return &s
}
