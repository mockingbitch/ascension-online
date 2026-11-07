package trial

import (
	"encoding/json"
	"testing"
	"time"

	"tiendao/game/internal/trial/hoapham"
	"tiendao/game/internal/trial/hoapham/hoaphamtest"
)

func TestShippedHoaPham(t *testing.T) {
	c := shipped(t)
	k, ok := c.Get("hoa_pham")
	if !ok || k.HoaPham() == nil || k.TamMa() != nil || len(k.ParamsJSON()) == 0 {
		t.Fatal("hoa_pham missing")
	}
	if g, ok := c.GateFor(4); !ok || g.ID != "hoa_pham" {
		t.Fatal("hoa_pham must gate the breakthrough leaving tier 4 (Nguyên Anh → Hóa Thần)")
	}
	if k.OpensAt != OpensAtLastStage || k.LoseCooldownSeconds != 1800 || k.PassValidSeconds != 7*86400 {
		t.Errorf("hoa_pham gate settings %+v", k)
	}
	// The catalogue params are the ones the reference player and the golden vector were tuned on.
	if got, want := *k.HoaPham(), *hoaphamtest.Params(); got.PassScore != want.PassScore || got.MinPassTicks != want.MinPassTicks ||
		got.TimeLimitTicks != want.TimeLimitTicks || got.MaxStep != want.MaxStep || len(got.Objects) != len(want.Objects) {
		t.Errorf("trials.yaml hoa_pham drifted from hoaphamtest.Params: %+v vs %+v", got, want)
	}
}

// hpFixture: a shipped hóa phàm run issued at t0 and a winning claim from the reference player.
func hpFixture(t *testing.T, seed uint32) (*Kind, Run, Claim, time.Time) {
	t.Helper()
	k, _ := shipped(t).Get("hoa_pham")
	t0 := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	strokes, r := hoaphamtest.AutoPlay(k.HoaPham(), seed, hoaphamtest.Skills[0], 0)
	if !r.Won {
		t.Fatalf("seed %d: reference player lost (%s); pick another seed", seed, r.Reason)
	}
	c := Claim{EndTick: r.EndTick, Lines: r.Cuts, Score: r.Score, StateHash: r.StateHash, Strokes: strokes}
	run := Run{Seed: seed, Params: k.ParamsJSON(), IssuedAt: t0, ExpiresAt: t0.Add(time.Duration(k.ExpireAfterSeconds) * time.Second)}
	return k, run, c, t0.Add(time.Duration(r.EndTick*k.HoaPham().TickMS)*time.Millisecond + 2*time.Second)
}

func cloneStrokes(in []hoapham.Stroke) []hoapham.Stroke {
	out := make([]hoapham.Stroke, len(in))
	for i, s := range in {
		out[i] = append(hoapham.Stroke(nil), s...)
	}
	return out
}

