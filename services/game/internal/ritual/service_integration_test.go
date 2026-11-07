//go:build integration

package ritual_test

import (
	"context"
	"math/rand/v2"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"tiendao/game/internal/content"
	"tiendao/game/internal/cultivation"
	"tiendao/game/internal/dailytask"
	"tiendao/game/internal/platform/testdb"
	"tiendao/game/internal/ritual"
)

type lockedRNG struct {
	mu sync.Mutex
	r  *rand.Rand
}

func (l *lockedRNG) IntN(n int) int { l.mu.Lock(); defer l.mu.Unlock(); return l.r.IntN(n) }

// fixedRNG always rolls v (to pick a specific outcome).
type fixedRNG int

func (f fixedRNG) IntN(int) int { return int(f) }

type env struct {
	pool    *pgxpool.Pool
	cb      *content.Bundle
	cult    *cultivation.Service
	incense *ritual.Service
	fortune *ritual.Fortune
	user    uuid.UUID
	char    uuid.UUID
	clock   *time.Time
	ctx     context.Context
}

func setup(t *testing.T) *env {
	t.Helper()
	pool := testdb.New(t)
	cb, err := content.LoadAll("../../../../content/catalog")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 6, 1, 1, 0, 0, 0, time.UTC) // 08:00 ICT
	e := &env{pool: pool, cb: cb, user: uuid.New(), char: uuid.New(), clock: &now, ctx: context.Background()}
	nowFn := func() time.Time { return *e.clock }
	e.cult = &cultivation.Service{Pool: pool, Realms: cb.Realms, Contribution: cb.Contribution, VIP: cb.VIP, Tasks: cb.DailyTasks, Now: nowFn}
	e.incense = &ritual.Service{Pool: pool, Tasks: cb.DailyTasks, Cultivation: e.cult, VIP: cb.VIP, Now: nowFn}
	e.fortune = &ritual.Fortune{Pool: pool, Cat: cb.Fortunes, Cultivation: e.cult, RNG: &lockedRNG{r: rand.New(rand.NewPCG(1, 2))}, Now: nowFn}
	e.exec(t, "INSERT INTO users (id, email) VALUES ($1, $2)", e.user, e.user.String()+"@t")
	e.exec(t, "INSERT INTO characters (id, user_id, name) VALUES ($1, $2, 'Thử Nghiệm')", e.char, e.user)
	// Trúc Cơ: stages are large, so idle credits never touch the bình cảnh in these tests.
	e.exec(t, "INSERT INTO character_progressions (character_id, realm_id, stage_id, last_cultivated_at) VALUES ($1, 'truc_co', 'truc_co.so_ky', $2)", e.char, now)
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

func TestIncenseStreakAcrossDays(t *testing.T) {
	e := setup(t)
	task, ok := e.cb.DailyTasks.ByKind(dailytask.Incense)
	if !ok {
		t.Fatal("no incense task")
	}
	pay := task.StreakPoints // rewards come from daily_tasks.yaml (tuned by balance)
	if pay(3) <= pay(1) {
		t.Fatalf("streak ladder must grow by day 3: %d → %d", pay(1), pay(3))
	}
	want := []struct {
		streak int32
		milli  int64
	}{{1, pay(1)}, {2, pay(2)}, {3, pay(3)}}
	for i, w := range want {
		r, err := e.incense.Incense(e.ctx, e.user)
		if err != nil {
			t.Fatal(err)
		}
		if r.Replayed || r.Streak.Current != w.streak || r.EarnedMilli != w.milli || !r.Task.Claimed || !r.Streak.OfferedToday {
			t.Fatalf("day %d: %+v", i, r)
		}
		again, err := e.incense.Incense(e.ctx, e.user) // same day: replay, nothing written
		if err != nil || !again.Replayed || again.EarnedMilli != w.milli || again.Streak.Current != w.streak {
			t.Fatalf("day %d replay: %+v %v", i, again, err)
		}
		*e.clock = e.clock.Add(24 * time.Hour)
	}
	if n := e.scalar(t, "SELECT COUNT(*) FROM cultivation_credits WHERE character_id=$1 AND source='ritual'", e.char); n != 3 {
		t.Fatalf("ritual credits %d", n)
	}
	if n := e.scalar(t, "SELECT COUNT(*) FROM vip_point_ledger WHERE user_id=$1 AND activity_id='daily_task_claim'", e.user); n != 3 {
		t.Fatalf("điểm duyên rows %d", n)
	}
	// Skip a day: the streak restarts at 1, best stays 3.
	*e.clock = e.clock.Add(24 * time.Hour)
	r, err := e.incense.Incense(e.ctx, e.user)
	if err != nil || r.Streak.Current != 1 || r.Streak.Best != 3 || r.EarnedMilli != pay(1) {
		t.Fatalf("after a gap: %+v %v", r, err)
	}
	// Concurrent offerings on a new day: exactly one credit.
	*e.clock = e.clock.Add(24 * time.Hour)
	var wg sync.WaitGroup
	for range 6 {
		wg.Add(1)
		go func() { defer wg.Done(); _, _ = e.incense.Incense(e.ctx, e.user) }()
	}
	wg.Wait()
	if n := e.scalar(t, "SELECT COUNT(*) FROM cultivation_credits WHERE character_id=$1 AND source='ritual'", e.char); n != 5 {
		t.Fatalf("concurrent offerings credited %d times", n-4)
	}
}

