package bicanh_test

import (
	"encoding/json"
	"flag"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"testing"

	"tiendao/game/internal/secretrealm/bicanh"
	"tiendao/game/internal/secretrealm/bicanh/bicanhtest"
)

var update = flag.Bool("update", false, "rewrite the shared golden vector packages/contracts/golden/bi-canh-v1.json")

const goldenPath = "../../../../../packages/contracts/golden/bi-canh-v1.json"

// Difficulty sanity (⚠ beta tuning): the reference player at three human paces survives most runs but not all, every
// replay equals live play, and the score spread separates the loot bands (secret_realms.yaml).
func TestReferencePlayerDistribution(t *testing.T) {
	if testing.Short() {
		t.Skip("slow")
	}
	p := bicanhtest.Params()
	const n = 40
	for _, s := range bicanhtest.Skills {
		survived := 0
		scores := make([]int64, 0, n)
		var falls int64
		for i := range uint32(n) {
			seed := i*2654435761 + 1
			runs, r := bicanhtest.AutoPlay(p, seed, s, 0)
			if r.Won {
				survived++
			} else {
				falls += r.EndTick
			}
			scores = append(scores, r.Score)
			rr, err := bicanh.Replay(p, seed, runs, r.EndTick)
			if err != nil || rr.StateHash != r.StateHash || rr.Score != r.Score || rr.Reason != r.Reason {
				t.Fatalf("%s seed %d: replay differs from live play: %+v vs %+v (%v)", s.Name, seed, rr, r, err)
			}
			if rr.MaxWindowChanges > p.MaxChangesPerWindow {
				t.Fatalf("recorder produced an implausible log")
			}
		}
		sort.Slice(scores, func(a, b int) bool { return scores[a] < scores[b] })
		fallAt := int64(0)
		if survived < n {
			fallAt = falls * p.TickMS / 1000 / int64(n-survived)
		}
		t.Logf("%s (think %d, look %d, margin %d, slip %d‰): survived %d/%d, score P10 %d · P50 %d · P90 %d (min %d, max %d), mean fall at %d s",
			s.Name, s.Think, s.Look, s.Margin, s.Noise, survived, n, scores[n/10], scores[n/2], scores[n*9/10], scores[0], scores[n-1], fallAt)
		if survived < n*4/10 {
			t.Errorf("%s: only %d/%d survived — too hard for a decent player?", s.Name, survived, n)
		}
		if survived == n && s.Name == "chậm" {
			t.Errorf("%s: every run survived — the realm tests nothing?", s.Name)
		}
	}
}

// Property: same seed + same moves ⇒ same result; another seed or a changed move changes the hash.
func TestReplayDeterministicAndTamperEvident(t *testing.T) {
	p := bicanhtest.Params()
	rng := rand.New(rand.NewPCG(3, 4))
	for range 6 {
		seed := rng.Uint32()
		runs, r := bicanhtest.AutoPlay(p, seed, bicanhtest.Skills[rng.IntN(len(bicanhtest.Skills))], 0)
		a, _ := bicanh.Replay(p, seed, runs, r.EndTick)
		b, _ := bicanh.Replay(p, seed, slices.Clone(runs), r.EndTick)
		if a != b || a.StateHash != r.StateHash {
			t.Fatalf("non-deterministic replay: %+v vs %+v vs %+v", a, b, r)
		}
		if c, _ := bicanh.Replay(p, seed^1, runs, r.EndTick); c.StateHash == a.StateHash {
			t.Fatalf("seed change kept the hash")
		}
		mod := slices.Clone(runs)
		i := len(mod) / 2
		mod[i][0] = (mod[i][0] + 4) % 9
		if (i > 0 && mod[i-1][0] == mod[i][0]) || (i+1 < len(mod) && mod[i+1][0] == mod[i][0]) {
			mod[i][0] = (mod[i][0] + 1) % 9
		}
		if c, err := bicanh.Replay(p, seed, mod, r.EndTick); err == nil && c.StateHash == a.StateHash {
			t.Fatalf("changing a run kept the hash")
		}
	}
}

