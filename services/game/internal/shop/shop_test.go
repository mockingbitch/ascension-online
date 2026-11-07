package shop

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"tiendao/game/internal/item"
)

func TestShippedShopIsValid(t *testing.T) {
	c, err := LoadFile(filepath.Join("..", "..", "..", "..", "content", "catalog", "shop.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	items, err := item.LoadFile(filepath.Join("..", "..", "..", "..", "content", "catalog", "items.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.CrossCheck(items, 12); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Get("tu_linh_dan"); !ok {
		t.Fatal("tu_linh_dan listing missing")
	}
}

func TestRejectsBadListings(t *testing.T) {
	_, err := Load(strings.NewReader(`
version: 1
settings: { currency: premium, max_lots_per_order: 0 }
listings:
  - { id: Bad, item_id: x, lot_size: 0, price: 0 }
  - { id: a, item_id: x, lot_size: 1, price: -5, daily_limit_lots: -1 }
  - { id: a, item_id: x, lot_size: 1, price: 2000000000, min_tier: 99 }
`))
	if err == nil {
		t.Fatal("expected error")
	}
	for _, want := range []string{"currency", "max_lots_per_order", "invalid id", "lot_size", "price", "daily_limit_lots", "duplicate", "min_tier"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("missing %q in %v", want, err)
		}
	}
	if _, err := Load(strings.NewReader("version: 1\nsettings: { currency: spirit_stone, max_lots_per_order: 1 }\nlistings: []\nextra: 1\n")); err == nil {
		t.Fatal("unknown field must fail")
	}
}

func TestCrossCheckRules(t *testing.T) {
	items, err := item.Load(strings.NewReader(`
version: 1
items:
  - { id: dan, name: Đan, category: pill, rarity: linh, instant_points_milli: 5000, max_use_per_game_day: 2 }
  - { id: phap_bao, name: Pháp bảo, category: artifact, rarity: huyen, tribulation_bonus_bp: 500 }
  - { id: khung, name: Khung, category: cosmetic, rarity: huyen, unique_per_character: true }
  - { id: truc_co_dan, name: Trúc Cơ Đan, category: pill, rarity: linh }
`))
	if err != nil {
		t.Fatal(err)
	}
	ok := `version: 1
settings: { currency: spirit_stone, max_lots_per_order: 10 }
listings:
  - { id: dan, item_id: dan, lot_size: 1, price: 125, daily_limit_lots: 2 }
  - { id: truc_co_dan, item_id: truc_co_dan, lot_size: 1, price: 400 }
  - { id: khung, item_id: khung, lot_size: 1, price: 3000, min_tier: 3 }
`
	c, err := Load(strings.NewReader(ok))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.CrossCheck(items, 12); err != nil {
		t.Fatalf("requirement + speed + cosmetic should pass: %v", err)
	}
	for name, tc := range map[string]struct{ yaml, want string }{
		"odds item":        {"  - { id: pb, item_id: phap_bao, lot_size: 1, price: 1, daily_limit_lots: 1 }\n", "raises breakthrough/tribulation odds"},
		"no daily limit":   {"  - { id: dan2, item_id: dan, lot_size: 1, price: 1 }\n", "need daily_limit_lots"},
		"tier on progress": {"  - { id: dan3, item_id: dan, lot_size: 1, price: 1, daily_limit_lots: 1, min_tier: 1 }\n", "min_tier is only allowed"},
		"unique lot size":  {"  - { id: k2, item_id: khung, lot_size: 2, price: 1 }\n", "lot_size 1"},
		"tier above max":   {"  - { id: k3, item_id: khung, lot_size: 1, price: 1, min_tier: 12 }\n", "above the last"},
	} {
		c, err := Load(strings.NewReader(ok + tc.yaml))
		if err != nil {
			t.Fatal(name, err)
		}
		if err := c.CrossCheck(items, 11); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: want %q, got %v", name, tc.want, err)
		}
	}
}

