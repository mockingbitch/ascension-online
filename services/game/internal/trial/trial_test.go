package trial

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tiendao/game/internal/trial/hoapham"
	"tiendao/game/internal/trial/tamma"
	"tiendao/game/internal/trial/tamma/tammatest"
)

func shipped(t *testing.T) *Catalogue {
	t.Helper()
	c, err := LoadFile(filepath.Join("..", "..", "..", "..", "content", "catalog", "trials.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestShippedTrialsAreValid(t *testing.T) {
	c := shipped(t)
	k, ok := c.Get("tam_ma")
	if !ok || k.TamMa() == nil || len(k.ParamsJSON()) == 0 {
		t.Fatal("tam_ma missing")
	}
	if g, ok := c.GateFor(3); !ok || g.ID != "tam_ma" {
		t.Fatal("tam_ma must gate the breakthrough leaving tier 3 (Kim Đan → Nguyên Anh)")
	}
	if k.LoseCooldownSeconds != 1800 {
		t.Errorf("lose cooldown %d, checklist proposes 30 min", k.LoseCooldownSeconds)
	}
}

func TestRejectsBadTrials(t *testing.T) {
	_, err := Load(strings.NewReader(`
version: 1
kinds:
  - id: Bad
    name: x
    summary: x
    engine: tam_ma
    gate: { tier: 3 }
    opens_at: last_stage
    expire_after_seconds: 900
    min_elapsed_bp: 9000
    lose_cooldown_seconds: 1800
    pass_valid_seconds: 604800
    params: {}
  - id: a
    name: ""
    summary: x
    engine: snake
    gate: { tier: 0 }
    opens_at: sometime
    expire_after_seconds: 10
    min_elapsed_bp: 100
    lose_cooldown_seconds: 1
    pass_valid_seconds: 1
    params: {}
  - id: b
    name: B
    summary: x
    engine: tam_ma
    gate: { tier: 4 }
    opens_at: realm
    expire_after_seconds: 120
    min_elapsed_bp: 9000
    lose_cooldown_seconds: 1800
    pass_valid_seconds: 604800
    params: { rules_version: 1, tick_ms: 50, width: 9, height: 16, time_limit_ticks: 3600, pass_lines: 12, typo: 1 }
`))
	if err == nil {
		t.Fatal("expected error")
	}
	for _, want := range []string{"invalid id", "empty name", "unknown engine", "gate.tier", "opens_at", "expire_after_seconds",
		"min_elapsed_bp", "lose_cooldown_seconds", "pass_valid_seconds", "typo"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("missing %q in %v", want, err)
		}
	}
}

// fixture: a shipped tâm ma run issued at t0 and a winning claim from the reference player.
func fixture(t *testing.T, seed uint32) (*Kind, Run, Claim, time.Time) {
	t.Helper()
	k, _ := shipped(t).Get("tam_ma")
	t0 := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	in, r := tammatest.AutoPlay(k.TamMa(), seed, 3, 0)
	if !r.Won {
		t.Fatalf("seed %d: reference player lost (%s); pick another seed", seed, r.Reason)
	}
	c := Claim{EndTick: r.EndTick, Lines: r.Lines, Score: r.Score, StateHash: r.StateHash}
	for _, x := range in {
		c.Inputs = append(c.Inputs, [2]int64{x.Tick, int64(x.Action)})
	}
	run := Run{Seed: seed, Params: k.ParamsJSON(), IssuedAt: t0, ExpiresAt: t0.Add(time.Duration(k.ExpireAfterSeconds) * time.Second)}
	return k, run, c, t0.Add(time.Duration(r.EndTick*k.TamMa().TickMS)*time.Millisecond + 2*time.Second)
}

func TestJudge(t *testing.T) {
	k, run, good, at := fixture(t, 20261107)
	clone := func(f func(c *Claim)) Claim {
		c := good
		c.Inputs = append([][2]int64(nil), good.Inputs...)
		f(&c)
		return c
	}
	cases := []struct {
		name    string
		claim   Claim
		now     time.Time
		verdict string
		reason  string
	}{
		{"honest pass", good, at, VerdictPass, tamma.ReasonCleared},
		{"edited score", clone(func(c *Claim) { c.Score += 100 }), at, VerdictRejected, ReasonMismatch},
		{"implausible score", clone(func(c *Claim) { c.Score = 1_000_000 }), at, VerdictRejected, ReasonImplausible},
		{"edited lines", clone(func(c *Claim) { c.Lines-- }), at, VerdictRejected, ReasonMismatch},
		{"edited hash", clone(func(c *Claim) { c.StateHash ^= 1 }), at, VerdictRejected, ReasonMismatch},
		{"claimed later end", clone(func(c *Claim) { c.EndTick += 50 }), at.Add(5 * time.Second), VerdictRejected, ReasonMismatch},
		{"dropped inputs", clone(func(c *Claim) { c.Inputs = c.Inputs[:len(c.Inputs)/2] }), at, VerdictRejected, ReasonMismatch},
		{"shortened time", good, run.IssuedAt.Add(time.Duration(good.EndTick*k.TamMa().TickMS/2) * time.Millisecond), VerdictRejected, ReasonTooFast},
		{"expired", good, run.ExpiresAt, VerdictExpired, ReasonLate},
		{"unknown action", clone(func(c *Claim) { c.Inputs[0][1] = 42 }), at, VerdictRejected, ReasonInvalidLog},
		// L2: a hóa phàm stroke log sent to a tâm ma run.
		{"hóa phàm strokes", clone(func(c *Claim) { c.Strokes = []hoapham.Stroke{{0, 1, 1}} }), at, VerdictRejected, ReasonInvalidLog},
		{"out of order", clone(func(c *Claim) { c.Inputs[1][0] = c.Inputs[0][0] - 1 }), at, VerdictRejected, ReasonInvalidLog},
		{"zero end", clone(func(c *Claim) { c.EndTick = 0 }), at, VerdictRejected, ReasonInvalidLog},
		{"huge lines", clone(func(c *Claim) { c.Lines = 1 << 61; c.Score = 0 }), at, VerdictRejected, ReasonImplausible},
		{"negative lines", clone(func(c *Claim) { c.Lines = -1 }), at, VerdictRejected, ReasonImplausible},
		{"bot burst", clone(func(c *Claim) {
			// 17 inputs inside one 20-tick window (a solver or macro), appended before the first input.
			var burst [][2]int64
			for i := range 17 {
				burst = append(burst, [2]int64{int64(i), 1})
			}
			first := c.Inputs[0][0]
			for i := range burst {
				burst[i][0] = min(burst[i][0], first)
			}
			c.Inputs = append(burst, c.Inputs...)
		}), at, VerdictRejected, ReasonInputRate},
	}
	for _, c := range cases {
		j := Judge(k, run, c.claim, c.now)
		if j.Verdict != c.verdict || j.Reason != c.reason {
			t.Errorf("%s: got %s/%s, want %s/%s", c.name, j.Verdict, j.Reason, c.verdict, c.reason)
		}
	}
	// Another seed (another run's seed) with the same log never verifies.
	other := run
	other.Seed++
	if j := Judge(k, other, good, at); j.Verdict != VerdictRejected {
		t.Errorf("log replayed on another seed: %s/%s", j.Verdict, j.Reason)
	}
}

func TestJudgeHonestLossAndGiveUp(t *testing.T) {
	k, run, _, _ := fixture(t, 20261107)
	p := k.TamMa()
	// One drop then nothing: tops out; an honest claim of that end is a plain fail.
	in, r := tammatest.AutoPlay(p, run.Seed, 1<<40, 0) // gap so large the player acts once
	c := Claim{EndTick: r.EndTick, Lines: r.Lines, Score: r.Score, StateHash: r.StateHash}
	for _, x := range in {
		c.Inputs = append(c.Inputs, [2]int64{x.Tick, int64(x.Action)})
	}
	now := run.IssuedAt.Add(time.Duration(r.EndTick*p.TickMS)*time.Millisecond + time.Second)
	if j := Judge(k, run, c, now); j.Verdict != VerdictFail || j.Reason != tamma.ReasonToppedOut {
		t.Fatalf("idle: %s/%s", j.Verdict, j.Reason)
	}
	in, r = tammatest.AutoPlay(p, run.Seed, 4, 120)
	c = Claim{EndTick: r.EndTick, Lines: r.Lines, Score: r.Score, StateHash: r.StateHash}
	for _, x := range in {
		c.Inputs = append(c.Inputs, [2]int64{x.Tick, int64(x.Action)})
	}
	now = run.IssuedAt.Add(time.Duration(r.EndTick*p.TickMS)*time.Millisecond + time.Second)
	if j := Judge(k, run, c, now); j.Verdict != VerdictFail || j.Reason != tamma.ReasonGaveUp {
		t.Fatalf("give up: %s/%s", j.Verdict, j.Reason)
	}
}
