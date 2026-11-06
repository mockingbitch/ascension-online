-- name: GetCharacterByUser :one
SELECT * FROM characters WHERE user_id = $1;

-- name: CreateCharacter :one
INSERT INTO characters (user_id, name) VALUES ($1, $2)
RETURNING *;

-- name: CreateProgression :exec
INSERT INTO character_progressions (character_id, realm_id, stage_id, last_cultivated_at, stage_reached_at, grant_reserve_milli)
VALUES (@character_id, @realm_id, @stage_id, @now, @now, @grant_reserve_milli);

-- name: EnsureCurrencyAccount :exec
INSERT INTO currency_accounts (character_id, currency) VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: GetCharacterByID :one
SELECT * FROM characters WHERE id = $1;

-- name: GetCharacterForUpdate :one
SELECT * FROM characters WHERE id = $1 FOR UPDATE;

-- name: RenameCharacter :exec
UPDATE characters SET name = $2 WHERE id = $1;
