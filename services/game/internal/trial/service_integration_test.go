//go:build integration

package trial_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"tiendao/game/internal/breakthrough"
	"tiendao/game/internal/character"
	"tiendao/game/internal/content"
	"tiendao/game/internal/cultivation"
	"tiendao/game/internal/platform/testdb"
	"tiendao/game/internal/realm"
	"tiendao/game/internal/trial"
	"tiendao/game/internal/trial/tamma/tammatest"
)

// winSeed: the reference player (gap 3) clears tâm ma on this seed with the shipped params (see trial_test.go).
const winSeed uint32 = 20261107

type fixedRNG int

func (f fixedRNG) IntN(int) int { return int(f) }

type env struct {
	pool   *pgxpool.Pool
	cb     *content.Bundle
	cult   *cultivation.Service
	chars  *character.Service
	svc    *trial.Service
	user   uuid.UUID
	charID uuid.UUID
	clock  *time.Time
}

func (e env) now() time.Time          { return *e.clock }
func (e env) advance(d time.Duration) { *e.clock = e.clock.Add(d) }

func setup(t *testing.T) env {
	t.Helper()
	pool := testdb.New(t)
	cb, err := content.LoadAll("../../../../content/catalog")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	clock := &now
	nowFn := func() time.Time { return *clock }
	cult := &cultivation.Service{Pool: pool, Realms: cb.Realms, Contribution: cb.Contribution, VIP: cb.VIP, Now: nowFn}
	chars := &character.Service{Pool: pool, Cultivation: cult}
	svc := &trial.Service{Pool: pool, Cat: cb.Trials, Realms: cb.Realms, Project: cult.ProjectedStage, VIP: cb.VIP, Now: nowFn,
		Seed: func() uint32 { return winSeed }}
	e := env{pool: pool, cb: cb, cult: cult, chars: chars, svc: svc, clock: clock}
	e.user, e.charID = e.newCharacter(t, "Lâm Phong")
	e.putAtLast(t, e.charID, "kim_dan", 0)
	return e
}

func (e env) newCharacter(t *testing.T, name string) (uuid.UUID, uuid.UUID) {
	t.Helper()
	user := uuid.New()
	if _, err := e.pool.Exec(context.Background(), "INSERT INTO users (id, email) VALUES ($1, $2)", user, user.String()+"@t"); err != nil {
		t.Fatal(err)
	}
	v, err := e.chars.Create(context.Background(), user, name)
	if err != nil {
		t.Fatal(err)
	}
	return user, v.ID
}

func (e env) lastOf(t *testing.T, realmID string) *realm.Stage {
	for _, s := range e.cb.Realms.Ladder("tien") {
		if s.RealmID == realmID && s.IsLastOfRealm() {
			return s
		}
	}
	t.Fatal("no stage")
	return nil
}

// putAtLast places a character on the last stage of a realm with `cultivation`, no pending idle time.
func (e env) putAtLast(t *testing.T, charID uuid.UUID, realmID string, cultivation int64) {
	t.Helper()
	e.putAt(t, charID, e.lastOf(t, realmID).ID, cultivation)
}

func (e env) putAt(t *testing.T, charID uuid.UUID, stageID string, cultivation int64) {
	t.Helper()
	_, err := e.pool.Exec(context.Background(),
		"UPDATE character_progressions SET stage_id=$2, realm_id=split_part($2,'.',1), cultivation_milli=$3, last_cultivated_at=$4 WHERE character_id=$1",
		charID, stageID, cultivation, e.now())
	if err != nil {
		t.Fatal(err)
	}
}

func (e env) scalar(t *testing.T, sql string, args ...any) (v int64) {
	t.Helper()
	if err := e.pool.QueryRow(context.Background(), sql, args...).Scan(&v); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return
}

func (e env) kind(t *testing.T) *trial.Kind {
	k, ok := e.cb.Trials.Get("tam_ma")
	if !ok {
		t.Fatal("no tam_ma")
	}
	return k
}

