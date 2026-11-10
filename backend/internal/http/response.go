package http

import (
	"encoding/json"
	"net/http"
)

// writeJSON serializes body as JSON with the given status code. Shared by all
// handlers so responses stay consistent.
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// errorBody is the single shape every error response uses, so clients can rely
// on it. Example: {"error":{"code":"invalid_request","message":"..."}}.
type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// writeError sends a structured JSON error. `code` is a stable machine-readable
// string; `message` is human-readable.
func writeError(w http.ResponseWriter, status int, code, message string) {
	var b errorBody
	b.Error.Code = code
	b.Error.Message = message
	writeJSON(w, status, b)
}