func TestQuoteAndLimits(t *testing.T) {
	c, err := Load(strings.NewReader(`version: 1
settings: { currency: spirit_stone, max_lots_per_order: 5 }
listings:
  - { id: dan, item_id: dan, lot_size: 1, price: 125, daily_limit_lots: 2 }
  - { id: thao, item_id: thao, lot_size: 10, price: 50 }
  - { id: khung, item_id: khung, lot_size: 1, price: 3000, min_tier: 3 }
`))
	if err != nil {
		t.Fatal(err)
	}
	dan := &item.Def{ID: "dan", InstantPointsMilli: 5000, MaxUsePerGameDay: 2}
	thao := &item.Def{ID: "thao"}
	khung := &item.Def{ID: "khung", Category: "cosmetic", UniquePerCharacter: true}
	cases := []struct {
		name    string
		listing string
		lots    int32
		def     *item.Def
		bought  int32
		owned   int64
		tier    int
		balance int64
		want    error
		total   int64
	}{
		{"ok", "dan", 2, dan, 0, 0, 0, 250, nil, 250},
		{"exactly at limit", "dan", 1, dan, 1, 0, 0, 125, nil, 125},
		{"over daily limit", "dan", 1, dan, 2, 0, 0, 1000, ErrDailyLimit, 0},
		{"order over limit", "dan", 3, dan, 0, 0, 0, 1000, ErrDailyLimit, 0},
		{"zero lots", "dan", 0, dan, 0, 0, 0, 1000, ErrBadLots, 0},
		{"too many lots", "thao", 6, thao, 0, 0, 0, 1000, ErrBadLots, 0},
		{"unlimited listing", "thao", 5, thao, 999, 0, 0, 250, nil, 250},
		{"insufficient", "dan", 2, dan, 0, 0, 0, 249, ErrInsufficient, 250},
		{"unknown", "nope", 1, dan, 0, 0, 0, 1000, ErrUnknownListing, 0},
		{"wrong def", "dan", 1, thao, 0, 0, 0, 1000, ErrUnknownListing, 0},
		{"tier locked", "khung", 1, khung, 0, 0, 2, 9999, ErrTierLocked, 0},
		{"tier ok", "khung", 1, khung, 0, 0, 3, 3000, nil, 3000},
		{"owned", "khung", 1, khung, 0, 1, 3, 9999, ErrAlreadyOwned, 0},
		{"unique two", "khung", 2, khung, 0, 0, 3, 9999, ErrBadLots, 0},
	}
	for _, tc := range cases {
		o, err := c.Quote(tc.listing, tc.lots, tc.def, tc.bought, tc.owned, tc.tier, tc.balance)
		if !errors.Is(err, tc.want) || (tc.total > 0 && o.TotalPrice != tc.total) {
			t.Errorf("%s: got (%+v, %v), want (%d, %v)", tc.name, o, err, tc.total, tc.want)
		}
	}
	l, _ := c.Get("thao")
	if o, _ := c.Quote("thao", 3, thao, 0, 0, 0, 1000); o.ItemQuantity != 30 {
		t.Fatalf("lot size not applied: %+v", o)
	}
	if l.Remaining(50) != -1 {
		t.Fatal("unlimited listing must report -1")
	}
	d, _ := c.Get("dan")
	for _, tc := range []struct {
		bought  int32
		owned   int64
		tier    int
		balance int64
		want    string
	}{{0, 0, 0, 125, ""}, {2, 0, 0, 999, LockedSoldOut}, {0, 0, 0, 124, LockedNoStones}} {
		if got := d.LockedReason(dan, tc.bought, tc.owned, tc.tier, tc.balance); got != tc.want {
			t.Errorf("locked(%+v) = %q", tc, got)
		}
	}
	k, _ := c.Get("khung")
	if k.LockedReason(khung, 0, 0, 1, 9999) != LockedTier || k.LockedReason(khung, 0, 1, 5, 9999) != LockedOwned {
		t.Fatal("khung locked reasons")
	}
}
