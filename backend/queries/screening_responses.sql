-- Explicit SQL for preliminary screening (FR-03/FR-04). One row per submission;
-- the most recent row is the donor's current screening state.

-- name: CreateScreeningResponse :one
INSERT INTO screening_responses (donor_user_id, answers, has_flags)
VALUES (@donor_user_id, @answers, @has_flags)
RETURNING id, donor_user_id, answers, has_flags, submitted_at;

-- name: GetLatestScreeningResponse :one
SELECT id, donor_user_id, answers, has_flags, submitted_at
FROM screening_responses
WHERE donor_user_id = @donor_user_id
ORDER BY submitted_at DESC
LIMIT 1;
