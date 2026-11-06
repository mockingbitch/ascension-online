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
SELECT c.id, c.name, p.stage_id, p.stage_reached_at
FROM characters c JOIN character_progressions p ON p.character_id = c.id
WHERE c.id = ANY(@ids::uuid[]);
