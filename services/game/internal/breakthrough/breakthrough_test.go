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
	from, _ := cat.Stage("luyen_khi.tang_3") // 66 points behind it in the realm
	p := Evaluate(cat, from, State{CultivationMilli: 10_000, Inventory: map[string]int64{"truc_co_dan": 1}}, time.Now())
	if p.Ready || len(p.Missing) != 1 || p.Missing[0].Kind != "cultivation" || p.Missing[0].Have != 76_000 || p.Missing[0].Need != 300_000 {
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
