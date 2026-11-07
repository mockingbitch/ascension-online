-- Factions (data model §6.2). Lock order (§2.3): character_progressions → character_faction.

-- name: GetCharacterFaction :one
SELECT * FROM character_faction WHERE character_id = $1;

-- name: GetCharacterFactionForUpdate :one
SELECT * FROM character_faction WHERE character_id = $1 FOR UPDATE;

-- name: InsertCharacterFaction :exec
INSERT INTO character_faction (character_id, faction_id, joined_at, last_switch_at, updated_at)
VALUES (@character_id, @faction_id, @joined_at, sqlc.narg(last_switch_at), @joined_at);

-- name: MoveCharacterFaction :exec
-- Player switch / split choice: new faction, pending split cleared. last_switch_at is passed through (unchanged for a
-- split choice, now for a switch).
UPDATE character_faction
SET faction_id = @faction_id, joined_at = @joined_at, last_switch_at = sqlc.narg(last_switch_at),
    pending_split_effect_id = NULL, version = version + 1, updated_at = @joined_at
WHERE character_id = @character_id;

-- name: SetFactionLock :exec
UPDATE character_faction
SET lock_reason = sqlc.narg(lock_reason), locked_at = sqlc.narg(locked_at), locked_until = sqlc.narg(locked_until),
    version = version + 1, updated_at = now()
WHERE character_id = @character_id;

-- name: InsertFactionHistory :exec
INSERT INTO faction_history (character_id, from_faction_id, to_faction_id, from_path, to_path, reason,
                             world_effect_id, idempotency_key, created_at)
VALUES (@character_id, sqlc.narg(from_faction_id), @to_faction_id, sqlc.narg(from_path), sqlc.narg(to_path), @reason,
        sqlc.narg(world_effect_id), @idempotency_key, @created_at);

-- name: GetFactionHistoryByKey :one
SELECT * FROM faction_history WHERE character_id = $1 AND idempotency_key = $2;

-- name: ListFactionHistory :many
SELECT * FROM faction_history WHERE character_id = @character_id ORDER BY created_at DESC, id DESC LIMIT @lim;

-- name: ConvertProgressionPath :exec
-- Path conversion (§3.8): same tier/stage on another path; cultivation, carry, reserve, streak, cooldown are kept.
UPDATE character_progressions
SET realm_id = @realm_id, stage_id = @stage_id, version = version + 1, updated_at = now()
WHERE character_id = @character_id;

-- name: MigrateFactionBatch :one
-- migrate_faction: move up to @lim members of @from_faction to @to_faction with one history row each. Returns the
-- number of members moved (not history rows inserted, L3).
WITH batch AS (
    SELECT character_id FROM character_faction
    WHERE faction_id = @from_faction::text
    ORDER BY character_id
    LIMIT @lim
    FOR UPDATE
), moved AS (
    UPDATE character_faction cf
    SET faction_id = @to_faction, joined_at = @now, pending_split_effect_id = NULL, version = cf.version + 1,
        updated_at = @now
    FROM batch WHERE cf.character_id = batch.character_id
    RETURNING cf.character_id
), hist AS (
    INSERT INTO faction_history (character_id, from_faction_id, to_faction_id, reason, world_effect_id, idempotency_key,
                                 created_at)
    SELECT character_id, @from_faction::text, @to_faction::text, 'auto_migrate', @effect_id::bigint,
           'effect:' || @effect_id::bigint, @now
    FROM moved
    ON CONFLICT (character_id, idempotency_key) DO NOTHING
)
SELECT count(*)::bigint AS moved FROM moved;

-- name: SetPendingSplitBatch :execrows
-- split_faction at due_at: mark up to @lim members of @from_faction as having to choose a branch.
WITH batch AS (
    SELECT b.character_id FROM character_faction b
    WHERE b.faction_id = @from_faction AND b.pending_split_effect_id IS NULL
    ORDER BY b.character_id
    LIMIT @lim
    FOR UPDATE
)
UPDATE character_faction cf
SET pending_split_effect_id = @effect_id, version = cf.version + 1, updated_at = @now
FROM batch WHERE cf.character_id = batch.character_id;

