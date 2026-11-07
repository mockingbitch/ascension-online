// Package tamma is the pure, deterministic simulator of "Vượt Tâm Ma", the falling-block trial that guards the
// breakthrough to Nguyên Anh (docs/design/tam-ma-luat-choi.md). The server never trusts a client score: the client
// submits only its input log (tick-numbered actions) and the claimed end state; Replay re-runs the whole game from
// the run's seed and params and the verdict comes from that replay.
//
// The TypeScript twin lives in apps/web/src/lib/minigames/tam-ma/sim.ts and MUST stay bit-identical: integer math
// only, a fixed PRNG (xorshift32), a fixed step order and the FNV-1a state hash below. The shared golden vector
// packages/contracts/golden/tam-ma-v2.json is checked by both test suites.
package tamma

import (
	"errors"
	"fmt"
)

// Action is one player input. Codes are part of the wire format (input log) — never renumber.
type Action uint8

const (
	ActLeft     Action = 1
	ActRight    Action = 2
	ActRotateCW Action = 3
	ActRotateCC Action = 4
	ActSoftDrop Action = 5
	ActHardDrop Action = 6
	ActCalm     Action = 7 // tĩnh tâm: spend a full calm charge to freeze gravity and the rising tạp niệm
)

// Garbage is the board value of a risen "tạp niệm" row cell; pieces use 1..len(pieces).
const Garbage uint8 = 15

// End reasons. Part of the API (verdict reason) — never rename.
const (
	ReasonCleared   = "cleared"     // pass_lines reached in time
	ReasonToppedOut = "topped_out"  // a piece could not spawn, or tạp niệm pushed a block out of the top
	ReasonTimeUp    = "time_up"     // time_limit_ticks elapsed
	ReasonGaveUp    = "gave_up"     // the client stopped before the game ended
	ReasonEndedEarl = "ended_early" // the replay ended before the claimed end tick (tampered log)
)

// Gravity: rows fall every max(min_ticks, start_ticks − tick / step_every_ticks) ticks.
type Gravity struct {
	StartTicks     int64 `yaml:"start_ticks" json:"startTicks"`
	MinTicks       int64 `yaml:"min_ticks" json:"minTicks"`
	StepEveryTicks int64 `yaml:"step_every_ticks" json:"stepEveryTicks"`
}

// Calm ("tĩnh tâm"): each cleared line charges charge_per_line up to charge_max; a full charge can be spent to
// freeze gravity and the rising rows for duration_ticks.
type Calm struct {
	ChargePerLine int64 `yaml:"charge_per_line" json:"chargePerLine"`
	ChargeMax     int64 `yaml:"charge_max" json:"chargeMax"`
	DurationTicks int64 `yaml:"duration_ticks" json:"durationTicks"`
}

// Piece is one "tạp niệm" shape: 3 or 5 connected cells, normalized to min x = min y = 0 (rotation 0).
type Piece struct {
	ID     string   `yaml:"id" json:"id"`
	Name   string   `yaml:"name" json:"name"`
	Weight int      `yaml:"weight" json:"weight"` // copies in the shuffled bag
	Cells  [][2]int `yaml:"cells" json:"cells"`   // [x, y]
}

// Params is everything the simulation depends on besides the seed and the inputs. A run stores a snapshot, so a
// catalogue change never alters a run in flight.
type Params struct {
	RulesVersion   int64   `yaml:"rules_version" json:"rulesVersion"`
	TickMS         int64   `yaml:"tick_ms" json:"tickMs"`
	Width          int     `yaml:"width" json:"width"`
	Height         int     `yaml:"height" json:"height"`
	TimeLimitTicks int64   `yaml:"time_limit_ticks" json:"timeLimitTicks"`
	PassLines      int64   `yaml:"pass_lines" json:"passLines"`
	Gravity        Gravity `yaml:"gravity" json:"gravity"`
	RiseEveryTicks int64   `yaml:"rise_every_ticks" json:"riseEveryTicks"`
	Calm           Calm    `yaml:"calm" json:"calm"`
	LineScores     []int64 `yaml:"line_scores" json:"lineScores"` // index = lines cleared by one lock
	MaxInputs      int     `yaml:"max_inputs" json:"maxInputs"`
	Pieces         []Piece `yaml:"pieces" json:"pieces"`

	// Human-plausibility rules (rules v2). They bound tool-assisted play; they do not make the gate bot-proof.
	// InputWindowTicks/MaxInputsPerWindow: a log with more inputs in any window of that many ticks is rejected
	// (implausible_input); the client throttles to stay inside, so an honest log never violates it.
	InputWindowTicks   int64 `yaml:"input_window_ticks" json:"inputWindowTicks"`
	MaxInputsPerWindow int   `yaml:"max_inputs_per_window" json:"maxInputsPerWindow"`
	// MinLockGapTicks: a hard drop, or a soft drop that would lock, is ignored until this many ticks after the
	// previous lock (a game rule, identical on both sides — superhuman placement speed is simply impossible).
	MinLockGapTicks int64 `yaml:"min_lock_gap_ticks" json:"minLockGapTicks"`
	// MinPassTicks: the pass needs pass_lines AND having held the mind this long; a pass costs real minutes.
	MinPassTicks int64 `yaml:"min_pass_ticks" json:"minPassTicks"`
}

