-- name: GetProgressionForUpdate :one
SELECT * FROM character_progressions
WHERE character_id = $1
FOR UPDATE;

-- name: GetProgression :one
SELECT * FROM character_progressions
WHERE character_id = $1;

-- name: UpdateProgressionCultivation :exec
-- Written by the single credit path: stage may change through auto-advance of minor stages.
UPDATE character_progressions
SET realm_id            = @realm_id,
    stage_id            = @stage_id,
    cultivation_milli   = @cultivation_milli,
    rate_carry          = @rate_carry,
    grant_reserve_milli = @grant_reserve_milli,
    last_cultivated_at  = @last_cultivated_at,
    stage_reached_at    = @stage_reached_at,
    version             = version + 1,
    updated_at          = now()
WHERE character_id = @character_id;

-- name: GetSettlementByKey :one
SELECT * FROM cultivation_settlements
WHERE character_id = $1 AND idempotency_key = $2;

-- name: InsertSettlement :one
INSERT INTO cultivation_settlements (
    character_id, idempotency_key, stage_id, elapsed_seconds, eligible_seconds,
    rate_milli_per_hour, offline_cap_seconds, raw_milli, multiplier_bp, credited_milli,
    cultivation_after_milli, game_day, settled_at
) VALUES (@character_id, @idempotency_key, @stage_id, @elapsed_seconds, @eligible_seconds,
    @rate_milli_per_hour, @offline_cap_seconds, @raw_milli, @multiplier_bp, @credited_milli,
    @cultivation_after_milli, @game_day, @settled_at)
RETURNING id;

-- name: SetOfflineBonus :exec
UPDATE character_progressions
SET offline_bonus_seconds = $2, offline_bonus_used_day = $3, version = version + 1, updated_at = now()
WHERE character_id = $1;

-- name: ClearOfflineBonus :exec
UPDATE character_progressions SET offline_bonus_seconds = 0, version = version + 1, updated_at = now()
WHERE character_id = $1;

-- name: ListActivePositionsPage :many
-- Keyset page of characters with credited points in [@from_day, @to_day] (server median at day close).
-- Uses daily_contribution's PK (character_id, game_day).
SELECT p.character_id, p.stage_id, p.cultivation_milli
FROM character_progressions p
WHERE p.character_id > @after
  AND EXISTS (SELECT 1 FROM daily_contribution dc
              WHERE dc.character_id = p.character_id AND dc.game_day BETWEEN @from_day AND @to_day
                AND dc.points_milli > 0)
ORDER BY p.character_id
LIMIT @lim;
