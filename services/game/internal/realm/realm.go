// Package realm is the in-memory, read-only realm/stage catalogue built from content/catalog/realms.yaml (v2).
//
// v2 model (docs/design/data-model-the-gioi-chung.md §3):
//   - numbers are defined once per TIER; each PATH (tiên, Vu) maps one realm onto every playable tier;
//   - stage requirements are generated from cumulative entry points and are in milli (1 point = 1000);
//   - minor stages auto-advance when credited; only a realm's LAST stage has the 120% bottleneck cap and a
//     player-initiated major breakthrough (a tribulation from `tribulation.from_tier` on).
//
// The catalogue is immutable after Load; share one *Catalogue across goroutines freely.
package realm

import (
	"fmt"

	"tiendao/game/internal/fixedpoint"
)

// PathID identifies a cultivation path ("tien" | "vu").
type PathID string

// BreakthroughKind is how a stage is left.
type BreakthroughKind string

const (
	// Auto: minor stage, advanced automatically when its requirement is met (v2).
	Auto BreakthroughKind = "auto"
	// Major: leaving a realm, player-initiated (tribulation from tribulation.from_tier).
	Major BreakthroughKind = "major"
	// Minor is kept for historical breakthrough_attempts rows only; v2 never produces it.
	Minor BreakthroughKind = "minor"
)

// PointMilli is the number of milli in one điểm tu vi.
const PointMilli = 1000

// ItemRequirement is an item consumed by a breakthrough attempt.
type ItemRequirement struct {
	ItemID   string `yaml:"item_id" json:"itemId"`
	Quantity int64  `yaml:"quantity" json:"quantity"`
}

// BreakthroughRule describes how to leave the last stage of a realm. For tribulation tiers the odds come from
// Tribulation and SuccessBP/PityStepBP/CooldownSeconds stay 0.
type BreakthroughRule struct {
	SuccessBP       int64             `yaml:"success_bp" json:"successBp"`
	PityStepBP      int64             `yaml:"pity_step_bp" json:"pityStepBp"`
	CooldownSeconds int64             `yaml:"cooldown_seconds" json:"cooldownSeconds"`
	Requirements    []ItemRequirement `yaml:"requirements" json:"requirements"`
}

// SurplusBonus rewards cultivation stored above the requirement (bình cảnh).
type SurplusBonus struct {
	BPPerSurplusBP int64 `yaml:"bp_per_surplus_bp" json:"bpPerSurplusBp"`
	MaxBP          int64 `yaml:"max_bp" json:"maxBp"`
}

// Tribulation is the rule for major breakthroughs from FromTier on (D3 + bổ sung F).
type Tribulation struct {
	FromTier        int               `yaml:"from_tier" json:"fromTier"`
	BaseBP          int64             `yaml:"base_bp" json:"baseBp"`
	StepBP          int64             `yaml:"step_bp" json:"stepBp"`
	FloorBP         int64             `yaml:"floor_bp" json:"floorBp"`
	LossBP          int64             `yaml:"loss_bp" json:"lossBp"`
	CooldownSeconds int64             `yaml:"cooldown_seconds" json:"cooldownSeconds"`
	GuaranteeAfter  int               `yaml:"guarantee_after" json:"guaranteeAfter"`
	SurplusBonus    SurplusBonus      `yaml:"surplus_bonus" json:"surplusBonus"`
	ItemBonusMaxBP  int64             `yaml:"item_bonus_max_bp" json:"itemBonusMaxBp"`
	ProtectorPerBP  int64             `yaml:"protector_bonus_per_member_bp" json:"protectorBonusPerMemberBp"`
	ProtectorMaxBP  int64             `yaml:"protector_bonus_max_bp" json:"protectorBonusMaxBp"`
	MaxTotalBonusBP int64             `yaml:"max_total_bonus_bp" json:"maxTotalBonusBp"`
	MaxSuccessBP    int64             `yaml:"max_success_bp" json:"maxSuccessBp"`
	Names           map[PathID]string `yaml:"names" json:"names"`
	// QuoteWindowSeconds bounds the preview→attempt drift accepted by the acceptLossMilli guard.
	QuoteWindowSeconds int64 `yaml:"quote_window_seconds" json:"quoteWindowSeconds"`
}

