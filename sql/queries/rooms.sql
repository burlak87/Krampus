-- name: GetRoomByID :one
SELECT id, type, owner_id, name, members, settings, created_at, updated_at
FROM rooms
WHERE id = $1 LIMIT 1;

-- name: UpsertRoom :exec
INSERT INTO rooms (id, type, owner_id, name, members, settings, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    members = EXCLUDED.members,
    settings = EXCLUDED.settings,
    updated_at = EXCLUDED.updated_at;

-- name: UpdateRoom :exec
UPDATE rooms
SET name = $2, members = $3, settings = $4, updated_at = $5
WHERE id = $1;

-- name: DeleteRoom :exec
DELETE FROM rooms WHERE id = $1;

-- name: ListUserRooms :many
SELECT * FROM rooms
WHERE members @> $1::jsonb;
