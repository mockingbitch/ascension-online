//go:build integration

package dailytask_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"tiendao/game/internal/character"
	"tiendao/game/internal/content"
	"tiendao/game/internal/cultivation"
	"tiendao/game/internal/dailytask"
	"tiendao/game/internal/platform/testdb"
	"tiendao/game/internal/vip"
)

type world struct {
	pool  *pgxpool.Pool
	cb    *content.Bundle
	clock *time.Time
	cult  *cultivation.Service
	tasks *dailytask.Service
	vip   *vip.Service
	user  uuid.UUID
	char  uuid.UUID
}

func newWorld(t *testing.T) *world {
	t.Helper()
	ctx := context.Background()
	pool := testdb.New(t)
	cb, err := content.LoadAll("../../../../content/catalog")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 6, 1, 3, 0, 0, 0, time.UTC) // 10:00 ICT
	w := &world{pool: pool, cb: cb, clock: &now}
	nowFn := func() time.Time { return *w.clock }
	w.cult = &cultivation.Service{Pool: pool, Realms: cb.Realms, Contribution: cb.Contribution, VIP: cb.VIP, Tasks: cb.DailyTasks, Now: nowFn}
	w.tasks = &dailytask.Service{Pool: pool, Cat: cb.DailyTasks, VIP: cb.VIP, Now: nowFn}
	w.vip = &vip.Service{Pool: pool, Table: cb.VIP, Now: nowFn}
	w.user = uuid.New()
	if _, err := pool.Exec(ctx, "INSERT INTO users (id, email) VALUES ($1, $2)", w.user, w.user.String()+"@t"); err != nil {
		t.Fatal(err)
	}
	v, err := (&character.Service{Pool: pool, Cultivation: w.cult}).Create(ctx, w.user, "Vân Du")
	if err != nil {
		t.Fatal(err)
	}
	w.char = v.ID
	return w
}

func (w *world) advance(d time.Duration) { *w.clock = w.clock.Add(d) }

