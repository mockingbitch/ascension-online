//go:build integration

package shop_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"tiendao/game/internal/content"
	"tiendao/game/internal/item"
	"tiendao/game/internal/platform/testdb"
	"tiendao/game/internal/shop"
)

type env struct {
	pool  *pgxpool.Pool
	svc   *shop.Service
	user  uuid.UUID
	char  uuid.UUID
	clock *time.Time
	ctx   context.Context
}

func setup(t *testing.T, stones int64) *env {
	t.Helper()
	pool := testdb.New(t)
	cb, err := content.LoadAll("../../../../content/catalog")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 6, 1, 3, 0, 0, 0, time.UTC) // 10:00 ICT
	e := &env{pool: pool, user: uuid.New(), char: uuid.New(), clock: &now, ctx: context.Background()}
	e.svc = &shop.Service{Pool: pool, Cat: cb.Shop, Items: cb.Items, VIP: cb.VIP, Now: func() time.Time { return *e.clock }}
	e.exec(t, "INSERT INTO users (id, email) VALUES ($1, $2)", e.user, e.user.String()+"@t")
	e.exec(t, "INSERT INTO characters (id, user_id, name) VALUES ($1, $2, 'Thử Nghiệm')", e.char, e.user)
	e.exec(t, "INSERT INTO character_progressions (character_id, realm_id, stage_id, last_cultivated_at) VALUES ($1, 'luyen_khi', 'luyen_khi.tang_1', $2)", e.char, now)
	e.exec(t, "INSERT INTO currency_accounts (character_id, currency, balance) VALUES ($1, 'spirit_stone', $2)", e.char, stones)
	return e
}

