package matching

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/blood-donation/backend/internal/store"
)

// captureStore records the params MatchRequestDonors was called with.
type captureStore struct {
	got    store.MatchRequestDonorsParams
	called bool
}

func (c *captureStore) MatchRequestDonors(_ context.Context, arg store.MatchRequestDonorsParams) ([]store.DonorMatch, error) {
	c.got = arg
	c.called = true
	return []store.DonorMatch{{ID: 1, RequestID: arg.RequestID}}, nil
}

func TestRun_PassesCompatibleTypesAndCutoff(t *testing.T) {
	fake := &captureStore{}
	e := NewEngine(fake)
	// Pin "now" so the cutoff is deterministic.
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	e.now = func() time.Time { return now }

	matches, err := e.Run(context.Background(), Request{ID: 42, BloodType: "A+"})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(matches) != 1 || matches[0].RequestID != 42 {
		t.Fatalf("unexpected matches: %+v", matches)
	}
	if !fake.called {
		t.Fatal("expected MatchRequestDonors to be called")
	}
	if fake.got.RequestID != 42 {
		t.Errorf("RequestID = %d, want 42", fake.got.RequestID)
	}

	// A+ recipients may receive from O-, O+, A-, A+ (the authoritative matrix).
	want := []string{"A+", "A-", "O+", "O-"}
	got := append([]string(nil), fake.got.CompatibleTypes...)
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("compatible types = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("compatible types = %v, want %v", got, want)
		}
	}

	// Cutoff must be exactly MinDonationIntervalDays before "now".
	wantCutoff := now.AddDate(0, 0, -MinDonationIntervalDays)
	if !fake.got.RecentCutoff.Valid {
		t.Fatal("RecentCutoff not set")
	}
	if !fake.got.RecentCutoff.Time.Equal(wantCutoff) {
		t.Errorf("cutoff = %v, want %v", fake.got.RecentCutoff.Time, wantCutoff)
	}
}

func TestRun_RejectsInvalidBloodType(t *testing.T) {
	fake := &captureStore{}
	e := NewEngine(fake)

	for _, bt := range []string{"unknown", "", "XY"} {
		if _, err := e.Run(context.Background(), Request{ID: 1, BloodType: bt}); err == nil {
			t.Errorf("blood type %q: expected error, got nil", bt)
		}
	}
	if fake.called {
		t.Error("MatchRequestDonors must not be called for an invalid blood type")
	}
}
