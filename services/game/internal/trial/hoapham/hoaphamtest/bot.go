// Package hoaphamtest holds the reference player for Hóa Phàm. TEST-ONLY: imported by _test.go files (difficulty
// sanity, golden vectors, integration tests), never by the API or worker — the verdict path does not need it, and a
// solver should not ship in the server binary.
package hoaphamtest

import (
	"math/rand/v2"

	"tiendao/game/internal/trial/hoapham"
)

// Params mirrors the shipped trials.yaml hóa phàm params (rules v1). Tests that must follow the catalogue load it
// instead; the golden file embeds its own copy.
func Params() *hoapham.Params {
	return &hoapham.Params{
		RulesVersion: 1, TickMS: 40, Width: 120, Height: 160, TimeLimitTicks: 2250, PassScore: 75, MinPassTicks: 1500,
		Lanterns: 3, GravitySub: 3,
		Knot:       hoapham.Motion{VY: hoapham.Range{Min: 125, Max: 170}, VXMax: 12},
		Lantern:    hoapham.Motion{VY: hoapham.Range{Min: 40, Max: 60}, VXMax: 6},
		Spawn:      hoapham.Spawn{StartTicks: 44, MinTicks: 22, StepEveryTicks: 60, WaveMax: 4, WaveGrowEveryTicks: 500, MarginUnits: 16},
		MinCutStep: 3, Combo: hoapham.Combo{StepBonus: 1, MaxBonus: 3},
		Calm:       hoapham.Calm{ChargePerCut: 1, ChargeMax: 12, DurationTicks: 125},
		MaxStrokes: 600, MaxStep: 48, InputWindowTicks: 25, MaxWindowTravel: 720, MaxStrokesPerWindow: 8, MinLiftTicks: 2,
		Objects: []hoapham.Object{
			{ID: "ket", Name: "Kết Trần Duyên", Role: hoapham.RoleKnot, Weight: 6, Radius: 9, Points: 1, HP: 1},
			{ID: "to", Name: "Tơ Vương", Role: hoapham.RoleKnot, Weight: 3, Radius: 6, Points: 2, HP: 1},
			{ID: "luoi", Name: "Lưới Nghiệp", Role: hoapham.RoleKnot, Weight: 1, Radius: 13, Points: 3, HP: 2},
			{ID: "den", Name: "Đăng Chân Tâm", Role: hoapham.RoleLantern, Weight: 2, Radius: 9, Points: 0, HP: 1},
		},
	}
}

// Skill is a human profile.
type Skill struct {
	Name   string
	React  int64 // ticks a knot must have been visible before the player goes for it
	Speed  int64 // brush speed while slicing, grid units per tick
	Think  int64 // ticks between two strokes (≥ min_lift_ticks)
	AimErr int64 // aim error, ± grid units on each axis
}

// Skills: decent players at three paces (40 ms ticks).
var Skills = []Skill{
	{Name: "nhanh", React: 8, Speed: 14, Think: 6, AimErr: 4},
	{Name: "vừa", React: 12, Speed: 10, Think: 10, AimErr: 5},
	{Name: "chậm", React: 18, Speed: 7, Think: 16, AimErr: 7},
}

type pt struct{ x, y int64 }

type recorder struct {
	p       *hoapham.Params
	strokes []hoapham.Stroke
	travel  []struct{ tick, l int64 }
	starts  []int64
	lastEnd int64
	cur     pt
	in      bool
}

// room is the travel budget at tick (same rule as the web recorder).
func (r *recorder) room(tick int64) int64 {
	used := int64(0)
	for i := len(r.travel) - 1; i >= 0 && r.travel[i].tick > tick-r.p.InputWindowTicks; i-- {
		used += r.travel[i].l
	}
	return min(r.p.MaxStep, r.p.MaxWindowTravel-used)
}

func (r *recorder) canStart(tick int64) bool {
	if len(r.strokes) >= r.p.MaxStrokes || (len(r.strokes) > 0 && tick-r.lastEnd-1 < r.p.MinLiftTicks) {
		return false
	}
	n := 0
	for i := len(r.starts) - 1; i >= 0 && r.starts[i] > tick-r.p.InputWindowTicks; i-- {
		n++
	}
	return n < r.p.MaxStrokesPerWindow
}

func clampField(p *hoapham.Params, q pt) pt {
	return pt{min(max(q.x, 0), p.Width-1), min(max(q.y, 0), p.Height-1)}
}

