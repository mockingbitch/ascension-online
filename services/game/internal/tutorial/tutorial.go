// Package tutorial is Vân Khê's guided "lần đầu" (decisions 2026-11-09 #1, GĐ-B): the scripts in
// content/story/tutorial/*.yaml and the per-character progress (tutorial_progress / tutorial_prefs).
//
// NO GAMEPLAY EFFECT: a step only tells the player something. Nothing here grants items, currency, tu vi or điểm
// duyên, and nothing else reads it. Which step shows is decided by the client from server facts (fxBus events carry
// the server's values; windows/scenes the player opened); the server only stores "seen" so it carries across devices.
package tutorial

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"

	"tiendao/game/internal/realm"
)

// MaxLineRunes: one line of the guide (story-writing rules: quest step text ≤ 200 chars), after placeholders.
const MaxLineRunes = 200

var (
	idRe        = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	spotlightRe = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	placeRe     = regexp.MustCompile(`\{[a-z_]+\}`)
	// Expressions = art slots characters/guide/<expr> (+ the SVG fallback draws each).
	Exprs = []string{"neutral", "smile", "point", "worried", "proud", "surprised"}
	// Flows: onboarding steps run in /create before a character exists (client-only progress); play steps are stored.
	flows = []string{"onboarding", "play"}
	// Triggers the web knows (lib/game/tutorial.svelte.ts). Each comes from a server fact or a place the player opened:
	//   onboarding.<step>          a wizard step of /create
	//   play.enter                 the first /play load of a character
	//   fx.settled / fx.stage_up   fxBus events emitted after the server's answer (server values)
	//   fx.realm_reached           fxBus breakthrough success (major) — shown once the celebration closed
	//   breakthrough.ready         the Đột Phá window shows a server preview that is ready and not a tribulation
	//   breakthrough.tribulation   the Đột Phá window shows a server preview with tribulation: true
	//   window.<kind>[.<tab>]      a window the player opened (faction, vip, companion, inventory.shop)
	//   scene.<id>                 a scene the player entered (tu_tieu_cung)
	//   mode.trial                 a mini game lobby the player opened
	triggers = []string{"onboarding.tu_tieu_cung", "onboarding.the_luc", "onboarding.ngoai_hinh", "onboarding.dao_hieu",
		"play.enter", "fx.settled", "fx.stage_up", "fx.realm_reached", "breakthrough.ready", "breakthrough.tribulation",
		"window.faction", "window.vip", "window.companion", "window.inventory.shop", "scene.tu_tieu_cung", "mode.trial"}
)

// Line is one speech of the guide.
type Line struct {
	Expr      string `yaml:"expr" json:"expr"`
	Text      string `yaml:"text" json:"text"`
	Spotlight string `yaml:"spotlight,omitempty" json:"spotlight,omitempty"` // data-tutorial="<id>" target to highlight
}

// Step is one guided moment.
type Step struct {
	ID      string `yaml:"id" json:"id"`
	Flow    string `yaml:"flow" json:"flow"`
	Trigger string `yaml:"trigger" json:"trigger"`
	Title   string `yaml:"title" json:"title"`
	Lines   []Line `yaml:"lines" json:"lines"`
}

// Script is every tutorial file merged (file name order), immutable once loaded.
type Script struct {
	Guide string `json:"guide"`
	Name  string `json:"name"`
	Steps []Step `json:"steps"`
	byID  map[string]*Step
}

type file struct {
	Version int    `yaml:"version"`
	Guide   string `yaml:"guide"`
	Name    string `yaml:"name"`
	Steps   []Step `yaml:"steps"`
}

// Vars fills the {placeholders} of the scripts from the catalogues, so game numbers live in YAML once
// (realms.yaml tribulation): {loss_pct}, {guarantee_after}, {cooldown_hours}.
func Vars(r *realm.Catalogue) map[string]string {
	t := r.Tribulation()
	return map[string]string{
		"loss_pct":        strconv.FormatInt(t.LossBP/100, 10),
		"guarantee_after": strconv.Itoa(t.GuaranteeAfter),
		"cooldown_hours":  strconv.FormatInt(t.CooldownSeconds/3600, 10),
	}
}

