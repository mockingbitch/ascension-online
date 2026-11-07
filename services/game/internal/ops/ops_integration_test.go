//go:build integration

package ops_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"tiendao/game/internal/ops"
	"tiendao/game/internal/platform/testdb"
	"tiendao/game/internal/world"
)

type fixture struct {
	pool          *pgxpool.Pool
	admin, player uuid.UUID
	char          uuid.UUID
}

func seed(t *testing.T) fixture {
	t.Helper()
	ctx := context.Background()
	f := fixture{pool: testdb.New(t), admin: uuid.New(), player: uuid.New(), char: uuid.New()}
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := f.pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	exec("INSERT INTO users (id, email, role) VALUES ($1, 'admin@t', 'admin'), ($2, 'p@t', 'player')", f.admin, f.player)
	exec("INSERT INTO characters (id, user_id, name) VALUES ($1, $2, 'Thử Nghiệm')", f.char, f.player)
	exec("INSERT INTO character_progressions (character_id, realm_id, stage_id, cultivation_milli) VALUES ($1, 'luyen_khi', 'luyen_khi.tang_1', 5000)", f.char)
	exec("INSERT INTO currency_accounts (character_id, currency, balance) VALUES ($1, 'spirit_stone', 70)", f.char)
	exec(`INSERT INTO currency_ledger (character_id, currency, delta, balance_after, reason, idempotency_key)
		VALUES ($1, 'spirit_stone', 100, 100, 'daily_reward', 'k1'), ($1, 'spirit_stone', -30, 70, 'shop_buy', 'k2')`, f.char)
	exec("INSERT INTO vip_accounts (user_id, points) VALUES ($1, 50)", f.player)
	exec(`INSERT INTO vip_point_ledger (user_id, delta, points_after, reason, idempotency_key, source, game_day)
		VALUES ($1, 50, 50, 'admin_grant', 'g1', 'admin', '2026-11-01')`, f.player)
	exec("INSERT INTO outbox_events (topic, aggregate_id, payload) VALUES ('leaderboard.dirty', $1, '{}')", f.char.String())
	exec("UPDATE world_state SET started_at = now() - interval '3 days', progress_milli = 1234")
	return f
}

func count(t *testing.T, pool *pgxpool.Pool, sql string) int64 {
	t.Helper()
	var n int64
	if err := pool.QueryRow(context.Background(), sql).Scan(&n); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return n
}

// Every table created by the migrations must be classified (keep / Tiên Duyên / reset): a new migration has to make
// an explicit decision in ops.ResetTables or ops.KeptTables.
func TestEveryTableIsClassified(t *testing.T) {
	f := seed(t)
	s, err := ops.Plan(context.Background(), f.pool, ops.ResetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Unclassified) > 0 {
		t.Fatalf("unclassified tables: %v — add them to internal/ops/reset.go", s.Unclassified)
	}
}

