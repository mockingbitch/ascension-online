-- name: GetFactionDonationByKey :one
SELECT * FROM faction_donations WHERE character_id = $1 AND idempotency_key = $2;

-- name: GetFactionDonationDay :one
SELECT COALESCE((SELECT amount FROM faction_donation_days WHERE character_id = $1 AND game_day = $2), 0)::BIGINT AS amount;

-- name: IncrementFactionDonationDay :one
-- Conditional upsert: returns no row when the day's total would exceed @cap.
INSERT INTO faction_donation_days (character_id, game_day, amount)
SELECT @character_id, @game_day, @amount::bigint WHERE @amount::bigint <= @cap::bigint
ON CONFLICT (character_id, game_day) DO UPDATE SET amount = faction_donation_days.amount + EXCLUDED.amount
WHERE faction_donation_days.amount + EXCLUDED.amount <= @cap::bigint
RETURNING amount;

-- name: AddFactionDonor :one
INSERT INTO faction_donors (faction_id, character_id, total, updated_at) VALUES (@faction_id, @character_id, @amount::bigint, @now)
ON CONFLICT (faction_id, character_id) DO UPDATE
SET total = faction_donors.total + EXCLUDED.total, updated_at = EXCLUDED.updated_at
RETURNING total;

-- name: RecordFactionDonation :one
-- The LAST statement of the donation tx (anti-cheat M3): the shared treasury row is locked here and released at the
-- commit right after, so concurrent donors to one faction serialize only on this one statement.
WITH t AS (
    INSERT INTO faction_treasuries (faction_id, donated_total, version, updated_at) VALUES (@faction_id, @amount::bigint, 1, @now)
    ON CONFLICT (faction_id) DO UPDATE
    SET donated_total = faction_treasuries.donated_total + EXCLUDED.donated_total, version = faction_treasuries.version + 1,
        updated_at = EXCLUDED.updated_at
    RETURNING donated_total
)
INSERT INTO faction_donations (character_id, faction_id, amount, total_after, game_day, idempotency_key, created_at)
SELECT @character_id, @faction_id, @amount::bigint, t.donated_total, @game_day, @idempotency_key, @now FROM t
RETURNING id, total_after;

-- name: ListFactionTreasuries :many
SELECT faction_id, donated_total FROM faction_treasuries ORDER BY faction_id;

-- name: GetFactionTreasury :one
SELECT COALESCE((SELECT donated_total FROM faction_treasuries WHERE faction_id = $1), 0)::BIGINT AS donated_total;

-- name: ListFactionDonors :many
SELECT d.character_id, c.name, d.total
FROM faction_donors d JOIN characters c ON c.id = d.character_id
WHERE d.faction_id = @faction_id
ORDER BY d.total DESC, d.character_id
LIMIT @lim;

-- name: GetFactionDonorTotal :one
SELECT COALESCE((SELECT total FROM faction_donors WHERE faction_id = $1 AND character_id = $2), 0)::BIGINT AS total;

-- name: MoveFactionTreasury :exec
-- migrate_faction (a rename, decisions 2026-11-07 L3): the pool follows the faction to its new id, once, in the worker
-- step that finishes the move. A split never calls this: the source pool stays as archived history.
WITH src AS (DELETE FROM faction_treasuries AS old WHERE old.faction_id = @from_faction RETURNING old.donated_total)
INSERT INTO faction_treasuries (faction_id, donated_total, version, updated_at)
SELECT @to_faction, donated_total, 1, @now FROM src
ON CONFLICT (faction_id) DO UPDATE
SET donated_total = faction_treasuries.donated_total + EXCLUDED.donated_total, version = faction_treasuries.version + 1,
    updated_at = EXCLUDED.updated_at;

-- name: MoveFactionDonors :exec
WITH src AS (DELETE FROM faction_donors AS old WHERE old.faction_id = @from_faction RETURNING old.character_id, old.total)
INSERT INTO faction_donors (faction_id, character_id, total, updated_at)
SELECT @to_faction, character_id, total, @now FROM src
ON CONFLICT (faction_id, character_id) DO UPDATE
SET total = faction_donors.total + EXCLUDED.total, updated_at = EXCLUDED.updated_at;
