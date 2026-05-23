SELECT
    message_id,
    room_id,
    user_id,
    content,
    created_at,

    ts_rank(
        search_vector,
        plainto_tsquery('simple', $1)
    ) AS rank

FROM message_search_projection

WHERE search_vector @@ plainto_tsquery(
    'simple',
    $1
)

ORDER BY
    rank DESC,
    created_at DESC

LIMIT $2
OFFSET $3;
