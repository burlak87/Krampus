-- name: CreateRetryJob :exec
INSERT INTO message_retry_queue (id, message_id, user_id, room_id, payload, attempt, next_retry_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: GetReadyRetryJobs :many
SELECT id, message_id, user_id, room_id, payload, attempt, next_retry_at, created_at
FROM message_retry_queue
WHERE next_retry_at <= NOW()
ORDER BY next_retry_at ASC
LIMIT $1;

-- name: DeleteRetryJob :exec
DELETE FROM message_retry_queue
WHERE message_id = $1
AND user_id = $2;
