package realm

import (
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"

	"tiendao/game/internal/fixedpoint"
)

type fileStage struct {
	ID   string `yaml:"id"`
	Name string `yaml:"name"`
}

type fileStageSet struct {
	PrefixRealm bool        `yaml:"prefix_realm"`
	Stages      []fileStage `yaml:"stages"`
}

type filePhaseSet struct {
	Phases     []fileStage `yaml:"phases"`
	LayerNames []string    `yaml:"layer_names"`
}

type fileStagesRef struct {
	Set    string `yaml:"set"`
	Phases string `yaml:"phases"`
	Layers int    `yaml:"layers"`
}

type fileExtraStage struct {
	ID             string `yaml:"id"`
	Name           string `yaml:"name"`
	RequiredPoints int64  `yaml:"required_points"`
}

type fileMajor struct {
	SuccessBP       *int64            `yaml:"success_bp"`
	PityStepBP      int64             `yaml:"pity_step_bp"`
	CooldownSeconds int64             `yaml:"cooldown_seconds"`
	Requirements    []ItemRequirement `yaml:"requirements"`
}

type fileTier struct {
	Tier              int              `yaml:"tier"`
	EntryPoints       int64            `yaml:"entry_points"`
	SpanPoints        int64            `yaml:"span_points"`
	StageWeights      []int64          `yaml:"stage_weights"`
	Reserved          bool             `yaml:"reserved"`
	Stages            fileStagesRef    `yaml:"stages"`
	ExtraStages       []fileExtraStage `yaml:"extra_stages"`
	MajorBreakthrough *fileMajor       `yaml:"major_breakthrough"`
	Visual            struct {
		FxTier int `yaml:"fx_tier"`
	} `yaml:"visual"`
}

type filePath struct {
	ID              PathID `yaml:"id"`
	Name            string `yaml:"name"`
	Default         bool   `yaml:"default"`
	NamePlaceholder bool   `yaml:"name_placeholder"`
}

type fileRealm struct {
	ID              string `yaml:"id"`
	Path            PathID `yaml:"path"`
	Tier            int    `yaml:"tier"`
	Name            string `yaml:"name"`
	NamePlaceholder bool   `yaml:"name_placeholder"`
	Title           string `yaml:"title"`
	Visual          Visual `yaml:"visual"`
}

type file struct {
	Version           int                     `yaml:"version"`
	Settings          Settings                `yaml:"settings"`
	Paths             []filePath              `yaml:"paths"`
	Tribulation       Tribulation             `yaml:"tribulation"`
	StageSets         map[string]fileStageSet `yaml:"stage_sets"`
	PhaseSets         map[string]filePhaseSet `yaml:"phase_sets"`
	MinorBreakthrough struct {
		Auto bool `yaml:"auto"`
	} `yaml:"minor_breakthrough"`
	Tiers  []fileTier  `yaml:"tiers"`
	Realms []fileRealm `yaml:"realms"`
}

var slugRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// MaxOverflowCapBP bounds the bottleneck overflow (and therefore breakthrough carry-over).
const MaxOverflowCapBP = 15_000

// LoadFile parses and validates a realms.yaml file.
func LoadFile(path string) (*Catalogue, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Load(f)
}

// Load parses and validates realm catalogue YAML.
func Load(r io.Reader) (*Catalogue, error) {
	var raw file
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("realm: decode: %w", err)
	}
	c, err := build(raw)
	if err != nil {
		return nil, fmt.Errorf("realm: invalid catalogue: %w", err)
	}
	return c, nil
}

// genStage is a stage template generated for a tier (path-independent).
type genStage struct {
	id, name      string
	requiredMilli int64
	prefixRealm   bool
}

func bpOK(v int64) bool { return v >= 0 && v <= fixedpoint.BPScale }

