package cosmetic

import (
	"errors"
	"strings"
	"testing"
	"time"
)

const shipped = "../../../../content/catalog/cosmetics.yaml"

type eras map[string]int

func (e eras) EraSeq(id string) (int, bool) { s, ok := e[id]; return s, ok }

var testEras = eras{"khai_giang": 0, "dai_chien": 1, "luan_hoi": 2, "tao_nhan": 3, "thanh_nhan": 4, "nhan_toc": 5, "phong_than": 6}

func TestShippedCosmeticsAreValid(t *testing.T) {
	c, err := LoadFile(shipped)
	if err != nil {
		t.Fatal(err)
	}
	if c.Settings.CaveSlots < 1 || c.ShopTotal() <= 0 {
		t.Fatalf("settings %+v total %d", c.Settings, c.ShopTotal())
	}
	for _, k := range Kinds {
		n := 0
		for _, cm := range c.All() {
			if cm.Kind == k && cm.Source == SourceShop {
				n++
			}
		}
		if n == 0 {
			t.Errorf("no purchasable %s", k)
		}
	}
	// Legacy shop cosmetics keep their price under their new id (migration 000014).
	for id, price := range map[string]int64{"avatar_frame:ngoc_bich": 3000, "cave_decoration:tung_hac": 5000} {
		if cm, ok := c.Get(id); !ok || cm.Price != price {
			t.Errorf("%s: %+v", id, cm)
		}
	}
}

func load(t *testing.T, y string) error {
	t.Helper()
	_, err := Load(strings.NewReader(y))
	return err
}

const base = `version: 1
settings: { currency: spirit_stone, cave_slots: 2 }
seasons:
  - { id: dong, name: Đông, months: [11, 12, 1] }
series:
`

func TestLoadRejects(t *testing.T) {
	cases := map[string]struct{ series, want string }{
		"effect field": {"  - { id: a, kind: title, name: A, items: [{ slug: x, name: X, rarity: linh, price: 10, breakthrough_bonus_bp: 100 }] }\n",
			`field "breakthrough_bonus_bp" is a gameplay effect`},
		"effect on series": {"  - { id: a, kind: title, name: A, rate_bonus_bp: 5, items: [{ slug: x, name: X, rarity: linh, price: 10 }] }\n",
			`field "rate_bonus_bp" is a gameplay effect`},
		"points field": {"  - { id: a, kind: title, name: A, items: [{ slug: x, name: X, rarity: linh, price: 10, instant_points_milli: 5 }] }\n",
			"gameplay effect"},
		"unknown field": {"  - { id: a, kind: title, name: A, items: [{ slug: x, name: X, rarity: linh, price: 10, colour: red }] }\n",
			`unknown field "colour"`},
		"price not escalating": {"  - { id: a, kind: title, name: A, items: [{ slug: x, name: X, rarity: linh, price: 10 }, { slug: y, name: Y, rarity: linh, price: 10 }] }\n",
			"escalating ladder"},
		"perk with price": {"  - { id: a, kind: title, name: A, items: [{ slug: z, name: Z, rarity: linh, price: 5 }] }\n  - { id: b, kind: title, name: B, source: tien_duyen, items: [{ slug: x, name: X, rarity: linh, price: 10 }] }\n",
			"no price or min_tier"},
		"perk with window": {"  - { id: a, kind: title, name: A, items: [{ slug: z, name: Z, rarity: linh, price: 5 }] }\n  - { id: b, kind: title, name: B, source: tien_duyen, season: dong, items: [{ slug: x, name: X, rarity: linh }] }\n",
			"no availability window"},
		"dup slug across kinds": {"  - { id: a, kind: title, name: A, items: [{ slug: x, name: X, rarity: linh, price: 5 }] }\n  - { id: b, kind: chat_aura, name: B, items: [{ slug: x, name: X, rarity: linh, price: 6 }] }\n",
			"duplicate slug"},
		"bad kind":     {"  - { id: a, kind: breakthrough_fx, name: A, items: [{ slug: x, name: X, rarity: linh, price: 5 }] }\n", "unknown kind"},
		"no shop item": {"  - { id: b, kind: title, name: B, source: tien_duyen, items: [{ slug: x, name: X, rarity: linh }] }\n", "decision G"},
		"bad season":   {"  - { id: a, kind: title, name: A, season: xuan, items: [{ slug: x, name: X, rarity: linh, price: 5 }] }\n", "unknown season"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if err := load(t, base+tc.series); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q, got %v", tc.want, err)
			}
		})
	}
}

