// Package faction holds the faction catalogue (factions.yaml), the pure membership rules (join, switch, split choice,
// default split branch) and the services that own character_faction / faction_history — data model §6.
package faction

import (
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"slices"
	"time"

	"gopkg.in/yaml.v3"

	"tiendao/game/internal/realm"
)

// OnClose says what happens to members when a faction closes.
type OnClose string

const (
	// OnCloseEffect: a split_faction / migrate_faction world effect moves the members (cross-checked in content).
	OnCloseEffect OnClose = "effect"
	// OnCloseKeepMembers: members stay in the closed faction (no effect decided); they may still switch out.
	OnCloseKeepMembers OnClose = "keep_members"
	// OnCloseSplit: when the world era first reaches close_from_era, the members are split (split_faction, decision 9
	// rules) into every faction open in that era that allows all of this faction's paths — targets computed from the
	// catalogue (CloseSplitTargets), scheduled by the world tick (world.Catalogue.SetEraEffects, decisions 2026-11-07).
	OnCloseSplit OnClose = "split"
)

// CloseSplit is the notice/choice window of an on_close: split faction (same meaning as split_faction's fields).
type CloseSplit struct {
	NoticeDays int `json:"noticeDays"`
	ChooseDays int `json:"chooseDays"`
}

// Faction is one catalogue entry. Immutable after Load.
type Faction struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	NamePlaceholder bool           `json:"namePlaceholder"`
	AllowedPaths    []realm.PathID `json:"allowedPaths"`
	OpenFromEra     string         `json:"openFromEra,omitempty"` // "" = never open (canon not decided)
	CloseFromEra    string         `json:"closeFromEra,omitempty"`
	OnClose         OnClose        `json:"-"`
	Split           *CloseSplit    `json:"-"`     // on_close: split only
	Color           string         `json:"color"` // design token suffix: var(--color-<color>)
	Motto           string         `json:"motto,omitempty"`
	Pitch           string         `json:"pitch,omitempty"`
	Seq             int            `json:"-"` // catalogue order (tie-break order)
}

// Allows reports whether p is one of the faction's paths.
func (f *Faction) Allows(p realm.PathID) bool { return slices.Contains(f.AllowedPaths, p) }

// Settings are the factions.yaml settings block.
type Settings struct {
	SwitchCooldown time.Duration
	FirstJoinFree  bool
}

// Catalogue is the validated faction catalogue. Immutable after Load.
type Catalogue struct {
	Settings Settings
	Factions []Faction
	byID     map[string]int
}

// Get returns a faction by id.
func (c *Catalogue) Get(id string) (*Faction, bool) {
	i, ok := c.byID[id]
	if !ok {
		return nil, false
	}
	return &c.Factions[i], true
}

// IDs lists faction ids in catalogue order.
func (c *Catalogue) IDs() []string {
	out := make([]string, len(c.Factions))
	for i, f := range c.Factions {
		out[i] = f.ID
	}
	return out
}

// Placeholders lists factions whose name is still a placeholder (release gate, decision 12).
func (c *Catalogue) Placeholders() []string {
	var out []string
	for _, f := range c.Factions {
		if f.NamePlaceholder {
			out = append(out, f.ID)
		}
	}
	return out
}

type fileFaction struct {
	ID              string         `yaml:"id"`
	Name            string         `yaml:"name"`
	NamePlaceholder bool           `yaml:"name_placeholder"`
	AllowedPaths    []realm.PathID `yaml:"allowed_paths"`
	OpenFromEra     string         `yaml:"open_from_era"`
	CloseFromEra    string         `yaml:"close_from_era"`
	OnClose         string         `yaml:"on_close"`
	Split           *struct {
		NoticeDays *int `yaml:"notice_days"`
		ChooseDays *int `yaml:"choose_days"`
	} `yaml:"split"`
	Color string `yaml:"color"`
	Motto string `yaml:"motto"`
	Pitch string `yaml:"pitch"`
}

type file struct {
	Version  int `yaml:"version"`
	Settings struct {
		SwitchCooldownSeconds *int64 `yaml:"switch_cooldown_seconds"`
		FirstJoinFree         *bool  `yaml:"first_join_free"`
	} `yaml:"settings"`
	Factions []fileFaction `yaml:"factions"`
}