func build(raw file) (*Catalogue, error) {
	var errs []error
	fail := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }

	// 1, 15: version + settings.
	if raw.Version != 2 {
		fail("unsupported version %d (want 2)", raw.Version)
	}
	if raw.Settings.OverflowCapBP < fixedpoint.BPScale || raw.Settings.OverflowCapBP > MaxOverflowCapBP {
		fail("settings.overflow_cap_bp must be within 10000..%d", MaxOverflowCapBP)
	}
	if !bpOK(raw.Settings.PityMaxBonusBP) {
		fail("settings.pity_max_bonus_bp out of range")
	}
	// 11: only automatic minors.
	if !raw.MinorBreakthrough.Auto {
		fail("minor_breakthrough.auto must be true (v2 supports only automatic minor stages)")
	}

	// 2, 3: paths.
	c := &Catalogue{
		Version: raw.Version, Settings: raw.Settings, trib: raw.Tribulation,
		ladder: map[PathID][]*Stage{}, byID: map[string]*Stage{}, byPathTier: map[PathID]map[int]*Realm{},
	}
	defaults := 0
	for i, p := range raw.Paths {
		if !slugRe.MatchString(string(p.ID)) {
			fail("path[%d]: invalid id %q", i, p.ID)
		}
		if _, dup := c.ladder[p.ID]; dup {
			fail("path %q: duplicate id", p.ID)
		}
		if p.Name == "" {
			fail("path %q: name required", p.ID)
		}
		c.ladder[p.ID] = nil
		c.byPathTier[p.ID] = map[int]*Realm{}
		c.paths = append(c.paths, Path{ID: p.ID, Name: p.Name, Default: p.Default, NamePlaceholder: p.NamePlaceholder})
		if p.Default {
			defaults++
			c.defaultPath = p.ID
		}
	}
	if len(raw.Paths) == 0 || defaults != 1 {
		fail("exactly one path must be default: true")
	}

	// Stage/phase set IDs.
	for id, ss := range raw.StageSets {
		if !slugRe.MatchString(id) || len(ss.Stages) == 0 {
			fail("stage_set %q: invalid id or no stages", id)
		}
		seen := map[string]bool{}
		for _, s := range ss.Stages {
			if !slugRe.MatchString(s.ID) || seen[s.ID] || s.Name == "" {
				fail("stage_set %q: invalid/duplicate stage %q", id, s.ID)
			}
			seen[s.ID] = true
		}
	}
	for id, ps := range raw.PhaseSets {
		if !slugRe.MatchString(id) || len(ps.Phases) == 0 {
			fail("phase_set %q: invalid id or no phases", id)
		}
		seen := map[string]bool{}
		for _, p := range ps.Phases {
			if !slugRe.MatchString(p.ID) || seen[p.ID] || p.Name == "" {
				fail("phase_set %q: invalid/duplicate phase %q", id, p.ID)
			}
			seen[p.ID] = true
		}
	}

	// 4, 5, 6: tiers.
	if len(raw.Tiers) == 0 {
		return nil, errors.Join(append(errs, errors.New("no tiers"))...)
	}
	lastPlayable := 0
	prevFx := 0
	for i, t := range raw.Tiers {
		// effect tier (display only): 1..MaxFxTier, never decreasing up the ladder
		if fx := t.Visual.FxTier; fx < 1 || fx > MaxFxTier {
			fail("tier %d: visual.fx_tier must be within 1..%d", t.Tier, MaxFxTier)
		} else if fx < prevFx {
			fail("tier %d: visual.fx_tier %d is below the previous tier's %d (effects never get smaller)", t.Tier, fx, prevFx)
		} else {
			prevFx = fx
		}
		if t.Tier != i+1 {
			fail("tiers[%d]: tier %d must equal position %d (contiguous from 1)", i, t.Tier, i+1)
		}
		if t.Reserved {
			if t.EntryPoints != 0 || t.SpanPoints != 0 || t.MajorBreakthrough != nil || len(t.ExtraStages) > 0 {
				fail("reserved tier %d: must not set points, extra stages or a breakthrough rule", t.Tier)
			}
			continue
		}
		if lastPlayable != i {
			fail("tier %d: playable tiers must come before every reserved tier", t.Tier)
		}
		lastPlayable = i + 1
	}
	c.lastPlayableTier = lastPlayable
	if lastPlayable == 0 {
		return nil, errors.Join(append(errs, errors.New("no playable tier"))...)
	}
	trib := raw.Tribulation
	if raw.Tiers[0].EntryPoints != 0 {
		fail("tier 1: entry_points must be 0")
	}

	tierStages := map[int][]genStage{}
	tierRule := map[int]BreakthroughRule{}
	cumBefore := map[int]int64{} // tier → cumulative milli at entry
	for i := range raw.Tiers {
		t := raw.Tiers[i]
		last := t.Tier == lastPlayable
		// Stage templates.
		var names []genStage
		ref := t.Stages
		switch {
		case ref.Set != "" && ref.Phases == "" && ref.Layers == 0:
			ss, ok := raw.StageSets[ref.Set]
			if !ok {
				fail("tier %d: unknown stage set %q", t.Tier, ref.Set)
				continue
			}
			for _, s := range ss.Stages {
				names = append(names, genStage{id: s.ID, name: s.Name, prefixRealm: ss.PrefixRealm})
			}
		case ref.Set == "" && ref.Phases != "":
			ps, ok := raw.PhaseSets[ref.Phases]
			if !ok {
				fail("tier %d: unknown phase set %q", t.Tier, ref.Phases)
				continue
			}
			if ref.Layers < 1 || ref.Layers > len(ps.LayerNames) {
				fail("tier %d: layers must be 1..%d", t.Tier, len(ps.LayerNames))
				continue
			}
			for _, ph := range ps.Phases {
				for l := 1; l <= ref.Layers; l++ {
					g := genStage{id: ph.ID, name: ph.Name}
					if ref.Layers > 1 {
						g.id = fmt.Sprintf("%s_%d", ph.ID, l)
						g.name = ph.Name + " " + ps.LayerNames[l-1]
					}
					names = append(names, g)
				}
			}
		default:
			fail("tier %d: stages must be exactly one of {set} or {phases, layers}", t.Tier)
			continue
		}
		if t.Reserved {
			tierStages[t.Tier] = names
			continue
		}

		// Span and generated requirements (whole points; remainder on the last stages).
		var span int64
		switch {
		case last && t.SpanPoints <= 0:
			fail("tier %d (last playable): span_points must be > 0", t.Tier)
		case !last && t.SpanPoints != 0:
			fail("tier %d: only the last playable tier may set span_points", t.Tier)
		}
		if last {
			span = t.SpanPoints
		} else {
			span = raw.Tiers[i+1].EntryPoints - t.EntryPoints
			if span <= 0 {
				fail("tier %d: entry_points must strictly increase (next tier %d)", t.Tier, raw.Tiers[i+1].EntryPoints)
				continue
			}
		}
		n := int64(len(names))
		if span < n {
			fail("tier %d: span %d < %d stages (every stage needs ≥ 1 point)", t.Tier, span, n)
			continue
		}
		weights, ok := stageWeights(t.StageWeights, len(names))
		if !ok {
			fail("tier %d: stage_weights must have one value ≥ 1 per stage (%d), non-decreasing", t.Tier, n)
			continue
		}
		var sum int64
		for k, req := range splitSpan(span, weights) {
			if req < 1 {
				fail("tier %d: stage %s gets %d points (every stage needs ≥ 1 point)", t.Tier, names[k].id, req)
			}
			names[k].requiredMilli = req * PointMilli
			sum += req
		}
		if sum != span {
			fail("tier %d: generated requirements sum %d != span %d", t.Tier, sum, span)
		}
		// Extra stages: append-only tail of the last playable tier.
		if len(t.ExtraStages) > 0 && !last {
			fail("tier %d: extra_stages only allowed on the last playable tier", t.Tier)
		}
		seen := map[string]bool{}
		for _, g := range names {
			seen[g.id] = true
		}
		for _, e := range t.ExtraStages {
			if !slugRe.MatchString(e.ID) || seen[e.ID] || e.RequiredPoints <= 0 || e.Name == "" {
				fail("tier %d: invalid extra stage %+v", t.Tier, e)
				continue
			}
			seen[e.ID] = true
			names = append(names, genStage{id: e.ID, name: e.Name, requiredMilli: e.RequiredPoints * PointMilli})
		}
		tierStages[t.Tier] = names
		cumBefore[t.Tier] = t.EntryPoints * PointMilli

		// Major rule (10, 13, 14).
		var rule BreakthroughRule
		tribTier := t.Tier >= trib.FromTier && !last
		if m := t.MajorBreakthrough; m != nil {
			if m.SuccessBP != nil {
				rule.SuccessBP = *m.SuccessBP
			}
			rule.PityStepBP, rule.CooldownSeconds, rule.Requirements = m.PityStepBP, m.CooldownSeconds, m.Requirements
			if !bpOK(rule.SuccessBP) || !bpOK(rule.PityStepBP) || rule.CooldownSeconds < 0 {
				fail("tier %d: major_breakthrough values out of range", t.Tier)
			}
			if (tribTier || last) && (rule.SuccessBP != 0 || rule.PityStepBP != 0 || rule.CooldownSeconds != 0) {
				fail("tier %d: tribulation/last tiers must not set success_bp, pity_step_bp or cooldown_seconds", t.Tier)
			}
			if last && (m.SuccessBP == nil || len(rule.Requirements) > 0) {
				fail("tier %d (last playable): major_breakthrough must be { success_bp: 0 }", t.Tier)
			}
			seenItem := map[string]bool{}
			for _, req := range rule.Requirements {
				if !slugRe.MatchString(req.ItemID) || req.Quantity <= 0 {
					fail("tier %d: invalid requirement %+v", t.Tier, req)
				}
				if seenItem[req.ItemID] {
					fail("tier %d: duplicate requirement item %q (merge quantities)", t.Tier, req.ItemID)
				}
				seenItem[req.ItemID] = true
			}
		} else if last {
			fail("tier %d (last playable): major_breakthrough { success_bp: 0 } required", t.Tier)
		}
		if !tribTier && !last && rule.SuccessBP <= 0 {
			fail("tier %d: pre-tribulation major breakthrough needs success_bp > 0", t.Tier)
		}
		tierRule[t.Tier] = rule
	}

	// 12: tribulation.
	switch {
	case trib.FromTier < 1 || trib.FromTier >= lastPlayable:
		fail("tribulation.from_tier must be in 1..%d", lastPlayable-1)
	case !(trib.FloorBP <= trib.BaseBP && trib.BaseBP <= fixedpoint.BPScale && trib.FloorBP >= 0):
		fail("tribulation: need 0 ≤ floor_bp ≤ base_bp ≤ 10000")
	}
	if trib.StepBP < 0 || trib.LossBP != DecisionTribulationLossBP {
		fail("tribulation: step_bp ≥ 0 and loss_bp == %d (decision Phase 0 #2) required", DecisionTribulationLossBP)
	}
	if trib.QuoteWindowSeconds < 0 || trib.QuoteWindowSeconds > MaxQuoteWindowSeconds {
		fail("tribulation.quote_window_seconds must be within 0..%d", MaxQuoteWindowSeconds)
	}
	if trib.CooldownSeconds <= 0 || trib.GuaranteeAfter < 1 {
		fail("tribulation: cooldown_seconds > 0 and guarantee_after ≥ 1 required")
	}
	if trib.SurplusBonus.BPPerSurplusBP < 0 || trib.SurplusBonus.MaxBP < 0 || trib.ItemBonusMaxBP < 0 || trib.ProtectorPerBP < 0 {
		fail("tribulation: bonus values must be ≥ 0")
	}
	if trib.SurplusBonus.MaxBP > trib.MaxTotalBonusBP || trib.ItemBonusMaxBP > trib.MaxTotalBonusBP ||
		trib.ProtectorPerBP > trib.ProtectorMaxBP || trib.ProtectorMaxBP > trib.MaxTotalBonusBP {
		fail("tribulation: each bonus cap must be ≤ max_total_bonus_bp (and per-member ≤ protector max)")
	}
	if trib.MaxSuccessBP <= 0 || trib.MaxSuccessBP > DecisionTribulationMaxSuccessBP {
		fail("tribulation: 0 < max_success_bp ≤ %d (decision bổ sung F) required", DecisionTribulationMaxSuccessBP)
	}
	if trib.BaseBP+trib.MaxTotalBonusBP < trib.MaxSuccessBP {
		fail("tribulation: base_bp + max_total_bonus_bp must reach max_success_bp")
	}
	for _, p := range raw.Paths {
		if trib.Names[p.ID] == "" {
			fail("tribulation.names: missing path %q", p.ID)
		}
	}
	for id := range trib.Names {
		if _, ok := c.ladder[id]; !ok {
			fail("tribulation.names: unknown path %q", id)
		}
	}

	// 2, 7: realms → stages.
	seenRealm := map[string]bool{}
	for ri, fr := range raw.Realms {
		if !slugRe.MatchString(fr.ID) || seenRealm[fr.ID] {
			fail("realm[%d]: invalid or duplicate id %q", ri, fr.ID)
			continue
		}
		seenRealm[fr.ID] = true
		if _, ok := c.ladder[fr.Path]; !ok {
			fail("realm %q: unknown path %q", fr.ID, fr.Path)
			continue
		}
		if fr.Tier < 1 || fr.Tier > len(raw.Tiers) {
			fail("realm %q: unknown tier %d", fr.ID, fr.Tier)
			continue
		}
		if fr.Name == "" {
			fail("realm %q: name required", fr.ID)
		}
		if _, dup := c.byPathTier[fr.Path][fr.Tier]; dup {
			fail("realm %q: path %q already has a realm at tier %d", fr.ID, fr.Path, fr.Tier)
			continue
		}
		ft := raw.Tiers[fr.Tier-1]
		vis := fr.Visual
		vis.FxTier = ft.Visual.FxTier
		rl := &Realm{ID: fr.ID, Path: fr.Path, Tier: fr.Tier, Name: fr.Name, NamePlaceholder: fr.NamePlaceholder,
			Title: fr.Title, Visual: vis, Reserved: ft.Reserved}
		c.byPathTier[fr.Path][fr.Tier] = rl
		c.realms = append(c.realms, rl)
		gens := tierStages[fr.Tier]
		for k, g := range gens {
			name := g.name
			if g.prefixRealm {
				name = fr.Name + " " + g.name
			}
			s := &Stage{ID: fr.ID + "." + g.id, RealmID: fr.ID, Path: fr.Path, Tier: fr.Tier, StageOrder: k + 1,
				StageCount: len(gens), Index: -1, Name: name, Reserved: ft.Reserved}
			rl.Stages = append(rl.Stages, s)
			if ft.Reserved {
				continue
			}
			s.RequiredMilli = g.requiredMilli
			if s.IsLastOfRealm() {
				s.CapMilli = fixedpoint.MulBP(s.RequiredMilli, raw.Settings.OverflowCapBP)
			}
		}
	}

	// 7: every path has exactly one realm per playable tier; reserved tiers are optional per path.
	for _, p := range c.paths {
		for tier := 1; tier <= lastPlayable; tier++ {
			if _, ok := c.byPathTier[p.ID][tier]; !ok {
				fail("path %q: no realm at tier %d", p.ID, tier)
			}
		}
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	// Ladders: cumulative positions, next kinds, rules.
	for _, p := range c.paths {
		for tier := 1; tier <= lastPlayable; tier++ {
			rl := c.byPathTier[p.ID][tier]
			cum := cumBefore[tier]
			for _, s := range rl.Stages {
				s.Index = len(c.ladder[p.ID])
				s.CumulativeBeforeMilli = cum
				cum += s.RequiredMilli
				switch {
				case !s.IsLastOfRealm():
					s.NextKind = Auto
				case tier < lastPlayable:
					s.NextKind = Major
					s.Breakthrough = tierRule[tier]
					s.Tribulation = tier >= trib.FromTier
				}
				if _, dup := c.byID[s.ID]; dup {
					fail("stage %q: duplicate id", s.ID)
				}
				c.byID[s.ID] = s
				c.ladder[p.ID] = append(c.ladder[p.ID], s)
				c.all = append(c.all, s)
			}
		}
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return c, nil
}

// stageWeights returns the per-stage weights of a tier: all 1 (even split) when none are given, otherwise exactly
// one weight ≥ 1 per stage, non-decreasing so a later stage never costs less than an earlier one.
func stageWeights(w []int64, n int) ([]int64, bool) {
	if len(w) == 0 {
		w = make([]int64, n)
		for i := range w {
			w[i] = 1
		}
		return w, true
	}
	if len(w) != n {
		return nil, false
	}
	for i, v := range w {
		if v < 1 || v > 1_000 || (i > 0 && v < w[i-1]) {
			return nil, false
		}
	}
	return w, true
}

// splitSpan divides span whole points across stages proportionally to weights (floored), the remainder one point
// each on the LAST stages. With equal weights this is the even split of balance §11.1.
func splitSpan(span int64, weights []int64) []int64 {
	var total int64
	for _, w := range weights {
		total += w
	}
	out := make([]int64, len(weights))
	var used int64
	for k, w := range weights {
		out[k] = span * w / total // span ≤ ~1e5 points, w ≤ 1000: no overflow
		used += out[k]
	}
	for k := len(out) - 1; used < span; k-- {
		out[k]++
		used++
	}
	return out
}
