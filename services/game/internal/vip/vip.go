// Package vip resolves the Tiên Duyên level (player-facing name of VIP, 0–12) from cumulative điểm duyên, the benefits
// of each level, and the in-game earning table (decision D). Identifiers stay "vip"; display strings come from vip.yaml.
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

	"tiendao/game/internal/fixedpoint"
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

// Activity is one in-game source of điểm duyên (decision D). Credited only by EarnLocked.
type Activity struct {
	ID                string `yaml:"id" json:"id"`
	Source            string `yaml:"source" json:"source"` // ledger source
	Name              string `yaml:"name" json:"name"`
	Points            int64  `yaml:"points" json:"points"`
	DailyMaxTimes     int64  `yaml:"daily_max_times" json:"dailyMaxTimes"`
	Live              bool   `yaml:"live" json:"live"` // false: the feature does not exist yet
	MinElapsedSeconds int64  `yaml:"min_elapsed_seconds,omitempty" json:"minElapsedSeconds,omitempty"`
}

// Activity IDs the game code credits. The loader requires each to exist (and be live) in vip.yaml.
const (
	ActOnlineCultivation = "online_cultivation"
	ActLecture           = "lecture_attendance"
	ActDailyTask         = "daily_task_claim"
	ActTrialPass         = "trial_pass"
	ActProtectorAssist   = "protector_assist" // hộ pháp counted by a tribulation attempt (Phase 3.4, worker)
	ActDualCultivation   = "dual_cultivation" // đạo lữ song tu claim reaching dao_lu.yaml duyen_min_seconds (Phase 5.1)
)

var wiredActivities = []string{ActOnlineCultivation, ActLecture, ActDailyTask, ActTrialPass, ActProtectorAssist, ActDualCultivation}

// Ledger sources (vip_point_ledger.source). Top-up and admin rows are never activities and never count toward caps.
const (
	SourceTopup = "topup"
	SourceAdmin = "admin"
)

var activitySources = map[string]bool{"cultivation": true, "lecture": true, "interaction": true, "task": true, "trial": true, "event": true}

// Earn is the in-game earning table.
type Earn struct {
	DailyCapPoints int64      `yaml:"daily_cap_points"`
	Activities     []Activity `yaml:"activities"`
}

// Config is vip.yaml as authored.
type Config struct {
	Version                      int    `yaml:"version"`
	DisplayName                  string `yaml:"display_name"`
	PointName                    string `yaml:"point_name"`
	PointsPer1000VND             int64  `yaml:"points_per_1000_vnd"`
	HardMaxOfflineCapHours       int    `yaml:"hard_max_offline_cap_hours"`
	DailyCultivationCeilingHours int    `yaml:"daily_cultivation_ceiling_hours"`
	Earn                         Earn   `yaml:"earn"`
	Tiers                        []Tier `yaml:"tiers"`
}

// Table is the immutable Tiên Duyên ladder + earning table.
type Table struct {
	DisplayName       string
	PointName         string
	PointsPer1000VND  int64
	EarnDailyCap      int64
	HardMaxOfflineCap time.Duration
	DailyCeiling      time.Duration
	tiers             []Tier
	benefits          []Benefits
	activities        []Activity
	actByID           map[string]*Activity
}

// LoadFile parses and validates vip.yaml.
func LoadFile(path string) (*Table, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Config
	dec := yaml.NewDecoder(strings.NewReader(string(b)))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("vip: decode: %w", err)
	}
	if c.Version != 3 {
		return nil, fmt.Errorf("vip: unsupported version %d", c.Version)
	}
	return New(c)
}

