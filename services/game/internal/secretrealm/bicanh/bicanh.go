// Package bicanh is the pure, deterministic simulator of the Bí Cảnh trial (docs/design/bi-canh-luat-choi.md): an
// original top-down survival run inside a sealed secret realm. The cultivator walks the realm in 8 directions; a
// flying sword strikes the nearest tà khí on its own, a kiếm trận of orbiting blades joins as kiếm ý grows, and the
// linh khí motes the fallen spirits leave behind feed that growth. Survive the time limit, or fall when hộ thể runs
// out.
//
// The server never trusts a client score: the client submits its movement log — one 8-way direction per tick,
// run-length encoded — plus the claimed end state; Replay re-runs the whole game from the run's seed and params.
//
// The TypeScript twin lives in apps/web/src/lib/minigames/bi-canh/sim.ts and MUST stay bit-identical: integer math
// only (truncating division), xorshift32, a fixed step order and the FNV-1a state hash below. The shared golden
// vector packages/contracts/golden/bi-canh-v1.json is checked by both test suites.
package bicanh

import (
	"errors"
	"fmt"
)

// Sub is the number of position sub-units per arena unit (rules constant — never change within a version).
const Sub = 16

// RulesVersion is the only rules version this simulator implements.
const RulesVersion = 1

// End reasons. Part of the API (verdict reason) — never rename.
const (
	ReasonSurvived   = "survived"    // the time limit was reached with hộ thể left
	ReasonFallen     = "fallen"      // hộ thể ran out
	ReasonGaveUp     = "gave_up"     // the client stopped before the game ended
	ReasonEndedEarly = "ended_early" // the replay ended before the claimed end tick (tampered log)
)

// Directions: 0 = stand still, 1..8 = N, NE, E, SE, S, SW, W, NW (screen y grows downward).
const MaxDir = 8

// cos64 is round(1024·cos(2πi/64)); sin(i) = cos64[(i+48) % 64]. Shared verbatim with sim.ts.
var cos64 = [64]int64{1024, 1019, 1004, 980, 946, 903, 851, 792, 724, 650, 569, 483, 392, 297, 200, 100, 0, -100, -200,
	-297, -392, -483, -569, -650, -724, -792, -851, -903, -946, -980, -1004, -1019, -1024, -1019, -1004, -980, -946, -903,
	-851, -792, -724, -650, -569, -483, -392, -297, -200, -100, 0, 100, 200, 297, 392, 483, 569, 650, 724, 792, 851, 903,
	946, 980, 1004, 1019}

// Arena is the sealed field in units.
type Arena struct {
	Width  int64 `yaml:"width" json:"width"`
	Height int64 `yaml:"height" json:"height"`
}

// Player is the cultivator. Speed is sub-units per tick on a straight move (diagonal: speed·181/256 per axis).
type Player struct {
	Radius      int64 `yaml:"radius" json:"radius"`
	Speed       int64 `yaml:"speed_sub" json:"speedSub"`
	HP          int64 `yaml:"hp" json:"hp"` // hộ thể: hits the cultivator can take
	InvulnTicks int64 `yaml:"invuln_ticks" json:"invulnTicks"`
}

// Sword is the auto-firing phi kiếm: every level.fire_every ticks it flies at the nearest tà khí within range.
type Sword struct {
	Range     int64 `yaml:"range" json:"range"` // units
	Speed     int64 `yaml:"speed_sub" json:"speedSub"`
	Radius    int64 `yaml:"radius" json:"radius"`
	LifeTicks int64 `yaml:"life_ticks" json:"lifeTicks"`
	Damage    int64 `yaml:"damage" json:"damage"`
}

// Orbit is the kiếm trận: level.orbit blades circling the cultivator.
type Orbit struct {
	Radius           int64 `yaml:"radius" json:"radius"`                       // units from the centre
	BladeRadius      int64 `yaml:"blade_radius" json:"bladeRadius"`            // units
	Step             int64 `yaml:"step" json:"step"`                           // 1/64 turns per tick
	Damage           int64 `yaml:"damage" json:"damage"`                       //
	HitCooldownTicks int64 `yaml:"hit_cooldown_ticks" json:"hitCooldownTicks"` // per spirit
}

