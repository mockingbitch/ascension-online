package main

import (
	"fmt"
	"math/rand/v2"
	"os"
	"text/tabwriter"

	"tiendao/game/internal/content"
	"tiendao/game/internal/realm"
)

// tradeEconomy (Phase 5.2, trade.yaml): trades MOVE linh thạch and materials between players and burn the stone fee;
// they create nothing. The section shows (1) conservation — a random trade population changes the world's stones by
// exactly −Σ fees; (2) the fee as a sink at the daily caps; (3) what received stones can buy for progress: only speed
// items (contribution speed cap) and requirement-item retries — the per-day caps (speed 10, buckets, 150 ceiling) are
// untouched; (4) requirement-item breakthroughs for an idle 1-login player fed the đạo lữ cap every day (report only).
func tradeEconomy(cb *content.Bundle, p params) {
	t, cfg := cb.Trade, cb.Contribution
	rc := t.Daily.ReceivedValuePerDay
	fmt.Printf("Trao đổi (trade.yaml): fee %d bp on linh thạch given (min %d, burned); received-value cap/day stranger %d · faction %d · đạo lữ %d; "+
		"sent cap %d/day; %d completed trades/day.\n", t.Fee.StoneFeeBP, t.Fee.MinStoneFee, rc.Stranger, rc.Faction, rc.DaoLu,
		t.Daily.SentValuePerDay, t.Daily.CompletedPerDay)

	// (1) Conservation: 2,000 random accepted trades between 200 wallets (fees paid by the givers).
	rng := rand.New(rand.NewPCG(7, 52))
	wallets := make([]int64, 200)
	var before int64
	for i := range wallets {
		wallets[i] = 500 + rng.Int64N(2000)
		before += wallets[i]
	}
	var fees, moved int64
	for range 2000 {
		a, b := rng.IntN(len(wallets)), rng.IntN(len(wallets))
		if a == b {
			continue
		}
		amt := 1 + rng.Int64N(rc.DaoLu)
		fee := t.StoneFee(amt)
		if wallets[a] < amt+fee { // refused at accept (re-verified balance)
			continue
		}
		wallets[a] -= amt + fee
		wallets[b] += amt
		fees += fee
		moved += amt
	}
	var after int64
	for _, w := range wallets {
		after += w
	}
	conserved := before-after == fees
	fmt.Printf("Conservation (2000 random trades, seeded): moved %d, fees burned %d, world stones %d → %d (Δ %d = −fees): %s\n",
		moved, fees, before, after, after-before, okFail(conserved))

	// (2) Fee as a sink at the caps.
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.AlignRight)
	fmt.Fprintln(w, "flow/day (linh thạch)\tmoved\tfee burned\tfee share\t")
	for _, r := range []struct {
		name string
		amt  int64
	}{{"stranger cap", rc.Stranger}, {"faction cap", rc.Faction}, {"đạo lữ cap", rc.DaoLu}, {"giver sent cap", t.Daily.SentValuePerDay}} {
		fee := t.StoneFee(r.amt)
		fmt.Fprintf(w, "%s\t%d\t%d\t%.1f%%\t\n", r.name, r.amt, fee, float64(fee)*100/float64(r.amt))
	}
	check(w.Flush())
	fmt.Printf("The fee is a small sink: at most %d stones/day per giver (sent cap), next to speed items ≤ 250/day and donations ≤ 500/day.\n",
		t.StoneFee(t.Daily.SentValuePerDay))

	// (3) Receiver progress: received stones only buy speed items (capped) — points/day can't pass the speed cap gain.
	w = tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.AlignRight)
	fmt.Fprintln(w, "receiver\tTD\tgift/day\tspeed pts/day\tpts/day\tmax day\tΔ vs no gift\tcaps\t")
	capsOK := true
	speedCap := float64(cfg.SpeedItemDailyMilli) / realm.PointMilli
	ceiling := float64(cfg.DailyCeilingMilli) / realm.PointMilli
	for _, n := range []int{1, 3} {
		for _, v := range []int{0, 12} {
			pr := profile{name: fmt.Sprintf("I%d$", n), sessions: n, stones: true}
			q := p
			q.vip = v
			base, _ := dailyStats(cb, pr, q)
			for _, g := range []int64{0, rc.Stranger, rc.DaoLu} {
				qg := q
				qg.giftStones = g
				avg, mx := dailyStats(cb, pr, qg)
				sp := float64(speedMilli(cb, pr, qg)) / realm.PointMilli
				ok := sp <= speedCap && mx <= ceiling && avg-base <= speedCap+1e-9
				capsOK = capsOK && ok
				fmt.Fprintf(w, "%s\t%d\t%d\t%.1f\t%.1f\t%.1f\t%+.1f\t%s\t\n", pr.name, v, g, sp, avg, mx, avg-base, okFail(ok))
			}
		}
	}
	check(w.Flush())
	fmt.Printf("Progress caps with trades: speed items ≤ %.0f points/day, community/activity buckets and the %.0f ceiling unchanged "+
		"(trade.yaml has no point source; no catalogue cap reads it): %s\n", speedCap, ceiling, okFail(capsOK))

	// (4) Requirement-item breakthroughs (report only): an idle 1-login player whose đạo lữ gives the full cap daily vs free@3.
	var gated []int
	for st := cb.Realms.First(cb.Realms.DefaultPath()); st != nil; st = cb.Realms.Next(st) {
		if n := cb.Realms.Next(st); n != nil && n.Tier != st.Tier && len(st.Breakthrough.Requirements) > 0 {
			gated = append(gated, n.Tier)
		}
	}
	q := p
	q.stoneItems, q.maxDays, q.vip = true, 150, 0
	w = tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.AlignRight)
	hdr := "idle+stones\tlogins\tgift/day"
	for _, tier := range gated {
		hdr += fmt.Sprintf("\tT%d day", tier)
	}
	fmt.Fprintln(w, hdr+"\titem fails\t")
	for _, c := range []struct {
		n    int
		gift int64
	}{{3, 0}, {1, 0}, {1, rc.Stranger}, {1, rc.DaoLu}} {
		qc := q
		qc.giftStones = c.gift
		rs := simulate(cb, profile{name: "idle+stones", sessions: c.n, stones: true}, qc)
		line := fmt.Sprintf("free\t%d\t%d", c.n, c.gift)
		for _, tier := range gated {
			line += "\t" + day(medianDay(rs, tier))
		}
		fmt.Fprintln(w, line+fmt.Sprintf("\t%.2f\t", avgReqFails(rs)))
	}
	check(w.Flush())
	fmt.Println("Report only: received stones fund requirement-item retries the same way the Tiên Duyên gift does (accepted exception " +
		"2026-11-07 covers the gift, not trades) — see decisions 'Trao đổi vật phẩm (Phase 5.2)'.")
	if !conserved || !capsOK {
		fmt.Fprintln(os.Stderr, "TRADE CHECK FAILED: trades must conserve stones (minus fees) and leave progress caps unchanged")
		os.Exit(1)
	}
}

func okFail(ok bool) string {
	if ok {
		return "OK"
	}
	return "FAIL"
}
