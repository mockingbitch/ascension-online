// Package inventory holds item-ownership mutations outside gameplay flows (admin grants, later rewards).
package inventory

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"tiendao/game/internal/db"
	"tiendao/game/internal/platform/postgres"
)

var errReplay = errors.New("inventory: idempotent replay")

// AdminGrant adds items to a character, writing the inventory ledger and an admin audit row in one tx.
// Re-sending the same idempotency key returns the current quantity without granting again.
func AdminGrant(ctx context.Context, pool *pgxpool.Pool, adminID, characterID uuid.UUID, itemID string, qty int64, note, idemKey string) (int64, error) {
	var after int64
	err := postgres.InTx(ctx, pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		var err error
		after, err = q.GrantInventory(ctx, db.GrantInventoryParams{CharacterID: characterID, ItemID: itemID, Qty: qty})
		if err != nil {
			return err
		}
		if err := q.InsertInventoryTransaction(ctx, db.InsertInventoryTransactionParams{
			CharacterID: characterID, ItemID: itemID, Delta: qty, QuantityAfter: after, Reason: "admin_grant",
			RefType: pgtype.Text{String: "admin", Valid: true}, RefID: pgtype.Text{String: adminID.String(), Valid: true}, IdempotencyKey: idemKey,
		}); err != nil {
			var pe *pgconn.PgError
			if errors.As(err, &pe) && pe.Code == "23505" {
				return errReplay // rolls back the duplicate grant
			}
			return err
		}
		afterJ, _ := json.Marshal(map[string]any{"itemId": itemID, "quantity": after, "delta": qty})
		return q.InsertAdminAudit(ctx, db.InsertAdminAuditParams{
			AdminUserID: adminID, Action: "inventory.grant", TargetType: "character", TargetID: characterID.String(),
			After: afterJ, Reason: pgtype.Text{String: note, Valid: note != ""},
		})
	})
	if errors.Is(err, errReplay) {
		rows, err := db.New(pool).GetInventoryItems(ctx, db.GetInventoryItemsParams{CharacterID: characterID, ItemIds: []string{itemID}})
		if err != nil || len(rows) == 0 {
			return 0, err
		}
		return rows[0].Quantity, nil
	}
	return after, err
}
