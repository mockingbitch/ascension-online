package world

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
)

// Era is a named period of the shared story (world_state.era_id). Seq = catalogue order.
type Era struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Seq  int    `json:"-"`
}

// EffectKind is what reaching a milestone does to the world.
type EffectKind string

const (
	// EffectSetEra moves world_state.era_id forward (never backwards).
	EffectSetEra EffectKind = "set_era"
	// EffectStartEvent opens a ranged event for DurationDays; its OnEnd effects (set_era only) run when it ends.
	EffectStartEvent EffectKind = "start_event"
	// EffectSplitFaction / EffectMigrateFaction are stored as pending world_effects rows; faction.Effects (worker,
	// right after the world tick) announces the notice, moves the members and finalizes the split.
	EffectSplitFaction   EffectKind = "split_faction"
	EffectMigrateFaction EffectKind = "migrate_faction"
)

// Effect is one milestone effect. Only the fields of its kind are set (the loader enforces it).
type Effect struct {
	Kind         EffectKind `json:"kind"`
	EraID        string     `json:"eraId,omitempty"`
	EventID      string     `json:"eventId,omitempty"`
	DurationDays int        `json:"durationDays,omitempty"`
	OnEnd        []Effect   `json:"onEnd,omitempty"`
	From         string     `json:"from,omitempty"`
	Into         []string   `json:"into,omitempty"`
	To           string     `json:"to,omitempty"`
	NoticeDays   int        `json:"noticeDays"`
	ChooseDays   int        `json:"chooseDays"`
}

// Milestone is one story milestone. Seq = catalogue order (0 = launch milestone).
type Milestone struct {
	ID             string
	Name           string
	Announce       string // announcement template key (content/story/announcements/world-milestones.yaml)
	Seq            int
	TargetDay      int
	ThresholdMilli int64 // threshold_points_per_active × 1000 (before admin override)
	Effects        []Effect
}

// Event returns the milestone's start_event effect, if any.
func (m *Milestone) Event() (Effect, bool) {
	for _, e := range m.Effects {
		if e.Kind == EffectStartEvent {
			return e, true
		}
	}
	return Effect{}, false
}

type eventRef struct{ milestone, effect int }

type fileEra struct {
	ID   string `yaml:"id"`
	Name string `yaml:"name"`
}

type fileEffect struct {
	Kind         string       `yaml:"kind"`
	EraID        string       `yaml:"era_id"`
	EventID      string       `yaml:"event_id"`
	DurationDays *int         `yaml:"duration_days"`
	OnEnd        []fileEffect `yaml:"on_end"`
	From         string       `yaml:"from"`
	Into         []string     `yaml:"into"`
	To           string       `yaml:"to"`
	NoticeDays   *int         `yaml:"notice_days"`
	ChooseDays   *int         `yaml:"choose_days"`
}

type fileMilestone struct {
	ID                       string       `yaml:"id"`
	Name                     string       `yaml:"name"`
	TargetDay                *int         `yaml:"target_day"`
	ThresholdPointsPerActive *int64       `yaml:"threshold_points_per_active"`
	Announce                 string       `yaml:"announce"`
	Effects                  []fileEffect `yaml:"effects"`
}

var slugRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// Bounds keep designer typos from scheduling something absurd.
const (
	maxTargetDay     = 3650
	maxDurationDays  = 365
	maxNoticeDays    = 30
	maxThresholdPts  = 10_000_000
	maxEffectsPerOne = 16
)