func TestLogShapeAndPlausibility(t *testing.T) {
	p := bicanhtest.Params()
	cases := []struct {
		name string
		runs []bicanh.Run
		end  int64
		want error
	}{
		{"empty", nil, 10, bicanh.ErrInvalidLog},
		{"short", []bicanh.Run{{1, 5}}, 10, bicanh.ErrInvalidLog},
		{"long", []bicanh.Run{{1, 11}}, 10, bicanh.ErrInvalidLog},
		{"bad dir", []bicanh.Run{{9, 10}}, 10, bicanh.ErrInvalidLog},
		{"neg dir", []bicanh.Run{{-1, 10}}, 10, bicanh.ErrInvalidLog},
		{"zero run", []bicanh.Run{{1, 0}, {2, 10}}, 10, bicanh.ErrInvalidLog},
		{"repeat", []bicanh.Run{{1, 5}, {1, 5}}, 10, bicanh.ErrInvalidLog},
		{"huge", []bicanh.Run{{1, 1 << 62}, {2, 1 << 62}}, 10, bicanh.ErrInvalidLog},
		{"end beyond limit", []bicanh.Run{{0, p.TimeLimitTicks + 1}}, p.TimeLimitTicks + 1, bicanh.ErrInvalidLog},
		{"ok", []bicanh.Run{{1, 5}, {0, 5}}, 10, nil},
	}
	for _, c := range cases {
		if _, err := bicanh.Replay(p, 1, c.runs, c.end); (c.want == nil) != (err == nil) || (c.want != nil && !errorsIs(err, c.want)) {
			t.Errorf("%s: err %v, want %v", c.name, err, c.want)
		}
	}
	// A jitter of one-tick runs: 20 changes in a 1 s window is beyond human cadence.
	var jitter []bicanh.Run
	for i := range 40 {
		jitter = append(jitter, bicanh.Run{int64(1 + i%2), 1})
	}
	if _, err := bicanh.Replay(p, 1, jitter, 40); !errorsIs(err, bicanh.ErrImplausibleInput) {
		t.Fatalf("jitter: %v", err)
	}
	// Exactly the cap (8 changes per 20 ticks) is fine.
	var ok []bicanh.Run
	for i := range 30 {
		ok = append(ok, bicanh.Run{int64(1 + i%2), 3})
	}
	if r, err := bicanh.Replay(p, 1, ok, 90); err != nil || r.MaxWindowChanges > p.MaxChangesPerWindow {
		t.Fatalf("at cap: %+v %v", r, err)
	}
}

func TestRecorderDefersFrantic(t *testing.T) {
	p := bicanhtest.Params()
	rec := bicanhtest.NewRecorder(p)
	for i := range 400 {
		rec.Sample(int64(i % 9)) // a new direction every tick
	}
	if _, err := bicanh.Replay(p, 5, rec.Runs, 400); err != nil {
		t.Fatalf("recorder log rejected: %v", err)
	}
}

func TestIdleFallsAndScoreBound(t *testing.T) {
	p := bicanhtest.Params()
	r, err := bicanh.Replay(p, 42, []bicanh.Run{{0, p.TimeLimitTicks}}, p.TimeLimitTicks)
	if err != nil {
		t.Fatal(err)
	}
	if r.Reason != bicanh.ReasonEndedEarly || r.Won {
		t.Fatalf("standing still for the whole run should fall before the end: %+v", r)
	}
	for _, s := range bicanhtest.Skills {
		_, r := bicanhtest.AutoPlay(p, 9, s, 0)
		if r.Score > p.MaxScore(r.Kills) || r.Qi > r.Score {
			t.Fatalf("score %d above bound %d for %d kills", r.Score, p.MaxScore(r.Kills), r.Kills)
		}
	}
}

