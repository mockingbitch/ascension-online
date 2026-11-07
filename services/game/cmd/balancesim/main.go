// Command balancesim simulates shared-world progression with the real catalogue (content.LoadAll) and the real
// rule functions: cultivation.Settle (idle rate + carry), cultivation.Credit (catch-up, bucket caps, 150/day
// ceiling, bình cảnh, auto-advance), breakthrough.Evaluate/Roll (pity, tribulation odds, guarantee, loss).
//
//	go run ./cmd/balancesim                      # archetype table (Tiên Duyên 0) + Tiên Duyên fairness check
//	go run ./cmd/balancesim -profile S3 -vip 4   # one archetype, per-realm table
//
// The default run also prints the Tiên Duyên gift gap (idle players funding speed items from daily-task stones +
// the daily gift; exits 1 on a fairness/ceiling violation) and the free-play Tiên Duyên level table.
//
// Every GAME number comes from content/catalog. The flags below are PLAYER-BEHAVIOUR assumptions from
// docs/design/balance-the-gioi-chung.md §1 (they move to lectures.yaml/shop.yaml when those systems ship):
//   - -lecture: points per attended lecture (2 slots/day; a 1-login player catches 1);
//   - -community: community points per login (≤ 2 logins counted, bucket cap still applies);
//   - active players (A*, S*) use the full speed-item cap (contribution.yaml speed_items) once per day, offer incense
//     every day (daily_tasks.yaml streak_rewards, source ritual) and draw gieo quẻ at their first login (fortunes.yaml,
//     EXPECTED bonus = Σ weight × rate_bonus, buff from that login for -fortunehours hours, i.e. until midnight).
//   - speed-item price per point comes from shop.yaml (the listing that sells the speed item) unless -speedprice > 0.
//
// The default run ends with the spirit-stone economy table: daily stone income (daily tasks + Tiên Duyên gift) vs
// shop spending (speed items), then the recurring non-progress sinks (cosmetics.yaml, faction_works.yaml) absorbing the
// surplus over -sinkdays days (-donateshare = assumed donation share).
//
// Player model (Monte Carlo, -runs players per profile): `n` logins per day, evenly spaced. Each login settles idle
// time (Tiên Duyên window, Tụ Linh Phù once per day for the first gap it extends), credits the day's activity
// share, then attempts the realm breakthrough as soon as it is allowed ("thử ngay"). Requirement items are
// assumed obtainable. Game days start at the first login of each simulated day.
package main

import (
	"flag"
	"fmt"
	"math/rand/v2"
	"os"
	"slices"
	"strings"
	"text/tabwriter"
	"time"

	"tiendao/game/internal/breakthrough"
	"tiendao/game/internal/content"
	"tiendao/game/internal/contribution"
	"tiendao/game/internal/cosmetic"
	"tiendao/game/internal/cultivation"
	"tiendao/game/internal/dailytask"
	"tiendao/game/internal/realm"
	"tiendao/game/internal/shop"
	"tiendao/game/internal/vip"
)

type profile struct {
	name      string
	sessions  int
	lectures  bool // attends lectures (+ uses speed items)
	community bool
	grind     bool // fills every bucket to its cap ("cày trần")
	stones    bool // speed items funded by spirit stones (daily tasks + Tiên Duyên gift), see giftGap
}

var profiles = []profile{
	{name: "I1", sessions: 1}, {name: "I2", sessions: 2}, {name: "I3", sessions: 3},
	{name: "A1", sessions: 1, lectures: true}, {name: "A2", sessions: 2, lectures: true}, {name: "A3", sessions: 3, lectures: true},
	{name: "S1", sessions: 1, lectures: true, community: true}, {name: "S2", sessions: 2, lectures: true, community: true},
	{name: "S3", sessions: 3, lectures: true, community: true},
	{name: "G", sessions: 3, grind: true},
}

type params struct {
	vip               int
	runs, maxDays     int
	lectureMilli      int64
	communityPerLogin int64
	songTuMilli       int64 // social profiles: one full đạo lữ song tu window per day (dao_lu.yaml), at the first login
	phu               bool
	phuItem           string
	trib              bool
	speedPrice        int64 // spirit stones per speed-item point
	rituals           bool  // active profiles: incense streak + gieo quẻ buff
	fortuneHours      int   // hours the gieo quẻ buff lasts after the first login (until midnight)
	stoneItems        bool  // requirement items must be BOUGHT with the day's spare stones (shop.yaml price + limit)
	donateShareBP     int64 // share of the daily stone surplus donated to faction works before the cosmetics are owned
	sinkDays          int   // horizon of the non-progress sink simulation
	reserve           int64 // stones a player keeps unspent on cosmetics (retry buffer)
	giftStones        int64 // spirit stones/day RECEIVED through trades (trade.yaml; 0 except in the trade section)
}

type runResult struct {
	reached   []float64 // per tier (index tier-1): day reached, -1 if not
	fails     int
	lostMilli int64
	guarantee int
	reqFails  int // failed attempts that consumed requirement items
}