// RulesVersion is the only rules version this simulator implements.
const RulesVersion = 2

// Validate checks ranges and the piece set (original identity: 3- or 5-cell connected blots, never tetrominoes).
func (p *Params) Validate() error {
	var errs []error
	fail := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	if p.RulesVersion != RulesVersion {
		fail("rules_version must be %d", RulesVersion)
	}
	if p.TickMS < 16 || p.TickMS > 100 {
		fail("tick_ms must be within 16..100")
	}
	if p.Width < 6 || p.Width > 16 || p.Height < 10 || p.Height > 30 {
		fail("board must be 6..16 wide and 10..30 tall")
	}
	if p.TimeLimitTicks <= 0 || p.TimeLimitTicks > 20*60*1000/max(p.TickMS, 1) {
		fail("time_limit_ticks must be within 1..20 minutes")
	}
	if p.PassLines <= 0 || p.PassLines > 200 {
		fail("pass_lines must be within 1..200")
	}
	g := p.Gravity
	if g.MinTicks < 1 || g.StartTicks < g.MinTicks || g.StepEveryTicks < 1 {
		fail("gravity: need 1 ≤ min_ticks ≤ start_ticks and step_every_ticks ≥ 1")
	}
	if p.RiseEveryTicks < 1 {
		fail("rise_every_ticks must be ≥ 1")
	}
	c := p.Calm
	if c.ChargePerLine < 0 || c.ChargeMax < 1 || c.DurationTicks < 1 {
		fail("calm: need charge_per_line ≥ 0, charge_max ≥ 1, duration_ticks ≥ 1")
	}
	if len(p.LineScores) < 2 || p.LineScores[0] != 0 {
		fail("line_scores needs ≥ 2 entries and line_scores[0] = 0")
	}
	for i := 1; i < len(p.LineScores); i++ {
		if p.LineScores[i] < p.LineScores[i-1] || p.LineScores[i] > 100_000 {
			fail("line_scores must be non-decreasing and ≤ 100000")
		}
	}
	if p.MaxInputs < 1 || p.MaxInputs > 10_000 {
		fail("max_inputs must be within 1..10000")
	}
	// Window: between 1 s and 5 s of play, at most 25 inputs per second on average.
	if p.InputWindowTicks*p.TickMS < 1000 || p.InputWindowTicks*p.TickMS > 5000 || p.MaxInputsPerWindow < 1 ||
		int64(p.MaxInputsPerWindow)*1000 > 25*p.InputWindowTicks*p.TickMS {
		fail("input window: input_window_ticks must span 1..5 s and max_inputs_per_window ≤ 25 inputs/s")
	}
	if p.MinLockGapTicks*p.TickMS < 100 || p.MinLockGapTicks*p.TickMS > 2000 {
		fail("min_lock_gap_ticks must span 100..2000 ms")
	}
	if p.MinPassTicks < 0 || p.MinPassTicks >= p.TimeLimitTicks {
		fail("min_pass_ticks must be within 0..time_limit_ticks-1")
	}
	if len(p.Pieces) < 2 || len(p.Pieces) > 8 {
		fail("need 2..8 pieces")
	}
	ids := map[string]bool{}
	for i, pc := range p.Pieces {
		if pc.ID == "" || ids[pc.ID] {
			fail("piece[%d]: empty or duplicate id %q", i, pc.ID)
		}
		ids[pc.ID] = true
		if pc.Name == "" {
			fail("piece %q: empty name", pc.ID)
		}
		if pc.Weight < 1 || pc.Weight > 8 {
			fail("piece %q: weight must be within 1..8", pc.ID)
		}
		if n := len(pc.Cells); n != 3 && n != 5 {
			fail("piece %q: %d cells; tâm ma blots have 3 or 5 cells (no 4-cell tetromino set)", pc.ID, n)
			continue
		}
		if err := checkShape(pc.Cells, p.Width); err != nil {
			fail("piece %q: %v", pc.ID, err)
		}
	}
	return errors.Join(errs...)
}

