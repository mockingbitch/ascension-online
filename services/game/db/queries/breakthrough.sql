-- name: GetBreakthroughAttemptByKey :one
SELECT * FROM breakthrough_attempts WHERE character_id = $1 AND idempotency_key = $2;

-- name: InsertBreakthroughAttempt :one
INSERT INTO breakthrough_attempts (character_id, idempotency_key, from_stage_id, to_stage_id, kind, success_bp, roll_bp,
                                   success, consumed, tribulation, guaranteed, cultivation_before_milli, loss_milli, odds)
VALUES (@character_id, @idempotency_key, @from_stage_id, @to_stage_id, @kind, @success_bp, @roll_bp,
        @success, @consumed, @tribulation, @guaranteed, @cultivation_before_milli, @loss_milli, @odds)
RETURNING id;

-- name: AdvanceStage :exec
-- Successful major breakthrough: new realm, carry-over (already auto-advanced), reserve after release.
UPDATE character_progressions
SET realm_id = @realm_id, stage_id = @stage_id, cultivation_milli = @cultivation_milli,
    grant_reserve_milli = @grant_reserve_milli,
    breakthrough_fail_streak = 0, breakthrough_cooldown_until = NULL,
    stage_reached_at = @stage_reached_at, version = version + 1, updated_at = now()
WHERE character_id = @character_id;

-- name: RecordBreakthroughFailure :exec
UPDATE character_progressions
SET breakthrough_fail_streak = breakthrough_fail_streak + 1,
    breakthrough_cooldown_until = $2, version = version + 1, updated_at = now()
WHERE character_id = $1;

-- name: ApplyTribulationFailure :exec
-- Tribulation failure: lose loss_milli of the CURRENT stage's stored cultivation (never drops a stage).
UPDATE character_progressions
SET cultivation_milli = cultivation_milli - @loss_milli,
    breakthrough_fail_streak = breakthrough_fail_streak + 1,
    breakthrough_cooldown_until = @cooldown_until, version = version + 1, updated_at = now()
WHERE character_id = @character_id AND cultivation_milli >= @loss_milli;

-- name: InsertProgressionEvent :exec
INSERT INTO progression_events (character_id, kind, stage_id, payload, occurred_at)
VALUES ($1, $2, $3, $4, $5);
