// Package hoapham is the pure, deterministic simulator of "Hóa Phàm", the slicing trial that guards the breakthrough
// to Hóa Thần (docs/design/hoa-pham-luat-choi.md). Ink "trần duyên" knots float up from the bottom of the field; the
// player severs them with brush strokes while sparing the "chân tâm" lanterns.
//
// The server never trusts a client score: the client submits its strokes — the pointer position sampled once per
// tick while pressed, quantized to the integer grid and delta-encoded — plus the claimed end state; Replay re-runs
// the whole game from the run's seed and params and the verdict comes from that replay.
//
// The TypeScript twin lives in apps/web/src/lib/minigames/hoa-pham/sim.ts and MUST stay bit-identical: integer math
// only, xorshift32, a fixed step order and the FNV-1a state hash below. The shared golden vector
// packages/contracts/golden/hoa-pham-v1.json is checked by both test suites.
package hoapham

import (
	"errors"
	"fmt"
)

// Sub is the number of physics sub-units per grid unit. Pointer points are whole grid units; object positions and
// velocities are sub-units so slow floating motion stays integer. Part of the rules — never change within a version.
const Sub = 32

// Object roles.
const (
	RoleKnot    = "knot"    // trần duyên: cut it (points), lets it fall away = escaped
	RoleLantern = "lantern" // chân tâm: never cut it (costs a tâm đăng)
)

// End reasons. Part of the API (verdict reason) — never rename.
const (
	ReasonCleared     = "cleared"      // pass_score reached and held until min_pass_ticks
	ReasonLanternsOut = "lanterns_out" // every tâm đăng spent: too many chân tâm lanterns cut
	ReasonTimeUp      = "time_up"      // time_limit_ticks elapsed
	ReasonGaveUp      = "gave_up"      // the client stopped before the game ended
	ReasonEndedEarly  = "ended_early"  // the replay ended before the claimed end tick (tampered log)
)

// Range is an inclusive integer range.
type Range struct {
	Min int64 `yaml:"min" json:"min"`
	Max int64 `yaml:"max" json:"max"`
}

// Motion of a spawned object, sub-units per tick. Knots are thrown up and fall under gravity_sub; lanterns rise at a
// constant speed and leave through the top.
type Motion struct {
	VY    Range `yaml:"vy_sub" json:"vySub"`        // upward speed (magnitude)
	VXMax int64 `yaml:"vx_max_sub" json:"vxMaxSub"` // sideways drift magnitude 0..max
}

// Spawn: a wave every max(min_ticks, start_ticks − tick / step_every_ticks) ticks, of 1 + rand(min(wave_max,
// 1 + tick / wave_grow_every_ticks)) objects.
type Spawn struct {
	StartTicks         int64 `yaml:"start_ticks" json:"startTicks"`
	MinTicks           int64 `yaml:"min_ticks" json:"minTicks"`
	StepEveryTicks     int64 `yaml:"step_every_ticks" json:"stepEveryTicks"`
	WaveMax            int64 `yaml:"wave_max" json:"waveMax"`
	WaveGrowEveryTicks int64 `yaml:"wave_grow_every_ticks" json:"waveGrowEveryTicks"`
	MarginUnits        int64 `yaml:"margin" json:"margin"` // spawn x within [margin, width − margin]
}

// Combo: the k-th knot severed by one stroke scores points + min(max_bonus, (k − 1) × step_bonus).
type Combo struct {
	StepBonus int64 `yaml:"step_bonus" json:"stepBonus"`
	MaxBonus  int64 `yaml:"max_bonus" json:"maxBonus"`
}

// Calm ("tâm tĩnh"): each severed knot charges charge_per_cut; a full charge starts duration_ticks of stillness in
// which the world moves only on every second tick. An escaped knot or a cut lantern empties the charge.
type Calm struct {
	ChargePerCut  int64 `yaml:"charge_per_cut" json:"chargePerCut"`
	ChargeMax     int64 `yaml:"charge_max" json:"chargeMax"`
	DurationTicks int64 `yaml:"duration_ticks" json:"durationTicks"`
}

