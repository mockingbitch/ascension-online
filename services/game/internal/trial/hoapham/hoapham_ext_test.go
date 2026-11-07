package hoapham_test

import (
	"encoding/json"
	"flag"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"tiendao/game/internal/trial/hoapham"
	"tiendao/game/internal/trial/hoapham/hoaphamtest"
)

var update = flag.Bool("update", false, "rewrite the shared golden vector packages/contracts/golden/hoa-pham-v1.json")

const goldenPath = "../../../../../packages/contracts/golden/hoa-pham-v1.json"

// Difficulty sanity (⚠ beta tuning; target ~80–90% for a decent human): the reference player at human paces passes
// most runs, every replay equals live play, and no pass comes before min_pass_ticks.
func TestReferencePlayerPassRates(t *testing.T) {
	if testing.Short() {
		t.Skip("slow")
	}
	p := hoaphamtest.Params()
	const n = 40
	for _, s := range hoaphamtest.Skills {
		passed := 0
		var ticks, score, lanterns int64
		for i := range uint32(n) {
			seed := i*2654435761 + 1
			strokes, r := hoaphamtest.AutoPlay(p, seed, s, 0)
			if r.Won {
				passed++
				ticks += r.EndTick
				if r.EndTick < p.MinPassTicks {
					t.Fatalf("seed %d: pass at tick %d < min_pass_ticks", seed, r.EndTick)
				}
			}
			score += r.Score
			lanterns += r.LanternsCut
			rr, err := hoapham.Replay(p, seed, strokes, r.EndTick)
			if err != nil || rr != r {
				t.Fatalf("%s seed %d: replay differs from live play: %+v vs %+v (%v)", s.Name, seed, rr, r, err)
			}
		}
		t.Logf("%s (react %d, speed %d, think %d, aim ±%d): passed %d/%d, mean pass at %d s, mean score %d, lanterns cut/run %.2f",
			s.Name, s.React, s.Speed, s.Think, s.AimErr, passed, n, ticks*p.TickMS/1000/int64(max(passed, 1)), score/n, float64(lanterns)/n)
		if passed < n*7/10 {
			t.Errorf("%s: only %d/%d passed — too hard for a decent player?", s.Name, passed, n)
		}
		if passed == n && s.Name == "chậm" {
			t.Errorf("%s: every run passed — the gate tests nothing?", s.Name)
		}
	}
}

// Property: same seed + same strokes ⇒ same result; another seed or half the strokes changes the hash.
func TestReplayDeterministicAndTamperEvident(t *testing.T) {
	p := hoaphamtest.Params()
	rng := rand.New(rand.NewPCG(1, 2))
	for range 8 {
		seed := rng.Uint32()
		s := hoaphamtest.Skills[rng.IntN(len(hoaphamtest.Skills))]
		strokes, r := hoaphamtest.AutoPlay(p, seed, s, 0)
		a, _ := hoapham.Replay(p, seed, strokes, r.EndTick)
		b, _ := hoapham.Replay(p, seed, slices.Clone(strokes), r.EndTick)
		if a != b || a != r {
			t.Fatalf("non-deterministic replay: %+v vs %+v vs %+v", a, b, r)
		}
		if c, _ := hoapham.Replay(p, seed^1, strokes, r.EndTick); c.StateHash == a.StateHash {
			t.Fatalf("seed change kept the hash")
		}
		drop := slices.Delete(slices.Clone(strokes), 0, len(strokes)/2)
		if c, _ := hoapham.Replay(p, seed, drop, r.EndTick); c.StateHash == a.StateHash {
			t.Fatalf("dropping half the strokes kept the hash")
		}
	}
}

func TestTelemetryWithinCaps(t *testing.T) {
	p := hoaphamtest.Params()
	strokes, r := hoaphamtest.AutoPlay(p, 99, hoaphamtest.Skills[0], 0)
	if r.Strokes != len(strokes) || r.Points < int64(len(strokes)) || r.MaxStep < p.MinCutStep || r.MaxStep > p.MaxStep ||
		r.MaxWindowTravel > p.MaxWindowTravel || r.MaxWindowStrokes > p.MaxStrokesPerWindow || r.MaxWindowStrokes < 1 {
		t.Fatalf("telemetry %+v", r)
	}
	if r.Score > p.MaxScore(r.Cuts) {
		t.Fatalf("score %d above bound %d for %d cuts", r.Score, p.MaxScore(r.Cuts), r.Cuts)
	}
}

