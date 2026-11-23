-- Explicit SQL for clinics and clinic membership. latitude/longitude are plain
-- nullable columns; the PostGIS `location` geography is derived from them in the
-- same write. Clinic creation and staff assignment are system_admin actions;
-- membership drives the clinic RBAC checks used in PR3 (FR-05, FR-06).

-- name: CreateClinic :one
INSERT INTO clinics (name, contact_email, contact_phone, address, latitude, longitude, location)
VALUES (
    @name, @contact_email, @contact_phone, @address,
    sqlc.narg('latitude'), sqlc.narg('longitude'),
    CASE
        WHEN sqlc.narg('longitude')::float8 IS NULL OR sqlc.narg('latitude')::float8 IS NULL THEN NULL
        ELSE ST_SetSRID(ST_MakePoint(sqlc.narg('longitude')::float8, sqlc.narg('latitude')::float8), 4326)::geography
    END
)
RETURNING id, name, contact_email, contact_phone, address, latitude, longitude, created_at, updated_at;

-- name: GetClinic :one
SELECT id, name, contact_email, contact_phone, address, latitude, longitude, created_at, updated_at
FROM clinics
WHERE id = @id;

-- name: AddClinicMember :one
INSERT INTO clinic_members (clinic_id, user_id, role)
VALUES (@clinic_id, @user_id, @role)
RETURNING id, clinic_id, user_id, role, created_at;

-- name: GetClinicMembershipByUser :one
-- Returns the clinic a staff user belongs to and their clinic role. Used by the
-- clinic RBAC checks; a user staffs at most one clinic in the MVP.
SELECT id, clinic_id, user_id, role, created_at
FROM clinic_members
WHERE user_id = @user_id;
