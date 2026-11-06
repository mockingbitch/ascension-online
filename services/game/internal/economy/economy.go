// Package economy applies currency and item changes inside a caller's transaction, always with a ledger row.
// Callers own the transaction and the business-level idempotency; these helpers add per-row guarantees.
package economy

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"tiendao/game/internal/db"
)

// Currencies.
const (
	SpiritStone = "spirit_stone"
	Premium     = "premium"
)

var ErrInsufficient = errors.New("economy: insufficient balance")

// Ref describes why a change happened (ledger columns).
type Ref struct {
	Reason  string // settlement | daily_task | vip_daily_gift | breakthrough_cost | admin_grant ...
	Type    string
	ID      string
	IdemKey string // unique per (character, currency|item)
}

// Credit adds delta (may be negative) to a currency account and writes the ledger, idempotent per IdemKey.
// Returns the balance after the change (or the current balance on replay).
func Credit(ctx context.Context, q *db.Queries, charID uuid.UUID, currency string, delta int64, ref Ref) (int64, error) {
	if delta == 0 {
		return 0, errors.New("economy: zero delta")
	}
	if err := q.EnsureCurrencyAccount(ctx, db.EnsureCurrencyAccountParams{CharacterID: charID, Currency: currency}); err != nil {
		return 0, err
	}
	acc, err := q.GetCurrencyAccountForUpdate(ctx, db.GetCurrencyAccountForUpdateParams{CharacterID: charID, Currency: currency})
	if err != nil {
		return 0, err
	}
	if _, err := q.GetCurrencyLedgerByKey(ctx, db.GetCurrencyLedgerByKeyParams{CharacterID: charID, Currency: currency, IdempotencyKey: ref.IdemKey}); err == nil {
		return acc.Balance, nil // replay
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return 0, err
	}
	after, err := q.AddCurrency(ctx, db.AddCurrencyParams{CharacterID: charID, Currency: currency, Delta: delta})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrInsufficient
	} else if err != nil {
		return 0, err
	}
	if err := q.InsertCurrencyLedger(ctx, db.InsertCurrencyLedgerParams{
		CharacterID: charID, Currency: currency, Delta: delta, BalanceAfter: after,
		Reason: ref.Reason, RefType: text(ref.Type), RefID: text(ref.ID), IdempotencyKey: ref.IdemKey,
	}); err != nil {
		return 0, fmt.Errorf("currency ledger: %w", err)
	}
	return after, nil
}

// GrantItem adds items and writes the inventory ledger. The caller guarantees business idempotency
// (a duplicate IdemKey for the same item fails with a unique violation and aborts the tx).
func GrantItem(ctx context.Context, q *db.Queries, charID uuid.UUID, itemID string, qty int64, ref Ref) (int64, error) {
	after, err := q.GrantInventory(ctx, db.GrantInventoryParams{CharacterID: charID, ItemID: itemID, Qty: qty})
	if err != nil {
		return 0, err
	}
	return after, q.InsertInventoryTransaction(ctx, db.InsertInventoryTransactionParams{
		CharacterID: charID, ItemID: itemID, Delta: qty, QuantityAfter: after,
		Reason: ref.Reason, RefType: text(ref.Type), RefID: text(ref.ID), IdempotencyKey: ref.IdemKey,
	})
}

// ConsumeItem removes items (row must be locked by the caller if a read-check preceded it).
func ConsumeItem(ctx context.Context, q *db.Queries, charID uuid.UUID, itemID string, qty int64, ref Ref) (int64, error) {
	left, err := q.ConsumeInventory(ctx, db.ConsumeInventoryParams{CharacterID: charID, ItemID: itemID, Qty: qty})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrInsufficient
	} else if err != nil {
		return 0, err
	}
	return left, q.InsertInventoryTransaction(ctx, db.InsertInventoryTransactionParams{
		CharacterID: charID, ItemID: itemID, Delta: -qty, QuantityAfter: left,
		Reason: ref.Reason, RefType: text(ref.Type), RefID: text(ref.ID), IdempotencyKey: ref.IdemKey,
	})
}

func text(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }
