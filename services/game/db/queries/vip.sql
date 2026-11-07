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

-- name: InsertVIPLedger :one
INSERT INTO vip_point_ledger (user_id, delta, points_after, reason, source, activity_id, game_day, reverses_ledger_id,
                              ref_type, ref_id, idempotency_key)
VALUES (@user_id, @delta, @points_after, @reason, @source, sqlc.narg('activity_id'), @game_day,
        sqlc.narg('reverses_ledger_id'), @ref_type, @ref_id, @idempotency_key)
RETURNING id;

-- name: GetVIPLedgerByID :one
SELECT * FROM vip_point_ledger WHERE id = $1;

-- name: GetVIPLedgerReversal :one
-- The refund row that reverses ledger row $1, if any (reverses_ledger_id is UNIQUE).
SELECT * FROM vip_point_ledger WHERE reverses_ledger_id = $1;

-- name: ListVIPEarnDay :many
-- In-game điểm duyên of one game day per activity (source not topup/admin): how many times and how many points.
-- Read under the vip_accounts row lock by EarnLocked; unlocked for GET /vip/status.
SELECT activity_id::text AS activity_id, COUNT(*)::bigint AS times, COALESCE(SUM(delta), 0)::bigint AS points
FROM vip_point_ledger
WHERE user_id = $1 AND game_day = $2 AND activity_id IS NOT NULL
GROUP BY activity_id;

-- name: InsertDailyGiftClaim :one
-- Returns no row when today's gift was already claimed.
INSERT INTO vip_daily_gift_claims (user_id, claim_day, tier, amount)
VALUES ($1, $2, $3, $4)
ON CONFLICT (user_id, claim_day) DO NOTHING
RETURNING claim_day;

-- name: GetDailyGiftClaim :one
SELECT * FROM vip_daily_gift_claims WHERE user_id = $1 AND claim_day = $2;

-- name: GetVIPPurchaseByOrder :one
-- The purchase row of a payment order (unique per order: vip_point_ledger_purchase_order_uq).
SELECT * FROM vip_point_ledger WHERE reason = 'purchase' AND ref_type = $1 AND ref_id = $2;

-- name: StripAppearancePerks :execrows
-- The Tiên Duyên level dropped (refund / admin revoke): unequip every perk cosmetic in @ids (perks of tiers above the
-- new level). Caller holds the vip_accounts lock; this locks character_appearance next (lock order vip → appearance).
UPDATE character_appearance a SET
    title_id = CASE WHEN a.title_id = ANY(@ids::text[]) THEN NULL ELSE a.title_id END,
    avatar_frame_id = CASE WHEN a.avatar_frame_id = ANY(@ids::text[]) THEN NULL ELSE a.avatar_frame_id END,
    chat_aura_id = CASE WHEN a.chat_aura_id = ANY(@ids::text[]) THEN NULL ELSE a.chat_aura_id END,
    cave_decoration_ids = ARRAY(SELECT x FROM unnest(a.cave_decoration_ids) WITH ORDINALITY AS t(x, n)
                                WHERE x <> ALL(@ids::text[]) ORDER BY n),
    version = a.version + 1, updated_at = @now
FROM characters c
WHERE c.id = a.character_id AND c.user_id = @user_id
  AND (a.title_id = ANY(@ids::text[]) OR a.avatar_frame_id = ANY(@ids::text[]) OR a.chat_aura_id = ANY(@ids::text[])
       OR a.cave_decoration_ids && @ids::text[]);
