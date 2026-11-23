-- Explicit SQL for push_tokens (FR-09). FCM device tokens the backend worker uses
-- to notify donors. A token is globally unique; re-registering an existing token
-- just re-points it at the current user/platform.

-- name: UpsertPushToken :one
INSERT INTO push_tokens (user_id, token, platform)
VALUES (@user_id, @token, @platform)
ON CONFLICT (token) DO UPDATE
    SET user_id = EXCLUDED.user_id, platform = EXCLUDED.platform, updated_at = now()
RETURNING id, user_id, token, platform, created_at, updated_at;

-- name: ListPushTokensForUser :many
SELECT id, user_id, token, platform, created_at, updated_at
FROM push_tokens
WHERE user_id = @user_id;

-- name: DeletePushToken :exec
DELETE FROM push_tokens
WHERE token = @token AND user_id = @user_id;
