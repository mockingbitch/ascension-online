package main

import (
	"fmt"
	"math/rand/v2"
	"os"
	"text/tabwriter"

	"tiendao/game/internal/breakthrough"
	"tiendao/game/internal/content"
	"tiendao/game/internal/fixedpoint"
	"tiendao/game/internal/realm"
	"tiendao/game/internal/secretrealm"
)

// skillMix is the band/survival distribution of one kind of bí cảnh player. MEASURED with the test-only reference
// player (internal/secretrealm/bicanh/bicanhtest, shipped params): "vừa" survives 98% with scores P10 875 · P50 1020;
// "chậm" survives 72%, P10 640 · P50 947; both reach ≥ Hạ Phẩm (score + linh khí + kills floors) on 99–100% of seeds.
// The shares below round those to the shipped bands.
type skillMix struct {
	name                  string
	thuong, trung, ha, nb int // per-mille of runs ending in each band (nb = no band)
	survivePermille       int // of the runs in a band, share that survived the time limit
}

var srMixes = []skillMix{
	{name: "vừa", thuong: 800, trung: 200, survivePermille: 980},
	{name: "chậm", thuong: 550, trung: 350, ha: 100, survivePermille: 720},
}

// secretRealmEconomy (Phase 3.4, decisions 2026-11-07 "Bí cảnh thử luyện + pháp bảo"): the bí cảnh SOURCE against the
// existing sinks, over 60 days.
//   - Stone and material income per day after the daily caps, pháp bảo acquisition (first, best, distinct), seeded.
//   - Stone budget per profile before/after: active profiles play every run of the day; idle profiles none.
//     Absorption over 60 days = cosmetics on sale + faction donation cap; steady residual = surplus − donation cap.
//   - Tribulation effect of the best owned pháp bảo (+ hộ pháp assumption): expected failed tribulations, each ≈ one
//     24 h cooldown day (balance §11.4: tribulations cost 2–4 days in total).
//   - Fairness: nothing in secret_realms.yaml reads Tiên Duyên — loot is identical at TD0 and TD12 (checked).
func secretRealmEconomy(cb *content.Bundle, p params) {
	r := cb.SecretRealms.All()[0]
	const days, players = 60, 2000
	type acc struct {
		stones, materials float64
		firstDay          []int
		best30, best60    []int64
		distinct60        []int
	}
	results := map[string]*acc{}
	for mi, m := range srMixes {
		a := &acc{}
		rng := rand.New(rand.NewPCG(uint64(mi)+7, 2026))
		for range players {
			owned := map[string]bool{}
			first := -1
			var best int64
			for d := 1; d <= days; d++ {
				today := secretrealm.Today{}
				for range r.RunsPerDay {
					x := rng.IntN(1000)
					var band *secretrealm.Band
					switch {
					case x < m.thuong:
						band = bandByID(r, "thuong")
					case x < m.thuong+m.trung:
						band = bandByID(r, "trung")
					case x < m.thuong+m.trung+m.ha:
						band = bandByID(r, "ha")
					}
					if band == nil {
						continue
					}
					l := secretrealm.RollLoot(r, band, rng.IntN(1000) < m.survivePermille, rng, func(id string) bool { return owned[id] }, today)
					today.SpiritStone += l.SpiritStone
					today.Materials += l.Materials()
					today.Artifacts += l.ArtifactCount()
					if l.Artifact != "" {
						owned[l.Artifact] = true
						if first < 0 {
							first = d
						}
						if it, ok := cb.Items.Get(l.Artifact); ok {
							best = max(best, it.TribulationBonusBP)
						}
					}
				}
				a.stones += float64(today.SpiritStone)
				a.materials += float64(today.Materials)
				if d == 30 {
					a.best30 = append(a.best30, best)
				}
			}
			a.firstDay = append(a.firstDay, first)
			a.best60 = append(a.best60, best)
			a.distinct60 = append(a.distinct60, len(owned))
		}
		a.stones /= days * players
		a.materials /= days * players
		results[m.name] = a
	}

	fmt.Printf("Bí cảnh %s (secret_realms.yaml): %d runs/day, caps %d linh thạch · %d materials · %d pháp bảo per day; %d players × %d days, seeded.\n",
		r.Name, r.RunsPerDay, r.DailyCaps.SpiritStone, r.DailyCaps.Materials, r.DailyCaps.Artifacts, players, days)
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.AlignRight)
	fmt.Fprintln(w, "skill\tstones/day\tmaterials/day\tfirst pháp bảo (median day)\tno pháp bảo by d60\tbest ≥300bp d30\tbest ≥300bp d60\tbest =500bp d60\tdistinct d60\t")
	for _, m := range srMixes {
		a := results[m.name]
		fmt.Fprintf(w, "%s\t%.0f\t%.1f\t%s\t%s\t%s\t%s\t%s\t%.1f\t\n", m.name, a.stones, a.materials, medianDayInt(a.firstDay),
			share(a.firstDay, func(v int) bool { return v < 0 }), share64(a.best30, func(v int64) bool { return v >= 300 }),
			share64(a.best60, func(v int64) bool { return v >= 300 }), share64(a.best60, func(v int64) bool { return v >= 500 }), meanInt(a.distinct60))
	}
	check(w.Flush())

	// Stone budget before/after for the profiles of the stone table (bí cảnh players = active/social profiles).
	srStones := int64(results[srMixes[0].name].stones + 0.5)
	stock := newCosmeticStock(cb)
	onSale := stock.total(0)
	donateCap := cb.FactionWorks.Settings.DailyCapPerCharacter
	fmt.Printf("\nStone budget with bí cảnh (+%d/day for profiles that play it, 'vừa' mix). Absorbable in %d days: cosmetics on sale %d + donation cap %d/day.\n",
		srStones, days, onSale, donateCap)
	w = tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.AlignRight)
	fmt.Fprintln(w, "profile\tTD\tsurplus before\tsurplus after\tΣ60 after\tabsorbable Σ60\tabsorbed\tsteady residual before\tsteady residual after\t")
	maxResidualAfter := int64(0)
	for _, pr := range []profile{profiles[0], profiles[2], profiles[3], profiles[5], profiles[8]} {
		for _, v := range []int{0, 12} {
			q := p
			q.vip = v
			tasks, _ := taskIncome(cb, pr)
			created := tasks + cb.VIP.Benefits(v).DailyGift
			destroyed := speedMilli(cb, pr, q) * q.speedPrice / realm.PointMilli
			before := created - destroyed
			after := before
			if pr.lectures { // plays bí cảnh
				after += srStones
			}
			sigma := after * days
			absorbable := onSale + donateCap*days
			res := func(s int64) int64 { return max(0, s-donateCap) }
			maxResidualAfter = max(maxResidualAfter, res(after))
			fmt.Fprintf(w, "%s\t%d\t%+d\t%+d\t%d\t%d\t%s\t%+d\t%+d\t\n", pr.name, v, before, after, sigma, absorbable,
				map[bool]string{true: "yes", false: "NO"}[sigma <= absorbable], res(before), res(after))
		}
	}
	check(w.Flush())
	fmt.Printf("Steady residual = surplus − donation cap once the on-sale collection is owned (after ≥ %d days of surplus). Highest after: +%d/day.\n",
		onSale/max(1, srStones+500), maxResidualAfter)

	// Tribulation effect of pháp bảo (+ hộ pháp): expected failed tribulations over the whole ladder.
	t := cb.Realms.Tribulation()
	var tribs []*realm.Stage
	for _, s := range cb.Realms.Ladder(cb.Realms.DefaultPath()) {
		if s.IsLastOfRealm() && s.Tribulation && cb.Realms.Next(s) != nil {
			tribs = append(tribs, s)
		}
	}
	fails := func(itemBP, members int64) float64 {
		rng := rand.New(rand.NewPCG(5, 6))
		var total int
		const n = 4000
		for range n {
			for _, s := range tribs {
				streak := int64(0)
				for {
					stored := s.RequiredMilli // first attempt as soon as ready; retries after 24 h near the bình cảnh
					if streak > 0 {
						stored = s.CapMilli
					}
					bp, _, _ := breakthrough.TribulationOdds(cb.Realms, s, breakthrough.State{CultivationMilli: stored, FailStreak: streak,
						ItemBonusBP: itemBP, ProtectorMembers: members})
					if int64(rng.IntN(fixedpoint.BPScale)) < bp {
						break
					}
					streak++
					total++
				}
			}
		}
		return float64(total) / n
	}
	base := fails(0, 0)
	fmt.Printf("\nTribulations (%d on the ladder): expected failed attempts per player ≈ days lost to 24 h cooldowns.\n", len(tribs))
	w = tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.AlignRight)
	fmt.Fprintln(w, "pháp bảo\thộ pháp\tfails\tdays saved vs none\t")
	for _, c := range []struct{ bp, m int64 }{{0, 0}, {200, 0}, {300, 0}, {500, 0}, {300, 1}, {500, 3}} {
		f := fails(c.bp, c.m)
		fmt.Fprintf(w, "+%d bp\t%d\t%.2f\t%.2f\t\n", c.bp, c.m, f, base-f)
	}
	check(w.Flush())
	fmt.Printf("Caps hold: items ≤ %d, hộ pháp ≤ %d, total ≤ %d, final ≤ %d bp (decision F) — the 'S3 → Chuẩn Thánh' day moves by at most the days saved above.\n",
		t.ItemBonusMaxBP, t.ProtectorMaxBP, t.MaxTotalBonusBP, t.MaxSuccessBP)

	// Fairness: Tiên Duyên has no input into bí cảnh. Same seed → same loot at TD0 and TD12 (the loot function has no
	// Tiên Duyên parameter at all; this guards a future one).
	a := secretrealm.RollLoot(r, bandByID(r, "thuong"), true, rand.New(rand.NewPCG(1, 1)), nil, secretrealm.Today{})
	b := secretrealm.RollLoot(r, bandByID(r, "thuong"), true, rand.New(rand.NewPCG(1, 1)), nil, secretrealm.Today{})
	if a.RolledStone != b.RolledStone || a.RolledArtifact != b.RolledArtifact {
		fmt.Fprintln(os.Stderr, "FAIRNESS VIOLATION: bí cảnh loot differs between identical runs")
		os.Exit(1)
	}
	fmt.Printf("Tiên Duyên → bí cảnh runs/day, loot, caps: no input (secret_realms.yaml has no Tiên Duyên key; loader rejects unknown keys): OK\n")
}

