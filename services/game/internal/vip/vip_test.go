package vip

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
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
}

func TestTierForPoints(t *testing.T) {
	tb := shipped(t)
	cases := map[int64]int{0: 0, 9: 0, 10: 1, 99: 2, 100: 3, 49_999: 11, 50_000: 12, 1 << 40: 12}
	for pts, want := range cases {
		if got := tb.TierForPoints(pts); got != want {
			t.Errorf("TierForPoints(%d) = %d, want %d", pts, got, want)
		}
	}
}

func TestEffectiveOfflineCap(t *testing.T) {
	tb := shipped(t)
	h := time.Hour
	if got := tb.EffectiveOfflineCap(0, 4*h); got != 12*h {
		t.Errorf("free+phù = %s", got)
	}
	if got := tb.EffectiveOfflineCap(7, 4*h); got != 24*h { // 22h + 4h capped at daily ceiling
		t.Errorf("vip7+phù = %s", got)
	}
	if got := tb.EffectiveOfflineCap(12, 4*h); got != 32*h { // already above ceiling: no gain, no loss
		t.Errorf("vip12+phù = %s", got)
	}
}

func TestRejectsPowerPerks(t *testing.T) {
	tiers := make([]Tier, MaxTier+1)
	for i := range tiers {
		tiers[i] = Tier{Tier: i, PointsRequired: int64(i * 10), OfflineCapHours: 8}
	}
	tiers[5].Perks = []string{"cultivation_rate:10"}
	tiers[6].Perks = []string{"breakthrough_chance"}
	_, err := New(tiers, 36, 24, 1000)
	if err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("expected perk rejection, got %v", err)
	}
}
