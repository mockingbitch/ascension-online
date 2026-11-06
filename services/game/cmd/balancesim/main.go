// Command balancesim simulates shared-world progression with the real catalogue (content.LoadAll) and the real
// rule functions: cultivation.Settle (idle rate + carry), cultivation.Credit (catch-up, bucket caps, 150/day
// ceiling, bình cảnh, auto-advance), breakthrough.Evaluate/Roll (pity, tribulation odds, guarantee, loss).
//
//	go run ./cmd/balancesim                      # archetype table (Tiên Duyên 0) + Tiên Duyên fairness check
//	go run ./cmd/balancesim -profile S3 -vip 4   # one archetype, per-realm table
//
// Every GAME number comes from content/catalog. The flags below are PLAYER-BEHAVIOUR assumptions from
// docs/design/balance-the-gioi-chung.md §1 (they move to lectures.yaml/shop.yaml when those systems ship):
//   - -lecture: points per attended lecture (2 slots/day; a 1-login player catches 1);
//   - -community: community points per login (≤ 2 logins counted, bucket cap still applies);
//   - active players (A*, S*) use the full speed-item cap (contribution.yaml speed_items) once per day.
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
	"tiendao/game/internal/cultivation"
	"tiendao/game/internal/realm"
)

type profile struct {
	name      string
	sessions  int
	lectures  bool // attends lectures (+ uses speed items)
	community bool
	grind     bool // fills every bucket to its cap ("cày trần")
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
	phu               bool
	phuItem           string
	trib              bool
}

type runResult struct {
	reached   []float64 // per tier (index tier-1): day reached, -1 if not
	fails     int
	lostMilli int64
	guarantee int
}

func main() {
	dir := flag.String("content", "../../content/catalog", "content/catalog dir")
	prof := flag.String("profile", "", "single archetype (I1..S3, G); empty = archetype table + fairness check")
	vipTier := flag.Int("vip", 0, "Tiên Duyên level")
	runs := flag.Int("runs", 300, "simulated players per profile")
	maxDays := flag.Int("maxdays", 1500, "stop after N days")
	lecture := flag.Int64("lecture", 10, "ASSUMPTION: points per lecture attended")
	community := flag.Int64("community", 10, "ASSUMPTION: community points per login (max 2 logins)")
	phu := flag.Bool("phu", true, "players use Tụ Linh Phù (daily check-in reward) once per day")
	trib := flag.Bool("trib", true, "tribulation enabled (false = baseline without tribulation)")
	flag.Parse()

	cb, err := content.LoadAll(*dir)
	check(err)
	p := params{vip: *vipTier, runs: *runs, maxDays: *maxDays, lectureMilli: *lecture * realm.PointMilli,
		communityPerLogin: *community * realm.PointMilli, phu: *phu, phuItem: "tu_linh_phu", trib: *trib}

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
	out := make([]runResult, 0, p.runs)
	for range p.runs {
		res := runResult{reached: make([]float64, cat.LastPlayableTier())}
		for i := range res.reached {
			res.reached[i] = -1
		}
		res.reached[0] = 0
		prog := cultivation.Progress{Stage: cat.First(path)}
		var carry, streak int64
		var cooldown *time.Time
		var last time.Time
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
					RateMilliPerHour: cfg.IdleRateMilliPerHour, OfflineCap: capDur})
				last, carry = st.LastCultivatedAt, st.RateCarry
				credit(contribution.SourceSettle, st.RawMilli)
				n := int64(pr.sessions)
				switch {
				case pr.grind:
					credit(contribution.SourceLecture, cfg.Buckets[contribution.Activity].CapMilli/n)
					credit(contribution.SourceFaction, cfg.Buckets[contribution.Community].CapMilli/n)
				default:
					if pr.lectures {
						if s == 0 {
							credit(contribution.SourceItem, cfg.SpeedItemDailyMilli)
						}
						if s < 2 {
							credit(contribution.SourceLecture, p.lectureMilli)
						}
					}
					if pr.community && s < 2 {
						credit(contribution.SourceFaction, p.communityPerLogin)
					}
				}
				// Break through while online, as soon as allowed.
				for {
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
	for day := 1; day <= days; day++ {
		var today contribution.Day
		phuLeft := p.phu
		add := func(src contribution.Source, raw int64) {
			a := cfg.Apply(src, raw, today, false, 1<<50)
			today = today.Add(a, src)
		}
		for s := 0; s < pr.sessions; s++ {
			now := epoch.Add(time.Duration(day)*24*time.Hour + time.Duration(s)*gap)
			capDur := baseCap
			if phuLeft && now.Sub(last) > baseCap {
				capDur, phuLeft = phuCap, false
			}
			st := cultivation.Settle(cultivation.SettleInput{Now: now, LastCultivatedAt: last, RateCarry: carry,
				RateMilliPerHour: cfg.IdleRateMilliPerHour, OfflineCap: capDur})
			last, carry = st.LastCultivatedAt, st.RateCarry
			add(contribution.SourceSettle, st.RawMilli)
			n := int64(pr.sessions)
			if pr.grind {
				add(contribution.SourceLecture, cfg.Buckets[contribution.Activity].CapMilli/n)
				add(contribution.SourceFaction, cfg.Buckets[contribution.Community].CapMilli/n)
				continue
			}
			if pr.lectures && s == 0 {
				add(contribution.SourceItem, cfg.SpeedItemDailyMilli)
			}
			if pr.lectures && s < 2 {
				add(contribution.SourceLecture, p.lectureMilli)
			}
			if pr.community && s < 2 {
				add(contribution.SourceFaction, p.communityPerLogin)
			}
		}
		total += today.Total()
	}
	return float64(total) / days / realm.PointMilli
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
	fmt.Printf("Assumptions: lecture %d pts × ≤2/day, community %d pts/login (≤2), speed items = catalogue cap. Target: S3 → %s ≈ day 329.\n",
		p.lectureMilli/realm.PointMilli, p.communityPerLogin/realm.PointMilli, names[len(names)-1])
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
