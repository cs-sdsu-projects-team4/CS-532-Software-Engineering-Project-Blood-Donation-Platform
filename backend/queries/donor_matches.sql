-- Explicit SQL for donor_matches — the output of the deterministic matching
-- pipeline (FR-07/08) and the donor's accept/decline surface (FR-10).

-- name: MatchRequestDonors :many
-- The pipeline's DB step: for an APPROVED request, record a match for every
-- eligible donor within its radius. Blood compatibility is resolved in Go
-- (domain.CompatibleDonorTypes) and passed as the allowed donor types, so the
-- authoritative matrix stays in one place. DB-level filters: available,
-- age-eligible, compatible type, not a recent donor, no flagged latest screening
-- (FR-04), and within radius via PostGIS ST_DWithin. distance_m is captured now
-- so results stay stable. ON CONFLICT keeps a donor to one match per request, so
-- re-running is safe.
INSERT INTO donor_matches (request_id, donor_user_id, distance_m)
SELECT
    r.id, dp.user_id, ST_Distance(dp.location, r.location)
FROM blood_requests r
JOIN donor_profiles dp
    ON dp.is_available
    AND dp.is_age_eligible
    AND dp.blood_type = ANY(@compatible_types::text[])
    AND (
        sqlc.narg('recent_cutoff')::date IS NULL
        OR dp.last_donation_date IS NULL
        OR dp.last_donation_date <= sqlc.narg('recent_cutoff')
    )
    AND dp.location IS NOT NULL
    AND ST_DWithin(dp.location, r.location, r.radius_m)
LEFT JOIN LATERAL (
    SELECT sr.has_flags
    FROM screening_responses sr
    WHERE sr.donor_user_id = dp.user_id
    ORDER BY sr.submitted_at DESC
    LIMIT 1
) s ON TRUE
WHERE r.id = @request_id
    AND r.status = 'approved'
    AND r.location IS NOT NULL
    AND (s.has_flags IS NULL OR s.has_flags = FALSE)
ON CONFLICT (request_id, donor_user_id) DO NOTHING
RETURNING id, request_id, donor_user_id, distance_m, status, notified_at, responded_at, created_at;

-- name: GetDonorMatch :one
SELECT id, request_id, donor_user_id, distance_m, status, notified_at, responded_at, created_at
FROM donor_matches
WHERE id = @id;

-- name: ListMatchesForRequest :many
-- Clinic-side view: matched donors for a request with contact details, nearest
-- first, so staff can follow up.
SELECT dm.id, dm.request_id, dm.donor_user_id, dm.distance_m, dm.status,
    dm.notified_at, dm.responded_at,
    u.full_name, u.email, u.phone
FROM donor_matches dm
JOIN users u ON u.id = dm.donor_user_id
WHERE dm.request_id = @request_id
ORDER BY dm.distance_m NULLS LAST;

-- name: ListMatchesForDonor :many
-- Donor-side view: "my matches" with the request context they need to respond.
SELECT dm.id, dm.request_id, dm.distance_m, dm.status, dm.notified_at, dm.responded_at,
    r.blood_type, r.urgency, r.needed_by, r.clinic_id, c.name AS clinic_name
FROM donor_matches dm
JOIN blood_requests r ON r.id = dm.request_id
JOIN clinics c ON c.id = r.clinic_id
WHERE dm.donor_user_id = @donor_user_id
ORDER BY dm.notified_at DESC;

-- name: RespondToMatch :one
-- Donor accepts/declines (FR-10). Guards ensure only the owning donor can respond
-- and only from the 'notified' state, so a response can't be replayed or forged.
UPDATE donor_matches
SET status = @status, responded_at = now()
WHERE id = @id AND donor_user_id = @donor_user_id AND status = 'notified'
RETURNING id, request_id, donor_user_id, distance_m, status, notified_at, responded_at, created_at;
