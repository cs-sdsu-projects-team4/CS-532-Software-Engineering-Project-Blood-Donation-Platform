-- Explicit SQL for donor_profiles. latitude/longitude are plain nullable columns
-- (the source of truth for a donor's position); the PostGIS `location` geography
-- is derived from them in the same write so they never drift. Reads return the
-- plain lat/lng columns, which are nullable when a donor has set no location.

-- name: CreateDonorProfile :one
INSERT INTO donor_profiles (
    user_id, blood_type, is_age_eligible, is_available, travel_radius_m,
    latitude, longitude, location, last_donation_date
) VALUES (
    @user_id, @blood_type, @is_age_eligible, @is_available, @travel_radius_m,
    sqlc.narg('latitude'), sqlc.narg('longitude'),
    CASE
        WHEN sqlc.narg('longitude')::float8 IS NULL OR sqlc.narg('latitude')::float8 IS NULL THEN NULL
        ELSE ST_SetSRID(ST_MakePoint(sqlc.narg('longitude')::float8, sqlc.narg('latitude')::float8), 4326)::geography
    END,
    sqlc.narg('last_donation_date')
)
RETURNING
    user_id, blood_type, is_age_eligible, is_available, travel_radius_m,
    latitude, longitude, last_donation_date, created_at, updated_at;

-- name: GetDonorProfile :one
SELECT
    user_id, blood_type, is_age_eligible, is_available, travel_radius_m,
    latitude, longitude, last_donation_date, created_at, updated_at
FROM donor_profiles
WHERE user_id = @user_id;

-- name: UpdateDonorProfile :one
-- Full replace of the mutable fields (FR-02). The handler loads the current row,
-- applies the request patch, and writes the complete new state back.
UPDATE donor_profiles
SET
    blood_type         = @blood_type,
    is_available       = @is_available,
    travel_radius_m    = @travel_radius_m,
    latitude           = sqlc.narg('latitude'),
    longitude          = sqlc.narg('longitude'),
    location = CASE
        WHEN sqlc.narg('longitude')::float8 IS NULL OR sqlc.narg('latitude')::float8 IS NULL THEN NULL
        ELSE ST_SetSRID(ST_MakePoint(sqlc.narg('longitude')::float8, sqlc.narg('latitude')::float8), 4326)::geography
    END,
    last_donation_date = sqlc.narg('last_donation_date'),
    updated_at         = now()
WHERE user_id = @user_id
RETURNING
    user_id, blood_type, is_age_eligible, is_available, travel_radius_m,
    latitude, longitude, last_donation_date, created_at, updated_at;
