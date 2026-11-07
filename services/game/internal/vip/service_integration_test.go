//go:build integration

package vip_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"tiendao/game/internal/content"
	"tiendao/game/internal/db"
	"tiendao/game/internal/platform/postgres"
	"tiendao/game/internal/platform/testdb"
	"tiendao/game/internal/vip"
)

type fixture struct {
	pool  *pgxpool.Pool
	svc   *vip.Service
	clock *time.Time
	user  uuid.UUID
	admin uuid.UUID
}

func setup(t *testing.T) *fixture {
	t.Helper()
	pool := testdb.New(t)
	cb, err := content.LoadAll("../../../../content/catalog")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 7, 3, 0, 0, 0, time.UTC) // 10:00 ICT
	f := &fixture{pool: pool, clock: &now, user: uuid.New(), admin: uuid.New()}
	f.svc = &vip.Service{Pool: pool, Table: cb.VIP, Now: func() time.Time { return *f.clock }}
	for _, id := range []uuid.UUID{f.user, f.admin} {
		if _, err := pool.Exec(context.Background(), "INSERT INTO users (id, email) VALUES ($1, $2)", id, id.String()+"@t"); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func (f *fixture) earn(t *testing.T, activity, key string) vip.Earned {
	t.Helper()
	var out vip.Earned
	err := postgres.InTx(context.Background(), f.pool, func(tx pgx.Tx) error {
		var err error
		out, err = f.svc.Table.EarnLocked(context.Background(), db.New(tx), f.user, activity, vip.Ref{Type: "test", ID: key, IdemKey: key}, *f.clock)
		return err
	})
	if err != nil {
		t.Fatalf("earn %s/%s: %v", activity, key, err)
	}
	return out
}

func (f *fixture) scalar(t *testing.T, sql string, args ...any) (n int64) {
	t.Helper()
	if err := f.pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// ledgerMatches checks the account balance equals the ledger sum (every change wrote exactly one row).
func (f *fixture) ledgerMatches(t *testing.T) int64 {
	t.Helper()
	pts := f.scalar(t, "SELECT points FROM vip_accounts WHERE user_id=$1", f.user)
	if sum := f.scalar(t, "SELECT COALESCE(SUM(delta),0) FROM vip_point_ledger WHERE user_id=$1", f.user); sum != pts {
		t.Fatalf("ledger sum %d != balance %d", sum, pts)
	}
	return pts
}

func TestEarnIdempotentAndCapped(t *testing.T) {
	f, ctx := setup(t), context.Background()
	if e := f.earn(t, vip.ActLecture, "lec-1"); e.Points != 3 || e.Replayed {
		t.Fatalf("first: %+v", e)
	}
	if e := f.earn(t, vip.ActLecture, "lec-1"); e.Points != 3 || !e.Replayed { // same key: replay, nothing written
		t.Fatalf("replay: %+v", e)
	}
	f.earn(t, vip.ActLecture, "lec-2")
	if e := f.earn(t, vip.ActLecture, "lec-3"); e.Points != 0 { // daily_max_times 2
		t.Fatalf("third lecture paid %+v", e)
	}
	// Same key, different activity → refused.
	err := postgres.InTx(ctx, f.pool, func(tx pgx.Tx) error {
		_, err := f.svc.Table.EarnLocked(ctx, db.New(tx), f.user, vip.ActDailyTask, vip.Ref{IdemKey: "lec-1"}, *f.clock)
		return err
	})
	if !errors.Is(err, vip.ErrKeyReuse) {
		t.Fatalf("key reuse: %v", err)
	}
	// Fill the day: 6 (lecture) + 4 (tasks) + 3 (online) = 13 ≤ 20 → all paid; nothing more for these activities.
	for i, a := range []string{vip.ActDailyTask, vip.ActDailyTask, vip.ActDailyTask, vip.ActDailyTask, vip.ActDailyTask,
		vip.ActOnlineCultivation, vip.ActOnlineCultivation, vip.ActOnlineCultivation, vip.ActOnlineCultivation} {
		f.earn(t, a, "k-"+string(rune('a'+i)))
	}
	if pts := f.ledgerMatches(t); pts != 13 {
		t.Fatalf("day total %d, want 13", pts)
	}
	// Concurrent duplicates of one key serialize on the account lock: one row.
	var wg sync.WaitGroup
	f.clock2(24 * time.Hour)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := postgres.InTx(ctx, f.pool, func(tx pgx.Tx) error {
				_, err := f.svc.Table.EarnLocked(ctx, db.New(tx), f.user, vip.ActLecture, vip.Ref{IdemKey: "race-key"}, *f.clock)
				return err
			}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if n := f.scalar(t, "SELECT COUNT(*) FROM vip_point_ledger WHERE user_id=$1 AND idempotency_key='race-key'", f.user); n != 1 {
		t.Fatalf("race rows %d", n)
	}
	st, err := f.svc.Status(ctx, f.user)
	if err != nil || st.EarnedToday != 3 || st.Points != 16 || st.Benefits.Tier != 0 || st.DisplayName != "Tiên Duyên" {
		t.Fatalf("status next day: %+v %v", st, err)
	}
	f.ledgerMatches(t)
}

func (f *fixture) clock2(d time.Duration) { *f.clock = f.clock.Add(d) }

func TestTopupRefundIdempotentAndLevelDrops(t *testing.T) {
	f, ctx := setup(t), context.Background()
	f.earn(t, vip.ActLecture, "lec-1") // 3 in-game points, never touched by the refund
	st, err := f.svc.Topup(ctx, f.user, 50_000, "order-1")
	if err != nil || st.Points != 503 || st.Benefits.Tier != 3 {
		t.Fatalf("topup: %+v %v", st, err)
	}
	if st, err := f.svc.Topup(ctx, f.user, 50_000, "order-1"); err != nil || st.Points != 503 { // replay
		t.Fatalf("topup replay: %+v %v", st, err)
	}
	if _, err := f.svc.Topup(ctx, f.user, 20_000, "order-1"); !errors.Is(err, vip.ErrOrderConflict) {
		t.Fatalf("same order, other amount: %v", err)
	}
	purchase := f.scalar(t, "SELECT id FROM vip_point_ledger WHERE user_id=$1 AND reason='purchase'", f.user)
	lecture := f.scalar(t, "SELECT id FROM vip_point_ledger WHERE user_id=$1 AND reason='activity'", f.user)

	if _, _, err := f.svc.Refund(ctx, f.admin, lecture, "x", "refund-0000"); !errors.Is(err, vip.ErrNotRefundable) {
		t.Fatalf("refund of activity row: %v", err)
	}
	if _, _, err := f.svc.Refund(ctx, f.admin, 1<<40, "x", "refund-0000"); !errors.Is(err, vip.ErrLedgerNotFound) {
		t.Fatalf("refund of missing row: %v", err)
	}
	r, st, err := f.svc.Refund(ctx, f.admin, purchase, "chargeback", "refund-0001")
	if err != nil || r.PointsRemoved != 500 || r.PointsBefore != 503 || r.PointsAfter != 3 || r.Replayed {
		t.Fatalf("refund: %+v %v", r, err)
	}
	if st.Points != 3 || st.Benefits.Tier != 0 { // level dropped from 3 to 0
		t.Fatalf("status after refund: %+v", st)
	}
	firstRefund := r.RefundLedgerID
	f.earn(t, vip.ActLecture, "lec-2") // balance moves on; the replay must still report the stored refund
	if r, _, err := f.svc.Refund(ctx, f.admin, purchase, "chargeback", "refund-0001"); err != nil || !r.Replayed ||
		r.PointsBefore != 503 || r.PointsAfter != 3 || r.RefundLedgerID != firstRefund || firstRefund == 0 {
		t.Fatalf("refund replay: %+v %v", r, err)
	}
	if got := f.scalar(t, "SELECT (after->>'refundLedgerId')::bigint FROM admin_audit_logs WHERE action='vip.refund'"); got != firstRefund {
		t.Fatalf("audit refundLedgerId %d, want %d", got, firstRefund)
	}
	if _, _, err := f.svc.Refund(ctx, f.admin, purchase, "again", "refund-0002"); !errors.Is(err, vip.ErrAlreadyRefunded) {
		t.Fatalf("second refund: %v", err)
	}
	if n := f.scalar(t, "SELECT COUNT(*) FROM admin_audit_logs WHERE action='vip.refund' AND target_id=$1", f.user.String()); n != 1 {
		t.Fatalf("audit rows %d", n)
	}
	if rev := f.scalar(t, "SELECT reverses_ledger_id FROM vip_point_ledger WHERE reason='refund' AND user_id=$1", f.user); rev != purchase {
		t.Fatalf("refund links %d", rev)
	}
	f.ledgerMatches(t)
}

func TestConcurrentRefundsReverseOnce(t *testing.T) {
	f, ctx := setup(t), context.Background()
	if _, err := f.svc.Topup(ctx, f.user, 10_000, "order-9"); err != nil {
		t.Fatal(err)
	}
	purchase := f.scalar(t, "SELECT id FROM vip_point_ledger WHERE user_id=$1 AND reason='purchase'", f.user)
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok := 0
	for i := range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, _, err := f.svc.Refund(ctx, f.admin, purchase, "race", "refund-race-"+string(rune('a'+i))); err == nil {
				mu.Lock()
				ok++
				mu.Unlock()
			} else if !errors.Is(err, vip.ErrAlreadyRefunded) {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if ok != 1 || f.ledgerMatches(t) != 0 {
		t.Fatalf("successful refunds %d", ok)
	}
}

func TestRefundRefusedWhenBalanceTooLow(t *testing.T) {
	f, ctx := setup(t), context.Background()
	if _, err := f.svc.Topup(ctx, f.user, 10_000, "order-7"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Grant(ctx, &f.admin, f.user, -60, "admin_revoke", "admin_note", "test", "revoke-0001"); err != nil {
		t.Fatal(err)
	}
	purchase := f.scalar(t, "SELECT id FROM vip_point_ledger WHERE user_id=$1 AND reason='purchase'", f.user)
	if _, _, err := f.svc.Refund(ctx, f.admin, purchase, "x", "refund-0007"); !errors.Is(err, vip.ErrInvalidGrant) {
		t.Fatalf("refund over balance: %v", err)
	}
	f.ledgerMatches(t)
}

// M2: an order is credited once, whatever path retries it; another user cannot claim the same order.
func TestTopupOrderCreditedOnce(t *testing.T) {
	f, ctx := setup(t), context.Background()
	other := uuid.New()
	if _, err := f.pool.Exec(ctx, "INSERT INTO users (id, email) VALUES ($1, $2)", other, other.String()+"@t"); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := f.svc.Topup(ctx, f.user, 20_000, "order-dup"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if st, err := f.svc.Topup(ctx, f.user, 20_000, "order-dup"); err != nil || st.Points != 200 {
		t.Fatalf("replay: %+v %v", st, err)
	}
	if _, err := f.svc.Topup(ctx, other, 20_000, "order-dup"); !errors.Is(err, vip.ErrOrderConflict) {
		t.Fatalf("other user, same order: %v", err)
	}
	if n := f.scalar(t, "SELECT COUNT(*) FROM vip_point_ledger WHERE reason='purchase' AND ref_id='order-dup'"); n != 1 {
		t.Fatalf("purchase rows %d", n)
	}
	// The DB index is the last guard even if a writer skipped TopupLocked.
	_, err := f.pool.Exec(ctx, `INSERT INTO vip_point_ledger (user_id, delta, points_after, reason, source, game_day, ref_type, ref_id, idempotency_key)
		VALUES ($1, 200, 400, 'purchase', 'topup', CURRENT_DATE, 'topup_order', 'order-dup', 'other-key')`, f.user)
	if err == nil {
		t.Fatal("unique order index missing")
	}
	f.ledgerMatches(t)
}

// L3: grant reasons must match the sign of the change.
func TestGrantReasonMatchesSign(t *testing.T) {
	f, ctx := setup(t), context.Background()
	for _, c := range []struct {
		reason string
		delta  int64
	}{{"admin_grant", -5}, {"promo", -5}, {"admin_revoke", 5}, {"purchase", 5}, {"admin_grant", 0}} {
		if _, err := f.svc.Grant(ctx, &f.admin, f.user, c.delta, c.reason, "admin_note", "x", "g-"+c.reason); !errors.Is(err, vip.ErrInvalidGrant) {
			t.Errorf("%s %d: %v", c.reason, c.delta, err)
		}
	}
	if _, err := f.svc.Grant(ctx, &f.admin, f.user, 5, "admin_grant", "admin_note", "x", "g-ok"); err != nil {
		t.Fatal(err)
	}
}

// M1: 000009 down → up keeps refund links and activity attribution (backfill reads ref_type/ref_id only).
func TestMigration9DownUpKeepsLinks(t *testing.T) {
	f, ctx := setup(t), context.Background()
	earnRef := func(act string, ref vip.Ref) {
		if err := postgres.InTx(ctx, f.pool, func(tx pgx.Tx) error {
			_, err := f.svc.Table.EarnLocked(ctx, db.New(tx), f.user, act, ref, *f.clock)
			return err
		}); err != nil {
			t.Fatal(err)
		}
	}
	earnRef(vip.ActOnlineCultivation, vip.Ref{Type: "settlement", ID: "s1", IdemKey: "settle:s1"})
	earnRef(vip.ActDailyTask, vip.Ref{Type: "daily_task", ID: "diem_danh", IdemKey: "task:x:diem_danh"})
	earnRef(vip.ActLecture, vip.Ref{Type: "lecture_insight", ID: "7", IdemKey: "lecture:x:10"})
	if _, err := f.svc.Topup(ctx, f.user, 10_000, "order-m1"); err != nil {
		t.Fatal(err)
	}
	purchase := f.scalar(t, "SELECT id FROM vip_point_ledger WHERE reason='purchase' AND user_id=$1", f.user)
	if _, _, err := f.svc.Refund(ctx, f.admin, purchase, "chargeback", "refund-m1-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Grant(ctx, &f.admin, f.user, 2, "admin_grant", "admin_note", "x", "grant-m1"); err != nil {
		t.Fatal(err)
	}
	type row struct{ key, source, activity string }
	snapshot := func() map[string]row {
		rows, err := f.pool.Query(ctx, `SELECT idempotency_key, reason || '/' || source, COALESCE(activity_id, '') || '/' ||
			COALESCE(reverses_ledger_id::text, '') FROM vip_point_ledger WHERE user_id = $1`, f.user)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]row{}
		for rows.Next() {
			var r row
			if err := rows.Scan(&r.key, &r.source, &r.activity); err != nil {
				t.Fatal(err)
			}
			out[r.key] = r
		}
		return out
	}
	before := snapshot()
	for _, file := range []string{"000009_tien_duyen_ledger.down.sql", "000009_tien_duyen_ledger.up.sql"} {
		sql, err := os.ReadFile(filepath.Join("..", "..", "migrations", file))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.pool.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
	}
	after := snapshot()
	if len(after) != len(before) || len(before) != 6 {
		t.Fatalf("rows before %d after %d", len(before), len(after))
	}
	for k, b := range before {
		if after[k] != b {
			t.Errorf("row %s: before %+v after %+v", k, b, after[k])
		}
	}
	if _, _, err := f.svc.Refund(ctx, f.admin, purchase, "again", "refund-m1-2"); !errors.Is(err, vip.ErrAlreadyRefunded) {
		t.Fatalf("second refund after down/up: %v", err)
	}
	f.ledgerMatches(t)
}