// AutoPlay plays seed with the reference player at skill s until the game ends or tick stopAt > 0. Returns the stroke
// log and the end result exactly as hoapham.Replay computes it (cadence telemetry included).
func AutoPlay(p *hoapham.Params, seed uint32, s Skill, stopAt int64) ([]hoapham.Stroke, hoapham.Result) {
	g := hoapham.New(p, seed)
	rng := rand.New(rand.NewPCG(uint64(seed), 0x484f41)) // the bot's own noise, never the game's stream
	rec := &recorder{p: p, lastEnd: -1 << 30}
	seen := map[int64]int64{}
	var plan []pt
	wait := int64(0)
	for !g.Over() && (stopAt == 0 || g.Tick() < stopAt) {
		t := g.Tick()
		for _, o := range g.Objects() {
			if _, ok := seen[o.ID]; !ok {
				seen[o.ID] = t
			}
		}
		switch {
		case len(plan) > 0 && rec.in:
			want := clampField(p, plan[0])
			plan = plan[1:]
			nx, ny := hoapham.Toward(rec.cur.x, rec.cur.y, want.x, want.y, rec.room(t))
			dx, dy := nx-rec.cur.x, ny-rec.cur.y
			last := &rec.strokes[len(rec.strokes)-1]
			*last = append(*last, dx, dy)
			rec.travel = append(rec.travel, struct{ tick, l int64 }{t, hoapham.StepLen(dx, dy)})
			rec.cur = pt{nx, ny}
			g.Step(true, nx, ny)
		case rec.in:
			rec.in = false
			rec.lastEnd = t - 1
			wait = s.Think
			g.Step(false, 0, 0)
		default:
			if wait > 0 {
				wait--
			}
			if wait == 0 && rec.canStart(t) {
				plan = choose(g, s, seen, rng)
			}
			if len(plan) == 0 {
				g.Step(false, 0, 0)
				continue
			}
			st := clampField(p, plan[0])
			plan = plan[1:]
			rec.strokes = append(rec.strokes, hoapham.Stroke{t, st.x, st.y})
			rec.starts = append(rec.starts, t)
			rec.cur, rec.in = st, true
			g.Step(true, st.x, st.y)
		}
	}
	r := g.Result()
	if c, err := hoapham.Check(p, rec.strokes, max(r.EndTick, 1)); err == nil {
		r.Strokes, r.Points, r.MaxStep, r.MaxWindowTravel, r.MaxWindowStrokes = len(rec.strokes), c.Points, c.MaxStep, c.MaxWindowTravel, c.MaxWindowStrokes
	}
	return rec.strokes, r
}

var dirs = [8][2]int64{{10, 0}, {7, 7}, {0, 10}, {-7, 7}, {-10, 0}, {-7, -7}, {0, -10}, {7, -7}}

// path is a straight swipe of 2·half units through c along dir at speed per tick (start point first).
func path(c pt, d [2]int64, half, speed int64) []pt {
	sx, sy := c.x-d[0]*half/10, c.y-d[1]*half/10
	n := max(2*half/speed, 1)
	out := []pt{{sx, sy}}
	for i := int64(1); i <= n; i++ {
		out = append(out, pt{sx + d[0]*2*half*i/(10*n), sy + d[1]*2*half*i/(10*n)})
	}
	return out
}

// eval plays the swipe on a copy of the game (what an attentive human would see happen) and scores it.
func eval(g *hoapham.Game, pl []pt) int64 {
	c := g.Clone()
	p := c.Params()
	s0, cuts0, l0 := c.Score()
	cur := clampField(p, pl[0])
	c.Step(true, cur.x, cur.y)
	for _, q := range pl[1:] {
		q = clampField(p, q)
		nx, ny := hoapham.Toward(cur.x, cur.y, q.x, q.y, p.MaxStep)
		cur = pt{nx, ny}
		c.Step(true, nx, ny)
	}
	s1, cuts1, l1 := c.Score()
	return (s1-s0)*10 + (cuts1-cuts0)*5 - (l0-l1)*1000
}

// choose picks the best swipe through a knot visible for ≥ React ticks, then adds aim error.
func choose(g *hoapham.Game, s Skill, seen map[int64]int64, rng *rand.Rand) []pt {
	p := g.Params()
	t := g.Tick()
	var best []pt
	bestV := int64(0)
	for _, o := range g.Objects() {
		def := p.Objects[o.Type]
		if def.Role != hoapham.RoleKnot || t-seen[o.ID] < s.React {
			continue
		}
		half := def.Radius + 10
		lead := half / s.Speed
		cx := (o.X + o.VX*lead) / hoapham.Sub
		cy := (o.Y + o.VY*lead + p.GravitySub*lead*lead/2) / hoapham.Sub
		if cx < 2 || cx > p.Width-3 || cy < 2 || cy > p.Height-3 {
			continue
		}
		for _, d := range dirs {
			pl := path(pt{cx, cy}, d, half, s.Speed)
			if v := eval(g, pl); v > bestV {
				bestV, best = v, pl
			}
		}
	}
	if best == nil {
		return nil
	}
	ex, ey := rng.Int64N(2*s.AimErr+1)-s.AimErr, rng.Int64N(2*s.AimErr+1)-s.AimErr
	out := make([]pt, len(best))
	for i, q := range best {
		out[i] = pt{q.x + ex, q.y + ey}
	}
	return out
}
