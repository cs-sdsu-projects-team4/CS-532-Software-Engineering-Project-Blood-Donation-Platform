package auth

import "context"

// Identity is the authenticated caller, extracted from a verified access token
// and carried on the request context by the Authenticator middleware.
type Identity struct {
	UserID int64
	Role   Role
}

// ctxKey is an unexported type so our context key can never collide with a key
// from another package (the standard pattern for context values in Go).
type ctxKey struct{}

var identityKey = ctxKey{}

// WithIdentity returns a copy of ctx carrying the given identity.
func WithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, identityKey, id)
}

// IdentityFromContext returns the authenticated identity and ok=false if the
// request was not authenticated.
func IdentityFromContext(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(identityKey).(Identity)
	return id, ok
}
