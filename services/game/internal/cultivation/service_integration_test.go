//go:build integration

package cultivation_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"tiendao/game/internal/character"
	"tiendao/game/internal/content"
	"tiendao/game/internal/contribution"
	"tiendao/game/internal/cultivation"
	"tiendao/game/internal/db"
	"tiendao/game/internal/platform/postgres"
	"tiendao/game/internal/platform/testdb"
)

type env struct {
	pool  *pgxpool.Pool
	cb    *content.Bundle
	svc   *cultivation.Service
	user  uuid.UUID
	char  uuid.UUID
	clock *time.Time
	ctx   context.Context
}

func setup(t *testing.T) *env {
	t.Helper()
	pool := testdb.New(t)
	cb, err := content.LoadAll("../../../../content/catalog")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 6, 1, 5, 0, 0, 0, time.UTC) // 12:00 game time
	e := &env{pool: pool, cb: cb, user: uuid.New(), char: uuid.New(), clock: &now, ctx: context.Background()}
	e.svc = &cultivation.Service{Pool: pool, Realms: cb.Realms, Contribution: cb.Contribution, VIP: cb.VIP, Now: func() time.Time { return *e.clock }}
	e.exec(t, "INSERT INTO users (id, email, password_hash) VALUES ($1, $2, NULL)", e.user, e.user.String()+"@test")
	e.exec(t, "INSERT INTO characters (id, user_id, name) VALUES ($1, $2, 'Thử Nghiệm')", e.char, e.user)
	e.exec(t, "INSERT INTO character_progressions (character_id, realm_id, stage_id, last_cultivated_at) VALUES ($1, 'luyen_khi', 'luyen_khi.tang_1', $2)",
		e.char, now.Add(-time.Hour))
	return e
}

func (e *env) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := e.pool.Exec(e.ctx, sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func (e *env) scalar(t *testing.T, sql string, args ...any) (v int64) {
	t.Helper()
	if err := e.pool.QueryRow(e.ctx, sql, args...).Scan(&v); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return v
}

// checkC3 asserts Σ credits (excluding grant) == daily_contribution.points_milli for today.
func (e *env) checkC3(t *testing.T) {
	t.Helper()
	sum := e.scalar(t, "SELECT COALESCE(SUM(credited_milli),0) FROM cultivation_credits WHERE character_id=$1 AND source <> 'late_join_grant'", e.char)
	pts := e.scalar(t, "SELECT COALESCE(SUM(points_milli),0) FROM daily_contribution WHERE character_id=$1", e.char)
	if sum != pts {
		t.Fatalf("C3 broken: credits %d vs daily_contribution %d", sum, pts)
	}
}

func TestSettleIdempotentReplay(t *testing.T) {
	e := setup(t)
	a, err := e.svc.Settle(e.ctx, e.user, "key-replay-0001")
	if err != nil {
		t.Fatal(err)
	}
	b, err := e.svc.Settle(e.ctx, e.user, "key-replay-0001")
	if err != nil {
		t.Fatal(err)
	}
	if a.EarnedMilli != 2083 || a.Today.PointsMilli != 2083 || !b.Replayed || b.CultivationMilli != a.CultivationMilli {
		t.Fatalf("first=%+v second=%+v", a, b)
	}
	if n := e.scalar(t, "SELECT count(*) FROM cultivation_credits WHERE character_id=$1", e.char); n != 1 {
		t.Fatalf("credits rows %d", n)
	}
	e.checkC3(t)
}

func TestConcurrentSettlementsNeverDoubleAward(t *testing.T) {
	e := setup(t)
	var wg sync.WaitGroup
	results := make([]cultivation.Status, 20)
	errs := make([]error, 20)
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], errs[i] = e.svc.Settle(e.ctx, e.user, fmt.Sprintf("key-concurrent-%04d", i))
		}()
	}
	wg.Wait()
	var total, final int64
	for i, r := range results {
		if errs[i] != nil {
			t.Fatalf("settle %d: %v", i, errs[i])
		}
		total += r.EarnedMilli
		st, err := e.cb.Realms.Stage(r.StageID)
		if err != nil {
			t.Fatalf("settle %d: %v", i, err)
		}
		final = max(final, e.cb.Realms.Position(st, r.CultivationMilli)) // ladder position: 1 h may pass Tầng 1
	}
	if total != 2083 || final != 2083 {
		t.Fatalf("total earned %d, final %d; want 2083/2083", total, final)
	}
	e.checkC3(t)
}

