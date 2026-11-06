-- name: GetWorldState :one
SELECT * FROM world_state WHERE id = 1;

-- name: GetWorldStateForUpdate :one
SELECT * FROM world_state WHERE id = 1 FOR UPDATE;

-- name: StartWorld :one
-- Idempotent: only the first start sets started_at.
UPDATE world_state SET started_at = COALESCE(started_at, @started_at), version = version + 1, updated_at = now()
WHERE id = 1
RETURNING *;

-- name: FoldWorldProgress :exec
UPDATE world_state
SET progress_milli        = @progress_milli,
    closed_through_day    = @closed_through_day,
    median_position_milli = @median_position_milli,
    median_game_day       = @median_game_day,
    version               = version + 1,
    updated_at            = now()
WHERE id = 1;

-- name: UpsertWorldDailyTotal :exec
-- Recompute (never increment) an OPEN game day from daily_contribution; closed days never change (W5).
INSERT INTO world_daily_totals (game_day, credited_milli, catch_up_bonus_milli, countable_milli, dau,
                                per_capita_milli, computed_at)
SELECT @game_day::date,
       COALESCE(SUM(points_milli), 0)::bigint,
       COALESCE(SUM(catch_up_bonus_milli), 0)::bigint,
       COALESCE(SUM(LEAST(@cap_milli::bigint, points_milli - catch_up_bonus_milli)), 0)::bigint,
       COUNT(*)::int,
       (COALESCE(SUM(LEAST(@cap_milli::bigint, points_milli - catch_up_bonus_milli)), 0) / GREATEST(COUNT(*), 1))::bigint,
       @computed_at::timestamptz
FROM daily_contribution
WHERE game_day = @game_day::date AND points_milli > 0
ON CONFLICT (game_day) DO UPDATE
SET credited_milli       = EXCLUDED.credited_milli,
    catch_up_bonus_milli = EXCLUDED.catch_up_bonus_milli,
    countable_milli      = EXCLUDED.countable_milli,
    dau                  = EXCLUDED.dau,
    per_capita_milli     = EXCLUDED.per_capita_milli,
    computed_at          = EXCLUDED.computed_at
WHERE world_daily_totals.closed_at IS NULL;

-- name: GetWorldDailyTotal :one
SELECT * FROM world_daily_totals WHERE game_day = $1;

-- name: CloseWorldDay :exec
UPDATE world_daily_totals SET median_position_milli = @median_position_milli, closed_at = @closed_at
WHERE game_day = @game_day AND closed_at IS NULL;

-- name: ListClosedDaysAfter :many
SELECT * FROM world_daily_totals
WHERE closed_at IS NOT NULL AND (sqlc.narg(after)::date IS NULL OR game_day > sqlc.narg(after)::date)
ORDER BY game_day;


-- name: AuditCreditsForDay :many
-- Invariant C3 for a whole game day: characters whose daily_contribution.points_milli differs from the sum of their
-- bucketed cultivation_credits (grant releases excluded). Must return no rows.
SELECT dc.character_id, dc.points_milli::bigint AS points_milli, COALESCE(c.credited, 0)::bigint AS credited_milli
FROM daily_contribution dc
LEFT JOIN (SELECT character_id, SUM(credited_milli) AS credited
           FROM cultivation_credits
           WHERE cultivation_credits.game_day = @game_day::date AND source <> 'late_join_grant'
           GROUP BY character_id) c ON c.character_id = dc.character_id
WHERE dc.game_day = @game_day::date AND dc.points_milli <> COALESCE(c.credited, 0);
