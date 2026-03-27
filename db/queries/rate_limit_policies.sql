-- name: CreateRateLimitPolicy :one
INSERT INTO rate_limit_policies (
    scope_type,
    scope_identifier,
    route_pattern,
    capacity,
    refill_tokens,
    refill_interval_seconds
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6
)
RETURNING *;

-- name: GetRateLimitPolicy :one
SELECT *
FROM rate_limit_policies
WHERE id = $1;

-- name: ListRateLimitPolicies :many
SELECT *
FROM rate_limit_policies
ORDER BY created_at DESC;

-- name: ListActiveRateLimitPolicies :many
SELECT *
FROM rate_limit_policies
WHERE is_active = TRUE
ORDER BY scope_type, created_at DESC;

-- name: FindRateLimitPolicyByScope :one
SELECT *
FROM rate_limit_policies
WHERE is_active = TRUE
  AND scope_type = $1
  AND scope_identifier IS NOT DISTINCT FROM $2
  AND route_pattern IS NOT DISTINCT FROM $3
LIMIT 1;

-- name: UpdateRateLimitPolicy :one
UPDATE rate_limit_policies
SET scope_type = $2,
    scope_identifier = $3,
    route_pattern = $4,
    capacity = $5,
    refill_tokens = $6,
    refill_interval_seconds = $7,
    is_active = $8
WHERE id = $1
RETURNING *;

-- name: DeactivateRateLimitPolicy :one
UPDATE rate_limit_policies
SET is_active = FALSE
WHERE id = $1
RETURNING *;
