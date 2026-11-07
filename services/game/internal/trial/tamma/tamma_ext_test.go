package tamma_test

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"tiendao/game/internal/trial/tamma"
	"tiendao/game/internal/trial/tamma/tammatest"
)

var update = flag.Bool("update", false, "rewrite the shared golden vector packages/contracts/golden/tam-ma-v2.json")

const goldenPath = "../../../../../packages/contracts/golden/tam-ma-v2.json"

// Human cadences of the reference player: one input every gap+1 ticks (50 ms each).
var cadences = []struct {
	name string
	gap  int64
}{{"5 inputs/s", 3}, {"3.3 inputs/s", 5}, {"2.2 inputs/s", 8}}

// Difficulty sanity (beta tuning target 70–85% of real players within 3 tries): the reference player, at human
// cadences, passes most runs, and no pass ever comes before min_pass_ticks.
func TestReferencePlayerPassRates(t *testing.T) {
	if testing.Short() {
		t.Skip("slow")
	}
	p := tammatest.Params()
	if v := os.Getenv("TAMMA_MIN_PASS"); v != "" {
		var n int64
		_, _ = fmt.Sscan(v, &n)
		p.MinPassTicks = n
	}
	const n = 40
	for _, c := range cadences {
		passed := 0
		var ticks int64
		for s := range uint32(n) {
			seed := s*2654435761 + 1
			inputs, r := tammatest.AutoPlay(p, seed, c.gap, 0)
			if r.Won {
				passed++
				ticks += r.EndTick
				if r.EndTick < p.MinPassTicks {
					t.Fatalf("seed %d: pass at tick %d < min_pass_ticks", seed, r.EndTick)
				}
			}
			rr, err := tamma.Replay(p, seed, inputs, r.EndTick)
			if err != nil || rr != r {
				t.Fatalf("seed %d: replay differs from live play: %+v vs %+v (%v)", seed, rr, r, err)
			}
		}
		t.Logf("%s: passed %d/%d, mean pass at %d s", c.name, passed, n, ticks*p.TickMS/1000/int64(max(passed, 1)))
		if c.gap <= 5 && passed < n*7/10 {
			t.Errorf("%s: only %d/%d passed — too hard for a decent player?", c.name, passed, n)
		}
	}
}

// Property: same seed + same inputs ⇒ same result; another seed or half the inputs changes the hash.
func TestReplayDeterministicAndTamperEvident(t *testing.T) {
	p := tammatest.Params()
	rng := rand.New(rand.NewPCG(1, 2))
	for range 12 {
		seed := rng.Uint32()
		inputs, r := tammatest.AutoPlay(p, seed, 2+int64(rng.IntN(4)), 0)
		a, _ := tamma.Replay(p, seed, inputs, r.EndTick)
		b, _ := tamma.Replay(p, seed, slices.Clone(inputs), r.EndTick)
		if a != b || a != r {
			t.Fatalf("non-deterministic replay: %+v vs %+v vs %+v", a, b, r)
		}
		if c, _ := tamma.Replay(p, seed^1, inputs, r.EndTick); c.StateHash == a.StateHash {
			t.Fatalf("seed change kept the hash")
		}
		drop := slices.Delete(slices.Clone(inputs), 0, len(inputs)/2)
		if c, _ := tamma.Replay(p, seed, drop, r.EndTick); c.StateHash == a.StateHash {
			t.Fatalf("dropping half the inputs kept the hash")
		}
	}
}

func TestTelemetry(t *testing.T) {
	p := tammatest.Params()
	inputs, r := tammatest.AutoPlay(p, 99, 3, 0)
	if r.Inputs != len(inputs) || r.MaxWindowInputs < 1 || r.MaxWindowInputs > p.MaxInputsPerWindow || r.MinLockGapTicks < p.MinLockGapTicks {
		t.Fatalf("telemetry %+v", r)
	}
}

func TestMaxScoreBoundsRealPlay(t *testing.T) {
	p := tammatest.Params()
	for s := range uint32(6) {
		_, r := tammatest.AutoPlay(p, s+11, 3, 0)
		if r.Score > p.MaxScore(r.Lines) {
			t.Fatalf("score %d above bound %d for %d lines", r.Score, p.MaxScore(r.Lines), r.Lines)
		}
	}
}

