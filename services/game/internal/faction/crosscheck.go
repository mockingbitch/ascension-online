package faction

import (
	"errors"
	"fmt"
	"slices"

	"tiendao/game/internal/realm"
	"tiendao/game/internal/world"
)

// CrossCheck validates the catalogue against realms.yaml paths and world_milestones.yaml eras and faction effects
// (§6.4 loader rules + F7). Called by content.LoadAll.
func (c *Catalogue) CrossCheck(realms *realm.Catalogue, w *world.Catalogue) error {
	var errs []error
	fail := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	seq := func(id string) int {
		s, _ := w.EraSeq(id)
		return s
	}
	for _, f := range c.Factions {
		for _, p := range f.AllowedPaths {
			if !realms.HasPath(p) {
				fail("faction %q: unknown path %q", f.ID, p)
			}
		}
		if f.OpenFromEra != "" {
			if _, ok := w.EraSeq(f.OpenFromEra); !ok {
				fail("faction %q: unknown open_from_era %q", f.ID, f.OpenFromEra)
			}
		}
		if f.CloseFromEra != "" {
			if _, ok := w.EraSeq(f.CloseFromEra); !ok {
				fail("faction %q: unknown close_from_era %q", f.ID, f.CloseFromEra)
			} else if f.OpenFromEra != "" && seq(f.CloseFromEra) <= seq(f.OpenFromEra) {
				fail("faction %q: close_from_era must come after open_from_era", f.ID)
			}
		}
	}
	// Every path needs a faction open in the first era (new characters must be able to join from day 0).
	if len(w.Eras) > 0 {
		for _, p := range realms.Paths() {
			ok := false
			for i := range c.Factions {
				if c.Factions[i].Allows(p.ID) && c.Factions[i].OpenIn(w, 0) {
					ok = true
				}
			}
			if !ok {
				fail("path %q has no faction open in the first era %q", p.ID, w.Eras[0].ID)
			}
		}
	}
	// Faction effects: ids exist, era windows match the milestone's era, paths carry over (no conversion needed),
	// each closing faction with on_close: effect is moved by exactly one effect (F7).
	movedBy := map[string]string{}
	era := -1
	for _, m := range w.Milestones {
		for _, e := range m.Effects {
			if e.Kind == world.EffectSetEra {
				era = max(era, seq(e.EraID))
			}
		}
		for i, e := range m.Effects {
			if e.Kind != world.EffectSplitFaction && e.Kind != world.EffectMigrateFaction {
				continue
			}
			where := fmt.Sprintf("milestone %q effects[%d] %s", m.ID, i, e.Kind)
			from, ok := c.Get(e.From)
			if !ok {
				fail("%s: unknown faction %q", where, e.From)
				continue
			}
			if prev, dup := movedBy[from.ID]; dup {
				fail("%s: faction %q is already moved by %s", where, from.ID, prev)
			}
			movedBy[from.ID] = m.ID
			if from.OnClose != OnCloseEffect || from.CloseFromEra == "" || seq(from.CloseFromEra) != era {
				fail("%s: %q must close (on_close: effect) in the era this milestone sets", where, from.ID)
			}
			targets := e.Into
			if e.Kind == world.EffectMigrateFaction {
				targets = []string{e.To}
			}
			for _, id := range targets {
				t, ok := c.Get(id)
				switch {
				case !ok:
					fail("%s: unknown faction %q", where, id)
				case t.OpenFromEra == "" || seq(t.OpenFromEra) != era:
					fail("%s: %q must open in the era this milestone sets", where, id)
				case t.CloseFromEra != "":
					fail("%s: target %q must not close", where, id)
				default:
					for _, p := range from.AllowedPaths {
						if !slices.Contains(t.AllowedPaths, p) {
							fail("%s: target %q does not allow path %q of %q", where, id, p, from.ID)
						}
					}
				}
			}
		}
	}
	for i := range c.Factions {
		f := &c.Factions[i]
		if f.OnClose == OnCloseEffect {
			if _, ok := movedBy[f.ID]; !ok {
				fail("faction %q: on_close: effect but no split_faction/migrate_faction moves it", f.ID)
			}
		}
		if f.OnClose != OnCloseSplit {
			continue
		}
		// on_close: split (decisions 2026-11-07): targets are computed, never listed. A split needs two branches
		// (one would be a migrate), and one branch that never closes so that a default always has somewhere to go
		// once the other branches' own eras have started (OpenBranches).
		targets := c.CloseSplitTargets(w, f)
		if len(targets) < 2 {
			fail("faction %q: on_close: split needs at least two factions open in era %q allowing its paths, found %v", f.ID, f.CloseFromEra, targets)
		}
		lasting := false
		for _, id := range targets {
			if t, _ := c.Get(id); t.CloseFromEra == "" {
				lasting = true
			}
		}
		if !lasting {
			fail("faction %q: on_close: split needs a target that never closes, got %v", f.ID, targets)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("faction: %w", errors.Join(errs...))
	}
	return nil
}

// EraSplits builds the split_faction effects of every on_close: split faction, keyed by its close_from_era, with the
// computed targets (CloseSplitTargets, catalogue order = default tie-break order). content.LoadAll registers them on
// the world catalogue (world.Catalogue.SetEraEffects) after CrossCheck passed.
func (c *Catalogue) EraSplits(w *world.Catalogue) map[string][]world.Effect {
	out := map[string][]world.Effect{}
	for i := range c.Factions {
		f := &c.Factions[i]
		if f.OnClose != OnCloseSplit || f.Split == nil {
			continue
		}
		out[f.CloseFromEra] = append(out[f.CloseFromEra], world.Effect{Kind: world.EffectSplitFaction, From: f.ID,
			Into: c.CloseSplitTargets(w, f), NoticeDays: f.Split.NoticeDays, ChooseDays: f.Split.ChooseDays})
	}
	return out
}
