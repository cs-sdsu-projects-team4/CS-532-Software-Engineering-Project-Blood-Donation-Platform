-- 0002_accounts: identity, donor profiles, clinics, and preliminary screening.
--
-- Covers the account/access foundation (FR-01, FR-02, FR-03, FR-12). Security is
-- enforced in the Go backend, never in the clients. Privacy (NFR-02): we store
-- only what matching, contact, and scheduling need — no national ID numbers and
-- no clinic medical test records. Age is a self-declared 18+ boolean; blood type
-- is self-reported (and may be 'unknown'). The clinic makes the final in-person
-- eligibility decision.

-- +goose Up

-- Roles are a fixed set shared across tables; a native enum keeps them consistent.
CREATE TYPE user_role AS ENUM ('donor', 'clinic_employee', 'clinic_admin', 'system_admin');

-- One row per login identity. password_hash holds an Argon2id encoded string
-- (algorithm + params + salt + hash), never a plaintext or reversible value.
CREATE TABLE users (
    id            BIGSERIAL PRIMARY KEY,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role          user_role NOT NULL,
    full_name     TEXT NOT NULL,
    phone         TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Donor-specific profile, 1:1 with a user whose role is 'donor'.
-- is_age_eligible is the donor's 18-or-older self declaration (FR-01). location
-- + travel_radius_m feed the PostGIS distance filter in the matching pipeline.
-- latitude/longitude are the plain, nullable source of truth for a donor's
-- position; `location` is the PostGIS geography derived from them in the same
-- write (so the two never drift). Reads return lat/lng directly; matching uses
-- `location` with its GiST index.
CREATE TABLE donor_profiles (
    user_id            BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    blood_type         TEXT NOT NULL DEFAULT 'unknown'
                       CHECK (blood_type IN ('O-','O+','A-','A+','B-','B+','AB-','AB+','unknown')),
    is_age_eligible    BOOLEAN NOT NULL,
    is_available       BOOLEAN NOT NULL DEFAULT TRUE,
    travel_radius_m    INTEGER NOT NULL DEFAULT 10000 CHECK (travel_radius_m > 0),
    latitude           DOUBLE PRECISION,
    longitude          DOUBLE PRECISION,
    location           GEOGRAPHY(Point, 4326),
    last_donation_date DATE,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- GiST index powers ST_DWithin radius queries during matching (§10.3).
CREATE INDEX idx_donor_profiles_location ON donor_profiles USING GIST (location);
-- Partial index: matching only ever scans available donors.
CREATE INDEX idx_donor_profiles_available ON donor_profiles (is_available) WHERE is_available;

CREATE TABLE clinics (
    id            BIGSERIAL PRIMARY KEY,
    name          TEXT NOT NULL,
    contact_email TEXT,
    contact_phone TEXT,
    address       TEXT,
    latitude      DOUBLE PRECISION,
    longitude     DOUBLE PRECISION,
    location      GEOGRAPHY(Point, 4326),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_clinics_location ON clinics USING GIST (location);

-- Which users staff which clinic, and in what staff role. A user can belong to a
-- clinic exactly once. clinic_admin approval gates request publishing (FR-06).
CREATE TABLE clinic_members (
    id         BIGSERIAL PRIMARY KEY,
    clinic_id  BIGINT NOT NULL REFERENCES clinics(id) ON DELETE CASCADE,
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role       user_role NOT NULL CHECK (role IN ('clinic_employee','clinic_admin')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (clinic_id, user_id)
);
CREATE INDEX idx_clinic_members_user ON clinic_members (user_id);

-- Preliminary screening answers (FR-03), stored as JSONB so the questionnaire can
-- evolve without a migration. has_flags = at least one answer suggests a possible
-- restriction; the guidance shown to the donor is non-diagnostic and defers the
-- final decision to the clinic (FR-04). One row per submission keeps revision
-- history; the latest row is the donor's current screening state.
CREATE TABLE screening_responses (
    id            BIGSERIAL PRIMARY KEY,
    donor_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    answers       JSONB NOT NULL,
    has_flags     BOOLEAN NOT NULL,
    submitted_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_screening_responses_donor ON screening_responses (donor_user_id, submitted_at DESC);

-- +goose Down
DROP TABLE screening_responses;
DROP TABLE clinic_members;
DROP TABLE clinics;
DROP TABLE donor_profiles;
DROP TABLE users;
DROP TYPE user_role;
