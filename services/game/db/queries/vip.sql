-- name: GetVIPPoints :one
-- Returns 0 when the user has no VIP account yet.
SELECT COALESCE((SELECT points FROM vip_accounts WHERE user_id = $1), 0)::bigint AS points;

-- name: EnsureVIPAccount :exec
INSERT INTO vip_accounts (user_id) VALUES ($1)
ON CONFLICT (user_id) DO NOTHING;

-- name: GetVIPAccountForUpdate :one
SELECT * FROM vip_accounts WHERE user_id = $1 FOR UPDATE;

-- name: GetVIPLedgerByKey :one
SELECT * FROM vip_point_ledger WHERE user_id = $1 AND idempotency_key = $2;

-- name: AddVIPPoints :one
UPDATE vip_accounts
SET points = points + @delta, version = version + 1, updated_at = now()
WHERE user_id = @user_id
RETURNING points;

-- name: InsertVIPLedger :exec
INSERT INTO vip_point_ledger (user_id, delta, points_after, reason, ref_type, ref_id, idempotency_key)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: InsertDailyGiftClaim :one
-- Returns no row when today's gift was already claimed.
INSERT INTO vip_daily_gift_claims (user_id, claim_day, tier, amount)
VALUES ($1, $2, $3, $4)
ON CONFLICT (user_id, claim_day) DO NOTHING
RETURNING claim_day;

-- name: GetDailyGiftClaim :one
SELECT * FROM vip_daily_gift_claims WHERE user_id = $1 AND claim_day = $2;