func main() {
	dir := flag.String("content", "../../content/catalog", "content/catalog dir")
	prof := flag.String("profile", "", "single archetype (I1..S3, G); empty = archetype table + fairness check")
	vipTier := flag.Int("vip", 0, "Tiên Duyên level")
	runs := flag.Int("runs", 300, "simulated players per profile")
	maxDays := flag.Int("maxdays", 1500, "stop after N days")
	lecture := flag.Int64("lecture", 10, "ASSUMPTION: points per lecture attended")
	community := flag.Int64("community", 5, "ASSUMPTION: other community points per login (max 2 logins) besides song tu")
	songTu := flag.Bool("songtu", true, "ASSUMPTION: social profiles have a đạo lữ and fill the song tu window daily (dao_lu.yaml)")
	phu := flag.Bool("phu", true, "players use Tụ Linh Phù (daily check-in reward) once per day")
	trib := flag.Bool("trib", true, "tribulation enabled (false = baseline without tribulation)")
	speedPrice := flag.Int64("speedprice", 0, "spirit stones per speed-item point (0 = from shop.yaml)")
	rituals := flag.Bool("rituals", true, "active profiles offer incense and draw gieo quẻ daily")
	fortuneHours := flag.Int("fortunehours", 16, "ASSUMPTION: hours from the first login to midnight (gieo quẻ buff length)")
	donateShare := flag.Int64("donateshare", 3000, "ASSUMPTION: bp of the daily stone surplus donated to faction works while cosmetics remain on sale")
	sinkDays := flag.Int("sinkdays", 120, "days simulated for the non-progress stone sinks")
	reserve := flag.Int64("reserve", 2000, "ASSUMPTION: stones kept unspent on cosmetics (breakthrough retry buffer)")
	flag.Parse()

	cb, err := content.LoadAll(*dir)
	check(err)
	p := params{vip: *vipTier, runs: *runs, maxDays: *maxDays, lectureMilli: *lecture * realm.PointMilli,
		communityPerLogin: *community * realm.PointMilli, phu: *phu, phuItem: "tu_linh_phu", trib: *trib,
		speedPrice: *speedPrice, rituals: *rituals, fortuneHours: *fortuneHours,
		donateShareBP: *donateShare, sinkDays: *sinkDays, reserve: *reserve}
	if p.speedPrice <= 0 {
		p.speedPrice = shopPricePerPoint(cb)
	}
	if *songTu {
		p.songTuMilli = cb.DaoLu.MaxSongTuMilli()
	}

	if *prof != "" {
		for _, pr := range profiles {
			if strings.EqualFold(pr.name, *prof) {
				single(cb, pr, p)
				return
			}
		}
		check(fmt.Errorf("unknown profile %q", *prof))
	}
	table(cb, p)
	fmt.Println()
	fairness(cb, p)
	fmt.Println()
	giftGap(cb, p)
	fmt.Println()
	duyenLevels(cb, p)
	fmt.Println()
	stoneEconomy(cb, p)
	fmt.Println()
	secretRealmEconomy(cb, p) // Phase 3.4: bí cảnh source vs sinks, pháp bảo, tribulation effect (secretrealm.go)
	fmt.Println()
	retryGap(cb, p)
	fmt.Println()
	tradeEconomy(cb, p) // Phase 5.2: trades move stones (fee burned), create nothing (trade.go)
}

// listingFor returns the cheapest shop listing selling itemID (nil = not sold).
func listingFor(cb *content.Bundle, itemID string) *shop.Listing {
	var best *shop.Listing
	for _, l := range cb.Shop.All() {
		if l.ItemID == itemID && (best == nil || l.Price*best.LotSize < best.Price*l.LotSize) {
			best = &l
		}
	}
	return best
}

// retryGap checks the fairness rule on TIME TO THE MAJOR BREAKTHROUGHS GATED BY REQUIREMENT ITEMS when those items
// must be bought with spare spirit stones (tasks + Tiên Duyên gift − speed items): Tiên Duyên L at 1 login/day must
// not reach them before free at 3 logins/day. More stones = more retries per day after a failure (anti-cheat L3).
//
// Accepted exception (docs/decisions.md 2026-11-07 "Ngoại lệ công bằng"): the gift funding requirement items gives
// idle TD L@1 (idle+stones profile only) a lead of 1–2.3 days (measured). A lead up to acceptedRetryLeadDays is report only; a larger lead on
// any gated breakthrough fails the run (exit 1), so the exception can't silently grow.
func retryGap(cb *content.Bundle, p params) {
	var gated []int // tiers reached through a major breakthrough with requirement items
	for st := cb.Realms.First(cb.Realms.DefaultPath()); st != nil; st = cb.Realms.Next(st) {
		if n := cb.Realms.Next(st); n != nil && n.Tier != st.Tier && len(st.Breakthrough.Requirements) > 0 {
			gated = append(gated, n.Tier)
		}
	}
	q := p
	q.stoneItems, q.maxDays = true, 150
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.AlignRight)
	hdr := "profile\tlogins\tTD"
	for _, t := range gated {
		hdr += fmt.Sprintf("\tT%d day", t)
	}
	hdr += "\titem fails\tnote"
	fmt.Fprintln(w, hdr+"\t")
	ok, within := true, true
	for _, base := range []profile{{name: "idle+stones", stones: true}, {name: "active", lectures: true}} {
		free3 := base
		free3.sessions = 3
		q0 := q
		q0.vip = 0
		ref := simulate(cb, free3, q0)
		row := func(name string, n, v int, rs []runResult, note string) {
			line := fmt.Sprintf("%s\t%d\t%d", name, n, v)
			for _, t := range gated {
				line += fmt.Sprintf("\t%s", day(medianDay(rs, t)))
			}
			line += fmt.Sprintf("\t%.2f\t%s", avgReqFails(rs), note)
			fmt.Fprintln(w, line+"\t")
		}
		row(base.name, 3, 0, ref, "reference")
		for _, v := range []int{0, 4, 8, 12} {
			pr := base
			pr.sessions = 1
			qv := q
			qv.vip = v
			rs := simulate(cb, pr, qv)
			note := "—"
			for _, t := range gated {
				d, d3 := medianDay(rs, t), medianDay(ref, t)
				if d < 0 || (d3 >= 0 && d >= d3) {
					continue
				}
				ok = false
				if !base.stones || d3 < 0 || d3-d > acceptedRetryLeadDays { // the exception covers idle+stones only
					within, note = false, "VIOLATION"
					fmt.Fprintf(os.Stderr, "RETRY-GAP VIOLATION: %s TD%d@1 reaches T%d on day %.1f, free@3 day %.1f — outside the accepted exception (idle+stones, ≤ %.0f days)\n",
						base.name, v, t, d, d3, acceptedRetryLeadDays)
					continue
				}
				if note == "—" {
					note = "accepted exception (decisions 2026-11-07)"
				}
				fmt.Fprintf(os.Stderr, "RETRY-GAP accepted exception (decisions 2026-11-07, report only): %s TD%d@1 reaches T%d on day %.1f < free@3 day %.1f (lead %.1f ≤ %.0f days)\n",
					base.name, v, t, d, d3, d3-d, acceptedRetryLeadDays)
			}
			row(base.name, 1, v, rs, note)
		}
	}
	check(w.Flush())
	verdict := "OK"
	switch {
	case !within:
		verdict = fmt.Sprintf("VIOLATED (lead > %.0f days)", acceptedRetryLeadDays)
	case !ok:
		verdict = fmt.Sprintf("accepted exception (decisions 2026-11-07), lead ≤ %.0f days", acceptedRetryLeadDays)
	}
	fmt.Printf("\nRequirement items bought with spare stones (shop.yaml price + daily limit, consumed on failure too). "+
		"Rule TD_L@1 not before free@3 on these breakthroughs: %s. 'fails' = mean failed attempts per player "+
		"over the whole run (retry frequency).\n", verdict)
	if !within {
		os.Exit(1)
	}
}

