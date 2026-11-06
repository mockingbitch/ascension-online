package cultivation

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"tiendao/game/internal/db"
	"tiendao/game/internal/economy"
	"tiendao/game/internal/gameday"
	"tiendao/game/internal/item"
	"tiendao/game/internal/platform/postgres"
)

var (
	ErrNotUsable      = errors.New("cultivation: item has no usable effect")
	ErrBonusActive    = errors.New("cultivation: an offline bonus is already pending")
	ErrBonusUsedToday = errors.New("cultivation: offline bonus item already used today")
	ErrNoItem         = errors.New("cultivation: item not in inventory")
)

// BonusStatus is returned after using an offline-bonus item.
type BonusStatus struct {
	ItemID              string `json:"itemId"`
	Remaining           int64  `json:"remaining"`
	OfflineBonusSeconds int32  `json:"offlineBonusSeconds"`
	Replayed            bool   `json:"replayed"`
}

// UseOfflineBonusItem consumes one offline-bonus item (e.g. Tụ Linh Phù) and stores a pending bonus.
// Limits: one use per game day, no stacking. The bonus is spent by the first settlement it extends.
func (s *Service) UseOfflineBonusItem(ctx context.Context, items *item.Catalogue, userID uuid.UUID, itemID, idemKey string) (BonusStatus, error) {
	def, ok := items.Get(itemID)
	if !ok || def.OfflineCapBonusHours <= 0 {
		return BonusStatus{}, ErrNotUsable
	}
	var out BonusStatus
	err := postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		ch, err := q.GetCharacterByUser(ctx, userID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNoCharacter
		} else if err != nil {
			return err
		}
		// Lock order: progression → inventory.
		p, err := q.GetProgressionForUpdate(ctx, ch.ID)
		if err != nil {
			return fmt.Errorf("lock progression: %w", err)
		}
		inv, err := q.GetInventoryItemsForUpdate(ctx, db.GetInventoryItemsForUpdateParams{CharacterID: ch.ID, ItemIds: []string{itemID}})
		if err != nil {
			return err
		}
		have := int64(0)
		if len(inv) == 1 {
			have = inv[0].Quantity
		}
		if _, err := q.GetInventoryTransactionByKey(ctx, db.GetInventoryTransactionByKeyParams{CharacterID: ch.ID, ItemID: itemID, IdempotencyKey: idemKey}); err == nil {
			out = BonusStatus{ItemID: itemID, Remaining: have, OfflineBonusSeconds: p.OfflineBonusSeconds, Replayed: true}
			return nil
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		now := s.Now()
		switch {
		case gameday.Same(p.OfflineBonusUsedDay, now):
			return ErrBonusUsedToday
		case p.OfflineBonusSeconds > 0:
			return ErrBonusActive
		case have < 1:
			return ErrNoItem
		}
		left, err := economy.ConsumeItem(ctx, q, ch.ID, itemID, 1, economy.Ref{Reason: "item_use", Type: "offline_bonus", ID: itemID, IdemKey: idemKey})
		if err != nil {
			return err
		}
		bonus := int32(time.Duration(def.OfflineCapBonusHours) * time.Hour / time.Second)
		if err := q.SetOfflineBonus(ctx, db.SetOfflineBonusParams{CharacterID: ch.ID, OfflineBonusSeconds: bonus, OfflineBonusUsedDay: gameday.PG(now)}); err != nil {
			return err
		}
		out = BonusStatus{ItemID: itemID, Remaining: left, OfflineBonusSeconds: bonus}
		return nil
	})
	return out, err
}
