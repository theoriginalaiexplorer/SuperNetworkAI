-- name: GetUserByID :one
SELECT id, email, created_at, updated_at
FROM users
WHERE id = $1;

-- name: UpsertUser :one
INSERT INTO users (id, email)
VALUES ($1, $2)
ON CONFLICT (id) DO UPDATE SET
  email = EXCLUDED.email,
  updated_at = NOW()
RETURNING *;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = $1;