// Object is one spawnable kind.
type Object struct {
	ID     string `yaml:"id" json:"id"`
	Name   string `yaml:"name" json:"name"`
	Role   string `yaml:"role" json:"role"`
	Weight int64  `yaml:"weight" json:"weight"`
	Radius int64  `yaml:"radius" json:"radius"` // grid units
	Points int64  `yaml:"points" json:"points"` // knots only
	HP     int64  `yaml:"hp" json:"hp"`         // knots: strokes needed (each stroke hits an object at most once)
}

// Params is everything the simulation depends on besides the seed and the strokes. A run stores a snapshot, so a
// catalogue change never alters a run in flight.
type Params struct {
	RulesVersion   int64    `yaml:"rules_version" json:"rulesVersion"`
	TickMS         int64    `yaml:"tick_ms" json:"tickMs"`
	Width          int64    `yaml:"width" json:"width"`   // grid units
	Height         int64    `yaml:"height" json:"height"` // grid units
	TimeLimitTicks int64    `yaml:"time_limit_ticks" json:"timeLimitTicks"`
	PassScore      int64    `yaml:"pass_score" json:"passScore"`
	MinPassTicks   int64    `yaml:"min_pass_ticks" json:"minPassTicks"`
	Lanterns       int64    `yaml:"lanterns" json:"lanterns"` // tâm đăng: cutting this many chân tâm lanterns fails
	GravitySub     int64    `yaml:"gravity_sub" json:"gravitySub"`
	Knot           Motion   `yaml:"knot_motion" json:"knotMotion"`
	Lantern        Motion   `yaml:"lantern_motion" json:"lanternMotion"`
	Spawn          Spawn    `yaml:"spawn" json:"spawn"`
	MinCutStep     int64    `yaml:"min_cut_step" json:"minCutStep"` // a segment shorter than this (grid units) cuts nothing
	Combo          Combo    `yaml:"combo" json:"combo"`
	Calm           Calm     `yaml:"calm" json:"calm"`
	MaxStrokes     int      `yaml:"max_strokes" json:"maxStrokes"`
	Objects        []Object `yaml:"objects" json:"objects"`

	// Human-plausibility rules. They bound tool-assisted play; they do not make the gate bot-proof. Each is also a
	// game rule the honest client applies while recording, so an honest log never violates them.
	// MaxStep: the brush follows the pointer by at most this many grid units (octile length) per tick; a longer step
	// in a log is a teleport (implausible_input).
	MaxStep int64 `yaml:"max_step" json:"maxStep"`
	// Within any window of InputWindowTicks ticks: at most MaxWindowTravel units of brush travel and at most
	// MaxStrokesPerWindow stroke starts.
	InputWindowTicks    int64 `yaml:"input_window_ticks" json:"inputWindowTicks"`
	MaxWindowTravel     int64 `yaml:"max_window_travel" json:"maxWindowTravel"`
	MaxStrokesPerWindow int   `yaml:"max_strokes_per_window" json:"maxStrokesPerWindow"`
	// MinLiftTicks: a new stroke starts at least this many unpressed ticks after the previous one ended.
	MinLiftTicks int64 `yaml:"min_lift_ticks" json:"minLiftTicks"`
}

// RulesVersion is the only rules version this simulator implements.
const RulesVersion = 1

