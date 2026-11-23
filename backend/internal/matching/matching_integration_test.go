//go:build integration

// Integration test for the matching pipeline against a real Postgres+PostGIS.
// It exercises the actual SQL (ST_DWithin radius, blood_type = ANY, screening
// join) that the in-memory fakes cannot. Run it with a live database:
//
//	# from backend/, with the compose DB up and migrations applied:
//	DATABASE_URL="postgres://blood:<pw>@localhost:5432/blooddb?sslmode=disable" \
//	    go test -tags integration ./internal/matching/...
//
// All work happens inside a transaction that is ALWAYS rolled back, so the test
// leaves no data behind and is safe to run against a shared dev database.
package matching_test

import (
	"context"
	"os"
	"testing"

	"github.com/blood-donation/backend/internal/matching"
	"github.com/blood-donation/backend/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMatchingPipeline_Integration(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx) // never commit — keep the DB clean

	seed := []string{
		`INSERT INTO users (id,email,password_hash,role,full_name) VALUES
			(900001,'it-staff@test','x','clinic_employee','Staff'),
			(900002,'it-d2@test','x','donor','D2'),
			(900003,'it-d3@test','x','donor','D3'),
			(900004,'it-d4@test','x','donor','D4')`,
		`INSERT INTO clinics (id,name,latitude,longitude,location) VALUES
			(900001,'IT Clinic',0,0, ST_SetSRID(ST_MakePoint(0,0),4326)::geography)`,
		`INSERT INTO blood_requests (id,clinic_id,created_by,blood_type,units_needed,urgency,status,radius_m,latitude,longitude,location) VALUES
			(900001,900001,900001,'A+',1,'routine','approved',50000,0,0, ST_SetSRID(ST_MakePoint(0,0),4326)::geography)`,
		// 900002 O+ near (match), 900003 B+ near (incompatible), 900004 O- far (out of radius).
		`INSERT INTO donor_profiles (user_id,blood_type,is_age_eligible,is_available,travel_radius_m,latitude,longitude,location) VALUES
			(900002,'O+',true,true,10000,0.1,0, ST_SetSRID(ST_MakePoint(0,0.1),4326)::geography),
			(900003,'B+',true,true,10000,0.1,0, ST_SetSRID(ST_MakePoint(0,0.1),4326)::geography),
			(900004,'O-',true,true,10000,1.0,0, ST_SetSRID(ST_MakePoint(0,1.0),4326)::geography)`,
	}
	for _, s := range seed {
		if _, err := tx.Exec(ctx, s); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	// Run the real pipeline against the transaction-scoped store.
	eng := matching.NewEngine(store.New(tx))
	matches, err := eng.Run(ctx, matching.Request{ID: 900001, BloodType: "A+"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(matches) != 1 {
		t.Fatalf("expected exactly 1 match, got %d: %+v", len(matches), matches)
	}
	if matches[0].DonorUserID != 900002 {
		t.Fatalf("matched donor = %d, want 900002 (O+, in radius)", matches[0].DonorUserID)
	}
}
