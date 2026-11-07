package fortune

import (
	"math/rand/v2"
	"path/filepath"
	"strings"
	"testing"
)

func shipped(t *testing.T) *Catalogue {
	t.Helper()
	c, err := LoadFile(filepath.Join("..", "..", "..", "..", "content", "catalog", "fortunes.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestShippedFortunesAreValid(t *testing.T) {
	c := shipped(t)
	if c.MaxRateBonusBP() > MaxRateBonusBP {
		t.Fatal("bonus above guard")
	}
}

func TestRejectsBadFortunes(t *testing.T) {
	_, err := Load(strings.NewReader(`
version: 1
name: ""
outcomes:
  - { id: Bad, name: A, weight_bp: 0, rate_bonus_bp: 2500 }
  - { id: a, name: "", weight_bp: 5000, rate_bonus_bp: -1 }
  - { id: a, name: A, weight_bp: 4000, rate_bonus_bp: 0 }
`))
	if err == nil {
		t.Fatal("expected error")
	}
	for _, want := range []string{"empty name", "invalid id", "weight_bp", "rate_bonus_bp", "duplicate", "Σ weight_bp"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("missing %q in %v", want, err)
		}
	}
}

func TestPickBoundaries(t *testing.T) {
	c := shipped(t)
	var acc int64
	for _, o := range c.All() {
		if got := c.Pick(acc); got.ID != o.ID {
			t.Fatalf("roll %d → %s, want %s", acc, got.ID, o.ID)
		}
		acc += o.WeightBP
		if got := c.Pick(acc - 1); got.ID != o.ID {
			t.Fatalf("roll %d → %s, want %s", acc-1, got.ID, o.ID)
		}
	}
	if c.Pick(9999).ID != c.All()[len(c.All())-1].ID {
		t.Fatal("last roll")
	}
}

// The disclosed odds are the real odds: 200k seeded draws land within ±0.5 percentage point of each weight.
func TestDrawDistributionMatchesDisclosedOdds(t *testing.T) {
	c := shipped(t)
	rng := rand.New(rand.NewPCG(7, 11))
	const n = 200_000
	got := map[string]int{}
	for range n {
		_, o := c.Draw(rng)
		got[o.ID]++
	}
	for _, o := range c.All() {
		want := float64(o.WeightBP) / 10000
		if share := float64(got[o.ID]) / n; share < want-0.005 || share > want+0.005 {
			t.Errorf("%s: share %.4f, disclosed %.4f", o.ID, share, want)
		}
	}
	// Determinism: same seed → same sequence.
	a, b := rand.New(rand.NewPCG(1, 2)), rand.New(rand.NewPCG(1, 2))
	for range 100 {
		ra, _ := c.Draw(a)
		rb, _ := c.Draw(b)
		if ra != rb {
			t.Fatal("seeded draws differ")
		}
	}
}
