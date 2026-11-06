-- name: GetIdentity :one
SELECT * FROM user_identities WHERE provider = $1 AND subject = $2;

-- name: InsertIdentity :exec
INSERT INTO user_identities (provider, subject, user_id, email, email_verified)
VALUES ($1, $2, $3, $4, $5);

-- name: TouchIdentity :exec
UPDATE user_identities SET email = $3, email_verified = $4, last_login_at = now()
WHERE provider = $1 AND subject = $2;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE lower(email) = lower($1);

-- name: GetUser :one
SELECT * FROM users WHERE id = $1;

-- name: CreateUser :one
INSERT INTO users (email, display_name, avatar_url, last_login_at)
VALUES ($1, $2, $3, now())
RETURNING *;

-- name: TouchUserLogin :exec
UPDATE users SET display_name = $2, avatar_url = $3, last_login_at = now(), updated_at = now()
WHERE id = $1;

-- name: InsertRefreshToken :one
INSERT INTO refresh_tokens (user_id, token_hash, family_id, expires_at, user_agent)
VALUES ($1, $2, $3, $4, $5)
RETURNING id;

-- name: GetRefreshTokenForUpdate :one
SELECT * FROM refresh_tokens WHERE token_hash = $1 FOR UPDATE;

-- name: RotateRefreshToken :exec
UPDATE refresh_tokens SET revoked_at = now(), replaced_by = $2, last_used_at = now()
WHERE id = $1;

-- name: RevokeRefreshFamily :exec
UPDATE refresh_tokens SET revoked_at = now()
WHERE family_id = $1 AND revoked_at IS NULL;
