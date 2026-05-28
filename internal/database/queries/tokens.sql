-- name: CreateToken :execlastid
INSERT INTO auth_tokens (
    token_id, token_type, client_info, expires_at, revoked_at,
    revoked_by, created_at, revoked_reason
) VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetTokenByID :one
SELECT id, token_id, token_type, client_info, expires_at, revoked_at,
    revoked_by, created_at, revoked_reason
FROM auth_tokens WHERE token_id = ?;

-- name: RevokeToken :execrows
UPDATE auth_tokens
SET revoked_at = ?, revoked_by = ?, revoked_reason = ?
WHERE token_id = ?;

-- name: CleanExpiredTokens :execrows
DELETE FROM auth_tokens WHERE expires_at < ?;

-- name: IsTokenRevoked :one
SELECT EXISTS(
    SELECT 1 FROM auth_tokens
    WHERE token_id = ? AND (revoked_at IS NOT NULL OR expires_at < ?)
);