func TestJudgeHoaPham(t *testing.T) {
	k, run, good, at := hpFixture(t, 20261107)
	p := k.HoaPham()
	clone := func(f func(c *Claim)) Claim {
		c := good
		c.Strokes = cloneStrokes(good.Strokes)
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
		{"honest pass", good, at, VerdictPass, hoapham.ReasonCleared},
		{"edited score", clone(func(c *Claim) { c.Score += 5 }), at, VerdictRejected, ReasonMismatch},
		{"edited cuts", clone(func(c *Claim) { c.Lines++ }), at, VerdictRejected, ReasonMismatch},
		{"edited hash", clone(func(c *Claim) { c.StateHash ^= 1 }), at, VerdictRejected, ReasonMismatch},
		{"implausible score", clone(func(c *Claim) { c.Score = 1_000_000 }), at, VerdictRejected, ReasonImplausible},
		{"negative cuts", clone(func(c *Claim) { c.Lines = -1 }), at, VerdictRejected, ReasonImplausible},
		{"huge cuts", clone(func(c *Claim) { c.Lines = 1 << 50; c.Score = 0 }), at, VerdictRejected, ReasonImplausible},
		{"claimed later end", clone(func(c *Claim) { c.EndTick += 50 }), at.Add(3 * time.Second), VerdictRejected, ReasonMismatch},
		{"dropped strokes", clone(func(c *Claim) { c.Strokes = c.Strokes[:len(c.Strokes)/2] }), at, VerdictRejected, ReasonMismatch},
		{"shortened time", good, run.IssuedAt.Add(time.Duration(good.EndTick*p.TickMS/2) * time.Millisecond), VerdictRejected, ReasonTooFast},
		{"expired", good, run.ExpiresAt, VerdictExpired, ReasonLate},
		{"zero end", clone(func(c *Claim) { c.EndTick = 0 }), at, VerdictRejected, ReasonInvalidLog},
		{"tâm ma inputs", clone(func(c *Claim) { c.Inputs = [][2]int64{{0, 1}} }), at, VerdictRejected, ReasonInvalidLog},
		{"broken stroke", clone(func(c *Claim) { c.Strokes[0] = c.Strokes[0][:2] }), at, VerdictRejected, ReasonInvalidLog},
		{"out of the field", clone(func(c *Claim) { c.Strokes[0][1] = p.Width }), at, VerdictRejected, ReasonInvalidLog},
		{"oversized log", clone(func(c *Claim) {
			for i := range p.MaxStrokes + 1 {
				c.Strokes = append(c.Strokes, hoapham.Stroke{int64(i), 1, 1})
			}
		}), at, VerdictRejected, ReasonInvalidLog},
		{"pointer teleport", clone(func(c *Claim) {
			c.Strokes[0] = hoapham.Stroke{c.Strokes[0][0], 1, 1, p.Width - 2, 0} // one tick across the whole field
		}), at, VerdictRejected, ReasonInputRate},
		{"superhuman speed", clone(func(c *Claim) {
			// A whole window of maximum steps back and forth (no single step too long), replacing the first strokes.
			s := hoapham.Stroke{0, 20, 80}
			for i := range 25 {
				d := p.MaxStep
				if i%2 == 1 {
					d = -d
				}
				s = append(s, d, 0)
			}
			rest := c.Strokes
			for len(rest) > 0 && rest[0][0] <= 28 {
				rest = rest[1:]
			}
			c.Strokes = append([]hoapham.Stroke{s}, rest...)
		}), at, VerdictRejected, ReasonInputRate},
	}
	for _, c := range cases {
		j := Judge(k, run, c.claim, c.now)
		if j.Verdict != c.verdict || j.Reason != c.reason {
			t.Errorf("%s: got %s/%s, want %s/%s", c.name, j.Verdict, j.Reason, c.verdict, c.reason)
		}
		if c.reason == ReasonInputRate && (j.Outcome == nil || (j.Outcome.MaxStep <= p.MaxStep && j.Outcome.MaxWindowTravel <= p.MaxWindowTravel)) {
			t.Errorf("%s: telemetry missing %+v", c.name, j.Outcome)
		}
	}
	other := run
	other.Seed++
	if j := Judge(k, other, good, at); j.Verdict != VerdictRejected {
		t.Errorf("log replayed on another seed: %s/%s", j.Verdict, j.Reason)
	}
	// Telemetry of the honest pass.
	j := Judge(k, run, good, at)
	if o := j.Outcome; o == nil || o.Strokes != len(good.Strokes) || o.Inputs == 0 || o.MaxStep == 0 || o.Lines != good.Lines || o.Pieces == 0 {
		t.Errorf("pass telemetry %+v", j.Outcome)
	}
}