// acceptedRetryLeadDays bounds the accepted retry-gap exception (decisions 2026-11-07): the largest lead, in days,
// that idle Tiên Duyên L at 1 login/day may have over free at 3 logins/day on a requirement-item breakthrough.
// Measured when accepted: Trúc Cơ 1.0, Kim Đan 2.3 days.
const acceptedRetryLeadDays = 3.0

// avgReqFails is the mean number of failed attempts that consumed requirement items, per player.
func avgReqFails(rs []runResult) float64 {
	var n int
	for _, r := range rs {
		n += r.reqFails
	}
	return float64(n) / float64(len(rs))
}

// shopPricePerPoint is the spirit-stone price of one speed point in shop.yaml (cheapest speed listing).
func shopPricePerPoint(cb *content.Bundle) int64 {
	best := int64(0)
	for _, l := range cb.Shop.All() {
		d, ok := cb.Items.Get(l.ItemID)
		if !ok || d.InstantPointsMilli <= 0 {
			continue
		}
		per := l.Price * realm.PointMilli / (d.InstantPointsMilli * l.LotSize)
		if best == 0 || per < best {
			best = per
		}
	}
	if best == 0 {
		check(fmt.Errorf("shop.yaml sells no speed item"))
	}
	return best
}

// expectedFortuneBP is Σ weight_bp × rate_bonus_bp / 10000 (the mean gieo quẻ buff).
func expectedFortuneBP(cb *content.Bundle) int64 {
	var sum int64
	for _, o := range cb.Fortunes.All() {
		sum += o.WeightBP * o.RateBonusBP
	}
	return min(sum/10_000, cb.Contribution.MaxBuffBonusBP)
}

// ritualMilli is the incense reward on simulated day `day` for a player who never misses (streak = day + 1).
func ritualMilli(cb *content.Bundle, day int) int64 {
	t, ok := cb.DailyTasks.ByKind(dailytask.Incense)
	if !ok {
		return 0
	}
	return t.StreakPoints(int32(min(day+1, 1<<20)))
}

