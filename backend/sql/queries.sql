-- name: ListActiveKeys :many
-- Newest first; legacy rows (NULL created_at) sort last.
SELECT * FROM api_keys WHERE status = 'active'
ORDER BY COALESCE(created_at, '') DESC, rowid DESC;

-- name: ListKeys :many
SELECT * FROM api_keys
ORDER BY COALESCE(created_at, '') DESC, rowid DESC;

-- name: GetPrimedKey :one
SELECT * FROM api_keys WHERE status = 'primed'
ORDER BY COALESCE(created_at, '') DESC, rowid DESC LIMIT 1;

-- name: CreateKey :exec
INSERT INTO api_keys (id, key, status, created_at) VALUES (:id, :key, :status, :created_at);

-- name: GetKeyByID :one
SELECT * FROM api_keys WHERE id = :id;

-- name: ActivateKey :exec
UPDATE api_keys SET status = 'active' WHERE id = :id;

-- name: RetireKey :execrows
UPDATE api_keys SET status = 'inactive' WHERE id = :id AND status = 'active';

-- name: TouchKeyLastUsed :exec
-- Only forward: a slow request's older stamp never overwrites a newer one
-- (fixed-width UTC text, so the comparison is chronological).
UPDATE api_keys SET last_used_at = :last_used_at
WHERE id = :id AND (last_used_at IS NULL OR last_used_at < :last_used_at);

-- name: UnprimeAll :exec
UPDATE api_keys SET status = 'inactive' WHERE status = 'primed';
