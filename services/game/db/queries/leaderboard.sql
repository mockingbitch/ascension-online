-- name: GetProgressionsByIDs :many
SELECT character_id, stage_id, cultivation_milli FROM character_progressions
WHERE character_id = ANY(@ids::uuid[]);

-- name: ListProgressionsPage :many
-- Keyset pagination for leaderboard rebuilds.
SELECT character_id, stage_id, cultivation_milli FROM character_progressions
WHERE character_id > @after
ORDER BY character_id
LIMIT @lim;

-- name: GetLeaderboardRows :many
-- Appearance (frame/title) is display only: it never affects the score.
-- vip_points lets the caller hide Tiên Duyên perks above the current level (cosmetic.Visible).
SELECT c.id, c.name, p.stage_id, p.stage_reached_at, a.avatar_frame_id, a.title_id, COALESCE(v.points, 0)::BIGINT AS vip_points
FROM characters c JOIN character_progressions p ON p.character_id = c.id
LEFT JOIN character_appearance a ON a.character_id = c.id
LEFT JOIN vip_accounts v ON v.user_id = c.user_id
WHERE c.id = ANY(@ids::uuid[]);