// Decision-log bounds for the tribulation (docs/decisions.md 2026-11-06): the loader refuses other values so a
// YAML edit cannot silently change a binding product decision.
const (
	// DecisionTribulationLossBP: "Phase 0" #2 — failure loses 25% of the current stage.
	DecisionTribulationLossBP = 2500
	// DecisionTribulationMaxSuccessBP: "bổ sung" F — final odds ≤ 90% except the guaranteed attempt.
	DecisionTribulationMaxSuccessBP = 9000
	// MaxQuoteWindowSeconds bounds the quote guard window.
	MaxQuoteWindowSeconds = 3600
)

// Visual is presentation metadata; the frontend maps these keys to design tokens.
type Visual struct {
	Badge string `yaml:"badge" json:"badge"`
	Aura  string `yaml:"aura" json:"aura"`
	// FxTier (1–6) comes from the realm's TIER (tiers[].visual.fx_tier), so both paths share it. Display only.
	FxTier int `yaml:"-" json:"fxTier"`
}

// MaxFxTier is the top effect tier (decisions 2026-11-09: six tiers).
const MaxFxTier = 6

// Path is a cultivation path.
type Path struct {
	ID              PathID `json:"id"`
	Name            string `json:"name"`
	Default         bool   `json:"default"`
	NamePlaceholder bool   `json:"namePlaceholder,omitempty"`
}

// Stage is one progression step. ID is global: "<realm_id>.<stage_id>".
type Stage struct {
	ID                    string           `json:"id"`
	RealmID               string           `json:"realmId"`
	Path                  PathID           `json:"path"`
	Tier                  int              `json:"tier"`
	StageOrder            int              `json:"stageOrder"` // 1-based inside realm
	StageCount            int              `json:"stageCount"`
	Index                 int              `json:"index"` // 0-based ordinal within its path (-1 for reserved tiers)
	Name                  string           `json:"name"`
	RequiredMilli         int64            `json:"requiredMilli"`
	CapMilli              int64            `json:"capMilli"` // > 0 only on a realm's last stage (bình cảnh)
	CumulativeBeforeMilli int64            `json:"cumulativeBeforeMilli"`
	NextKind              BreakthroughKind `json:"nextKind,omitempty"` // auto | major | "" (top of the ladder)
	Tribulation           bool             `json:"tribulation"`
	Breakthrough          BreakthroughRule `json:"breakthrough"` // major rule (last stage only)
	Reserved              bool             `json:"reserved,omitempty"`
}

// IsLastOfRealm reports whether s is the last stage of its realm.
func (s *Stage) IsLastOfRealm() bool { return s.StageOrder == s.StageCount }

// Realm groups stages.
type Realm struct {
	ID              string   `json:"id"`
	Path            PathID   `json:"path"`
	Tier            int      `json:"tier"`
	Name            string   `json:"name"`
	NamePlaceholder bool     `json:"namePlaceholder,omitempty"`
	Title           string   `json:"title"`
	Visual          Visual   `json:"visual"`
	Reserved        bool     `json:"reserved,omitempty"`
	Stages          []*Stage `json:"stages"`
}

// Settings are catalogue-wide knobs.
type Settings struct {
	OverflowCapBP  int64 `yaml:"overflow_cap_bp" json:"overflowCapBp"`
	PityMaxBonusBP int64 `yaml:"pity_max_bonus_bp" json:"pityMaxBonusBp"`
}

// Catalogue is the immutable realm catalogue.
type Catalogue struct {
	Version          int
	Settings         Settings
	paths            []Path
	defaultPath      PathID
	trib             Tribulation
	lastPlayableTier int
	realms           []*Realm
	all              []*Stage            // every playable stage, path by path
	ladder           map[PathID][]*Stage // playable stages per path, in order
	byID             map[string]*Stage   // playable stages only
	byPathTier       map[PathID]map[int]*Realm
}

// Paths returns the paths in catalogue order.
func (c *Catalogue) Paths() []Path { return c.paths }

// DefaultPath is the path new characters start on.
func (c *Catalogue) DefaultPath() PathID { return c.defaultPath }

// HasPath reports whether p is a known path.
func (c *Catalogue) HasPath(p PathID) bool { _, ok := c.ladder[p]; return ok }

// Realms returns all realms (both paths, incl. reserved) in catalogue order. Do not mutate.
func (c *Catalogue) Realms() []*Realm { return c.realms }