func TestJudgeHoaPhamHonestLoss(t *testing.T) {
	k, run, _, _ := hpFixture(t, 20261107)
	p := k.HoaPham()
	r, _ := hoapham.Replay(p, run.Seed, nil, p.TimeLimitTicks)
	c := Claim{EndTick: r.EndTick, StateHash: r.StateHash}
	now := run.IssuedAt.Add(time.Duration(r.EndTick*p.TickMS)*time.Millisecond + time.Second)
	if j := Judge(k, run, c, now); j.Verdict != VerdictFail || j.Reason != hoapham.ReasonTimeUp {
		t.Fatalf("idle: %s/%s", j.Verdict, j.Reason)
	}
	strokes, r := hoaphamtest.AutoPlay(p, run.Seed, hoaphamtest.Skills[1], 200)
	c = Claim{EndTick: r.EndTick, Lines: r.Cuts, Score: r.Score, StateHash: r.StateHash, Strokes: strokes}
	now = run.IssuedAt.Add(time.Duration(r.EndTick*p.TickMS)*time.Millisecond + time.Second)
	if j := Judge(k, run, c, now); j.Verdict != VerdictFail || j.Reason != hoapham.ReasonGaveUp {
		t.Fatalf("give up: %s/%s", j.Verdict, j.Reason)
	}
}

// M1: a submit long after the game's own length is flagged (not rejected) as possible replay-shopping of the seed.
func TestJudgeHoaPhamFlagsSlowSubmit(t *testing.T) {
	k, run, good, at := hpFixture(t, 20261107)
	j := Judge(k, run, good, at)
	if j.Verdict != VerdictPass || j.Outcome.SlowSubmit || j.Outcome.ElapsedMillis <= 0 {
		t.Fatalf("prompt submit: %s %+v", j.Verdict, j.Outcome)
	}
	j = Judge(k, run, good, at.Add(time.Duration(SlowSubmitSlackMillis)*time.Millisecond))
	if j.Verdict != VerdictPass || !j.Outcome.SlowSubmit {
		t.Fatalf("slow submit: %s %+v", j.Verdict, j.Outcome)
	}
}

// L3: min_elapsed_bp is read from the run's params snapshot; runs without it fall back to the catalogue.
func TestMinElapsedFromSnapshot(t *testing.T) {
	for _, kind := range []string{"tam_ma", "hoa_pham"} {
		k, _ := shipped(t).Get(kind)
		if got := minElapsedBP(k, k.ParamsJSON()); got != k.MinElapsedBP {
			t.Fatalf("%s: snapshot %d, catalogue %d", kind, got, k.MinElapsedBP)
		}
		strict := json.RawMessage(`{"minElapsedBp":10000}`)
		if got := minElapsedBP(k, strict); got != 10000 {
			t.Fatalf("%s: snapshot not used: %d", kind, got)
		}
		for _, old := range []string{`{}`, `{"minElapsedBp":1}`, `not json`} {
			if got := minElapsedBP(k, json.RawMessage(old)); got != k.MinElapsedBP {
				t.Fatalf("%s %s: fallback %d", kind, old, got)
			}
		}
	}
	// A catalogue edit after issue does not change the verdict of a run in flight.
	k, run, good, _ := hpFixture(t, 20261107)
	edited := *k
	edited.MinElapsedBP = 10000
	edge := run.IssuedAt.Add(time.Duration(good.EndTick*k.HoaPham().TickMS*9500/10000) * time.Millisecond)
	if j := Judge(&edited, run, good, edge); j.Verdict != VerdictPass {
		t.Fatalf("snapshot 90%% must accept 95%% elapsed even if the catalogue now says 100%%: %s/%s", j.Verdict, j.Reason)
	}
	var p map[string]any
	_ = json.Unmarshal(run.Params, &p)
	delete(p, "minElapsedBp")
	run.Params, _ = json.Marshal(p)
	if j := Judge(&edited, run, good, edge); j.Verdict != VerdictRejected || j.Reason != ReasonTooFast {
		t.Fatalf("legacy run falls back to the catalogue: %s/%s", j.Verdict, j.Reason)
	}
}