// simulate runs `runs` players of one archetype and returns their results.
func simulate(cb *content.Bundle, pr profile, p params) []runResult {
	cat, cfg := cb.Realms, cb.Contribution
	path := cat.DefaultPath()
	gap := 24 * time.Hour / time.Duration(pr.sessions)
	baseCap := cb.VIP.OfflineCap(p.vip)
	phuCap := baseCap
	if def, ok := cb.Items.Get(p.phuItem); ok && p.phu {
		phuCap = cb.VIP.EffectiveOfflineCap(p.vip, time.Duration(def.OfflineCapBonusHours)*time.Hour)
	}
	rng := rand.New(rand.NewPCG(42, uint64(len(pr.name)*1000+pr.sessions*100+p.vip)))
	inv := map[string]int64{}
	for _, s := range cat.Stages() {
		for _, r := range s.Breakthrough.Requirements {
			inv[r.ItemID] = 1 << 40 // requirement items assumed obtainable
		}
	}
	income, _ := taskIncome(cb, pr)
	income += cb.VIP.Benefits(p.vip).DailyGift + p.giftStones
	speedCost := speedMilli(cb, pr, p) * p.speedPrice / realm.PointMilli
	out := make([]runResult, 0, p.runs)
	for range p.runs {
		var wallet int64
		bought := map[string]int32{}
		if p.stoneItems {
			inv = map[string]int64{}
		}
		res := runResult{reached: make([]float64, cat.LastPlayableTier())}
		for i := range res.reached {
			res.reached[i] = -1
		}
		res.reached[0] = 0
		prog := cultivation.Progress{Stage: cat.First(path)}
		var carry, streak int64
		var cooldown *time.Time
		var last time.Time
		var buff cultivation.Buff
		top := cat.LastPlayableTier()

	days:
		for day := 0; day < p.maxDays; day++ {
			var today contribution.Day
			phuLeft := p.phu
			credit := func(src contribution.Source, raw int64) {
				r := cultivation.Credit(cat, cfg, prog, today, src, raw, false)
				today = today.Add(r.Applied, src)
				prog = r.After
			}
			dayStart := epoch.Add(time.Duration(day) * 24 * time.Hour)
			if pr.lectures && p.rituals && !pr.grind {
				buff = cultivation.Buff{BonusBP: expectedFortuneBP(cb), Start: dayStart, End: dayStart.Add(time.Duration(p.fortuneHours) * time.Hour)}
			}
			// Stone model: the day's stones arrive at the first login. A player who needs a requirement item for the
			// waiting breakthrough saves (no speed items that day); otherwise speed items are bought first.
			saving := false
			if p.stoneItems {
				for _, r := range prog.Stage.Breakthrough.Requirements {
					saving = saving || inv[r.ItemID] < r.Quantity
				}
				wallet += income
				if !saving {
					wallet -= min(speedCost, wallet)
				}
				clear(bought)
			}
			for s := 0; s < pr.sessions; s++ {
				now := epoch.Add(time.Duration(day)*24*time.Hour + time.Duration(s)*gap)
				if day == 0 && s == 0 {
					last = now
				}
				capDur := baseCap
				if phuLeft && now.Sub(last) > baseCap {
					capDur, phuLeft = phuCap, false
				}
				st := cultivation.Settle(cultivation.SettleInput{Now: now, LastCultivatedAt: last, RateCarry: carry,
					RateMilliPerHour: cfg.IdleRateMilliPerHour, OfflineCap: capDur, Buff: buff})
				last, carry = st.LastCultivatedAt, st.RateCarry
				credit(contribution.SourceSettle, st.RawMilli)
				if s == 0 && pr.lectures && p.rituals && !pr.grind {
					credit(contribution.SourceRitual, ritualMilli(cb, day))
				}
				n := int64(pr.sessions)
				switch {
				case pr.grind:
					credit(contribution.SourceLecture, cfg.Buckets[contribution.Activity].CapMilli/n)
					credit(contribution.SourceFaction, cfg.Buckets[contribution.Community].CapMilli/n)
				default:
					if s == 0 && !saving {
						if sp := speedMilli(cb, pr, p); sp > 0 {
							credit(contribution.SourceItem, sp)
						}
					}
					if pr.lectures && s < 2 {
						credit(contribution.SourceLecture, p.lectureMilli)
					}
					if pr.community && s < 2 {
						credit(contribution.SourceFaction, p.communityPerLogin)
					}
					if pr.community && s == 0 && p.songTuMilli > 0 {
						credit(contribution.SourceDualCultivation, p.songTuMilli)
					}
				}
				// Break through while online, as soon as allowed.
				for {
					if p.stoneItems { // buy the missing requirement items the wallet and today's limit allow
						for _, r := range prog.Stage.Breakthrough.Requirements {
							l := listingFor(cb, r.ItemID)
							for l != nil && inv[r.ItemID] < r.Quantity && wallet >= l.Price*1 &&
								(l.DailyLimitLots == 0 || bought[l.ID] < l.DailyLimitLots) {
								wallet -= l.Price
								bought[l.ID]++
								inv[r.ItemID] += l.LotSize
							}
						}
					}
					pv := breakthrough.Evaluate(cat, prog.Stage, breakthrough.State{CultivationMilli: prog.StoredMilli, FailStreak: streak,
						CooldownUntil: cooldown, Inventory: inv}, now)
					if !pv.Ready {
						break
					}
					trib := pv.Tribulation != nil && p.trib
					bp := pv.SuccessBP
					if pv.Tribulation != nil && !p.trib {
						bp = 10_000
					}
					o := breakthrough.Roll(bp, rng)
					if p.stoneItems { // consumed on success AND failure
						for _, r := range pv.Consumes {
							inv[r.ItemID] -= r.Quantity
						}
					}
					if o.Success {
						if trib && pv.Tribulation.Guaranteed {
							res.guarantee++
						}
						next := cat.Next(prog.Stage)
						st, stored, _ := cat.Advance(next, prog.StoredMilli-prog.Stage.RequiredMilli)
						prog = cultivation.Progress{Stage: st, StoredMilli: stored}
						streak, cooldown = 0, nil
						res.reached[next.Tier-1] = float64(day) + float64(s)/float64(pr.sessions)
						if next.Tier == top {
							break days
						}
						continue
					}
					streak++
					if len(pv.Consumes) > 0 {
						res.reqFails++
					}
					cd := now.Add(time.Duration(pv.CooldownSeconds) * time.Second)
					cooldown = &cd
					if trib {
						res.fails++
						res.lostMilli += pv.Tribulation.LossMilli
						prog.StoredMilli -= pv.Tribulation.LossMilli
					}
					break
				}
			}
		}
		out = append(out, res)
	}
	return out
}

// medianDay returns the median day each tier is reached (-1 when fewer than half the players reach it).
func medianDay(rs []runResult, tier int) float64 {
	var ds []float64
	for _, r := range rs {
		if d := r.reached[tier-1]; d >= 0 {
			ds = append(ds, d)
		}
	}
	if len(ds) < (len(rs)+1)/2 {
		return -1
	}
	slices.Sort(ds)
	return ds[len(ds)/2]
}

func pct(rs []runResult, tier int, q float64) float64 {
	var ds []float64
	for _, r := range rs {
		if d := r.reached[tier-1]; d >= 0 {
			ds = append(ds, d)
		}
	}
	if len(ds) < len(rs) {
		return -1
	}
	slices.Sort(ds)
	return ds[int(q*float64(len(ds)-1))]
}

func avgFails(rs []runResult) (fails, lostPoints float64) {
	for _, r := range rs {
		fails += float64(r.fails)
		lostPoints += float64(r.lostMilli) / realm.PointMilli
	}
	n := float64(len(rs))
	return fails / n, lostPoints / n
}

// dailyPoints is the steady-state points/day of an archetype (credited days 1..30, before any bottleneck).
func dailyPoints(cb *content.Bundle, pr profile, p params) float64 {
	avg, _ := dailyStats(cb, pr, p)
	return avg
}