// Validate checks ranges. The bounds also keep every product in the collision test below 2^53 (exact in TS).
func (p *Params) Validate() error {
	var errs []error
	fail := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	if p.RulesVersion != RulesVersion {
		fail("rules_version must be %d", RulesVersion)
	}
	if p.TickMS < 16 || p.TickMS > 100 {
		fail("tick_ms must be within 16..100")
	}
	if p.Width < 40 || p.Width > 200 || p.Height < 40 || p.Height > 200 {
		fail("field must be 40..200 × 40..200 grid units")
	}
	if p.TimeLimitTicks <= 0 || p.TimeLimitTicks > 10*60*1000/max(p.TickMS, 1) {
		fail("time_limit_ticks must be within 1..10 minutes")
	}
	if p.PassScore < 1 || p.PassScore > 10_000 {
		fail("pass_score must be within 1..10000")
	}
	if p.MinPassTicks < 0 || p.MinPassTicks >= p.TimeLimitTicks {
		fail("min_pass_ticks must be within 0..time_limit_ticks-1")
	}
	if p.Lanterns < 1 || p.Lanterns > 9 {
		fail("lanterns must be within 1..9")
	}
	if p.GravitySub < 1 || p.GravitySub > 16 {
		fail("gravity_sub must be within 1..16")
	}
	for name, m := range map[string]Motion{"knot_motion": p.Knot, "lantern_motion": p.Lantern} {
		if m.VY.Min < 1 || m.VY.Max < m.VY.Min || m.VY.Max > 400 || m.VXMax < 0 || m.VXMax > 64 {
			fail("%s: need 1 ≤ vy_sub.min ≤ vy_sub.max ≤ 400 and 0 ≤ vx_max_sub ≤ 64", name)
		}
	}
	// A knot thrown at the top speed must peak inside the field (rise = v² / 2g sub-units from below the bottom).
	if p.GravitySub > 0 && p.Knot.VY.Max*p.Knot.VY.Max/(2*p.GravitySub) > p.Height*Sub {
		fail("knot_motion.vy_sub.max throws knots out of the top of the field")
	}
	s := p.Spawn
	if s.MinTicks < 1 || s.StartTicks < s.MinTicks || s.StepEveryTicks < 1 || s.WaveMax < 1 || s.WaveMax > 6 ||
		s.WaveGrowEveryTicks < 1 || s.MarginUnits < 0 || 2*s.MarginUnits >= p.Width {
		fail("spawn: need 1 ≤ min_ticks ≤ start_ticks, step_every_ticks ≥ 1, wave_max 1..6, wave_grow_every_ticks ≥ 1, 0 ≤ 2·margin < width")
	}
	if p.MinCutStep < 1 || p.MinCutStep > 16 {
		fail("min_cut_step must be within 1..16")
	}
	if p.Combo.StepBonus < 0 || p.Combo.MaxBonus < 0 || p.Combo.MaxBonus > 100 {
		fail("combo: need step_bonus ≥ 0 and 0 ≤ max_bonus ≤ 100")
	}
	c := p.Calm
	if c.ChargePerCut < 0 || c.ChargeMax < 1 || c.DurationTicks < 2 || c.DurationTicks > 1000 {
		fail("calm: need charge_per_cut ≥ 0, charge_max ≥ 1, duration_ticks 2..1000")
	}
	if p.MaxStrokes < 1 || p.MaxStrokes > 5000 {
		fail("max_strokes must be within 1..5000")
	}
	if p.MaxStep < p.MinCutStep || p.MaxStep > 64 {
		fail("max_step must be within min_cut_step..64")
	}
	// Window: 0.5..5 s; travel ≥ one max step and ≤ 2000 units/s; ≤ 12 stroke starts per second.
	win := p.InputWindowTicks * p.TickMS
	if win < 500 || win > 5000 || p.MaxWindowTravel < p.MaxStep || p.MaxWindowTravel*1000 > 2000*win ||
		p.MaxStrokesPerWindow < 1 || int64(p.MaxStrokesPerWindow)*1000 > 12*win {
		fail("input window: input_window_ticks must span 0.5..5 s, max_window_travel within max_step..2000 units/s, max_strokes_per_window ≤ 12/s")
	}
	if p.MinLiftTicks < 1 || p.MinLiftTicks*p.TickMS > 500 {
		fail("min_lift_ticks must be ≥ 1 and span ≤ 500 ms")
	}
	if len(p.Objects) < 2 || len(p.Objects) > 8 {
		fail("need 2..8 objects")
	}
	ids := map[string]bool{}
	knots, lanterns := 0, 0
	for i, o := range p.Objects {
		if o.ID == "" || ids[o.ID] {
			fail("object[%d]: empty or duplicate id %q", i, o.ID)
		}
		ids[o.ID] = true
		if o.Name == "" {
			fail("object %q: empty name", o.ID)
		}
		if o.Weight < 1 || o.Weight > 20 {
			fail("object %q: weight must be within 1..20", o.ID)
		}
		if o.Radius < 3 || o.Radius > 20 {
			fail("object %q: radius must be within 3..20", o.ID)
		}
		switch o.Role {
		case RoleKnot:
			knots++
			if o.Points < 1 || o.Points > 10 || o.HP < 1 || o.HP > 3 {
				fail("object %q: knots need points 1..10 and hp 1..3", o.ID)
			}
		case RoleLantern:
			lanterns++
			if o.Points != 0 || o.HP != 1 {
				fail("object %q: lanterns have points 0 and hp 1", o.ID)
			}
		default:
			fail("object %q: role must be %s or %s", o.ID, RoleKnot, RoleLantern)
		}
	}
	if knots < 1 || lanterns < 1 {
		fail("need at least one knot and one lantern")
	}
	return errors.Join(errs...)
}

