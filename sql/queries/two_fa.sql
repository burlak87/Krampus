-- name: CreateTwoFaCode :one
INSERT INTO two_fa_codes (
    user_id,
    code,
    expires_at
) VALUES (
    $1, $2, $3
) RETURNING id;

-- name: GetTwoFaCodeByUserID :one
SELECT 
    id,
    user_id,
    code,
    expires_at,
    attempts,
    is_used,
    created_at
FROM two_fa_codes 
WHERE user_id = $1 
  AND is_used = false 
  AND expires_at > CURRENT_TIMESTAMP
ORDER BY created_at DESC 
LIMIT 1;

-- name: UpdateTwoFaCodeAttempts :exec
UPDATE two_fa_codes 
SET attempts = $2 
WHERE id = $1; 

-- name: MarkTwoFaCodeAsUsed :exec
UPDATE two_fa_codes 
SET is_used = true 
WHERE id = $1;

-- name: GetRecentCodeRequests :one
SELECT COUNT(*) as count
FROM two_fa_codes 
WHERE user_id = $1 
  AND created_at >= $2;

-- name: GetRecentVerificationAttempts :one
SELECT COUNT(*) as count
FROM two_fa_codes 
WHERE user_id = $1 
  AND created_at >= $2 
  AND attempts > 0;