// Package item is the immutable item-definition catalogue from content/catalog/items.yaml.
package item

import (
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Category groups items for UI and rules.
type Category string

// Rarity maps to design tokens --color-rarity-<rarity>.
type Rarity string

var (
	categories = map[Category]bool{"pill": true, "herb": true, "material": true, "fragment": true, "consumable": true, "currency_bundle": true,
		"cosmetic": true, "artifact": true}
	rarities = map[Rarity]bool{"pham": true, "linh": true, "huyen": true, "dia": true, "thien": true, "tien": true}
	slugRe   = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
)

// CategoryArtifact is pháp bảo: tribulation odds, bí cảnh drops only (Phase 3.4).
const CategoryArtifact Category = "artifact"

// Def is one item definition.
type Def struct {
	ID          string   `yaml:"id" json:"id"`
	Name        string   `yaml:"name" json:"name"`
	Category    Category `yaml:"category" json:"category"`
	Rarity      Rarity   `yaml:"rarity" json:"rarity"`
	Tradable    bool     `yaml:"tradable" json:"tradable"`
	Description string   `yaml:"description" json:"description"`
	// Effect parameters (optional, category-specific; data-model §8.1). EffectKeys lists the ones set.
	OfflineCapBonusHours int   `yaml:"offline_cap_bonus_hours,omitempty" json:"offlineCapBonusHours,omitempty"`
	InstantPointsMilli   int64 `yaml:"instant_points_milli,omitempty" json:"instantPointsMilli,omitempty"` // speed item: points via CreditLocked(source item)
	MaxUsePerGameDay     int   `yaml:"max_use_per_game_day,omitempty" json:"maxUsePerGameDay,omitempty"`   // per character per game day of USE
	// Odds items (Phase 3 pháp bảo): never sellable for stones or money (decisions D1a, Phase 0 #11; shop.CrossCheck).
	BreakthroughBonusBP int64 `yaml:"breakthrough_bonus_bp,omitempty" json:"breakthroughBonusBp,omitempty"`
	TribulationBonusBP  int64 `yaml:"tribulation_bonus_bp,omitempty" json:"tribulationBonusBp,omitempty"`
	// UniquePerCharacter: a character may own at most one (cosmetics); the shop refuses a second purchase.
	UniquePerCharacter bool `yaml:"unique_per_character,omitempty" json:"uniquePerCharacter,omitempty"`
}

// Effect keys (YAML names) reported by EffectKeys.
const (
	EffectOfflineCapBonus = "offline_cap_bonus_hours"
	EffectInstantPoints   = "instant_points_milli"
	EffectMaxUsePerDay    = "max_use_per_game_day"
	EffectBreakthroughBP  = "breakthrough_bonus_bp"
	EffectTribulationBP   = "tribulation_bonus_bp"
	maxInstantPointsMilli = 50_000 // typo guard; the real bound is contribution speed_items (content cross-check)
	maxUsesPerDayGuard    = 100
	maxOddsBonusBP        = 10_000
)

// EffectKeys returns the effect fields this item sets (empty for plain materials, requirement items, cosmetics).
func (d *Def) EffectKeys() []string {
	var keys []string
	if d.OfflineCapBonusHours != 0 {
		keys = append(keys, EffectOfflineCapBonus)
	}
	if d.InstantPointsMilli != 0 {
		keys = append(keys, EffectInstantPoints)
	}
	if d.MaxUsePerGameDay != 0 {
		keys = append(keys, EffectMaxUsePerDay)
	}
	if d.BreakthroughBonusBP != 0 {
		keys = append(keys, EffectBreakthroughBP)
	}
	if d.TribulationBonusBP != 0 {
		keys = append(keys, EffectTribulationBP)
	}
	return keys
}

// RaisesOdds reports whether the item changes breakthrough or tribulation odds.
func (d *Def) RaisesOdds() bool { return d.BreakthroughBonusBP != 0 || d.TribulationBonusBP != 0 }

// Catalogue is immutable after load.
type Catalogue struct {
	items []Def
	byID  map[string]*Def
}

type file struct {
	Version int   `yaml:"version"`
	Items   []Def `yaml:"items"`
}

// LoadFile parses and validates items.yaml.
func LoadFile(path string) (*Catalogue, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Load(f)
}

// Load parses and validates item YAML.
func Load(r io.Reader) (*Catalogue, error) {
	var raw file
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("item: decode: %w", err)
	}
	var errs []error
	if raw.Version != 1 {
		errs = append(errs, fmt.Errorf("unsupported version %d", raw.Version))
	}
	c := &Catalogue{items: raw.Items, byID: map[string]*Def{}}
	for i := range c.items {
		d := &c.items[i]
		if !slugRe.MatchString(d.ID) {
			errs = append(errs, fmt.Errorf("item[%d]: invalid id %q", i, d.ID))
		}
		if _, dup := c.byID[d.ID]; dup {
			errs = append(errs, fmt.Errorf("item %q: duplicate id", d.ID))
		}
		if d.Name == "" {
			errs = append(errs, fmt.Errorf("item %q: empty name", d.ID))
		}
		if !categories[d.Category] {
			errs = append(errs, fmt.Errorf("item %q: unknown category %q", d.ID, d.Category))
		}
		if !rarities[d.Rarity] {
			errs = append(errs, fmt.Errorf("item %q: unknown rarity %q", d.ID, d.Rarity))
		}
		if d.OfflineCapBonusHours < 0 || d.OfflineCapBonusHours > 24 {
			errs = append(errs, fmt.Errorf("item %q: offline_cap_bonus_hours out of range", d.ID))
		}
		if d.InstantPointsMilli < 0 || d.InstantPointsMilli > maxInstantPointsMilli {
			errs = append(errs, fmt.Errorf("item %q: instant_points_milli out of range 0..%d", d.ID, maxInstantPointsMilli))
		}
		if d.MaxUsePerGameDay < 0 || d.MaxUsePerGameDay > maxUsesPerDayGuard {
			errs = append(errs, fmt.Errorf("item %q: max_use_per_game_day out of range", d.ID))
		}
		if d.InstantPointsMilli > 0 && d.MaxUsePerGameDay < 1 { // §8.1: speed items must carry a daily use limit
			errs = append(errs, fmt.Errorf("item %q: instant_points_milli requires max_use_per_game_day ≥ 1", d.ID))
		}
		if d.MaxUsePerGameDay > 0 && d.InstantPointsMilli == 0 {
			errs = append(errs, fmt.Errorf("item %q: max_use_per_game_day without a usable effect", d.ID))
		}
		if d.InstantPointsMilli > 0 && d.OfflineCapBonusHours > 0 {
			errs = append(errs, fmt.Errorf("item %q: one usable effect per item", d.ID))
		}
		for _, bp := range []int64{d.BreakthroughBonusBP, d.TribulationBonusBP} {
			if bp < 0 || bp > maxOddsBonusBP {
				errs = append(errs, fmt.Errorf("item %q: odds bonus bp out of range 0..10000", d.ID))
			}
		}
		if d.Category == "cosmetic" && len(d.EffectKeys()) > 0 { // cosmetics never affect gameplay (decisions 2026-11-07)
			errs = append(errs, fmt.Errorf("item %q: cosmetic items may not carry effect fields %v", d.ID, d.EffectKeys()))
		}
		// Pháp bảo (Phase 3.4): a tribulation-odds item that drops only from bí cảnh — never tradable (no stone/money
		// laundering of odds), exactly one effect (tribulation_bonus_bp > 0). Breakthrough/tribulation odds fields are
		// for artifacts only, so no other category can carry odds.
		if d.Category == CategoryArtifact {
			if d.Tradable {
				errs = append(errs, fmt.Errorf("item %q: artifacts may not be tradable", d.ID))
			}
			if d.TribulationBonusBP <= 0 {
				errs = append(errs, fmt.Errorf("item %q: artifacts need tribulation_bonus_bp > 0", d.ID))
			}
			if keys := d.EffectKeys(); len(keys) != 1 {
				errs = append(errs, fmt.Errorf("item %q: artifacts carry only tribulation_bonus_bp, got %v", d.ID, keys))
			}
		} else if d.RaisesOdds() {
			errs = append(errs, fmt.Errorf("item %q: only artifacts may raise breakthrough/tribulation odds", d.ID))
		}
		if d.UniquePerCharacter && d.Category != "cosmetic" {
			errs = append(errs, fmt.Errorf("item %q: unique_per_character is for cosmetics only", d.ID))
		}
		c.byID[d.ID] = d
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("item: invalid catalogue: %w", errors.Join(errs...))
	}
	return c, nil
}

