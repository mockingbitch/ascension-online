//go:build integration

package payment_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"tiendao/game/internal/content"
	"tiendao/game/internal/payment"
	"tiendao/game/internal/platform/testdb"
	"tiendao/game/internal/vip"
)

type fixture struct {
	pool  *pgxpool.Pool
	svc   *payment.Service
	sb    *payment.Sandbox
	clock *time.Time
	mu    sync.Mutex
	user  uuid.UUID
	other uuid.UUID
	admin uuid.UUID
	char  uuid.UUID
}

func setup(t *testing.T) *fixture {
	t.Helper()
	pool := testdb.New(t)
	cb, err := content.LoadAll("../../../../content/catalog")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 7, 3, 0, 0, 0, time.UTC)
	f := &fixture{pool: pool, clock: &now, user: uuid.New(), other: uuid.New(), admin: uuid.New(), char: uuid.New()}
	clk := func() time.Time { f.mu.Lock(); defer f.mu.Unlock(); return *f.clock }
	f.sb, err = payment.NewSandbox(bytes.Repeat([]byte("s"), 32), "http://localhost:8000/tien-duyen/sandbox/", payment.PGCharges{Pool: pool})
	if err != nil {
		t.Fatal(err)
	}
	f.sb.Now = clk
	f.svc = &payment.Service{Pool: pool, Cat: cb.Payments, VIP: cb.VIP, Providers: map[string]payment.Provider{payment.ProviderSandbox: f.sb},
		ReturnURL: "http://localhost:8000/tien-duyen?order=", Now: clk}
	for _, id := range []uuid.UUID{f.user, f.other, f.admin} {
		f.exec(t, "INSERT INTO users (id, email) VALUES ($1, $2)", id, id.String()+"@t")
	}
	f.exec(t, "INSERT INTO characters (id, user_id, name) VALUES ($1, $2, 'Khách Nạp')", f.char, f.user)
	return f
}

func (f *fixture) advance(d time.Duration) { f.mu.Lock(); *f.clock = f.clock.Add(d); f.mu.Unlock() }
func (f *fixture) now() time.Time          { f.mu.Lock(); defer f.mu.Unlock(); return *f.clock }

