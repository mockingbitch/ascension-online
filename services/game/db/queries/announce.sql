-- name: InsertRealmFirst :execrows
-- Race-free "first to reach this realm": exactly one row per realm id ever (PK), the first commit wins.
INSERT INTO realm_firsts (realm_id, character_id, reached_at) VALUES (@realm_id, @character_id, @now)
ON CONFLICT (realm_id) DO NOTHING;

-- name: InsertWorldAnnouncement :one
-- Idempotent per outbox event (source_event_id UNIQUE): a replayed drain gets the existing row back.
INSERT INTO world_announcements (source_event_id, kind, tier, character_id, name, realm_id, stage_id, path, rank, count, text, created_at)
-- character_id resolves to NULL when the character is gone by the time the worker writes (never an FK error).
VALUES (sqlc.narg(source_event_id), @kind, @tier, (SELECT c.id FROM characters c WHERE c.id = sqlc.narg(character_id)), @name, sqlc.narg(realm_id), sqlc.narg(stage_id),
        sqlc.narg(path), sqlc.narg(rank), sqlc.narg(count), @text, @now)
ON CONFLICT (source_event_id) DO UPDATE SET source_event_id = EXCLUDED.source_event_id
RETURNING id, (xmax = 0) AS inserted;

-- name: ListWorldAnnouncements :many
SELECT id, kind, tier, character_id, name, realm_id, stage_id, path, rank, count, text, created_at
FROM world_announcements
ORDER BY created_at DESC, id DESC
LIMIT @lim;

-- name: PruneWorldAnnouncements :execrows
DELETE FROM world_announcements WHERE created_at < @before;

-- name: DeleteWorldAnnouncementsOfCharacter :execrows
-- A moderated rename: the old đạo hiệu leaves the public history at once.
DELETE FROM world_announcements WHERE character_id = $1;
