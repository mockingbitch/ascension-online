// Package factionwork is cúng góp thế lực (decisions 2026-11-07 "Linh thạch sinks", sink 3): members donate spirit
// stones to their faction's shared works (content/catalog/faction_works.yaml); completed stages unlock PURELY VISUAL
// faction decorations.
//
// INVARIANT (tested): a donation never adds faction merit, faction or global ranking points, điểm duyên or tu vi.
// This package never calls cultivation.CreditLocked, vip.EarnLocked or writes daily_contribution / leaderboard outbox
// events; the công đức bảng (donor list) is display only and is not an input of any ranking.
package factionwork

import (
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"slices"

	"gopkg.in/yaml.v3"

	"tiendao/game/internal/cosmetic"
)

// Currency donated.
const Currency = "spirit_stone"

const (
	maxGoal      = 10_000_000_000
	maxDailyCap  = 1_000_000
	maxBoardSize = 100
)

var slugRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// Decoration is the visual a completed stage unlocks (art slot cosmetics/<slug>).
type Decoration struct {
	Slug string `yaml:"slug" json:"slug"`
	Name string `yaml:"name" json:"name"`
}

// Stage completes when the work's own pool reaches At stones.
type Stage struct {
	At         int64      `json:"at"`
	Decoration Decoration `json:"decoration"`
}

// Work is one shared construction; Goal = last stage's At. Start = Σ goals of the earlier works.
type Work struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Goal        int64   `json:"goal"`
	Start       int64   `json:"start"`
	Stages      []Stage `json:"stages"`
}

// Settings are the donation rules.
type Settings struct {
	Currency             string `yaml:"currency" json:"currency"`
	DailyCapPerCharacter int64  `yaml:"daily_cap_per_character" json:"dailyCapPerCharacter"`
	MinAmount            int64  `yaml:"min_amount" json:"minAmount"`
	DonorBoardSize       int32  `yaml:"donor_board_size" json:"donorBoardSize"`
}

// Catalogue is immutable after load.
type Catalogue struct {
	Settings Settings
	works    []Work
}

type fileStage struct {
	At         int64          `yaml:"at"`
	Decoration Decoration     `yaml:"decoration"`
	Extra      map[string]any `yaml:",inline"`
}

type fileWork struct {
	ID          string         `yaml:"id"`
	Name        string         `yaml:"name"`
	Description string         `yaml:"description"`
	Stages      []fileStage    `yaml:"stages"`
	Extra       map[string]any `yaml:",inline"`
}

type file struct {
	Version  int        `yaml:"version"`
	Settings Settings   `yaml:"settings"`
	Works    []fileWork `yaml:"works"`
}

// LoadFile parses and validates faction_works.yaml.
func LoadFile(path string) (*Catalogue, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Load(f)
}

func rejectExtra(where string, extra map[string]any, fail func(string, ...any)) {
	keys := make([]string, 0, len(extra))
	for k := range extra {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		if cosmetic.EffectLike(k) {
			fail("%s: field %q is a gameplay effect — faction works are purely visual", where, k)
		} else {
			fail("%s: unknown field %q", where, k)
		}
	}
}

