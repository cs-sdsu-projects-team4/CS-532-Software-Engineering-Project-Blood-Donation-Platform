-- 0001_init: baseline database setup.
--
-- Goose reads the "-- +goose Up" section to apply a migration and the
-- "-- +goose Down" section to roll it back. Apply with:
--   goose -dir migrations postgres "$DATABASE_URL" up
--
-- This baseline only enables the extensions the rest of the schema depends on.
-- Actual tables (users, clinics, blood_requests, ...) arrive in later PRs.

-- +goose Up
-- PostGIS gives us geography/geometry types + distance queries for donor matching.
CREATE EXTENSION IF NOT EXISTS postgis;
-- pgcrypto provides gen_random_uuid() for UUID primary keys.
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- +goose Down
DROP EXTENSION IF EXISTS pgcrypto;
DROP EXTENSION IF EXISTS postgis;
