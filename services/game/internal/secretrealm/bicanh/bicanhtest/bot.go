// Package bicanhtest holds the reference player for Bí Cảnh. TEST-ONLY: imported by _test.go files (difficulty
// sanity, golden vectors, integration tests, balance notes), never by the API or worker binaries — the verdict path
// does not need it, and a solver should not ship in the server.
package bicanhtest

import (
	"math/rand/v2"

	"tiendao/game/internal/secretrealm/bicanh"
)

// Params mirrors the shipped secret_realms.yaml params (rules v1). Tests that must follow the catalogue load it
// instead; the golden file embeds its own copy.
func Params() *bicanh.Params {
	return &bicanh.Params{
		RulesVersion: 1, TickMS: 50, TimeLimitTicks: 3000,
		Arena:  bicanh.Arena{Width: 640, Height: 640},
		Player: bicanh.Player{Radius: 10, Speed: 80, HP: 6, InvulnTicks: 24},
		Sword:  bicanh.Sword{Range: 230, Speed: 240, Radius: 5, LifeTicks: 22, Damage: 1},
		Orbit:  bicanh.Orbit{Radius: 46, BladeRadius: 7, Step: 2, Damage: 1, HitCooldownTicks: 10},
		Levels: []bicanh.Level{{Qi: 0, FireEvery: 14}, {Qi: 10, FireEvery: 12}, {Qi: 30, FireEvery: 11, Orbit: 1},
			{Qi: 60, FireEvery: 10, Orbit: 2}, {Qi: 110, FireEvery: 9, Orbit: 2}, {Qi: 180, FireEvery: 8, Orbit: 3}},
		Spawn: bicanh.Spawn{StartTicks: 28, MinTicks: 8, StepEveryTicks: 140, WaveMax: 4, WaveGrowEveryTicks: 600, MaxEnemies: 80, SafeRadius: 120},
		Enemies: []bicanh.Enemy{
			{ID: "am_hon", Name: "Âm Hồn", Weight: 6, HP: 1, Speed: 28, Radius: 8, Points: 1},
			{ID: "hoa_linh", Name: "Hỏa Linh", Weight: 3, FromTick: 400, HP: 1, Speed: 46, Radius: 6, Points: 2},
			{ID: "thach_quy", Name: "Thạch Quỷ", Weight: 2, FromTick: 900, HP: 4, Speed: 20, Radius: 13, Points: 4},
			{ID: "ta_tuong", Name: "Tà Tướng", Weight: 0, HP: 24, Speed: 16, Radius: 18, Points: 15, AtTicks: []int64{1200, 2100, 2700}},
		},
		Orbs:      bicanh.Orbs{PickupRadius: 22, MagnetRadius: 72, MagnetSpeed: 112, LifeTicks: 500, Max: 90},
		MaxSwords: 12, InputWindowTicks: 20, MaxChangesPerWindow: 8, MaxRuns: 1500,
	}
}

// Skill is a human profile.
type Skill struct {
	Name   string
	Think  int64 // ticks between two decisions (reaction + hand)
	Look   int64 // ticks of look-ahead when judging a direction
	Margin int64 // extra units of distance the player keeps from spirits
	Noise  int   // per-mille chance a decision is a random direction (a slip)
}

// Skills: decent players at three paces (50 ms ticks).
var Skills = []Skill{
	{Name: "nhanh", Think: 3, Look: 8, Margin: 14, Noise: 20},
	{Name: "vừa", Think: 5, Look: 7, Margin: 10, Noise: 50},
	{Name: "chậm", Think: 8, Look: 6, Margin: 6, Noise: 90},
}

// Recorder turns desired directions into the canonical RLE log under the game's cadence rule (≤ max changes per
// window): a change that would exceed it is deferred, exactly as the web recorder does.
type Recorder struct {
	p       *bicanh.Params
	Runs    []bicanh.Run
	changes []int64
	tick    int64
}

// NewRecorder starts an empty log.
func NewRecorder(p *bicanh.Params) *Recorder { return &Recorder{p: p} }