func (e *env) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := e.pool.Exec(e.ctx, sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func (e *env) scalar(t *testing.T, sql string, args ...any) (v int64) {
	t.Helper()
	if err := e.pool.QueryRow(e.ctx, sql, args...).Scan(&v); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return v
}

func (e *env) balance(t *testing.T) int64 {
	return e.scalar(t, "SELECT balance FROM currency_accounts WHERE character_id=$1 AND currency='spirit_stone'", e.char)
}

func (e *env) qty(t *testing.T, item string) int64 {
	return e.scalar(t, "SELECT COALESCE((SELECT quantity FROM inventory_items WHERE character_id=$1 AND item_id=$2),0)", e.char, item)
}

// S4: one purchase = 1 purchase row + 1 currency ledger row + 1 inventory ledger row; a retry with the same key replays.
func TestBuyIdempotentAndLedgered(t *testing.T) {
	e := setup(t, 1000)
	r, err := e.svc.Buy(e.ctx, e.user, "truc_co_dan", 2, "buy-key-000001")
	if err != nil {
		t.Fatal(err)
	}
	if r.Replayed || r.TotalPrice != 800 || r.Item.Quantity != 2 || r.Shop.Balance != 200 {
		t.Fatalf("buy: %+v", r)
	}
	again, err := e.svc.Buy(e.ctx, e.user, "truc_co_dan", 2, "buy-key-000001")
	if err != nil || !again.Replayed || again.PurchaseID != r.PurchaseID {
		t.Fatalf("replay: %+v %v", again, err)
	}
	if _, err := e.svc.Buy(e.ctx, e.user, "truc_co_dan", 1, "buy-key-000001"); !errors.Is(err, shop.ErrKeyReuse) {
		t.Fatalf("same key, different lots: %v", err)
	}
	if _, err := e.svc.Buy(e.ctx, e.user, "tu_linh_dan", 2, "buy-key-000001"); !errors.Is(err, shop.ErrKeyReuse) {
		t.Fatalf("same key, different listing: %v", err)
	}
	if e.balance(t) != 200 || e.qty(t, "truc_co_dan") != 2 {
		t.Fatalf("double effect: balance %d qty %d", e.balance(t), e.qty(t, "truc_co_dan"))
	}
	if n := e.scalar(t, "SELECT COUNT(*) FROM shop_purchases WHERE character_id=$1", e.char); n != 1 {
		t.Fatalf("purchases %d", n)
	}
	if n := e.scalar(t, "SELECT COUNT(*) FROM currency_ledger WHERE character_id=$1 AND reason='shop_purchase' AND delta=-800 AND balance_after=200", e.char); n != 1 {
		t.Fatalf("currency ledger rows %d", n)
	}
	if n := e.scalar(t, "SELECT COUNT(*) FROM inventory_transactions WHERE character_id=$1 AND reason='shop_purchase' AND delta=2", e.char); n != 1 {
		t.Fatalf("inventory ledger rows %d", n)
	}
	for _, l := range r.Shop.Listings {
		if l.ID == "truc_co_dan" && (l.BoughtTodayLots != 2 || l.RemainingTodayLots != 1) {
			t.Fatalf("view after buy: %+v", l)
		}
	}
}

func TestBuyInsufficientStonesWritesNothing(t *testing.T) {
	e := setup(t, 124)
	if _, err := e.svc.Buy(e.ctx, e.user, "tu_linh_dan", 1, "buy-key-poor01"); !errors.Is(err, shop.ErrInsufficient) {
		t.Fatalf("want insufficient, got %v", err)
	}
	if e.balance(t) != 124 || e.qty(t, "tu_linh_dan") != 0 ||
		e.scalar(t, "SELECT COUNT(*) FROM shop_daily_counts WHERE character_id=$1", e.char) != 0 {
		t.Fatal("partial write on refusal")
	}
}

func TestBuyDailyLimitResetsNextDay(t *testing.T) {
	e := setup(t, 10_000)
	if _, err := e.svc.Buy(e.ctx, e.user, "tu_linh_dan", 2, "buy-key-lim001"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Buy(e.ctx, e.user, "tu_linh_dan", 1, "buy-key-lim002"); !errors.Is(err, shop.ErrDailyLimit) {
		t.Fatalf("want daily limit, got %v", err)
	}
	if _, err := e.svc.Buy(e.ctx, e.user, "tu_linh_dan", 3, "buy-key-lim003"); !errors.Is(err, shop.ErrDailyLimit) {
		t.Fatalf("order larger than limit: %v", err)
	}
	*e.clock = e.clock.Add(24 * time.Hour)
	if _, err := e.svc.Buy(e.ctx, e.user, "tu_linh_dan", 2, "buy-key-lim004"); err != nil {
		t.Fatalf("next day: %v", err)
	}
	if e.qty(t, "tu_linh_dan") != 4 || e.balance(t) != 10_000-4*125 {
		t.Fatalf("qty %d balance %d", e.qty(t, "tu_linh_dan"), e.balance(t))
	}
}

// S7 with a test catalogue: shipped cosmetics moved to cosmetics.yaml (decisions 2026-11-07), but the shop rule for a
// unique_per_character item stays.
func TestBuyUniqueCosmeticOnce(t *testing.T) {
	e := setup(t, 10_000)
	items, err := item.Load(strings.NewReader("version: 1\nitems:\n  - { id: khung, name: Khung, category: cosmetic, rarity: huyen, unique_per_character: true }\n"))
	if err != nil {
		t.Fatal(err)
	}
	cat, err := shop.Load(strings.NewReader("version: 1\nsettings: { currency: spirit_stone, max_lots_per_order: 10 }\nlistings:\n  - { id: khung, item_id: khung, lot_size: 1, price: 3000 }\n"))
	if err != nil {
		t.Fatal(err)
	}
	e.svc.Cat, e.svc.Items = cat, items
	if _, err := e.svc.Buy(e.ctx, e.user, "khung", 1, "buy-key-uni001"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Buy(e.ctx, e.user, "khung", 1, "buy-key-uni002"); !errors.Is(err, shop.ErrAlreadyOwned) {
		t.Fatalf("second copy: %v", err)
	}
	v, err := e.svc.View(e.ctx, e.user)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range v.Listings {
		if l.ID == "khung" && (l.Available || l.LockedReason != shop.LockedOwned) {
			t.Fatalf("view: %+v", l)
		}
	}
}

// Concurrency: 8 different keys race for a 2/day listing with stones for 3 → exactly 2 succeed, balances consistent.
// The same key sent 8 times concurrently → one purchase.
func TestBuyConcurrent(t *testing.T) {
	e := setup(t, 375)
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok, limited := 0, 0
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := e.svc.Buy(e.ctx, e.user, "tu_linh_dan", 1, fmt.Sprintf("buy-key-conc%02d", i))
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				ok++
			case errors.Is(err, shop.ErrDailyLimit):
				limited++
			default:
				t.Errorf("unexpected: %v", err)
			}
		}()
	}
	wg.Wait()
	if ok != 2 || limited != 6 || e.balance(t) != 125 || e.qty(t, "tu_linh_dan") != 2 {
		t.Fatalf("ok %d limited %d balance %d qty %d", ok, limited, e.balance(t), e.qty(t, "tu_linh_dan"))
	}

	e2 := setup(t, 10_000)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := e2.svc.Buy(e2.ctx, e2.user, "truc_co_dan", 1, "buy-key-samekey"); err != nil {
				t.Errorf("same key: %v", err)
			}
		}()
	}
	wg.Wait()
	if e2.balance(t) != 9600 || e2.qty(t, "truc_co_dan") != 1 {
		t.Fatalf("same key applied twice: balance %d qty %d", e2.balance(t), e2.qty(t, "truc_co_dan"))
	}
}