// Activity credits are clipped by the 45 bucket cap and the 150 ceiling; clipped points are logged.
func TestCreditDailyCapsAndLog(t *testing.T) {
	e := setup(t)
	e.exec(t, "UPDATE character_progressions SET realm_id='chuan_thanh', stage_id='chuan_thanh.so_ky_1' WHERE character_id=$1", e.char)
	credit := func(src contribution.Source, raw int64, key string) cultivation.CreditOutcome {
		var out cultivation.CreditOutcome
		err := postgres.InTx(e.ctx, e.pool, func(tx pgx.Tx) error {
			q := db.New(tx)
			p, err := q.GetProgressionForUpdate(e.ctx, e.char)
			if err != nil {
				return err
			}
			_, out, err = e.svc.CreditLocked(e.ctx, q, p, src, raw, cultivation.Ref{Type: "test", IdemKey: key}, *e.clock)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	if o := credit(contribution.SourceLecture, 40_000, "lec-1"); o.Applied.CreditedMilli != 40_000 {
		t.Fatalf("lecture 1: %+v", o.Applied)
	}
	if o := credit(contribution.SourceLecture, 40_000, "lec-2"); o.Applied.CreditedMilli != 5_000 || o.Applied.ClippedCeilingMilli != 35_000 {
		t.Fatalf("lecture 2 must hit the 45 activity cap: %+v", o.Applied)
	}
	credit(contribution.SourceFaction, 100_000, "fac-1") // community → 30
	e.clock = ptr(e.clock.Add(10 * time.Hour))           // idle: 8h window → 16.664 (still today: 22:00 game time)
	st, err := e.svc.Settle(e.ctx, e.user, "settle-cap-0001")
	if err != nil {
		t.Fatal(err)
	}
	if st.Today.PointsMilli != 45_000+30_000+16_664 || st.Today.Buckets[contribution.Activity].Milli != 45_000 || !st.HitOfflineCap {
		t.Fatalf("today %+v", st.Today)
	}
	if got := e.scalar(t, "SELECT clipped_ceiling_milli FROM cultivation_credits WHERE character_id=$1 AND idempotency_key='lecture:lec-2'", e.char); got != 35_000 {
		t.Fatalf("clipped logged %d", got)
	}
	e.checkC3(t)
	// Replaying a credit key is rejected by the UNIQUE constraint (callers check their own log first).
	err = postgres.InTx(e.ctx, e.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		p, _ := q.GetProgressionForUpdate(e.ctx, e.char)
		_, _, err := e.svc.CreditLocked(e.ctx, q, p, contribution.SourceLecture, 1000, cultivation.Ref{IdemKey: "lec-1"}, *e.clock)
		return err
	})
	if err == nil {
		t.Fatal("duplicate credit key must fail")
	}
}

// Auto-advance of minor stages + late-join reserve release: never past the realm's bottleneck, not in the day totals.
func TestAutoAdvanceAndGrantRelease(t *testing.T) {
	e := setup(t)
	e.exec(t, "UPDATE character_progressions SET grant_reserve_milli=1000000 WHERE character_id=$1", e.char)
	st, err := e.svc.Settle(e.ctx, e.user, "settle-grant-0001")
	if err != nil {
		t.Fatal(err)
	}
	last, _ := e.cb.Realms.Stage("luyen_khi.tang_9")
	if st.StageID != last.ID || st.CultivationMilli != last.CapMilli || st.GrantReserveMilli != 1_000_000-(104_200-2083) || len(st.StagesAdvanced) != 8 || !st.HitBottleneck {
		t.Fatalf("status %+v", st)
	}
	if st.Today.PointsMilli != 2083 {
		t.Fatalf("grant must not count in today's points: %d", st.Today.PointsMilli)
	}
	if n := e.scalar(t, "SELECT count(*) FROM progression_events WHERE character_id=$1 AND kind='stage_reached'", e.char); n != 8 {
		t.Fatalf("stage events %d", n)
	}
	if g := e.scalar(t, "SELECT credited_milli FROM cultivation_credits WHERE character_id=$1 AND source='late_join_grant'", e.char); g != 104_200-2083 {
		t.Fatalf("grant release log %d", g)
	}
	e.checkC3(t)
}

// Catch-up ×2: world started, server day ≥ 3, position below the median.
func TestCatchUpDoublesAndStoresBonus(t *testing.T) {
	e := setup(t)
	e.exec(t, "UPDATE world_state SET started_at=$1, median_position_milli=5000000", e.clock.Add(-10*24*time.Hour))
	st, err := e.svc.Settle(e.ctx, e.user, "settle-catchup-01")
	if err != nil {
		t.Fatal(err)
	}
	if st.EarnedMilli != 4166 || !st.Today.CatchUp.Active || st.Today.CatchUp.MultiplierBP != 20000 {
		t.Fatalf("status %+v", st)
	}
	if b := e.scalar(t, "SELECT catch_up_bonus_milli FROM daily_contribution WHERE character_id=$1", e.char); b != 2083 {
		t.Fatalf("bonus %d", b)
	}
	if m := e.scalar(t, "SELECT multiplier_bp FROM cultivation_settlements WHERE character_id=$1", e.char); m != 20000 {
		t.Fatalf("settlement multiplier %d", m)
	}
	e.checkC3(t)
}

// Late-join grant at character creation: once per account, only after server day 14.
func TestLateJoinGrantOncePerAccount(t *testing.T) {
	e := setup(t)
	chars := &character.Service{Pool: e.pool, Cultivation: e.svc}
	u := uuid.New()
	e.exec(t, "INSERT INTO users (id, email) VALUES ($1, $2)", u, u.String()+"@t")
	e.exec(t, "UPDATE world_state SET started_at=$1, median_position_milli=2000000, median_game_day='2026-05-31'", e.clock.Add(-20*24*time.Hour))
	v, err := chars.Create(e.ctx, u, "Hậu Lai")
	if err != nil {
		t.Fatal(err)
	}
	if v.GrantReserveMilli != 1_000_000-104_200 { // view projects the release into Luyện Khí's room
		t.Fatalf("grant reserve %d", v.GrantReserveMilli)
	}
	// Projection releases into the realm's room without writing anything.
	if v.StageID != "luyen_khi.tang_9" || e.scalar(t, "SELECT grant_reserve_milli FROM character_progressions WHERE character_id=$1", v.ID) != 1_000_000 {
		t.Fatalf("view %+v", v)
	}
	e.exec(t, "DELETE FROM characters WHERE id=$1", v.ID)
	v2, err := chars.Create(e.ctx, u, "Hậu Lai Nhị")
	if err != nil {
		t.Fatal(err)
	}
	if v2.GrantReserveMilli != 0 || e.scalar(t, "SELECT count(*) FROM late_join_grants WHERE user_id=$1", u) != 1 {
		t.Fatal("second grant for the same account")
	}
	// Server day ≤ 14: no grant.
	u3 := uuid.New()
	e.exec(t, "INSERT INTO users (id, email) VALUES ($1, $2)", u3, u3.String()+"@t")
	e.exec(t, "UPDATE world_state SET started_at=$1", e.clock.Add(-14*24*time.Hour))
	if v3, err := chars.Create(e.ctx, u3, "Sớm Tới"); err != nil || v3.GrantReserveMilli != 0 {
		t.Fatalf("day 14 grant: %+v %v", v3.GrantReserveMilli, err)
	}
}

func ptr(t time.Time) *time.Time { return &t }

// M2: below MinSettleSeconds the reserve is still released (matching Project), and the key replays idempotently.
func TestGrantReleaseBelowMinSettle(t *testing.T) {
	e := setup(t)
	e.exec(t, "UPDATE character_progressions SET last_cultivated_at=$2, grant_reserve_milli=50000 WHERE character_id=$1", e.char, *e.clock)
	a, err := e.svc.Settle(e.ctx, e.user, "settle-short-0001")
	if err != nil {
		t.Fatal(err)
	}
	if a.EarnedMilli != 0 || a.GrantReserveMilli != 0 || a.CultivationMilli+45_000 != 50_000 || len(a.StagesAdvanced) != 6 {
		t.Fatalf("status %+v", a)
	}
	b, err := e.svc.Settle(e.ctx, e.user, "settle-short-0001")
	if err != nil || !b.Replayed || b.CultivationMilli != a.CultivationMilli {
		t.Fatalf("replay %+v %v", b, err)
	}
	if n := e.scalar(t, "SELECT count(*) FROM cultivation_credits WHERE character_id=$1 AND source='late_join_grant'", e.char); n != 1 {
		t.Fatalf("grant rows %d", n)
	}
	if last := e.scalar(t, "SELECT extract(epoch FROM last_cultivated_at)::bigint FROM character_progressions WHERE character_id=$1", e.char); last != e.clock.Unix() {
		t.Fatal("last_cultivated_at must not move")
	}
	e.checkC3(t)
}
