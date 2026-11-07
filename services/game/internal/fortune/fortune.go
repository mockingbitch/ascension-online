// Package fortune is "Gieo quẻ" (data-model §9.3/§9.4): once per game day a character draws one outcome from
// content/catalog/fortunes.yaml with DISCLOSED odds; the outcome grants a small idle-rate buff (rate_bonus_bp) from
// the draw until the end of that game day. The buff only changes idle settlement (cultivation.Settle), is capped by
// contribution.yaml idle.max_buff_bonus_bp and still bounded by the idle bucket cap and the 150/day ceiling.
//
// Not to be confused with the lecture "ngộ đạo" moment in Tử Tiêu Cung (internal/lecture).
package fortune

import (
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"

	"tiendao/game/internal/fixedpoint"
)

// RNG is injected (seeded in tests, crypto-seeded ChaCha8 in prod).
type RNG interface{ IntN(n int) int }

// Outcome is one possible draw.
type Outcome struct {
	ID          string `yaml:"id" json:"id"`
	Name        string `yaml:"name" json:"name"`
	WeightBP    int64  `yaml:"weight_bp" json:"weightBp"`        // disclosed odds; Σ = 10000
	RateBonusBP int64  `yaml:"rate_bonus_bp" json:"rateBonusBp"` // idle-rate bonus until the end of the game day
	Verse       string `yaml:"verse" json:"verse"`               // short original text shown on the result card
}

// Catalogue is immutable after load.
type Catalogue struct {
	Name     string
	outcomes []Outcome
	byID     map[string]*Outcome
}

type file struct {
	Version  int       `yaml:"version"`
	Name     string    `yaml:"name"`
	Outcomes []Outcome `yaml:"outcomes"`
}

var slugRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// MaxRateBonusBP is the hard guard of §9.3 (the effective cap is contribution idle.max_buff_bonus_bp, cross-checked).
const MaxRateBonusBP = 2000

// LoadFile parses and validates fortunes.yaml.
func LoadFile(path string) (*Catalogue, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Load(f)
}

// Load parses and validates fortune YAML.
func Load(r io.Reader) (*Catalogue, error) {
	var raw file
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("fortune: decode: %w", err)
	}
	var errs []error
	fail := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	if raw.Version != 1 {
		fail("unsupported version %d", raw.Version)
	}
	if raw.Name == "" {
		fail("empty name")
	}
	c := &Catalogue{Name: raw.Name, outcomes: raw.Outcomes, byID: map[string]*Outcome{}}
	var sum int64
	for i := range c.outcomes {
		o := &c.outcomes[i]
		if !slugRe.MatchString(o.ID) {
			fail("outcome[%d]: invalid id %q", i, o.ID)
		}
		if _, dup := c.byID[o.ID]; dup {
			fail("outcome %q: duplicate id", o.ID)
		}
		if o.Name == "" {
			fail("outcome %q: empty name", o.ID)
		}
		if o.WeightBP <= 0 || o.WeightBP > fixedpoint.BPScale {
			fail("outcome %q: weight_bp must be within 1..10000", o.ID)
		}
		if o.RateBonusBP < 0 || o.RateBonusBP > MaxRateBonusBP {
			fail("outcome %q: rate_bonus_bp must be within 0..%d", o.ID, MaxRateBonusBP)
		}
		sum += o.WeightBP
		c.byID[o.ID] = o
	}
	if sum != fixedpoint.BPScale {
		fail("Σ weight_bp = %d, must be exactly 10000 (odds are disclosed as-is)", sum)
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("fortune: invalid catalogue: %w", errors.Join(errs...))
	}
	return c, nil
}

// All returns outcomes in file order (the disclosed odds table). Do not mutate.
func (c *Catalogue) All() []Outcome { return c.outcomes }

// Get returns an outcome by ID.
func (c *Catalogue) Get(id string) (*Outcome, bool) { o, ok := c.byID[id]; return o, ok }

// MaxRateBonusBP returns the highest bonus any outcome grants.
func (c *Catalogue) MaxRateBonusBP() int64 {
	var m int64
	for _, o := range c.outcomes {
		m = max(m, o.RateBonusBP)
	}
	return m
}

// Pick maps a roll in [0, 10000) to an outcome by cumulative weight (file order). Pure.
func (c *Catalogue) Pick(rollBP int64) *Outcome {
	var acc int64
	for i := range c.outcomes {
		acc += c.outcomes[i].WeightBP
		if rollBP < acc {
			return &c.outcomes[i]
		}
	}
	return &c.outcomes[len(c.outcomes)-1]
}

// Draw rolls once with the injected RNG and returns the roll and outcome.
func (c *Catalogue) Draw(rng RNG) (int64, *Outcome) {
	roll := int64(rng.IntN(fixedpoint.BPScale))
	return roll, c.Pick(roll)
}
