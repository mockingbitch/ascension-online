//go:build integration

package world_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"tiendao/game/internal/content"
	"tiendao/game/internal/platform/testdb"
	"tiendao/game/internal/world"
)

func TestTickTotalsCloseMedianFold(t *testing.T) {
	ctx := context.Background()
	pool := testdb.New(t)
	cb, err := content.LoadAll("../../../../content/catalog")
	if err != nil {
		t.Fatal(err)
	}
	// 2026-06-02 00:10 game time: 2026-06-01 is closable (grace 5 min).
	now := time.Date(2026, 6, 1, 17, 10, 0, 0, time.UTC)
	svc := &world.Service{Pool: pool, Realms: cb.Realms, Cat: cb.World, Contribution: cb.Contribution, Now: func() time.Time { return now }}
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	exec("UPDATE world_state SET started_at=$1", time.Date(2026, 5, 20, 1, 0, 0, 0, time.UTC))
	// Three characters: (points, ×2 bonus, stage, stored) on 2026-06-01.
	for _, c := range []struct {
		points, bonus int64
		stage         string
		stored        int64
	}{{150_000, 0, "kim_dan.so_ky", 0}, {80_000, 40_000, "luyen_khi.tang_2", 0}, {20_000, 0, "truc_co.trung_ky", 1_000}} {
		u, ch := uuid.New(), uuid.New()
		exec("INSERT INTO users (id, email) VALUES ($1, $2)", u, u.String()+"@t")
		exec("INSERT INTO characters (id, user_id, name) VALUES ($1, $2, $3)", ch, u, "N"+ch.String()[:8])
		exec("INSERT INTO character_progressions (character_id, realm_id, stage_id, cultivation_milli) VALUES ($1, split_part($2,'.',1), $2, $3)", ch, c.stage, c.stored)
		exec("INSERT INTO daily_contribution (character_id, game_day, idle_milli, catch_up_bonus_milli) VALUES ($1, '2026-06-01', $2, $3)", ch, c.points, c.bonus)
	}
	res, err := svc.Tick(ctx)
	if err != nil || !res.Ran {
		t.Fatalf("tick %+v %v", res, err)
	}
	// The fixture writes daily_contribution without cultivation_credits: the C3 audit must flag all 3.
	if res.Violations != 3 {
		t.Fatalf("audit violations %d, want 3", res.Violations)
	}
	var credited, countable, perCapita, median int64
	var dau int32
	var closed *time.Time
	if err := pool.QueryRow(ctx, "SELECT credited_milli, countable_milli, dau, per_capita_milli, median_position_milli, closed_at FROM world_daily_totals WHERE game_day='2026-06-01'").
		Scan(&credited, &countable, &dau, &perCapita, &median, &closed); err != nil {
		t.Fatal(err)
	}
	// countable = min(100, 150) + min(100, 80−40) + 20 = 160; per capita 53.333.
	truc, _ := cb.Realms.Stage("truc_co.trung_ky")
	if credited != 250_000 || countable != 160_000 || dau != 3 || perCapita != 53_333 || closed == nil || median != truc.CumulativeBeforeMilli+1_000 {
		t.Fatalf("totals %d %d %d %d median %d closed %v", credited, countable, dau, perCapita, median, closed)
	}
	var progress, wsMedian int64
	if err := pool.QueryRow(ctx, "SELECT progress_milli, median_position_milli FROM world_state").Scan(&progress, &wsMedian); err != nil {
		t.Fatal(err)
	}
	if progress != 53_333 || wsMedian != median {
		t.Fatalf("world_state progress %d median %d", progress, wsMedian)
	}
	// Replay-safe: a second tick neither changes the closed day nor folds it twice.
	exec("UPDATE daily_contribution SET idle_milli = idle_milli + 1000 WHERE game_day='2026-06-01'")
	if _, err := svc.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "SELECT progress_milli FROM world_state").Scan(&progress); err != nil || progress != 53_333 {
		t.Fatalf("second tick progress %d %v", progress, err)
	}
}

// Worker down for several days: every missing day gets a (DAU 0) row, is closed in order and folded once.
func TestTickCatchesUpMissingDays(t *testing.T) {
	ctx := context.Background()
	pool := testdb.New(t)
	cb, err := content.LoadAll("../../../../content/catalog")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 6, 1, 17, 10, 0, 0, time.UTC) // 2026-06-02 00:10 game time
	svc := &world.Service{Pool: pool, Realms: cb.Realms, Cat: cb.World, Contribution: cb.Contribution, Now: func() time.Time { return now }}
	if _, err := pool.Exec(ctx, "UPDATE world_state SET started_at=$1, closed_through_day='2026-05-27'", time.Date(2026, 5, 20, 1, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	res, err := svc.Tick(ctx)
	if err != nil || res.Recomputed != 6 || res.Closed != 5 || res.Violations != 0 {
		t.Fatalf("tick %+v %v", res, err)
	}
	var rows, closed int
	if err := pool.QueryRow(ctx, "SELECT count(*), count(closed_at) FROM world_daily_totals WHERE dau = 0").Scan(&rows, &closed); err != nil {
		t.Fatal(err)
	}
	if rows != 6 || closed != 5 {
		t.Fatalf("rows %d closed %d", rows, closed)
	}
	var through time.Time
	if err := pool.QueryRow(ctx, "SELECT closed_through_day FROM world_state").Scan(&through); err != nil || through.Format(time.DateOnly) != "2026-06-01" {
		t.Fatalf("closed_through_day %v %v", through, err)
	}
}