// dailyStats returns the steady-state mean points/day and the highest single day (credited, before any bottleneck).
func dailyStats(cb *content.Bundle, pr profile, p params) (avg, maxDay float64) {
	cfg := cb.Contribution
	gap := 24 * time.Hour / time.Duration(pr.sessions)
	baseCap := cb.VIP.OfflineCap(p.vip)
	phuCap := baseCap
	if def, ok := cb.Items.Get(p.phuItem); ok && p.phu {
		phuCap = cb.VIP.EffectiveOfflineCap(p.vip, time.Duration(def.OfflineCapBonusHours)*time.Hour)
	}
	var total, carry int64
	last := epoch.Add(24*time.Hour - gap) // previous login, so day 1 is steady state
	const days = 30
	var buff cultivation.Buff
	for day := 1; day <= days; day++ {
		var today contribution.Day
		phuLeft := p.phu
		add := func(src contribution.Source, raw int64) {
			a := cfg.Apply(src, raw, today, false, 1<<50)
			today = today.Add(a, src)
		}
		rituals := pr.lectures && p.rituals && !pr.grind
		if rituals {
			dayStart := epoch.Add(time.Duration(day) * 24 * time.Hour)
			buff = cultivation.Buff{BonusBP: expectedFortuneBP(cb), Start: dayStart, End: dayStart.Add(time.Duration(p.fortuneHours) * time.Hour)}
		}
		for s := 0; s < pr.sessions; s++ {
			now := epoch.Add(time.Duration(day)*24*time.Hour + time.Duration(s)*gap)
			capDur := baseCap
			if phuLeft && now.Sub(last) > baseCap {
				capDur, phuLeft = phuCap, false
			}
			st := cultivation.Settle(cultivation.SettleInput{Now: now, LastCultivatedAt: last, RateCarry: carry,
				RateMilliPerHour: cfg.IdleRateMilliPerHour, OfflineCap: capDur, Buff: buff})
			last, carry = st.LastCultivatedAt, st.RateCarry
			add(contribution.SourceSettle, st.RawMilli)
			if s == 0 && rituals {
				add(contribution.SourceRitual, ritualMilli(cb, 30)) // steady state: long streak
			}
			n := int64(pr.sessions)
			if pr.grind {
				add(contribution.SourceLecture, cfg.Buckets[contribution.Activity].CapMilli/n)
				add(contribution.SourceFaction, cfg.Buckets[contribution.Community].CapMilli/n)
				continue
			}
			if sp := speedMilli(cb, pr, p); s == 0 && sp > 0 {
				add(contribution.SourceItem, sp)
			}
			if pr.lectures && s < 2 {
				add(contribution.SourceLecture, p.lectureMilli)
			}
			if pr.community && s < 2 {
				add(contribution.SourceFaction, p.communityPerLogin)
			}
			if pr.community && s == 0 && p.songTuMilli > 0 {
				add(contribution.SourceDualCultivation, p.songTuMilli)
			}
		}
		total += today.Total()
		maxDay = max(maxDay, float64(today.Total())/realm.PointMilli)
	}
	return float64(total) / days / realm.PointMilli, maxDay
}

// speedMilli is the speed-item points a profile uses per day: active profiles use the catalogue cap (balance doc
// assumption); `stones` profiles buy what their spirit stones (daily tasks + Tiên Duyên gift) pay for, capped.
func speedMilli(cb *content.Bundle, pr profile, p params) int64 {
	cfg := cb.Contribution
	switch {
	case pr.stones:
		stones, _ := taskIncome(cb, pr)
		stones += cb.VIP.Benefits(p.vip).DailyGift + p.giftStones
		return min(cfg.SpeedItemDailyMilli, stones*realm.PointMilli/p.speedPrice)
	case pr.lectures:
		return cfg.SpeedItemDailyMilli
	}
	return 0
}

// taskIncome is the spirit stones/day and the number of daily tasks a profile completes: check-in always, settle
// tasks when its logins are enough and far enough apart, lecture/incense tasks for active profiles. Breakthrough-attempt
// tasks are not counted (they depend on the stage; conservative for free players).
func taskIncome(cb *content.Bundle, pr profile) (stones, done int64) {
	gap := 24 * time.Hour / time.Duration(max(1, pr.sessions))
	for _, t := range cb.DailyTasks.All() {
		ok := false
		switch t.Kind {
		case dailytask.Checkin:
			ok = true
		case dailytask.Settle:
			ok = int32(pr.sessions) >= t.Target && gap >= time.Duration(t.MinElapsedSeconds)*time.Second
		case dailytask.LectureAttend, dailytask.Incense:
			ok = pr.lectures
		}
		if !ok {
			continue
		}
		done++
		for _, r := range t.Rewards {
			if r.Currency == "spirit_stone" {
				stones += r.Amount
			}
		}
	}
	return stones, done
}

func realmNames(cb *content.Bundle) []string {
	var names []string
	for _, r := range cb.Realms.Realms() {
		if r.Path == cb.Realms.DefaultPath() && !r.Reserved {
			names = append(names, r.Name)
		}
	}
	return names
}

func table(cb *content.Bundle, p params) {
	names := realmNames(cb)
	top := cb.Realms.LastPlayableTier()
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.AlignRight)
	hdr := []string{"profile", "pts/day"}
	for tier := 2; tier <= top; tier++ {
		hdr = append(hdr, fmt.Sprintf("T%d", tier))
	}
	hdr = append(hdr, "P10–P90 top", "trib fails", "pts lost")
	fmt.Fprintln(w, strings.Join(hdr, "\t")+"\t")
	for _, pr := range profiles {
		rs := simulate(cb, pr, p)
		row := []string{pr.name, fmt.Sprintf("%.1f", dailyPoints(cb, pr, p))}
		for tier := 2; tier <= top; tier++ {
			row = append(row, day(medianDay(rs, tier)))
		}
		f, l := avgFails(rs)
		row = append(row, fmt.Sprintf("%s–%s", day(pct(rs, top, 0.1)), day(pct(rs, top, 0.9))), fmt.Sprintf("%.1f", f), fmt.Sprintf("%.0f", l))
		fmt.Fprintln(w, strings.Join(row, "\t")+"\t")
	}
	check(w.Flush())
	var legend []string
	for i, n := range names {
		legend = append(legend, fmt.Sprintf("T%d %s", i+1, n))
	}
	fmt.Println("\n" + strings.Join(legend, " · "))
	fmt.Printf("Median day each realm is reached · Tiên Duyên %d · %d runs/profile · tribulation=%v · phù=%v\n", p.vip, p.runs, p.trib, p.phu)
	fmt.Printf("Assumptions: lecture %d pts × ≤2/day, community %d pts/login (≤2) + song tu %d pts/day (S*), speed items = catalogue cap. Target: S3 → %s ≈ day 329.\n",
		p.lectureMilli/realm.PointMilli, p.communityPerLogin/realm.PointMilli, p.songTuMilli/realm.PointMilli, names[len(names)-1])
	fmt.Println("'trib fails'/'pts lost' = mean per player over the whole run (failed tribulations, cultivation lost).")
}

