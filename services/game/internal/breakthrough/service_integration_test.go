//go:build integration

package breakthrough_test

import (
	"context"
	"errors"
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
)

type fixedRNG int

func (f fixedRNG) IntN(int) int { return int(f) }

type env struct {
	pool   *pgxpool.Pool
	cult   *cultivation.Service
	chars  *character.Service
	user   uuid.UUID
	charID uuid.UUID
	now    time.Time
	clock  *time.Time // server clock used by the services; advance with e.advance
}

func (e env) advance(d time.Duration) { *e.clock = e.clock.Add(d) }

func setup(t *testing.T) env {
	t.Helper()
	ctx := context.Background()
	pool := testdb.New(t)
	cb, err := content.LoadAll("../../../../content/catalog")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	clock := &now
	cult := &cultivation.Service{Pool: pool, Realms: cb.Realms, Contribution: cb.Contribution, VIP: cb.VIP, Now: func() time.Time { return *clock }}
	chars := &character.Service{Pool: pool, Cultivation: cult}
	user := uuid.New()
	if _, err := pool.Exec(ctx, "INSERT INTO users (id, email) VALUES ($1, $2)", user, user.String()+"@t"); err != nil {
		t.Fatal(err)
	}
	v, err := chars.Create(ctx, user, "Lâm Phong")
	if err != nil {
		t.Fatal(err)
	}
	return env{pool, cult, chars, user, v.ID, now, clock}
}

// putAt places the character at a stage with the given cultivation and no pending offline time.
func (e env) putAt(t *testing.T, stageID string, cultivation int64) {
	t.Helper()
	_, err := e.pool.Exec(context.Background(),
		"UPDATE character_progressions SET stage_id=$2, realm_id=split_part($2,'.',1), cultivation_milli=$3, last_cultivated_at=$4 WHERE character_id=$1",
		e.charID, stageID, cultivation, e.now)
	if err != nil {
		t.Fatal(err)
	}
}

func (e env) give(t *testing.T, item string, qty int64) {
	t.Helper()
	if _, err := e.pool.Exec(context.Background(), "INSERT INTO inventory_items VALUES ($1,$2,$3)", e.charID, item, qty); err != nil {
		t.Fatal(err)
	}
}

func (e env) qty(t *testing.T, item string) (q int64) {
	_ = e.pool.QueryRow(context.Background(), "SELECT COALESCE((SELECT quantity FROM inventory_items WHERE character_id=$1 AND item_id=$2),0)", e.charID, item).Scan(&q)
	return
}

func (e env) scalar(t *testing.T, sql string, args ...any) (v int64) {
	t.Helper()
	if err := e.pool.QueryRow(context.Background(), sql, args...).Scan(&v); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return
}

func TestCharacterCreateRules(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	if _, err := e.chars.Create(ctx, e.user, "Khác Tên"); !errors.Is(err, character.ErrAlreadyExists) {
		t.Fatalf("second character: %v", err)
	}
	other := uuid.New()
	_, _ = e.pool.Exec(ctx, "INSERT INTO users (id, email) VALUES ($1, $2)", other, other.String()+"@t")
	if _, err := e.chars.Create(ctx, other, "lâm phong"); !errors.Is(err, character.ErrNameTaken) {
		t.Fatalf("case-insensitive duplicate name: %v", err)
	}
	v, _ := e.chars.Get(ctx, e.user)
	if v.StageID != "luyen_khi.tang_1" || v.Path != "tien" || v.Tier != 1 || v.Today.CeilingMilli != 150_000 {
		t.Fatalf("start %+v", v)
	}
}

