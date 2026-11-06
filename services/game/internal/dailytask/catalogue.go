// Package dailytask implements daily tasks: catalogue (content/catalog/daily_tasks.yaml), progress hooks
// called inside gameplay transactions, and reward claiming.
package dailytask

import (
	"errors"
	"fmt"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

// Kind is what advances a task.
type Kind string

const (
	Checkin             Kind = "checkin"
	Settle              Kind = "settle"
	BreakthroughAttempt Kind = "breakthrough_attempt"
)

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
		case Checkin, Settle, BreakthroughAttempt:
		default:
			fail("task %q: unknown kind %q", t.ID, t.Kind)
		}
		if t.Target < 1 || t.Target > 100 || (t.Kind == Checkin && t.Target != 1) {
			fail("task %q: target out of range", t.ID)
		}
		if len(t.Rewards) == 0 {
			fail("task %q: no rewards", t.ID)
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

// Get returns a task by ID.
func (c *Catalogue) Get(id string) (*Task, bool) { t, ok := c.byID[id]; return t, ok }
