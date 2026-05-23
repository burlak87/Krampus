SELECT
    message_id,
    room_id,
    user_id,
    content,
    similarity(content, $1) AS score

FROM message_search_projection

WHERE content % $1

ORDER BY score DESC

LIMIT $2;
