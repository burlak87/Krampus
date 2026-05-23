SELECT
    message_id,
    room_id,
    user_id,
    content,
    created_at
FROM message_search_projection
WHERE created_at < $1
AND search_vector @@ plainto_tsquery(
    'simple',
    $2
)
ORDER BY created_at DESC
LIMIT $3;
