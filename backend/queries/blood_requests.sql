-- Explicit SQL for blood_requests. latitude/longitude are the request's own
-- nullable coordinates; when omitted they (and the derived PostGIS `location`)
-- default to the owning clinic's position, set in the same write so they never
-- drift. Reads return the plain lat/lng columns, never the geography. A request
-- stays pending_approval until a clinic_admin approves it (FR-06); only approved
-- requests are matched.

-- name: CreateBloodRequest :one
INSERT INTO blood_requests (
    clinic_id, created_by, blood_type, units_needed, urgency,
    radius_m, latitude, longitude, location, notes, needed_by
) VALUES (
    @clinic_id, @created_by, @blood_type, @units_needed, @urgency,
    @radius_m,
    COALESCE(sqlc.narg('latitude'), (SELECT latitude FROM clinics WHERE id = @clinic_id)),
    COALESCE(sqlc.narg('longitude'), (SELECT longitude FROM clinics WHERE id = @clinic_id)),
    CASE
        WHEN sqlc.narg('longitude')::float8 IS NOT NULL AND sqlc.narg('latitude')::float8 IS NOT NULL
            THEN ST_SetSRID(ST_MakePoint(sqlc.narg('longitude')::float8, sqlc.narg('latitude')::float8), 4326)::geography
        ELSE (SELECT location FROM clinics WHERE id = @clinic_id)
    END,
    sqlc.narg('notes'), sqlc.narg('needed_by')
)
RETURNING id, clinic_id, created_by, blood_type, units_needed, urgency, status,
    approved_by, approved_at, radius_m, latitude, longitude, notes, needed_by,
    created_at, updated_at;

-- name: GetBloodRequest :one
SELECT id, clinic_id, created_by, blood_type, units_needed, urgency, status,
    approved_by, approved_at, radius_m, latitude, longitude, notes, needed_by,
    created_at, updated_at
FROM blood_requests
WHERE id = @id;

-- name: ListBloodRequestsByClinic :many
SELECT id, clinic_id, created_by, blood_type, units_needed, urgency, status,
    approved_by, approved_at, radius_m, latitude, longitude, notes, needed_by,
    created_at, updated_at
FROM blood_requests
WHERE clinic_id = @clinic_id
ORDER BY created_at DESC
LIMIT @row_limit OFFSET @row_offset;

-- name: ListPendingBloodRequestsByClinic :many
-- The clinic_admin approval queue (FR-06): requests awaiting a decision.
SELECT id, clinic_id, created_by, blood_type, units_needed, urgency, status,
    approved_by, approved_at, radius_m, latitude, longitude, notes, needed_by,
    created_at, updated_at
FROM blood_requests
WHERE clinic_id = @clinic_id AND status = 'pending_approval'
ORDER BY created_at ASC;

-- name: ApproveBloodRequest :one
-- The approval gate (FR-06). The status guard makes this idempotent and safe:
-- only a still-pending request transitions, and matching may run only afterward.
UPDATE blood_requests
SET status = 'approved', approved_by = @approved_by, approved_at = now(), updated_at = now()
WHERE id = @id AND status = 'pending_approval'
RETURNING id, clinic_id, created_by, blood_type, units_needed, urgency, status,
    approved_by, approved_at, radius_m, latitude, longitude, notes, needed_by,
    created_at, updated_at;

-- name: SetBloodRequestStatus :one
-- Generic status transition for terminal states (fulfilled/cancelled/expired).
UPDATE blood_requests
SET status = @status, updated_at = now()
WHERE id = @id
RETURNING id, clinic_id, created_by, blood_type, units_needed, urgency, status,
    approved_by, approved_at, radius_m, latitude, longitude, notes, needed_by,
    created_at, updated_at;