-- name: ListPendingSplitBatch :many
-- split_faction after the deadline: the next @lim still-pending members, deterministic order, locked.
SELECT character_id, faction_id FROM character_faction
WHERE pending_split_effect_id = @effect_id
ORDER BY character_id
LIMIT @lim
FOR UPDATE;

-- name: CountSplitPlacements :many
-- Members placed in each branch BY THIS SPLIT so far (choices + defaults), the basis of the default (H2): alts or
-- switchers who joined a branch directly don't steer it.
SELECT to_faction_id, COUNT(*)::bigint AS placed FROM faction_history
WHERE world_effect_id = @effect_id AND reason IN ('split_choice', 'split_default')
GROUP BY to_faction_id;

-- name: AssignSplitDefaults :one
-- Move pending members to their computed default branch (parallel arrays), one history row each. Returns members moved.
WITH plan AS (
    SELECT unnest(@character_ids::uuid[]) AS character_id, unnest(@to_factions::text[]) AS to_faction
), moved AS (
    UPDATE character_faction cf
    SET faction_id = plan.to_faction, joined_at = @now, pending_split_effect_id = NULL, version = cf.version + 1,
        updated_at = @now
    FROM plan
    WHERE cf.character_id = plan.character_id AND cf.pending_split_effect_id = @effect_id::bigint
    RETURNING cf.character_id, plan.to_faction
), hist AS (
    INSERT INTO faction_history (character_id, from_faction_id, to_faction_id, reason, world_effect_id, idempotency_key,
                                 created_at)
    SELECT character_id, @from_faction::text, to_faction, 'split_default', @effect_id::bigint,
           'effect:' || @effect_id::bigint, @now
    FROM moved
    ON CONFLICT (character_id, idempotency_key) DO NOTHING
)
SELECT count(*)::bigint AS moved FROM moved;

-- name: CountFactionMembers :many
SELECT faction_id, COUNT(*)::bigint AS members FROM character_faction GROUP BY faction_id;

-- name: SumFactionMerit :many
-- Faction merit (§10, decision H): the world-countable points (same rule as world progress: min(cap, points − ×2
-- catch-up bonus), speed items included) per faction since @from_day. A day's points belong to the faction stamped on
-- its daily_contribution row (the faction at the day's first credit).
SELECT faction_id::text AS faction_id,
       COALESCE(SUM(LEAST(@cap_milli::bigint, points_milli - catch_up_bonus_milli)), 0)::bigint AS merit_milli
FROM daily_contribution
WHERE faction_id IS NOT NULL AND game_day >= @from_day::date
GROUP BY faction_id;

-- name: StampTodayFaction :exec
-- First join: today's contribution row (if any) now belongs to the new faction. Runs under the progression lock,
-- which every daily_contribution writer of this character holds first.
UPDATE daily_contribution SET faction_id = @faction_id
WHERE character_id = @character_id AND game_day = @game_day AND faction_id IS NULL;

-- name: ListFactionProgressionsPage :many
-- Keyset page for the per-faction realm ZSET rebuild.
SELECT cf.character_id, cf.faction_id, p.stage_id, p.cultivation_milli
FROM character_faction cf JOIN character_progressions p ON p.character_id = cf.character_id
WHERE cf.character_id > @after
ORDER BY cf.character_id
LIMIT @lim;

-- name: GetFactionProgressionsByIDs :many
SELECT p.character_id, cf.faction_id, p.stage_id, p.cultivation_milli
FROM character_progressions p LEFT JOIN character_faction cf ON cf.character_id = p.character_id
WHERE p.character_id = ANY(@ids::uuid[]);