// Load reads every *.yaml in dir and validates it; placeholders are substituted with vars (unknown ones fail).
func Load(dir string, vars map[string]string) (*Script, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("tutorial: no scripts in %s", dir)
	}
	slices.Sort(paths)
	s := &Script{byID: map[string]*Step{}}
	var errs []error
	fail := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			return nil, err
		}
		var raw file
		dec := yaml.NewDecoder(f)
		dec.KnownFields(true)
		err = dec.Decode(&raw)
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("tutorial: %s: %w", filepath.Base(p), err)
		}
		where := filepath.Base(p)
		if raw.Version != 1 {
			fail("%s: unsupported version %d", where, raw.Version)
		}
		if raw.Guide == "" || raw.Name == "" {
			fail("%s: guide and name required", where)
		}
		if s.Guide == "" {
			s.Guide, s.Name = raw.Guide, raw.Name
		} else if raw.Guide != s.Guide || raw.Name != s.Name {
			fail("%s: every tutorial file has the same guide (%s)", where, s.Guide)
		}
		for _, st := range raw.Steps {
			sw := fmt.Sprintf("%s step %q", where, st.ID)
			if !idRe.MatchString(st.ID) {
				fail("%s: invalid id", sw)
			}
			if _, dup := s.byID[st.ID]; dup {
				fail("%s: duplicate id", sw)
			}
			if !slices.Contains(flows, st.Flow) {
				fail("%s: flow must be onboarding or play", sw)
			}
			if !slices.Contains(triggers, st.Trigger) {
				fail("%s: unknown trigger %q", sw, st.Trigger)
			} else if strings.HasPrefix(st.Trigger, "onboarding.") != (st.Flow == "onboarding") {
				fail("%s: onboarding triggers belong to the onboarding flow (and only those)", sw)
			}
			if st.Title == "" || len(st.Lines) == 0 {
				fail("%s: title and lines required", sw)
			}
			for i := range st.Lines {
				l := &st.Lines[i]
				lw := fmt.Sprintf("%s line %d", sw, i+1)
				if !slices.Contains(Exprs, l.Expr) {
					fail("%s: unknown expr %q", lw, l.Expr)
				}
				if l.Spotlight != "" && !spotlightRe.MatchString(l.Spotlight) {
					fail("%s: invalid spotlight %q", lw, l.Spotlight)
				}
				l.Text = placeRe.ReplaceAllStringFunc(l.Text, func(m string) string {
					if v, ok := vars[m[1:len(m)-1]]; ok {
						return v
					}
					fail("%s: unknown placeholder %s", lw, m)
					return m
				})
				if n := utf8.RuneCountInString(l.Text); n == 0 || n > MaxLineRunes {
					fail("%s: %d characters (1..%d)", lw, n, MaxLineRunes)
				}
			}
			s.Steps = append(s.Steps, st)
			s.byID[st.ID] = &s.Steps[len(s.Steps)-1]
		}
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("tutorial: invalid scripts: %w", errors.Join(errs...))
	}
	for i := range s.Steps {
		s.byID[s.Steps[i].ID] = &s.Steps[i]
	}
	// Every play trigger is covered once at most (a trigger shows one step).
	seen := map[string]string{}
	for _, st := range s.Steps {
		if prev, ok := seen[st.Trigger]; ok {
			errs = append(errs, fmt.Errorf("tutorial: steps %q and %q share trigger %q", prev, st.ID, st.Trigger))
		}
		seen[st.Trigger] = st.ID
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("tutorial: invalid scripts: %w", errors.Join(errs...))
	}
	return s, nil
}

// Stored reports whether a step id is a stored (play) step.
func (s *Script) Stored(id string) bool {
	st, ok := s.byID[id]
	return ok && st.Flow == "play"
}