var (
	slugRe  = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	colorRe = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
)

// Bounds: cooldown ≤ 90 days; display text short enough for a card.
const (
	maxCooldownSeconds = 90 * 24 * 3600
	maxPitchRunes      = 80
	maxMottoRunes      = 80
	maxSplitDays       = 30 // same bound as world_milestones.yaml notice_days / choose_days
)

// LoadFile parses and validates factions.yaml.
func LoadFile(path string) (*Catalogue, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Load(f)
}

// Load parses and validates faction YAML. Era ids, paths and world effects are cross-checked in content.LoadAll
// (CheckEras / CheckPaths / CheckEffects).
func Load(r io.Reader) (*Catalogue, error) {
	var raw file
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("faction: decode: %w", err)
	}
	var errs []error
	fail := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	if raw.Version != 1 {
		fail("unsupported version %d", raw.Version)
	}
	c := &Catalogue{byID: map[string]int{}}
	switch s := raw.Settings.SwitchCooldownSeconds; {
	case s == nil:
		fail("settings.switch_cooldown_seconds required")
	case *s < 0 || *s > maxCooldownSeconds:
		fail("settings.switch_cooldown_seconds must be within 0..%d", maxCooldownSeconds)
	default:
		c.Settings.SwitchCooldown = time.Duration(*s) * time.Second
	}
	if raw.Settings.FirstJoinFree == nil {
		fail("settings.first_join_free required")
	} else {
		c.Settings.FirstJoinFree = *raw.Settings.FirstJoinFree
	}
	if len(raw.Factions) == 0 {
		fail("factions: at least one faction is required")
	}
	for i, ff := range raw.Factions {
		where := fmt.Sprintf("factions[%d] %q", i, ff.ID)
		if !slugRe.MatchString(ff.ID) {
			fail("%s: bad id", where)
		}
		if _, dup := c.byID[ff.ID]; dup {
			fail("%s: duplicate id", where)
		}
		c.byID[ff.ID] = i
		if ff.Name == "" {
			fail("%s: name required", where)
		}
		if len(ff.AllowedPaths) == 0 {
			fail("%s: allowed_paths must not be empty", where)
		}
		seen := map[realm.PathID]bool{}
		for _, p := range ff.AllowedPaths {
			if seen[p] || !slugRe.MatchString(string(p)) {
				fail("%s: bad or duplicate path %q", where, p)
			}
			seen[p] = true
		}
		if ff.OpenFromEra != "" && !slugRe.MatchString(ff.OpenFromEra) {
			fail("%s: bad open_from_era %q", where, ff.OpenFromEra)
		}
		if ff.CloseFromEra != "" && !slugRe.MatchString(ff.CloseFromEra) {
			fail("%s: bad close_from_era %q", where, ff.CloseFromEra)
		}
		if ff.CloseFromEra != "" && ff.OpenFromEra == "" {
			fail("%s: close_from_era without open_from_era", where)
		}
		oc := OnClose(ff.OnClose)
		switch {
		case ff.CloseFromEra == "" && oc != "":
			fail("%s: on_close only applies with close_from_era", where)
		case ff.CloseFromEra != "" && oc != OnCloseEffect && oc != OnCloseKeepMembers && oc != OnCloseSplit:
			fail("%s: on_close must be %q, %q or %q", where, OnCloseEffect, OnCloseKeepMembers, OnCloseSplit)
		}
		var split *CloseSplit
		switch {
		case oc == OnCloseSplit && ff.Split == nil:
			fail("%s: on_close: split needs split: { notice_days, choose_days }", where)
		case oc != OnCloseSplit && ff.Split != nil:
			fail("%s: split only applies with on_close: split", where)
		case ff.Split != nil:
			split = &CloseSplit{}
			for _, d := range []struct {
				name string
				in   *int
				out  *int
			}{{"notice_days", ff.Split.NoticeDays, &split.NoticeDays}, {"choose_days", ff.Split.ChooseDays, &split.ChooseDays}} {
				switch {
				case d.in == nil:
					fail("%s: split.%s required", where, d.name)
				case *d.in < 0 || *d.in > maxSplitDays:
					fail("%s: split.%s out of range 0..%d", where, d.name, maxSplitDays)
				default:
					*d.out = *d.in
				}
			}
		}
		if !colorRe.MatchString(ff.Color) {
			fail("%s: color must be a design token suffix (e.g. aura-jade)", where)
		}
		if n := len([]rune(ff.Pitch)); n > maxPitchRunes {
			fail("%s: pitch is %d chars (max %d)", where, n, maxPitchRunes)
		}
		if n := len([]rune(ff.Motto)); n > maxMottoRunes {
			fail("%s: motto is %d chars (max %d)", where, n, maxMottoRunes)
		}
		c.Factions = append(c.Factions, Faction{ID: ff.ID, Name: ff.Name, NamePlaceholder: ff.NamePlaceholder,
			AllowedPaths: slices.Clone(ff.AllowedPaths), OpenFromEra: ff.OpenFromEra, CloseFromEra: ff.CloseFromEra,
			OnClose: oc, Split: split, Color: ff.Color, Motto: ff.Motto, Pitch: ff.Pitch, Seq: i})
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("faction: invalid catalogue: %w", errors.Join(errs...))
	}
	return c, nil
}

