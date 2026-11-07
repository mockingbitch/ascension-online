-- Secret realm runs (internal/secretrealm). Every mutation runs under the character's progression lock.

-- name: InsertSecretRealmRun :one
INSERT INTO secret_realm_runs (character_id, realm_id, seed, params, issue_key, issued_at, expires_at, game_day)
VALUES (@character_id, @realm_id, @seed, @params, @issue_key, @issued_at, @expires_at, @game_day)
RETURNING *;

-- name: GetSecretRealmRunByIssueKey :one
SELECT * FROM secret_realm_runs WHERE character_id = $1 AND issue_key = $2;

-- name: GetOpenSecretRealmRun :one
SELECT * FROM secret_realm_runs WHERE character_id = $1 AND realm_id = $2 AND verdict IS NULL;

-- name: GetSecretRealmRunForUpdate :one
-- Scoped to the character: another character's run id is simply "not found".
SELECT * FROM secret_realm_runs WHERE id = $1 AND character_id = $2 FOR UPDATE;

-- name: JudgeSecretRealmRun :one
-- Writes the verdict exactly once (verdict IS NULL guard).
UPDATE secret_realm_runs
SET submit_key = @submit_key, submitted_at = @submitted_at, claim = @claim, result = @result, verdict = @verdict,
    reason = @reason, verdict_at = @verdict_at, band = @band, loot = @loot
WHERE id = @id AND verdict IS NULL
RETURNING *;

-- name: ListRecentSecretRealmRuns :many
SELECT id, realm_id, issued_at, verdict, reason, band, loot, result FROM secret_realm_runs
WHERE character_id = $1 AND verdict IS NOT NULL
ORDER BY issued_at DESC LIMIT $2;

-- name: LockSecretRealmDay :one
-- Ensure + lock the (character, day) counter row (totals across every realm).
INSERT INTO secret_realm_days (character_id, game_day) VALUES (@character_id, @game_day)
ON CONFLICT (character_id, game_day) DO UPDATE SET runs = secret_realm_days.runs
RETURNING *;

-- name: GetSecretRealmDay :one
SELECT * FROM secret_realm_days WHERE character_id = $1 AND game_day = $2;

-- name: AddSecretRealmRun :exec
UPDATE secret_realm_days SET runs = runs + 1 WHERE character_id = $1 AND game_day = $2;

-- name: AddSecretRealmRewards :exec
UPDATE secret_realm_days
SET spirit_stone = spirit_stone + @spirit_stone, materials = materials + @materials, artifacts = artifacts + @artifacts
WHERE character_id = @character_id AND game_day = @game_day;

-- name: GetSecretRealmRun :one
SELECT * FROM secret_realm_runs WHERE id = $1 AND character_id = $2;

-- name: OwnedItems :many
SELECT item_id FROM inventory_items WHERE character_id = $1 AND item_id = ANY(@item_ids::text[]) AND quantity > 0;