// honest plays the run's seed with the reference player and returns the claim plus how long the game takes.
func (e env) honest(t *testing.T, seed uint32) (trial.Claim, time.Duration) {
	t.Helper()
	p := e.kind(t).TamMa()
	in, r := tammatest.AutoPlay(p, seed, 3, 0)
	if !r.Won {
		t.Fatalf("reference player lost seed %d", seed)
	}
	c := trial.Claim{EndTick: r.EndTick, Lines: r.Lines, Score: r.Score, StateHash: r.StateHash}
	for _, x := range in {
		c.Inputs = append(c.Inputs, [2]int64{x.Tick, int64(x.Action)})
	}
	return c, time.Duration(r.EndTick*p.TickMS) * time.Millisecond
}

func (e env) start(t *testing.T, user uuid.UUID, key string) trial.StartView {
	t.Helper()
	v, err := e.svc.Start(context.Background(), user, "tam_ma", key)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	return v
}

func (e env) pass(t *testing.T) trial.SubmitView {
	t.Helper()
	run := e.start(t, e.user, "start-pass-0001")
	claim, dur := e.honest(t, run.Seed)
	e.advance(dur + 3*time.Second)
	v, err := e.svc.Submit(context.Background(), e.user, run.RunID, "submit-pass-001", claim)
	if err != nil || v.Verdict != trial.VerdictPass {
		t.Fatalf("pass: %+v %v", v, err)
	}
	return v
}

func blockedCode(err error) string {
	var b *trial.ErrBlocked
	if errors.As(err, &b) {
		return b.Code
	}
	return ""
}

// The full loop: missing trial → pass (+điểm duyên) → failed breakthrough keeps the pass → success consumes it.
func TestPassGatesNguyenAnhBreakthrough(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	last := e.lastOf(t, "kim_dan")
	e.putAtLast(t, e.charID, "kim_dan", last.RequiredMilli)
	bt := &breakthrough.Service{Pool: e.pool, Cultivation: e.cult, RNG: fixedRNG(9999), Trials: e.cb.Trials}

	pv, err := bt.Preview(ctx, e.user)
	if err != nil || pv.Ready || pv.Trial == nil || pv.Trial.Passed || len(pv.Missing) != 1 || pv.Missing[0].Kind != "trial" || pv.Missing[0].TrialKind != "tam_ma" {
		t.Fatalf("preview without pass: %+v %v", pv, err)
	}
	var nr *breakthrough.ErrNotReady
	if _, err := bt.Attempt(ctx, e.user, "bt-no-pass-0001", nil); !errors.As(err, &nr) {
		t.Fatalf("attempt without pass: %v", err)
	}

	sv := e.pass(t)
	if sv.VIPPoints != 2 || sv.PassValidUntil == nil || sv.Status.State != trial.StatePassed || sv.Outcome == nil || !sv.Outcome.Won {
		t.Fatalf("pass view %+v", sv)
	}
	if !sv.PassValidUntil.Equal(e.now().Add(7 * 24 * time.Hour)) {
		t.Fatalf("validity %v", sv.PassValidUntil)
	}
	// A valid pass blocks a new run.
	if _, err := e.svc.Start(ctx, e.user, "tam_ma", "start-again-001"); blockedCode(err) != trial.BlockPassActive {
		t.Fatalf("start with pass: %v", err)
	}
	pv, _ = bt.Preview(ctx, e.user)
	if !pv.Ready || !pv.Trial.Passed || pv.Trial.ValidUntil == nil {
		t.Fatalf("preview with pass: %+v", pv)
	}
	// Failure keeps the pass.
	r, err := bt.Attempt(ctx, e.user, "bt-fail-000001", nil)
	if err != nil || r.Success || r.TrialConsumed != "" {
		t.Fatalf("failed attempt %+v %v", r, err)
	}
	if n := e.scalar(t, "SELECT count(*) FROM trial_runs WHERE character_id=$1 AND verdict='pass' AND consumed_at IS NULL", e.charID); n != 1 {
		t.Fatalf("pass consumed by a failure: %d", n)
	}
	// After the breakthrough cooldown: success consumes it.
	e.advance(3 * time.Hour)
	e.putAtLast(t, e.charID, "kim_dan", last.RequiredMilli)
	bt.RNG = fixedRNG(0)
	r, err = bt.Attempt(ctx, e.user, "bt-success-0001", nil)
	if err != nil || !r.Success || r.TrialConsumed != "tam_ma" {
		t.Fatalf("success %+v %v", r, err)
	}
	if ref := e.scalar(t, "SELECT count(*) FROM trial_runs WHERE character_id=$1 AND consumed_ref='bt:bt-success-0001'", e.charID); ref != 1 {
		t.Fatal("pass not consumed by the success")
	}
	// Replaying the success does not consume anything else.
	if r2, err := bt.Attempt(ctx, e.user, "bt-success-0001", nil); err != nil || !r2.Replayed {
		t.Fatalf("replay %+v %v", r2, err)
	}
	// Now in Nguyên Anh: tâm ma is no longer startable.
	if _, err := e.svc.Start(ctx, e.user, "tam_ma", "start-after-0001"); blockedCode(err) != trial.BlockNotEligible {
		t.Fatalf("start after the gate: %v", err)
	}
}

