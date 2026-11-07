// Package dailytask implements daily tasks: catalogue (content/catalog/daily_tasks.yaml), progress hooks
// called inside gameplay transactions, and reward claiming.
package dailytask

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"time"

	"gopkg.in/yaml.v3"
)

// Kind is what advances a task.
type Kind string

const (
	Checkin             Kind = "checkin"
	Settle              Kind = "settle"
	BreakthroughAttempt Kind = "breakthrough_attempt"
	LectureAttend       Kind = "lecture_attend" // a paid ngộ đạo moment in Tử Tiêu Cung (decision 13)
	Incense             Kind = "incense"        // morning incense with a streak (data-model §9.2); done via ritual.Service
)

// StreakReward pays points_milli (CreditLocked, source ritual) when the streak is ≥ min_streak (highest tier wins).
type StreakReward struct {
	MinStreak   int32 `yaml:"min_streak" json:"minStreak"`
	PointsMilli int64 `yaml:"points_milli" json:"pointsMilli"`
}

// maxStreakPointsMilli guards a single ritual credit (the activity bucket cap is the real bound, content cross-check).
const maxStreakPointsMilli = 10_000

// Reward is either an item or a soft-currency amount.
type Reward struct {
	ItemID   string `yaml:"item_id,omitempty" json:"itemId,omitempty"`
	Quantity int64  `yaml:"quantity,omitempty" json:"quantity,omitempty"`
	Currency string `yaml:"currency,omitempty" json:"currency,omitempty"`
	Amount   int64  `yaml:"amount,omitempty" json:"amount,omitempty"`
}

// Task is one daily task definition.
type Task struct {
	ID                string   `yaml:"id" json:"id"`
	Name              string   `yaml:"name" json:"name"`
	Description       string   `yaml:"description" json:"description"`
	Kind              Kind     `yaml:"kind" json:"kind"`
	Target            int32    `yaml:"target" json:"target"`
	MinElapsedSeconds int64    `yaml:"min_elapsed_seconds,omitempty" json:"minElapsedSeconds,omitempty"`
	Rewards           []Reward `yaml:"rewards" json:"rewards"`
	// StreakRewards: incense tasks only, ascending min_streak starting at 1.
	StreakRewards []StreakReward `yaml:"streak_rewards,omitempty" json:"streakRewards,omitempty"`
}

// StreakPoints returns the points_milli paid at this streak (0 when no tier applies).
func (t *Task) StreakPoints(streak int32) int64 {
	var pts int64
	for _, r := range t.StreakRewards {
		if streak >= r.MinStreak {
			pts = r.PointsMilli
		}
	}
	return pts
}

// NextStreak is the streak after an offering on `today` when the previous one was on `lastDay` (zero = never):
// consecutive game day → +1, otherwise restart at 1. Days are gameday.Of values. Pure.
func NextStreak(lastDay time.Time, prev int32, today time.Time) int32 {
	if !lastDay.IsZero() && lastDay.AddDate(0, 0, 1).Equal(today) {
		return prev + 1
	}
	return 1
}

// LiveStreak is the streak still alive on `today`: kept when the last offering was today or yesterday, else 0. Pure.
func LiveStreak(lastDay time.Time, streak int32, today time.Time) int32 {
	if lastDay.IsZero() || (!lastDay.Equal(today) && !lastDay.AddDate(0, 0, 1).Equal(today)) {
		return 0
	}
	return streak
}

// Catalogue is immutable after load.
type Catalogue struct {
	tasks []Task
	byID  map[string]*Task
}

var slugRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// LoadFile parses and validates daily_tasks.yaml. Item references are cross-checked by content.LoadAll.
func LoadFile(path string) (*Catalogue, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var raw struct {
		Version int    `yaml:"version"`
		Tasks   []Task `yaml:"tasks"`
	}
	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("dailytask: decode: %w", err)
	}
	var errs []error
	fail := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	if raw.Version != 1 {
		fail("unsupported version %d", raw.Version)
	}
	c := &Catalogue{tasks: raw.Tasks, byID: map[string]*Task{}}
	for i := range c.tasks {
		t := &c.tasks[i]
		if !slugRe.MatchString(t.ID) {
			fail("task[%d]: invalid id %q", i, t.ID)
		}
		if _, dup := c.byID[t.ID]; dup {
			fail("task %q: duplicate", t.ID)
		}
		switch t.Kind {
		case Checkin, Settle, BreakthroughAttempt, LectureAttend, Incense:
		default:
			fail("task %q: unknown kind %q", t.ID, t.Kind)
		}
		if t.Target < 1 || t.Target > 100 || ((t.Kind == Checkin || t.Kind == Incense) && t.Target != 1) {
			fail("task %q: target out of range", t.ID)
		}
		if t.Kind == Incense {
			if len(t.StreakRewards) == 0 {
				fail("task %q: incense needs streak_rewards", t.ID)
			}
			for j, r := range t.StreakRewards {
				if (j == 0 && r.MinStreak != 1) || (j > 0 && r.MinStreak <= t.StreakRewards[j-1].MinStreak) {
					fail("task %q: streak_rewards min_streak must start at 1 and increase", t.ID)
				}
				if r.PointsMilli <= 0 || r.PointsMilli > maxStreakPointsMilli {
					fail("task %q: streak points_milli must be within 1..%d", t.ID, maxStreakPointsMilli)
				}
			}
		} else {
			if len(t.StreakRewards) > 0 {
				fail("task %q: streak_rewards only for incense tasks", t.ID)
			}
			if len(t.Rewards) == 0 {
				fail("task %q: no rewards", t.ID)
			}
		}
		for _, r := range t.Rewards {
			isItem := r.ItemID != "" && r.Quantity > 0 && r.Currency == "" && r.Amount == 0
			isSoft := r.Currency == "spirit_stone" && r.Amount > 0 && r.ItemID == "" && r.Quantity == 0
			if !isItem && !isSoft {
				fail("task %q: reward %+v must be {item_id, quantity} or {currency: spirit_stone, amount}", t.ID, r)
			}
		}
		c.byID[t.ID] = t
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("dailytask: invalid catalogue: %w", errors.Join(errs...))
	}
	return c, nil
}

// All returns tasks in file order.
func (c *Catalogue) All() []Task { return c.tasks }

// ByKind returns the first task of a kind.
func (c *Catalogue) ByKind(k Kind) (*Task, bool) {
	for i := range c.tasks {
		if c.tasks[i].Kind == k {
			return &c.tasks[i], true
		}
	}
	return nil, false
}

// MaxStreakPointsMilli is the largest single streak reward (cross-checked against the activity bucket).
func (c *Catalogue) MaxStreakPointsMilli() int64 {
	var m int64
	for _, t := range c.tasks {
		for _, r := range t.StreakRewards {
			m = max(m, r.PointsMilli)
		}
	}
	return m
}

// Get returns a task by ID.
func (c *Catalogue) Get(id string) (*Task, bool) { t, ok := c.byID[id]; return t, ok }
