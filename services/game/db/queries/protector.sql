-- Hộ pháp (internal/protector).

-- name: InsertProtectorInvite :one
INSERT INTO protector_invites (character_id, faction_id, stage_id, idempotency_key, created_at, expires_at)
VALUES (@character_id, @faction_id, @stage_id, @idempotency_key, @created_at, @expires_at)
RETURNING *;

-- name: GetProtectorInviteByKey :one
SELECT * FROM protector_invites WHERE character_id = $1 AND idempotency_key = $2;

-- name: GetOpenProtectorInvite :one
-- The character's newest unused, unexpired invite.
SELECT * FROM protector_invites
WHERE character_id = @character_id AND used_at IS NULL AND expires_at > @now
ORDER BY created_at DESC LIMIT 1;

-- name: GetOpenProtectorInviteForUpdate :one
SELECT * FROM protector_invites
WHERE character_id = @character_id AND used_at IS NULL AND expires_at > @now
ORDER BY created_at DESC LIMIT 1
FOR UPDATE;

-- name: GetProtectorInviteForUpdate :one
SELECT * FROM protector_invites WHERE id = $1 FOR UPDATE;

-- name: UseProtectorInvite :execrows
UPDATE protector_invites SET used_at = @used_at, used_ref = @used_ref WHERE id = @id AND used_at IS NULL;

-- name: ListFactionOpenInvites :many
-- Open invites of a faction (newest first), with the inviter's name and stage, and the number of protectors.
SELECT i.id, i.character_id, i.stage_id, i.created_at, i.expires_at, c.name,
       (SELECT count(*) FROM protector_assists a WHERE a.invite_id = i.id)::INT AS protectors
FROM protector_invites i JOIN characters c ON c.id = i.character_id
WHERE i.faction_id = @faction_id AND i.used_at IS NULL AND i.expires_at > @now
ORDER BY i.created_at DESC LIMIT @lim;

-- name: ListInviteProtectors :many
-- Accepted protectors of an invite with their CURRENT faction (NULL = none), oldest first.
SELECT a.id, a.protector_character_id, a.accepted_at, a.counted_at, c.name, c.user_id, cf.faction_id
FROM protector_assists a
JOIN characters c ON c.id = a.protector_character_id
LEFT JOIN character_faction cf ON cf.character_id = a.protector_character_id
WHERE a.invite_id = $1
ORDER BY a.accepted_at, a.id;

-- name: GetProtectorAssistByKey :one
SELECT * FROM protector_assists WHERE protector_character_id = $1 AND idempotency_key = $2;

-- name: GetProtectorAssist :one
SELECT * FROM protector_assists WHERE invite_id = $1 AND protector_character_id = $2;

-- name: InsertProtectorAssist :one
INSERT INTO protector_assists (invite_id, protector_character_id, game_day, idempotency_key, accepted_at)
VALUES (@invite_id, @protector_character_id, @game_day, @idempotency_key, @accepted_at)
RETURNING *;

-- name: CountProtectorAssist :exec
UPDATE protector_assists SET counted_at = @counted_at WHERE id = @id;

-- name: LockProtectorDay :one
-- Ensure + lock the protector's day row (serializes their accepts).
INSERT INTO protector_days (character_id, game_day, uses) VALUES (@character_id, @game_day, 0)
ON CONFLICT (character_id, game_day) DO UPDATE SET uses = protector_days.uses
RETURNING uses;

-- name: SetProtectorDayUses :exec
UPDATE protector_days SET uses = @uses WHERE character_id = @character_id AND game_day = @game_day;

-- name: CountProtectorUses :one
-- Uses of a game day: assists accepted that day that were counted, or are pending on an open invite.
SELECT count(*)::INT FROM protector_assists a JOIN protector_invites i ON i.id = a.invite_id
WHERE a.protector_character_id = @protector_character_id AND a.game_day = @game_day
  AND (a.counted_at IS NOT NULL OR (i.used_at IS NULL AND i.expires_at > @now));

-- name: ListProtectorPairs :many
-- Admin review (anti-cheat M2): inviter → protector pairs since a day, with assists and counted assists.
SELECT i.character_id AS inviter_id, ci.name AS inviter_name, a.protector_character_id AS protector_id, cp.name AS protector_name,
       count(*)::INT AS assists, count(a.counted_at)::INT AS counted,
       min(a.game_day)::DATE AS first_day, max(a.game_day)::DATE AS last_day
FROM protector_assists a
JOIN protector_invites i ON i.id = a.invite_id
JOIN characters ci ON ci.id = i.character_id
JOIN characters cp ON cp.id = a.protector_character_id
WHERE a.game_day >= @since
GROUP BY i.character_id, ci.name, a.protector_character_id, cp.name
HAVING count(*) >= @min_assists
ORDER BY count(*) DESC, max(a.game_day) DESC
LIMIT @lim;

-- name: GetCharacterCreatedAt :one
SELECT created_at FROM characters WHERE id = $1;

-- name: ListMyAcceptedInvites :many
-- Invites this protector accepted that are still open (to show "đang hộ pháp").
SELECT a.invite_id FROM protector_assists a JOIN protector_invites i ON i.id = a.invite_id
WHERE a.protector_character_id = @protector_character_id AND i.used_at IS NULL AND i.expires_at > @now;

-- name: ListCharacterBriefs :many
SELECT c.id, c.name, p.stage_id FROM characters c JOIN character_progressions p ON p.character_id = c.id
WHERE c.id = ANY(@ids::uuid[]);
