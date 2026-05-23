-- name: SearchMessages :many
SELECT *
FROM messages
WHERE search_vector @@ plainto_tsquery($1)
AND deleted_at IS NULL
AND (
    scheduled_at IS NULL
    OR scheduled_at <= NOW()
)
ORDER BY ts_rank(search_vector, plainto_tsquery($1)) DESC
LIMIT $2 OFFSET $3;
