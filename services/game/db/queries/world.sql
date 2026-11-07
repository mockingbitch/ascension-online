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

-- ───────────── Milestones (Phase 2.1, §5.4–5.6) ─────────────

-- name: SetWorldEra :exec
UPDATE world_state SET era_id = @era_id, era_started_at = @era_started_at, version = version + 1, updated_at = now()
WHERE id = 1;

-- name: SetWorldLastMilestone :exec
UPDATE world_state SET last_milestone_id = @milestone_id, version = version + 1, updated_at = now() WHERE id = 1;

-- name: SetWorldPaused :one
UPDATE world_state SET paused_at = sqlc.narg(paused_at), version = version + 1, updated_at = now() WHERE id = 1
RETURNING *;

-- name: ListRecentWorldTotals :many
SELECT * FROM world_daily_totals ORDER BY game_day DESC LIMIT @lim;

-- name: InsertMilestoneReached :one
-- W1: idempotent. pgx.ErrNoRows = already reached (or the seq is taken) → do nothing.
INSERT INTO world_milestones_reached (milestone_id, seq, reached_at, reason, server_day, progress_milli,
                                      threshold_milli, target_day, admin_user_id)
VALUES (@milestone_id, @seq, @reached_at, @reason, @server_day, @progress_milli, @threshold_milli, @target_day,
        sqlc.narg(admin_user_id))
ON CONFLICT DO NOTHING
RETURNING milestone_id;

-- name: ListMilestonesReached :many
SELECT * FROM world_milestones_reached ORDER BY seq;

-- name: InsertWorldEffect :exec
INSERT INTO world_effects (milestone_id, effect_index, kind, due_at, deadline_at, applied_at, snapshot)
VALUES (@milestone_id, @effect_index, @kind, @due_at, sqlc.narg(deadline_at), sqlc.narg(applied_at), @snapshot)
ON CONFLICT (milestone_id, effect_index) DO NOTHING;

-- name: ListPendingWorldEffects :many
SELECT * FROM world_effects WHERE applied_at IS NULL OR (kind = 'split_faction' AND finalized_at IS NULL)
ORDER BY due_at, id LIMIT @lim;

-- name: InsertWorldEvent :exec
INSERT INTO world_events (event_id, milestone_id, started_at, ends_at)
VALUES (@event_id, @milestone_id, @started_at, @ends_at)
ON CONFLICT (event_id) DO NOTHING;

-- name: ListDueWorldEvents :many
-- Events whose duration has passed and that are not ended yet (bounded per tick).
SELECT * FROM world_events WHERE ended_at IS NULL AND ends_at <= @now ORDER BY ends_at, event_id LIMIT @lim;

-- name: EndWorldEvent :execrows
UPDATE world_events SET ended_at = @ended_at WHERE event_id = @event_id AND ended_at IS NULL;

-- name: ListOpenWorldEvents :many
SELECT * FROM world_events WHERE ended_at IS NULL ORDER BY started_at, event_id;

-- name: GetMilestoneOverride :one
SELECT * FROM world_milestone_overrides WHERE milestone_id = $1;

-- name: ListMilestoneOverrides :many
SELECT * FROM world_milestone_overrides ORDER BY milestone_id;

-- name: UpsertMilestoneOverride :one
INSERT INTO world_milestone_overrides (milestone_id, threshold_points_per_active, target_day, force_reach, paused,
                                       updated_by, updated_at)
VALUES (@milestone_id, sqlc.narg(threshold_points_per_active), sqlc.narg(target_day), @force_reach, @paused,
        @updated_by, @updated_at)
ON CONFLICT (milestone_id) DO UPDATE
SET threshold_points_per_active = EXCLUDED.threshold_points_per_active,
    target_day                  = EXCLUDED.target_day,
    force_reach                 = EXCLUDED.force_reach,
    paused                      = EXCLUDED.paused,
    updated_by                  = EXCLUDED.updated_by,
    updated_at                  = EXCLUDED.updated_at
RETURNING *;

-- ───────────── Faction effects (Phase 2.2, §5.6 step 5) ─────────────

-- name: ClaimFactionEffect :one
-- Next faction effect with work to do at @now (single runner under the world advisory lock; SKIP LOCKED anyway):
-- notice not announced yet, due and not applied, or a split past its deadline and not finalized.
SELECT * FROM world_effects
WHERE kind IN ('split_faction', 'migrate_faction')
  AND (noticed_at IS NULL
       OR (applied_at IS NULL AND due_at <= @now)
       OR (kind = 'split_faction' AND applied_at IS NOT NULL AND finalized_at IS NULL AND deadline_at <= @now))
ORDER BY id
LIMIT 1
FOR UPDATE SKIP LOCKED;

-- name: GetWorldEffect :one
SELECT * FROM world_effects WHERE id = $1;

-- name: MarkWorldEffectNoticed :exec
UPDATE world_effects SET noticed_at = @noticed_at WHERE id = @id AND noticed_at IS NULL;

-- name: MarkWorldEffectApplied :exec
-- deadline_at only ever moves later: a split applied late still gets its full choose window (H1).
UPDATE world_effects
SET applied_at = @applied_at,
    deadline_at = CASE WHEN deadline_at IS NULL THEN NULL ELSE GREATEST(deadline_at, sqlc.narg(min_deadline_at)) END
WHERE id = @id AND applied_at IS NULL;

-- name: RescheduleWorldEffect :exec
-- A notice announced late (outage, failing tick): the full notice period starts now (H1). Only later, never earlier.
UPDATE world_effects
SET due_at = GREATEST(due_at, @due_at),
    deadline_at = CASE WHEN deadline_at IS NULL THEN NULL ELSE GREATEST(deadline_at, sqlc.narg(deadline_at)) END
WHERE id = @id AND applied_at IS NULL;

-- name: GetWorldStateForShare :one
-- Join/switch read the era under FOR SHARE so the world tick can't change it mid-transaction (L2).
SELECT * FROM world_state WHERE id = 1 FOR SHARE;

-- name: MarkWorldEffectFinalized :exec
UPDATE world_effects SET finalized_at = @finalized_at WHERE id = @id AND finalized_at IS NULL;