func (f *fixture) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := f.pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func (f *fixture) scalar(t *testing.T, sql string, args ...any) (n int64) {
	t.Helper()
	if err := f.pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func (f *fixture) points(t *testing.T) int64 {
	t.Helper()
	pts := f.scalar(t, "SELECT COALESCE((SELECT points FROM vip_accounts WHERE user_id=$1), 0)", f.user)
	if sum := f.scalar(t, "SELECT COALESCE(SUM(delta),0) FROM vip_point_ledger WHERE user_id=$1", f.user); sum != pts {
		t.Fatalf("ledger sum %d != balance %d", sum, pts)
	}
	return pts
}

func (f *fixture) order(t *testing.T, pkg, key string) payment.OrderView {
	t.Helper()
	res, err := f.svc.CreateOrder(context.Background(), f.user, pkg, "", key)
	if err != nil {
		t.Fatalf("create %s: %v", pkg, err)
	}
	return res.Order
}

// deliver sends a correctly signed sandbox webhook through the public entry point.
func (f *fixture) deliver(t *testing.T, ev payment.Event) (payment.WebhookResult, error) {
	t.Helper()
	body, sig := f.sb.BuildWebhook(ev, f.now())
	h := http.Header{}
	h.Set(payment.SandboxSignatureHeader, sig)
	return f.svc.HandleWebhook(context.Background(), payment.ProviderSandbox, h, body)
}

func paidEvent(o payment.OrderView, id string, amount int64) payment.Event {
	return payment.Event{EventID: id, OrderID: o.ID, ProviderRef: payment.SandboxRef(o.ID), Kind: payment.KindPaid, AmountVND: amount}
}

func (f *fixture) status(t *testing.T, id uuid.UUID) string {
	t.Helper()
	var s string
	if err := f.pool.QueryRow(context.Background(), "SELECT status FROM payment_orders WHERE id=$1", id).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestCreateOrderIdempotentAndCapped(t *testing.T) {
	f, ctx := setup(t), context.Background()
	o := f.order(t, "so_duyen", "key-aaaaaaa1")
	if o.Status != payment.StatusPending || o.AmountVND != 20000 || o.Points != 200 || o.Checkout == nil ||
		o.Checkout.URL != "http://localhost:8000/tien-duyen/sandbox/"+o.ID.String() {
		t.Fatalf("order: %+v", o)
	}
	again, err := f.svc.CreateOrder(ctx, f.user, "so_duyen", "", "key-aaaaaaa1")
	if err != nil || !again.Replayed || again.Order.ID != o.ID {
		t.Fatalf("replay: %+v %v", again, err)
	}
	if _, err := f.svc.CreateOrder(ctx, f.user, "ket_duyen", "", "key-aaaaaaa1"); !errors.Is(err, payment.ErrKeyReuse) {
		t.Fatalf("key reuse: %v", err)
	}
	if _, err := f.svc.CreateOrder(ctx, f.user, "nope", "", "key-x"); !errors.Is(err, payment.ErrUnknownPackage) {
		t.Fatalf("unknown package: %v", err)
	}
	if _, err := f.svc.CreateOrder(ctx, f.user, "so_duyen", "payos", "key-y"); !errors.Is(err, payment.ErrUnknownProvider) {
		t.Fatalf("disabled provider: %v", err)
	}
	f.order(t, "so_duyen", "key-aaaaaaa2")
	f.order(t, "so_duyen", "key-aaaaaaa3")
	if _, err := f.svc.CreateOrder(ctx, f.user, "so_duyen", "", "key-aaaaaaa4"); !errors.Is(err, payment.ErrTooManyOpen) {
		t.Fatalf("max open: %v", err)
	}
	// Concurrent creates with distinct keys never exceed the cap (advisory lock per user).
	var wg sync.WaitGroup
	for i := range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = f.svc.CreateOrder(ctx, f.other, "so_duyen", "", "race-key-"+string(rune('a'+i)))
		}()
	}
	wg.Wait()
	if n := f.scalar(t, "SELECT COUNT(*) FROM payment_orders WHERE user_id=$1", f.other); n != 3 {
		t.Fatalf("concurrent creates: %d orders", n)
	}
	// Own orders only.
	if _, err := f.svc.GetOrder(ctx, f.other, o.ID); !errors.Is(err, payment.ErrOrderNotFound) {
		t.Fatalf("foreign order: %v", err)
	}
	list, err := f.svc.ListOrders(ctx, f.user, 50)
	if err != nil || len(list) != 3 {
		t.Fatalf("history: %d %v", len(list), err)
	}
	if f.points(t) != 0 {
		t.Fatal("creating orders must not credit")
	}
}

func TestWebhookCreditsOnceAndReplays(t *testing.T) {
	f, ctx := setup(t), context.Background()
	o := f.order(t, "so_duyen", "key-bbbbbbb1")
	ev := paidEvent(o, "evt_1", 20000)
	res, err := f.deliver(t, ev)
	if err != nil || res.Outcome != payment.OutcomeApplied || res.Status != payment.StatusPaid {
		t.Fatalf("first: %+v %v", res, err)
	}
	if got := f.points(t); got != 200 {
		t.Fatalf("points %d", got)
	}
	// Duplicate delivery (same event id, re-signed later) → replay no-op.
	f.advance(time.Minute)
	if res, err := f.deliver(t, ev); err != nil || res.Outcome != payment.OutcomeReplayed || res.Detail != "duplicate_event" {
		t.Fatalf("duplicate: %+v %v", res, err)
	}
	// Another event id saying paid again → a second payment at the provider: not credited, alerted.
	if res, err := f.deliver(t, paidEvent(o, "evt_2", 20000)); err != nil || res.Outcome != payment.OutcomeIgnored || res.Detail != "duplicate_payment" {
		t.Fatalf("second paid: %+v %v", res, err)
	}
	// Out of order: failed after paid is ignored, the order stays paid.
	fail := payment.Event{EventID: "evt_3", OrderID: o.ID, Kind: payment.KindFailed}
	if res, err := f.deliver(t, fail); err != nil || res.Outcome != payment.OutcomeIgnored || res.Status != payment.StatusPaid {
		t.Fatalf("failed after paid: %+v %v", res, err)
	}
	if got := f.points(t); got != 200 {
		t.Fatalf("points after replays %d", got)
	}
	if n := f.scalar(t, "SELECT COUNT(*) FROM vip_point_ledger WHERE user_id=$1 AND reason='purchase'", f.user); n != 1 {
		t.Fatalf("purchase rows %d", n)
	}
	got, err := f.svc.GetOrder(ctx, f.user, o.ID)
	if err != nil || got.Order.Status != payment.StatusPaid || got.Order.Checkout != nil || got.VIP == nil || got.VIP.Points != 200 ||
		got.VIP.Benefits.Tier != 2 {
		t.Fatalf("order view: %+v %v", got, err)
	}
	// Bad signature: rejected before any write.
	body, _ := f.sb.BuildWebhook(paidEvent(o, "evt_forged", 20000), f.now())
	h := http.Header{}
	h.Set(payment.SandboxSignatureHeader, "t=1,v1=00")
	before := f.scalar(t, "SELECT COUNT(*) FROM payment_events")
	if _, err := f.svc.HandleWebhook(ctx, payment.ProviderSandbox, h, body); !errors.Is(err, payment.ErrBadSignature) {
		t.Fatalf("forged: %v", err)
	}
	if after := f.scalar(t, "SELECT COUNT(*) FROM payment_events"); after != before {
		t.Fatal("a forged webhook wrote an event row")
	}
}

