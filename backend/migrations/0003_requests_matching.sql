-- 0003_requests_matching: blood requests, the approval gate, matching output,
-- donor responses, appointments, push tokens, and the activity log.
--
-- Covers request creation (FR-05), the clinic_admin approval gate before any
-- donor is contacted (FR-06), the deterministic match results (FR-07/08), donor
-- notifications (FR-09), accept/decline (FR-10), appointment scheduling with no
-- double-booking (FR-11), and the audit trail (FR-13). Security/RBAC stays in the
-- Go backend. Privacy (NFR-02): we store only what matching, contact, and
-- scheduling need; the clinic makes the final in-person eligibility decision.

-- +goose Up

-- A request moves pending_approval -> approved (by a clinic_admin) before any
-- donor is matched/notified. fulfilled/cancelled/expired are terminal.
CREATE TYPE request_status AS ENUM ('pending_approval', 'approved', 'fulfilled', 'cancelled', 'expired');

-- Lifecycle of a single donor's match to a request: notified on dispatch, then
-- accepted/declined by the donor (FR-10), or expired if the request closes first.
CREATE TYPE match_status AS ENUM ('notified', 'accepted', 'declined', 'expired');

CREATE TYPE appointment_status AS ENUM ('scheduled', 'completed', 'cancelled', 'no_show');

-- A clinic's need for a specific blood type. Created by a clinic_employee
-- (created_by) and held at pending_approval until a clinic_admin approves it
-- (approved_by/approved_at) — only then may matching run (FR-06). blood_type is
-- always one of the 8 concrete types ('unknown' is a donor self-report only).
-- latitude/longitude are the nullable source of truth; `location` is the PostGIS
-- geography derived from them in the same write (defaults to the clinic's
-- location when the request omits coordinates) and drives the ST_DWithin radius.
CREATE TABLE blood_requests (
    id           BIGSERIAL PRIMARY KEY,
    clinic_id    BIGINT NOT NULL REFERENCES clinics(id) ON DELETE CASCADE,
    created_by   BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    blood_type   TEXT NOT NULL CHECK (blood_type IN ('O-','O+','A-','A+','B-','B+','AB-','AB+')),
    units_needed INTEGER NOT NULL CHECK (units_needed > 0),
    urgency      TEXT NOT NULL DEFAULT 'routine' CHECK (urgency IN ('routine','urgent','critical')),
    status       request_status NOT NULL DEFAULT 'pending_approval',
    approved_by  BIGINT REFERENCES users(id) ON DELETE SET NULL,
    approved_at  TIMESTAMPTZ,
    radius_m     INTEGER NOT NULL DEFAULT 10000 CHECK (radius_m > 0),
    latitude     DOUBLE PRECISION,
    longitude    DOUBLE PRECISION,
    location     GEOGRAPHY(Point, 4326),
    notes        TEXT,
    needed_by    TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- GiST index powers ST_DWithin radius queries during matching (§10.3).
CREATE INDEX idx_blood_requests_location ON blood_requests USING GIST (location);
CREATE INDEX idx_blood_requests_clinic ON blood_requests (clinic_id);
-- Dashboards list by status (e.g. the admin approval queue).
CREATE INDEX idx_blood_requests_status ON blood_requests (status);

-- One row per donor the pipeline matched to a request. distance_m is captured at
-- match time so results are stable even if the donor later moves. A donor appears
-- at most once per request.
CREATE TABLE donor_matches (
    id            BIGSERIAL PRIMARY KEY,
    request_id    BIGINT NOT NULL REFERENCES blood_requests(id) ON DELETE CASCADE,
    donor_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    distance_m    DOUBLE PRECISION,
    status        match_status NOT NULL DEFAULT 'notified',
    notified_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    responded_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (request_id, donor_user_id)
);
-- A donor's "my matches / respond" view and request-side ranking by proximity.
CREATE INDEX idx_donor_matches_donor ON donor_matches (donor_user_id, status);
CREATE INDEX idx_donor_matches_request ON donor_matches (request_id, distance_m);

-- A donor's confirmed slot at a clinic after accepting a match (FR-11). No
-- double-booking: a donor cannot hold two scheduled slots at the same instant,
-- and a clinic cannot have two donors in the same slot. Enforced by partial
-- unique indexes scoped to still-scheduled rows, so cancelled slots free up.
CREATE TABLE appointments (
    id            BIGSERIAL PRIMARY KEY,
    request_id    BIGINT NOT NULL REFERENCES blood_requests(id) ON DELETE CASCADE,
    donor_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    clinic_id     BIGINT NOT NULL REFERENCES clinics(id) ON DELETE CASCADE,
    scheduled_at  TIMESTAMPTZ NOT NULL,
    status        appointment_status NOT NULL DEFAULT 'scheduled',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_appointments_donor_slot ON appointments (donor_user_id, scheduled_at)
    WHERE status = 'scheduled';
CREATE UNIQUE INDEX idx_appointments_clinic_slot ON appointments (clinic_id, scheduled_at)
    WHERE status = 'scheduled';

-- FCM device tokens for push notifications (FR-09), dispatched from the backend
-- worker. A token is globally unique; a user may have several (multiple devices).
CREATE TABLE push_tokens (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token      TEXT NOT NULL UNIQUE,
    platform   TEXT NOT NULL CHECK (platform IN ('android','ios','web')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_push_tokens_user ON push_tokens (user_id);

-- Append-only audit trail (FR-13). actor_user_id is nullable for system actions
-- (e.g. the matching worker). entity_type/entity_id point at the affected row;
-- metadata carries action-specific detail as JSONB so the shape can evolve.
CREATE TABLE activity_logs (
    id            BIGSERIAL PRIMARY KEY,
    actor_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    action        TEXT NOT NULL,
    entity_type   TEXT,
    entity_id     BIGINT,
    metadata      JSONB,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_activity_logs_entity ON activity_logs (entity_type, entity_id);
CREATE INDEX idx_activity_logs_actor ON activity_logs (actor_user_id, created_at DESC);

-- +goose Down
DROP TABLE activity_logs;
DROP TABLE push_tokens;
DROP TABLE appointments;
DROP TABLE donor_matches;
DROP TABLE blood_requests;
DROP TYPE appointment_status;
DROP TYPE match_status;
DROP TYPE request_status;