// ───────────────────────────── Eras ─────────────────────────────

// EraIndex orders eras (world.Catalogue implements it).
type EraIndex interface {
	EraSeq(id string) (int, bool)
}

// OpenIn reports whether players may join f while the world is in era seq cur (pure). An omitted open_from_era
// never opens; close_from_era closes from that era on.
func (f *Faction) OpenIn(eras EraIndex, cur int) bool {
	if f.OpenFromEra == "" {
		return false
	}
	open, ok := eras.EraSeq(f.OpenFromEra)
	if !ok || cur < open {
		return false
	}
	if f.CloseFromEra != "" {
		if cl, ok := eras.EraSeq(f.CloseFromEra); ok && cur >= cl {
			return false
		}
	}
	return true
}

// CloseSplitTargets lists, in catalogue order (= default tie-break order), the factions an on_close: split faction's
// members are split into: every OTHER faction open in the era f closes in (so not closing there itself) that allows
// all of f's paths, so no member needs a path conversion. Pure; nil when f doesn't close.
func (c *Catalogue) CloseSplitTargets(eras EraIndex, f *Faction) []string {
	if f.CloseFromEra == "" {
		return nil
	}
	at, ok := eras.EraSeq(f.CloseFromEra)
	if !ok {
		return nil
	}
	var out []string
	for i := range c.Factions {
		t := &c.Factions[i]
		if t.ID == f.ID || !t.OpenIn(eras, at) {
			continue
		}
		all := true
		for _, p := range f.AllowedPaths {
			all = all && t.Allows(p)
		}
		if all {
			out = append(out, t.ID)
		}
	}
	return out
}

// OpenBranches keeps the branches of a split that are still open in era cur (catalogue order kept). A computed
// close split may target a faction that closes in a later era (e.g. huyen_khung_mon): once that era has started, a
// member must not be placed there after its own split/migrate already moved its members, so choices and defaults
// skip it. Falls back to all of into when none is open (the loader guarantees one never-closing branch). Pure.
func (c *Catalogue) OpenBranches(eras EraIndex, into []string, cur int) []string {
	var out []string
	for _, id := range into {
		if f, ok := c.Get(id); ok && f.OpenIn(eras, cur) {
			out = append(out, id)
		}
	}
	if len(out) == 0 {
		return slices.Clone(into)
	}
	return out
}

// ───────────────────────────── Pure rules ─────────────────────────────

// Domain errors (mapped to HTTP in httpapi).
var (
	ErrUnknownFaction = errors.New("faction: unknown faction")
	ErrFactionClosed  = errors.New("faction: faction is not open in the current era")
	ErrPathNotAllowed = errors.New("faction: path not allowed by this faction")
	ErrAlreadyMember  = errors.New("faction: already in a faction")
	ErrNotMember      = errors.New("faction: not in a faction")
	ErrSameFaction    = errors.New("faction: already in this faction")
	ErrCooldown       = errors.New("faction: switch cooldown not over")
	ErrLocked         = errors.New("faction: membership is locked")
	ErrNoPendingSplit = errors.New("faction: no split choice pending")
	ErrNotABranch     = errors.New("faction: not a branch of the pending split")
	ErrChoiceClosed   = errors.New("faction: the split choice window is closed")
)

