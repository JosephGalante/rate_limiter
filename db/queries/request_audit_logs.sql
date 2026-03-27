-- name: CreateRequestAuditLog :one
INSERT INTO request_audit_logs (
    api_key_id,
    route_id,
    policy_id,
    outcome,
    request_cost,
    tokens_remaining
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6
)
RETURNING *;

-- name: ListRecentRequestAuditLogs :many
SELECT *
FROM request_audit_logs
ORDER BY blocked_at DESC
LIMIT $1;