func TestFailedThenOutOfOrderAndMismatch(t *testing.T) {
	f, ctx := setup(t), context.Background()
	o := f.order(t, "thien_duyen", "key-ccccccc1") // 50.000đ → 500
	// Amount mismatch: rejected and alerted, nothing credited, order still open.
	res, err := f.deliver(t, paidEvent(o, "evt_m", 10000))
	if err != nil || res.Outcome != payment.OutcomeRejected || res.Detail != "amount_mismatch" {
		t.Fatalf("mismatch: %+v %v", res, err)
	}
	if f.points(t) != 0 || f.status(t, o.ID) != payment.StatusPending {
		t.Fatal("mismatch credited or closed the order")
	}
	alerts, err := f.svc.Alerts(ctx, 10)
	if err != nil || len(alerts) != 1 || alerts[0].Detail != "amount_mismatch" || alerts[0].OrderID == nil || *alerts[0].OrderID != o.ID {
		t.Fatalf("alerts: %+v %v", alerts, err)
	}
	// Unknown order id (correctly signed) → rejected, recorded without an order.
	ghost := payment.OrderView{ID: uuid.New()}
	if res, err := f.deliver(t, paidEvent(ghost, "evt_g", 20000)); err != nil || res.Detail != "unknown_order" {
		t.Fatalf("unknown order: %+v %v", res, err)
	}
	// Failed, then a verified paid → late payment honoured exactly once.
	if res, err := f.deliver(t, payment.Event{EventID: "evt_f", OrderID: o.ID, Kind: payment.KindFailed}); err != nil || res.Status != payment.StatusFailed {
		t.Fatalf("failed: %+v %v", res, err)
	}
	if res, err := f.deliver(t, paidEvent(o, "evt_p", 50000)); err != nil || res.Outcome != payment.OutcomeApplied || res.Detail != "late_payment" {
		t.Fatalf("late paid: %+v %v", res, err)
	}
	if got := f.points(t); got != 500 {
		t.Fatalf("points %d", got)
	}
	// The DB trigger refuses an illegal transition even if code tried one.
	if _, err := f.pool.Exec(ctx, "UPDATE payment_orders SET status='pending' WHERE id=$1", o.ID); err == nil {
		t.Fatal("trigger allowed paid → pending")
	}
}

func TestConcurrentWebhookAndReconcileSettleOnce(t *testing.T) {
	f, ctx := setup(t), context.Background()
	for round := range 5 {
		o := f.order(t, "so_duyen", "key-race-"+string(rune('a'+round)))
		// The player paid at the gateway; the webhook is in flight while the reconciliation job also runs.
		if _, err := f.svc.SandboxComplete(ctx, f.user, false, o.ID, payment.KindPaid, false, 0); err != nil {
			t.Fatal(err)
		}
		f.advance(6 * time.Minute) // older than reconcile_after
		var wg sync.WaitGroup
		errs := make(chan error, 16)
		for i := range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if i%2 == 0 {
					_, err := f.svc.Reconcile(ctx)
					errs <- err
					return
				}
				_, err := f.deliver(t, paidEvent(o, "evt_race_"+string(rune('a'+round))+string(rune('0'+i)), 20000))
				errs <- err
			}()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatal(err)
			}
		}
		if s := f.status(t, o.ID); s != payment.StatusPaid {
			t.Fatalf("round %d: status %s", round, s)
		}
		if n := f.scalar(t, "SELECT COUNT(*) FROM vip_point_ledger WHERE ref_id=$1", o.ID.String()); n != 1 {
			t.Fatalf("round %d: %d purchase rows", round, n)
		}
		if n := f.scalar(t, "SELECT COUNT(*) FROM payment_events WHERE order_id=$1 AND outcome='applied'", o.ID); n != 1 {
			t.Fatalf("round %d: %d applied events", round, n)
		}
	}
	if got := f.points(t); got != 1000 {
		t.Fatalf("points %d, want 5 × 200", got)
	}
}

