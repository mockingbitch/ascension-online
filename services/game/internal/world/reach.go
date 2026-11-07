package world

import (
	"time"

	"tiendao/game/internal/fixedpoint"
)

// Reason records why a milestone was reached (world_milestones_reached.reason).
type Reason string

const (
	ReasonThreshold Reason = "threshold" // progress ≥ threshold after the min_pct floor
	ReasonMaxTime   Reason = "max_time"  // the max_pct ceiling passed regardless of progress
	ReasonAdmin     Reason = "admin"     // forced by an admin override
)

// Override is an admin adjustment of one milestone (world_milestone_overrides). Nil fields keep the catalogue value.
type Override struct {
	ThresholdMilli *int64
	TargetDay      *int
	Force          bool // reach as soon as it is next in order, ignoring time and progress
	Paused         bool // hold this milestone (and therefore every later one)
}

// Snapshot is what the reach rule needs from world_state + overrides.
type Snapshot struct {
	StartedAt     *time.Time
	ProgressMilli int64 // Σ per-capita of closed days since launch
	Reached       map[string]bool
	Paused        bool // world_state.paused_at IS NOT NULL
	Overrides     map[string]Override
}

// Decision is the milestone to record this tick.
type Decision struct {
	Milestone      *Milestone
	Reason         Reason
	ThresholdMilli int64 // effective (override applied)
	TargetDay      int   // effective (override applied)
}

// NextUnreached is the first milestone in catalogue order that is not reached (nil when all are).
func (c *Catalogue) NextUnreached(reached map[string]bool) *Milestone {
	for i := range c.Milestones {
		if !reached[c.Milestones[i].ID] {
			return &c.Milestones[i]
		}
	}
	return nil
}

// Effective returns the threshold (milli) and target day of m after an admin override.
func Effective(m *Milestone, ov Override) (thresholdMilli int64, targetDay int) {
	thresholdMilli, targetDay = m.ThresholdMilli, m.TargetDay
	if ov.ThresholdMilli != nil {
		thresholdMilli = *ov.ThresholdMilli
	}
	if ov.TargetDay != nil {
		targetDay = *ov.TargetDay
	}
	return thresholdMilli, targetDay
}

// Window is the safety window of a target day: never before min_pct, always by max_pct of it (exact seconds since
// started_at, §2.2).
func (c *Catalogue) Window(targetDay int, startedAt time.Time) (earliest, latest time.Time) {
	secs := int64(targetDay) * 86_400
	earliest = startedAt.Add(time.Duration(fixedpoint.MulBP(secs, c.Settings.MinPctBP)) * time.Second)
	latest = startedAt.Add(time.Duration(fixedpoint.MulBP(secs, c.Settings.MaxPctBP)) * time.Second)
	return earliest, latest
}

// Next is the reach rule (§5.3). Pure. At most one milestone per call (= per tick), always the first unreached one,
// so the previous milestone is always required. Nothing happens before launch or while the world or that milestone
// is paused.
func (c *Catalogue) Next(s Snapshot, now time.Time) (Decision, bool) {
	if s.StartedAt == nil || s.Paused || now.Before(*s.StartedAt) {
		return Decision{}, false
	}
	m := c.NextUnreached(s.Reached)
	if m == nil {
		return Decision{}, false
	}
	ov := s.Overrides[m.ID]
	if ov.Paused {
		return Decision{}, false
	}
	thr, day := Effective(m, ov)
	d := Decision{Milestone: m, ThresholdMilli: thr, TargetDay: day}
	earliest, latest := c.Window(day, *s.StartedAt)
	switch {
	case ov.Force:
		d.Reason = ReasonAdmin
	case !now.Before(earliest) && s.ProgressMilli >= thr:
		d.Reason = ReasonThreshold
	case !now.Before(latest):
		d.Reason = ReasonMaxTime
	default:
		return Decision{}, false
	}
	return d, true
}

// ProgressBP is progress toward a threshold in basis points, capped at 10000 (threshold 0 → 10000). Pure.
func ProgressBP(progressMilli, thresholdMilli int64) int64 {
	if thresholdMilli <= 0 || progressMilli >= thresholdMilli {
		return fixedpoint.BPScale
	}
	return max(0, fixedpoint.MulDiv(progressMilli, fixedpoint.BPScale, thresholdMilli))
}

// Estimate is when the threshold should be reached at the recent average pace, clamped to [earliest, latest]
// (§11 estimatedAt). nil when there is no pace data and the threshold isn't met yet.
func Estimate(now time.Time, progressMilli, thresholdMilli, avgPerDayMilli int64, earliest, latest time.Time) *time.Time {
	var t time.Time
	switch remaining := thresholdMilli - progressMilli; {
	case remaining <= 0:
		t = now
	case avgPerDayMilli <= 0:
		return nil
	default:
		days := (remaining + avgPerDayMilli - 1) / avgPerDayMilli // ceil; whole closed days
		if days > maxTargetDay*2 {
			t = latest
		} else {
			t = now.Add(time.Duration(days) * 24 * time.Hour)
		}
	}
	if t.Before(earliest) {
		t = earliest
	}
	if t.After(latest) {
		t = latest
	}
	return &t
}