func TestFortuneOncePerDayAndBuffApplies(t *testing.T) {
	e := setup(t)
	e.fortune.RNG = fixedRNG(0)              // roll 0 → first outcome (Đại Cát)
	bp := e.cb.Fortunes.All()[0].RateBonusBP // tuned in fortunes.yaml
	if bp <= 0 {
		t.Fatal("first fortune outcome must grant a bonus")
	}
	buffed := func(hours int64) int64 { return hours*2083 + hours*2083*bp/10_000 }
	v, err := e.fortune.View(e.ctx, e.user)
	if err != nil || v.Today != nil || v.Buff != nil || len(v.Outcomes) == 0 {
		t.Fatalf("before draw: %+v %v", v, err)
	}
	d, err := e.fortune.Draw(e.ctx, e.user)
	if err != nil || d.Today == nil || d.Today.Outcome.ID != "dai_cat" || d.Buff == nil || d.Buff.BonusBP != bp || d.Replayed {
		t.Fatalf("draw: %+v %v", d, err)
	}
	e.fortune.RNG = fixedRNG(9999)
	again, err := e.fortune.Draw(e.ctx, e.user)
	if err != nil || !again.Replayed || again.Today.Outcome.ID != "dai_cat" {
		t.Fatalf("second draw same day: %+v %v", again, err)
	}
	if n := e.scalar(t, "SELECT COUNT(*) FROM daily_fortunes WHERE character_id=$1", e.char); n != 1 {
		t.Fatalf("draws %d", n)
	}
	// 2 h later: idle = 2 × 2083 × (1 + bp) (the buff covers the whole window).
	*e.clock = e.clock.Add(2 * time.Hour)
	st, err := e.cult.Settle(e.ctx, e.user, "settle-buff-01")
	if err != nil {
		t.Fatal(err)
	}
	if want := buffed(2); st.RawMilli < want || st.RawMilli > want+1 { // +1: sub-milli carry
		t.Fatalf("buffed raw %d, want %d", st.RawMilli, want)
	}
	// Next game day: buff expired at midnight ICT; a new draw is allowed.
	*e.clock = time.Date(2026, 6, 2, 1, 0, 0, 0, time.UTC) // 08:00 ICT next day
	st, err = e.cult.Settle(e.ctx, e.user, "settle-buff-02")
	if err != nil {
		t.Fatal(err)
	}
	// Window 8 h from 10:00 ICT (eligible = first 8 h, cap) → 10:00–18:00, fully before midnight: fully buffed.
	if want := buffed(8); st.RawMilli < want || st.RawMilli > want+1 { // +1: sub-milli carry
		t.Fatalf("capped buffed raw %d, want %d", st.RawMilli, want)
	}
	e.fortune.RNG = fixedRNG(9999) // last outcome (Bình, +0%)
	d, err = e.fortune.Draw(e.ctx, e.user)
	if err != nil || d.Replayed || d.Today.Outcome.ID != "binh" || d.Buff != nil {
		t.Fatalf("next day draw: %+v %v", d, err)
	}
	*e.clock = e.clock.Add(time.Hour)
	if st, err = e.cult.Settle(e.ctx, e.user, "settle-buff-03"); err != nil || st.RawMilli != 2083 {
		t.Fatalf("expired buff still applied: %+v %v", st.RawMilli, err)
	}
}
