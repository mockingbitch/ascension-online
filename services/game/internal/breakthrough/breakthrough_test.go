package breakthrough

import (
	"path/filepath"
	"testing"
	"time"

	"tiendao/game/internal/realm"
)

type fixedRNG int

func (f fixedRNG) IntN(int) int { return int(f) }

func loadCat(t *testing.T) *realm.Catalogue {
	t.Helper()
	c, err := realm.LoadFile(filepath.Join("..", "..", "..", "..", "content", "catalog", "realms.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func lastOf(t *testing.T, c *realm.Catalogue, realmID string) *realm.Stage {
	t.Helper()
	for _, s := range c.Ladder("tien") {
		if s.RealmID == realmID && s.IsLastOfRealm() {
			return s
		}
	}
	t.Fatalf("no realm %s", realmID)
	return nil
}

func TestEvaluateMissing(t *testing.T) {
	cat := loadCat(t)
	from, _ := cat.Stage("luyen_khi.tang_9")
	now := time.Now()
	cd := now.Add(time.Minute)
	p := Evaluate(cat, from, State{CultivationMilli: 1, CooldownUntil: &cd}, now)
	if p.Ready {
		t.Fatal("should not be ready")
	}
	kinds := map[string]bool{}
	for _, m := range p.Missing {
		kinds[m.Kind] = true
	}
	for _, k := range []string{"cultivation", "item", "cooldown"} {
		if !kinds[k] {
			t.Errorf("missing kind %s in %+v", k, p.Missing)
		}
	}
	if p.Kind != realm.Major || p.To != "truc_co.so_ky" || p.Tribulation != nil {
		t.Errorf("unexpected target %+v", p)
	}
}

// A non-last stage previews the realm's major breakthrough with the realm-wide shortfall.
func TestEvaluateFromMinorStage(t *testing.T) {
	cat := loadCat(t)
	from, _ := cat.Stage("luyen_khi.tang_3") // 6 points (2+4) behind it in the realm
	p := Evaluate(cat, from, State{CultivationMilli: 10_000, Inventory: map[string]int64{"truc_co_dan": 1}}, time.Now())
	if p.Ready || len(p.Missing) != 1 || p.Missing[0].Kind != "cultivation" || p.Missing[0].Have != 16_000 || p.Missing[0].Need != 100_000 {
		t.Fatalf("got %+v", p.Missing)
	}
}

func TestEvaluateReady(t *testing.T) {
	cat := loadCat(t)
	from, _ := cat.Stage("luyen_khi.tang_9")
	p := Evaluate(cat, from, State{CultivationMilli: from.RequiredMilli, Inventory: map[string]int64{"truc_co_dan": 1}}, time.Now())
	if !p.Ready || p.SuccessBP != 7000 || p.CooldownSeconds != 1800 {
		t.Fatalf("should be ready: %+v", p)
	}
}

func TestPityCapped(t *testing.T) {
	rule := realm.BreakthroughRule{SuccessBP: 5000, PityStepBP: 1000}
	if got := EffectiveSuccessBP(rule, 100, 3000); got != 8000 {
		t.Fatalf("pity cap: got %d", got)
	}
	if got := EffectiveSuccessBP(realm.BreakthroughRule{SuccessBP: 9500, PityStepBP: 1000}, 5, 3000); got != 10000 {
		t.Fatalf("clamp: got %d", got)
	}
}

// Odds table of data-model §3.6: base / +20% surplus / max with items + hộ pháp (cap 90%).
func TestTribulationOddsTable(t *testing.T) {
	cat := loadCat(t)
	rows := []struct {
		realm                string
		base, surplus, maxed int64
	}{
		{"hoa_than", 8000, 8500, 9000}, {"luyen_hu", 7700, 8200, 8900}, {"hop_the", 7400, 7900, 8600},
		{"dai_thua", 7100, 7600, 8300}, {"do_kiep", 6800, 7300, 8000}, {"nhan_tien", 6500, 7000, 7700},
		{"dia_tien", 6200, 6700, 7400}, {"kim_tien", 5900, 6400, 7100}, {"thai_at_kim_tien", 5600, 6100, 6800},
		{"dai_la_kim_tien", 5300, 5800, 6500},
	}
	for _, r := range rows {
		last := lastOf(t, cat, r.realm)
		if !last.Tribulation {
			t.Fatalf("%s must be a tribulation", r.realm)
		}
		if bp, _, g := TribulationOdds(cat, last, State{CultivationMilli: last.RequiredMilli}); bp != r.base || g {
			t.Errorf("%s base %d want %d", r.realm, bp, r.base)
		}
		if bp, _, _ := TribulationOdds(cat, last, State{CultivationMilli: last.CapMilli}); bp != r.surplus {
			t.Errorf("%s +surplus %d want %d", r.realm, bp, r.surplus)
		}
		if bp, _, _ := TribulationOdds(cat, last, State{CultivationMilli: last.CapMilli, ItemBonusBP: 700, ProtectorMembers: 5}); bp != r.maxed {
			t.Errorf("%s max %d want %d", r.realm, bp, r.maxed)
		}
	}
}

func TestTribulationGuaranteeAndLoss(t *testing.T) {
	cat := loadCat(t)
	last := lastOf(t, cat, "luyen_hu") // requires 263 points, cap 315.6
	p := Evaluate(cat, last, State{CultivationMilli: last.CapMilli, FailStreak: 2}, time.Now())
	if p.Tribulation == nil || p.Tribulation.Guaranteed || p.Tribulation.LossMilli != 78_900 || p.Tribulation.Name != "Lôi Kiếp" ||
		p.CooldownSeconds != 86400 || !p.Ready {
		t.Fatalf("preview %+v trib %+v", p, p.Tribulation)
	}
	p = Evaluate(cat, last, State{CultivationMilli: last.RequiredMilli, FailStreak: 3}, time.Now())
	if p.SuccessBP != 10_000 || !p.Tribulation.Guaranteed || p.Tribulation.LossMilli != 0 {
		t.Fatalf("guarantee after 3 fails: %+v %+v", p, p.Tribulation)
	}
	if !Roll(p.SuccessBP, fixedRNG(9999)).Success {
		t.Fatal("guaranteed attempt must always succeed")
	}
	vu, _ := cat.Convert(last, "vu")
	if Evaluate(cat, vu, State{CultivationMilli: vu.RequiredMilli}, time.Now()).Tribulation.Name != "Huyết Kiếp" {
		t.Fatal("vu tribulation name")
	}
}

// Surplus bonus is monotonic and never exceeds its cap; total never exceeds max_success_bp.
func TestSurplusBonusBounds(t *testing.T) {
	cat := loadCat(t)
	tr := cat.Tribulation()
	last := lastOf(t, cat, "hoa_than")
	prev := int64(-1)
	for stored := last.RequiredMilli - 10; stored <= last.CapMilli*2; stored += 997 {
		b := SurplusBonusBP(tr, last.RequiredMilli, stored)
		if b < prev || b > tr.SurplusBonus.MaxBP {
			t.Fatalf("surplus %d at %d", b, stored)
		}
		prev = b
		if bp, _, _ := TribulationOdds(cat, last, State{CultivationMilli: stored, ItemBonusBP: 1 << 20, ProtectorMembers: 99}); bp > tr.MaxSuccessBP {
			t.Fatalf("odds %d exceed cap", bp)
		}
	}
}

func TestFinalStage(t *testing.T) {
	cat := loadCat(t)
	top := cat.Ladder("tien")[len(cat.Ladder("tien"))-1]
	p := Evaluate(cat, top, State{CultivationMilli: top.CapMilli}, time.Now())
	if p.Ready || len(p.Missing) != 1 || p.Missing[0].Kind != "final_stage" {
		t.Fatalf("Chuẩn Thánh has no further breakthrough: %+v", p)
	}
}

func TestRoll(t *testing.T) {
	if !Roll(5000, fixedRNG(4999)).Success || Roll(5000, fixedRNG(5000)).Success {
		t.Fatal("roll boundary wrong")
	}
	if Roll(0, fixedRNG(0)).Success {
		t.Fatal("0bp must never succeed")
	}
}

func TestQuoteBound(t *testing.T) {
	cat := loadCat(t)
	last := lastOf(t, cat, "luyen_hu")
	cur := LossMilli(cat, last.RequiredMilli)
	b := QuoteBoundMilli(cat, last, last.RequiredMilli, 2083, 20000)
	// 600 s × 2083/h × 2 ≈ 694 milli more stored → ≈ 173 milli more loss.
	if b <= cur || b > cur+200 {
		t.Fatalf("bound %d vs loss %d", b, cur)
	}
	if QuoteBoundMilli(cat, last, last.CapMilli, 2083, 20000) != LossMilli(cat, last.CapMilli) {
		t.Fatal("at the bình cảnh cap the bound equals the loss")
	}
}

// Kim Đan → Nguyên Anh is gated by Tâm Ma: without a valid pass the preview lists missing "trial"; an expired
// pass counts as none; a valid pass discloses its expiry and the breakthrough is ready.
func TestEvaluateTrialGate(t *testing.T) {
	cat := loadCat(t)
	last := lastOf(t, cat, "kim_dan")
	now := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	full := State{CultivationMilli: last.RequiredMilli}
	cases := []struct {
		name   string
		until  *time.Time
		ready  bool
		passed bool
	}{
		{"no pass", nil, false, false},
		{"expired pass", ptr(now.Add(-time.Second)), false, false},
		{"pass expiring now", ptr(now), false, false},
		{"valid pass", ptr(now.Add(time.Hour)), true, true},
	}
	for _, c := range cases {
		st := full
		st.Trial = &TrialGate{Kind: "tam_ma", Name: "Vượt Tâm Ma", ValidUntil: c.until}
		p := Evaluate(cat, last, st, now)
		if p.Ready != c.ready || p.Trial == nil || p.Trial.Passed != c.passed {
			t.Fatalf("%s: ready %v trial %+v missing %+v", c.name, p.Ready, p.Trial, p.Missing)
		}
		hasTrial := len(p.Missing) == 1 && p.Missing[0].Kind == "trial" && p.Missing[0].TrialKind == "tam_ma"
		if hasTrial == c.ready {
			t.Fatalf("%s: missing %+v", c.name, p.Missing)
		}
		if !c.passed && p.Trial.ValidUntil != nil {
			t.Fatalf("%s: a lapsed pass must not be shown as valid", c.name)
		}
	}
	// No gate → no trial in the preview.
	if p := Evaluate(cat, last, full, now); p.Trial != nil || !p.Ready {
		t.Fatalf("ungated: %+v", p)
	}
}

func ptr[T any](v T) *T { return &v }

// Decision F at each cap boundary (Phase 3.4): items ≤ item_bonus_max_bp, protectors ≤ max members, total bonus ≤
// max_total_bonus_bp, final ≤ max_success_bp unless guaranteed; the disclosed parts always sum to the success chance.
func TestTribulationCapBoundaries(t *testing.T) {
	cat := loadCat(t)
	tr := cat.Tribulation()
	low := lastOf(t, cat, "dai_la_kim_tien") // base 53%: the final cap never bites, the bonus cap does
	high := lastOf(t, cat, "hoa_than")       // base 80%: the final cap bites
	sum := func(p []OddsPart) (s int64) {
		for _, x := range p {
			s += x.BP
		}
		return
	}
	get := func(p []OddsPart, src string) int64 {
		for _, x := range p {
			if x.Source == src {
				return x.BP
			}
		}
		return 0
	}
	cases := []struct {
		name                      string
		stage                     *realm.Stage
		surplus                   bool
		items, members            int64
		want, wantItems, wantProt int64
		wantBonusCap, wantMaxCap  int64
	}{
		{"items at cap", low, false, tr.ItemBonusMaxBP, 0, 5300 + tr.ItemBonusMaxBP, tr.ItemBonusMaxBP, 0, 0, 0},
		{"items above cap", low, false, tr.ItemBonusMaxBP + 100, 0, 5300 + tr.ItemBonusMaxBP, tr.ItemBonusMaxBP, 0, 0, 0},
		{"protectors at cap", low, false, 0, 3, 5300 + tr.ProtectorMaxBP, 0, tr.ProtectorMaxBP, 0, 0},
		{"protectors above cap", low, false, 0, 5, 5300 + tr.ProtectorMaxBP, 0, tr.ProtectorMaxBP, 0, 0},
		{"total exactly 12%", low, false, tr.ItemBonusMaxBP, 3, 5300 + 1000, 700, 300, 0, 0},
		{"total above 12%", low, true, tr.ItemBonusMaxBP, 3, 5300 + tr.MaxTotalBonusBP, 700, 300, tr.MaxTotalBonusBP - 1500, 0},
		{"final cap", high, true, tr.ItemBonusMaxBP, 0, tr.MaxSuccessBP, 700, 0, 0, tr.MaxSuccessBP - 8000 - 500 - 700},
		{"both caps", high, true, tr.ItemBonusMaxBP, 3, tr.MaxSuccessBP, 700, 300, tr.MaxTotalBonusBP - 1500, tr.MaxSuccessBP - 8000 - tr.MaxTotalBonusBP},
	}
	for _, c := range cases {
		stored := c.stage.RequiredMilli
		if c.surplus {
			stored = c.stage.CapMilli
		}
		bp, odds, g := TribulationOdds(cat, c.stage, State{CultivationMilli: stored, ItemBonusBP: c.items, ProtectorMembers: c.members})
		if g || bp != c.want || sum(odds) != bp || get(odds, "items") != c.wantItems || get(odds, "protector") != c.wantProt ||
			get(odds, "bonus_cap") != c.wantBonusCap || get(odds, "max_cap") != c.wantMaxCap {
			t.Errorf("%s: %d %+v", c.name, bp, odds)
		}
	}
	// Guaranteed: 10000 and the guarantee part closes the sum.
	bp, odds, g := TribulationOdds(cat, high, State{CultivationMilli: high.CapMilli, FailStreak: int64(tr.GuaranteeAfter), ItemBonusBP: 700, ProtectorMembers: 3})
	if !g || bp != 10000 || sum(odds) != 10000 {
		t.Fatalf("guaranteed: %d %+v", bp, odds)
	}
}