// buildStory validates eras + milestones and fills the catalogue (§5.7 loader invariants).
func (c *Catalogue) buildStory(eras []fileEra, ms []fileMilestone, fail func(string, ...any)) {
	c.eraSeq = map[string]int{}
	c.byID = map[string]int{}
	c.events = map[string]eventRef{}
	for i, e := range eras {
		_, dup := c.eraSeq[e.ID]
		switch {
		case !slugRe.MatchString(e.ID):
			fail("eras[%d]: bad id %q", i, e.ID)
		case dup:
			fail("eras[%d]: duplicate id %q", i, e.ID)
		case e.Name == "":
			fail("eras[%d] %q: name required", i, e.ID)
		}
		c.eraSeq[e.ID] = i
		c.Eras = append(c.Eras, Era{ID: e.ID, Name: e.Name, Seq: i})
	}
	if len(ms) == 0 {
		fail("milestones: at least one milestone is required")
		return
	}
	lastEra := -1
	prevDay, prevThr := 0, int64(0)
	for i, fm := range ms {
		where := fmt.Sprintf("milestones[%d] %q", i, fm.ID)
		if !slugRe.MatchString(fm.ID) {
			fail("%s: bad id", where)
		}
		if _, dup := c.byID[fm.ID]; dup {
			fail("%s: duplicate id", where)
		}
		c.byID[fm.ID] = i
		if fm.Name == "" {
			fail("%s: name required", where)
		}
		if fm.Announce == "" {
			fail("%s: announce key required", where)
		}
		m := Milestone{ID: fm.ID, Name: fm.Name, Announce: fm.Announce, Seq: i}
		switch {
		case fm.TargetDay == nil:
			fail("%s: target_day required", where)
		case *fm.TargetDay < 0 || *fm.TargetDay > maxTargetDay:
			fail("%s: target_day out of range 0..%d", where, maxTargetDay)
		case i == 0 && *fm.TargetDay != 0:
			fail("%s: the first milestone must have target_day 0", where)
		case *fm.TargetDay < prevDay:
			fail("%s: target_day %d before the previous milestone's %d", where, *fm.TargetDay, prevDay)
		default:
			m.TargetDay = *fm.TargetDay
			prevDay = m.TargetDay
		}
		switch {
		case fm.ThresholdPointsPerActive == nil:
			fail("%s: threshold_points_per_active required", where)
		case *fm.ThresholdPointsPerActive < 0 || *fm.ThresholdPointsPerActive > maxThresholdPts:
			fail("%s: threshold_points_per_active out of range", where)
		case i > 0 && *fm.ThresholdPointsPerActive == 0:
			fail("%s: only the first milestone may have threshold 0", where)
		case *fm.ThresholdPointsPerActive*1000 < prevThr:
			fail("%s: threshold below the previous milestone's", where)
		default:
			m.ThresholdMilli = *fm.ThresholdPointsPerActive * 1000
			prevThr = m.ThresholdMilli
		}
		if len(fm.Effects) > maxEffectsPerOne {
			fail("%s: too many effects", where)
		}
		for j, fe := range fm.Effects {
			ew := fmt.Sprintf("%s effects[%d]", where, j)
			e, ok := c.effect(fe, ew, false, &lastEra, fail)
			if !ok {
				continue
			}
			if e.Kind == EffectStartEvent {
				if _, dup := c.events[e.EventID]; dup {
					fail("%s: duplicate event_id %q", ew, e.EventID)
				}
				c.events[e.EventID] = eventRef{milestone: i, effect: len(m.Effects)}
			}
			m.Effects = append(m.Effects, e)
		}
		c.Milestones = append(c.Milestones, m)
	}
}

