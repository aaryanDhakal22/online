-- name: GetActiveKey :one
SELECT * FROM api_keys WHERE status = 'active' ORDER BY id DESC LIMIT 1;

-- name: GetPrimedKey :one
SELECT * FROM api_keys WHERE status = 'primed' ORDER BY id ASC LIMIT 1;

-- name: CreateKey :exec
INSERT INTO api_keys (id, key, status) VALUES (:id, :key, :status);

-- name: GetKeyByID :one
SELECT * FROM api_keys WHERE id = :id;

-- name: DeleteKey :exec
DELETE FROM api_keys WHERE id = :id;

-- name: ActivateKey :exec
UPDATE api_keys SET status = 'active' WHERE id = :id;

-- name: DeactivateKey :exec
UPDATE api_keys Set status = 'inactive' WHERE status = 'active';

-- name: UnprimeAll :exec
UPDATE api_keys SET status = 'inactive' WHERE status = 'primed';
