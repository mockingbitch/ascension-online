-- name: GetCosmeticPurchaseByKey :one
SELECT * FROM character_cosmetics WHERE character_id = $1 AND idempotency_key = $2;

-- name: ListOwnedCosmetics :many
SELECT cosmetic_id FROM character_cosmetics WHERE character_id = $1 ORDER BY acquired_at, cosmetic_id;

-- name: CosmeticOwned :one
SELECT EXISTS (SELECT 1 FROM character_cosmetics WHERE character_id = $1 AND cosmetic_id = $2) AS owned;

-- name: InsertCosmeticOwnership :exec
INSERT INTO character_cosmetics (character_id, cosmetic_id, source, price, idempotency_key, acquired_at)
VALUES (@character_id, @cosmetic_id, @source, @price, @idempotency_key, @now);

-- name: EnsureAppearance :exec
INSERT INTO character_appearance (character_id) VALUES ($1) ON CONFLICT DO NOTHING;

-- name: GetAppearanceForUpdate :one
SELECT * FROM character_appearance WHERE character_id = $1 FOR UPDATE;

-- name: GetAppearance :one
SELECT * FROM character_appearance WHERE character_id = $1;

-- name: SetAppearance :exec
UPDATE character_appearance
SET title_id = @title_id, avatar_frame_id = @avatar_frame_id, chat_aura_id = @chat_aura_id,
    cave_decoration_ids = @cave_decoration_ids::text[], version = version + 1, updated_at = @now
WHERE character_id = @character_id;

-- name: GetCosmeticEquipByKey :one
SELECT * FROM cosmetic_equips WHERE character_id = $1 AND idempotency_key = $2;

-- name: InsertCosmeticEquip :exec
INSERT INTO cosmetic_equips (character_id, title_id, avatar_frame_id, chat_aura_id, cave_decoration_ids, idempotency_key, created_at)
VALUES (@character_id, @title_id, @avatar_frame_id, @chat_aura_id, @cave_decoration_ids::text[], @idempotency_key, @now);