// MaxScore is an upper bound of the score for a number of severed knots (cheap plausibility check before replay).
func (p *Params) MaxScore(cuts int64) int64 {
	best := int64(0)
	for _, o := range p.Objects {
		best = max(best, o.Points)
	}
	return (best + p.Combo.MaxBonus) * max(cuts, 0)
}

// ───── geometry (shared with the client recorder) ─────

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// StepLen is the octile length of a step: max(|dx|, |dy|) + min(|dx|, |dy|) / 2 (integer). Within 6% of the
// Euclidean length and integer on both sides.
func StepLen(dx, dy int64) int64 {
	a, b := abs64(dx), abs64(dy)
	if a < b {
		a, b = b, a
	}
	return a + b/2
}

// Toward returns the point reached when moving from (px, py) toward (tx, ty) by at most budget (octile length).
// Truncates toward zero, so StepLen(result − p) ≤ budget always. The honest client uses it to apply max_step and the
// window travel cap; the reference player too.
func Toward(px, py, tx, ty, budget int64) (int64, int64) {
	dx, dy := tx-px, ty-py
	l := StepLen(dx, dy)
	if l <= budget {
		return tx, ty
	}
	if budget <= 0 {
		return px, py
	}
	return px + dx*budget/l, py + dy*budget/l // Go truncates toward zero; TS uses quo — identical
}

// segHits reports whether the segment A→B (sub-units) passes within r (sub-units) of C. Integer only.
func segHits(ax, ay, bx, by, cx, cy, r int64) bool {
	dx, dy := bx-ax, by-ay
	wx, wy := cx-ax, cy-ay
	t := wx*dx + wy*dy
	r2 := r * r
	if t <= 0 {
		return wx*wx+wy*wy <= r2
	}
	l2 := dx*dx + dy*dy
	if t >= l2 {
		ex, ey := cx-bx, cy-by
		return ex*ex+ey*ey <= r2
	}
	cr := dx*wy - dy*wx
	return cr*cr <= r2*l2
}

// ───── PRNG ─────

// XorShift32 is the shared PRNG (Marsaglia xorshift32, shifts 13/17/5). State 0 is replaced by a constant.
type XorShift32 struct{ S uint32 }

// NewXorShift32 seeds the generator.
func NewXorShift32(seed uint32) XorShift32 {
	if seed == 0 {
		seed = 0x9E3779B9
	}
	return XorShift32{S: seed}
}

// Next returns the next 32-bit value.
func (x *XorShift32) Next() uint32 {
	s := x.S
	s ^= s << 13
	s ^= s >> 17
	s ^= s << 5
	x.S = s
	return s
}

// IntN returns Next() mod n (n ≥ 1).
func (x *XorShift32) IntN(n int64) int64 { return int64(x.Next() % uint32(n)) }

// ───── game ─────

// Obj is one live object.
type Obj struct {
	ID         int64
	Type       int64 // index into Params.Objects
	X, Y       int64 // sub-units, centre
	VX, VY     int64 // sub-units per tick
	HP         int64
	LastStroke int64 // stroke that last hit it (a stroke hits an object at most once); 0 = none
}

// Game is one deterministic session. Not safe for concurrent use.
type Game struct {
	p    *Params
	rng  XorShift32
	objs []Obj

	tick         int64
	spawnCounter int64
	nextID       int64
	spawned      int64
	weightSum    int64

	score, cuts, escaped, lanternsCut int64
	lanternsLeft                      int64
	charge, calmLeft                  int64

	pressed     bool // pointer down on the previous tick
	px, py      int64
	strokeID    int64
	strokeCuts  int64
	bestCombo   int64
	lastCutTick int64 // render hint/telemetry: tick of the last severed knot (−1 = none)

	over   bool
	won    bool
	reason string
}

