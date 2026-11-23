package auth

import (
	"encoding/json"
	"net/http"
	"strings"
)

// writeAuthError emits the same structured error shape the rest of the API uses
// ({"error":{"code","message"}}). Kept here so the middleware has no dependency
// on the http package (which would create an import cycle).
func writeAuthError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}

// Authenticator verifies the Bearer access token on each request and stores the
// resulting Identity on the context. Requests without a valid access token are
// rejected with 401 before reaching the handler.
func Authenticator(tm *TokenManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			token, ok := bearerToken(header)
			if !ok {
				writeAuthError(w, http.StatusUnauthorized, "unauthenticated", "missing or malformed Authorization header")
				return
			}

			userID, role, err := tm.Parse(token, AccessToken)
			if err != nil {
				writeAuthError(w, http.StatusUnauthorized, "unauthenticated", "invalid or expired access token")
				return
			}

			ctx := WithIdentity(r.Context(), Identity{UserID: userID, Role: role})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole allows the request through only if the authenticated caller holds
// one of the listed roles; otherwise 403 (FR-12). It must be mounted after
// Authenticator. A system_admin is NOT auto-granted every role here — list it
// explicitly where it should have access, so authorization stays obvious.
func RequireRole(allowed ...Role) func(http.Handler) http.Handler {
	allowedSet := make(map[Role]bool, len(allowed))
	for _, r := range allowed {
		allowedSet[r] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, ok := IdentityFromContext(r.Context())
			if !ok {
				writeAuthError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
				return
			}
			if !allowedSet[id.Role] {
				writeAuthError(w, http.StatusForbidden, "forbidden", "your role may not perform this action")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// bearerToken extracts the token from an "Authorization: Bearer <token>" header.
// The scheme match is case-insensitive per RFC 7235.
func bearerToken(header string) (string, bool) {
	const prefix = "bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", false
	}
	token := strings.TrimSpace(header[len(prefix):])
	if token == "" {
		return "", false
	}
	return token, true
}
