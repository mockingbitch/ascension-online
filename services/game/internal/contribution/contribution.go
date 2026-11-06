// Package contribution holds the daily points rules (content/catalog/contribution.yaml): bucket caps, the 150/day
// ceiling, the idle rate, catch-up ×2 and the late-joiner grant. Pure functions only; the single writer of points
// is cultivation.CreditLocked (docs/design/data-model-the-gioi-chung.md §2.4, §4).
package contribution

import (
	"errors"
	"fmt"
	"io"
	"os"
	"slices"

	"gopkg.in/yaml.v3"

	"tiendao/game/internal/fixedpoint"
)

// Bucket groups sources for the daily caps.
type Bucket string

const (
	Idle      Bucket = "idle"
	Activity  Bucket = "activity"
	Community Bucket = "community"
)

// Buckets lists every bucket in display order.
var Buckets = []Bucket{Idle, Activity, Community}

// Source is where a credit comes from (DB enum cultivation_credits.source).
type Source string

const (
	SourceSettle          Source = "settle"
	SourceLecture         Source = "lecture"
	SourceDailyTask       Source = "daily_task"
	SourceRitual          Source = "ritual"
	SourceFortune         Source = "fortune"
	SourceTrial           Source = "trial"
	SourceItem            Source = "item"
	SourceFaction         Source = "faction"
	SourceProtector       Source = "protector"
	SourceDualCultivation Source = "dual_cultivation"
	SourceEvent           Source = "event"
	// SourceLateJoinGrant is the reserve release; it never goes through Apply and has no bucket.
	SourceLateJoinGrant Source = "late_join_grant"
)

// Sources lists every bucketed source; each must be mapped in the YAML.
var Sources = []Source{SourceSettle, SourceLecture, SourceDailyTask, SourceRitual, SourceFortune, SourceTrial,
	SourceItem, SourceFaction, SourceProtector, SourceDualCultivation, SourceEvent}

// BucketCfg holds one bucket's numbers in milli.
type BucketCfg struct {
	TargetMilli int64 `json:"targetMilli"`
	CapMilli    int64 `json:"capMilli"`
}

// CatchUp is the late-joiner multiplier.
type CatchUp struct {
	MultiplierBP int64
	MinServerDay int
	AppliesTo    []Bucket
}

// LateJoinGrant is the one-time grant for characters created late.
type LateJoinGrant struct {
	AfterServerDay int
	MedianShareBP  int64
}

// Config is the validated, immutable contribution catalogue.
type Config struct {
	DailyCeilingMilli    int64
	Buckets              map[Bucket]BucketCfg
	IdleRateMilliPerHour int64
	SpeedItemDailyMilli  int64
	CatchUp              CatchUp
	Grant                LateJoinGrant
	Sources              map[Source]Bucket
}

// Day is today's credited milli for one character (daily_contribution row).
type Day struct {
	Idle, Activity, Community, SpeedItem, CatchUpBonus int64
}

// Total is the points credited today across buckets.
func (d Day) Total() int64 { return d.Idle + d.Activity + d.Community }

// Get returns the bucket's credited milli.
func (d Day) Get(b Bucket) int64 {
	switch b {
	case Idle:
		return d.Idle
	case Activity:
		return d.Activity
	case Community:
		return d.Community
	}
	return 0
}

// Add returns d with a credit applied.
func (d Day) Add(a Applied, src Source) Day {
	switch a.Bucket {
	case Idle:
		d.Idle += a.CreditedMilli
	case Activity:
		d.Activity += a.CreditedMilli
	case Community:
		d.Community += a.CreditedMilli
	}
	if src == SourceItem {
		d.SpeedItem += a.CreditedMilli
	}
	d.CatchUpBonus += a.CatchUpBonusMilli
	return d
}

