// Package world holds the shared-world clock: per-game-day totals, the server median ladder position and (Phase 2)
// story milestones. Phase 1 subset: settings loader + pure helpers used by the worker day job and by catch-up /
// late-join grant (docs/design/data-model-the-gioi-chung.md §5).
package world

import (
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"time"

	"gopkg.in/yaml.v3"

	"tiendao/game/internal/fixedpoint"
	"tiendao/game/internal/gameday"
)

// Settings are the world_milestones.yaml settings block (points converted to milli).
type Settings struct {
	WorldCreditCapMilli     int64
	ReferencePerActiveMilli int64
	MinPctBP, MaxPctBP      int64
	DayCloseGrace           time.Duration
	MedianWindowDays        int
	ExcludeCatchUpBonus     bool
}

// Catalogue is the validated world catalogue (Phase 1: settings only).
type Catalogue struct {
	Settings Settings
}

type file struct {
	Version  int `yaml:"version"`
	Settings struct {
		ProgressMode                string `yaml:"progress_mode"`
		WorldCreditCapPoints        int64  `yaml:"world_credit_cap_points"`
		ExcludeCatchUpBonus         bool   `yaml:"exclude_catch_up_bonus"`
		ReferencePointsPerActiveDay int64  `yaml:"reference_points_per_active_day"`
		MinPctBP                    int64  `yaml:"min_pct_bp"`
		MaxPctBP                    int64  `yaml:"max_pct_bp"`
		DayCloseGraceSeconds        int64  `yaml:"day_close_grace_seconds"`
		MedianWindowDays            int    `yaml:"median_window_days"`
	} `yaml:"settings"`
	Eras       []yaml.Node `yaml:"eras"`       // Phase 2
	Milestones []yaml.Node `yaml:"milestones"` // Phase 2
}

// LoadFile parses and validates world_milestones.yaml.
func LoadFile(path string) (*Catalogue, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Load(f)
}

// Load parses and validates world YAML. The cross-check against contribution's ceiling lives in content.LoadAll.
func Load(r io.Reader) (*Catalogue, error) {
	var raw file
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("world: decode: %w", err)
	}
	var errs []error
	fail := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	s := raw.Settings
	if raw.Version != 1 {
		fail("unsupported version %d", raw.Version)
	}
	if s.ProgressMode != "per_active_capita" {
		fail("settings.progress_mode must be per_active_capita")
	}
	if !s.ExcludeCatchUpBonus {
		fail("settings.exclude_catch_up_bonus must be true")
	}
	if !(0 < s.ReferencePointsPerActiveDay && s.ReferencePointsPerActiveDay <= s.WorldCreditCapPoints) {
		fail("settings: need 0 < reference_points_per_active_day ≤ world_credit_cap_points")
	}
	if !(0 < s.MinPctBP && s.MinPctBP <= fixedpoint.BPScale && fixedpoint.BPScale <= s.MaxPctBP && s.MaxPctBP <= 30_000) {
		fail("settings: need 0 < min_pct_bp ≤ 10000 ≤ max_pct_bp ≤ 30000")
	}
	if s.DayCloseGraceSeconds < 0 || s.DayCloseGraceSeconds > 3600 {
		fail("settings.day_close_grace_seconds must be within 0..3600")
	}
	if s.MedianWindowDays < 1 || s.MedianWindowDays > 60 {
		fail("settings.median_window_days must be within 1..60")
	}
	if len(raw.Eras) > 0 || len(raw.Milestones) > 0 {
		fail("eras/milestones are Phase 2 and not supported by this loader yet")
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("world: invalid catalogue: %w", errors.Join(errs...))
	}
	return &Catalogue{Settings: Settings{
		WorldCreditCapMilli:     s.WorldCreditCapPoints * 1000,
		ReferencePerActiveMilli: s.ReferencePointsPerActiveDay * 1000,
		MinPctBP:                s.MinPctBP,
		MaxPctBP:                s.MaxPctBP,
		DayCloseGrace:           time.Duration(s.DayCloseGraceSeconds) * time.Second,
		MedianWindowDays:        s.MedianWindowDays,
		ExcludeCatchUpBonus:     s.ExcludeCatchUpBonus,
	}}, nil
}

// Countable is what one character-day contributes to world progress: min(cap, points − ×2 bonus). Pure.
func Countable(pointsMilli, catchUpBonusMilli, capMilli int64) int64 {
	return max(0, min(capMilli, pointsMilli-catchUpBonusMilli))
}

// PerCapita = countable / DAU (integer division; 0 when DAU = 0). Pure.
func PerCapita(countableMilli int64, dau int64) int64 {
	if dau <= 0 {
		return 0
	}
	return countableMilli / dau
}

// Median is the lower median of positions (0 for none). It sorts a copy.
func Median(positions []int64) int64 {
	if len(positions) == 0 {
		return 0
	}
	s := slices.Clone(positions)
	slices.Sort(s)
	return s[(len(s)-1)/2]
}

// ServerDay is the number of whole game days since the launch day (day 0 = launch day); -1 before launch.
func ServerDay(startedAt *time.Time, now time.Time) int {
	if startedAt == nil || now.Before(*startedAt) {
		return -1
	}
	return int(gameday.Of(now).Sub(gameday.Of(*startedAt)) / (24 * time.Hour))
}

// ClosableAt is when game day `day` (a gameday.Of value) may be closed: its next midnight + grace.
func ClosableAt(day time.Time, grace time.Duration) time.Time {
	return time.Date(day.Year(), day.Month(), day.Day()+1, 0, 0, 0, 0, gameday.Zone).Add(grace)
}
