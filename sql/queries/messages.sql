-- name: SaveMessage :exec
INSERT INTO messages (id, type, user_id, room_id, timestamp, payload, signature, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, NOW() + INTERVAL '60 days');

-- name: GetMessage :one
SELECT id, type, user_id, room_id, timestamp, payload, signature, created_at
FROM messages
WHERE id = $1 LIMIT 1;

-- name: GetRoomMessages :many
SELECT id, type, user_id, room_id, timestamp, payload, signature, created_at
FROM messages
WHERE room_id = $1
ORDER BY timestamp DESC
LIMIT $2;

-- name: CleanupOldMessages :exec
DELETE FROM messages WHERE expires_at < NOW();

-- name: GetReplies :many
SELECT *
FROM messages
WHERE reply_to_id = $1
AND deleted_at IS NULL
ORDER BY created_at ASC;

-- name: SoftDelete :exec
UPDATE messages
SET deleted_at = NOW()
WHERE id = $1;

-- name: HardDelete :exec
