package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestHashPassword_RoundTrip(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("unexpected hash format: %s", hash)
	}
	if err := VerifyPassword("correct horse battery staple", hash); err != nil {
		t.Errorf("VerifyPassword should accept the right password: %v", err)
	}
}

func TestVerifyPassword_WrongPassword(t *testing.T) {
	hash, _ := HashPassword("s3cret")
	err := VerifyPassword("not-it", hash)
	if !errors.Is(err, ErrMismatchedHash) {
		t.Errorf("expected ErrMismatchedHash, got %v", err)
	}
}

func TestHashPassword_UniqueSalts(t *testing.T) {
	h1, _ := HashPassword("same")
	h2, _ := HashPassword("same")
	if h1 == h2 {
		t.Error("two hashes of the same password must differ (random salt)")
	}
}

func TestVerifyPassword_MalformedHash(t *testing.T) {
	for _, bad := range []string{"", "plaintext", "$argon2id$bad", "$bcrypt$v=19$m=1,t=1,p=1$x$y"} {
		if err := VerifyPassword("x", bad); err == nil {
			t.Errorf("VerifyPassword(%q) should error", bad)
		}
	}
}