// effect validates one effect; on_end effects (nested=true) may only be set_era.
func (c *Catalogue) effect(fe fileEffect, where string, nested bool, lastEra *int, fail func(string, ...any)) (Effect, bool) {
	e := Effect{Kind: EffectKind(fe.Kind), EraID: fe.EraID, EventID: fe.EventID, From: fe.From, Into: fe.Into, To: fe.To}
	var errs []string
	bad := func(s string) { errs = append(errs, s) }
	set := map[string]bool{"era_id": fe.EraID != "", "event_id": fe.EventID != "", "duration_days": fe.DurationDays != nil,
		"on_end": len(fe.OnEnd) > 0, "from": fe.From != "", "into": len(fe.Into) > 0, "to": fe.To != "",
		"notice_days": fe.NoticeDays != nil, "choose_days": fe.ChooseDays != nil}
	allowed := map[EffectKind][]string{
		EffectSetEra:         {"era_id"},
		EffectStartEvent:     {"event_id", "duration_days", "on_end"},
		EffectSplitFaction:   {"from", "into", "notice_days", "choose_days"},
		EffectMigrateFaction: {"from", "to", "notice_days"},
	}
	fields, known := allowed[e.Kind]
	if !known {
		fail("%s: unknown kind %q", where, fe.Kind)
		return e, false
	}
	if nested && e.Kind != EffectSetEra {
		fail("%s: on_end supports only set_era", where)
		return e, false
	}
	for f, on := range set {
		if on && !slices.Contains(fields, f) {
			bad(fmt.Sprintf("field %s not allowed for %s", f, e.Kind))
		}
	}
	days := func(p *int, name string) int {
		if p == nil {
			bad(name + " required")
			return 0
		}
		if *p < 0 || *p > maxNoticeDays {
			bad(fmt.Sprintf("%s out of range 0..%d", name, maxNoticeDays))
		}
		return *p
	}
	switch e.Kind {
	case EffectSetEra:
		seq, ok := c.eraSeq[e.EraID]
		switch {
		case !ok:
			bad(fmt.Sprintf("unknown era_id %q", e.EraID))
		case seq < *lastEra:
			bad(fmt.Sprintf("era %q goes backwards (eras must be set in catalogue order)", e.EraID))
		default:
			*lastEra = seq
		}
	case EffectStartEvent:
		if !slugRe.MatchString(e.EventID) {
			bad(fmt.Sprintf("bad event_id %q", e.EventID))
		}
		if fe.DurationDays == nil || *fe.DurationDays < 1 || *fe.DurationDays > maxDurationDays {
			bad(fmt.Sprintf("duration_days must be within 1..%d", maxDurationDays))
		} else {
			e.DurationDays = *fe.DurationDays
		}
		for k, oe := range fe.OnEnd {
			if ne, ok := c.effect(oe, fmt.Sprintf("%s on_end[%d]", where, k), true, lastEra, fail); ok {
				e.OnEnd = append(e.OnEnd, ne)
			}
		}
	case EffectSplitFaction:
		if !slugRe.MatchString(e.From) {
			bad(fmt.Sprintf("bad from %q", e.From))
		}
		if len(e.Into) < 2 {
			bad("into needs at least two factions")
		}
		seen := map[string]bool{}
		for _, id := range e.Into {
			if !slugRe.MatchString(id) || seen[id] || id == e.From {
				bad(fmt.Sprintf("into: bad, duplicate or self id %q", id))
			}
			seen[id] = true
		}
		e.NoticeDays = days(fe.NoticeDays, "notice_days")
		e.ChooseDays = days(fe.ChooseDays, "choose_days")
	case EffectMigrateFaction:
		if !slugRe.MatchString(e.From) || !slugRe.MatchString(e.To) || e.From == e.To {
			bad(fmt.Sprintf("bad from/to %q → %q", e.From, e.To))
		}
		e.NoticeDays = days(fe.NoticeDays, "notice_days")
	}
	if len(errs) > 0 {
		for _, s := range errs {
			fail("%s: %s", where, s)
		}
		return e, false
	}
	return e, true
}

// Era returns an era by id.
func (c *Catalogue) Era(id string) (Era, bool) {
	seq, ok := c.eraSeq[id]
	if !ok {
		return Era{}, false
	}
	return c.Eras[seq], true
}

// Milestone returns a milestone by id.
func (c *Catalogue) Milestone(id string) (*Milestone, bool) {
	i, ok := c.byID[id]
	if !ok {
		return nil, false
	}
	return &c.Milestones[i], true
}