func TestMajorBreakthroughSuccessCarriesOver(t *testing.T) {
	e := setup(t)
	stage, _ := e.cult.Realms.Stage("luyen_khi.tang_9")
	e.putAt(t, stage.ID, stage.RequiredMilli+6_000)
	e.give(t, "truc_co_dan", 2)
	svc := &breakthrough.Service{Pool: e.pool, Cultivation: e.cult, RNG: fixedRNG(0)}

	r, err := svc.Attempt(context.Background(), e.user, "bt-key-success-01", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Success || r.StageID != "truc_co.so_ky" || r.CultivationMilli != 6_000 || r.Tribulation {
		t.Fatalf("result %+v", r)
	}
	if got := e.qty(t, "truc_co_dan"); got != 1 {
		t.Fatalf("pill not consumed: %d", got)
	}
	// Replay returns the same outcome and consumes nothing more.
	r2, err := svc.Attempt(context.Background(), e.user, "bt-key-success-01", nil)
	if err != nil || !r2.Replayed || !r2.Success || e.qty(t, "truc_co_dan") != 1 {
		t.Fatalf("replay %+v %v", r2, err)
	}
	if n := e.scalar(t, "SELECT count(*) FROM outbox_events WHERE topic='announcement.major_breakthrough'"); n != 1 {
		t.Fatalf("announcements %d", n)
	}
}

func TestFailureKeepsCultivationConsumesPillStartsCooldown(t *testing.T) {
	e := setup(t)
	stage, _ := e.cult.Realms.Stage("luyen_khi.tang_9")
	e.putAt(t, stage.ID, stage.RequiredMilli)
	e.give(t, "truc_co_dan", 1)
	svc := &breakthrough.Service{Pool: e.pool, Cultivation: e.cult, RNG: fixedRNG(9999)}

	r, err := svc.Attempt(context.Background(), e.user, "bt-key-fail-0001", nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Success || r.CultivationMilli != stage.RequiredMilli || r.FailStreak != 1 || r.CooldownUntil == nil || r.LossMilli != 0 {
		t.Fatalf("result %+v", r)
	}
	if e.qty(t, "truc_co_dan") != 0 {
		t.Fatal("pill must be consumed on failure")
	}
	// Next attempt: blocked by cooldown AND missing pill, odds include pity.
	_, err = svc.Attempt(context.Background(), e.user, "bt-key-fail-0002", nil)
	var nr *breakthrough.ErrNotReady
	if !errors.As(err, &nr) || len(nr.Preview.Missing) != 2 || nr.Preview.SuccessBP != stage.Breakthrough.SuccessBP+stage.Breakthrough.PityStepBP {
		t.Fatalf("want not-ready with pity, got %v %+v", err, nr)
	}
}

func lastStage(t *testing.T, e env, realmID string) *realm.Stage {
	for _, s := range e.cult.Realms.Ladder("tien") {
		if s.RealmID == realmID && s.IsLastOfRealm() {
			return s
		}
	}
	t.Fatal("no stage")
	return nil
}

// Tribulation failure: −25% of the current stage (never drops a stage), +1 streak, 24h cooldown, audited.
func TestTribulationFailureLosesQuarter(t *testing.T) {
	e := setup(t)
	last := lastStage(t, e, "hoa_than")
	e.putAt(t, last.ID, last.CapMilli)
	svc := &breakthrough.Service{Pool: e.pool, Cultivation: e.cult, RNG: fixedRNG(9999)}
	pv, err := svc.Preview(context.Background(), e.user)
	if err != nil || pv.Tribulation == nil || pv.SuccessBP != 8500 || pv.Tribulation.LossMilli != last.CapMilli/4 {
		t.Fatalf("preview %+v %v", pv, err)
	}
	loss := pv.Tribulation.LossMilli
	r, err := svc.Attempt(context.Background(), e.user, "trib-fail-00001", &loss)
	if err != nil {
		t.Fatal(err)
	}
	if r.Success || !r.Tribulation || r.LossMilli != loss || r.StageID != last.ID || r.CultivationMilli != last.CapMilli-loss ||
		r.FailStreak != 1 || !r.CooldownUntil.Equal(e.now.Add(24*time.Hour)) {
		t.Fatalf("result %+v", r)
	}
	if got := e.scalar(t, "SELECT loss_milli FROM breakthrough_attempts WHERE character_id=$1 AND tribulation", e.charID); got != loss {
		t.Fatalf("audit loss %d", got)
	}
	if got := e.scalar(t, "SELECT cultivation_before_milli FROM breakthrough_attempts WHERE character_id=$1", e.charID); got != last.CapMilli {
		t.Fatalf("audit before %d", got)
	}
	// Replay: same outcome, no second loss.
	r2, err := svc.Attempt(context.Background(), e.user, "trib-fail-00001", &loss)
	if err != nil || !r2.Replayed || r2.LossMilli != loss || r2.CultivationMilli != last.CapMilli-loss {
		t.Fatalf("replay %+v %v", r2, err)
	}
}

// After guarantee_after consecutive fails the next tribulation succeeds regardless of the roll.
func TestTribulationGuaranteeAfterThreeFails(t *testing.T) {
	e := setup(t)
	last := lastStage(t, e, "do_kiep")
	e.putAt(t, last.ID, last.RequiredMilli)
	if _, err := e.pool.Exec(context.Background(), "UPDATE character_progressions SET breakthrough_fail_streak=3 WHERE character_id=$1", e.charID); err != nil {
		t.Fatal(err)
	}
	svc := &breakthrough.Service{Pool: e.pool, Cultivation: e.cult, RNG: fixedRNG(9999)}
	r, err := svc.Attempt(context.Background(), e.user, "trib-guarantee-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Success || !r.Guaranteed || r.SuccessBP != 10000 || r.StageID != "nhan_tien.so_ky_1" || r.FailStreak != 0 {
		t.Fatalf("result %+v", r)
	}
}

// The acceptLossMilli guard: a larger loss than accepted → 409 quote_changed, nothing written.
// Preview includes the pending settlement, so the previewed loss is exactly the applied loss.
func TestTribulationGuardAndPreviewIncludesSettlement(t *testing.T) {
	e := setup(t)
	last := lastStage(t, e, "luyen_hu")
	e.putAt(t, last.ID, last.RequiredMilli)
	if _, err := e.pool.Exec(context.Background(), "UPDATE character_progressions SET last_cultivated_at=$2 WHERE character_id=$1", e.charID, e.now.Add(-2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	svc := &breakthrough.Service{Pool: e.pool, Cultivation: e.cult, RNG: fixedRNG(9999)}
	pv, err := svc.Preview(context.Background(), e.user)
	if err != nil {
		t.Fatal(err)
	}
	stored := last.RequiredMilli + 2*2083
	if !pv.SettlementIncluded || pv.ProjectedCultivationMilli != stored || pv.Tribulation.LossMilli != stored/4 {
		t.Fatalf("preview %+v %+v", pv, pv.Tribulation)
	}
	low := pv.Tribulation.LossMilli - 1
	_, err = svc.Attempt(context.Background(), e.user, "trib-guard-00001", &low)
	var qc *breakthrough.ErrQuoteChanged
	if !errors.As(err, &qc) || qc.Preview.Tribulation.LossMilli != pv.Tribulation.LossMilli {
		t.Fatalf("want quote_changed, got %v", err)
	}
	if n := e.scalar(t, "SELECT count(*) FROM breakthrough_attempts WHERE character_id=$1", e.charID) +
		e.scalar(t, "SELECT count(*) FROM cultivation_settlements WHERE character_id=$1", e.charID); n != 0 {
		t.Fatalf("guard must write nothing (rows %d)", n)
	}
	ok := pv.Tribulation.LossMilli
	r, err := svc.Attempt(context.Background(), e.user, "trib-guard-00002", &ok)
	if err != nil || r.LossMilli != pv.Tribulation.LossMilli {
		t.Fatalf("applied loss %+v %v", r, err)
	}
}

// H1: the client sends the preview's maxLossMilli; idle accrual between preview and attempt must not refuse it.
func TestTribulationQuoteBoundSurvivesDrift(t *testing.T) {
	e := setup(t)
	last := lastStage(t, e, "luyen_hu")
	e.putAt(t, last.ID, last.RequiredMilli)
	if _, err := e.pool.Exec(context.Background(), "UPDATE character_progressions SET last_cultivated_at=$2 WHERE character_id=$1", e.charID, e.now.Add(-2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	svc := &breakthrough.Service{Pool: e.pool, Cultivation: e.cult, RNG: fixedRNG(9999)}
	pv, err := svc.Preview(context.Background(), e.user)
	if err != nil || pv.Tribulation.MaxLossMilli <= pv.Tribulation.LossMilli {
		t.Fatalf("preview %+v %v", pv.Tribulation, err)
	}
	e.advance(30 * time.Second)
	bound := pv.Tribulation.MaxLossMilli
	r, err := svc.Attempt(context.Background(), e.user, "trib-drift-00001", &bound)
	if err != nil {
		t.Fatalf("attempt after 30s drift must be accepted: %v", err)
	}
	if r.Success || r.LossMilli <= pv.Tribulation.LossMilli || r.LossMilli > bound {
		t.Fatalf("applied loss %d (preview %d, bound %d)", r.LossMilli, pv.Tribulation.LossMilli, bound)
	}
	stored := last.RequiredMilli + (2*3600+30)*2083/3600
	if r.LossMilli != stored/4 {
		t.Fatalf("applied loss %d, want the server's own %d (never the client's value)", r.LossMilli, stored/4)
	}
}

func TestTribulationQuoteBoundExceeded(t *testing.T) {
	e := setup(t)
	last := lastStage(t, e, "luyen_hu")
	e.putAt(t, last.ID, last.RequiredMilli)
	svc := &breakthrough.Service{Pool: e.pool, Cultivation: e.cult, RNG: fixedRNG(9999)}
	pv, err := svc.Preview(context.Background(), e.user)
	if err != nil {
		t.Fatal(err)
	}
	e.advance(2 * time.Hour) // far beyond quote_window_seconds of accrual
	bound := pv.Tribulation.MaxLossMilli
	_, err = svc.Attempt(context.Background(), e.user, "trib-exceed-0001", &bound)
	var qc *breakthrough.ErrQuoteChanged
	if !errors.As(err, &qc) || qc.Preview.Tribulation.LossMilli <= bound {
		t.Fatalf("want quote_changed, got %v", err)
	}
	if n := e.scalar(t, "SELECT count(*) FROM breakthrough_attempts WHERE character_id=$1", e.charID); n != 0 {
		t.Fatal("quote_changed must write nothing")
	}
}

// M3: a non-guaranteed tribulation without acceptLossMilli is refused.
func TestTribulationQuoteRequired(t *testing.T) {
	e := setup(t)
	last := lastStage(t, e, "hop_the")
	e.putAt(t, last.ID, last.RequiredMilli)
	svc := &breakthrough.Service{Pool: e.pool, Cultivation: e.cult, RNG: fixedRNG(0)}
	_, err := svc.Attempt(context.Background(), e.user, "trib-noquote-001", nil)
	var qr *breakthrough.ErrQuoteRequired
	if !errors.As(err, &qr) || qr.Preview.Tribulation == nil {
		t.Fatalf("want quote_required, got %v", err)
	}
	if n := e.scalar(t, "SELECT count(*) FROM breakthrough_attempts WHERE character_id=$1", e.charID); n != 0 {
		t.Fatal("quote_required must write nothing")
	}
}

// M2: with a late-join reserve and < MinSettleSeconds elapsed, preview and attempt release the same reserve,
// so the disclosed odds are the applied odds.
func TestPreviewMatchesAttemptWithGrantReserve(t *testing.T) {
	e := setup(t)
	last := lastStage(t, e, "hoa_than")
	e.putAt(t, last.ID, last.RequiredMilli)
	if _, err := e.pool.Exec(context.Background(), "UPDATE character_progressions SET grant_reserve_milli=100000 WHERE character_id=$1", e.charID); err != nil {
		t.Fatal(err)
	}
	svc := &breakthrough.Service{Pool: e.pool, Cultivation: e.cult, RNG: fixedRNG(0)}
	pv, err := svc.Preview(context.Background(), e.user)
	if err != nil || pv.SuccessBP != 8500 || pv.ProjectedCultivationMilli != last.CapMilli {
		t.Fatalf("preview %+v %v", pv, err)
	}
	bound := pv.Tribulation.MaxLossMilli
	r, err := svc.Attempt(context.Background(), e.user, "trib-reserve-001", &bound)
	if err != nil || r.SuccessBP != pv.SuccessBP {
		t.Fatalf("attempt successBp %d vs preview %d (%v)", r.SuccessBP, pv.SuccessBP, err)
	}
	if got := e.scalar(t, "SELECT credited_milli FROM cultivation_credits WHERE character_id=$1 AND source='late_join_grant'", e.charID); got != last.CapMilli-last.RequiredMilli {
		t.Fatalf("grant release logged %d", got)
	}
}
