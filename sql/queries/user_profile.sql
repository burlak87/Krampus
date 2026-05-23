-- name: UpdateProfile :exec
UPDATE user_profiles
SET
    display_name = $1,
    bio = $2,
    version = version + 1
WHERE id = $3
AND version = $4;
