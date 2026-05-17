-- name: CreateDLQMessage :exec
INSERT INTO message_dlq (id, message_id, user_id, room_id, payload, reason)
VALUES ($1, $2, $3, $4, $5, $6);