// Applied is the outcome of one credit through the caps (all milli).
type Applied struct {
	Bucket                 Bucket
	RawMilli               int64
	MultiplierBP           int64
	BoostedMilli           int64
	CreditedMilli          int64
	CatchUpBonusMilli      int64 // = credited − min(credited, raw): the ×2 portion that survived clipping
	ClippedCeilingMilli    int64 // removed by bucket cap / daily ceiling / speed-item cap
	ClippedBottleneckMilli int64 // removed by the bình cảnh room (does not consume the ceiling)
}

// Apply runs one credit through catch-up → bucket/ceiling caps → bottleneck room. Pure.
// The ×2 is applied BEFORE the ceiling (C5); bottleneck-clipped points never consume the ceiling (C6).
func (c *Config) Apply(src Source, raw int64, day Day, catchUp bool, bottleneckRoom int64) Applied {
	b := c.Sources[src]
	a := Applied{Bucket: b, RawMilli: max(0, raw), MultiplierBP: fixedpoint.BPScale}
	if catchUp && slices.Contains(c.CatchUp.AppliesTo, b) {
		a.MultiplierBP = c.CatchUp.MultiplierBP
	}
	a.BoostedMilli = fixedpoint.MulBP(a.RawMilli, a.MultiplierBP)
	room := min(c.Buckets[b].CapMilli-day.Get(b), c.DailyCeilingMilli-day.Total())
	if src == SourceItem {
		room = min(room, c.SpeedItemDailyMilli-day.SpeedItem)
	}
	allowed := min(a.BoostedMilli, max(0, room))
	a.CreditedMilli = min(allowed, max(0, bottleneckRoom))
	a.ClippedCeilingMilli = a.BoostedMilli - allowed
	a.ClippedBottleneckMilli = allowed - a.CreditedMilli
	a.CatchUpBonusMilli = a.CreditedMilli - min(a.CreditedMilli, a.RawMilli)
	return a
}

// CatchUpEligible: the world has started, server day ≥ min_server_day and the character is below the median.
func (c *Config) CatchUpEligible(started bool, serverDay int, positionMilli, medianMilli int64) bool {
	return started && serverDay >= c.CatchUp.MinServerDay && positionMilli < medianMilli
}

// GrantMilli is the late-joiner grant for a character created on serverDay (0 when not eligible).
func (c *Config) GrantMilli(started bool, serverDay int, medianMilli int64) int64 {
	if !started || serverDay <= c.Grant.AfterServerDay || medianMilli <= 0 {
		return 0
	}
	return fixedpoint.MulBP(medianMilli, c.Grant.MedianShareBP)
}

// ───── loader ─────

type fileBucket struct {
	TargetPoints   int64 `yaml:"target_points"`
	DailyCapPoints int64 `yaml:"daily_cap_points"`
}

type file struct {
	Version            int                   `yaml:"version"`
	DailyCeilingPoints int64                 `yaml:"daily_ceiling_points"`
	Buckets            map[Bucket]fileBucket `yaml:"buckets"`
	Idle               struct {
		RateMilliPerHour int64 `yaml:"rate_milli_per_hour"`
	} `yaml:"idle"`
	SpeedItems struct {
		DailyCapPoints int64 `yaml:"daily_cap_points"`
	} `yaml:"speed_items"`
	CatchUp struct {
		MultiplierBP int64    `yaml:"multiplier_bp"`
		Until        string   `yaml:"until"`
		MinServerDay int      `yaml:"min_server_day"`
		AppliesTo    []Bucket `yaml:"applies_to"`
	} `yaml:"catch_up"`
	LateJoinGrant struct {
		AfterServerDay int   `yaml:"after_server_day"`
		MedianShareBP  int64 `yaml:"median_share_bp"`
		PerAccount     int   `yaml:"per_account"`
	} `yaml:"late_join_grant"`
	Sources map[Source]struct {
		Bucket Bucket `yaml:"bucket"`
	} `yaml:"sources"`
}

// LoadFile parses and validates contribution.yaml.
func LoadFile(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Load(f)
}