func TestResetBetaKeepsAccountsAndAudits(t *testing.T) {
	ctx := context.Background()
	f := seed(t)
	s, err := ops.Plan(ctx, f.pool, ops.ResetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if s.Counts["characters"] != 1 || s.Counts["currency_ledger"] != 2 || s.Counts["vip_point_ledger"] != 1 || !s.WorldStarted {
		t.Fatalf("plan %+v", s)
	}
	// Guard rails: wrong phrase, non-admin.
	if _, err := ops.ResetBeta(ctx, f.pool, ops.ResetOptions{Admin: f.admin, ConfirmPhrase: "reset prod"}); !errors.Is(err, ops.ErrConfirm) {
		t.Fatalf("wrong phrase: %v", err)
	}
	phrase := ops.ResetConfirmPhrase(s.Database)
	if _, err := ops.ResetBeta(ctx, f.pool, ops.ResetOptions{Admin: f.player, ConfirmPhrase: phrase}); !errors.Is(err, ops.ErrNotAdmin) {
		t.Fatalf("non-admin: %v", err)
	}
	if n := count(t, f.pool, "SELECT count(*) FROM characters"); n != 1 {
		t.Fatalf("refused reset changed data: %d characters", n)
	}

	got, err := ops.ResetBeta(ctx, f.pool, ops.ResetOptions{Admin: f.admin, ConfirmPhrase: phrase, Reason: "beta end"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Total() < 7 {
		t.Fatalf("total %d", got.Total())
	}
	for _, tbl := range append(ops.ResetTables(), ops.DuyenTables()...) {
		want := int64(0)
		if tbl == "world_state" {
			want = 1
		}
		if n := count(t, f.pool, "SELECT count(*) FROM "+tbl); n != want {
			t.Errorf("%s: %d rows after reset, want %d", tbl, n, want)
		}
	}
	if n := count(t, f.pool, "SELECT count(*) FROM users"); n != 2 {
		t.Fatalf("users kept: %d", n)
	}
	if n := count(t, f.pool, "SELECT count(*) FROM world_state WHERE id = 1 AND started_at IS NULL AND progress_milli = 0"); n != 1 {
		t.Fatal("world_state not back to pre-launch")
	}
	if n := count(t, f.pool, "SELECT count(*) FROM admin_audit_logs WHERE action = 'ops.reset_beta' AND reason = 'beta end' AND before ? 'counts'"); n != 1 {
		t.Fatal("audit row missing")
	}
	// Identities restart: a new outbox row gets id 1.
	if n := count(t, f.pool, "INSERT INTO outbox_events (topic, aggregate_id, payload) VALUES ('x', 'y', '{}') RETURNING id"); n != 1 {
		t.Fatalf("identity not restarted: %d", n)
	}

	// Day 0 after the reset: the existing world.Start logic (wrapped by opsctl start-world).
	st, err := (&world.Service{Pool: f.pool, Now: time.Now}).Start(ctx, f.admin)
	if err != nil || st.StartedAt == nil || st.ServerDay != 0 {
		t.Fatalf("start %+v %v", st, err)
	}
}

func TestResetBetaKeepDuyenAndPurchaseGuard(t *testing.T) {
	ctx := context.Background()
	f := seed(t)
	if _, err := f.pool.Exec(ctx, `INSERT INTO vip_point_ledger (user_id, delta, points_after, reason, ref_type, ref_id, idempotency_key, source, game_day)
		VALUES ($1, 100, 150, 'purchase', 'order', 'o-1', 'p1', 'topup', '2026-11-02')`, f.player); err != nil {
		t.Fatal(err)
	}
	s, _ := ops.Plan(ctx, f.pool, ops.ResetOptions{})
	phrase := ops.ResetConfirmPhrase(s.Database)
	if s.Purchases != 1 {
		t.Fatalf("purchases %d", s.Purchases)
	}
	if _, err := ops.ResetBeta(ctx, f.pool, ops.ResetOptions{Admin: f.admin, ConfirmPhrase: phrase}); !errors.Is(err, ops.ErrPurchases) {
		t.Fatalf("purchase guard: %v", err)
	}
	if _, err := ops.ResetBeta(ctx, f.pool, ops.ResetOptions{Admin: f.admin, ConfirmPhrase: phrase, KeepDuyen: true}); err != nil {
		t.Fatal(err)
	}
	if n := count(t, f.pool, "SELECT count(*) FROM vip_point_ledger"); n != 2 {
		t.Fatalf("keep-duyen wiped the ledger: %d", n)
	}
	if n := count(t, f.pool, "SELECT count(*) FROM characters"); n != 0 {
		t.Fatalf("characters %d", n)
	}
}

func TestResetRefusesUnclassifiedTable(t *testing.T) {
	ctx := context.Background()
	f := seed(t)
	if _, err := f.pool.Exec(ctx, "CREATE TABLE brand_new_feature (id int)"); err != nil {
		t.Fatal(err)
	}
	s, _ := ops.Plan(ctx, f.pool, ops.ResetOptions{})
	_, err := ops.ResetBeta(ctx, f.pool, ops.ResetOptions{Admin: f.admin, ConfirmPhrase: ops.ResetConfirmPhrase(s.Database)})
	if !errors.Is(err, ops.ErrUnclassified) {
		t.Fatalf("got %v", err)
	}
}

type sink struct{ stone, duyen map[string][2]int64 }

func (s *sink) Stone(r string, c, d int64) {
	v := s.stone[r]
	s.stone[r] = [2]int64{v[0] + c, v[1] + d}
}
func (s *sink) Duyen(r string, e, rm int64) {
	v := s.duyen[r]
	s.duyen[r] = [2]int64{v[0] + e, v[1] + rm}
}

func TestLedgerScannerCountsOnlyNewSettledRows(t *testing.T) {
	ctx := context.Background()
	f := seed(t)
	sc := &ops.LedgerScanner{Pool: f.pool, Lag: time.Nanosecond}
	k := &sink{stone: map[string][2]int64{}, duyen: map[string][2]int64{}}
	if err := sc.Scan(ctx, k); err != nil { // positions the cursors: history is not counted
		t.Fatal(err)
	}
	if len(k.stone) != 0 {
		t.Fatalf("history counted: %v", k.stone)
	}
	if _, err := f.pool.Exec(ctx, `INSERT INTO currency_ledger (character_id, currency, delta, balance_after, reason, idempotency_key, created_at)
		VALUES ($1, 'spirit_stone', 40, 110, 'daily_reward', 'k3', now() - interval '1 minute'),
		       ($1, 'spirit_stone', -10, 100, 'shop_buy', 'k4', now() - interval '1 minute')`, f.char); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `INSERT INTO vip_point_ledger (user_id, delta, points_after, reason, idempotency_key, source, game_day, created_at)
		VALUES ($1, 5, 55, 'admin_grant', 'g2', 'admin', '2026-11-02', now() - interval '1 minute')`, f.player); err != nil {
		t.Fatal(err)
	}
	for range 2 { // a second scan must not count the same rows again
		if err := sc.Scan(ctx, k); err != nil {
			t.Fatal(err)
		}
	}
	if k.stone["daily_reward"] != [2]int64{40, 0} || k.stone["shop_buy"] != [2]int64{0, 10} || k.duyen["admin"] != [2]int64{5, 0} {
		t.Fatalf("stone %v duyen %v", k.stone, k.duyen)
	}
	n, age, err := ops.OutboxBacklog(ctx, f.pool)
	if err != nil || n != 1 || age < 0 {
		t.Fatalf("backlog %d %v %v", n, age, err)
	}
}

// Payment orders are financial records: kept, so the Tiên Duyên tables they reference are emptied with DELETE; a
// real-provider order forbids wiping Tiên Duyên at all.
func TestResetKeepsPaymentOrders(t *testing.T) {
	ctx := context.Background()
	f := seed(t)
	order := func(provider, key string) {
		t.Helper()
		if _, err := f.pool.Exec(ctx, `INSERT INTO payment_orders (user_id, package_id, amount_vnd, points, provider, idempotency_key, expires_at)
			VALUES ($1, 'goi_test', 10000, 100, $2, $3, now() + interval '1 hour')`, f.player, provider, key); err != nil {
			t.Fatal(err)
		}
	}
	order("sandbox", "order-sandbox-1")
	s, _ := ops.Plan(ctx, f.pool, ops.ResetOptions{})
	phrase := ops.ResetConfirmPhrase(s.Database)
	if _, err := ops.ResetBeta(ctx, f.pool, ops.ResetOptions{Admin: f.admin, ConfirmPhrase: phrase}); err != nil {
		t.Fatal(err)
	}
	if n := count(t, f.pool, "SELECT count(*) FROM vip_point_ledger"); n != 0 {
		t.Fatalf("duyen not wiped: %d", n)
	}
	if n := count(t, f.pool, "SELECT count(*) FROM payment_orders"); n != 1 {
		t.Fatalf("payment orders must be kept: %d", n)
	}
	order("payos", "order-real-1")
	if _, err := ops.ResetBeta(ctx, f.pool, ops.ResetOptions{Admin: f.admin, ConfirmPhrase: phrase, WipePurchases: true}); !errors.Is(err, ops.ErrRealPayments) {
		t.Fatalf("real payments guard: %v", err)
	}
}