// Kind is a player-initiated membership change.
type Kind string

const (
	KindJoin   Kind = "join"
	KindSwitch Kind = "switch"
)

// Member is the part of character_faction the rules need (nil = no faction yet).
type Member struct {
	FactionID    string
	LastSwitchAt *time.Time
	LockReason   string
	LockedUntil  *time.Time
}

// Locked reports whether a lock is active at now (NULL locked_until = permanent).
func (m *Member) Locked(now time.Time) bool {
	return m != nil && m.LockReason != "" && (m.LockedUntil == nil || now.Before(*m.LockedUntil))
}

// CanSwitchAt is when the next player switch is allowed (nil = now).
func (c *Catalogue) CanSwitchAt(m *Member) *time.Time {
	if m == nil || m.LastSwitchAt == nil {
		return nil
	}
	t := m.LastSwitchAt.Add(c.Settings.SwitchCooldown)
	return &t
}

// Request is a player's join/switch intent with the context the rules need.
type Request struct {
	Kind     Kind
	Member   *Member // nil = no faction
	Target   string
	EraSeq   int          // current era (world_state.era_id; NULL = first era → 0)
	CurPath  realm.PathID // derived from the character's stage
	WantPath realm.PathID // optional path choice ("" = keep the current path when allowed)
	Now      time.Time
}

// CanJoin validates a join or switch (F2–F5) and returns the path the character must be on afterwards (pure).
func (c *Catalogue) CanJoin(eras EraIndex, r Request) (realm.PathID, error) {
	f, ok := c.Get(r.Target)
	if !ok {
		return "", ErrUnknownFaction
	}
	switch r.Kind {
	case KindJoin:
		if r.Member != nil {
			return "", ErrAlreadyMember
		}
	case KindSwitch:
		if r.Member == nil {
			return "", ErrNotMember
		}
		if r.Member.FactionID == r.Target {
			return "", ErrSameFaction
		}
		if r.Member.Locked(r.Now) {
			return "", ErrLocked
		}
		if at := c.CanSwitchAt(r.Member); at != nil && r.Now.Before(*at) {
			return "", ErrCooldown
		}
	default:
		return "", fmt.Errorf("faction: unknown change kind %q", r.Kind)
	}
	if !f.OpenIn(eras, r.EraSeq) {
		return "", ErrFactionClosed
	}
	return ResolvePath(f, r.CurPath, r.WantPath)
}

// ResolvePath picks the path after joining f: the requested one (must be allowed), else the current one when
// allowed, else the faction's first path (pure).
func ResolvePath(f *Faction, cur, want realm.PathID) (realm.PathID, error) {
	switch {
	case want != "":
		if !f.Allows(want) {
			return "", ErrPathNotAllowed
		}
		return want, nil
	case f.Allows(cur):
		return cur, nil
	default:
		return f.AllowedPaths[0], nil
	}
}

// DefaultBranch is the branch with the fewest members; ties go to the earliest branch in `into` order (F8, pure).
func DefaultBranch(into []string, members map[string]int64) string {
	best := ""
	var bestN int64
	for _, id := range into {
		if n := members[id]; best == "" || n < bestN {
			best, bestN = id, n
		}
	}
	return best
}

// AssignDefaults places n undecided members one by one, each into the branch that currently has the fewest members
// placed by this split (`placed`: choices + earlier defaults; ties → `into` order), so the branches end as even as
// possible (decision 9 as refined 2026-11-07, H2). Callers pass members in character_id order. It does not modify
// `placed`. Pure.
func AssignDefaults(into []string, placed map[string]int64, n int) []string {
	counts := make(map[string]int64, len(into))
	for _, id := range into {
		counts[id] = placed[id]
	}
	out := make([]string, n)
	for i := range out {
		b := DefaultBranch(into, counts)
		out[i] = b
		counts[b]++
	}
	return out
}