func TestEligibilityAndResume(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	e.putAt(t, e.charID, "kim_dan.so_ky", 0)
	if _, err := e.svc.Start(ctx, e.user, "tam_ma", "start-early-001"); blockedCode(err) != trial.BlockNotEligible {
		t.Fatalf("Sơ Kỳ: %v", err)
	}
	st, err := e.svc.Status(ctx, e.user)
	if err != nil || len(st.Kinds) != 2 || st.Kinds[0].Kind != "tam_ma" || st.Kinds[0].State != trial.StateLocked || st.Kinds[0].FromRealmID != "kim_dan" || st.Kinds[0].ToRealmID != "nguyen_anh" {
		t.Fatalf("status %+v %v", st, err)
	}
	if _, err := e.svc.Start(ctx, e.user, "khong_co", "start-unknown-1"); !errors.Is(err, trial.ErrUnknownKind) {
		t.Fatalf("unknown kind: %v", err)
	}
	e.putAtLast(t, e.charID, "kim_dan", 0)
	a := e.start(t, e.user, "start-first-001")
	if a.Resumed || a.Seed != winSeed || !a.ExpiresAt.Equal(e.now().Add(8*time.Minute)) || a.Status.State != trial.StateInProgress {
		t.Fatalf("first start %+v", a)
	}
	// Same key → same run; another key while the run is open → the same run (same seed), never a fresh seed.
	b := e.start(t, e.user, "start-first-001")
	c := e.start(t, e.user, "start-second-01")
	if !b.Resumed || !c.Resumed || b.RunID != a.RunID || c.RunID != a.RunID || c.Seed != a.Seed {
		t.Fatalf("resume %+v %+v", b, c)
	}
	if n := e.scalar(t, "SELECT count(*) FROM trial_runs WHERE character_id=$1", e.charID); n != 1 {
		t.Fatalf("runs %d", n)
	}
}

// Anti-cheat: resubmitting an old run (same key → stored verdict, other key → refused), no second điểm duyên.
func TestResubmitOldRun(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	first := e.pass(t)
	run := first.RunID
	claim, _ := e.honest(t, winSeed)
	again, err := e.svc.Submit(ctx, e.user, run, "submit-pass-001", claim)
	if err != nil || !again.Replayed || again.Verdict != trial.VerdictPass || !again.PassValidUntil.Equal(*first.PassValidUntil) || again.VIPPoints != 0 {
		t.Fatalf("same-key replay %+v %v", again, err)
	}
	if _, err := e.svc.Submit(ctx, e.user, run, "submit-other-01", claim); !errors.Is(err, trial.ErrAlreadySubmitted) {
		t.Fatalf("other key: %v", err)
	}
	if n := e.scalar(t, "SELECT count(*) FROM vip_point_ledger WHERE user_id=$1 AND activity_id='trial_pass'", e.user); n != 1 {
		t.Fatalf("điểm duyên rows %d", n)
	}
}

