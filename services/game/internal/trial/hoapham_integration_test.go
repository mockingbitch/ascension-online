//go:build integration

package trial_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"tiendao/game/internal/breakthrough"
	"tiendao/game/internal/trial"
	"tiendao/game/internal/trial/hoapham"
	"tiendao/game/internal/trial/hoapham/hoaphamtest"
)

// Hóa Phàm (Phase 3.3) through the same service: the reference player ("nhanh") clears winSeed with the shipped params.

func (e env) hpKind(t *testing.T) *trial.Kind {
	k, ok := e.cb.Trials.Get("hoa_pham")
	if !ok {
		t.Fatal("no hoa_pham")
	}
	return k
}

func (e env) hpSetup(t *testing.T) env {
	e.putAtLast(t, e.charID, "nguyen_anh", 0)
	return e
}

func (e env) hpHonest(t *testing.T, seed uint32) (trial.Claim, time.Duration) {
	t.Helper()
	p := e.hpKind(t).HoaPham()
	strokes, r := hoaphamtest.AutoPlay(p, seed, hoaphamtest.Skills[0], 0)
	if !r.Won {
		t.Fatalf("reference player lost seed %d (%s)", seed, r.Reason)
	}
	return trial.Claim{EndTick: r.EndTick, Lines: r.Cuts, Score: r.Score, StateHash: r.StateHash, Strokes: strokes},
		time.Duration(r.EndTick*p.TickMS) * time.Millisecond
}

func (e env) hpStart(t *testing.T, key string) trial.StartView {
	t.Helper()
	v, err := e.svc.Start(context.Background(), e.user, "hoa_pham", key)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	return v
}

func (e env) hpPass(t *testing.T) trial.SubmitView {
	t.Helper()
	run := e.hpStart(t, "hp-start-pass-01")
	claim, dur := e.hpHonest(t, run.Seed)
	e.advance(dur + 3*time.Second)
	v, err := e.svc.Submit(context.Background(), e.user, run.RunID, "hp-submit-pass-1", claim)
	if err != nil || v.Verdict != trial.VerdictPass {
		t.Fatalf("pass: %+v %v", v, err)
	}
	return v
}

// The full loop: Kim Đan's tâm ma does not apply; Nguyên Anh Viên Mãn needs hóa phàm → pass → failed breakthrough
// keeps it → success consumes it.
func TestHoaPhamGatesHoaThan(t *testing.T) {
	e := setup(t).hpSetup(t)
	ctx := context.Background()
	last := e.lastOf(t, "nguyen_anh")
	e.putAtLast(t, e.charID, "nguyen_anh", last.RequiredMilli)
	bt := &breakthrough.Service{Pool: e.pool, Cultivation: e.cult, RNG: fixedRNG(9999), Trials: e.cb.Trials}
	pv, err := bt.Preview(ctx, e.user)
	if err != nil || pv.Ready || pv.Trial == nil || pv.Trial.Kind != "hoa_pham" || len(pv.Missing) != 1 || pv.Missing[0].Kind != "trial" ||
		pv.Missing[0].TrialKind != "hoa_pham" {
		t.Fatalf("preview without pass: %+v %v", pv, err)
	}
	// Tâm ma is not startable here, hóa phàm is.
	if _, err := e.svc.Start(ctx, e.user, "tam_ma", "hp-tamma-000001"); blockedCode(err) != trial.BlockNotEligible {
		t.Fatalf("tâm ma in Nguyên Anh: %v", err)
	}
	st, _ := e.svc.Status(ctx, e.user)
	if st.Kinds[1].Kind != "hoa_pham" || st.Kinds[1].State != trial.StateAvailable || st.Kinds[1].FromRealmID != "nguyen_anh" || st.Kinds[1].ToRealmID != "hoa_than" {
		t.Fatalf("status %+v", st.Kinds[1])
	}
	sv := e.hpPass(t)
	if sv.VIPPoints != 2 || sv.Outcome == nil || !sv.Outcome.Won || sv.Outcome.Strokes == 0 || sv.Outcome.MaxStep == 0 {
		t.Fatalf("pass view %+v / %+v", sv, sv.Outcome)
	}
	if _, err := e.svc.Start(ctx, e.user, "hoa_pham", "hp-start-again-1"); blockedCode(err) != trial.BlockPassActive {
		t.Fatalf("start with pass: %v", err)
	}
	if r, err := bt.Attempt(ctx, e.user, "hp-bt-fail-00001", nil); err != nil || r.Success {
		t.Fatalf("failed attempt %+v %v", r, err)
	}
	if n := e.scalar(t, "SELECT count(*) FROM trial_runs WHERE character_id=$1 AND kind='hoa_pham' AND verdict='pass' AND consumed_at IS NULL", e.charID); n != 1 {
		t.Fatalf("pass consumed by a failure: %d", n)
	}
	e.advance(4 * time.Hour)
	e.putAtLast(t, e.charID, "nguyen_anh", last.RequiredMilli)
	bt.RNG = fixedRNG(0)
	r, err := bt.Attempt(ctx, e.user, "hp-bt-success-01", nil)
	if err != nil || !r.Success || r.TrialConsumed != "hoa_pham" {
		t.Fatalf("success %+v %v", r, err)
	}
	if _, err := e.svc.Start(ctx, e.user, "hoa_pham", "hp-start-after-1"); blockedCode(err) != trial.BlockNotEligible {
		t.Fatalf("start after the gate: %v", err)
	}
}

