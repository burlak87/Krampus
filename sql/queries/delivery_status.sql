-- name: UpsertDeliveryStatus :exec
INSERT INTO message_delivery_status (message_id, user_id, status, delivered_at, read_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT(message_id, user_id)
DO UPDATE SET
    status = EXCLUDED.status,
    delivered_at = EXCLUDED.delivered_at,
    read_at = EXCLUDED.read_at,
    updated_at = NOW();
