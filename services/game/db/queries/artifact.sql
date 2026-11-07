-- Equipped pháp bảo (internal/artifact).

-- name: GetEquippedArtifact :one
SELECT * FROM character_artifacts WHERE character_id = $1;

-- name: GetEquippedArtifactForUpdate :one
SELECT * FROM character_artifacts WHERE character_id = $1 FOR UPDATE;

-- name: EquipArtifact :one
INSERT INTO character_artifacts (character_id, item_id, equipped_at) VALUES (@character_id, @item_id, @equipped_at)
ON CONFLICT (character_id) DO UPDATE SET item_id = EXCLUDED.item_id, equipped_at = EXCLUDED.equipped_at
RETURNING *;

-- name: UnequipArtifact :exec
DELETE FROM character_artifacts WHERE character_id = $1;