// Level is one kiếm ý level, reached at Qi linh khí collected.
type Level struct {
	Qi        int64 `yaml:"qi" json:"qi"`
	FireEvery int64 `yaml:"fire_every" json:"fireEvery"`
	Orbit     int64 `yaml:"orbit" json:"orbit"`
}

// Spawn: a wave every max(min_ticks, start_ticks − tick / step_every_ticks) ticks of 1 + rand(min(wave_max,
// 1 + tick / wave_grow_every_ticks)) spirits at the realm's edge; never more than max_enemies alive.
type Spawn struct {
	StartTicks         int64 `yaml:"start_ticks" json:"startTicks"`
	MinTicks           int64 `yaml:"min_ticks" json:"minTicks"`
	StepEveryTicks     int64 `yaml:"step_every_ticks" json:"stepEveryTicks"`
	WaveMax            int64 `yaml:"wave_max" json:"waveMax"`
	WaveGrowEveryTicks int64 `yaml:"wave_grow_every_ticks" json:"waveGrowEveryTicks"`
	MaxEnemies         int   `yaml:"max_enemies" json:"maxEnemies"`
	SafeRadius         int64 `yaml:"safe_radius" json:"safeRadius"` // units: a spawn this close to the cultivator moves to the opposite edge
}

// Enemy is one tà khí kind. Weight 0 = spawned only at EliteAtTicks.
type Enemy struct {
	ID       string  `yaml:"id" json:"id"`
	Name     string  `yaml:"name" json:"name"`
	Weight   int64   `yaml:"weight" json:"weight"`
	FromTick int64   `yaml:"from_tick" json:"fromTick"`
	HP       int64   `yaml:"hp" json:"hp"`
	Speed    int64   `yaml:"speed_sub" json:"speedSub"`
	Radius   int64   `yaml:"radius" json:"radius"`
	Points   int64   `yaml:"points" json:"points"` // score on defeat; also the linh khí mote it leaves
	AtTicks  []int64 `yaml:"at_ticks,omitempty" json:"atTicks,omitempty"`
}

// Orbs are the linh khí motes.
type Orbs struct {
	PickupRadius int64 `yaml:"pickup_radius" json:"pickupRadius"`
	MagnetRadius int64 `yaml:"magnet_radius" json:"magnetRadius"`
	MagnetSpeed  int64 `yaml:"magnet_speed_sub" json:"magnetSpeedSub"`
	LifeTicks    int64 `yaml:"life_ticks" json:"lifeTicks"`
	Max          int   `yaml:"max" json:"max"`
}

// Params is everything the simulation depends on besides the seed and the moves. A run stores a snapshot.
type Params struct {
	RulesVersion   int64   `yaml:"rules_version" json:"rulesVersion"`
	TickMS         int64   `yaml:"tick_ms" json:"tickMs"`
	TimeLimitTicks int64   `yaml:"time_limit_ticks" json:"timeLimitTicks"`
	Arena          Arena   `yaml:"arena" json:"arena"`
	Player         Player  `yaml:"player" json:"player"`
	Sword          Sword   `yaml:"sword" json:"sword"`
	Orbit          Orbit   `yaml:"orbit" json:"orbit"`
	Levels         []Level `yaml:"levels" json:"levels"`
	Spawn          Spawn   `yaml:"spawn" json:"spawn"`
	Enemies        []Enemy `yaml:"enemies" json:"enemies"`
	Orbs           Orbs    `yaml:"orbs" json:"orbs"`
	MaxSwords      int     `yaml:"max_swords" json:"maxSwords"`

	// Human-plausibility rules (input log). Each is also a game rule the honest client recorder applies, so an
	// honest log never violates them: at most MaxChangesPerWindow direction changes within any InputWindowTicks
	// ticks, and at most MaxRuns runs in the whole log.
	InputWindowTicks    int64 `yaml:"input_window_ticks" json:"inputWindowTicks"`
	MaxChangesPerWindow int64 `yaml:"max_changes_per_window" json:"maxChangesPerWindow"`
	MaxRuns             int   `yaml:"max_runs" json:"maxRuns"`
}