// Sample records one tick with the wanted direction and returns the direction actually applied.
func (r *Recorder) Sample(want int64) int64 {
	defer func() { r.tick++ }()
	if len(r.Runs) == 0 {
		r.Runs = append(r.Runs, bicanh.Run{want, 1})
		return want
	}
	last := &r.Runs[len(r.Runs)-1]
	if want != last[0] && len(r.Runs) < r.p.MaxRuns {
		n := int64(0)
		for i := len(r.changes) - 1; i >= 0 && r.changes[i] > r.tick-r.p.InputWindowTicks; i-- {
			n++
		}
		if n < r.p.MaxChangesPerWindow {
			r.changes = append(r.changes, r.tick)
			r.Runs = append(r.Runs, bicanh.Run{want, 1})
			return want
		}
	}
	last[1]++
	return last[0]
}

func clamp(v, lo, hi int64) int64 { return min(max(v, lo), hi) }

// decide picks the direction with the best utility: stay clear of every spirit (projected toward the player), drift
// to motes when it is safe, keep away from the walls.
func decide(g *bicanh.Game, s Skill, rng *rand.Rand) int64 {
	if s.Noise > 0 && rng.IntN(1000) < s.Noise {
		return int64(rng.IntN(9))
	}
	p := g.Params()
	px, py := g.Pos()
	w, h := p.Arena.Width*bicanh.Sub, p.Arena.Height*bicanh.Sub
	r := p.Player.Radius * bicanh.Sub
	best, bestU := int64(0), int64(-1<<62)
	for d := int64(0); d <= bicanh.MaxDir; d++ {
		vx, vy := bicanh.DirVec(d, p.Player.Speed)
		x, y := clamp(px+vx*s.Look, r, w-r), clamp(py+vy*s.Look, r, h-r)
		var u int64
		minGap := int64(1 << 40)
		for _, f := range g.Foes() {
			def := p.Enemies[f.Type]
			// The spirit closes in by speed × look toward where we will be.
			dx, dy := x-f.X, y-f.Y
			l := bicanh.StepLen(dx, dy)
			adv := min(l, def.Speed*s.Look)
			gap := l - adv - (def.Radius+p.Player.Radius+s.Margin)*bicanh.Sub
			minGap = min(minGap, gap)
			if gap < 160*bicanh.Sub {
				u -= (160*bicanh.Sub - gap) * (160*bicanh.Sub - gap) / (64 * bicanh.Sub * bicanh.Sub)
			}
		}
		if minGap < 0 {
			u -= 1_000_000 + (-minGap)*100
		}
		// Motes: go for the nearest one within 220 units when nothing is close.
		nearest := int64(1 << 40)
		for _, m := range g.Motes() {
			nearest = min(nearest, bicanh.StepLen(m.X-x, m.Y-y))
		}
		if nearest < 220*bicanh.Sub && minGap > 40*bicanh.Sub {
			u -= nearest / bicanh.Sub * 4
		}
		// Walls and corners trap: prefer the middle.
		cx, cy := x-w/2, y-h/2
		u -= (cx*cx + cy*cy) / (bicanh.Sub * bicanh.Sub * 300)
		if u > bestU {
			best, bestU = d, u
		}
	}
	return best
}

// AutoPlay plays seed with the profile and returns the log and the live result. stopAt > 0 stops early (give up).
func AutoPlay(p *bicanh.Params, seed uint32, s Skill, stopAt int64) ([]bicanh.Run, bicanh.Result) {
	g := bicanh.New(p, seed)
	rec := NewRecorder(p)
	rng := rand.New(rand.NewPCG(uint64(seed), 7))
	want := int64(0)
	for !g.Over() && (stopAt <= 0 || g.Tick() < stopAt) {
		if g.Tick()%s.Think == 0 {
			want = decide(g, s, rng)
		}
		g.Step(rec.Sample(want))
	}
	return rec.Runs, g.Result()
}
