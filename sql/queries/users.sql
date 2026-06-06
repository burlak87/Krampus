-- name: CreateUser :one
INSERT INTO users (
    username,
    firstname,
    lastname,
    email,
    password_hash,
    two_fa_enabled
) VALUES (
    $1, $2, $3, $4, $5, $6
) RETURNING *;

-- name: GetUserByEmail :one
SELECT
    id,
    username,
    firstname,
    lastname,
    email,
    password_hash,
    two_fa_enabled,
    created_at,
    blocked_until,
    failed_attempts
FROM users
WHERE email = $1
LIMIT 1;

-- name: GetUserByID :one
SELECT
    id,
    username,
    firstname,
    lastname,
    email,
    password_hash,
    two_fa_enabled,
    created_at,
    blocked_until,
    failed_attempts
FROM users
WHERE id = $1
LIMIT 1;

-- name: ListUsers :many
SELECT
    id,
    username,
    firstname,
    lastname,
    email,
    two_fa_enabled,
    created_at
FROM users
ORDER BY username
LIMIT $1 OFFSET $2;

-- name: CreateRefreshToken :exec
INSERT INTO refresh_tokens (user_id, token, expires_at)
VALUES ($1, $2, $3);

-- name: GetRefreshToken :one
SELECT user_id, expires_at FROM refresh_tokens
WHERE token = $1
LIMIT 1;

-- name: DeleteRefreshToken :exec
DELETE FROM refresh_tokens
WHERE token = $1;

-- name: RefreshDeleteByUserI :exec
DELETE FROM refresh_tokens
WHERE user_id = $1;

-- name: CreateLoginAttempt :exec
INSERT INTO login_attempts (email, success, attempted_at)
VALUES ($1, $2, $3);

-- name: GetRecentFailedAttempts :one
SELECT COUNT(*) as count
FROM login_attempts
WHERE email = $1
  AND success = false
  AND attempted_at >= $2;

-- name: GetBlockedStatus :one
SELECT blocked_until
FROM users
WHERE email = $1;

-- name: UpdateTwoFAStatus :exec
UPDATE users
SET two_fa_enabled = $2
WHERE id = $1;

-- name: GetFailedLogAttempts :one
SELECT COUNT(*) as count
FROM login_attempts
WHERE email = $1
  AND success = false
  AND attempted_at >= $2;

-- name: BlockUser :exec
UPDATE users
SET
    blocked_until = $2,
    failed_attempts = failed_attempts + 1,
    last_failed_attempt = CURRENT_TIMESTAMP
WHERE email = $1;

-- name: ResetFailedAttempts :exec
UPDATE users
SET
    failed_attempts = 0,
    blocked_until = NULL
WHERE email = $1;

-- name: UpdatePasswordHash :exec
UPDATE users
SET password_hash = $2
WHERE id = $1;

-- name: DeleteExpiredRefreshTokens :exec
DELETE FROM refresh_tokens
WHERE expires_at <= CURRENT_TIMESTAMP;

-- name: RefreshDeleteByUserID :exec
DELETE FROM refresh_tokens
WHERE user_id = $1;

-- name: UpdateUserLastActive :exec
UPDATE users
SET updated_at = $2
WHERE id = $1;

-- name: CreateUserClient :exec
INSERT INTO users (id, username, firstname, lastname, email, password_hash)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: UpdateUserClient :exec
UPDATE users
SET username = $2, firstname = $3, lastname = $4, email = $5
WHERE id = $1;
