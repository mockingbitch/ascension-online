-- name: ListTutorialProgress :many
SELECT step_id, status FROM tutorial_progress WHERE character_id = $1 ORDER BY created_at, step_id;

-- name: GetTutorialProgressByKey :one
SELECT step_id, status FROM tutorial_progress WHERE character_id = $1 AND idempotency_key = $2;

-- name: InsertTutorialProgress :execrows
-- First write wins: a step seen on one device stays seen (done or skipped) — a later mark is a no-op.
INSERT INTO tutorial_progress (character_id, step_id, status, idempotency_key, created_at)
VALUES (@character_id, @step_id, @status, @idempotency_key, @now)
ON CONFLICT (character_id, step_id) DO NOTHING;

-- name: DeleteTutorialProgress :exec
DELETE FROM tutorial_progress WHERE character_id = $1;

-- name: GetTutorialMuted :one
SELECT muted FROM tutorial_prefs WHERE character_id = $1;

-- name: EnsureTutorialPrefs :exec
INSERT INTO tutorial_prefs (character_id) VALUES ($1) ON CONFLICT DO NOTHING;

-- name: LockTutorialPrefs :one
-- Per-character lock of every tutorial write (two devices / a double tap serialise here).
SELECT muted, last_key FROM tutorial_prefs WHERE character_id = $1 FOR UPDATE;

-- name: SetTutorialMuted :exec
UPDATE tutorial_prefs SET muted = @muted, last_key = sqlc.narg(last_key), version = version + 1, updated_at = @now
WHERE character_id = @character_id;