// ───── shared golden vector (Go ⇄ TS) ─────

type goldenCase struct {
	Name       string     `json:"name"`
	Seed       uint32     `json:"seed"`
	EndTick    int64      `json:"endTick"`
	Inputs     [][2]int64 `json:"inputs"`
	Reason     string     `json:"reason"`
	Won        bool       `json:"won"`
	Lines      int64      `json:"lines"`
	Score      int64      `json:"score"`
	Pieces     int64      `json:"pieces"`
	StateHash  uint32     `json:"stateHash"`
	MinLockGap int64      `json:"minLockGapTicks"`
}

type goldenFile struct {
	Comment string       `json:"_comment"`
	Params  tamma.Params `json:"params"`
	Cases   []goldenCase `json:"cases"`
}

func toWire(in []tamma.Input) [][2]int64 {
	out := make([][2]int64, len(in))
	for i, x := range in {
		out[i] = [2]int64{x.Tick, int64(x.Action)}
	}
	return out
}

func fromWire(in [][2]int64) []tamma.Input {
	out := make([]tamma.Input, len(in))
	for i, x := range in {
		out[i] = tamma.Input{Tick: x[0], Action: tamma.Action(x[1])}
	}
	return out
}

func buildGolden(t *testing.T) goldenFile {
	p := tammatest.Params()
	gf := goldenFile{Comment: "Tâm Ma rules v2 golden vectors: Go (internal/trial/tamma) and TS (lib/minigames/tam-ma) must both reproduce every case. Regenerate only with a rules_version bump: go test ./internal/trial/tamma -run Golden -update",
		Params: *p}
	add := func(name string, seed uint32, inputs []tamma.Input, end int64) {
		r, err := tamma.Replay(p, seed, inputs, end)
		if err != nil {
			t.Fatal(err)
		}
		gf.Cases = append(gf.Cases, goldenCase{Name: name, Seed: seed, EndTick: end, Inputs: toWire(inputs), Reason: r.Reason, Won: r.Won,
			Lines: r.Lines, Score: r.Score, Pieces: r.Pieces, StateHash: r.StateHash, MinLockGap: r.MinLockGapTicks})
	}
	in, r := tammatest.AutoPlay(p, 20261107, 3, 0)
	add("bot_pass", 20261107, in, r.EndTick)
	in, r = tammatest.AutoPlay(p, 3141592653, 5, 0)
	add("bot_slow", 3141592653, in, r.EndTick)
	in, _ = tammatest.AutoPlay(p, 777, 4, 160)
	add("give_up_160", 777, in, 160)
	g := tamma.New(p, 1)
	for !g.Over() {
		g.Step(nil)
	}
	add("idle_top_out", 1, nil, g.Tick())
	// Hand-written log touching every action incl. calm attempts, wall kicks and drops inside the lock gap.
	var hand []tamma.Input
	for tk := int64(0); tk < 1200; tk += 3 {
		hand = append(hand, tamma.Input{Tick: tk, Action: tamma.Action(tk/3%7 + 1)})
	}
	add("all_actions", 424242, hand, 1200)
	return gf
}

func TestGoldenVector(t *testing.T) {
	path := filepath.Clean(goldenPath)
	if *update {
		b, _ := json.MarshalIndent(buildGolden(t), "", "\t")
		if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var gf goldenFile
	if err := json.Unmarshal(b, &gf); err != nil {
		t.Fatal(err)
	}
	if err := gf.Params.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(gf.Cases) < 5 {
		t.Fatal("golden file lost cases")
	}
	for _, c := range gf.Cases {
		r, err := tamma.Replay(&gf.Params, c.Seed, fromWire(c.Inputs), c.EndTick)
		if err != nil {
			t.Fatalf("%s: %v", c.Name, err)
		}
		if r.StateHash != c.StateHash || r.Lines != c.Lines || r.Score != c.Score || r.Reason != c.Reason || r.Won != c.Won ||
			r.Pieces != c.Pieces || r.MinLockGapTicks != c.MinLockGap {
			t.Errorf("%s: got %+v, want %+v", c.Name, r, c)
		}
	}
}
