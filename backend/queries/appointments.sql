-- Explicit SQL for appointments (FR-11). The partial unique indexes on
-- (donor_user_id, scheduled_at) and (clinic_id, scheduled_at) WHERE
-- status='scheduled' enforce no double-booking at the DB level; CreateAppointment
-- surfaces a unique-violation the handler maps to 409 Conflict.

-- name: CreateAppointment :one
INSERT INTO appointments (request_id, donor_user_id, clinic_id, scheduled_at)
VALUES (@request_id, @donor_user_id, @clinic_id, @scheduled_at)
RETURNING id, request_id, donor_user_id, clinic_id, scheduled_at, status, created_at, updated_at;

-- name: GetAppointment :one
SELECT id, request_id, donor_user_id, clinic_id, scheduled_at, status, created_at, updated_at
FROM appointments
WHERE id = @id;

-- name: ListAppointmentsForDonor :many
SELECT id, request_id, donor_user_id, clinic_id, scheduled_at, status, created_at, updated_at
FROM appointments
WHERE donor_user_id = @donor_user_id
ORDER BY scheduled_at
LIMIT @row_limit OFFSET @row_offset;

-- name: ListAppointmentsForClinic :many
SELECT id, request_id, donor_user_id, clinic_id, scheduled_at, status, created_at, updated_at
FROM appointments
WHERE clinic_id = @clinic_id
ORDER BY scheduled_at
LIMIT @row_limit OFFSET @row_offset;

-- name: SetAppointmentStatus :one
-- Cancel/complete/no_show. Cancelling frees the slot (it leaves the partial
-- unique index), so the time can be rebooked.
UPDATE appointments
SET status = @status, updated_at = now()
WHERE id = @id
RETURNING id, request_id, donor_user_id, clinic_id, scheduled_at, status, created_at, updated_at;