func single(cb *content.Bundle, pr profile, p params) {
	rs := simulate(cb, pr, p)
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.AlignRight)
	fmt.Fprintf(w, "tier\trealm\tentry pts\tstages\tmedian day\tdays in prev realm\t\n")
	prev := 0.0
	for _, r := range cb.Realms.Realms() {
		if r.Path != cb.Realms.DefaultPath() || r.Reserved {
			continue
		}
		d := medianDay(rs, r.Tier)
		gapS := ""
		if r.Tier > 1 && d >= 0 && prev >= 0 {
			gapS = fmt.Sprintf("%.1f", d-prev)
		}
		fmt.Fprintf(w, "%d\t%s\t%d\t%d\t%s\t%s\t\n", r.Tier, r.Name, r.Stages[0].CumulativeBeforeMilli/realm.PointMilli, len(r.Stages), day(d), gapS)
		prev = d
	}
	check(w.Flush())
	f, l := avgFails(rs)
	fmt.Printf("\n%s · %.1f pts/day · Tiên Duyên %d · %d runs · tribulation fails %.2f, points lost %.0f (mean)\n",
		pr.name, dailyPoints(cb, pr, p), p.vip, p.runs, f, l)
}

// fairness checks the binding rule: Tiên Duyên L at 1 login/day ≤ free at 3 logins/day, every level and
// activity profile (points/day; daily-gift stones are not modelled here because speed items are already capped).
func fairness(cb *content.Bundle, p params) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.AlignRight)
	fmt.Fprintln(w, "activity\tlogins\tTD0\tTD4\tTD8\tTD12\t")
	ok := true
	for _, base := range []profile{{name: "idle"}, {name: "active", lectures: true}, {name: "social", lectures: true, community: true}} {
		free3 := 0.0
		for _, n := range []int{1, 2, 3} {
			pr := base
			pr.sessions = n
			row := []string{base.name, fmt.Sprint(n)}
			for _, v := range []int{0, 4, 8, 12} {
				q := p
				q.vip = v
				pts := dailyPoints(cb, pr, q)
				row = append(row, fmt.Sprintf("%.1f", pts))
				if n == 3 && v == 0 {
					free3 = pts
				}
			}
			fmt.Fprintln(w, strings.Join(row, "\t")+"\t")
		}
		for v := range cb.VIP.Tiers() {
			pr := base
			pr.sessions = 1
			q := p
			q.vip = v
			if got := dailyPoints(cb, pr, q); got > free3+1e-9 {
				ok = false
				fmt.Fprintf(os.Stderr, "FAIRNESS VIOLATION: %s TD%d@1 login = %.2f > free@3 = %.2f\n", base.name, v, got, free3)
			}
		}
	}
	check(w.Flush())
	fmt.Printf("\nPoints/day by Tiên Duyên level and logins (phù=%v). Rule TD_L@1 ≤ free@3 for every level: %v\n", p.phu, map[bool]string{true: "OK", false: "VIOLATED"}[ok])
}

// giftGap measures what the Tiên Duyên daily gift buys: idle players whose only speed items are the ones their
// spirit stones pay for (daily tasks + gift, -speedprice stones/point, catalogue speed cap). Checks (data-model §7.5
// V6, decisions Phase 0 #5): gift-driven speed ≤ 6 points/day at every level, TD_L@1 ≤ free@3, and no profile at any
// level ever credits more than the daily ceiling.
func giftGap(cb *content.Bundle, p params) {
	const maxGiftSpeed = 6.0
	top := cb.Realms.LastPlayableTier()
	names := realmNames(cb)
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.AlignRight)
	fmt.Fprintf(w, "idle+stones\tlogins\tstones TD0\tstones TD12\tspeed TD0\tspeed TD12\tpts/day TD0\tpts/day TD12\tTD12/TD0\t%s TD0\t%s TD12\t\n",
		names[len(names)-1], names[len(names)-1])
	ok := true
	var free3 float64
	for _, n := range []int{3, 2, 1} {
		pr := profile{name: fmt.Sprintf("I%d$", n), sessions: n, stones: true}
		q0, q12 := p, p
		q0.vip, q12.vip = 0, vip.MaxTier
		st, _ := taskIncome(cb, pr)
		pts0, pts12 := dailyPoints(cb, pr, q0), dailyPoints(cb, pr, q12)
		if n == 3 {
			free3 = pts0
		}
		d0, d12 := medianDay(simulate(cb, pr, q0), top), medianDay(simulate(cb, pr, q12), top)
		fmt.Fprintf(w, "I%d\t%d\t%d\t%d\t%.1f\t%.1f\t%.1f\t%.1f\t%.2f×\t%s\t%s\t\n", n, n, st, st+cb.VIP.Benefits(vip.MaxTier).DailyGift,
			float64(speedMilli(cb, pr, q0))/realm.PointMilli, float64(speedMilli(cb, pr, q12))/realm.PointMilli, pts0, pts12, pts12/pts0, day(d0), day(d12))
	}
	check(w.Flush())
	// Every level, every login count: gift speed ≤ 6, TD_L@1 ≤ free@3 (stones model), ceiling never exceeded.
	var worstGift, worstDay float64
	for _, n := range []int{1, 2, 3} {
		pr := profile{sessions: n, stones: true}
		for v := range cb.VIP.Tiers() {
			q0, q := p, p
			q0.vip, q.vip = 0, v
			gift := float64(speedMilli(cb, pr, q)-speedMilli(cb, pr, q0)) / realm.PointMilli
			worstGift = max(worstGift, gift)
			if gift > maxGiftSpeed {
				ok = false
				fmt.Fprintf(os.Stderr, "GIFT BUDGET VIOLATION: TD%d@%d gift speed %.1f > %.0f\n", v, n, gift, maxGiftSpeed)
			}
			if n == 1 && dailyPoints(cb, pr, q) > free3+1e-9 {
				ok = false
				fmt.Fprintf(os.Stderr, "FAIRNESS VIOLATION: idle+stones TD%d@1 > free@3\n", v)
			}
		}
	}
	ceiling := float64(cb.Contribution.DailyCeilingMilli) / realm.PointMilli
	for _, pr := range profiles {
		for v := range cb.VIP.Tiers() {
			q := p
			q.vip = v
			_, m := dailyStats(cb, pr, q)
			worstDay = max(worstDay, m)
			if m > ceiling {
				ok = false
				fmt.Fprintf(os.Stderr, "CEILING VIOLATION: %s TD%d day %.1f > %.0f\n", pr.name, v, m, ceiling)
			}
		}
	}
	fmt.Printf("\nTiên Duyên gift → speed items (%d stones/point, cap %d/day). Max gift-driven speed %.1f/day (≤ %.0f), "+
		"highest single day any profile/level %.1f (≤ %.0f), TD_L@1 ≤ free@3: %v\n", p.speedPrice,
		cb.Contribution.SpeedItemDailyMilli/realm.PointMilli, worstGift, maxGiftSpeed, worstDay, ceiling, map[bool]string{true: "OK", false: "VIOLATED"}[ok])
	if !ok {
		os.Exit(1)
	}
}

