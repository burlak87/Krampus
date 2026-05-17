-- name: GetMessagesAfter :many
SELECT id, room_id, user_id, type, payload, timestamp, signature
FROM messages
WHERE room_id = sqlc.arg(room_id)
  AND timestamp > sqlc.arg(after_ts)
ORDER BY timestamp ASC
LIMIT sqlc.arg(limit_count);