// Load parses and validates faction works YAML.
func Load(r io.Reader) (*Catalogue, error) {
	var raw file
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("factionwork: decode: %w", err)
	}
	var errs []error
	fail := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	if raw.Version != 1 {
		fail("unsupported version %d", raw.Version)
	}
	s := raw.Settings
	if s.Currency != Currency {
		fail("settings.currency must be %s", Currency)
	}
	if s.DailyCapPerCharacter < 1 || s.DailyCapPerCharacter > maxDailyCap {
		fail("settings.daily_cap_per_character must be within 1..%d", maxDailyCap)
	}
	if s.MinAmount < 1 || s.MinAmount > s.DailyCapPerCharacter {
		fail("settings.min_amount must be within 1..daily_cap_per_character")
	}
	if s.DonorBoardSize < 1 || s.DonorBoardSize > maxBoardSize {
		fail("settings.donor_board_size must be within 1..%d", maxBoardSize)
	}
	if len(raw.Works) == 0 {
		fail("no works")
	}
	c := &Catalogue{Settings: s}
	ids, slugs := map[string]bool{}, map[string]bool{}
	var start int64
	for i, fw := range raw.Works {
		where := fmt.Sprintf("work[%d] %q", i, fw.ID)
		rejectExtra(where, fw.Extra, fail)
		if !slugRe.MatchString(fw.ID) || ids[fw.ID] {
			fail("%s: invalid or duplicate id", where)
		}
		ids[fw.ID] = true
		if fw.Name == "" {
			fail("%s: empty name", where)
		}
		if len(fw.Stages) == 0 {
			fail("%s: no stages", where)
		}
		w := Work{ID: fw.ID, Name: fw.Name, Description: fw.Description, Start: start}
		var last int64
		for j, st := range fw.Stages {
			sw := fmt.Sprintf("%s stage[%d]", where, j)
			rejectExtra(sw, st.Extra, fail)
			if st.At <= last || st.At > maxGoal {
				fail("%s: at must increase and stay ≤ %d", sw, int64(maxGoal))
			}
			last = st.At
			if !slugRe.MatchString(st.Decoration.Slug) || slugs[st.Decoration.Slug] {
				fail("%s: invalid or duplicate decoration slug %q", sw, st.Decoration.Slug)
			}
			slugs[st.Decoration.Slug] = true
			if st.Decoration.Name == "" {
				fail("%s: decoration name required", sw)
			}
			w.Stages = append(w.Stages, Stage{At: st.At, Decoration: st.Decoration})
		}
		w.Goal = last
		start += last
		c.works = append(c.works, w)
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("factionwork: invalid catalogue: %w", errors.Join(errs...))
	}
	return c, nil
}

// All returns the works in build order. Do not mutate.
func (c *Catalogue) All() []Work { return c.works }

// CrossCheck: decoration slugs share the cosmetics art-slot namespace (cosmetics/<slug>).
func (c *Catalogue) CrossCheck(cos *cosmetic.Catalogue) error {
	var errs []error
	for _, w := range c.works {
		for _, st := range w.Stages {
			if cos.HasSlug(st.Decoration.Slug) {
				errs = append(errs, fmt.Errorf("faction work %q: decoration slug %q also used in cosmetics.yaml", w.ID, st.Decoration.Slug))
			}
		}
	}
	return errors.Join(errs...)
}

// ───── pure rules ─────

// Domain errors (mapped to HTTP in httpapi).
var (
	ErrNoCharacter  = errors.New("factionwork: character not found")
	ErrNoFaction    = errors.New("factionwork: not in a faction")
	ErrBadAmount    = errors.New("factionwork: amount out of range")
	ErrDailyCap     = errors.New("factionwork: daily donation cap reached")
	ErrInsufficient = errors.New("factionwork: not enough spirit stones")
	ErrKeyReuse     = errors.New("factionwork: idempotency key reused with a different amount")
)

// Check validates a donation intent. Order: amount range → daily cap → stones. Pure.
func (c *Catalogue) Check(amount, donatedToday, balance int64) error {
	switch {
	case amount < c.Settings.MinAmount || amount > c.Settings.DailyCapPerCharacter:
		return ErrBadAmount
	case donatedToday+amount > c.Settings.DailyCapPerCharacter:
		return ErrDailyCap
	case balance < amount:
		return ErrInsufficient
	}
	return nil
}

// Progress is a faction pool projected onto the works (derived on read, never stored).
type Progress struct {
	Total       int64    `json:"total"`
	WorkIndex   int      `json:"workIndex"`   // current work (len(works) = everything built)
	WorkDonated int64    `json:"workDonated"` // stones inside the current work
	Unlocked    []string `json:"unlocked"`    // decoration slugs, build order
}

// ProgressOf projects a pool total onto the works. Pure; stones past the last work stay in Total.
func (c *Catalogue) ProgressOf(total int64) Progress {
	p := Progress{Total: total, Unlocked: []string{}, WorkIndex: len(c.works)}
	found := false
	for i, w := range c.works {
		in := total - w.Start
		for _, st := range w.Stages {
			if in >= st.At {
				p.Unlocked = append(p.Unlocked, st.Decoration.Slug)
			}
		}
		if !found && in < w.Goal {
			p.WorkIndex, p.WorkDonated, found = i, max(0, in), true
		}
	}
	return p
}