// duyenLevels estimates the Tiên Duyên level free players reach through live in-game điểm duyên only (vip.yaml earn;
// activities with live: false are not counted). Expected (balance §11.3): active free players level 3–4 by day 90.
func duyenLevels(cb *content.Bundle, p params) {
	t := cb.VIP
	perDay := func(pr profile) int64 {
		var total int64
		add := func(id string, times int64) {
			if a, ok := t.Activity(id); ok && a.Live {
				total += a.Points * min(times, a.DailyMaxTimes)
			}
		}
		if a, ok := t.Activity(vip.ActOnlineCultivation); ok && 24*time.Hour/time.Duration(pr.sessions) >= time.Duration(a.MinElapsedSeconds)*time.Second {
			add(vip.ActOnlineCultivation, int64(pr.sessions))
		}
		_, done := taskIncome(cb, pr)
		add(vip.ActDailyTask, done)
		if pr.lectures {
			lec := p.lectureMilli * int64(min(pr.sessions, 2))
			var marks int64
			for _, m := range cb.Lectures.VIPMilestonesMilli {
				if m <= lec {
					marks++
				}
			}
			add(vip.ActLecture, marks)
		}
		if pr.community && p.songTuMilli > 0 && p.songTuMilli >= cb.DaoLu.PayMilli(cb.DaoLu.SongTu.DuyenMinSeconds) {
			add(vip.ActDualCultivation, 1)
		}
		return min(total, t.EarnDailyCap)
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.AlignRight)
	fmt.Fprintln(w, "profile\tđiểm duyên/day\tTD1\tTD2\tTD3\tTD4\tTD5\tTD6\tlevel@90\tlevel@365\t")
	for _, pr := range profiles {
		if pr.grind {
			continue
		}
		d := perDay(pr)
		row := []string{pr.name, fmt.Sprint(d)}
		for lv := 1; lv <= 6; lv++ {
			req := t.Tiers()[lv].PointsRequired
			if d == 0 {
				row = append(row, "—")
				continue
			}
			row = append(row, fmt.Sprint((req+d-1)/d))
		}
		row = append(row, fmt.Sprint(t.TierForPoints(90*d)), fmt.Sprint(t.TierForPoints(365*d)))
		fmt.Fprintln(w, strings.Join(row, "\t")+"\t")
	}
	check(w.Flush())
	fmt.Printf("\nDays to each Tiên Duyên level from live in-game điểm duyên only (cap %d/day; thế lực, sự kiện not live yet; song tu counted for S*; "+
		"hộ pháp and thí luyện/bí cảnh trial_pass are live but not modelled here — they only shorten these days).\n", t.EarnDailyCap)
}

