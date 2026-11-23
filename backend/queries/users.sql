-- Explicit SQL for the `users` table. sqlc reads these annotations and generates
-- type-safe Go into internal/store. Never hand-edit the generated output.

-- name: CreateUser :one
INSERT INTO users (email, password_hash, role, full_name, phone)
VALUES (@email, @password_hash, @role, @full_name, @phone)
RETURNING id, email, role, full_name, phone, created_at, updated_at;

-- name: GetUserByEmail :one
-- Used by login; this is the ONE place password_hash is read out.
SELECT id, email, password_hash, role, full_name, phone, created_at, updated_at
FROM users
WHERE email = @email;

-- name: GetUserByID :one
SELECT id, email, role, full_name, phone, created_at, updated_at
FROM users
WHERE id = @id;

-- name: EmailExists :one
SELECT EXISTS (SELECT 1 FROM users WHERE email = @email) AS exists;