// Event returns the milestone that starts eventID and its start_event effect.
func (c *Catalogue) Event(eventID string) (*Milestone, Effect, bool) {
	r, ok := c.events[eventID]
	if !ok {
		return nil, Effect{}, false
	}
	m := &c.Milestones[r.milestone]
	return m, m.Effects[r.effect], true
}

// EventEndKey is the announcement template key used when a ranged event ends.
func EventEndKey(eventID string) string { return "world.event_end." + eventID }

// AnnouncementKeys lists every template key the worker may render: each milestone's announce key and
// world.event_end.<event_id> for each start_event.
func (c *Catalogue) AnnouncementKeys() []string {
	var keys []string
	for _, m := range c.Milestones {
		keys = append(keys, m.Announce)
		for _, e := range m.Effects {
			if e.Kind == EffectStartEvent {
				keys = append(keys, EventEndKey(e.EventID))
			}
		}
	}
	return keys
}

// CheckAnnouncements fails when a key from AnnouncementKeys has no template (worker-start check, §5.7).
func (c *Catalogue) CheckAnnouncements(has func(key string) bool) error {
	var errs []error
	for _, k := range c.AnnouncementKeys() {
		if !has(k) {
			errs = append(errs, fmt.Errorf("announcement template %q missing", k))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("world: %w", errors.Join(errs...))
	}
	return nil
}

// EraSeq returns an era's catalogue order (faction.EraIndex).
func (c *Catalogue) EraSeq(id string) (int, bool) {
	seq, ok := c.eraSeq[id]
	return seq, ok
}

// EraEffectIndexBase offsets the world_effects.effect_index of era-start effects so they never collide with the
// milestone's own effects (≤ maxEffectsPerOne) under the same milestone_id.
const EraEffectIndexBase = 1000

// EraEffect is an effect scheduled when the world era first reaches (or passes) an era, whichever milestone or event
// end moved it there. Index is its stable position among all era effects (world_effects.effect_index − base).
type EraEffect struct {
	Effect Effect
	Index  int
}

// SetEraEffects registers era-start effects (today: split_faction for factions.yaml on_close: split, with targets
// computed by faction.CloseSplitTargets). content.LoadAll calls it once, after the cross-checks, before the
// catalogue is shared; the catalogue is immutable afterwards. Keys are era ids; order inside an era is kept.
func (c *Catalogue) SetEraEffects(byEra map[string][]Effect) error {
	var errs []error
	out := make([][]EraEffect, len(c.Eras))
	n := 0
	for _, era := range c.Eras { // era order → stable indexes
		for i, e := range byEra[era.ID] {
			switch {
			case e.Kind != EffectSplitFaction:
				errs = append(errs, fmt.Errorf("era %q effect[%d]: only split_faction may run at era start", era.ID, i))
			case !slugRe.MatchString(e.From) || len(e.Into) < 2:
				errs = append(errs, fmt.Errorf("era %q effect[%d]: split needs a from and at least two branches", era.ID, i))
			case e.NoticeDays < 0 || e.NoticeDays > maxNoticeDays || e.ChooseDays < 0 || e.ChooseDays > maxNoticeDays:
				errs = append(errs, fmt.Errorf("era %q effect[%d]: notice/choose days out of range 0..%d", era.ID, i, maxNoticeDays))
			}
			e.Into = slices.Clone(e.Into)
			out[era.Seq] = append(out[era.Seq], EraEffect{Effect: e, Index: n})
			n++
		}
	}
	for id := range byEra {
		if _, ok := c.eraSeq[id]; !ok {
			errs = append(errs, fmt.Errorf("era effects: unknown era %q", id))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("world: %w", errors.Join(errs...))
	}
	c.eraEffects = out
	return nil
}

// EraEffects returns the effects scheduled when the world era first reaches era seq (nil when none).
func (c *Catalogue) EraEffects(seq int) []EraEffect {
	if seq < 0 || seq >= len(c.eraEffects) {
		return nil
	}
	return c.eraEffects[seq]
}
