package contribution

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var shipped = filepath.Join("..", "..", "..", "..", "content", "catalog", "contribution.yaml")

func load(t *testing.T) *Config {
	t.Helper()
	c, err := LoadFile(shipped)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestShippedContributionIsValid(t *testing.T) {
	c := load(t)
	if c.DailyCeilingMilli != 150_000 || c.Buckets[Idle].CapMilli != 75_000 || c.Buckets[Activity].CapMilli != 45_000 ||
		c.Buckets[Community].CapMilli != 30_000 || c.IdleRateMilliPerHour != 2083 || c.SpeedItemDailyMilli != 10_000 ||
		c.CatchUp.MultiplierBP != 20_000 || c.Grant.AfterServerDay != 14 || c.Grant.MedianShareBP != 5000 {
		t.Fatalf("unexpected shipped numbers: %+v", c)
	}
}

func TestApply(t *testing.T) {
	c := load(t)
	const big = int64(1) << 50
	tests := []struct {
		name    string
		src     Source
		raw     int64
		day     Day
		catchUp bool
		room    int64
		want    Applied
	}{
		{"plain", SourceSettle, 16_000, Day{}, false, big,
			Applied{Bucket: Idle, RawMilli: 16_000, MultiplierBP: 10_000, BoostedMilli: 16_000, CreditedMilli: 16_000}},
		{"idle bucket cap", SourceSettle, 20_000, Day{Idle: 70_000}, false, big,
			Applied{Bucket: Idle, RawMilli: 20_000, MultiplierBP: 10_000, BoostedMilli: 20_000, CreditedMilli: 5_000, ClippedCeilingMilli: 15_000}},
		{"daily ceiling", SourceLecture, 10_000, Day{Idle: 75_000, Activity: 40_000, Community: 30_000}, false, big,
			Applied{Bucket: Activity, RawMilli: 10_000, MultiplierBP: 10_000, BoostedMilli: 10_000, CreditedMilli: 5_000, ClippedCeilingMilli: 5_000}},
		{"catch-up x2 before caps", SourceSettle, 40_000, Day{}, true, big,
			Applied{Bucket: Idle, RawMilli: 40_000, MultiplierBP: 20_000, BoostedMilli: 80_000, CreditedMilli: 75_000, CatchUpBonusMilli: 35_000, ClippedCeilingMilli: 5_000}},
		{"bottleneck eats bonus first", SourceSettle, 10_000, Day{}, true, 12_000,
			Applied{Bucket: Idle, RawMilli: 10_000, MultiplierBP: 20_000, BoostedMilli: 20_000, CreditedMilli: 12_000, CatchUpBonusMilli: 2_000, ClippedBottleneckMilli: 8_000}},
		{"bottleneck does not consume ceiling", SourceSettle, 10_000, Day{}, false, 0,
			Applied{Bucket: Idle, RawMilli: 10_000, MultiplierBP: 10_000, BoostedMilli: 10_000, ClippedBottleneckMilli: 10_000}},
		{"speed item cap", SourceItem, 5_000, Day{Activity: 10_000, SpeedItem: 8_000}, false, big,
			Applied{Bucket: Activity, RawMilli: 5_000, MultiplierBP: 10_000, BoostedMilli: 5_000, CreditedMilli: 2_000, ClippedCeilingMilli: 3_000}},
		{"over-full day never negative", SourceSettle, 5_000, Day{Idle: 80_000}, false, big,
			Applied{Bucket: Idle, RawMilli: 5_000, MultiplierBP: 10_000, BoostedMilli: 5_000, ClippedCeilingMilli: 5_000}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := c.Apply(tc.src, tc.raw, tc.day, tc.catchUp, tc.room); got != tc.want {
				t.Fatalf("\n got %+v\nwant %+v", got, tc.want)
			}
		})
	}
}

// C1/C2/C6b: a day of many credits never exceeds caps; bonus ≤ points.
func TestApplyInvariants(t *testing.T) {
	c := load(t)
	var d Day
	srcs := []Source{SourceSettle, SourceLecture, SourceItem, SourceFaction, SourceEvent}
	for i := 0; i < 500; i++ {
		src := srcs[i%len(srcs)]
		a := c.Apply(src, int64(1+i*137%9000), d, i%3 == 0, int64(i*911%20000))
		if a.CreditedMilli < 0 || a.ClippedCeilingMilli < 0 || a.ClippedBottleneckMilli < 0 ||
			a.CreditedMilli+a.ClippedCeilingMilli+a.ClippedBottleneckMilli != a.BoostedMilli {
			t.Fatalf("accounting broken: %+v", a)
		}
		d = d.Add(a, src)
	}
	if d.Total() > c.DailyCeilingMilli || d.Idle > c.Buckets[Idle].CapMilli || d.Activity > c.Buckets[Activity].CapMilli ||
		d.Community > c.Buckets[Community].CapMilli || d.SpeedItem > c.SpeedItemDailyMilli || d.CatchUpBonus > d.Total() {
		t.Fatalf("caps violated: %+v", d)
	}
}

func TestCatchUpAndGrant(t *testing.T) {
	c := load(t)
	if c.CatchUpEligible(false, 30, 0, 1_000) || c.CatchUpEligible(true, 2, 0, 1_000) || c.CatchUpEligible(true, 30, 1_000, 1_000) {
		t.Fatal("catch-up must need started, day ≥ 3 and position < median")
	}
	if !c.CatchUpEligible(true, 3, 999, 1_000) {
		t.Fatal("eligible case")
	}
	if c.GrantMilli(true, 14, 1_000_000) != 0 || c.GrantMilli(false, 30, 1_000_000) != 0 {
		t.Fatal("no grant on day ≤ 14 or before launch")
	}
	if got := c.GrantMilli(true, 15, 1_000_000); got != 500_000 {
		t.Fatalf("grant %d", got)
	}
}

func TestRejectsBadContribution(t *testing.T) {
	b, _ := os.ReadFile(shipped)
	for _, tc := range []struct{ from, to, want string }{
		{"rate_milli_per_hour: 2083", "rate_milli_per_hour: 4000", "rate_milli_per_hour"},
		{"multiplier_bp: 20000", "multiplier_bp: 9000", "multiplier_bp"},
		{"  event:            { bucket: community }\n", "", `source "event"`},
		{"daily_cap_points: 75", "daily_cap_points: 200", "bucket \"idle\""},
	} {
		_, err := Load(strings.NewReader(strings.Replace(string(b), tc.from, tc.to, 1)))
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: got %v", tc.want, err)
		}
	}
}
