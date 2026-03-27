-- name: CreateAPIKey :one
INSERT INTO api_keys (
    user_id,
    name,
    key_prefix,
    key_hash
) VALUES (
    $1,
    $2,
    $3,
    $4
)
RETURNING *;

-- name: GetAPIKeyByID :one
SELECT *
FROM api_keys
WHERE id = $1;

-- name: GetAPIKeyByHash :one
SELECT *
FROM api_keys
WHERE key_hash = $1
  AND is_active = TRUE;

-- name: ListAPIKeys :many
SELECT *
FROM api_keys
ORDER BY created_at DESC;

-- name: DeactivateAPIKey :one
UPDATE api_keys
SET is_active = FALSE
WHERE id = $1
RETURNING *;

-- name: TouchAPIKeyLastUsed :one
UPDATE api_keys
SET last_used_at = NOW()
WHERE id = $1
RETURNING *;
