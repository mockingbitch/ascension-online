package cultivation

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"tiendao/game/internal/contribution"
	"tiendao/game/internal/db"
	"tiendao/game/internal/economy"
	"tiendao/game/internal/gameday"
	"tiendao/game/internal/item"
	"tiendao/game/internal/platform/postgres"
)

var (
	// ErrItemDailyLimit: the item's max_use_per_game_day is used up for this character today.
	ErrItemDailyLimit = errors.New("cultivation: item daily use limit reached")
	// ErrNoRoom: the pill's full value doesn't fit today (speed-item cap, activity bucket or 150 ceiling) — kept.
	ErrNoRoom = errors.New("cultivation: no room left today for speed-item points")
	// ErrBottleneckFull: the bình cảnh room is smaller than the pill (break through first) — the pill is kept.
	ErrBottleneckFull = errors.New("cultivation: bình cảnh full")
)

// SpeedItemResult is POST /items/{itemId}/use for a speed item (instant_points_milli).
type SpeedItemResult struct {
	ItemID         string   `json:"itemId"`
	Remaining      int64    `json:"remaining"`
	UsesToday      int32    `json:"usesToday"`
	MaxUsesPerDay  int32    `json:"maxUsesPerDay"`
	RawMilli       int64    `json:"rawMilli"`
	EarnedMilli    int64    `json:"earnedMilli"` // after the speed-item cap, bucket, ceiling and bình cảnh
	StagesAdvanced []string `json:"stagesAdvanced"`
	Cultivation    Snapshot `json:"cultivation"`
	Replayed       bool     `json:"replayed"`
}

// UseSpeedItem consumes one speed item (e.g. Tụ Linh Đan) and credits its points (data-model §4.5, §12 F3):
// lock progression → idempotency (inventory ledger key) → lock inventory → settle first → conditional upsert
// item_daily_uses (max_use_per_game_day, counted per character per day of USE) → consume 1 → CreditLocked(source item),
// which also clips by speed_items.daily_cap_points, the activity bucket and the 150 ceiling. Unless the pill's FULL
// value is credited the whole transaction is rolled back (pill and daily use kept) with ErrNoRoom/ErrBottleneckFull.
func (s *Service) UseSpeedItem(ctx context.Context, items *item.Catalogue, userID uuid.UUID, itemID, idemKey string) (SpeedItemResult, error) {
	def, ok := items.Get(itemID)
	if !ok || def.InstantPointsMilli <= 0 || def.MaxUsePerGameDay < 1 {
		return SpeedItemResult{}, ErrNotUsable
	}
	var out SpeedItemResult
	err := postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		ch, err := q.GetCharacterByUser(ctx, userID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNoCharacter
		} else if err != nil {
			return err
		}
		p, err := q.GetProgressionForUpdate(ctx, ch.ID)
		if err != nil {
			return fmt.Errorf("lock progression: %w", err)
		}
		now := s.Now().UTC()
		if _, err := q.GetInventoryTransactionByKey(ctx, db.GetInventoryTransactionByKeyParams{CharacterID: ch.ID, ItemID: itemID, IdempotencyKey: idemKey}); err == nil {
			out, err = s.speedReplay(ctx, q, userID, ch.ID, p, def, idemKey)
			return err
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		// Lock order (§2.3): progression → daily_contribution (settle) → item_daily_uses → inventory.
		p, _, err = s.SettleLocked(ctx, q, userID, p, "item:"+idemKey)
		if err != nil {
			return err
		}
		uses, err := q.IncrementItemDailyUse(ctx, db.IncrementItemDailyUseParams{
			CharacterID: ch.ID, GameDay: gameday.PG(now), ItemID: itemID, MaxUses: int32(def.MaxUsePerGameDay),
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrItemDailyLimit
		} else if err != nil {
			return fmt.Errorf("item daily use: %w", err)
		}
		inv, err := q.GetInventoryItemsForUpdate(ctx, db.GetInventoryItemsForUpdateParams{CharacterID: ch.ID, ItemIds: []string{itemID}})
		if err != nil {
			return err
		}
		if len(inv) == 0 || inv[0].Quantity < 1 {
			return ErrNoItem
		}
		left, err := economy.ConsumeItem(ctx, q, ch.ID, itemID, 1, economy.Ref{Reason: "item_use", Type: "speed_item", ID: itemID, IdemKey: idemKey})
		if err != nil {
			return err
		}
		var credit CreditOutcome
		p, credit, err = s.CreditLocked(ctx, q, p, contribution.SourceItem, def.InstantPointsMilli, Ref{Type: "item", ID: itemID, IdemKey: idemKey}, now)
		if err != nil {
			return err
		}
		// All or nothing (anti-cheat M1): a pill is consumed only when its full value is credited; any clip by the
		// speed cap, activity bucket, 150 ceiling or bình cảnh rolls the whole transaction back (pill + use kept).
		if a := credit.Applied; a.CreditedMilli < def.InstantPointsMilli {
			if a.ClippedBottleneckMilli > 0 {
				return ErrBottleneckFull
			}
			return ErrNoRoom
		}
		snap, err := s.SnapshotLocked(ctx, q, userID, p, now)
		if err != nil {
			return err
		}
		out = SpeedItemResult{ItemID: itemID, Remaining: left, UsesToday: uses, MaxUsesPerDay: int32(def.MaxUsePerGameDay),
			RawMilli: def.InstantPointsMilli, EarnedMilli: credit.Applied.CreditedMilli, StagesAdvanced: []string{}, Cultivation: snap}
		for _, st := range credit.Passed {
			out.StagesAdvanced = append(out.StagesAdvanced, st.ID)
		}
		return nil
	})
	return out, err
}

func (s *Service) speedReplay(ctx context.Context, q *db.Queries, userID, charID uuid.UUID, p db.CharacterProgression, def *item.Def, idemKey string) (SpeedItemResult, error) {
	now := s.Now().UTC()
	out := SpeedItemResult{ItemID: def.ID, MaxUsesPerDay: int32(def.MaxUsePerGameDay), RawMilli: def.InstantPointsMilli, StagesAdvanced: []string{}, Replayed: true}
	if c, err := q.GetCultivationCreditByKey(ctx, db.GetCultivationCreditByKeyParams{CharacterID: charID, IdempotencyKey: string(contribution.SourceItem) + ":" + idemKey}); err == nil {
		out.EarnedMilli = c.CreditedMilli
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return out, err
	}
	inv, err := q.GetInventoryItems(ctx, db.GetInventoryItemsParams{CharacterID: charID, ItemIds: []string{def.ID}})
	if err != nil {
		return out, err
	}
	if len(inv) == 1 {
		out.Remaining = inv[0].Quantity
	}
	if out.UsesToday, err = q.GetItemDailyUses(ctx, db.GetItemDailyUsesParams{CharacterID: charID, GameDay: gameday.PG(now), ItemID: def.ID}); err != nil {
		return out, err
	}
	out.Cultivation, err = s.SnapshotLocked(ctx, q, userID, p, now)
	return out, err
}