// Get returns a definition by ID.
func (c *Catalogue) Get(id string) (*Def, bool) { d, ok := c.byID[id]; return d, ok }

// All returns all definitions in file order. Do not mutate.
func (c *Catalogue) All() []Def { return c.items }

// SpeedBudgetMilli is Σ instant_points_milli × max_use_per_game_day over all speed items: the most a character can
// credit from speed items in a game day by per-item limits alone (cross-checked ≤ contribution speed_items cap).
func (c *Catalogue) SpeedBudgetMilli() int64 {
	var sum int64
	for _, d := range c.items {
		sum += d.InstantPointsMilli * int64(d.MaxUsePerGameDay)
	}
	return sum
}

// EffectLikeKey reports whether a YAML key looks like a gameplay effect: any items.yaml effect key, any unit-suffixed
// number (`*_bp`, `*_milli`, `*_hours`, `*_seconds`, `*_points`) or a known modifier name. Display-only catalogues
// (cosmetics, faction works, appearances) reject such keys so a designer cannot attach power to a look.
func EffectLikeKey(key string) bool {
	switch key {
	case EffectOfflineCapBonus, EffectInstantPoints, EffectMaxUsePerDay, EffectBreakthroughBP,
		EffectTribulationBP, "rate_bonus", "points", "merit", "rank_points", "vip_points", "cultivation":
		return true
	}
	for _, suf := range []string{"_bp", "_milli", "_hours", "_seconds", "_points"} {
		if strings.HasSuffix(key, suf) {
			return true
		}
	}
	return false
}
