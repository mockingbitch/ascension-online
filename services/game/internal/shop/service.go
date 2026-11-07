package shop

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"tiendao/game/internal/db"
	"tiendao/game/internal/economy"
	"tiendao/game/internal/gameday"
	"tiendao/game/internal/item"
	"tiendao/game/internal/platform/postgres"
	"tiendao/game/internal/vip"
)

// Ledger reason shared by the currency and inventory rows of a purchase.
const reasonPurchase = "shop_purchase"

// ListingView is one listing with today's state for the viewer (GET /shop).
type ListingView struct {
	Listing
	Name               string `json:"name"`
	Category           string `json:"category"`
	Rarity             string `json:"rarity"`
	BoughtTodayLots    int32  `json:"boughtTodayLots"`
	RemainingTodayLots int32  `json:"remainingTodayLots"` // -1 = no daily limit
	Owned              int64  `json:"owned"`
	Available          bool   `json:"available"`
	LockedReason       string `json:"lockedReason,omitempty"` // tier | owned | sold_out | stones
}

// View is GET /shop and part of every buy response (authoritative state).
type View struct {
	Currency        string        `json:"currency"`
	Balance         int64         `json:"balance"`
	MaxLotsPerOrder int32         `json:"maxLotsPerOrder"`
	Tier            int           `json:"tier"`
	GameDay         string        `json:"gameDay"`
	ResetsAt        time.Time     `json:"resetsAt"`
	Listings        []ListingView `json:"listings"`
}

// PurchaseItem is the item side of a purchase: quantity = what the character holds after it.
type PurchaseItem struct {
	ItemID   string `json:"itemId"`
	Quantity int64  `json:"quantity"`
}

// BuyResult is POST /shop/buy.
type BuyResult struct {
	PurchaseID int64        `json:"purchaseId"`
	ListingID  string       `json:"listingId"`
	Lots       int32        `json:"lots"`
	TotalPrice int64        `json:"totalPrice"`
	Item       PurchaseItem `json:"item"`
	Replayed   bool         `json:"replayed"`
	Shop       View         `json:"shop"`
}

// Service owns the shop reads and the buy transaction.
type Service struct {
	Pool  *pgxpool.Pool
	Cat   *Catalogue
	Items *item.Catalogue
	VIP   *vip.Table
	Now   func() time.Time
}

func character(ctx context.Context, q *db.Queries, userID uuid.UUID) (db.Character, error) {
	ch, err := q.GetCharacterByUser(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ch, ErrNoCharacter
	}
	return ch, err
}

// View returns the shop with today's limits for the caller (read-only, no locks).
func (s *Service) View(ctx context.Context, userID uuid.UUID) (View, error) {
	q := db.New(s.Pool)
	ch, err := character(ctx, q, userID)
	if err != nil {
		return View{}, err
	}
	return s.view(ctx, q, userID, ch.ID, s.Now())
}

func (s *Service) view(ctx context.Context, q *db.Queries, userID, charID uuid.UUID, now time.Time) (View, error) {
	balance, err := q.GetCurrencyBalance(ctx, db.GetCurrencyBalanceParams{CharacterID: charID, Currency: Currency})
	if err != nil {
		return View{}, err
	}
	points, err := q.GetVIPPoints(ctx, userID)
	if err != nil {
		return View{}, err
	}
	counts, err := q.GetShopDailyCounts(ctx, db.GetShopDailyCountsParams{CharacterID: charID, GameDay: gameday.PG(now)})
	if err != nil {
		return View{}, err
	}
	bought := map[string]int32{}
	for _, c := range counts {
		bought[c.ListingID] = c.Lots
	}
	ids := make([]string, 0, len(s.Cat.All()))
	for _, l := range s.Cat.All() {
		ids = append(ids, l.ItemID)
	}
	inv, err := q.GetInventoryItems(ctx, db.GetInventoryItemsParams{CharacterID: charID, ItemIds: ids})
	if err != nil {
		return View{}, err
	}
	owned := map[string]int64{}
	for _, it := range inv {
		owned[it.ItemID] = it.Quantity
	}
	tier := s.VIP.TierForPoints(points)
	v := View{Currency: Currency, Balance: balance, MaxLotsPerOrder: s.Cat.Settings.MaxLotsPerOrder, Tier: tier,
		GameDay: gameday.Of(now).Format(time.DateOnly), ResetsAt: gameday.NextReset(now).UTC(), Listings: []ListingView{}}
	for _, l := range s.Cat.All() {
		d, _ := s.Items.Get(l.ItemID)
		lv := ListingView{Listing: l, BoughtTodayLots: bought[l.ID], RemainingTodayLots: l.Remaining(bought[l.ID]), Owned: owned[l.ItemID]}
		if d != nil {
			lv.Name, lv.Category, lv.Rarity = d.Name, string(d.Category), string(d.Rarity)
		}
		lv.LockedReason = l.LockedReason(d, bought[l.ID], owned[l.ItemID], tier, balance)
		lv.Available = lv.LockedReason == ""
		v.Listings = append(v.Listings, lv)
	}
	return v, nil
}