func checkShape(cells [][2]int, width int) error {
	seen := map[[2]int]bool{}
	minX, minY, maxX := 99, 99, -1
	for _, c := range cells {
		if c[0] < 0 || c[1] < 0 || c[0] > 4 || c[1] > 4 {
			return fmt.Errorf("cell %v outside 0..4", c)
		}
		if seen[c] {
			return fmt.Errorf("duplicate cell %v", c)
		}
		seen[c] = true
		minX, minY, maxX = min(minX, c[0]), min(minY, c[1]), max(maxX, c[0])
	}
	if minX != 0 || minY != 0 {
		return errors.New("cells must be normalized (min x = min y = 0)")
	}
	if maxX+1 > width {
		return errors.New("wider than the board")
	}
	// 4-connected flood fill from the first cell.
	stack, reached := [][2]int{cells[0]}, map[[2]int]bool{cells[0]: true}
	for len(stack) > 0 {
		c := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			n := [2]int{c[0] + d[0], c[1] + d[1]}
			if seen[n] && !reached[n] {
				reached[n] = true
				stack = append(stack, n)
			}
		}
	}
	if len(reached) != len(cells) {
		return errors.New("cells are not connected")
	}
	return nil
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

// IntN returns Next() mod n (the tiny modulo bias is irrelevant here and keeps Go/TS trivially identical).
func (x *XorShift32) IntN(n int) int { return int(x.Next() % uint32(n)) }

// RiseSeedXor separates the rising-row stream from the piece stream.
const RiseSeedXor uint32 = 0xA5A5A5A5

// ───── game ─────

type cell struct{ x, y int }

type shape struct {
	cells []cell
	w, h  int
}

// Game is one deterministic session. Not safe for concurrent use.
type Game struct {
	p      *Params
	shapes [][4]shape // per piece, per rotation (0 = as authored, then clockwise)
	board  []uint8    // row-major, y = 0 is the top row

	pieceRng, riseRng XorShift32
	bag               []int
	bagPos            int

	cur, rot, px, py, next int

	tick        int64
	gravCounter int64
	riseCounter int64
	calmLeft    int64
	charge      int64
	lines       int64
	score       int64
	locked      int64
	lastLock    int64 // tick of the previous lock (starts at −min_lock_gap_ticks)
	minGap      int64 // telemetry: smallest gap between two player-driven locks (0 = none yet)

	over   bool
	won    bool
	reason string
}

// New starts a game. p must be valid (Validate).
func New(p *Params, seed uint32) *Game {
	g := &Game{p: p, board: make([]uint8, p.Width*p.Height), pieceRng: NewXorShift32(seed), riseRng: NewXorShift32(seed ^ RiseSeedXor),
		lastLock: -p.MinLockGapTicks}
	g.shapes = make([][4]shape, len(p.Pieces))
	for i, pc := range p.Pieces {
		s := shape{}
		for _, c := range pc.Cells {
			s.cells = append(s.cells, cell{c[0], c[1]})
		}
		s.w, s.h = extent(s.cells)
		g.shapes[i][0] = s
		for r := 1; r < 4; r++ {
			g.shapes[i][r] = rotateCW(g.shapes[i][r-1])
		}
	}
	g.cur = g.draw()
	g.next = g.draw()
	g.spawn()
	return g
}

func extent(cs []cell) (w, h int) {
	for _, c := range cs {
		w, h = max(w, c.x+1), max(h, c.y+1)
	}
	return
}

// rotateCW maps (x, y) → (h − 1 − y, x), keeping cell order; the result is already normalized.
func rotateCW(s shape) shape {
	out := shape{w: s.h, h: s.w}
	for _, c := range s.cells {
		out.cells = append(out.cells, cell{s.h - 1 - c.y, c.x})
	}
	return out
}

