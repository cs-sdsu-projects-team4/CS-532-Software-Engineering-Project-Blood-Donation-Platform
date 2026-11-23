-- Explicit SQL for the append-only activity log (FR-13). actor_user_id is null for
-- system actions (e.g. the matching worker); entity_type/entity_id point at the
-- affected row; metadata carries action-specific detail as JSONB.

-- name: CreateActivityLog :one
INSERT INTO activity_logs (actor_user_id, action, entity_type, entity_id, metadata)
VALUES (
    sqlc.narg('actor_user_id'), @action,
    sqlc.narg('entity_type'), sqlc.narg('entity_id'), sqlc.narg('metadata')
)
RETURNING id, actor_user_id, action, entity_type, entity_id, metadata, created_at;

-- name: ListActivityLogsForEntity :many
SELECT id, actor_user_id, action, entity_type, entity_id, metadata, created_at
FROM activity_logs
WHERE entity_type = @entity_type AND entity_id = @entity_id
ORDER BY created_at DESC;