func TestExpiryAndLostWebhookReconciled(t *testing.T) {
	f, ctx := setup(t), context.Background()
	stale := f.order(t, "so_duyen", "key-ddddddd1")
	lost := f.order(t, "ket_duyen", "key-ddddddd2") // 100.000đ → 1000
	// lost: paid at the gateway but the webhook never arrives.
	if _, err := f.svc.SandboxComplete(ctx, f.user, false, lost.ID, payment.KindPaid, false, 0); err != nil {
		t.Fatal(err)
	}
	f.advance(2 * time.Minute)
	if res, err := f.svc.Reconcile(ctx); err != nil || res.Checked != 0 {
		t.Fatalf("too young to reconcile: %+v %v", res, err)
	}
	f.advance(29 * time.Minute) // both past expiry (30 min)
	res, err := f.svc.Reconcile(ctx)
	if err != nil || res.Checked != 2 || res.Settled != 1 {
		t.Fatalf("reconcile: %+v %v", res, err)
	}
	n, err := f.svc.Expire(ctx)
	if err != nil || n != 1 {
		t.Fatalf("expire: %d %v", n, err)
	}
	if f.status(t, stale.ID) != payment.StatusExpired || f.status(t, lost.ID) != payment.StatusPaid {
		t.Fatalf("statuses: %s %s", f.status(t, stale.ID), f.status(t, lost.ID))
	}
	if n, _ := f.svc.Expire(ctx); n != 0 {
		t.Fatal("expiry not idempotent")
	}
	// An expired order still open at the provider: queried again after reconcile_every, still pending → stays expired.
	f.advance(6 * time.Minute)
	if res, err := f.svc.Reconcile(ctx); err != nil || res.Checked != 1 || res.Settled != 0 {
		t.Fatalf("expired re-check: %+v %v", res, err)
	}
	// Expired orders no longer count as open, and show no checkout.
	got, err := f.svc.GetOrder(ctx, f.user, stale.ID)
	if err != nil || got.Order.Checkout != nil {
		t.Fatalf("expired view: %+v %v", got, err)
	}
	if got := f.points(t); got != 1000 {
		t.Fatalf("points %d", got)
	}
}

func TestRefundDropsLevelAndStripsPerks(t *testing.T) {
	f, ctx := setup(t), context.Background()
	o := f.order(t, "hong_duyen", "key-eeeeeee1") // 1.000.000đ → 10,000 → Tiên Duyên 7
	if _, err := f.deliver(t, paidEvent(o, "evt_big", 1000000)); err != nil {
		t.Fatal(err)
	}
	f.exec(t, "INSERT INTO character_appearance (character_id, title_id, avatar_frame_id) VALUES ($1, 'title:vip_7', 'avatar_frame:kim')", f.char)
	if _, err := f.svc.Refund(ctx, f.admin, uuid.New(), "x"); !errors.Is(err, payment.ErrOrderNotFound) {
		t.Fatalf("unknown order: %v", err)
	}
	pending := f.order(t, "so_duyen", "key-eeeeeee2")
	if _, err := f.svc.Refund(ctx, f.admin, pending.ID, "x"); !errors.Is(err, payment.ErrNotRefundable) {
		t.Fatalf("pending refund: %v", err)
	}
	res, err := f.svc.Refund(ctx, f.admin, o.ID, "chargeback test")
	if err != nil || res.Replayed || res.Order.Status != payment.StatusRefunded || res.Refund.PointsBefore != 10000 ||
		res.Refund.PointsAfter != 0 || res.VIP.Benefits.Tier != 0 {
		t.Fatalf("refund: %+v %v", res, err)
	}
	var title, frame *string
	if err := f.pool.QueryRow(ctx, "SELECT title_id, avatar_frame_id FROM character_appearance WHERE character_id=$1", f.char).Scan(&title, &frame); err != nil {
		t.Fatal(err)
	}
	if title != nil || frame != nil {
		t.Fatalf("perks kept after level drop: %v %v", title, frame)
	}
	if f.points(t) != 0 {
		t.Fatal("points remain")
	}
	if n := f.scalar(t, "SELECT COUNT(*) FROM admin_audit_logs WHERE action IN ('vip.refund','payment.refund')"); n != 2 {
		t.Fatalf("audit rows %d", n)
	}
	again, err := f.svc.Refund(ctx, f.admin, o.ID, "retry")
	if err != nil || !again.Replayed {
		t.Fatalf("refund replay: %+v %v", again, err)
	}
	if n := f.scalar(t, "SELECT COUNT(*) FROM vip_point_ledger WHERE reason='refund'"); n != 1 {
		t.Fatalf("refund rows %d", n)
	}
	// A paid webhook replayed after the refund never re-credits.
	if res, err := f.deliver(t, paidEvent(o, "evt_after_refund", 1000000)); err != nil || res.Outcome != payment.OutcomeIgnored {
		t.Fatalf("paid after refund: %+v %v", res, err)
	}
	if f.points(t) != 0 {
		t.Fatal("re-credited after refund")
	}
}