// Stages returns every playable stage of every path. Do not mutate.
func (c *Catalogue) Stages() []*Stage { return c.all }

// Ladder returns the playable stages of a path in order. Do not mutate.
func (c *Catalogue) Ladder(p PathID) []*Stage { return c.ladder[p] }

// LastPlayableTier is the top tier a player can hold (Chuẩn Thánh).
func (c *Catalogue) LastPlayableTier() int { return c.lastPlayableTier }

// Tribulation returns the tribulation rule.
func (c *Catalogue) Tribulation() Tribulation { return c.trib }

// First returns the starting stage of a path.
func (c *Catalogue) First(p PathID) *Stage {
	l := c.ladder[p]
	if len(l) == 0 {
		return nil
	}
	return l[0]
}

// Stage looks up a playable stage by global ID.
func (c *Catalogue) Stage(id string) (*Stage, error) {
	s, ok := c.byID[id]
	if !ok {
		return nil, fmt.Errorf("realm: unknown stage %q", id)
	}
	return s, nil
}

// Next returns the stage after s within its path, or nil at the top of the ladder.
func (c *Catalogue) Next(s *Stage) *Stage {
	l := c.ladder[s.Path]
	if s.Index < 0 || s.Index+1 >= len(l) {
		return nil
	}
	return l[s.Index+1]
}

// RealmOf returns the realm of a stage.
func (c *Catalogue) RealmOf(s *Stage) *Realm { return c.byPathTier[s.Path][s.Tier] }

// StageAt returns the stage of a path at (tier, stageOrder).
func (c *Catalogue) StageAt(p PathID, tier, stageOrder int) (*Stage, bool) {
	r, ok := c.byPathTier[p][tier]
	if !ok || r.Reserved || stageOrder < 1 || stageOrder > len(r.Stages) {
		return nil, false
	}
	return r.Stages[stageOrder-1], true
}

// Convert maps s to the same (tier, stageOrder) on another path. Requirements are identical by construction,
// so stored cultivation is kept as-is by the caller.
func (c *Catalogue) Convert(s *Stage, to PathID) (*Stage, error) {
	t, ok := c.StageAt(to, s.Tier, s.StageOrder)
	if !ok {
		return nil, fmt.Errorf("realm: no stage on path %q at tier %d stage %d", to, s.Tier, s.StageOrder)
	}
	return t, nil
}

// FirstOfRealm returns the first stage of s's realm.
func (c *Catalogue) FirstOfRealm(s *Stage) *Stage { return c.ladder[s.Path][s.Index-(s.StageOrder-1)] }

// LastOfRealm returns the last stage of s's realm.
func (c *Catalogue) LastOfRealm(s *Stage) *Stage {
	return c.ladder[s.Path][s.Index+(s.StageCount-s.StageOrder)]
}

// BottleneckRoomMilli is how many milli can still be credited from stage s holding `stored` before the realm's
// last stage reaches its cap: Σ required(s … last−1) − stored + last.CapMilli (never negative).
func (c *Catalogue) BottleneckRoomMilli(s *Stage, stored int64) int64 {
	last := c.LastOfRealm(s)
	room := last.CumulativeBeforeMilli - s.CumulativeBeforeMilli - stored + last.CapMilli
	return max(0, room)
}

// Advance auto-advances minor stages: while s is not its realm's last stage and stored ≥ required, it moves to
// the next stage carrying the remainder. Returns the final stage, its stored cultivation and the stages left.
func (c *Catalogue) Advance(s *Stage, stored int64) (*Stage, int64, []*Stage) {
	var passed []*Stage
	for !s.IsLastOfRealm() && stored >= s.RequiredMilli {
		stored -= s.RequiredMilli
		passed = append(passed, s)
		s = c.Next(s)
	}
	return s, stored, passed
}

// Position is the ladder position in milli (cumulative points of the stage start + stored).
func (c *Catalogue) Position(s *Stage, stored int64) int64 {
	return fixedpoint.SatAdd(s.CumulativeBeforeMilli, stored)
}

// TribulationBaseBP = max(floor, base − step·(tier − from_tier)) for a tribulation stage; 0 otherwise.
func (c *Catalogue) TribulationBaseBP(s *Stage) int64 {
	if !s.Tribulation {
		return 0
	}
	t := c.trib
	return max(t.FloorBP, t.BaseBP-t.StepBP*int64(s.Tier-t.FromTier))
}