// ───── shared golden vector (Go ⇄ TS) ─────

type goldenCase struct {
	Name      string       `json:"name"`
	Seed      uint32       `json:"seed"`
	EndTick   int64        `json:"endTick"`
	Runs      []bicanh.Run `json:"runs"`
	Reason    string       `json:"reason"`
	Won       bool         `json:"won"`
	Kills     int64        `json:"kills"`
	Score     int64        `json:"score"`
	Qi        int64        `json:"qi"`
	Level     int64        `json:"level"`
	HPLeft    int64        `json:"hpLeft"`
	Spawned   int64        `json:"spawned"`
	StateHash uint32       `json:"stateHash"`
}

type goldenFile struct {
	Comment string        `json:"_comment"`
	Params  bicanh.Params `json:"params"`
	Cases   []goldenCase  `json:"cases"`
}

func buildGolden(t *testing.T) goldenFile {
	p := bicanhtest.Params()
	gf := goldenFile{Comment: "Bí Cảnh rules v1 golden vectors: Go (internal/secretrealm/bicanh) and TS (lib/minigames/bi-canh) must both reproduce every case. Regenerate only with a rules_version bump: go test ./internal/secretrealm/bicanh -run Golden -update",
		Params: *p}
	add := func(name string, seed uint32, runs []bicanh.Run, end int64) {
		r, err := bicanh.Replay(p, seed, runs, end)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		gf.Cases = append(gf.Cases, goldenCase{Name: name, Seed: seed, EndTick: end, Runs: runs, Reason: r.Reason, Won: r.Won, Kills: r.Kills,
			Score: r.Score, Qi: r.Qi, Level: r.Level, HPLeft: r.HPLeft, Spawned: r.Spawned, StateHash: r.StateHash})
	}
	runs, r := bicanhtest.AutoPlay(p, 20261107, bicanhtest.Skills[0], 0)
	add("bot_fast", 20261107, runs, r.EndTick)
	runs, r = bicanhtest.AutoPlay(p, 3141592653, bicanhtest.Skills[2], 0)
	add("bot_slow", 3141592653, runs, r.EndTick)
	runs, _ = bicanhtest.AutoPlay(p, 777, bicanhtest.Skills[1], 400)
	add("give_up_400", 777, runs, 400)
	idle, _ := bicanh.Replay(p, 1, []bicanh.Run{{0, p.TimeLimitTicks}}, p.TimeLimitTicks)
	add("idle_falls", 1, []bicanh.Run{{0, idle.EndTick}}, idle.EndTick)
	// Hand-written square walk: every direction, corners and walls.
	var sq []bicanh.Run
	var tot int64
	for i := 0; tot < 1200; i++ {
		d := int64(1 + i%8)
		sq = append(sq, bicanh.Run{d, 25})
		tot += 25
	}
	rs, _ := bicanh.Replay(p, 424242, sq, tot)
	if rs.EndTick < tot { // fell: trim the walk at the end tick
		var cut []bicanh.Run
		var acc int64
		for _, rr := range sq {
			if acc+rr[1] >= rs.EndTick {
				cut = append(cut, bicanh.Run{rr[0], rs.EndTick - acc})
				break
			}
			cut = append(cut, rr)
			acc += rr[1]
		}
		sq, tot = cut, rs.EndTick
	}
	add("square_walk", 424242, sq, tot)
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
		r, err := bicanh.Replay(&gf.Params, c.Seed, c.Runs, c.EndTick)
		if err != nil {
			t.Fatalf("%s: %v", c.Name, err)
		}
		if r.StateHash != c.StateHash || r.Kills != c.Kills || r.Score != c.Score || r.Reason != c.Reason || r.Won != c.Won || r.Qi != c.Qi ||
			r.Level != c.Level || r.HPLeft != c.HPLeft || r.Spawned != c.Spawned {
			t.Errorf("%s: got %+v, want %+v", c.Name, r, c)
		}
	}
}