func TestHoaPhamEligibilityAndResume(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	e.putAt(t, e.charID, "nguyen_anh.so_ky_1", 0)
	if _, err := e.svc.Start(ctx, e.user, "hoa_pham", "hp-early-000001"); blockedCode(err) != trial.BlockNotEligible {
		t.Fatalf("Sơ Kỳ: %v", err)
	}
	e.putAtLast(t, e.charID, "nguyen_anh", 0)
	a := e.hpStart(t, "hp-first-000001")
	b := e.hpStart(t, "hp-second-00001")
	if a.Resumed || !b.Resumed || a.RunID != b.RunID || a.Seed != b.Seed || a.Kind != "hoa_pham" {
		t.Fatalf("resume %+v %+v", a, b)
	}
	if _, err := e.svc.Start(ctx, e.user, "tam_ma", "hp-first-000001"); !errors.Is(err, trial.ErrKeyReused) {
		t.Fatalf("key reused across kinds: %v", err)
	}
}

func TestHoaPhamResubmitOldRun(t *testing.T) {
	e := setup(t).hpSetup(t)
	ctx := context.Background()
	first := e.hpPass(t)
	claim, _ := e.hpHonest(t, winSeed)
	again, err := e.svc.Submit(ctx, e.user, first.RunID, "hp-submit-pass-1", claim)
	if err != nil || !again.Replayed || again.Verdict != trial.VerdictPass || again.VIPPoints != 0 {
		t.Fatalf("same-key replay %+v %v", again, err)
	}
	if _, err := e.svc.Submit(ctx, e.user, first.RunID, "hp-submit-other1", claim); !errors.Is(err, trial.ErrAlreadySubmitted) {
		t.Fatalf("other key: %v", err)
	}
	if n := e.scalar(t, "SELECT count(*) FROM vip_point_ledger WHERE user_id=$1 AND activity_id='trial_pass'", e.user); n != 1 {
		t.Fatalf("điểm duyên rows %d", n)
	}
}

func TestHoaPhamEditedScoreThenCooldown(t *testing.T) {
	e := setup(t).hpSetup(t)
	ctx := context.Background()
	run := e.hpStart(t, "hp-start-edit-01")
	claim, dur := e.hpHonest(t, run.Seed)
	claim.Score += 10
	e.advance(dur + 2*time.Second)
	v, err := e.svc.Submit(ctx, e.user, run.RunID, "hp-submit-edit-1", claim)
	if err != nil || v.Verdict != trial.VerdictRejected || v.Reason != trial.ReasonMismatch || v.Outcome != nil || v.Status.State != trial.StateCooldown {
		t.Fatalf("edited score %+v %v", v, err)
	}
	if _, err := e.svc.Submit(ctx, e.user, run.RunID, "hp-submit-edit-2", claim); !errors.Is(err, trial.ErrAlreadySubmitted) {
		t.Fatalf("resubmit during cooldown: %v", err)
	}
	if _, err := e.svc.Start(ctx, e.user, "hoa_pham", "hp-start-edit-02"); blockedCode(err) != trial.BlockCooldown {
		t.Fatalf("start during cooldown: %v", err)
	}
	e.advance(30 * time.Minute)
	if next := e.hpStart(t, "hp-start-edit-03"); next.Resumed || next.RunID == run.RunID {
		t.Fatalf("after cooldown %+v", next)
	}
}

