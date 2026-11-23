package auth

import (
	"testing"
	"time"
)

func newTM(t *testing.T, accessTTL, refreshTTL time.Duration) *TokenManager {
	t.Helper()
	tm, err := NewTokenManager("test-secret-value", "test-issuer", accessTTL, refreshTTL)
	if err != nil {
		t.Fatalf("NewTokenManager: %v", err)
	}
	return tm
}

func TestNewTokenManager_RequiresSecret(t *testing.T) {
	if _, err := NewTokenManager("", "iss", time.Minute, time.Hour); err == nil {
		t.Error("expected error for empty secret")
	}
}

func TestIssueAndParse_Access(t *testing.T) {
	tm := newTM(t, 15*time.Minute, time.Hour)
	pair, err := tm.IssuePair(42, RoleClinicAdmin)
	if err != nil {
		t.Fatalf("IssuePair: %v", err)
	}
	if pair.TokenType != "Bearer" || pair.ExpiresIn != 900 {
		t.Errorf("unexpected pair metadata: %+v", pair)
	}

	id, role, err := tm.Parse(pair.AccessToken, AccessToken)
	if err != nil {
		t.Fatalf("Parse access: %v", err)
	}
	if id != 42 || role != RoleClinicAdmin {
		t.Errorf("got (%d,%s), want (42,clinic_admin)", id, role)
	}
}

func TestParse_RejectsWrongTokenType(t *testing.T) {
	tm := newTM(t, time.Minute, time.Hour)
	pair, _ := tm.IssuePair(1, RoleDonor)

	// A refresh token must not be accepted where an access token is required.
	if _, _, err := tm.Parse(pair.RefreshToken, AccessToken); err == nil {
		t.Error("refresh token accepted as access token")
	}
	// And vice versa.
	if _, _, err := tm.Parse(pair.AccessToken, RefreshToken); err == nil {
		t.Error("access token accepted as refresh token")
	}
	// Correct usage still works.
	if _, _, err := tm.Parse(pair.RefreshToken, RefreshToken); err != nil {
		t.Errorf("refresh token should parse as refresh: %v", err)
	}
}

func TestParse_RejectsExpired(t *testing.T) {
	tm := newTM(t, -time.Minute, time.Hour) // already expired access tokens
	pair, _ := tm.IssuePair(7, RoleDonor)
	if _, _, err := tm.Parse(pair.AccessToken, AccessToken); err == nil {
		t.Error("expired access token should be rejected")
	}
}

func TestParse_RejectsWrongSecret(t *testing.T) {
	tm := newTM(t, time.Minute, time.Hour)
	pair, _ := tm.IssuePair(1, RoleDonor)

	other, _ := NewTokenManager("different-secret", "test-issuer", time.Minute, time.Hour)
	if _, _, err := other.Parse(pair.AccessToken, AccessToken); err == nil {
		t.Error("token signed with a different secret must be rejected")
	}
}

func TestParse_RejectsWrongIssuer(t *testing.T) {
	tm := newTM(t, time.Minute, time.Hour)
	pair, _ := tm.IssuePair(1, RoleDonor)

	other, _ := NewTokenManager("test-secret-value", "other-issuer", time.Minute, time.Hour)
	if _, _, err := other.Parse(pair.AccessToken, AccessToken); err == nil {
		t.Error("token with a different issuer must be rejected")
	}
}

func TestIsValidRole(t *testing.T) {
	for _, r := range []Role{RoleDonor, RoleClinicEmployee, RoleClinicAdmin, RoleSystemAdmin} {
		if !IsValidRole(r) {
			t.Errorf("%s should be valid", r)
		}
	}
	if IsValidRole("root") {
		t.Error("unknown role should be invalid")
	}
}
