package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestLiveEndpoint shows the shape of a Go test: a function named TestXxx that
// takes *testing.T. `go test ./...` runs every such function.
//
// httptest.NewRecorder() is a fake ResponseWriter we can assert against, so we
// test the handler without opening a real socket. We call live() directly
// because it doesn't touch the database.
func TestLiveEndpoint(t *testing.T) {
	h := &healthHandler{} // db is nil, but live() never uses it

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	h.live(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	want := `{"status":"ok"}` + "\n" // json.Encoder appends a newline
	if got := rec.Body.String(); got != want {
		t.Fatalf("expected body %q, got %q", want, got)
	}
}