// New starts a game. p must be valid (Validate).
func New(p *Params, seed uint32) *Game {
	g := &Game{p: p, rng: NewXorShift32(seed), lanternsLeft: p.Lanterns, nextID: 1, lastCutTick: -1}
	for _, o := range p.Objects {
		g.weightSum += o.Weight
	}
	// The first wave comes ~12 ticks in, not after a full interval.
	g.spawnCounter = max(0, p.Spawn.StartTicks-12)
	return g
}

func (g *Game) end(reason string, won bool) { g.over, g.won, g.reason = true, won, reason }

func (g *Game) spawnInterval() int64 {
	s := g.p.Spawn
	return max(s.MinTicks, s.StartTicks-g.tick/s.StepEveryTicks)
}

// spawnWave draws, per object, in this order: type, x, vy, vx, side. Fixed so Go and TS consume the stream equally.
func (g *Game) spawnWave() {
	s := g.p.Spawn
	n := 1 + g.rng.IntN(min(s.WaveMax, 1+g.tick/s.WaveGrowEveryTicks))
	for range n {
		pick := g.rng.IntN(g.weightSum)
		t := int64(0)
		for i, o := range g.p.Objects {
			if pick < o.Weight {
				t = int64(i)
				break
			}
			pick -= o.Weight
		}
		def := g.p.Objects[t]
		m := g.p.Knot
		if def.Role == RoleLantern {
			m = g.p.Lantern
		}
		xu := s.MarginUnits + g.rng.IntN(g.p.Width-2*s.MarginUnits+1)
		vy := m.VY.Min + g.rng.IntN(m.VY.Max-m.VY.Min+1)
		vx := g.rng.IntN(m.VXMax + 1)
		side := g.rng.IntN(2)
		// Drift toward the centre from the outer thirds; either way in the middle third.
		switch {
		case xu*3 < g.p.Width:
		case xu*3 > 2*g.p.Width:
			vx = -vx
		default:
			if side == 1 {
				vx = -vx
			}
		}
		g.objs = append(g.objs, Obj{ID: g.nextID, Type: t, X: xu * Sub, Y: (g.p.Height + def.Radius) * Sub, VX: vx, VY: -vy, HP: def.HP})
		g.nextID++
		g.spawned++
	}
}

// cut applies the stroke segment (px,py)→(x,y) (grid units) to every live object, in list order.
func (g *Game) cut(x, y int64) {
	if StepLen(x-g.px, y-g.py) < g.p.MinCutStep {
		return
	}
	ax, ay, bx, by := g.px*Sub, g.py*Sub, x*Sub, y*Sub
	keep := g.objs[:0]
	for i := range g.objs {
		o := g.objs[i]
		if g.over || o.LastStroke == g.strokeID {
			keep = append(keep, o)
			continue
		}
		def := g.p.Objects[o.Type]
		if !segHits(ax, ay, bx, by, o.X, o.Y, def.Radius*Sub) {
			keep = append(keep, o)
			continue
		}
		o.LastStroke = g.strokeID
		if def.Role == RoleLantern {
			g.lanternsCut++
			g.lanternsLeft--
			g.strokeCuts = 0
			if g.calmLeft == 0 {
				g.charge = 0
			}
			if g.lanternsLeft <= 0 {
				g.end(ReasonLanternsOut, false)
			}
			continue // the lantern goes out
		}
		o.HP--
		if o.HP > 0 {
			keep = append(keep, o)
			continue
		}
		g.strokeCuts++
		g.cuts++
		g.lastCutTick = g.tick
		g.bestCombo = max(g.bestCombo, g.strokeCuts)
		g.score += def.Points + min(g.p.Combo.MaxBonus, (g.strokeCuts-1)*g.p.Combo.StepBonus)
		if g.calmLeft == 0 {
			g.charge += g.p.Calm.ChargePerCut
			if g.charge >= g.p.Calm.ChargeMax {
				g.charge = 0
				g.calmLeft = g.p.Calm.DurationTicks
			}
		}
	}
	g.objs = keep
}

