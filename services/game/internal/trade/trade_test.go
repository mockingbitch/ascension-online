package trade

import (
	"errors"
	"strings"
	"testing"
	"time"
)

const shipped = "../../../../content/catalog/trade.yaml"

func loadShipped(t *testing.T) *Config {
	t.Helper()
	c, err := LoadFile(shipped)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestShippedTradeCatalogue(t *testing.T) {
	c := loadShipped(t)
	if _, ok := c.Asset(SpiritStone); !ok {
		t.Fatal("spirit_stone must be tradable")
	}
	for _, id := range []string{"premium", "thanh_dong_kinh", "truc_co_dan", "ket_dan_qua", "tu_linh_dan", "tu_linh_phu"} {
		if _, ok := c.Asset(id); ok {
			t.Errorf("%s must not be tradable", id)
		}
	}
	rc := c.Daily.ReceivedValuePerDay
	if !(rc.Stranger <= rc.Faction && rc.Faction <= rc.DaoLu) {
		t.Errorf("caps not ordered: %+v", rc)
	}
	if c.Gates.Receive.MinAccountAgeDays < 1 || c.Gates.Receive.MinCharacterAgeDays < 1 {
		t.Error("new accounts/characters must not receive")
	}
}

func TestLoadRejectsBadConfigs(t *testing.T) {
	base := `version: 1
assets:
  - { id: spirit_stone, kind: currency, value: 1 }
fee: { stone_fee_bp: 500, min_stone_fee: 1 }
offers: { expiry_hours: 24, max_lines_per_side: 4, max_quantity_per_line: 1000, max_open_outgoing: 5, list_limit: 20, history_limit: 30 }
daily: { offers_per_day: 5, completed_per_day: 5, sent_value_per_day: 1500, received_value_per_day: { stranger: 300, faction: 600, dao_lu: 1000 } }
gates:
  send: { min_account_age_days: 3, min_character_age_days: 1, min_tier: 1 }
  receive: { min_account_age_days: 7, min_character_age_days: 3, min_tier: 2 }
alerts: { window_days: 7, funnel_min_senders: 3, pair_min_value: 600, lopsided_min_bp: 9000, list_limit: 50 }
`
	if _, err := Load(strings.NewReader(base)); err != nil {
		t.Fatalf("base: %v", err)
	}
	for name, edit := range map[string][2]string{
		"premium currency":     {"id: spirit_stone, kind: currency", "id: premium, kind: currency"},
		"zero value":           {"kind: currency, value: 1", "kind: currency, value: 0"},
		"fee too high":         {"stone_fee_bp: 500", "stone_fee_bp: 2500"},
		"caps unordered":       {"faction: 600", "faction: 200"},
		"new accounts receive": {"receive: { min_account_age_days: 7", "receive: { min_account_age_days: 0"},
		"new chars receive":    {"min_character_age_days: 3", "min_character_age_days: 0"},
		"unknown key":          {"fee: {", "feex: {"},
	} {
		if _, err := Load(strings.NewReader(strings.Replace(base, edit[0], edit[1], 1))); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestValidate(t *testing.T) {
	c := loadShipped(t)
	code := func(err error) string {
		var le *LineError
		if errors.As(err, &le) {
			return le.Code
		}
		return ""
	}
	cases := []struct {
		name       string
		give, take []Line
		want       string
		kind       string
	}{
		{"gift stones", []Line{{SpiritStone, 100}}, nil, "", KindGift},
		{"swap stones for herbs", []Line{{SpiritStone, 50}}, []Line{{"linh_thao", 10}}, "", KindSwap},
		{"empty", nil, []Line{{"linh_thao", 1}}, "empty_gift", ""},
		{"artifact (crafted)", []Line{{"cuu_tieu_hoan", 1}}, nil, "not_tradable", ""},
		{"requirement item", []Line{{SpiritStone, 1}}, []Line{{"truc_co_dan", 1}}, "not_tradable", ""},
		{"shop speed item", []Line{{"tu_linh_dan", 1}}, nil, "not_tradable", ""},
		{"premium", []Line{{"premium", 1}}, nil, "not_tradable", ""},
		{"unknown", []Line{{"khong_co", 1}}, nil, "not_tradable", ""},
		{"zero qty", []Line{{SpiritStone, 0}}, nil, "bad_quantity", ""},
		{"negative qty", []Line{{SpiritStone, -5}}, nil, "bad_quantity", ""},
		{"huge qty", []Line{{SpiritStone, c.Offers.MaxQuantityPerLine + 1}}, nil, "bad_quantity", ""},
		{"duplicate", []Line{{SpiritStone, 1}, {SpiritStone, 2}}, nil, "duplicate_line", ""},
		{"same both sides", []Line{{SpiritStone, 1}}, []Line{{SpiritStone, 2}}, "same_asset_both_sides", ""},
		{"too many lines", []Line{{SpiritStone, 1}, {"linh_thao", 1}, {"a", 1}, {"b", 1}, {"c", 1}}, nil, "too_many_lines", ""},
	}
	for _, tc := range cases {
		p, err := c.Validate(tc.give, tc.take)
		if got := code(err); got != tc.want {
			t.Errorf("%s: code %q want %q (err %v)", tc.name, got, tc.want, err)
		}
		if tc.want == "" && p.Kind != tc.kind {
			t.Errorf("%s: kind %q want %q", tc.name, p.Kind, tc.kind)
		}
	}
	p, _ := c.Validate([]Line{{SpiritStone, 50}, {"linh_thao", 4}}, []Line{{"linh_thao", 1}}[:0])
	if p.ValueToReceiver() != 50+4*5 || p.StonesGiven() != 50 || p.ValueToSender() != 0 {
		t.Errorf("values: %d %d %d", p.ValueToReceiver(), p.StonesGiven(), p.ValueToSender())
	}
}

func TestStoneFee(t *testing.T) {
	c := loadShipped(t) // 500 bp, min 1
	for _, tc := range []struct{ stones, fee int64 }{{0, 0}, {1, 1}, {19, 1}, {20, 1}, {21, 2}, {100, 5}, {101, 6}, {300, 15}, {1000, 50}} {
		if got := c.StoneFee(tc.stones); got != tc.fee {
			t.Errorf("fee(%d) = %d want %d", tc.stones, got, tc.fee)
		}
	}
}

func TestRelationAndCaps(t *testing.T) {
	c := loadShipped(t)
	if Relation(true, "a", "b") != RelDaoLu || Relation(false, "a", "a") != RelFaction || Relation(false, "", "") != RelStranger ||
		Relation(false, "a", "b") != RelStranger {
		t.Fatal("relation")
	}
	if c.ReceiveCap(RelStranger) != 300 || c.ReceiveCap(RelFaction) != 600 || c.ReceiveCap(RelDaoLu) != 1000 || c.ReceiveCap("x") != 300 {
		t.Fatal("caps")
	}
}

func TestCheckAcceptBoundaries(t *testing.T) {
	c := loadShipped(t)
	gift := func(stones int64) Plan {
		p, err := c.Validate([]Line{{SpiritStone, stones}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	code := func(err error) string {
		var le *LimitError
		if errors.As(err, &le) {
			return le.Side + ":" + le.Code
		}
		return ""
	}
	// Exactly at the stranger cap passes; one over fails.
	if err := c.CheckAccept(gift(300), RelStranger, Day{}, Day{}); err != nil {
		t.Fatalf("at cap: %v", err)
	}
	if got := code(c.CheckAccept(gift(301), RelStranger, Day{}, Day{})); got != "receiver:"+LimitReceived {
		t.Fatalf("over cap: %q", got)
	}
	if got := code(c.CheckAccept(gift(1), RelStranger, Day{}, Day{ValueReceived: 300})); got != "receiver:"+LimitReceived {
		t.Fatalf("cap used: %q", got)
	}
	if err := c.CheckAccept(gift(1000), RelDaoLu, Day{}, Day{}); err != nil {
		t.Fatalf("dao lu cap: %v", err)
	}
	if got := code(c.CheckAccept(gift(601), RelFaction, Day{}, Day{})); got != "receiver:"+LimitReceived {
		t.Fatalf("faction over: %q", got)
	}
	if got := code(c.CheckAccept(gift(10), RelStranger, Day{ValueSent: 1495}, Day{})); got != "sender:"+LimitSent {
		t.Fatalf("sent cap: %q", got)
	}
	if err := c.CheckAccept(gift(5), RelStranger, Day{ValueSent: 1495}, Day{}); err != nil {
		t.Fatalf("sent at cap: %v", err)
	}
	if got := code(c.CheckAccept(gift(1), RelStranger, Day{}, Day{Completed: 5})); got != "receiver:"+LimitCompleted {
		t.Fatalf("completed: %q", got)
	}
	if got := code(c.CheckAccept(gift(1), RelStranger, Day{Completed: 5}, Day{})); got != "sender:"+LimitCompleted {
		t.Fatalf("completed sender: %q", got)
	}
	// Swap: the sender receives too and is capped like any receiver.
	swap, _ := c.Validate([]Line{{"linh_thao", 1}}, []Line{{SpiritStone, 301}})
	if got := code(c.CheckAccept(swap, RelStranger, Day{}, Day{})); got != "sender:"+LimitReceived {
		t.Fatalf("swap sender cap: %q", got)
	}
	if got := code(c.CheckCreate(gift(1), RelStranger, Day{OffersCreated: 5})); got != "sender:"+LimitOffersPerDay {
		t.Fatalf("offers/day: %q", got)
	}
	if got := code(c.CheckCreate(gift(301), RelStranger, Day{})); got != "receiver:"+LimitReceived {
		t.Fatalf("create static cap: %q", got)
	}
}

func TestGates(t *testing.T) {
	c := loadShipped(t)
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	d := 24 * time.Hour
	old := Party{AccountCreatedAt: now.Add(-30 * d), CharacterCreatedAt: now.Add(-30 * d), Tier: 2}
	if b := c.ReceiveBlockers(old, now); len(b) != 0 {
		t.Fatalf("old: %v", b)
	}
	newAcc := Party{AccountCreatedAt: now.Add(-7*d + time.Second), CharacterCreatedAt: now.Add(-30 * d), Tier: 2}
	if b := c.ReceiveBlockers(newAcc, now); strings.Join(b, ",") != BlockAccountAge {
		t.Fatalf("new account: %v", b)
	}
	atAge := Party{AccountCreatedAt: now.Add(-7 * d), CharacterCreatedAt: now.Add(-3 * d), Tier: 2}
	if b := c.ReceiveBlockers(atAge, now); len(b) != 0 {
		t.Fatalf("exactly at age: %v", b)
	}
	fresh := Party{AccountCreatedAt: now, CharacterCreatedAt: now, Tier: 1}
	if b := c.ReceiveBlockers(fresh, now); strings.Join(b, ",") != "account_age,character_age,tier" {
		t.Fatalf("fresh: %v", b)
	}
	if b := c.SendBlockers(Party{AccountCreatedAt: now.Add(-3 * d), CharacterCreatedAt: now.Add(-d), Tier: 1}, now); len(b) != 0 {
		t.Fatalf("send at age: %v", b)
	}
}