// Anti-cheat: an edited score is rejected, starts the lose cooldown, and the cooldown blocks a new run.
func TestEditedScoreRejectedThenCooldown(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	run := e.start(t, e.user, "start-edit-0001")
	claim, dur := e.honest(t, run.Seed)
	claim.Score += 300
	e.advance(dur + 2*time.Second)
	v, err := e.svc.Submit(ctx, e.user, run.RunID, "submit-edit-001", claim)
	if err != nil || v.Verdict != trial.VerdictRejected || v.Reason != trial.ReasonMismatch || v.CooldownUntil == nil || v.Status.State != trial.StateCooldown {
		t.Fatalf("edited score %+v %v", v, err)
	}
	if !v.CooldownUntil.Equal(e.now().Add(30 * time.Minute)) {
		t.Fatalf("cooldown %v", v.CooldownUntil)
	}
	// Submitting during the cooldown (the same run, a new key) is refused; a new run is blocked.
	if _, err := e.svc.Submit(ctx, e.user, run.RunID, "submit-edit-002", claim); !errors.Is(err, trial.ErrAlreadySubmitted) {
		t.Fatalf("resubmit during cooldown: %v", err)
	}
	if _, err := e.svc.Start(ctx, e.user, "tam_ma", "start-edit-0002"); blockedCode(err) != trial.BlockCooldown {
		t.Fatalf("start during cooldown: %v", err)
	}
	e.advance(30 * time.Minute)
	if next := e.start(t, e.user, "start-edit-0003"); next.Resumed || next.RunID == run.RunID {
		t.Fatalf("after cooldown %+v", next)
	}
	if n := e.scalar(t, "SELECT count(*) FROM vip_point_ledger WHERE user_id=$1 AND activity_id='trial_pass'", e.user); n != 0 {
		t.Fatal("rejected run paid điểm duyên")
	}
}

// Anti-cheat: a shortened time (a valid log replayed faster than real time) is rejected.
func TestShortenedTimeRejected(t *testing.T) {
	e := setup(t)
	run := e.start(t, e.user, "start-fast-0001")
	claim, dur := e.honest(t, run.Seed)
	e.advance(dur / 2)
	v, err := e.svc.Submit(context.Background(), e.user, run.RunID, "submit-fast-001", claim)
	if err != nil || v.Verdict != trial.VerdictRejected || v.Reason != trial.ReasonTooFast {
		t.Fatalf("too fast %+v %v", v, err)
	}
}

// Anti-cheat: an expired run is judged "expired/late"; an abandoned run (never submitted) costs a cooldown too.
func TestExpiredAndAbandonedRuns(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	run := e.start(t, e.user, "start-late-0001")
	claim, _ := e.honest(t, run.Seed)
	e.advance(16 * time.Minute)
	v, err := e.svc.Submit(ctx, e.user, run.RunID, "submit-late-001", claim)
	if err != nil || v.Verdict != trial.VerdictExpired || v.Reason != trial.ReasonLate || v.CooldownUntil == nil {
		t.Fatalf("late %+v %v", v, err)
	}

	e.advance(time.Hour)
	run2 := e.start(t, e.user, "start-aband-001")
	e.advance(16 * time.Minute) // never submitted
	st, _ := e.svc.Status(ctx, e.user)
	if st.Kinds[0].State != trial.StateCooldown || st.Kinds[0].OpenRun != nil {
		t.Fatalf("status after abandon %+v", st.Kinds[0])
	}
	if _, err := e.svc.Start(ctx, e.user, "tam_ma", "start-aband-002"); blockedCode(err) != trial.BlockCooldown {
		t.Fatalf("start after abandon: %v", err)
	}
	if n := e.scalar(t, "SELECT count(*) FROM trial_runs WHERE id=$1 AND verdict='expired' AND reason='abandoned' AND submitted_at IS NULL", run2.RunID); n != 1 {
		t.Fatal("abandoned run not closed")
	}
	if _, err := e.svc.Submit(ctx, e.user, run2.RunID, "submit-aband-01", claim); !errors.Is(err, trial.ErrAlreadySubmitted) {
		t.Fatalf("submit after abandon: %v", err)
	}
}

