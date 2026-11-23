package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/mail"
	"strings"
)

// decodeJSON reads a JSON request body into dst, rejecting unknown fields and
// bodies that contain more than a single JSON value. It returns a client-safe
// message on failure so callers can pass it straight to writeError (400). All
// input is validated server-side (CLAUDE.md conventions / NFR-01).
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) (ok bool) {
	// Cap body size to a sane limit to avoid unbounded memory use.
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MiB

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", decodeErrMessage(err))
		return false
	}
	// Ensure there's exactly one JSON value in the body.
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must contain a single JSON object")
		return false
	}
	return true
}

func decodeErrMessage(err error) string {
	var typeErr *json.UnmarshalTypeError
	switch {
	case errors.As(err, &typeErr):
		return "field " + typeErr.Field + " has the wrong type"
	case strings.HasPrefix(err.Error(), "json: unknown field "):
		return "unknown " + strings.TrimPrefix(err.Error(), "json: ")
	default:
		return "request body is not valid JSON"
	}
}

// validEmail reports whether s is a syntactically valid email address, and
// returns the normalized (lower-cased, trimmed) form.
func validEmail(s string) (string, bool) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return "", false
	}
	if _, err := mail.ParseAddress(s); err != nil {
		return "", false
	}
	return s, true
}

// minPasswordLen is the minimum acceptable password length. Modest for a course
// prototype; the point is to reject empty/trivial values, not enforce a policy.
const minPasswordLen = 8

// checkLatLng validates an optional coordinate pair. Latitude and longitude must
// be supplied together (or not at all) and fall within valid WGS-84 ranges. It
// returns a client-safe message and ok=false on failure.
func checkLatLng(lat, lng *float64) (msg string, ok bool) {
	switch {
	case lat == nil && lng == nil:
		return "", true
	case lat == nil || lng == nil:
		return "latitude and longitude must be provided together", false
	case *lat < -90 || *lat > 90:
		return "latitude must be between -90 and 90", false
	case *lng < -180 || *lng > 180:
		return "longitude must be between -180 and 180", false
	default:
		return "", true
	}
}
