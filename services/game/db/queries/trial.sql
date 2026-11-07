-- Trial runs (internal/trial). Every mutation runs under the character's progression lock.

-- name: InsertTrialRun :one
INSERT INTO trial_runs (character_id, kind, seed, params, issue_key, issued_at, expires_at)
VALUES (@character_id, @kind, @seed, @params, @issue_key, @issued_at, @expires_at)
RETURNING *;

-- name: GetTrialRunByIssueKey :one
SELECT * FROM trial_runs WHERE character_id = $1 AND issue_key = $2;

-- name: GetOpenTrialRun :one
SELECT * FROM trial_runs WHERE character_id = $1 AND kind = $2 AND verdict IS NULL;

-- name: GetTrialRunForUpdate :one
-- Scoped to the character: another character's run id is simply "not found".
SELECT * FROM trial_runs WHERE id = $1 AND character_id = $2 FOR UPDATE;

-- name: JudgeTrialRun :one
-- Writes the verdict exactly once (verdict IS NULL guard).
UPDATE trial_runs
SET submit_key = @submit_key, submitted_at = @submitted_at, claim = @claim, result = @result, verdict = @verdict,
    reason = @reason, verdict_at = @verdict_at, cooldown_until = @cooldown_until, pass_valid_until = @pass_valid_until
WHERE id = @id AND verdict IS NULL
RETURNING *;

-- name: GetTrialCooldown :one
SELECT cooldown_until FROM trial_runs
WHERE character_id = $1 AND kind = $2 AND cooldown_until IS NOT NULL
ORDER BY cooldown_until DESC LIMIT 1;

-- name: GetValidTrialPass :one
SELECT * FROM trial_runs
WHERE character_id = @character_id AND kind = @kind AND verdict = 'pass' AND consumed_at IS NULL
  AND pass_valid_until > @now
ORDER BY pass_valid_until DESC LIMIT 1;

-- name: GetValidTrialPassForUpdate :one
SELECT * FROM trial_runs
WHERE character_id = @character_id AND kind = @kind AND verdict = 'pass' AND consumed_at IS NULL
  AND pass_valid_until > @now
ORDER BY pass_valid_until DESC LIMIT 1
FOR UPDATE;

-- name: ConsumeTrialPass :execrows
UPDATE trial_runs SET consumed_at = @consumed_at, consumed_ref = @consumed_ref
WHERE id = @id AND verdict = 'pass' AND consumed_at IS NULL;