// world moves every object, removes those that left the field (an uncut knot falling out = escaped) and spawns.
func (g *Game) world() {
	w, h := g.p.Width*Sub, g.p.Height*Sub
	keep := g.objs[:0]
	for _, o := range g.objs {
		def := g.p.Objects[o.Type]
		r := def.Radius * Sub
		o.X += o.VX
		o.Y += o.VY
		if def.Role == RoleKnot {
			o.VY += g.p.GravitySub
		}
		gone := o.X+r < 0 || o.X-r > w || (o.VY > 0 && o.Y-r > h) || (def.Role == RoleLantern && o.Y+r < 0)
		if !gone {
			keep = append(keep, o)
			continue
		}
		if def.Role == RoleKnot {
			g.escaped++
			if g.calmLeft == 0 {
				g.charge = 0
			}
		}
	}
	g.objs = keep
	g.spawnCounter++
	if g.spawnCounter >= g.spawnInterval() {
		g.spawnCounter = 0
		g.spawnWave()
	}
}

// Step advances one tick. down = pointer pressed this tick at (x, y) (grid units, inside the field). Order: the
// stroke segment from the previous tick's point cuts objects at their current positions; then the world moves
// (during tâm tĩnh only on ticks where calmLeft is even), then calm counts down; then the end checks.
func (g *Game) Step(down bool, x, y int64) {
	if g.over {
		return
	}
	if down {
		if g.pressed {
			g.cut(x, y)
		} else {
			g.strokeID++
			g.strokeCuts = 0
		}
		g.px, g.py = x, y
	}
	g.pressed = down
	if !g.over {
		if g.calmLeft == 0 || g.calmLeft%2 == 0 {
			g.world()
		}
		if g.calmLeft > 0 {
			g.calmLeft--
		}
	}
	g.tick++
	if !g.over && g.score >= g.p.PassScore && g.tick >= g.p.MinPassTicks {
		g.end(ReasonCleared, true)
	}
	if !g.over && g.tick >= g.p.TimeLimitTicks {
		g.end(ReasonTimeUp, false)
	}
}

// Hash is FNV-1a (32-bit) over the canonical state, each value as 4 little-endian bytes (uint32 two's complement).
func (g *Game) Hash() uint32 {
	h := uint32(2166136261)
	put := func(v int64) {
		u := uint32(v)
		for i := 0; i < 4; i++ {
			h ^= u & 0xff
			h *= 16777619
			u >>= 8
		}
	}
	b := func(v bool) int64 {
		if v {
			return 1
		}
		return 0
	}
	for _, v := range []int64{g.tick, b(g.over), b(g.won), g.score, g.cuts, g.escaped, g.lanternsCut, g.lanternsLeft, g.charge,
		g.calmLeft, g.spawnCounter, g.nextID, g.spawned, b(g.pressed), g.px, g.py, g.strokeID, g.strokeCuts, g.bestCombo,
		int64(g.rng.S), int64(len(g.objs))} {
		put(v)
	}
	for _, o := range g.objs {
		for _, v := range []int64{o.ID, o.Type, o.X, o.Y, o.VX, o.VY, o.HP, o.LastStroke} {
			put(v)
		}
	}
	return h
}

// ───── input log ─────

// Stroke is one pressed period: the point at Start, then one (dx, dy) per following tick. JSON wire form:
// [start, x0, y0, dx1, dy1, dx2, dy2, …] (grid units). Idle ticks between strokes cost nothing in the log.
type Stroke []int64

// Points is the number of sampled ticks of a well-formed stroke.
func (s Stroke) Points() int64 { return 1 + int64(len(s)-3)/2 }

// End is the last pressed tick of a well-formed stroke.
func (s Stroke) End() int64 { return s[0] + s.Points() - 1 }

// Result is the replay's authoritative end state.
type Result struct {
	Over      bool   `json:"over"`
	Won       bool   `json:"won"`
	Reason    string `json:"reason"`
	EndTick   int64  `json:"endTick"`
	Cuts      int64  `json:"cuts"`
	Score     int64  `json:"score"`
	Spawned   int64  `json:"spawned"`
	StateHash uint32 `json:"stateHash"`
	// Telemetry (audit).
	LanternsCut      int64 `json:"lanternsCut"`
	Escaped          int64 `json:"escaped"`
	BestCombo        int64 `json:"bestCombo"`
	Strokes          int   `json:"strokes"`
	Points           int64 `json:"points"`
	MaxStep          int64 `json:"maxStep"`
	MaxWindowTravel  int64 `json:"maxWindowTravel"`
	MaxWindowStrokes int   `json:"maxWindowStrokes"`
}

// ErrInvalidLog: the stroke log is malformed (shape, order, overlap, range, out of the field, too many strokes).
var ErrInvalidLog = errors.New("hoapham: invalid stroke log")

