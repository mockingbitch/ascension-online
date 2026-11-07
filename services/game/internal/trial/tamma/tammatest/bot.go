// Package tammatest holds the reference player for Vượt Tâm Ma. TEST-ONLY: imported by _test.go files (difficulty
// sanity, golden vectors, integration tests), never by the API or worker — the verdict path does not need it, and a
// solver should not ship in the server binary.
package tammatest

import "tiendao/game/internal/trial/tamma"

// Params mirrors the shipped trials.yaml tâm ma params (rules v2). Tests that must follow the catalogue load it
// instead; the golden file embeds its own copy.
func Params() *tamma.Params {
	return &tamma.Params{
		RulesVersion: 2, TickMS: 50, Width: 9, Height: 16, TimeLimitTicks: 3600, PassLines: 12,
		Gravity: tamma.Gravity{StartTicks: 14, MinTicks: 5, StepEveryTicks: 400}, RiseEveryTicks: 400,
		Calm: tamma.Calm{ChargePerLine: 1, ChargeMax: 4, DurationTicks: 160}, LineScores: []int64{0, 100, 300, 600, 1000, 1500},
		MaxInputs: 4000, InputWindowTicks: 20, MaxInputsPerWindow: 16, MinLockGapTicks: 6, MinPassTicks: 1500,
		Pieces: []tamma.Piece{
			{ID: "vong", Name: "Vọng Niệm", Weight: 3, Cells: [][2]int{{0, 0}, {1, 0}, {2, 0}}},
			{ID: "chap", Name: "Chấp Niệm", Weight: 3, Cells: [][2]int{{0, 0}, {0, 1}, {1, 1}}},
			{ID: "tham", Name: "Tham", Weight: 1, Cells: [][2]int{{0, 0}, {0, 1}, {0, 2}, {1, 2}, {2, 2}}},
			{ID: "san", Name: "Sân", Weight: 1, Cells: [][2]int{{0, 0}, {1, 0}, {0, 1}, {1, 1}, {0, 2}}},
			{ID: "si", Name: "Si", Weight: 1, Cells: [][2]int{{0, 0}, {1, 0}, {2, 0}, {1, 1}, {1, 2}}},
			{ID: "man", Name: "Mạn", Weight: 1, Cells: [][2]int{{0, 0}, {1, 0}, {2, 0}, {3, 0}, {4, 0}}},
		},
	}
}

// ThinkTicks is how long the reference player looks at a new piece before its first input (human reading time).
var ThinkTicks = func(gap int64) int64 { return 3 * gap }

// AutoPlay plays seed with the reference player — reads the falling piece and the visible "next" (two-piece
// look-ahead with a classic placement heuristic), waits ThinkTicks(gap) on each new piece, then one input every
// gap+1 ticks, spends tĩnh tâm when full — until the game ends or tick stopAt > 0. Returns the input log and the end
// result exactly as tamma.Replay computes it.
func AutoPlay(p *tamma.Params, seed uint32, gap, stopAt int64) ([]tamma.Input, tamma.Result) {
	g := tamma.New(p, seed)
	var inputs []tamma.Input
	var plan []tamma.Action
	planned := int64(-1)
	wait := int64(0)
	for !g.Over() && (stopAt == 0 || g.Tick() < stopAt) {
		_, locked, charge, calmLeft := g.Counters()
		if planned != locked {
			planned = locked
			plan = bestPlan(g)
			if charge >= p.Calm.ChargeMax && calmLeft == 0 {
				plan = append([]tamma.Action{tamma.ActCalm}, plan...)
			}
			wait = max(wait, ThinkTicks(gap))
		}
		var batch []tamma.Action
		if wait > 0 {
			wait--
		} else {
			if len(plan) == 0 {
				plan = []tamma.Action{tamma.ActHardDrop} // the drop came too soon after the last lock: try again
			}
			batch = []tamma.Action{plan[0]}
			plan = plan[1:]
			wait = gap
			inputs = append(inputs, tamma.Input{Tick: g.Tick(), Action: batch[0]})
		}
		g.Step(batch)
	}
	r := g.Result()
	r.Inputs, r.MaxWindowInputs = len(inputs), tamma.MaxWindowInputs(inputs, p.InputWindowTicks)
	return inputs, r
}

type placement struct {
	acts  []tamma.Action
	after *tamma.Game
	lines int64
}

func lines(g *tamma.Game) int64 { l, _, _, _ := g.Counters(); return l }

// placements lists every reachable rotation × column for the falling piece, hard-dropped (ignoring the lock gap,
// which only delays the drop).
func placements(g *tamma.Game) []placement {
	p := g.Params()
	var out []placement
	for r := range 4 {
		for tx := -2; tx < p.Width+2; tx++ {
			c := g.Clone()
			var acts []tamma.Action
			for range r {
				c.Apply(tamma.ActRotateCW)
				acts = append(acts, tamma.ActRotateCW)
			}
			if _, rot, _, _ := c.Piece(); rot != r {
				continue
			}
			for {
				_, _, x, _ := c.Piece()
				if x == tx {
					break
				}
				a := tamma.ActRight
				if tx < x {
					a = tamma.ActLeft
				}
				c.Apply(a)
				if _, _, nx, _ := c.Piece(); nx == x {
					break
				}
				acts = append(acts, a)
			}
			if _, _, x, _ := c.Piece(); x != tx {
				continue
			}
			before, locked := lines(c), lockedOf(c)
			for range p.MinLockGapTicks + 1 { // let the lock gap pass in the look-ahead copy
				if lockedOf(c) != locked {
					break
				}
				c.Apply(tamma.ActHardDrop)
				if lockedOf(c) == locked {
					c.Step(nil)
				}
			}
			if lockedOf(c) == locked {
				continue
			}
			out = append(out, placement{append(acts, tamma.ActHardDrop), c, lines(c) - before})
		}
	}
	return out
}

func lockedOf(g *tamma.Game) int64 { _, l, _, _ := g.Counters(); return l }

// boardScore: lower stack, fewer holes, flatter surface.
func boardScore(g *tamma.Game) int64 {
	p := g.Params()
	w, h, b := p.Width, p.Height, g.Board()
	var holes, agg, bump, maxH int64
	prev := int64(-1)
	for x := range w {
		colH, seen := int64(0), false
		for y := range h {
			filled := b[y*w+x] != 0
			if filled && !seen {
				seen, colH = true, int64(h-y)
			} else if !filled && seen {
				holes++
			}
		}
		agg += colH
		maxH = max(maxH, colH)
		if prev >= 0 {
			d := colH - prev
			bump += max(d, -d)
		}
		prev = colH
	}
	return -holes*40 - agg*5 - bump*3 - maxH*6
}

func value(c *tamma.Game, l int64) int64 {
	if c.Over() && !c.Won() {
		return -1 << 40
	}
	return l*400 + boardScore(c)
}

func bestPlan(g *tamma.Game) []tamma.Action {
	var best []tamma.Action
	bestScore := int64(-1 << 62)
	for _, p1 := range placements(g) {
		s := value(p1.after, p1.lines)
		if !p1.after.Over() {
			s2 := int64(-1 << 62)
			for _, p2 := range placements(p1.after) {
				s2 = max(s2, value(p2.after, p1.lines+p2.lines))
			}
			if s2 > -1<<62 {
				s = s2
			}
		}
		if s > bestScore {
			bestScore, best = s, p1.acts
		}
	}
	if best == nil {
		best = []tamma.Action{tamma.ActHardDrop}
	}
	return best
}