func (w *world) scalar(t *testing.T, sql string, args ...any) (n int64) {
	t.Helper()
	if err := w.pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func (w *world) phu(t *testing.T) int64 {
	return w.scalar(t, "SELECT COALESCE((SELECT quantity FROM inventory_items WHERE character_id=$1 AND item_id='tu_linh_phu'),0)", w.char)
}

func TestCheckinGivesPhuOncePerDay(t *testing.T) {
	w, ctx := newWorld(t), context.Background()
	if _, err := w.tasks.Claim(ctx, w.user, "diem_danh"); err != nil {
		t.Fatal(err)
	}
	if _, err := w.tasks.Claim(ctx, w.user, "diem_danh"); !errors.Is(err, dailytask.ErrAlreadyClaimed) {
		t.Fatalf("second claim: %v", err)
	}
	if w.phu(t) != 1 || w.scalar(t, "SELECT balance FROM currency_accounts WHERE character_id=$1 AND currency='spirit_stone'", w.char) != 100 {
		t.Fatal("rewards not granted")
	}
	w.advance(24 * time.Hour) // next game day
	if _, err := w.tasks.Claim(ctx, w.user, "diem_danh"); err != nil {
		t.Fatalf("next day claim: %v", err)
	}
	if w.phu(t) != 2 {
		t.Fatal("next day reward missing")
	}
}

func TestSettleTaskCountsOnlySpacedSettlements(t *testing.T) {
	w, ctx := newWorld(t), context.Background()
	for i, gap := range []time.Duration{2 * time.Hour, 10 * time.Second, 70 * time.Minute, 61 * time.Minute} {
		w.advance(gap)
		if _, err := w.cult.Settle(ctx, w.user, "settle-key-0000"+string(rune('a'+i))); err != nil {
			t.Fatal(err)
		}
	}
	b, err := w.tasks.Board(ctx, w.user)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range b.Tasks {
		if v.ID == "van_cong" && (v.Progress != 3 || !v.Complete) {
			t.Fatalf("van_cong progress %d", v.Progress) // the 10s settlement must not count
		}
	}
	if _, err := w.tasks.Claim(ctx, w.user, "nghe_giang"); !errors.Is(err, dailytask.ErrNotComplete) {
		t.Fatalf("lecture task should be incomplete: %v", err)
	}
}

func TestTuLinhPhuExtendsOneSettlementAndOncePerDay(t *testing.T) {
	w, ctx := newWorld(t), context.Background()
	if _, err := w.pool.Exec(ctx, "INSERT INTO inventory_items VALUES ($1,'tu_linh_phu',3)", w.char); err != nil {
		t.Fatal(err)
	}
	// Move to Trúc Cơ so stages are large enough not to hit the bottleneck in 12h.
	if _, err := w.pool.Exec(ctx, "UPDATE character_progressions SET realm_id='truc_co', stage_id='truc_co.so_ky', last_cultivated_at=$2 WHERE character_id=$1", w.char, *w.clock); err != nil {
		t.Fatal(err)
	}
	st, err := w.cult.UseOfflineBonusItem(ctx, w.cb.Items, w.user, "tu_linh_phu", "use-phu-key-0001")
	if err != nil || st.Remaining != 2 || st.OfflineBonusSeconds != 4*3600 {
		t.Fatalf("use: %+v %v", st, err)
	}
	if again, err := w.cult.UseOfflineBonusItem(ctx, w.cb.Items, w.user, "tu_linh_phu", "use-phu-key-0001"); err != nil || !again.Replayed || again.Remaining != 2 {
		t.Fatalf("replay: %+v %v", again, err)
	}
	if _, err := w.cult.UseOfflineBonusItem(ctx, w.cb.Items, w.user, "tu_linh_phu", "use-phu-key-0002"); !errors.Is(err, cultivation.ErrBonusUsedToday) {
		t.Fatalf("second use same day: %v", err)
	}
	// A short settlement does not spend the bonus.
	w.advance(time.Hour)
	if _, err := w.cult.Settle(ctx, w.user, "phu-settle-0001"); err != nil {
		t.Fatal(err)
	}
	if w.scalar(t, "SELECT offline_bonus_seconds FROM character_progressions WHERE character_id=$1", w.char) == 0 {
		t.Fatal("bonus spent by a settlement it did not extend")
	}
	// 14h away: free cap 8h + 4h bonus = 12h eligible, then the bonus is spent.
	w.advance(14 * time.Hour)
	r, err := w.cult.Settle(ctx, w.user, "phu-settle-0002")
	if err != nil {
		t.Fatal(err)
	}
	if want := w.cb.Contribution.IdleRateMilliPerHour * 12; r.EarnedMilli != want {
		t.Fatalf("earned %d, want %d (12h)", r.EarnedMilli, want)
	}
	if w.scalar(t, "SELECT offline_bonus_seconds FROM character_progressions WHERE character_id=$1", w.char) != 0 {
		t.Fatal("bonus not spent")
	}
}

func TestVIPDailyGift(t *testing.T) {
	w, ctx := newWorld(t), context.Background()
	if _, err := w.vip.ClaimDailyGift(ctx, w.user, *w.clock); !errors.Is(err, vip.ErrNoGift) {
		t.Fatalf("VIP0 gift: %v", err)
	}
	if _, err := w.vip.Grant(ctx, nil, w.user, 150, "promo", "", "", "grant-key-0001"); err != nil { // → Tiên Duyên 2
		t.Fatal(err)
	}
	g, err := w.vip.ClaimDailyGift(ctx, w.user, *w.clock)
	if err != nil || g.Amount != 50 || g.Balance != 50 {
		t.Fatalf("gift %+v %v", g, err)
	}
	if _, err := w.vip.ClaimDailyGift(ctx, w.user, *w.clock); !errors.Is(err, vip.ErrGiftClaimed) {
		t.Fatalf("second gift: %v", err)
	}
	st, _ := w.vip.Status(ctx, w.user)
	if !st.GiftClaimedToday {
		t.Fatal("status should show claimed")
	}
}

// Decision D: each task claim pays daily_task_claim điểm duyên once; online settles ≥ 1 h apart pay
// online_cultivation at most daily_max_times per game day.
func TestTaskClaimAndSettleEarnDiemDuyen(t *testing.T) {
	w, ctx := newWorld(t), context.Background()
	duyen := func(activity string) int64 {
		return w.scalar(t, "SELECT COALESCE(SUM(delta),0) FROM vip_point_ledger WHERE user_id=$1 AND activity_id=$2", w.user, activity)
	}
	v, err := w.tasks.Claim(ctx, w.user, "diem_danh")
	if err != nil || v.VIPPoints != 1 {
		t.Fatalf("claim: %+v %v", v, err)
	}
	if _, err := w.tasks.Claim(ctx, w.user, "diem_danh"); !errors.Is(err, dailytask.ErrAlreadyClaimed) {
		t.Fatalf("second claim: %v", err)
	}
	if got := duyen(vip.ActDailyTask); got != 1 {
		t.Fatalf("task điểm duyên %d", got)
	}
	// 5 settles: 30 min (too soon), then four ≥ 1 h apart → capped at 3.
	for i, gap := range []time.Duration{30 * time.Minute, 2 * time.Hour, 70 * time.Minute, 61 * time.Minute, 3 * time.Hour} {
		w.advance(gap)
		if _, err := w.cult.Settle(ctx, w.user, "earn-settle-"+string(rune('a'+i))); err != nil {
			t.Fatal(err)
		}
	}
	// Replaying a settle key changes nothing.
	if _, err := w.cult.Settle(ctx, w.user, "earn-settle-b"); err != nil {
		t.Fatal(err)
	}
	if got := duyen(vip.ActOnlineCultivation); got != 3 {
		t.Fatalf("online cultivation điểm duyên %d, want 3", got)
	}
	st, err := w.vip.Status(ctx, w.user)
	if err != nil || st.Points != 4 || st.EarnedToday != 4 {
		t.Fatalf("status %+v %v", st, err)
	}
}
