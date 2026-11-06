-- name: GetDailyContribution :one
SELECT * FROM daily_contribution WHERE character_id = $1 AND game_day = $2;

-- name: EnsureDailyContribution :exec
-- Call under the progression lock before GetDailyContributionForUpdate.
INSERT INTO daily_contribution (character_id, game_day) VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: GetDailyContributionForUpdate :one
SELECT * FROM daily_contribution WHERE character_id = $1 AND game_day = $2 FOR UPDATE;

-- name: AddDailyContribution :exec
UPDATE daily_contribution
SET idle_milli           = idle_milli + @idle_milli,
    activity_milli       = activity_milli + @activity_milli,
    community_milli      = community_milli + @community_milli,
    speed_item_milli     = speed_item_milli + @speed_item_milli,
    catch_up_bonus_milli = catch_up_bonus_milli + @catch_up_bonus_milli,
    updated_at           = now()
WHERE character_id = @character_id AND game_day = @game_day;

-- name: InsertCultivationCredit :one
INSERT INTO cultivation_credits (
    character_id, game_day, source, bucket, stage_id, raw_milli, multiplier_bp, credited_milli,
    catch_up_bonus_milli, clipped_ceiling_milli, clipped_bottleneck_milli, ref_type, ref_id, idempotency_key
) VALUES (@character_id, @game_day, @source, @bucket, @stage_id, @raw_milli, @multiplier_bp, @credited_milli,
    @catch_up_bonus_milli, @clipped_ceiling_milli, @clipped_bottleneck_milli, @ref_type, @ref_id, @idempotency_key)
RETURNING id;

-- name: GetCultivationCreditByKey :one
SELECT * FROM cultivation_credits WHERE character_id = $1 AND idempotency_key = $2;

-- name: SumCreditsForDay :one
-- Invariant C3: equals daily_contribution.points_milli (grant releases excluded).
SELECT COALESCE(SUM(credited_milli), 0)::bigint AS credited_milli,
       COALESCE(SUM(catch_up_bonus_milli), 0)::bigint AS catch_up_bonus_milli
FROM cultivation_credits
WHERE character_id = $1 AND game_day = $2 AND source <> 'late_join_grant';

-- name: ListContributionOverCeiling :many
-- Audit (C1): rows above the ceiling must never exist.
SELECT character_id, game_day, points_milli FROM daily_contribution
WHERE game_day = @game_day AND points_milli > @ceiling_milli;

-- name: IncrementItemDailyUse :one
-- Conditional upsert: returns no row when the per-day limit is already reached.
INSERT INTO item_daily_uses (character_id, game_day, item_id, uses) VALUES (@character_id, @game_day, @item_id, 1)
ON CONFLICT (character_id, game_day, item_id) DO UPDATE SET uses = item_daily_uses.uses + 1
WHERE item_daily_uses.uses < @max_uses
RETURNING uses;

-- name: InsertLateJoinGrant :one
INSERT INTO late_join_grants (user_id, character_id, server_day, median_position_milli, median_game_day, grant_milli)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (user_id) DO NOTHING
RETURNING grant_milli;