// draw takes the next piece from the weighted bag, refilling + Fisher–Yates shuffling it when empty.
func (g *Game) draw() int {
	if g.bagPos >= len(g.bag) {
		g.bag = g.bag[:0]
		for i, pc := range g.p.Pieces {
			for range pc.Weight {
				g.bag = append(g.bag, i)
			}
		}
		for i := len(g.bag) - 1; i > 0; i-- {
			j := g.pieceRng.IntN(i + 1)
			g.bag[i], g.bag[j] = g.bag[j], g.bag[i]
		}
		g.bagPos = 0
	}
	v := g.bag[g.bagPos]
	g.bagPos++
	return v
}

func (g *Game) collides(piece, rot, px, py int) bool {
	for _, c := range g.shapes[piece][rot].cells {
		x, y := px+c.x, py+c.y
		if x < 0 || x >= g.p.Width || y < 0 || y >= g.p.Height {
			return true
		}
		if g.board[y*g.p.Width+x] != 0 {
			return true
		}
	}
	return false
}

func (g *Game) end(reason string, won bool) {
	g.over, g.won, g.reason = true, won, reason
}

func (g *Game) spawn() {
	g.rot = 0
	g.px = (g.p.Width - g.shapes[g.cur][0].w) / 2
	g.py = 0
	g.gravCounter = 0
	if g.collides(g.cur, 0, g.px, g.py) {
		g.end(ReasonToppedOut, false)
	}
}

func (g *Game) tryMove(dx, dy int) bool {
	if g.collides(g.cur, g.rot, g.px+dx, g.py+dy) {
		return false
	}
	g.px += dx
	g.py += dy
	return true
}

var kicks = [...][2]int{{0, 0}, {-1, 0}, {1, 0}, {0, -1}, {-2, 0}, {2, 0}}

func (g *Game) tryRotate(dir int) {
	nr := (g.rot + dir + 4) % 4
	from, to := g.shapes[g.cur][g.rot], g.shapes[g.cur][nr]
	bx := g.px + (from.w-to.w)/2 // Go truncates toward zero; TS uses `| 0` — identical
	by := g.py + (from.h-to.h)/2
	for _, k := range kicks {
		if !g.collides(g.cur, nr, bx+k[0], by+k[1]) {
			g.rot, g.px, g.py = nr, bx+k[0], by+k[1]
			return
		}
	}
}

// lock merges the falling piece, clears full rows, scores, charges calm, then spawns the next piece.
func (g *Game) lock() {
	w := g.p.Width
	for _, c := range g.shapes[g.cur][g.rot].cells {
		g.board[(g.py+c.y)*w+g.px+c.x] = uint8(g.cur + 1)
	}
	g.locked++
	g.lastLock = g.tick
	cleared := 0
	for y := g.p.Height - 1; y >= 0; {
		full := true
		for x := range w {
			if g.board[y*w+x] == 0 {
				full = false
				break
			}
		}
		if !full {
			y--
			continue
		}
		cleared++
		copy(g.board[w:(y+1)*w], g.board[:y*w]) // shift rows 0..y-1 down by one
		for x := range w {
			g.board[x] = 0
		}
	}
	if cleared > 0 {
		g.lines += int64(cleared)
		g.score += g.p.LineScores[min(cleared, len(g.p.LineScores)-1)]
		g.charge = min(g.p.Calm.ChargeMax, g.charge+int64(cleared)*g.p.Calm.ChargePerLine)
	}
	g.cur = g.next
	g.next = g.draw()
	g.spawn()
}

// rise pushes one "tạp niệm" row in from the bottom with a single gap.
func (g *Game) rise() {
	w, h := g.p.Width, g.p.Height
	for x := range w {
		if g.board[x] != 0 {
			g.end(ReasonToppedOut, false)
			return
		}
	}
	copy(g.board[:(h-1)*w], g.board[w:])
	gap := g.riseRng.IntN(w)
	for x := range w {
		v := Garbage
		if x == gap {
			v = 0
		}
		g.board[(h-1)*w+x] = v
	}
	if g.collides(g.cur, g.rot, g.px, g.py) {
		if g.collides(g.cur, g.rot, g.px, g.py-1) {
			g.end(ReasonToppedOut, false)
			return
		}
		g.py--
	}
}

func (g *Game) lockAllowed() bool { return g.tick-g.lastLock >= g.p.MinLockGapTicks }

func (g *Game) playerLock() {
	if gap := g.tick - g.lastLock; g.locked > 0 && (g.minGap == 0 || gap < g.minGap) {
		g.minGap = gap
	}
	g.lock()
}

func (g *Game) gravityTicks() int64 {
	gr := g.p.Gravity
	return max(gr.MinTicks, gr.StartTicks-g.tick/gr.StepEveryTicks)
}

