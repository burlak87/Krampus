-- name: UpdateProfile :exec
UPDATE user_profiles
SET
    username = $1,
    bio = $2,
    version = version + 1
WHERE user_id = $3
AND version = $4;