// ErrImplausibleInput: the log is well-formed but not human: a teleporting step (> max_step), too much travel or too
// many stroke starts inside one input window, or a re-press faster than min_lift_ticks.
var ErrImplausibleInput = errors.New("hoapham: implausible pointer input")

// maxDelta bounds a single logged delta before any arithmetic (anything near it is a teleport anyway).
const maxDelta = 1 << 16

// Cadence is the log's cadence summary (what the plausibility rules look at).
type Cadence struct {
	Points           int64
	MaxStep          int64
	MaxWindowTravel  int64
	MaxWindowStrokes int
	MinLiftTicks     int64 // smallest gap between two strokes (0 = fewer than two strokes)
}

// event is one step (travel) or one stroke start at a tick.
type event struct{ tick, v int64 }

// maxWindowSum is the largest sum of v over events within any window of `window` consecutive ticks (ticks
// non-decreasing). Same two-pointer algorithm as sim.ts maxWindowSum.
func maxWindowSum(ev []event, window int64) int64 {
	var best, sum int64
	lo := 0
	for hi := range ev {
		sum += ev[hi].v
		for ev[hi].tick-ev[lo].tick >= window {
			sum -= ev[lo].v
			lo++
		}
		best = max(best, sum)
	}
	return best
}

// Check validates the log shape (ErrInvalidLog) and measures its cadence. It does not apply the plausibility caps.
func Check(p *Params, strokes []Stroke, endTick int64) (Cadence, error) {
	var c Cadence
	if endTick <= 0 || endTick > p.TimeLimitTicks {
		return c, fmt.Errorf("%w: endTick %d outside 1..%d", ErrInvalidLog, endTick, p.TimeLimitTicks)
	}
	if len(strokes) > p.MaxStrokes {
		return c, fmt.Errorf("%w: %d strokes > %d", ErrInvalidLog, len(strokes), p.MaxStrokes)
	}
	var travel, starts []event
	prevEnd := int64(-1 << 40)
	for i, s := range strokes {
		if len(s) < 3 || len(s)%2 == 0 {
			return c, fmt.Errorf("%w: stroke %d has %d numbers", ErrInvalidLog, i, len(s))
		}
		// Range-check the start before End(): a huge start would overflow start + points (anti-cheat L1).
		if s[0] < 0 || s[0] >= endTick || s[0] <= prevEnd || s.End() >= endTick {
			return c, fmt.Errorf("%w: stroke %d at tick %d overlaps or is out of range", ErrInvalidLog, i, s[0])
		}
		if prevEnd >= 0 {
			gap := s[0] - prevEnd - 1
			if gap < 1 {
				return c, fmt.Errorf("%w: stroke %d touches the previous one", ErrInvalidLog, i)
			}
			if c.MinLiftTicks == 0 || gap < c.MinLiftTicks {
				c.MinLiftTicks = gap
			}
		}
		x, y := s[1], s[2]
		if x < 0 || x >= p.Width || y < 0 || y >= p.Height {
			return c, fmt.Errorf("%w: stroke %d starts outside the field", ErrInvalidLog, i)
		}
		starts = append(starts, event{s[0], 1})
		for j := 3; j < len(s); j += 2 {
			dx, dy := s[j], s[j+1]
			if abs64(dx) > maxDelta || abs64(dy) > maxDelta {
				return c, fmt.Errorf("%w: stroke %d delta out of range", ErrInvalidLog, i)
			}
			x, y = x+dx, y+dy
			if x < 0 || x >= p.Width || y < 0 || y >= p.Height {
				return c, fmt.Errorf("%w: stroke %d leaves the field", ErrInvalidLog, i)
			}
			l := StepLen(dx, dy)
			c.MaxStep = max(c.MaxStep, l)
			travel = append(travel, event{s[0] + int64(j-1)/2, l})
		}
		c.Points += s.Points()
		prevEnd = s.End()
	}
	c.MaxWindowTravel = maxWindowSum(travel, p.InputWindowTicks)
	c.MaxWindowStrokes = int(maxWindowSum(starts, p.InputWindowTicks))
	return c, nil
}