func (g *Game) apply(a Action) {
	switch a {
	case ActLeft:
		g.tryMove(-1, 0)
	case ActRight:
		g.tryMove(1, 0)
	case ActRotateCW:
		g.tryRotate(1)
	case ActRotateCC:
		g.tryRotate(-1)
	case ActSoftDrop:
		if g.tryMove(0, 1) {
			g.gravCounter = 0
		} else if g.lockAllowed() {
			g.playerLock()
		}
	case ActHardDrop:
		if g.lockAllowed() {
			for g.tryMove(0, 1) {
			}
			g.playerLock()
		}
	case ActCalm:
		if g.charge >= g.p.Calm.ChargeMax && g.calmLeft == 0 {
			g.calmLeft = g.p.Calm.DurationTicks
			g.charge = 0
		}
	}
}

// Step advances one tick: the tick's actions in order, then gravity and rising rows (both frozen while calm).
func (g *Game) Step(actions []Action) {
	if g.over {
		return
	}
	for _, a := range actions {
		if g.over {
			break
		}
		g.apply(a)
	}
	if !g.over {
		if g.calmLeft > 0 {
			g.calmLeft--
		} else {
			g.gravCounter++
			if g.gravCounter >= g.gravityTicks() {
				g.gravCounter = 0
				if !g.tryMove(0, 1) {
					g.lock()
				}
			}
			if !g.over {
				g.riseCounter++
				if g.riseCounter >= g.p.RiseEveryTicks {
					g.riseCounter = 0
					g.rise()
				}
			}
		}
	}
	g.tick++
	if !g.over && g.lines >= g.p.PassLines && g.tick >= g.p.MinPassTicks {
		g.end(ReasonCleared, true)
	}
	if !g.over && g.tick >= g.p.TimeLimitTicks {
		g.end(ReasonTimeUp, false)
	}
}

// Over reports whether the game ended.
func (g *Game) Over() bool { return g.over }

// Tick is the number of ticks simulated so far.
func (g *Game) Tick() int64 { return g.tick }

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
	for _, v := range []int64{g.tick, b(g.over), b(g.won), g.lines, g.score, g.charge, g.calmLeft, g.gravCounter, g.riseCounter,
		int64(g.cur), int64(g.rot), int64(g.px), int64(g.py), int64(g.next), int64(g.bagPos), int64(len(g.bag))} {
		put(v)
	}
	for _, v := range g.bag {
		put(int64(v))
	}
	put(int64(g.pieceRng.S))
	put(int64(g.riseRng.S))
	put(g.locked)
	put(g.lastLock)
	put(g.minGap)
	for _, c := range g.board {
		put(int64(c))
	}
	return h
}

// Input is one logged action. JSON wire form: [tick, action].
type Input struct {
	Tick   int64
	Action Action
}

// Result is the replay's authoritative end state.
type Result struct {
	Over      bool   `json:"over"`
	Won       bool   `json:"won"`
	Reason    string `json:"reason"`
	EndTick   int64  `json:"endTick"`
	Lines     int64  `json:"lines"`
	Score     int64  `json:"score"`
	Pieces    int64  `json:"pieces"`
	StateHash uint32 `json:"stateHash"`
	// Cadence telemetry (audit): inputs, densest input window, smallest gap between player-driven locks.
	Inputs          int   `json:"inputs"`
	MaxWindowInputs int   `json:"maxWindowInputs"`
	MinLockGapTicks int64 `json:"minLockGapTicks"`
}

// ErrInvalidLog: the input log is malformed (order, range, unknown action, too many inputs).
var ErrInvalidLog = errors.New("tamma: invalid input log")

// ErrImplausibleInput: the log is well-formed but faster than a human (more than max_inputs_per_window inputs in
// some window of input_window_ticks).
var ErrImplausibleInput = errors.New("tamma: implausible input cadence")

// MaxWindowInputs is the largest number of inputs inside any window of `window` consecutive ticks
// (ticks must be non-decreasing). Same two-pointer algorithm as sim.ts maxWindowInputs.
func MaxWindowInputs(inputs []Input, window int64) int {
	best, lo := 0, 0
	for hi := range inputs {
		for inputs[hi].Tick-inputs[lo].Tick >= window {
			lo++
		}
		best = max(best, hi-lo+1)
	}
	return best
}