func testCat(t *testing.T) *Catalogue {
	t.Helper()
	c, err := Load(strings.NewReader(base + `  - { id: t, kind: title, name: T, items: [{ slug: a, name: A, rarity: linh, price: 100 }, { slug: b, name: B, rarity: linh, price: 200, min_tier: 3 }] }
  - { id: w, kind: cave_decoration, name: W, season: dong, items: [{ slug: c1, name: C1, rarity: linh, price: 50 }, { slug: c2, name: C2, rarity: linh, price: 60 }, { slug: c3, name: C3, rarity: linh, price: 70 }] }
  - { id: e, kind: cave_decoration, name: E, from_era: dai_chien, until_era: luan_hoi, items: [{ slug: e1, name: E1, rarity: linh, price: 50 }] }
  - { id: p, kind: avatar_frame, name: P, source: tien_duyen, items: [{ slug: ngoc, name: Ngọc, rarity: huyen }] }
`))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.BindPerks([]PerkGrant{{"avatar_frame:ngoc", 3}, {"alchemy_slot", 3}, {"breakthrough_fx:x", 12}}); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestBindPerks(t *testing.T) {
	c := testCat(t)
	if cm, _ := c.Get("avatar_frame:ngoc"); cm.PerkTier != 3 {
		t.Fatalf("perk tier %d", cm.PerkTier)
	}
	c2, _ := Load(strings.NewReader(base + "  - { id: t, kind: title, name: T, items: [{ slug: a, name: A, rarity: linh, price: 100 }] }\n  - { id: p, kind: avatar_frame, name: P, source: tien_duyen, items: [{ slug: ngoc, name: N, rarity: huyen }] }\n"))
	err := c2.BindPerks([]PerkGrant{{"avatar_frame:ngoc", 3}, {"avatar_frame:ngoc", 6}, {"title:a", 1}, {"chat_aura:zz", 2}})
	if err := c2.BindPerks([]PerkGrant{{"avatar_frame:ngoc", 0}}); err == nil || !strings.Contains(err.Error(), "need tier ≥ 1") {
		t.Errorf("tier-0 perk accepted: %v", err)
	}
	c2, _ = Load(strings.NewReader(base + "  - { id: t, kind: title, name: T, items: [{ slug: a, name: A, rarity: linh, price: 100 }] }\n  - { id: p, kind: avatar_frame, name: P, source: tien_duyen, items: [{ slug: ngoc, name: N, rarity: huyen }] }\n"))
	err = c2.BindPerks([]PerkGrant{{"avatar_frame:ngoc", 3}, {"avatar_frame:ngoc", 6}, {"title:a", 1}, {"chat_aura:zz", 2}})
	for _, want := range []string{"more than one tier", "must be source tien_duyen", `"chat_aura:zz" (tier 2) has no cosmetics.yaml entry`} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("want %q in %v", want, err)
		}
	}
}

func TestAvailableAndQuote(t *testing.T) {
	c := testCat(t)
	nov, may := When{Month: time.November, EraSeq: 0}, When{Month: time.May, EraSeq: 1}
	cases := []struct {
		id      string
		owned   bool
		tier    int
		balance int64
		w       When
		want    error
	}{
		{"title:a", false, 0, 100, nov, nil},
		{"title:a", false, 0, 99, nov, ErrInsufficient},
		{"title:a", true, 0, 1000, nov, ErrAlreadyOwned},
		{"title:b", false, 2, 1000, nov, ErrTierLocked},
		{"title:b", false, 3, 1000, nov, nil},
		{"cave_decoration:c1", false, 0, 1000, nov, nil},
		{"cave_decoration:c1", false, 0, 1000, may, ErrUnavailable},               // out of season
		{"cave_decoration:e1", false, 0, 1000, nov, ErrUnavailable},               // era 0 < from
		{"cave_decoration:e1", false, 0, 1000, may, nil},                          // era 1 in window
		{"cave_decoration:e1", false, 0, 1000, When{time.May, 3}, ErrUnavailable}, // past until
		{"avatar_frame:ngoc", false, 12, 1000, nov, ErrNotForSale},
		{"title:zz", false, 0, 1000, nov, ErrUnknown},
	}
	for _, tc := range cases {
		if _, err := c.Quote(tc.id, tc.owned, tc.tier, tc.balance, tc.w, testEras); !errors.Is(err, tc.want) {
			t.Errorf("%s %+v: got %v want %v", tc.id, tc, err, tc.want)
		}
	}
}

func TestValidateLoadout(t *testing.T) {
	c := testCat(t)
	bought := map[string]bool{"title:a": true, "cave_decoration:c1": true, "cave_decoration:c2": true, "cave_decoration:c3": true}
	owns := func(tier int) func(*Cosmetic) bool { return func(cm *Cosmetic) bool { return Owns(cm, bought, tier) } }
	cases := []struct {
		a    Appearance
		tier int
		want error
	}{
		{Appearance{}, 0, nil},
		{Appearance{TitleID: "title:a", CaveDecorationIDs: []string{"cave_decoration:c1", "cave_decoration:c2"}}, 0, nil},
		{Appearance{TitleID: "title:b"}, 9, ErrNotOwned},                                                                              // not bought
		{Appearance{AvatarFrameID: "avatar_frame:ngoc"}, 2, ErrNotOwned},                                                              // perk tier 3
		{Appearance{AvatarFrameID: "avatar_frame:ngoc"}, 3, nil},                                                                      // perk reached
		{Appearance{TitleID: "cave_decoration:c1"}, 0, ErrBadLoadout},                                                                 // wrong slot
		{Appearance{CaveDecorationIDs: []string{"cave_decoration:c1", "cave_decoration:c2", "cave_decoration:c3"}}, 0, ErrBadLoadout}, // > slots
		{Appearance{CaveDecorationIDs: []string{"cave_decoration:c1", "cave_decoration:c1"}}, 0, ErrBadLoadout},                       // dup
		{Appearance{ChatAuraID: "chat_aura:nope"}, 0, ErrBadLoadout},
	}
	for i, tc := range cases {
		if err := c.Validate(tc.a, owns(tc.tier)); !errors.Is(err, tc.want) {
			t.Errorf("case %d: got %v want %v", i, err, tc.want)
		}
	}
}

func TestEffectLike(t *testing.T) {
	for _, k := range []string{"rate_bonus_bp", "instant_points_milli", "offline_cap_bonus_hours", "cooldown_seconds", "merit", "rank_points", "tribulation_bonus_bp"} {
		if !EffectLike(k) {
			t.Errorf("%s should be effect-like", k)
		}
	}
	for _, k := range []string{"description", "colour", "slug"} {
		if EffectLike(k) {
			t.Errorf("%s is not an effect", k)
		}
	}
}