// New validates the ladder and earning table and precomputes cumulative benefits.
func New(c Config) (*Table, error) {
	var errs []error
	fail := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	tiers, hardMaxHours, dailyCeilingHours := c.Tiers, c.HardMaxOfflineCapHours, c.DailyCultivationCeilingHours

	if strings.TrimSpace(c.DisplayName) == "" || strings.TrimSpace(c.PointName) == "" {
		fail("display_name and point_name required")
	}
	if c.PointsPer1000VND <= 0 {
		fail("points_per_1000_vnd must be > 0")
	}
	if len(tiers) != MaxTier+1 {
		fail("need exactly %d tiers (0..%d), got %d", MaxTier+1, MaxTier, len(tiers))
	}
	if dailyCeilingHours <= 0 || dailyCeilingHours > 24 {
		fail("daily_cultivation_ceiling_hours must be 1..24")
	}
	t := &Table{
		DisplayName:       c.DisplayName,
		PointName:         c.PointName,
		PointsPer1000VND:  c.PointsPer1000VND,
		EarnDailyCap:      c.Earn.DailyCapPoints,
		HardMaxOfflineCap: time.Duration(hardMaxHours) * time.Hour,
		DailyCeiling:      time.Duration(dailyCeilingHours) * time.Hour,
		tiers:             tiers,
		activities:        c.Earn.Activities,
		actByID:           map[string]*Activity{},
	}
	if c.Earn.DailyCapPoints <= 0 {
		fail("earn.daily_cap_points must be > 0")
	}
	for i := range t.activities {
		a := &t.activities[i]
		if !slugRe.MatchString(a.ID) {
			fail("earn activity[%d]: invalid id %q", i, a.ID)
		}
		if _, dup := t.actByID[a.ID]; dup {
			fail("earn activity %q: duplicate", a.ID)
		}
		t.actByID[a.ID] = a
		if !activitySources[a.Source] {
			fail("earn activity %q: source %q not allowed (topup/admin are not activities)", a.ID, a.Source)
		}
		if strings.TrimSpace(a.Name) == "" {
			fail("earn activity %q: name required", a.ID)
		}
		if a.Points <= 0 || a.DailyMaxTimes < 1 {
			fail("earn activity %q: points > 0 and daily_max_times ≥ 1", a.ID)
		}
		if a.Points > c.Earn.DailyCapPoints {
			fail("earn activity %q: points exceed earn.daily_cap_points", a.ID)
		}
		if a.MinElapsedSeconds < 0 {
			fail("earn activity %q: negative min_elapsed_seconds", a.ID)
		}
	}
	for _, id := range wiredActivities {
		if a, ok := t.actByID[id]; !ok || !a.Live {
			fail("earn activity %q is credited by the game and must exist with live: true", id)
		}
	}
	var acc Benefits
	for i, tr := range tiers {
		if tr.Tier != i {
			fail("tier at position %d has tier=%d", i, tr.Tier)
		}
		if strings.TrimSpace(tr.Name) == "" || strings.Contains(strings.ToUpper(tr.Name), "VIP") {
			fail("tier %d: name %q must be set and must not say VIP (display name is Tiên Duyên)", i, tr.Name)
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
				fail("tier %d: perk %q not allowed (Tiên Duyên perks must be cosmetic/convenience only)", i, p)
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

// Activities returns the earning table as authored.
func (t *Table) Activities() []Activity { return t.activities }

// Activity looks up an earning activity.
func (t *Table) Activity(id string) (Activity, bool) {
	a, ok := t.actByID[id]
	if !ok {
		return Activity{}, false
	}
	return *a, true
}

// EarnAmount is the pure daily-max rule: how many điểm duyên the next credit of activityID pays, given how many times
// it was already credited this game day and how many in-game points the user earned today across all activities.
// 0 when the activity is used up or the daily cap is reached; a credit that would cross the cap is clipped to it.
func (t *Table) EarnAmount(activityID string, timesToday, earnedToday int64) (int64, error) {
	a, ok := t.actByID[activityID]
	if !ok {
		return 0, fmt.Errorf("%w: %q", ErrUnknownActivity, activityID)
	}
	if !a.Live {
		return 0, fmt.Errorf("%w: %q is not live", ErrUnknownActivity, activityID)
	}
	if timesToday >= a.DailyMaxTimes {
		return 0, nil
	}
	return max(0, min(a.Points, t.EarnDailyCap-earnedToday)), nil
}

// PointsForVND converts a top-up amount to điểm duyên (floor; 1.000đ = PointsPer1000VND).
func (t *Table) PointsForVND(amountVND int64) int64 {
	if amountVND <= 0 {
		return 0
	}
	return fixedpoint.MulDiv(amountVND, t.PointsPer1000VND, 1000)
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