func bandByID(r *secretrealm.Realm, id string) *secretrealm.Band {
	for i := range r.Bands {
		if r.Bands[i].ID == id {
			return &r.Bands[i]
		}
	}
	return nil
}

func medianDayInt(v []int) string {
	var got []int
	for _, x := range v {
		if x >= 0 {
			got = append(got, x)
		}
	}
	if len(got)*2 < len(v) {
		return "—"
	}
	// all players count: missing = after day 60
	s := append([]int(nil), v...)
	for i := range s {
		if s[i] < 0 {
			s[i] = 1 << 30
		}
	}
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
	return fmt.Sprint(s[len(s)/2])
}

func share(v []int, f func(int) bool) string {
	n := 0
	for _, x := range v {
		if f(x) {
			n++
		}
	}
	return fmt.Sprintf("%.0f%%", 100*float64(n)/float64(max(1, len(v))))
}

func share64(v []int64, f func(int64) bool) string {
	n := 0
	for _, x := range v {
		if f(x) {
			n++
		}
	}
	return fmt.Sprintf("%.0f%%", 100*float64(n)/float64(max(1, len(v))))
}

func meanInt(v []int) float64 {
	s := 0
	for _, x := range v {
		s += x
	}
	return float64(s) / float64(max(1, len(v)))
}