func TestSandboxCompleteOwnerOnly(t *testing.T) {
	f, ctx := setup(t), context.Background()
	o := f.order(t, "so_duyen", "key-fffffff1")
	if _, err := f.svc.SandboxComplete(ctx, f.other, false, o.ID, payment.KindPaid, true, 0); !errors.Is(err, payment.ErrOrderNotFound) {
		t.Fatalf("foreign: %v", err)
	}
	res, err := f.svc.SandboxComplete(ctx, f.user, false, o.ID, payment.KindFailed, true, 0)
	if err != nil || res.Webhook == nil || res.Order.Status != payment.StatusFailed {
		t.Fatalf("fail: %+v %v", res, err)
	}
}

// countingProvider returns a NEW checkout per call (like real gateways), to prove only one is ever created per order.
type countingProvider struct {
	*payment.Sandbox
	n atomic.Int32
}

func (c *countingProvider) CreateCheckout(_ context.Context, req payment.CheckoutRequest) (payment.Checkout, error) {
	k := strconv.Itoa(int(c.n.Add(1)))
	time.Sleep(20 * time.Millisecond) // widen the race window
	return payment.Checkout{ProviderRef: "cp_" + k, RedirectURL: "http://gw/" + k}, nil
}

func TestConcurrentSameKeyCreatesOneCheckout(t *testing.T) {
	f, ctx := setup(t), context.Background()
	cp := &countingProvider{Sandbox: f.sb}
	f.svc.Providers = map[string]payment.Provider{payment.ProviderSandbox: cp}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := f.svc.CreateOrder(ctx, f.user, "so_duyen", "", "key-same-race"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if got := cp.n.Load(); got != 1 {
		t.Fatalf("%d checkouts created for one order", got)
	}
	var ref, url string
	if err := f.pool.QueryRow(ctx, "SELECT provider_ref, checkout_url FROM payment_orders WHERE user_id=$1", f.user).Scan(&ref, &url); err != nil {
		t.Fatal(err)
	}
	if ref != "cp_1" || url != "http://gw/1" {
		t.Fatalf("checkout mixed up: %s %s", ref, url)
	}
}

func TestRefundRefusedBeforeProviderWhenBalanceShort(t *testing.T) {
	f, ctx := setup(t), context.Background()
	o := f.order(t, "so_duyen", "key-ggggggg1")
	if _, err := f.svc.SandboxComplete(ctx, f.user, false, o.ID, payment.KindPaid, true, 0); err != nil {
		t.Fatal(err)
	}
	vs := &vip.Service{Pool: f.pool, Table: f.svc.VIP, Now: f.now}
	if _, err := vs.Grant(ctx, &f.admin, f.user, -150, "admin_revoke", "test", "x", "revoke-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Refund(ctx, f.admin, o.ID, "chargeback"); !errors.Is(err, vip.ErrInvalidGrant) {
		t.Fatalf("refund with short balance: %v", err)
	}
	c, ok, err := payment.PGCharges{Pool: f.pool}.Get(ctx, o.ID)
	if err != nil || !ok || c.Status != payment.KindPaid {
		t.Fatalf("provider refund ran although the ledger refused: %+v %v", c, err)
	}
	if f.status(t, o.ID) != payment.StatusPaid {
		t.Fatal("order changed")
	}
}
