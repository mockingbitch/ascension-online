package cultivation

import (
	"path/filepath"
	"testing"

	"tiendao/game/internal/content"
	"tiendao/game/internal/contribution"
)

func bundle(t *testing.T) *content.Bundle {
	t.Helper()
	b, err := content.LoadAll(filepath.Join("..", "..", "..", "..", "content", "catalog"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestCreditAutoAdvancesAndStopsAtBottleneck(t *testing.T) {
	b := bundle(t)
	cat, cfg := b.Realms, b.Contribution
	first := cat.First("tien")
	// 70 points of idle in a day (≤ 75 cap): Luyện Khí tang_1..tang_7 (2+4+6+8+11+14+16 = 61) → tang_8 with 9.
	r := Credit(cat, cfg, Progress{Stage: first}, contribution.Day{}, contribution.SourceSettle, 70_000, false)
	if r.After.Stage.ID != "luyen_khi.tang_8" || r.After.StoredMilli != 9_000 || len(r.Passed) != 7 || r.Applied.CreditedMilli != 70_000 {
		t.Fatalf("got %s %d passed=%d %+v", r.After.Stage.ID, r.After.StoredMilli, len(r.Passed), r.Applied)
	}
	// Last stage: bottleneck at 120% of 21 = 25.2 points.
	last, _ := cat.Stage("luyen_khi.tang_9")
	r = Credit(cat, cfg, Progress{Stage: last, StoredMilli: 25_000}, contribution.Day{}, contribution.SourceSettle, 10_000, false)
	if r.Applied.CreditedMilli != 200 || r.Applied.ClippedBottleneckMilli != 9_800 || r.After.StoredMilli != 25_200 || r.BottleneckRoom != 0 {
		t.Fatalf("bottleneck: %+v after %+v", r.Applied, r.After)
	}
}

func TestGrantReleaseNeverSkipsMajor(t *testing.T) {
	b := bundle(t)
	cat, cfg := b.Realms, b.Contribution
	first := cat.First("tien")
	r := Credit(cat, cfg, Progress{Stage: first, GrantReserveMilli: 5_000_000}, contribution.Day{}, contribution.SourceSettle, 1_000, false)
	last, _ := cat.Stage("luyen_khi.tang_9")
	if r.After.Stage != last || r.After.StoredMilli != last.CapMilli || r.BottleneckRoom != 0 {
		t.Fatalf("grant must fill the realm up to the cap: %s %d", r.After.Stage.ID, r.After.StoredMilli)
	}
	if r.GrantReleasedMilli+r.After.GrantReserveMilli != 5_000_000 || r.GrantReleasedMilli != 104_200-1_000 {
		t.Fatalf("release %d reserve %d", r.GrantReleasedMilli, r.After.GrantReserveMilli)
	}
	if r.Applied.CreditedMilli != 1_000 {
		t.Fatal("grant release must not touch the bucketed credit")
	}
}