func TestHoaPhamShortenedTimeAndExpired(t *testing.T) {
	e := setup(t).hpSetup(t)
	ctx := context.Background()
	run := e.hpStart(t, "hp-start-fast-01")
	claim, dur := e.hpHonest(t, run.Seed)
	e.advance(dur / 2)
	if v, err := e.svc.Submit(ctx, e.user, run.RunID, "hp-submit-fast-1", claim); err != nil || v.Verdict != trial.VerdictRejected || v.Reason != trial.ReasonTooFast {
		t.Fatalf("too fast %+v %v", v, err)
	}
	e.advance(time.Hour)
	run2 := e.hpStart(t, "hp-start-late-01")
	e.advance(16 * time.Minute)
	if v, err := e.svc.Submit(ctx, e.user, run2.RunID, "hp-submit-late-1", claim); err != nil || v.Verdict != trial.VerdictExpired || v.Reason != trial.ReasonLate {
		t.Fatalf("late %+v %v", v, err)
	}
}

func TestHoaPhamOtherCharactersRun(t *testing.T) {
	e := setup(t).hpSetup(t)
	run := e.hpStart(t, "hp-start-mine-01")
	claim, dur := e.hpHonest(t, run.Seed)
	e.advance(dur + time.Second)
	thief, thiefChar := e.newCharacter(t, "Kẻ Cắp")
	e.putAtLast(t, thiefChar, "nguyen_anh", 0)
	if _, err := e.svc.Submit(context.Background(), thief, run.RunID, "hp-submit-thief", claim); !errors.Is(err, trial.ErrRunNotFound) {
		t.Fatalf("thief: %v", err)
	}
	if v, err := e.svc.Submit(context.Background(), e.user, run.RunID, "hp-submit-mine-1", claim); err != nil || v.Verdict != trial.VerdictPass {
		t.Fatalf("owner %+v %v", v, err)
	}
}

func TestHoaPhamConcurrentSubmits(t *testing.T) {
	e := setup(t).hpSetup(t)
	run := e.hpStart(t, "hp-start-race-01")
	claim, dur := e.hpHonest(t, run.Seed)
	e.advance(dur + time.Second)
	const n = 8
	var wg sync.WaitGroup
	errs := make([]error, n)
	verdicts := make([]string, n)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := e.svc.Submit(context.Background(), e.user, run.RunID, "hp-submit-race-"+string(rune('a'+i)), claim)
			errs[i], verdicts[i] = err, v.Verdict
		}()
	}
	wg.Wait()
	ok := 0
	for i, err := range errs {
		switch {
		case err == nil && verdicts[i] == trial.VerdictPass:
			ok++
		case errors.Is(err, trial.ErrAlreadySubmitted):
		default:
			t.Fatalf("submit %d: %v", i, err)
		}
	}
	if ok != 1 {
		t.Fatalf("%d submissions judged", ok)
	}
	if c := e.scalar(t, "SELECT count(*) FROM vip_point_ledger WHERE user_id=$1 AND activity_id='trial_pass'", e.user); c != 1 {
		t.Fatalf("điểm duyên rows %d", c)
	}
}

// Pointer teleport / superhuman speed / oversized log: rejected, cooldown, telemetry kept server-side only.
func TestHoaPhamInhumanLogs(t *testing.T) {
	e := setup(t).hpSetup(t)
	ctx := context.Background()
	p := e.hpKind(t).HoaPham()
	cases := []struct {
		name   string
		mut    func(c *trial.Claim)
		reason string
		col    string
	}{
		{"teleport", func(c *trial.Claim) { c.Strokes[0] = hoapham.Stroke{c.Strokes[0][0], 1, 1, p.Width - 2, 0} }, trial.ReasonInputRate, "maxStep"},
		{"superhuman", func(c *trial.Claim) {
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
		}, trial.ReasonInputRate, "maxWindowTravel"},
		{"oversized", func(c *trial.Claim) {
			c.Strokes = nil
			for i := range p.MaxStrokes + 1 {
				c.Strokes = append(c.Strokes, hoapham.Stroke{int64(i) * 3, 1, 1})
			}
		}, trial.ReasonInvalidLog, ""},
	}
	for i, c := range cases {
		run := e.hpStart(t, "hp-start-bad-0"+string(rune('a'+i)))
		claim, dur := e.hpHonest(t, run.Seed)
		c.mut(&claim)
		e.advance(dur + 2*time.Second)
		v, err := e.svc.Submit(ctx, e.user, run.RunID, "hp-submit-bad-0"+string(rune('a'+i)), claim)
		if err != nil || v.Verdict != trial.VerdictRejected || v.Reason != c.reason || v.Outcome != nil || v.CooldownUntil == nil {
			t.Fatalf("%s: %+v %v", c.name, v, err)
		}
		if c.col != "" {
			if got := e.scalar(t, "SELECT (result->>'"+c.col+"')::bigint FROM trial_runs WHERE id=$1", run.RunID); got == 0 {
				t.Fatalf("%s: telemetry %s missing", c.name, c.col)
			}
		}
		e.advance(31 * time.Minute)
	}
}
