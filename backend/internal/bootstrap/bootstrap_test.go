package bootstrap

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/blood-donation/backend/internal/store"
)

// fakeAdminStore implements AdminStore in memory.
type fakeAdminStore struct {
	existing map[string]bool
	created  []store.CreateUserParams
}

func (f *fakeAdminStore) EmailExists(_ context.Context, email string) (bool, error) {
	return f.existing[email], nil
}

func (f *fakeAdminStore) CreateUser(_ context.Context, arg store.CreateUserParams) (store.CreateUserRow, error) {
	f.created = append(f.created, arg)
	return store.CreateUserRow{ID: 1, Email: arg.Email, Role: arg.Role, FullName: arg.FullName}, nil
}

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestEnsureSystemAdmin_CreatesWhenAbsent(t *testing.T) {
	f := &fakeAdminStore{existing: map[string]bool{}}
	if err := EnsureSystemAdmin(context.Background(), f, "Admin@Example.com", "password123", discardLogger()); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(f.created) != 1 {
		t.Fatalf("expected 1 account created, got %d", len(f.created))
	}
	c := f.created[0]
	if c.Email != "admin@example.com" {
		t.Errorf("email not normalized: %q", c.Email)
	}
	if c.Role != store.UserRoleSystemAdmin {
		t.Errorf("role = %v, want system_admin", c.Role)
	}
}

func TestEnsureSystemAdmin_SkipsWhenPresent(t *testing.T) {
	f := &fakeAdminStore{existing: map[string]bool{"admin@example.com": true}}
	if err := EnsureSystemAdmin(context.Background(), f, "admin@example.com", "password123", discardLogger()); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(f.created) != 0 {
		t.Fatalf("should not create when the admin already exists")
	}
}

func TestEnsureSystemAdmin_SkipsWhenUnconfigured(t *testing.T) {
	f := &fakeAdminStore{existing: map[string]bool{}}
	if err := EnsureSystemAdmin(context.Background(), f, "", "", discardLogger()); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(f.created) != 0 {
		t.Fatalf("should not create when unconfigured")
	}
}

func TestEnsureSystemAdmin_RejectsBadInput(t *testing.T) {
	for _, tc := range []struct{ name, email, pass string }{
		{"short password", "admin@example.com", "short"},
		{"bad email", "not-an-email", "password123"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeAdminStore{existing: map[string]bool{}}
			if err := EnsureSystemAdmin(context.Background(), f, tc.email, tc.pass, discardLogger()); err == nil {
				t.Fatal("expected an error")
			}
			if len(f.created) != 0 {
				t.Fatal("should not create on bad input")
			}
		})
	}
}