// Load parses and validates contribution YAML.
func Load(r io.Reader) (*Config, error) {
	var raw file
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("contribution: decode: %w", err)
	}
	var errs []error
	fail := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	const pm = 1000
	if raw.Version != 1 {
		fail("unsupported version %d", raw.Version)
	}
	c := &Config{
		DailyCeilingMilli:    raw.DailyCeilingPoints * pm,
		Buckets:              map[Bucket]BucketCfg{},
		IdleRateMilliPerHour: raw.Idle.RateMilliPerHour,
		SpeedItemDailyMilli:  raw.SpeedItems.DailyCapPoints * pm,
		CatchUp:              CatchUp{MultiplierBP: raw.CatchUp.MultiplierBP, MinServerDay: raw.CatchUp.MinServerDay, AppliesTo: raw.CatchUp.AppliesTo},
		Grant:                LateJoinGrant{AfterServerDay: raw.LateJoinGrant.AfterServerDay, MedianShareBP: raw.LateJoinGrant.MedianShareBP},
		Sources:              map[Source]Bucket{},
	}
	if raw.DailyCeilingPoints <= 0 {
		fail("daily_ceiling_points must be > 0")
	}
	var targets int64
	for b, fb := range raw.Buckets {
		if !slices.Contains(Buckets, b) {
			fail("unknown bucket %q", b)
			continue
		}
		if !(0 < fb.TargetPoints && fb.TargetPoints <= fb.DailyCapPoints && fb.DailyCapPoints <= raw.DailyCeilingPoints) {
			fail("bucket %q: need 0 < target ≤ daily_cap ≤ ceiling", b)
		}
		targets += fb.TargetPoints
		c.Buckets[b] = BucketCfg{TargetMilli: fb.TargetPoints * pm, CapMilli: fb.DailyCapPoints * pm}
	}
	for _, b := range Buckets {
		if _, ok := raw.Buckets[b]; !ok {
			fail("bucket %q missing", b)
		}
	}
	if targets > raw.DailyCeilingPoints {
		fail("Σ bucket targets %d > ceiling", targets)
	}
	if raw.Idle.RateMilliPerHour <= 0 || raw.Idle.RateMilliPerHour*24 > raw.Buckets[Idle].DailyCapPoints*pm {
		fail("idle.rate_milli_per_hour must be > 0 and ×24 ≤ idle daily cap")
	}
	if raw.SpeedItems.DailyCapPoints < 0 || raw.SpeedItems.DailyCapPoints > raw.Buckets[Activity].DailyCapPoints {
		fail("speed_items.daily_cap_points must be within 0..activity cap")
	}
	if raw.CatchUp.MultiplierBP < fixedpoint.BPScale || raw.CatchUp.MultiplierBP > 50_000 {
		fail("catch_up.multiplier_bp must be within 10000..50000")
	}
	if raw.CatchUp.Until != "server_median" {
		fail("catch_up.until must be server_median")
	}
	if raw.CatchUp.MinServerDay < 0 {
		fail("catch_up.min_server_day must be ≥ 0")
	}
	for _, b := range raw.CatchUp.AppliesTo {
		if !slices.Contains(Buckets, b) {
			fail("catch_up.applies_to: unknown bucket %q", b)
		}
	}
	if raw.LateJoinGrant.AfterServerDay < 0 || raw.LateJoinGrant.MedianShareBP <= 0 || raw.LateJoinGrant.MedianShareBP > fixedpoint.BPScale {
		fail("late_join_grant: after_server_day ≥ 0 and 0 < median_share_bp ≤ 10000 required")
	}
	if raw.LateJoinGrant.PerAccount != 1 {
		fail("late_join_grant.per_account must be 1")
	}
	for s, m := range raw.Sources {
		if !slices.Contains(Sources, s) {
			fail("unknown source %q", s)
			continue
		}
		if !slices.Contains(Buckets, m.Bucket) {
			fail("source %q: unknown bucket %q", s, m.Bucket)
		}
		c.Sources[s] = m.Bucket
	}
	for _, s := range Sources {
		if _, ok := raw.Sources[s]; !ok {
			fail("source %q has no bucket mapping", s)
		}
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("contribution: invalid catalogue: %w", errors.Join(errs...))
	}
	return c, nil
}
