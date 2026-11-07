-- name: GetShopPurchaseByKey :one
SELECT * FROM shop_purchases WHERE character_id = $1 AND idempotency_key = $2;

-- name: IncrementShopDailyCount :one
-- Conditional upsert (S2): returns no row when lots would exceed @max_lots for the day.
INSERT INTO shop_daily_counts (character_id, game_day, listing_id, lots)
SELECT @character_id, @game_day, @listing_id, @lots::int WHERE @lots::int <= @max_lots::int
ON CONFLICT (character_id, game_day, listing_id) DO UPDATE SET lots = shop_daily_counts.lots + EXCLUDED.lots
WHERE shop_daily_counts.lots + EXCLUDED.lots <= @max_lots::int
RETURNING lots;

-- name: GetShopDailyCounts :many
SELECT listing_id, lots FROM shop_daily_counts WHERE character_id = $1 AND game_day = $2;

-- name: InsertShopPurchase :one
INSERT INTO shop_purchases (character_id, listing_id, item_id, lots, item_quantity, currency, unit_price, total_price,
    game_day, idempotency_key, created_at)
VALUES (@character_id, @listing_id, @item_id, @lots, @item_quantity, @currency, @unit_price, @total_price,
    @game_day, @idempotency_key, @created_at)
RETURNING id;

-- name: GetCurrencyBalance :one
SELECT COALESCE((SELECT balance FROM currency_accounts WHERE character_id = $1 AND currency = $2), 0)::BIGINT AS balance;
