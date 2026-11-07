package vip

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tiendao/game/internal/db"
)

func shipped(t *testing.T) *Table {
	t.Helper()
	tb, err := LoadFile(filepath.Join("..", "..", "..", "..", "content", "catalog", "vip.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return tb
}

func TestShippedVIP(t *testing.T) {
	tb := shipped(t)
	if tb.OfflineCap(0) != 8*time.Hour {
		t.Fatalf("free cap = %s", tb.OfflineCap(0))
	}
	if tb.OfflineCap(99) != tb.OfflineCap(MaxTier) {
		t.Fatal("tier must clamp")
	}
	b := tb.Benefits(MaxTier)
	if b.AlchemySlots != 3 || len(b.Flags) != 1 {
		t.Fatalf("cumulative perks wrong: %+v", b)
	}
	if tb.DisplayName != "Tiên Duyên" || tb.PointName != "điểm duyên" || tb.PointsPer1000VND != 10 {
		t.Fatalf("display/pricing: %q %q %d", tb.DisplayName, tb.PointName, tb.PointsPer1000VND)
	}
	for _, tr := range tb.Tiers() {
		if strings.Contains(strings.ToUpper(tr.Name), "VIP") {
			t.Errorf("tier %d still named %q", tr.Tier, tr.Name)
		}
	}
	// Decision E thresholds.
	want := []int64{0, 30, 150, 500, 1200, 2500, 4500, 10_000, 25_000, 50_000, 100_000, 250_000, 500_000}
	for i, tr := range tb.Tiers() {
		if tr.PointsRequired != want[i] {
			t.Errorf("tier %d threshold %d, want %d", i, tr.PointsRequired, want[i])
		}
	}
	if MaxGrantDelta < tb.Tiers()[MaxTier].PointsRequired {
		t.Error("MaxGrantDelta must cover Tiên Duyên 12 in one grant")
	}
	// Decision D: ≤ 20/day; per-activity amounts.
	if tb.EarnDailyCap != 20 {
		t.Fatalf("earn cap %d", tb.EarnDailyCap)
	}
	d := map[string][2]int64{
		"online_cultivation": {1, 3}, "lecture_attendance": {3, 2}, "daily_task_claim": {1, 4}, "protector_assist": {2, 2},
		"dual_cultivation": {2, 1}, "faction_merit": {1, 3}, "trial_pass": {2, 2}, "event_participation": {3, 1},
	}
	if len(tb.Activities()) != len(d) {
		t.Fatalf("activities: %d", len(tb.Activities()))
	}
	for id, pm := range d {
		a, ok := tb.Activity(id)
		if !ok || a.Points != pm[0] || a.DailyMaxTimes != pm[1] {
			t.Errorf("activity %s = %+v, want %d × %d", id, a, pm[0], pm[1])
		}
	}
	// Daily gifts kept from v2 (D1a).
	gifts := []int64{0, 0, 50, 50, 50, 100, 100, 100, 200, 200, 200, 300, 300}
	for i, tr := range tb.Tiers() {
		if tr.DailyGift != gifts[i] {
			t.Errorf("tier %d gift %d, want %d", i, tr.DailyGift, gifts[i])
		}
	}
}

func TestTierForPoints(t *testing.T) {
	tb := shipped(t)
	cases := map[int64]int{0: 0, 29: 0, 30: 1, 149: 1, 150: 2, 499: 2, 500: 3, 499_999: 11, 500_000: 12, 1 << 40: 12}
	for pts, want := range cases {
		if got := tb.TierForPoints(pts); got != want {
			t.Errorf("TierForPoints(%d) = %d, want %d", pts, got, want)
		}
	}
}

func TestPointsForVND(t *testing.T) {
	tb := shipped(t)
	cases := map[int64]int64{0: 0, -1000: 0, 999: 9, 1000: 10, 50_000: 500, 50_000_000: 500_000}
	for vnd, want := range cases {
		if got := tb.PointsForVND(vnd); got != want {
			t.Errorf("PointsForVND(%d) = %d, want %d", vnd, got, want)
		}
	}
}

func TestEarnAmount(t *testing.T) {
	tb := shipped(t)
	cases := []struct {
		act           string
		times, earned int64
		want          int64
	}{
		{ActLecture, 0, 0, 3},
		{ActLecture, 1, 3, 3},
		{ActLecture, 2, 6, 0},   // per-activity daily max
		{ActDailyTask, 3, 3, 1}, // 4th claim still pays
		{ActDailyTask, 4, 4, 0},
		{ActLecture, 0, 18, 2}, // clipped to the daily cap
		{ActLecture, 0, 20, 0},
		{ActLecture, 0, 25, 0}, // over cap (should not happen) never negative
		{ActOnlineCultivation, 2, 19, 1},
	}
	for _, c := range cases {
		got, err := tb.EarnAmount(c.act, c.times, c.earned)
		if err != nil || got != c.want {
			t.Errorf("EarnAmount(%s, %d, %d) = %d, %v; want %d", c.act, c.times, c.earned, got, err, c.want)
		}
	}
	if _, err := tb.EarnAmount("nope", 0, 0); !errors.Is(err, ErrUnknownActivity) {
		t.Errorf("unknown activity: %v", err)
	}
	if _, err := tb.EarnAmount("event_participation", 0, 0); !errors.Is(err, ErrUnknownActivity) { // sự kiện: not live yet
		t.Errorf("non-live activity must be refused: %v", err)
	}
}

// Invariant V2: whatever order a player does things in, one game day never earns more than the cap, and no activity
// more than points × daily_max_times.
func TestEarnDayNeverExceedsCaps(t *testing.T) {
	tb := shipped(t)
	live := []string{}
	for _, a := range tb.Activities() {
		if a.Live {
			live = append(live, a.ID)
		}
	}
	for seed := range 200 {
		times := map[string]int64{}
		perAct := map[string]int64{}
		var earned int64
		for i := range 40 {
			id := live[(seed*7+i*(seed%5+1))%len(live)]
			pay, err := tb.EarnAmount(id, times[id], earned)
			if err != nil {
				t.Fatal(err)
			}
			if pay > 0 {
				times[id]++
				perAct[id] += pay
				earned += pay
			}
		}
		if earned > tb.EarnDailyCap {
			t.Fatalf("seed %d: earned %d > cap", seed, earned)
		}
		for id, p := range perAct {
			a, _ := tb.Activity(id)
			if p > a.Points*a.DailyMaxTimes {
				t.Fatalf("seed %d: %s earned %d", seed, id, p)
			}
		}
	}
}

func TestWithToday(t *testing.T) {
	tb := shipped(t)
	now := time.Date(2026, 11, 7, 3, 0, 0, 0, time.UTC)
	st := tb.withToday(tb.StatusFor(160), []db.ListVIPEarnDayRow{{ActivityID: ActLecture, Times: 2, Points: 6}, {ActivityID: ActDailyTask, Times: 1, Points: 1}}, now)
	if st.EarnedToday != 7 || st.EarnDailyCap != 20 || len(st.Activities) != len(tb.Activities()) || st.Benefits.Tier != 2 {
		t.Fatalf("status: %+v", st)
	}
	if st.Activities[1].ID != ActLecture || st.Activities[1].TimesToday != 2 || st.Activities[1].PointsToday != 6 {
		t.Fatalf("lecture row: %+v", st.Activities[1])
	}
	if st.ResetsAt.IsZero() || *st.NextTier != 3 || st.PointsToNext != 340 {
		t.Fatalf("next: %+v", st)
	}
}

func TestEffectiveOfflineCap(t *testing.T) {
	tb := shipped(t)
	h := time.Hour
	if got := tb.EffectiveOfflineCap(0, 4*h); got != 12*h {
		t.Errorf("free+phù = %s", got)
	}
	if got := tb.EffectiveOfflineCap(7, 4*h); got != 24*h { // 22h + 4h capped at daily ceiling
		t.Errorf("td7+phù = %s", got)
	}
	if got := tb.EffectiveOfflineCap(12, 4*h); got != 32*h { // already above ceiling: no gain, no loss
		t.Errorf("td12+phù = %s", got)
	}
}

func validConfig() Config {
	tiers := make([]Tier, MaxTier+1)
	for i := range tiers {
		tiers[i] = Tier{Tier: i, Name: "Tiên Duyên", PointsRequired: int64(i * 10), OfflineCapHours: 8}
	}
	acts := []Activity{}
	for _, id := range wiredActivities {
		acts = append(acts, Activity{ID: id, Source: "task", Name: "x", Points: 1, DailyMaxTimes: 1, Live: true})
	}
	return Config{Version: 3, DisplayName: "Tiên Duyên", PointName: "điểm duyên", PointsPer1000VND: 10,
		HardMaxOfflineCapHours: 36, DailyCultivationCeilingHours: 24, Earn: Earn{DailyCapPoints: 20, Activities: acts}, Tiers: tiers}
}

func TestRejectsPowerPerks(t *testing.T) {
	if _, err := New(validConfig()); err != nil {
		t.Fatalf("baseline config must load: %v", err)
	}
	for _, perk := range []string{"cultivation_rate:10", "breakthrough_chance", "tribulation_bonus_bp:300", "rank_points:5", "rate_bonus_bp"} {
		c := validConfig()
		c.Tiers[5].Perks = []string{perk}
		if _, err := New(c); err == nil || !strings.Contains(err.Error(), "not allowed") {
			t.Errorf("perk %q: expected rejection, got %v", perk, err)
		}
	}
}

func TestRejectsBadEarnConfig(t *testing.T) {
	cases := map[string]func(*Config){
		"vip name":         func(c *Config) { c.Tiers[3].Name = "VIP 3" },
		"no price":         func(c *Config) { c.PointsPer1000VND = 0 },
		"no display":       func(c *Config) { c.DisplayName = "" },
		"no cap":           func(c *Config) { c.Earn.DailyCapPoints = 0 },
		"topup source":     func(c *Config) { c.Earn.Activities[0].Source = "topup" },
		"admin source":     func(c *Config) { c.Earn.Activities[0].Source = "admin" },
		"zero points":      func(c *Config) { c.Earn.Activities[0].Points = 0 },
		"zero times":       func(c *Config) { c.Earn.Activities[0].DailyMaxTimes = 0 },
		"duplicate":        func(c *Config) { c.Earn.Activities[1].ID = c.Earn.Activities[0].ID },
		"bad id":           func(c *Config) { c.Earn.Activities[0].ID = "Bad-Id" },
		"wired not live":   func(c *Config) { c.Earn.Activities[0].Live = false },
		"wired missing":    func(c *Config) { c.Earn.Activities = c.Earn.Activities[1:] },
		"points above cap": func(c *Config) { c.Earn.Activities[0].Points = 21 },
	}
	for name, mut := range cases {
		c := validConfig()
		mut(&c)
		if _, err := New(c); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}
