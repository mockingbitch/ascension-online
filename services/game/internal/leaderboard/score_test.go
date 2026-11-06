package leaderboard

import (
	"path/filepath"
	"testing"

	"tiendao/game/internal/realm"
)

func TestScoreOrdering(t *testing.T) {
	a := &realm.Stage{Tier: 2, StageOrder: 9, RequiredMilli: 100_000}
	b := &realm.Stage{Tier: 3, StageOrder: 1, RequiredMilli: 100_000}
	if Score(a, 120_000) >= Score(b, 0) {
		t.Fatal("higher tier must always outrank, even with overflow progress")
	}
	if Score(a, 10_000) >= Score(a, 11_000) {
		t.Fatal("more progress must rank higher")
	}
	if Score(a, 1<<60) != Score(a, 100_000) {
		t.Fatal("huge values saturate")
	}
}

// Score must be strictly increasing along each path ladder and equal across paths at the same position.
func TestScoreAcrossShippedLadder(t *testing.T) {
	c, err := realm.LoadFile(filepath.Join("..", "..", "..", "..", "content", "catalog", "realms.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	prev := -1.0
	for _, s := range c.Ladder("tien") {
		if lo := Score(s, 0); lo <= prev {
			t.Fatalf("%s: score %f not above previous %f", s.ID, lo, prev)
		}
		prev = Score(s, s.RequiredMilli-1)
		v, _ := c.Convert(s, "vu")
		if Score(v, 1234) != Score(s, 1234) {
			t.Fatalf("path-agnostic score broken at %s", s.ID)
		}
	}
}