// Validate checks ranges. The bounds keep every product below 2^53 (exact in TS) and the per-tick work bounded.
func (p *Params) Validate() error {
	var errs []error
	fail := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	if p.RulesVersion != RulesVersion {
		fail("rules_version must be %d", RulesVersion)
	}
	if p.TickMS < 16 || p.TickMS > 100 {
		fail("tick_ms must be within 16..100")
	}
	if p.TimeLimitTicks <= 0 || p.TimeLimitTicks*max(p.TickMS, 1) > 4*60*1000 {
		fail("time_limit_ticks must be within 1 tick..4 minutes")
	}
	a := p.Arena
	if a.Width < 200 || a.Width > 2000 || a.Height < 200 || a.Height > 2000 {
		fail("arena must be 200..2000 × 200..2000 units")
	}
	pl := p.Player
	if pl.Radius < 4 || pl.Radius > 40 || pl.Speed < 8 || pl.Speed > 400 || pl.HP < 1 || pl.HP > 20 || pl.InvulnTicks < 0 || pl.InvulnTicks > 200 {
		fail("player: need radius 4..40, speed_sub 8..400, hp 1..20, invuln_ticks 0..200")
	}
	s := p.Sword
	if s.Range < 20 || s.Range > 2000 || s.Speed < 8 || s.Speed > 800 || s.Radius < 1 || s.Radius > 40 || s.LifeTicks < 1 || s.LifeTicks > 200 ||
		s.Damage < 1 || s.Damage > 100 {
		fail("sword: need range 20..2000, speed_sub 8..800, radius 1..40, life_ticks 1..200, damage 1..100")
	}
	o := p.Orbit
	if o.Radius < 8 || o.Radius > 400 || o.BladeRadius < 1 || o.BladeRadius > 40 || o.Step < 1 || o.Step > 16 || o.Damage < 1 ||
		o.Damage > 100 || o.HitCooldownTicks < 1 || o.HitCooldownTicks > 200 {
		fail("orbit: need radius 8..400, blade_radius 1..40, step 1..16, damage 1..100, hit_cooldown_ticks 1..200")
	}
	if len(p.Levels) < 1 || len(p.Levels) > 12 {
		fail("need 1..12 levels")
	}
	for i, l := range p.Levels {
		if i == 0 && l.Qi != 0 {
			fail("level 0 must start at qi 0")
		}
		if i > 0 && l.Qi <= p.Levels[i-1].Qi {
			fail("level %d: qi must increase", i)
		}
		if l.FireEvery < 1 || l.FireEvery > 200 || l.Orbit < 0 || l.Orbit > 8 {
			fail("level %d: need fire_every 1..200 and orbit 0..8", i)
		}
	}
	sp := p.Spawn
	if sp.MinTicks < 1 || sp.StartTicks < sp.MinTicks || sp.StepEveryTicks < 1 || sp.WaveMax < 1 || sp.WaveMax > 8 || sp.WaveGrowEveryTicks < 1 ||
		sp.MaxEnemies < 1 || sp.MaxEnemies > 150 || sp.SafeRadius < 0 || 2*sp.SafeRadius >= min(a.Width, a.Height) {
		fail("spawn: need 1 ≤ min_ticks ≤ start_ticks, step_every_ticks ≥ 1, wave_max 1..8, wave_grow_every_ticks ≥ 1, max_enemies 1..150, 0 ≤ 2·safe_radius < arena")
	}
	if len(p.Enemies) < 1 || len(p.Enemies) > 8 {
		fail("need 1..8 enemies")
	}
	ids := map[string]bool{}
	spawnable := false
	for i, e := range p.Enemies {
		if e.ID == "" || ids[e.ID] || e.Name == "" {
			fail("enemy[%d]: empty or duplicate id/name %q", i, e.ID)
		}
		ids[e.ID] = true
		if e.Weight < 0 || e.Weight > 20 || e.FromTick < 0 || e.HP < 1 || e.HP > 500 || e.Speed < 1 || e.Speed > 400 || e.Radius < 3 ||
			e.Radius > 60 || e.Points < 0 || e.Points > 100 {
			fail("enemy %q: need weight 0..20, from_tick ≥ 0, hp 1..500, speed_sub 1..400, radius 3..60, points 0..100", e.ID)
		}
		if e.Weight > 0 && e.FromTick == 0 {
			spawnable = true
		}
		if e.Weight == 0 && len(e.AtTicks) == 0 {
			fail("enemy %q: weight 0 needs at_ticks", e.ID)
		}
		for j, t := range e.AtTicks {
			if t < 1 || t >= p.TimeLimitTicks || (j > 0 && t <= e.AtTicks[j-1]) {
				fail("enemy %q: at_ticks must increase within 1..time_limit_ticks-1", e.ID)
			}
		}
	}
	if !spawnable {
		fail("need at least one enemy with weight > 0 and from_tick 0")
	}
	ob := p.Orbs
	if ob.PickupRadius < 1 || ob.MagnetRadius < ob.PickupRadius || ob.MagnetRadius > 400 || ob.MagnetSpeed < 1 || ob.MagnetSpeed > 800 ||
		ob.LifeTicks < 1 || ob.LifeTicks > 4000 || ob.Max < 1 || ob.Max > 200 {
		fail("orbs: need 1 ≤ pickup_radius ≤ magnet_radius ≤ 400, magnet_speed_sub 1..800, life_ticks 1..4000, max 1..200")
	}
	if p.MaxSwords < 1 || p.MaxSwords > 64 {
		fail("max_swords must be within 1..64")
	}
	win := p.InputWindowTicks * p.TickMS
	if win < 500 || win > 5000 || p.MaxChangesPerWindow < 1 || p.MaxChangesPerWindow*1000 > 15*win {
		fail("input window: input_window_ticks must span 0.5..5 s and max_changes_per_window ≤ 15/s")
	}
	if p.MaxRuns < 1 || p.MaxRuns > 20000 {
		fail("max_runs must be within 1..20000")
	}
	return errors.Join(errs...)
}

