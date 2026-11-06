// Package vip resolves VIP level from cumulative points and the benefits of each level.
//
// Fairness invariants (enforced by the loader, see content/catalog/vip.yaml header):
//   - VIP affects only the offline accumulation window, daily gift and cosmetic/convenience perks.
//   - No perk may touch cultivation rate, breakthrough odds or ranking.
package vip

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// MaxTier is the highest VIP level supported by the game.
const MaxTier = 12

// Allowed perk keys. Anything else (e.g. cultivation_rate, breakthrough_chance, rank_points) is rejected.
var (
	perkWithID = map[string]bool{"title": true, "avatar_frame": true, "chat_aura": true, "cave_decoration": true, "breakthrough_fx": true}
	perkFlag   = map[string]bool{"alchemy_slot": true, "garden_auto_harvest": true}
	slugRe     = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
)

// Tier is one VIP level as authored.
type Tier struct {
	Tier            int      `yaml:"tier" json:"tier"`
	Name            string   `yaml:"name" json:"name"`
	PointsRequired  int64    `yaml:"points_required" json:"pointsRequired"`
	OfflineCapHours int      `yaml:"offline_cap_hours" json:"offlineCapHours"`
	DailyGift       int64    `yaml:"daily_gift" json:"dailyGift"`
	Perks           []string `yaml:"perks" json:"perks"`
}

// Benefits are the cumulative benefits at a level.
type Benefits struct {
	Tier            int      `json:"tier"`
	Name            string   `json:"name"`
	OfflineCapHours int      `json:"offlineCapHours"`
	DailyGift       int64    `json:"dailyGift"`
	AlchemySlots    int      `json:"alchemySlots"` // extra slots on top of the free base
	Cosmetics       []string `json:"cosmetics"`    // "title:vip_3", "avatar_frame:ngoc", ...
	Flags           []string `json:"flags"`        // "garden_auto_harvest"
}

// Table is the immutable VIP ladder.
type Table struct {
	PointValueVND     int64
	HardMaxOfflineCap time.Duration
	DailyCeiling      time.Duration
	tiers             []Tier
	benefits          []Benefits
}

type file struct {
	Version                      int    `yaml:"version"`
	PointValueVND                int64  `yaml:"point_value_vnd"`
	HardMaxOfflineCapHours       int    `yaml:"hard_max_offline_cap_hours"`
	DailyCultivationCeilingHours int    `yaml:"daily_cultivation_ceiling_hours"`
	Tiers                        []Tier `yaml:"tiers"`
}

// LoadFile parses and validates vip.yaml.
func LoadFile(path string) (*Table, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f file
	dec := yaml.NewDecoder(strings.NewReader(string(b)))
	dec.KnownFields(true)
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("vip: decode: %w", err)
	}
	if f.Version != 2 {
		return nil, fmt.Errorf("vip: unsupported version %d", f.Version)
	}
	return New(f.Tiers, f.HardMaxOfflineCapHours, f.DailyCultivationCeilingHours, f.PointValueVND)
}

// New validates the ladder and precomputes cumulative benefits.
func New(tiers []Tier, hardMaxHours, dailyCeilingHours int, pointValueVND int64) (*Table, error) {
	var errs []error
	fail := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }

	if len(tiers) != MaxTier+1 {
		fail("need exactly %d tiers (0..%d), got %d", MaxTier+1, MaxTier, len(tiers))
	}
	if dailyCeilingHours <= 0 || dailyCeilingHours > 24 {
		fail("daily_cultivation_ceiling_hours must be 1..24")
	}
	t := &Table{
		PointValueVND:     pointValueVND,
		HardMaxOfflineCap: time.Duration(hardMaxHours) * time.Hour,
		DailyCeiling:      time.Duration(dailyCeilingHours) * time.Hour,
		tiers:             tiers,
	}
	var acc Benefits
	for i, tr := range tiers {
		if tr.Tier != i {
			fail("tier at position %d has tier=%d", i, tr.Tier)
		}
		if i == 0 && tr.PointsRequired != 0 {
			fail("tier 0 must require 0 points")
		}
		if i > 0 {
			prev := tiers[i-1]
			if tr.PointsRequired <= prev.PointsRequired {
				fail("tier %d: points_required must increase", i)
			}
			if tr.OfflineCapHours < prev.OfflineCapHours {
				fail("tier %d: offline cap decreases", i)
			}
			if tr.DailyGift < prev.DailyGift {
				fail("tier %d: daily_gift decreases", i)
			}
		}
		if tr.OfflineCapHours <= 0 || tr.OfflineCapHours > hardMaxHours {
			fail("tier %d: offline cap %dh outside (0, %d]", i, tr.OfflineCapHours, hardMaxHours)
		}
		if tr.DailyGift < 0 {
			fail("tier %d: negative daily_gift", i)
		}
		acc.Tier, acc.Name, acc.OfflineCapHours, acc.DailyGift = tr.Tier, tr.Name, tr.OfflineCapHours, tr.DailyGift
		for _, p := range tr.Perks {
			key, id, hasID := strings.Cut(p, ":")
			switch {
			case hasID && perkWithID[key] && slugRe.MatchString(id):
				acc.Cosmetics = append(acc.Cosmetics, p)
			case !hasID && key == "alchemy_slot":
				acc.AlchemySlots++
			case !hasID && perkFlag[key]:
				acc.Flags = append(acc.Flags, key)
			default:
				fail("tier %d: perk %q not allowed (VIP perks must be cosmetic/convenience only)", i, p)
			}
		}
		b := acc
		b.Cosmetics = append([]string{}, acc.Cosmetics...)
		b.Flags = append([]string{}, acc.Flags...)
		t.benefits = append(t.benefits, b)
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("vip: invalid ladder: %w", errors.Join(errs...))
	}
	return t, nil
}

// TierForPoints returns the highest tier whose threshold is ≤ points.
func (t *Table) TierForPoints(points int64) int {
	tier := 0
	for i, tr := range t.tiers {
		if points >= tr.PointsRequired {
			tier = i
		}
	}
	return tier
}

func (t *Table) clamp(tier int) int { return max(0, min(tier, len(t.tiers)-1)) }

// OfflineCap returns the VIP offline accumulation window for a tier.
func (t *Table) OfflineCap(tier int) time.Duration {
	return time.Duration(t.tiers[t.clamp(tier)].OfflineCapHours) * time.Hour
}

// EffectiveOfflineCap adds a temporary bonus (e.g. Tụ Linh Phù) to the tier cap. A bonus can raise the
// window up to the daily ceiling but never beyond it, and never lowers a VIP cap that is already above it.
func (t *Table) EffectiveOfflineCap(tier int, bonus time.Duration) time.Duration {
	base := t.OfflineCap(tier)
	if bonus <= 0 {
		return base
	}
	return min(base+bonus, max(base, t.DailyCeiling))
}

// Benefits returns cumulative benefits at a tier.
func (t *Table) Benefits(tier int) Benefits { return t.benefits[t.clamp(tier)] }

// Tiers returns the authored ladder.
func (t *Table) Tiers() []Tier { return t.tiers }