// Replay simulates from tick 0 to endTick (or until the game ends earlier) feeding inputs at their ticks.
// Rules: 0 < endTick ≤ time_limit_ticks; ticks non-decreasing within [0, endTick); ≤ max_inputs in total; actions
// 1..7 (else ErrInvalidLog); ≤ max_inputs_per_window in any input_window_ticks window (else ErrImplausibleInput).
// A game still running at endTick ends with reason gave_up; a game that ends before endTick is reported with reason
// ended_early (the claim and the replay disagree).
func Replay(p *Params, seed uint32, inputs []Input, endTick int64) (Result, error) {
	if endTick <= 0 || endTick > p.TimeLimitTicks {
		return Result{}, fmt.Errorf("%w: endTick %d outside 1..%d", ErrInvalidLog, endTick, p.TimeLimitTicks)
	}
	if len(inputs) > p.MaxInputs {
		return Result{}, fmt.Errorf("%w: %d inputs > %d", ErrInvalidLog, len(inputs), p.MaxInputs)
	}
	prev := int64(-1)
	for i, in := range inputs {
		if in.Tick < 0 || in.Tick >= endTick || in.Tick < prev {
			return Result{}, fmt.Errorf("%w: input %d tick %d out of order or range", ErrInvalidLog, i, in.Tick)
		}
		if in.Action < ActLeft || in.Action > ActCalm {
			return Result{}, fmt.Errorf("%w: input %d unknown action %d", ErrInvalidLog, i, in.Action)
		}
		prev = in.Tick
	}
	window := MaxWindowInputs(inputs, p.InputWindowTicks)
	if window > p.MaxInputsPerWindow {
		return Result{MaxWindowInputs: window, Inputs: len(inputs)}, fmt.Errorf("%w: %d inputs within %d ticks (max %d)",
			ErrImplausibleInput, window, p.InputWindowTicks, p.MaxInputsPerWindow)
	}
	g := New(p, seed)
	i := 0
	var batch []Action
	for !g.over && g.tick < endTick {
		batch = batch[:0]
		for i < len(inputs) && inputs[i].Tick == g.tick {
			batch = append(batch, inputs[i].Action)
			i++
		}
		g.Step(batch)
	}
	r := g.result()
	r.Inputs, r.MaxWindowInputs = len(inputs), window
	switch {
	case !g.over:
		r.Reason = ReasonGaveUp
	case g.tick < endTick:
		r.Reason = ReasonEndedEarl
	}
	return r, nil
}

func (g *Game) result() Result {
	r := Result{Over: g.over, Won: g.won, Reason: g.reason, EndTick: g.tick, Lines: g.lines, Score: g.score, Pieces: g.locked,
		StateHash: g.Hash(), MinLockGapTicks: g.minGap}
	if !g.over {
		r.Reason = ReasonGaveUp
	}
	return r
}

// ───── read-only accessors (rendering helpers and the test-only reference player) ─────

// Clone deep-copies the game.
func (g *Game) Clone() *Game {
	c := *g
	c.board = append([]uint8(nil), g.board...)
	c.bag = append([]int(nil), g.bag...)
	return &c
}

// Apply applies one action immediately (no tick passes). For look-ahead in tools; Step is the game loop.
func (g *Game) Apply(a Action) { g.apply(a) }

// Params returns the rules.
func (g *Game) Params() *Params { return g.p }

// Board returns the cells (row-major, y = 0 on top). Do not mutate.
func (g *Game) Board() []uint8 { return g.board }

// Piece returns the falling piece index, rotation and position.
func (g *Game) Piece() (piece, rot, x, y int) { return g.cur, g.rot, g.px, g.py }

// Counters returns lines, locked pieces, calm charge, calm ticks left; Won/Reason report the end.
func (g *Game) Counters() (lines, locked, charge, calmLeft int64) {
	return g.lines, g.locked, g.charge, g.calmLeft
}

// Won reports a pass.
func (g *Game) Won() bool { return g.won }

// Reason is the end reason ("" while running).
func (g *Game) Reason() string { return g.reason }

// Result returns the end state as Replay would report it (reason gave_up while running).
func (g *Game) Result() Result { return g.result() }

// MaxScore is an upper bound of the score for a number of cleared lines (cheap plausibility check before replay).
func (p *Params) MaxScore(lines int64) int64 {
	best := int64(0) // best score per line over all clear sizes
	for n := 1; n < len(p.LineScores); n++ {
		best = max(best, (p.LineScores[n]+int64(n)-1)/int64(n))
	}
	return best * max(lines, 0)
}