// MaxScore bounds the score for a number of defeated spirits: each scores its points and leaves a mote of the same
// value (cheap plausibility check before replay).
func (p *Params) MaxScore(kills int64) int64 {
	best := int64(0)
	for _, e := range p.Enemies {
		best = max(best, e.Points)
	}
	return 2 * best * max(kills, 0)
}

// ───── geometry ─────

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// StepLen is the octile length max(|dx|,|dy|) + min(|dx|,|dy|)/2 (integer).
func StepLen(dx, dy int64) int64 {
	a, b := abs64(dx), abs64(dy)
	if a < b {
		a, b = b, a
	}
	return a + b/2
}

// toward returns the step (≤ budget octile) from (px,py) toward (tx,ty). Truncating division (TS: quo).
func toward(px, py, tx, ty, budget int64) (int64, int64) {
	dx, dy := tx-px, ty-py
	l := StepLen(dx, dy)
	if l <= budget {
		return dx, dy
	}
	return dx * budget / l, dy * budget / l
}

// DirVec is the per-tick move of direction d at straight speed s (sub-units).
func DirVec(d, s int64) (int64, int64) {
	g := s * 181 / 256
	switch d {
	case 1:
		return 0, -s
	case 2:
		return g, -g
	case 3:
		return s, 0
	case 4:
		return g, g
	case 5:
		return 0, s
	case 6:
		return -g, g
	case 7:
		return -s, 0
	case 8:
		return -g, -g
	}
	return 0, 0
}

