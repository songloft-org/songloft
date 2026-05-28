-- name: GetConfig :one
SELECT id, key, value, updated_at FROM configs WHERE key = ?;

-- name: SetConfig :exec
INSERT INTO configs (key, value) VALUES (?, ?)
ON CONFLICT(key) DO UPDATE SET value = excluded.value;

-- name: DeleteConfig :execrows
DELETE FROM configs WHERE key = ?;
