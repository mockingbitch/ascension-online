-- name: GetInventoryItems :many
SELECT item_id, quantity FROM inventory_items
WHERE character_id = $1 AND item_id = ANY(@item_ids::text[]);

-- name: GetInventoryItemsForUpdate :many
-- Lock order: always by item_id.
SELECT item_id, quantity FROM inventory_items
WHERE character_id = $1 AND item_id = ANY(@item_ids::text[])
ORDER BY item_id
FOR UPDATE;

-- name: ConsumeInventory :one
UPDATE inventory_items SET quantity = quantity - @qty
WHERE character_id = @character_id AND item_id = @item_id AND quantity >= @qty
RETURNING quantity;

-- name: GrantInventory :one
INSERT INTO inventory_items (character_id, item_id, quantity) VALUES (@character_id, @item_id, @qty)
ON CONFLICT (character_id, item_id) DO UPDATE SET quantity = inventory_items.quantity + EXCLUDED.quantity
RETURNING quantity;

-- name: InsertInventoryTransaction :exec
INSERT INTO inventory_transactions (character_id, item_id, delta, quantity_after, reason, ref_type, ref_id, idempotency_key)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: GetInventoryTransactionByKey :one
SELECT * FROM inventory_transactions WHERE character_id = $1 AND item_id = $2 AND idempotency_key = $3;