func within(ax, ay, bx, by, r int64) bool {
	dx, dy := ax-bx, ay-by
	return dx*dx+dy*dy <= r*r
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

// Foe is one live tà khí.
type Foe struct {
	ID       int64
	Type     int64 // index into Params.Enemies
	X, Y     int64 // sub-units
	HP       int64
	OrbitHit int64 // tick of the last kiếm trận hit (−1 = never)
}

// Shot is one flying sword.
type Shot struct {
	X, Y, VX, VY int64 // sub-units
	Life         int64
}

// Mote is one linh khí orb.
type Mote struct {
	X, Y  int64 // sub-units
	Value int64
	Life  int64
}

// Game is one deterministic session. Not safe for concurrent use.
type Game struct {
	p   *Params
	rng XorShift32

	tick         int64
	px, py       int64 // sub-units
	hp           int64
	invuln       int64
	level        int64
	qi, score    int64
	kills        int64
	spawned      int64
	nextID       int64
	spawnCounter int64
	fireCounter  int64
	hits         int64 // hits taken

	foes  []Foe
	shots []Shot
	motes []Mote

	over   bool
	won    bool
	reason string
}

// New starts a game. p must be valid (Validate).
func New(p *Params, seed uint32) *Game {
	g := &Game{p: p, rng: NewXorShift32(seed), px: p.Arena.Width * Sub / 2, py: p.Arena.Height * Sub / 2, hp: p.Player.HP, nextID: 1}
	g.spawnCounter = max(0, p.Spawn.StartTicks-20) // the first wave comes ~1 s in
	return g
}

func (g *Game) end(reason string, won bool) { g.over, g.won, g.reason = true, won, reason }

func (g *Game) spawnInterval() int64 {
	s := g.p.Spawn
	return max(s.MinTicks, s.StartTicks-g.tick/s.StepEveryTicks)
}

// place puts a new spirit of type t at the edge. RNG draws: side, then position along the side.
func (g *Game) place(t int64) {
	def := g.p.Enemies[t]
	w, h := g.p.Arena.Width, g.p.Arena.Height
	side := g.rng.IntN(4)
	var xu, yu int64
	switch side {
	case 0: // top
		xu, yu = def.Radius+g.rng.IntN(w-2*def.Radius+1), def.Radius
	case 1: // right
		xu, yu = w-def.Radius, def.Radius+g.rng.IntN(h-2*def.Radius+1)
	case 2: // bottom
		xu, yu = def.Radius+g.rng.IntN(w-2*def.Radius+1), h-def.Radius
	default: // left
		xu, yu = def.Radius, def.Radius+g.rng.IntN(h-2*def.Radius+1)
	}
	x, y := xu*Sub, yu*Sub
	if within(x, y, g.px, g.py, g.p.Spawn.SafeRadius*Sub) { // never on top of the cultivator: mirror through the centre
		x, y = w*Sub-x, h*Sub-y
	}
	g.foes = append(g.foes, Foe{ID: g.nextID, Type: t, X: x, Y: y, HP: def.HP, OrbitHit: -1})
	g.nextID++
	g.spawned++
}

func (g *Game) spawnWave() {
	s := g.p.Spawn
	n := 1 + g.rng.IntN(min(s.WaveMax, 1+g.tick/s.WaveGrowEveryTicks))
	var sum int64
	for _, e := range g.p.Enemies {
		if e.FromTick <= g.tick {
			sum += e.Weight
		}
	}
	for range n {
		if len(g.foes) >= s.MaxEnemies {
			return
		}
		pick := g.rng.IntN(sum)
		t := int64(0)
		for i, e := range g.p.Enemies {
			if e.FromTick > g.tick || e.Weight == 0 {
				continue
			}
			if pick < e.Weight {
				t = int64(i)
				break
			}
			pick -= e.Weight
		}
		g.place(t)
	}
}

// defeat removes nothing itself: it scores the foe and drops its mote (the caller filters dead foes).
func (g *Game) defeat(f *Foe) {
	def := g.p.Enemies[f.Type]
	g.kills++
	g.score += def.Points
	if def.Points > 0 && len(g.motes) < g.p.Orbs.Max {
		g.motes = append(g.motes, Mote{X: f.X, Y: f.Y, Value: def.Points, Life: g.p.Orbs.LifeTicks})
	}
}

func (g *Game) damage(i int, d int64) {
	f := &g.foes[i]
	if f.HP <= 0 {
		return
	}
	f.HP -= d
	if f.HP <= 0 {
		g.defeat(f)
	}
}

func (g *Game) sweepDead() {
	keep := g.foes[:0]
	for _, f := range g.foes {
		if f.HP > 0 {
			keep = append(keep, f)
		}
	}
	g.foes = keep
}

// Step advances one tick with movement direction dir (0..8). Order: move → fire → swords → kiếm trận → spirits
// move and touch → motes → level → spawn → timers → end checks.
func (g *Game) Step(dir int64) {
	if g.over {
		return
	}
	p := g.p
	// 1. Move, clamped inside the realm.
	vx, vy := DirVec(dir, p.Player.Speed)
	r := p.Player.Radius * Sub
	g.px = min(max(g.px+vx, r), p.Arena.Width*Sub-r)
	g.py = min(max(g.py+vy, r), p.Arena.Height*Sub-r)

	// 2. Phi kiếm: when charged, fly at the nearest spirit within range (ties: lowest id = list order).
	lv := p.Levels[g.level]
	if g.fireCounter < lv.FireEvery {
		g.fireCounter++
	}
	if g.fireCounter >= lv.FireEvery && len(g.shots) < p.MaxSwords {
		best, bestD := -1, (p.Sword.Range*Sub)*(p.Sword.Range*Sub)
		for i, f := range g.foes {
			dx, dy := f.X-g.px, f.Y-g.py
			if d := dx*dx + dy*dy; d <= bestD && (best < 0 || d < bestD) {
				best, bestD = i, d
			}
		}
		if best >= 0 {
			f := g.foes[best]
			sx, sy := toward(g.px, g.py, f.X, f.Y, p.Sword.Speed)
			if sx == 0 && sy == 0 {
				sy = -p.Sword.Speed
			}
			// Normalize to full speed so a near target does not get a slow sword.
			if l := StepLen(sx, sy); l < p.Sword.Speed {
				sx, sy = sx*p.Sword.Speed/l, sy*p.Sword.Speed/l
			}
			g.shots = append(g.shots, Shot{X: g.px, Y: g.py, VX: sx, VY: sy, Life: p.Sword.LifeTicks})
			g.fireCounter = 0
		}
	}

	// 3. Swords fly and strike the first spirit they touch (list order); one strike each.
	keepS := g.shots[:0]
	for _, s := range g.shots {
		s.X += s.VX
		s.Y += s.VY
		s.Life--
		hit := false
		for i := range g.foes {
			f := &g.foes[i]
			if f.HP > 0 && within(s.X, s.Y, f.X, f.Y, (p.Enemies[f.Type].Radius+p.Sword.Radius)*Sub) {
				g.damage(i, p.Sword.Damage)
				hit = true
				break
			}
		}
		out := s.X < 0 || s.Y < 0 || s.X > p.Arena.Width*Sub || s.Y > p.Arena.Height*Sub
		if !hit && !out && s.Life > 0 {
			keepS = append(keepS, s)
		}
	}
	g.shots = keepS

	// 4. Kiếm trận: blades at angle (tick·step + b·64/n) mod 64 around the cultivator.
	if n := lv.Orbit; n > 0 {
		base := g.tick * p.Orbit.Step % 64
		for b := int64(0); b < n; b++ {
			idx := (base + b*64/n) % 64
			bx := g.px + p.Orbit.Radius*Sub*cos64[idx]/1024
			by := g.py + p.Orbit.Radius*Sub*cos64[(idx+48)%64]/1024
			for i := range g.foes {
				f := &g.foes[i]
				if f.HP <= 0 || (f.OrbitHit >= 0 && g.tick-f.OrbitHit < p.Orbit.HitCooldownTicks) {
					continue
				}
				if within(bx, by, f.X, f.Y, (p.Enemies[f.Type].Radius+p.Orbit.BladeRadius)*Sub) {
					f.OrbitHit = g.tick
					g.damage(i, p.Orbit.Damage)
				}
			}
		}
	}
	g.sweepDead()

	// 5. Spirits drift toward the cultivator; a touch outside invulnerability costs one hộ thể and dissolves it.
	keepF := g.foes[:0]
	for _, f := range g.foes {
		def := p.Enemies[f.Type]
		dx, dy := toward(f.X, f.Y, g.px, g.py, def.Speed)
		f.X += dx
		f.Y += dy
		if g.invuln == 0 && !g.over && within(f.X, f.Y, g.px, g.py, (def.Radius+p.Player.Radius)*Sub) {
			g.hp--
			g.hits++
			g.invuln = p.Player.InvulnTicks
			if g.hp <= 0 {
				g.end(ReasonFallen, false)
			}
			continue
		}
		keepF = append(keepF, f)
	}
	g.foes = keepF

	// 6. Linh khí motes: picked up within pickup_radius, pulled within magnet_radius, fade after life_ticks.
	keepM := g.motes[:0]
	for _, m := range g.motes {
		if within(m.X, m.Y, g.px, g.py, p.Orbs.PickupRadius*Sub) {
			g.qi += m.Value
			g.score += m.Value
			continue
		}
		if within(m.X, m.Y, g.px, g.py, p.Orbs.MagnetRadius*Sub) {
			dx, dy := toward(m.X, m.Y, g.px, g.py, p.Orbs.MagnetSpeed)
			m.X += dx
			m.Y += dy
		}
		m.Life--
		if m.Life > 0 {
			keepM = append(keepM, m)
		}
	}
	g.motes = keepM

	// 7. Kiếm ý grows with the linh khí gathered.
	for g.level+1 < int64(len(p.Levels)) && g.qi >= p.Levels[g.level+1].Qi {
		g.level++
	}

	// 8. Spawns: scheduled elites first, then the regular wave.
	if !g.over {
		for i, e := range p.Enemies {
			for _, at := range e.AtTicks {
				if at == g.tick && len(g.foes) < p.Spawn.MaxEnemies {
					g.place(int64(i))
				}
			}
		}
		g.spawnCounter++
		if g.spawnCounter >= g.spawnInterval() {
			g.spawnCounter = 0
			g.spawnWave()
		}
	}

	// 9. Timers and end.
	if g.invuln > 0 {
		g.invuln--
	}
	g.tick++
	if !g.over && g.tick >= p.TimeLimitTicks {
		g.end(ReasonSurvived, true)
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
	for _, v := range []int64{g.tick, b(g.over), b(g.won), g.px, g.py, g.hp, g.invuln, g.level, g.qi, g.score, g.kills, g.spawned,
		g.nextID, g.spawnCounter, g.fireCounter, g.hits, int64(g.rng.S), int64(len(g.foes)), int64(len(g.shots)), int64(len(g.motes))} {
		put(v)
	}
	for _, f := range g.foes {
		for _, v := range []int64{f.ID, f.Type, f.X, f.Y, f.HP, f.OrbitHit} {
			put(v)
		}
	}
	for _, s := range g.shots {
		for _, v := range []int64{s.X, s.Y, s.VX, s.VY, s.Life} {
			put(v)
		}
	}
	for _, m := range g.motes {
		for _, v := range []int64{m.X, m.Y, m.Value, m.Life} {
			put(v)
		}
	}
	return h
}

// ───── input log ─────

// Run is one RLE entry on the wire: [dir, ticks].
type Run [2]int64

// Result is the replay's authoritative end state.
type Result struct {
	Over      bool   `json:"over"`
	Won       bool   `json:"won"`
	Reason    string `json:"reason"`
	EndTick   int64  `json:"endTick"`
	Kills     int64  `json:"kills"`
	Score     int64  `json:"score"`
	Qi        int64  `json:"qi"`
	Level     int64  `json:"level"`
	HPLeft    int64  `json:"hpLeft"`
	Spawned   int64  `json:"spawned"`
	StateHash uint32 `json:"stateHash"`
	// Cadence telemetry (audit).
	Runs             int   `json:"runs"`
	Changes          int64 `json:"changes"`
	MaxWindowChanges int64 `json:"maxWindowChanges"`
	MovingTicks      int64 `json:"movingTicks"`
}

// ErrInvalidLog: the move log is malformed (shape, direction, total length, non-canonical runs, too many runs).
var ErrInvalidLog = errors.New("bicanh: invalid move log")

// ErrImplausibleInput: well-formed but not human — too many direction changes inside one input window.
var ErrImplausibleInput = errors.New("bicanh: implausible movement input")

// Cadence is the log's cadence summary.
type Cadence struct {
	Runs             int
	Changes          int64
	MaxWindowChanges int64
	MovingTicks      int64
}

// maxWindowCount is the largest number of ticks (sorted) inside any window of `window` consecutive ticks.
func maxWindowCount(ticks []int64, window int64) int64 {
	var best int64
	lo := 0
	for hi := range ticks {
		for ticks[hi]-ticks[lo] >= window {
			lo++
		}
		best = max(best, int64(hi-lo+1))
	}
	return best
}

// Check validates the log shape (ErrInvalidLog) and measures its cadence. The runs must cover exactly endTick ticks,
// each run ≥ 1 tick, consecutive runs with different directions (canonical RLE).
func Check(p *Params, runs []Run, endTick int64) (Cadence, error) {
	var c Cadence
	if endTick <= 0 || endTick > p.TimeLimitTicks {
		return c, fmt.Errorf("%w: endTick %d outside 1..%d", ErrInvalidLog, endTick, p.TimeLimitTicks)
	}
	if len(runs) == 0 || len(runs) > p.MaxRuns {
		return c, fmt.Errorf("%w: %d runs (1..%d)", ErrInvalidLog, len(runs), p.MaxRuns)
	}
	var total int64
	changes := make([]int64, 0, len(runs))
	for i, r := range runs {
		if r[0] < 0 || r[0] > MaxDir || r[1] < 1 || r[1] > p.TimeLimitTicks {
			return c, fmt.Errorf("%w: run %d out of range", ErrInvalidLog, i)
		}
		if i > 0 {
			if r[0] == runs[i-1][0] {
				return c, fmt.Errorf("%w: run %d repeats the previous direction", ErrInvalidLog, i)
			}
			changes = append(changes, total)
		}
		total += r[1]
		if total > endTick {
			return c, fmt.Errorf("%w: runs exceed endTick", ErrInvalidLog)
		}
		if r[0] != 0 {
			c.MovingTicks += r[1]
		}
	}
	if total != endTick {
		return c, fmt.Errorf("%w: runs cover %d ticks, endTick %d", ErrInvalidLog, total, endTick)
	}
	c.Runs = len(runs)
	c.Changes = int64(len(changes))
	c.MaxWindowChanges = maxWindowCount(changes, p.InputWindowTicks)
	return c, nil
}

// Plausible applies the human-cadence cap to a checked log.
func (p *Params) Plausible(c Cadence) error {
	if c.MaxWindowChanges > p.MaxChangesPerWindow {
		return fmt.Errorf("%w: %d direction changes within %d ticks (max %d)", ErrImplausibleInput, c.MaxWindowChanges, p.InputWindowTicks, p.MaxChangesPerWindow)
	}
	return nil
}

// Replay simulates from tick 0 to endTick (or until the game ends earlier) feeding the runs. Malformed logs →
// ErrInvalidLog; non-human cadence → ErrImplausibleInput (Result carries the cadence telemetry). A game still running
// at endTick ends with reason gave_up; one that ends before endTick is reported with reason ended_early.
func Replay(p *Params, seed uint32, runs []Run, endTick int64) (Result, error) {
	c, err := Check(p, runs, endTick)
	if err != nil {
		return Result{}, err
	}
	tele := Result{Runs: c.Runs, Changes: c.Changes, MaxWindowChanges: c.MaxWindowChanges, MovingTicks: c.MovingTicks}
	if err := p.Plausible(c); err != nil {
		return tele, err
	}
	g := New(p, seed)
	for _, r := range runs {
		for k := int64(0); k < r[1] && !g.over; k++ {
			g.Step(r[0])
		}
		if g.over {
			break
		}
	}
	res := g.Result()
	res.Runs, res.Changes, res.MaxWindowChanges, res.MovingTicks = tele.Runs, tele.Changes, tele.MaxWindowChanges, tele.MovingTicks
	switch {
	case !g.over:
		res.Reason = ReasonGaveUp
	case g.tick < endTick:
		res.Reason = ReasonEndedEarly
	}
	return res, nil
}

// Result returns the end state (reason gave_up while running; no cadence telemetry).
func (g *Game) Result() Result {
	r := Result{Over: g.over, Won: g.won, Reason: g.reason, EndTick: g.tick, Kills: g.kills, Score: g.score, Qi: g.qi, Level: g.level,
		HPLeft: g.hp, Spawned: g.spawned, StateHash: g.Hash()}
	if !g.over {
		r.Reason = ReasonGaveUp
	}
	return r
}

// ───── read-only accessors (reference player, tests) ─────

// Clone deep-copies the game.
func (g *Game) Clone() *Game {
	c := *g
	c.foes = append([]Foe(nil), g.foes...)
	c.shots = append([]Shot(nil), g.shots...)
	c.motes = append([]Mote(nil), g.motes...)
	return &c
}

// Params returns the rules.
func (g *Game) Params() *Params { return g.p }

// Tick is the number of ticks simulated so far.
func (g *Game) Tick() int64 { return g.tick }

// Over reports whether the game ended.
func (g *Game) Over() bool { return g.over }

// Pos returns the cultivator's position (sub-units).
func (g *Game) Pos() (int64, int64) { return g.px, g.py }

// HP returns hộ thể left and the invulnerability ticks left.
func (g *Game) HP() (int64, int64) { return g.hp, g.invuln }

// Foes returns the live spirits. Do not mutate.
func (g *Game) Foes() []Foe { return g.foes }

// Motes returns the linh khí motes. Do not mutate.
func (g *Game) Motes() []Mote { return g.motes }