// Anti-cheat: another character's run is "not found".
func TestOtherCharactersRun(t *testing.T) {
	e := setup(t)
	run := e.start(t, e.user, "start-mine-0001")
	claim, dur := e.honest(t, run.Seed)
	e.advance(dur + time.Second)
	thief, thiefChar := e.newCharacter(t, "Kẻ Trộm")
	e.putAtLast(t, thiefChar, "kim_dan", 0)
	if _, err := e.svc.Submit(context.Background(), thief, run.RunID, "submit-thief-01", claim); !errors.Is(err, trial.ErrRunNotFound) {
		t.Fatalf("thief: %v", err)
	}
	if _, err := e.svc.Submit(context.Background(), thief, uuid.New(), "submit-thief-02", claim); !errors.Is(err, trial.ErrRunNotFound) {
		t.Fatalf("random id: %v", err)
	}
	// The owner's honest submission still works.
	if v, err := e.svc.Submit(context.Background(), e.user, run.RunID, "submit-mine-001", claim); err != nil || v.Verdict != trial.VerdictPass {
		t.Fatalf("owner %+v %v", v, err)
	}
}

// Anti-cheat: concurrent submits of one run → exactly one judgement, one điểm duyên credit.
func TestConcurrentSubmits(t *testing.T) {
	e := setup(t)
	run := e.start(t, e.user, "start-race-0001")
	claim, dur := e.honest(t, run.Seed)
	e.advance(dur + time.Second)
	const n = 8
	var wg sync.WaitGroup
	results := make([]error, n)
	verdicts := make([]string, n)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := e.svc.Submit(context.Background(), e.user, run.RunID, "submit-race-00"+string(rune('a'+i)), claim)
			results[i], verdicts[i] = err, v.Verdict
		}()
	}
	wg.Wait()
	ok := 0
	for i, err := range results {
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

// M1: a macro/solver burst (more inputs per window than a human) is rejected as implausible_input; the cadence
// telemetry is stored for review, and the rejection details (outcome) are not returned to the client.
func TestImplausibleInputRejectedWithTelemetry(t *testing.T) {
	e := setup(t)
	run := e.start(t, e.user, "start-burst-001")
	claim, dur := e.honest(t, run.Seed)
	first := claim.Inputs[0][0]
	var burst [][2]int64
	for i := range e.kind(t).TamMa().MaxInputsPerWindow + 4 {
		burst = append(burst, [2]int64{min(int64(i), first), 1})
	}
	claim.Inputs = append(burst, claim.Inputs...)
	e.advance(dur + 2*time.Second)
	v, err := e.svc.Submit(context.Background(), e.user, run.RunID, "submit-burst-01", claim)
	if err != nil || v.Verdict != trial.VerdictRejected || v.Reason != trial.ReasonInputRate || v.Outcome != nil || v.CooldownUntil == nil {
		t.Fatalf("burst %+v %v", v, err)
	}
	if w := e.scalar(t, "SELECT (result->>'maxWindowInputs')::bigint FROM trial_runs WHERE id=$1", run.RunID); w < int64(e.kind(t).TamMa().MaxInputsPerWindow+1) {
		t.Fatalf("telemetry maxWindowInputs %d", w)
	}
	// An honest pass stores telemetry too, and returns its outcome.
	e.advance(31 * time.Minute)
	ok := e.pass(t)
	if ok.Outcome == nil || ok.Outcome.Inputs == 0 || ok.Outcome.MinLockGapTicks < e.kind(t).TamMa().MinLockGapTicks || ok.Outcome.InputsPerSecMilli <= 0 {
		t.Fatalf("pass telemetry %+v", ok.Outcome)
	}
}