// ───── shared golden vector (Go ⇄ TS) ─────

type goldenCase struct {
	Name        string           `json:"name"`
	Seed        uint32           `json:"seed"`
	EndTick     int64            `json:"endTick"`
	Strokes     []hoapham.Stroke `json:"strokes"`
	Reason      string           `json:"reason"`
	Won         bool             `json:"won"`
	Cuts        int64            `json:"cuts"`
	Score       int64            `json:"score"`
	Spawned     int64            `json:"spawned"`
	LanternsCut int64            `json:"lanternsCut"`
	Escaped     int64            `json:"escaped"`
	BestCombo   int64            `json:"bestCombo"`
	StateHash   uint32           `json:"stateHash"`
}

type goldenFile struct {
	Comment string         `json:"_comment"`
	Params  hoapham.Params `json:"params"`
	Cases   []goldenCase   `json:"cases"`
}

func buildGolden(t *testing.T) goldenFile {
	p := hoaphamtest.Params()
	gf := goldenFile{Comment: "Hóa Phàm rules v1 golden vectors: Go (internal/trial/hoapham) and TS (lib/minigames/hoa-pham) must both reproduce every case. Regenerate only with a rules_version bump: go test ./internal/trial/hoapham -run Golden -update",
		Params: *p}
	add := func(name string, seed uint32, strokes []hoapham.Stroke, end int64) {
		r, err := hoapham.Replay(p, seed, strokes, end)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if strokes == nil {
			strokes = []hoapham.Stroke{}
		}
		gf.Cases = append(gf.Cases, goldenCase{Name: name, Seed: seed, EndTick: end, Strokes: strokes, Reason: r.Reason, Won: r.Won,
			Cuts: r.Cuts, Score: r.Score, Spawned: r.Spawned, LanternsCut: r.LanternsCut, Escaped: r.Escaped, BestCombo: r.BestCombo,
			StateHash: r.StateHash})
	}
	in, r := hoaphamtest.AutoPlay(p, 20261107, hoaphamtest.Skills[0], 0)
	add("bot_fast", 20261107, in, r.EndTick)
	in, r = hoaphamtest.AutoPlay(p, 3141592653, hoaphamtest.Skills[2], 0)
	add("bot_slow", 3141592653, in, r.EndTick)
	in, _ = hoaphamtest.AutoPlay(p, 777, hoaphamtest.Skills[1], 300)
	add("give_up_300", 777, in, 300)
	add("idle_time_up", 1, nil, p.TimeLimitTicks)
	// Hand-written zig-zag sweeps across the whole field: cuts everything incl. lanterns until they run out.
	var zig []hoapham.Stroke
	for tk := int64(20); tk+30 < p.TimeLimitTicks && len(zig) < 60; tk += 34 {
		s := hoapham.Stroke{tk, 2, 20 + (tk/34)%120}
		for i := range int64(28) {
			dy := int64(4)
			if i%8 >= 4 {
				dy = -4
			}
			s = append(s, 4, dy)
		}
		zig = append(zig, s)
	}
	rz, err := hoapham.Replay(p, 424242, zig, p.TimeLimitTicks)
	if err != nil {
		t.Fatal(err)
	}
	// The game may end (lanterns out) during a sweep: drop later sweeps and cut the last one at the end tick.
	for len(zig) > 0 && zig[len(zig)-1][0] >= rz.EndTick {
		zig = zig[:len(zig)-1]
	}
	if last := zig[len(zig)-1]; last.End() >= rz.EndTick {
		zig[len(zig)-1] = last[:3+2*(rz.EndTick-1-last[0])]
	}
	add("zigzag", 424242, zig, rz.EndTick)
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
		r, err := hoapham.Replay(&gf.Params, c.Seed, c.Strokes, c.EndTick)
		if err != nil {
			t.Fatalf("%s: %v", c.Name, err)
		}
		if r.StateHash != c.StateHash || r.Cuts != c.Cuts || r.Score != c.Score || r.Reason != c.Reason || r.Won != c.Won ||
			r.Spawned != c.Spawned || r.LanternsCut != c.LanternsCut || r.Escaped != c.Escaped || r.BestCombo != c.BestCombo {
			t.Errorf("%s: got %+v, want %+v", c.Name, r, c)
		}
	}
}
