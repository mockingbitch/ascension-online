-- name: GetCurrencyAccountForUpdate :one
SELECT * FROM currency_accounts WHERE character_id = $1 AND currency = $2 FOR UPDATE;

-- name: AddCurrency :one
UPDATE currency_accounts SET balance = balance + @delta, version = version + 1
WHERE character_id = @character_id AND currency = @currency AND balance + @delta >= 0
RETURNING balance;

-- name: GetCurrencyLedgerByKey :one
SELECT * FROM currency_ledger WHERE character_id = $1 AND currency = $2 AND idempotency_key = $3;

-- name: InsertCurrencyLedger :exec
INSERT INTO currency_ledger (character_id, currency, delta, balance_after, reason, ref_type, ref_id, idempotency_key)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: ListCurrencyBalances :many
SELECT currency, balance FROM currency_accounts WHERE character_id = $1 ORDER BY currency;

-- name: ListInventory :many
SELECT item_id, quantity FROM inventory_items WHERE character_id = $1 AND quantity > 0 ORDER BY item_id;
