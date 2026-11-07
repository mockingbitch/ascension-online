//go:build integration

package world_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"tiendao/game/internal/content"
	"tiendao/game/internal/platform/testdb"
	"tiendao/game/internal/world"
)

// Phase 2.1 "xong khi": a test server driven by a fake clock passes ≥ 2 milestones — one by threshold, one by the
// 150% ceiling — idempotently, ends a ranged event (on_end era), and honours admin force / pause.
func TestTickReachesMilestones(t *testing.T) {
	ctx := context.Background()
	pool := testdb.New(t)
	cb, err := content.LoadAll("../../../../content/catalog")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2027, 1, 10, 1, 0, 0, 0, time.UTC) // 08:00 ICT, launch game day 2027-01-10
	now := start
	svc := &world.Service{Pool: pool, Realms: cb.Realms, Cat: cb.World, Contribution: cb.Contribution,
		Log: slog.New(slog.DiscardHandler), Now: func() time.Time { return now }} // fixture has no cultivation_credits (C3 noise)
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	scalar := func(sql string, dst any, args ...any) {
		t.Helper()
		if err := pool.QueryRow(ctx, sql, args...).Scan(dst); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	tick := func(at time.Time, wantReached string) world.TickResult {
		t.Helper()
		now = at
		res, err := svc.Tick(ctx)
		if err != nil || !res.Ran {
			t.Fatalf("tick at %v: %+v %v", at, res, err)
		}
		if res.Reached != wantReached {
			t.Fatalf("tick at day %.2f reached %q, want %q", at.Sub(start).Hours()/24, res.Reached, wantReached)
		}
		return res
	}
	reason := func(id string) string {
		t.Helper()
		var r string
		scalar("SELECT reason FROM world_milestones_reached WHERE milestone_id=$1", &r, id)
		return r
	}
	era := func() string {
		t.Helper()
		var e *string
		scalar("SELECT era_id FROM world_state", &e)
		if e == nil {
			return ""
		}
		return *e
	}
	countTopic := func(topic string) int {
		t.Helper()
		var n int
		scalar("SELECT count(*) FROM outbox_events WHERE topic=$1", &n, topic)
		return n
	}
	at := func(days float64) time.Time { return start.Add(time.Duration(days * float64(24*time.Hour))) }

	// Pre-launch: nothing happens.
	tick(start, "")
	admin, ch := uuid.New(), uuid.New()
	exec("INSERT INTO users (id, email, role) VALUES ($1, 'admin@t', 'admin')", admin)
	exec("INSERT INTO characters (id, user_id, name) VALUES ($1, $2, 'Thử Nghiệm')", ch, admin)
	exec("INSERT INTO character_progressions (character_id, realm_id, stage_id) VALUES ($1, 'luyen_khi', 'luyen_khi.tang_1')", ch)
	// One active player crediting the 100-point world cap every day for 22 game days (2027-01-10 … 2027-01-31).
	exec(`INSERT INTO daily_contribution (character_id, game_day, idle_milli)
	      SELECT $1, d::date, 100000 FROM generate_series('2027-01-10'::date, '2027-01-31'::date, '1 day') d`, ch)
	if _, err := svc.Start(ctx, admin); err != nil {
		t.Fatal(err)
	}

	// Day 0: the launch milestone (threshold 0) and its era.
	tick(at(0.001), "khai_giang")
	if reason("khai_giang") != "threshold" || era() != "khai_giang" {
		t.Fatalf("khai_giang %s era %s", reason("khai_giang"), era())
	}
	// Day 20: progress 2,000 < 2,010 and before the 70% floor (day 21) — nothing.
	tick(at(20.5), "")
	// Day 21 + 1h: 21 closed days × 100 = 2,100 ≥ 2,010 and floor passed → giang_dao_2 by threshold.
	tick(at(21.05), "giang_dao_2")
	if reason("giang_dao_2") != "threshold" {
		t.Fatalf("giang_dao_2 reason %s", reason("giang_dao_2"))
	}
	var progress, thr int64
	var day int32
	if err := pool.QueryRow(ctx, "SELECT progress_milli, threshold_milli, server_day FROM world_milestones_reached WHERE milestone_id='giang_dao_2'").
		Scan(&progress, &thr, &day); err != nil || progress != 2_100_000 || thr != 2_010_000 || day != 21 {
		t.Fatalf("giang_dao_2 row %d %d %d %v", progress, thr, day, err)
	}
	// Idempotent re-run at the same instant: nothing new.
	tick(at(21.05), "")
	var reached, events int
	scalar("SELECT count(*) FROM world_milestones_reached", &reached)
	scalar("SELECT count(*) FROM world_events", &events)
	if reached != 2 || events != 1 || countTopic(world.TopicMilestoneReached) != 2 {
		t.Fatalf("after replay: reached %d events %d outbox %d", reached, events, countTopic(world.TopicMilestoneReached))
	}

	// Day 23: the 1-day giang_dao_2 event has ended (once).
	if res := tick(at(23), ""); res.EventsEnded != 1 {
		t.Fatalf("events ended %d", res.EventsEnded)
	}
	if res := tick(at(23.1), ""); res.EventsEnded != 0 || countTopic(world.TopicEventEnded) != 1 {
		t.Fatalf("event ended twice: %+v", res)
	}

	// Day 90 = 150% of giang_dao_3's day 60: reached by max_time although progress (2,200) < 4,020.
	tick(at(89.99), "")
	tick(at(90), "giang_dao_3")
	if reason("giang_dao_3") != "max_time" {
		t.Fatalf("giang_dao_3 reason %s", reason("giang_dao_3"))
	}

	// Day 150 = 150% of dai_chien: era dai_chien; its 15-day event ends on day 165 → on_end era luan_hoi.
	tick(at(150), "dai_chien")
	if era() != "dai_chien" {
		t.Fatalf("era %s", era())
	}
	if res := tick(at(165.01), ""); res.EventsEnded != 1 || era() != "luan_hoi" {
		t.Fatalf("dai_chien end: %+v era %s", res, era())
	}
	// Entering luan_hoi schedules the on_close: split of thuong_lan_dinh (decisions 2026-11-07) as a pending effect
	// of dai_chien (the event whose on_end moved the era), with the milestone-effect lifecycle: due +3 d, deadline +10 d.
	checkCloseSplit(t, ctx, pool, "dai_chien", at(165.01))

	// Admin: overriding a reached milestone is refused; forcing nu_oa reaches it on the next tick (reason admin).
	if _, err := svc.SetOverride(ctx, admin, "dai_chien", world.OverrideInput{ForceReach: true, Reason: "x"}); !errors.Is(err, world.ErrMilestoneReached) {
		t.Fatalf("override reached: %v", err)
	}
	if _, err := svc.SetOverride(ctx, admin, "nu_oa", world.OverrideInput{ForceReach: true}); !errors.Is(err, world.ErrInvalidOverride) {
		t.Fatalf("override without reason: %v", err)
	}
	if _, err := svc.SetOverride(ctx, admin, "nu_oa", world.OverrideInput{ForceReach: true, Reason: "sự kiện ra mắt"}); err != nil {
		t.Fatal(err)
	}
	tick(at(166), "nu_oa")
	var adminID uuid.UUID
	scalar("SELECT admin_user_id FROM world_milestones_reached WHERE milestone_id='nu_oa'", &adminID)
	if reason("nu_oa") != "admin" || adminID != admin || era() != "tao_nhan" {
		t.Fatalf("nu_oa %s %v era %s", reason("nu_oa"), adminID, era())
	}
	// Faction effects of thanh_nhan are stored pending (Phase 2.2 executes them).
	if _, err := svc.SetOverride(ctx, admin, "thanh_nhan", world.OverrideInput{ForceReach: true, Reason: "thử"}); err != nil {
		t.Fatal(err)
	}
	tick(at(166.1), "thanh_nhan")
	var pending, applied int
	scalar("SELECT count(*) FILTER (WHERE applied_at IS NULL) FROM world_effects WHERE milestone_id='thanh_nhan'", &pending)
	scalar("SELECT count(*) FILTER (WHERE applied_at IS NOT NULL) FROM world_effects WHERE milestone_id='thanh_nhan'", &applied)
	if pending != 2 || applied != 2 {
		t.Fatalf("thanh_nhan effects pending %d applied %d", pending, applied)
	}

	// Pause holds even a forced milestone; resume releases it. Both audited.
	if _, err := svc.SetPaused(ctx, admin, true, "sự cố"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetOverride(ctx, admin, "nhan_toc", world.OverrideInput{ForceReach: true, Reason: "thử"}); err != nil {
		t.Fatal(err)
	}
	tick(at(167), "")
	v, err := svc.View(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !v.Paused || v.Era == nil || v.Era.ID != "thanh_nhan" || v.NextMilestone == nil || v.NextMilestone.ID != "nhan_toc" ||
		len(v.PendingEffects) != 3 || len(v.ActiveEvents) != 1 || v.ActiveEvents[0].ID != "thanh_nhan" || v.LastMilestone.ID != "thanh_nhan" ||
		v.ServerDay != 167 || len(v.Timeline) != 10 || !v.Timeline[5].Reached || v.Timeline[6].Reached {
		t.Fatalf("view %+v", v)
	}
	if _, err := svc.SetPaused(ctx, admin, false, "xong"); err != nil {
		t.Fatal(err)
	}
	tick(at(167.1), "nhan_toc")
	var audits int
	scalar("SELECT count(*) FROM admin_audit_logs WHERE action IN ('world.milestone_override', 'world.pause', 'world.resume')", &audits)
	if audits != 5 || countTopic(world.TopicMilestoneReached) != 7 || countTopic(world.TopicEventEnded) != 3 { // giang_dao_2, giang_dao_3, dai_chien
		t.Fatalf("audits %d reached outbox %d ended outbox %d", audits, countTopic(world.TopicMilestoneReached), countTopic(world.TopicEventEnded))
	}
	av, err := svc.AdminView(ctx)
	if err != nil || len(av.Totals) != 14 || len(av.Overrides) != 3 || len(av.Reached) != 7 {
		t.Fatalf("admin view totals %d overrides %d reached %d %v", len(av.Totals), len(av.Overrides), len(av.Reached), err)
	}
}

// checkCloseSplit asserts the single pending split_faction row of thuong_lan_dinh (on_close: split, computed targets).
func checkCloseSplit(t *testing.T, ctx context.Context, pool *pgxpool.Pool, milestoneID string, at time.Time) {
	t.Helper()
	var (
		n        int
		index    int16
		due      time.Time
		deadline *time.Time
		applied  *time.Time
		snap     []byte
		owner    string
	)
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM world_effects WHERE kind='split_faction' AND snapshot->>'from'='thuong_lan_dinh'").Scan(&n); err != nil || n != 1 {
		t.Fatalf("thuong_lan_dinh split rows %d %v", n, err)
	}
	if err := pool.QueryRow(ctx, `SELECT milestone_id, effect_index, due_at, deadline_at, applied_at, snapshot FROM world_effects
	                              WHERE kind='split_faction' AND snapshot->>'from'='thuong_lan_dinh'`).
		Scan(&owner, &index, &due, &deadline, &applied, &snap); err != nil {
		t.Fatal(err)
	}
	var e world.Effect
	if err := json.Unmarshal(snap, &e); err != nil {
		t.Fatal(err)
	}
	day := 24 * time.Hour
	if owner != milestoneID || index < world.EraEffectIndexBase || applied != nil || !due.Equal(at.Add(3*day)) || deadline == nil ||
		!deadline.Equal(at.Add(10*day)) || e.NoticeDays != 3 || e.ChooseDays != 7 ||
		!slices.Equal(e.Into, []string{"huyen_khung_mon", "kim_sa_mon", "tinh_ha_yeu_dinh"}) {
		t.Fatalf("close split row: %s[%d] due %v deadline %v applied %v %+v", owner, index, due, deadline, applied, e)
	}
}

// An era that SKIPS luan_hoi (nu_oa reached before the 15-day dai_chien event ends: no minimum gap, decision H) still
// schedules the thuong_lan_dinh split — under nu_oa — and the late on_end neither moves the era back nor schedules it
// twice.
func TestEraSkipStillSchedulesCloseSplit(t *testing.T) {
	ctx := context.Background()
	pool := testdb.New(t)
	cb, err := content.LoadAll("../../../../content/catalog")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2027, 1, 10, 1, 0, 0, 0, time.UTC)
	now := start
	svc := &world.Service{Pool: pool, Realms: cb.Realms, Cat: cb.World, Contribution: cb.Contribution,
		Log: slog.New(slog.DiscardHandler), Now: func() time.Time { return now }}
	admin := uuid.New()
	if _, err := pool.Exec(ctx, "INSERT INTO users (id, email, role) VALUES ($1, 'admin@t', 'admin')", admin); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Start(ctx, admin); err != nil {
		t.Fatal(err)
	}
	tick := func(at time.Time, want string) {
		t.Helper()
		now = at
		res, err := svc.Tick(ctx)
		if err != nil || res.Reached != want {
			t.Fatalf("tick: %+v %v, want %q", res, err, want)
		}
	}
	tick(start.Add(time.Minute), "khai_giang")
	for i, id := range []string{"giang_dao_2", "giang_dao_3", "dai_chien", "nu_oa"} {
		if _, err := svc.SetOverride(ctx, admin, id, world.OverrideInput{ForceReach: true, Reason: "thử"}); err != nil {
			t.Fatal(err)
		}
		tick(start.Add(time.Duration(i+2)*time.Hour), id)
	}
	var era string
	if err := pool.QueryRow(ctx, "SELECT era_id FROM world_state").Scan(&era); err != nil || era != "tao_nhan" {
		t.Fatalf("era %s %v", era, err)
	}
	checkCloseSplit(t, ctx, pool, "nu_oa", start.Add(5*time.Hour))
	// Day 16: dai_chien ends; its on_end (luan_hoi) is behind the current era → skipped, nothing rescheduled.
	tick(start.Add(16*24*time.Hour), "")
	if err := pool.QueryRow(ctx, "SELECT era_id FROM world_state").Scan(&era); err != nil || era != "tao_nhan" {
		t.Fatalf("era moved back to %s %v", era, err)
	}
	checkCloseSplit(t, ctx, pool, "nu_oa", start.Add(5*time.Hour))
}