var epoch = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func day(d float64) string {
	if d < 0 {
		return "—"
	}
	if d < 1 {
		return fmt.Sprintf("%.0fh", d*24)
	}
	return fmt.Sprintf("%.0f", d)
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// stoneEconomy is the spirit-stone CREATION vs DESTRUCTION budget per profile and game day (decision G, data-model
// §8.2): created = daily tasks + Tiên Duyên gift; destroyed every day = speed items bought at the shop price (capped by
// the speed-item cap). Requirement items (gates, consumed per attempt) are not a non-progress sink (anti-cheat L3/M4).
//
// Recurring non-progress sinks (decisions 2026-11-07 "Linh thạch sinks"), simulated per day over -sinkdays days:
//   - cúng góp thế lực: the player donates -donateshare (bp) of the day's surplus, at most faction_works.yaml
//     daily_cap_per_character; once the cosmetics on sale are all owned, the whole surplus goes to it (up to the cap);
//   - cosmetics.yaml: the wallet buys the cheapest cosmetic ON SALE that day while -reserve stones stay unspent — every
//     year-round shop series, the era sets of the first era (the sim starts at launch) and one seasonal cave set per
//     91 days (the season changes).
//
// None of these sinks feeds progress: they are not inputs of the progression model above (table / fairness / pacing
// are computed without them), and their services write no points, merit, rank or điểm duyên (integration-tested).
func stoneEconomy(cb *content.Bundle, p params) {
	var reqCost int64
	var reqNames []string
	reqs := map[string]bool{}
	for _, s := range cb.Realms.Stages() {
		for _, r := range s.Breakthrough.Requirements {
			reqs[r.ItemID] = true
		}
	}
	for _, l := range cb.Shop.All() {
		d, _ := cb.Items.Get(l.ItemID)
		if reqs[l.ItemID] {
			reqCost += l.Price
			reqNames = append(reqNames, d.Name)
		}
	}
	stock := newCosmeticStock(cb)
	var rows []stoneRow
	for _, pr := range []profile{profiles[0], profiles[2], profiles[3], profiles[5], profiles[8], {name: "I1$", sessions: 1, stones: true}, {name: "I3$", sessions: 3, stones: true}} {
		for _, v := range []int{0, 12} {
			q := p
			q.vip = v
			created, _ := taskIncome(cb, pr)
			created += cb.VIP.Benefits(v).DailyGift
			destroyed := speedMilli(cb, pr, q) * p.speedPrice / realm.PointMilli
			rows = append(rows, stoneRow{pr.name, v, created, destroyed, created - destroyed})
		}
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.AlignRight)
	fmt.Fprintln(w, "profile\tTD\tcreated/day\tdestroyed/day (speed)\tsurplus/day\t")
	for _, r := range rows {
		fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%+d\t\n", r.name, r.td, r.created, r.destroyed, r.sp)
	}
	check(w.Flush())

	capDay := cb.FactionWorks.Settings.DailyCapPerCharacter
	fmt.Printf("\nRecurring non-progress sinks (decisions 2026-11-07): cosmetics on sale at launch %d stones in %d pieces "+
		"(the season set rotates every 91 days, unbought pieces leave with it; next season's set %d; full catalogue %d), faction donation ≤ %d/day.\n"+
		"ASSUMPTIONS: donate %d%% of the day's surplus while cosmetics remain on sale (then up to the cap); buy the cheapest "+
		"cosmetic on sale whenever the wallet stays ≥ %d stones (a retry reserve) after paying.\n",
		stock.total(0), len(stock.onSale(0)), stock.seasonTotal(91), cb.Cosmetics.ShopTotal(), capDay,
		p.donateShareBP/100, p.reserve)
	w = tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.AlignRight)
	fmt.Fprintln(w, "profile\tTD\tsurplus/day\tday\tΣ surplus\tΣ cosmetics (pieces)\tΣ donated\twallet\tabsorbed\tsteady residual/day\t")
	worst := int64(0)
	for _, r := range rows {
		var wallet, cos, don, sum int64
		owned := map[string]bool{}
		for d := 1; d <= p.sinkDays; d++ {
			sum += r.sp
			sale := stock.onSale(d)
			left := false
			for _, c := range sale {
				left = left || !owned[c.ID]
			}
			give := min(capDay, r.sp*p.donateShareBP/10_000)
			if !left {
				give = min(capDay, r.sp)
			}
			don += give
			wallet += r.sp - give
			for _, c := range sale { // cheapest first
				if !owned[c.ID] && wallet-c.Price >= p.reserve {
					owned[c.ID], wallet, cos = true, wallet-c.Price, cos+c.Price
				}
			}
			if d == p.sinkDays/2 || d == p.sinkDays {
				abs := "—"
				if sum > 0 {
					abs = fmt.Sprintf("%d%%", (cos+don)*100/sum)
				}
				residual := max(0, r.sp-capDay) // once the collection on sale is owned
				worst = max(worst, residual)
				fmt.Fprintf(w, "%s\t%d\t%+d\t%d\t%d\t%d (%d)\t%d\t%d\t%s\t%+d\t\n", r.name, r.td, r.sp, d, sum, cos, len(owned), don, wallet, abs, residual)
			}
		}
	}
	check(w.Flush())
	lo, hi := surplusRange(rows)
	fmt.Printf("Steady-state surplus BEFORE the new sinks: %+d…%+d stones/day; AFTER (collection owned, donating up to the cap): "+
		"at most %+d/day (only profiles creating more than %d/day above their speed items keep a residual).\n",
		lo, hi, worst, capDay)
	fmt.Printf("Other sinks: speed items %d stones/point (≤ %d points/day), requirement items %s = %d per attempt set "+
		"(gates, not a non-progress sink).\n",
		p.speedPrice, cb.Contribution.SpeedItemDailyMilli/realm.PointMilli, strings.Join(reqNames, " + "), reqCost)
}

// cosmeticStock is the shop cosmetics on sale per sim day: every series without a window, era series open in the
// first era (the sim starts at launch; their until_era is assumed not reached within the horizon), and the CURRENT
// season's set only. Later-era sets (Phong Thần…) are outside a 120-day horizon.
type cosmeticStock struct {
	base     []cosmetic.Cosmetic
	seasonal [][]cosmetic.Cosmetic
}

func newCosmeticStock(cb *content.Bundle) cosmeticStock {
	var st cosmeticStock
	for _, s := range cb.Cosmetics.AllSeries() {
		var items []cosmetic.Cosmetic
		for _, id := range s.Items {
			c, _ := cb.Cosmetics.Get(id)
			items = append(items, *c)
		}
		switch {
		case s.Source != cosmetic.SourceShop:
		case s.Season != "":
			st.seasonal = append(st.seasonal, items)
		case s.FromEra == "":
			st.base = append(st.base, items...)
		default:
			if seq, ok := cb.World.EraSeq(s.FromEra); ok && seq == 0 {
				st.base = append(st.base, items...)
			}
		}
	}
	return st
}

// onSale lists the cosmetics on sale ON `day`, cheapest first: the year-round stock plus ONLY the current season's set
// (launch season first, a new one every 91 days); a season's unbought pieces leave the shop when its window closes.
func (st cosmeticStock) onSale(day int) []cosmetic.Cosmetic {
	out := slices.Clone(st.base)
	if len(st.seasonal) > 0 {
		out = append(out, st.seasonal[(day/91)%len(st.seasonal)]...)
	}
	slices.SortStableFunc(out, func(a, b cosmetic.Cosmetic) int { return int(a.Price - b.Price) })
	return out
}

// seasonTotal is the price of the season set on sale on `day`.
func (st cosmeticStock) seasonTotal(day int) (sum int64) {
	if len(st.seasonal) == 0 {
		return 0
	}
	for _, c := range st.seasonal[(day/91)%len(st.seasonal)] {
		sum += c.Price
	}
	return sum
}

func (st cosmeticStock) total(day int) (sum int64) {
	for _, c := range st.onSale(day) {
		sum += c.Price
	}
	return sum
}

// stoneRow is one profile × Tiên Duyên level of the stone budget.
type stoneRow struct {
	name                   string
	td                     int
	created, destroyed, sp int64
}

func surplusRange(rows []stoneRow) (lo, hi int64) {
	lo, hi = rows[0].sp, rows[0].sp
	for _, r := range rows {
		lo, hi = min(lo, r.sp), max(hi, r.sp)
	}
	return lo, hi
}