// Plausible applies the human-cadence caps to a checked log.
func (p *Params) Plausible(c Cadence) error {
	switch {
	case c.MaxStep > p.MaxStep:
		return fmt.Errorf("%w: step of %d units in one tick (max %d)", ErrImplausibleInput, c.MaxStep, p.MaxStep)
	case c.MaxWindowTravel > p.MaxWindowTravel:
		return fmt.Errorf("%w: %d units within %d ticks (max %d)", ErrImplausibleInput, c.MaxWindowTravel, p.InputWindowTicks, p.MaxWindowTravel)
	case c.MaxWindowStrokes > p.MaxStrokesPerWindow:
		return fmt.Errorf("%w: %d strokes within %d ticks (max %d)", ErrImplausibleInput, c.MaxWindowStrokes, p.InputWindowTicks, p.MaxStrokesPerWindow)
	case c.MinLiftTicks > 0 && c.MinLiftTicks < p.MinLiftTicks:
		return fmt.Errorf("%w: re-pressed after %d ticks (min %d)", ErrImplausibleInput, c.MinLiftTicks, p.MinLiftTicks)
	}
	return nil
}

// Replay simulates from tick 0 to endTick (or until the game ends earlier) feeding the strokes at their ticks.
// Malformed logs → ErrInvalidLog; non-human cadence → ErrImplausibleInput (Result carries the cadence telemetry).
// A game still running at endTick ends with reason gave_up; one that ends before endTick is reported with reason
// ended_early (the claim and the replay disagree).
func Replay(p *Params, seed uint32, strokes []Stroke, endTick int64) (Result, error) {
	c, err := Check(p, strokes, endTick)
	if err != nil {
		return Result{}, err
	}
	tele := Result{Strokes: len(strokes), Points: c.Points, MaxStep: c.MaxStep, MaxWindowTravel: c.MaxWindowTravel,
		MaxWindowStrokes: c.MaxWindowStrokes}
	if err := p.Plausible(c); err != nil {
		return tele, err
	}
	g := New(p, seed)
	si, j := 0, 0 // stroke index, position inside it
	var x, y int64
	for !g.over && g.tick < endTick {
		down := false
		if si < len(strokes) {
			s := strokes[si]
			switch {
			case g.tick == s[0]:
				x, y, j, down = s[1], s[2], 3, true
			case g.tick > s[0] && j < len(s):
				x, y = x+s[j], y+s[j+1]
				j += 2
				down = true
			}
			if down && j >= len(s) {
				si++ // last point of this stroke
				j = 0
			}
		}
		g.Step(down, x, y)
	}
	r := g.result()
	r.Strokes, r.Points, r.MaxStep, r.MaxWindowTravel, r.MaxWindowStrokes = tele.Strokes, tele.Points, tele.MaxStep, tele.MaxWindowTravel, tele.MaxWindowStrokes
	switch {
	case !g.over:
		r.Reason = ReasonGaveUp
	case g.tick < endTick:
		r.Reason = ReasonEndedEarly
	}
	return r, nil
}

func (g *Game) result() Result {
	r := Result{Over: g.over, Won: g.won, Reason: g.reason, EndTick: g.tick, Cuts: g.cuts, Score: g.score, Spawned: g.spawned,
		StateHash: g.Hash(), LanternsCut: g.lanternsCut, Escaped: g.escaped, BestCombo: g.bestCombo}
	if !g.over {
		r.Reason = ReasonGaveUp
	}
	return r
}

// ───── read-only accessors (the test-only reference player) ─────

// Clone deep-copies the game.
func (g *Game) Clone() *Game {
	c := *g
	c.objs = append([]Obj(nil), g.objs...)
	return &c
}

// Params returns the rules.
func (g *Game) Params() *Params { return g.p }

// Objects returns the live objects. Do not mutate.
func (g *Game) Objects() []Obj { return g.objs }

// Tick is the number of ticks simulated so far.
func (g *Game) Tick() int64 { return g.tick }

// Over reports whether the game ended; Won a pass.
func (g *Game) Over() bool { return g.over }

// Won reports a pass.
func (g *Game) Won() bool { return g.won }

// Score returns score, cuts, lantern lives left.
func (g *Game) Score() (score, cuts, lanternsLeft int64) { return g.score, g.cuts, g.lanternsLeft }

// Result returns the end state as Replay would report it (reason gave_up while running; no cadence telemetry).
func (g *Game) Result() Result { return g.result() }
