// Package matching implements the deterministic donor-matching pipeline
// (FR-07, FR-08). For an APPROVED blood request it selects donors by, in order:
//
//  1. blood compatibility — domain.CompatibleDonorTypes, the authoritative matrix,
//  2. screening/eligibility — no flagged latest screening, age-eligible,
//  3. availability + recent-donation window,
//  4. proximity — PostGIS radius (ST_DWithin).
//
// Step 1 is resolved here so the compatibility matrix lives in exactly one place;
// steps 2–4 run in the DB (store.MatchRequestDonors) where the data and the
// PostGIS index are. The MVP is deterministic only — no ML, no ranking beyond
// distance (decisions D-014/D-015). The clinic still makes the final in-person
// eligibility decision.
package matching

import (
	"context"
	"fmt"
	"time"

	"github.com/blood-donation/backend/internal/domain"
	"github.com/blood-donation/backend/internal/store"
	"github.com/jackc/pgx/v5/pgtype"
)

// MinDonationIntervalDays is the minimum gap we require since a donor's last
// donation before matching them again. 56 days is the standard whole-blood
// interval; donors who gave more recently are filtered out.
const MinDonationIntervalDays = 56

// Store is the slice of the data layer the engine needs. store.Querier satisfies
// it, and tests supply a fake.
type Store interface {
	MatchRequestDonors(ctx context.Context, arg store.MatchRequestDonorsParams) ([]store.DonorMatch, error)
}

// Engine runs the pipeline. now is injectable so the recent-donation cutoff is
// deterministic in tests.
type Engine struct {
	store Store
	now   func() time.Time
}

// NewEngine wires the engine to a data layer.
func NewEngine(s Store) *Engine {
	return &Engine{store: s, now: time.Now}
}

// Request is the minimal view of an approved request the pipeline needs.
type Request struct {
	ID        int64
	BloodType string
}

// Run executes the pipeline for an approved request and returns the matches it
// created (a donor is matched at most once per request, so re-running is safe).
// It errors if the request's blood type is not one of the 8 concrete types —
// requests never name "unknown".
func (e *Engine) Run(ctx context.Context, req Request) ([]store.DonorMatch, error) {
	donorTypes, ok := domain.CompatibleDonorTypes(domain.BloodType(req.BloodType))
	if !ok {
		return nil, fmt.Errorf("matching: request %d has invalid blood type %q", req.ID, req.BloodType)
	}
	types := make([]string, len(donorTypes))
	for i, d := range donorTypes {
		types[i] = string(d)
	}

	cutoff := e.now().AddDate(0, 0, -MinDonationIntervalDays)

	return e.store.MatchRequestDonors(ctx, store.MatchRequestDonorsParams{
		CompatibleTypes: types,
		RecentCutoff:    pgtype.Date{Time: cutoff, Valid: true},
		RequestID:       req.ID,
	})
}