// Buy purchases `lots` of a listing in ONE transaction (data-model §12 F7, invariants S2–S7):
// lock currency account → idempotency (shop_purchases) → lock inventory row → quote (price from the catalogue,
// Tiên Duyên gate, unique, daily limit, balance) → conditional upsert shop_daily_counts → debit (currency ledger)
// → grant (inventory ledger) → purchase row. Lock order (§2.3): currency_accounts → inventory_items → shop_daily_counts.
func (s *Service) Buy(ctx context.Context, userID uuid.UUID, listingID string, lots int32, idemKey string) (BuyResult, error) {
	l, ok := s.Cat.Get(listingID)
	if !ok {
		return BuyResult{}, ErrUnknownListing
	}
	def, ok := s.Items.Get(l.ItemID)
	if !ok {
		return BuyResult{}, ErrUnknownListing
	}
	var out BuyResult
	err := postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		ch, err := character(ctx, q, userID)
		if err != nil {
			return err
		}
		if err := q.EnsureCurrencyAccount(ctx, db.EnsureCurrencyAccountParams{CharacterID: ch.ID, Currency: Currency}); err != nil {
			return err
		}
		acc, err := q.GetCurrencyAccountForUpdate(ctx, db.GetCurrencyAccountForUpdateParams{CharacterID: ch.ID, Currency: Currency})
		if err != nil {
			return fmt.Errorf("lock currency: %w", err)
		}
		now := s.Now()
		// Idempotency after the lock: concurrent duplicates serialize on the account row.
		if prev, err := q.GetShopPurchaseByKey(ctx, db.GetShopPurchaseByKeyParams{CharacterID: ch.ID, IdempotencyKey: idemKey}); err == nil {
			if prev.ListingID != l.ID || prev.Lots != lots {
				return ErrKeyReuse
			}
			out, err = s.replay(ctx, q, userID, ch.ID, prev, now)
			return err
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		inv, err := q.GetInventoryItemsForUpdate(ctx, db.GetInventoryItemsForUpdateParams{CharacterID: ch.ID, ItemIds: []string{l.ItemID}})
		if err != nil {
			return err
		}
		var owned int64
		if len(inv) == 1 {
			owned = inv[0].Quantity
		}
		gd := gameday.PG(now)
		counts, err := q.GetShopDailyCounts(ctx, db.GetShopDailyCountsParams{CharacterID: ch.ID, GameDay: gd})
		if err != nil {
			return err
		}
		var bought int32
		for _, c := range counts {
			if c.ListingID == l.ID {
				bought = c.Lots
			}
		}
		points, err := q.GetVIPPoints(ctx, userID)
		if err != nil {
			return err
		}
		o, err := s.Cat.Quote(l.ID, lots, def, bought, owned, s.VIP.TierForPoints(points), acc.Balance)
		if err != nil {
			return err
		}
		limit := l.DailyLimitLots
		if limit == 0 {
			limit = 1 << 30 // unlimited listing: the row still counts lots for the view
		}
		if _, err := q.IncrementShopDailyCount(ctx, db.IncrementShopDailyCountParams{
			CharacterID: ch.ID, GameDay: gd, ListingID: l.ID, Lots: o.Lots, MaxLots: limit,
		}); errors.Is(err, pgx.ErrNoRows) {
			return ErrDailyLimit
		} else if err != nil {
			return fmt.Errorf("shop daily count: %w", err)
		}
		ref := economy.Ref{Reason: reasonPurchase, Type: "shop_listing", ID: l.ID, IdemKey: "shop:" + idemKey}
		if _, err := economy.Credit(ctx, q, ch.ID, Currency, -o.TotalPrice, ref); errors.Is(err, economy.ErrInsufficient) {
			return ErrInsufficient
		} else if err != nil {
			return fmt.Errorf("debit: %w", err)
		}
		after, err := economy.GrantItem(ctx, q, ch.ID, l.ItemID, o.ItemQuantity, ref)
		if err != nil {
			return fmt.Errorf("grant: %w", err)
		}
		id, err := q.InsertShopPurchase(ctx, db.InsertShopPurchaseParams{
			CharacterID: ch.ID, ListingID: l.ID, ItemID: l.ItemID, Lots: o.Lots, ItemQuantity: o.ItemQuantity, Currency: Currency,
			UnitPrice: l.Price, TotalPrice: o.TotalPrice, GameDay: gd, IdempotencyKey: idemKey, CreatedAt: now,
		})
		if err != nil {
			return fmt.Errorf("insert purchase: %w", err)
		}
		out = BuyResult{PurchaseID: id, ListingID: l.ID, Lots: o.Lots, TotalPrice: o.TotalPrice, Item: PurchaseItem{ItemID: l.ItemID, Quantity: after}}
		out.Shop, err = s.view(ctx, q, userID, ch.ID, now)
		return err
	})
	return out, err
}

func (s *Service) replay(ctx context.Context, q *db.Queries, userID, charID uuid.UUID, prev db.ShopPurchase, now time.Time) (BuyResult, error) {
	inv, err := q.GetInventoryItems(ctx, db.GetInventoryItemsParams{CharacterID: charID, ItemIds: []string{prev.ItemID}})
	if err != nil {
		return BuyResult{}, err
	}
	var qty int64
	if len(inv) == 1 {
		qty = inv[0].Quantity
	}
	out := BuyResult{PurchaseID: prev.ID, ListingID: prev.ListingID, Lots: prev.Lots, TotalPrice: prev.TotalPrice,
		Item: PurchaseItem{ItemID: prev.ItemID, Quantity: qty}, Replayed: true}
	out.Shop, err = s.view(ctx, q, userID, charID, now)
	return out, err
}
