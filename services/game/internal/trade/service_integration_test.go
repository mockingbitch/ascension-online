//go:build integration

package trade_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"tiendao/game/internal/content"
	"tiendao/game/internal/db"
	"tiendao/game/internal/economy"
	"tiendao/game/internal/netsig"
	"tiendao/game/internal/platform/postgres"
	"tiendao/game/internal/platform/testdb"
	"tiendao/game/internal/report"
	"tiendao/game/internal/trade"
)

type player struct{ user, char uuid.UUID }

type env struct {
	pool  *pgxpool.Pool
	cb    *content.Bundle
	svc   *trade.Service
	net   *netsig.Store
	clock *time.Time
	mu    sync.Mutex
	n     int
}

func setup(t *testing.T) *env {
	t.Helper()
	pool := testdb.New(t)
	cb, err := content.LoadAll("../../../../content/catalog")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 6, 1, 3, 0, 0, 0, time.UTC) // 10:00 game time (UTC+7)
	e := &env{pool: pool, cb: cb, clock: &now}
	nowFn := func() time.Time { e.mu.Lock(); defer e.mu.Unlock(); return *e.clock }
	e.net = &netsig.Store{Pool: pool, Hasher: &netsig.Hasher{Keys: []netsig.Key{{ID: "t1", Secret: []byte("test-salt-test-salt-test-salt-test-salt")}}}, Now: nowFn}
	e.svc = &trade.Service{Pool: pool, Cfg: cb.Trade, Items: cb.Items, Realms: cb.Realms, Net: e.net, Now: nowFn}
	return e
}

func (e *env) advance(d time.Duration) { e.mu.Lock(); *e.clock = e.clock.Add(d); e.mu.Unlock() }

// player: an account `accountDays` old with a character of the same age at `tier`, holding `stones` and `herbs`.
func (e *env) player(t *testing.T, accountDays, tier int, stones, herbs int64) player {
	t.Helper()
	e.n++
	ctx := context.Background()
	user, char := uuid.New(), uuid.New()
	created := e.clock.Add(-time.Duration(accountDays) * 24 * time.Hour)
	st, ok := e.cb.Realms.StageAt(e.cb.Realms.DefaultPath(), tier, 1)
	if !ok {
		t.Fatalf("no stage at tier %d", tier)
	}
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{"INSERT INTO users (id, email, created_at) VALUES ($1, $2, $3)", []any{user, user.String() + "@t", created}},
		{"INSERT INTO characters (id, user_id, name, created_at) VALUES ($1, $2, $3, $4)", []any{char, user, fmt.Sprintf("Đạo Hữu %03d", e.n), created}},
		{"INSERT INTO character_progressions (character_id, realm_id, stage_id, last_cultivated_at, stage_reached_at) VALUES ($1, $2, $3, $4, $4)",
			[]any{char, st.RealmID, st.ID, *e.clock}},
	} {
		if _, err := e.pool.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatal(err)
		}
	}
	e.grant(t, char, stones, herbs)
	return player{user, char}
}

func (e *env) grant(t *testing.T, char uuid.UUID, stones, herbs int64) {
	t.Helper()
	err := postgres.InTx(context.Background(), e.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		k := uuid.NewString()
		if stones > 0 {
			if _, err := economy.Credit(context.Background(), q, char, economy.SpiritStone, stones, economy.Ref{Reason: "admin_grant", IdemKey: k}); err != nil {
				return err
			}
		}
		if herbs > 0 {
			if _, err := economy.GrantItem(context.Background(), q, char, "linh_thao", herbs, economy.Ref{Reason: "admin_grant", IdemKey: k}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func (e *env) ok(t *testing.T) player { return e.player(t, 30, 2, 1000, 50) }

func (e *env) stones(t *testing.T, char uuid.UUID) int64 {
	t.Helper()
	var b int64
	err := e.pool.QueryRow(context.Background(), "SELECT COALESCE((SELECT balance FROM currency_accounts WHERE character_id = $1 AND currency = 'spirit_stone'), 0)", char).Scan(&b)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func (e *env) herbs(t *testing.T, char uuid.UUID) int64 {
	t.Helper()
	var n int64
	err := e.pool.QueryRow(context.Background(), "SELECT COALESCE((SELECT quantity FROM inventory_items WHERE character_id = $1 AND item_id = 'linh_thao'), 0)", char).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// totalStones is the whole economy's spirit stones (conservation checks).
func (e *env) totalStones(t *testing.T) int64 {
	t.Helper()
	var n int64
	if err := e.pool.QueryRow(context.Background(), "SELECT COALESCE(sum(balance), 0)::bigint FROM currency_accounts WHERE currency = 'spirit_stone'").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func key() string { return strings.ReplaceAll(uuid.NewString(), "-", "") }

func stones(n int64) []trade.Line { return []trade.Line{{AssetID: trade.SpiritStone, Quantity: n}} }
func herbs(n int64) []trade.Line  { return []trade.Line{{AssetID: "linh_thao", Quantity: n}} }

func (e *env) offer(t *testing.T, from, to player, give, take []trade.Line) uuid.UUID {
	t.Helper()
	k := key()
	if _, err := e.svc.Create(context.Background(), from.user, to.char, give, take, k); err != nil {
		t.Fatalf("create: %v", err)
	}
	var id uuid.UUID
	if err := e.pool.QueryRow(context.Background(), "SELECT id FROM trade_offers WHERE sender_character_id = $1 AND idempotency_key = $2",
		from.char, k).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func limitCode(err error) string {
	var le *trade.LimitError
	if errors.As(err, &le) {
		return le.Side + ":" + le.Code
	}
	return ""
}

func TestGiftMovesStonesBurnsFeeAndLogs(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	a, b := e.ok(t), e.ok(t)
	before := e.totalStones(t)
	id := e.offer(t, a, b, stones(100), nil)
	if e.stones(t, a.char) != 1000 {
		t.Fatal("nothing is escrowed at creation")
	}
	if _, err := e.svc.Accept(ctx, b.user, id, false, key()); !errors.Is(err, trade.ErrNotAcknowledged) {
		t.Fatalf("no ack: %v", err)
	}
	k := key()
	v, err := e.svc.Accept(ctx, b.user, id, true, k)
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Incoming) != 0 || v.Limits.ValueReceived != 100 || v.Limits.CompletedLeft != e.cb.Trade.Daily.CompletedPerDay-1 {
		t.Fatalf("view after accept: %+v", v.Limits)
	}
	if e.stones(t, a.char) != 1000-100-5 || e.stones(t, b.char) != 1100 {
		t.Fatalf("balances %d %d", e.stones(t, a.char), e.stones(t, b.char))
	}
	if got := before - e.totalStones(t); got != 5 {
		t.Fatalf("burned %d, want the 5%% fee (5)", got)
	}
	// Replay: same key → same state, nothing moves again.
	if rv, err := e.svc.Accept(ctx, b.user, id, true, k); err != nil || !rv.Replayed || v.Replayed {
		t.Fatalf("replay: %v (replayed %v, first %v)", err, rv.Replayed, v.Replayed)
	}
	if e.stones(t, a.char) != 895 || e.stones(t, b.char) != 1100 {
		t.Fatal("replay moved stones again")
	}
	// Ledger rows: sender −100 trade_send, −5 trade_fee; receiver +100 trade_receive; all ref the offer.
	rows, err := e.pool.Query(ctx, "SELECT character_id, reason, delta FROM currency_ledger WHERE ref_type = 'trade' AND ref_id = $1 ORDER BY id", id.String())
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for rows.Next() {
		var c uuid.UUID
		var reason string
		var d int64
		if err := rows.Scan(&c, &reason, &d); err != nil {
			t.Fatal(err)
		}
		who := "a"
		if c == b.char {
			who = "b"
		}
		got = append(got, fmt.Sprintf("%s:%s:%d", who, reason, d))
	}
	if strings.Join(got, ",") != "a:trade_send:-100,b:trade_receive:100,a:trade_fee:-5" {
		t.Fatalf("ledger: %v", got)
	}
	var fees int64
	var status string
	if err := e.pool.QueryRow(ctx, "SELECT sender_fee, status FROM trade_offers WHERE id = $1", id).Scan(&fees, &status); err != nil || fees != 5 || status != "accepted" {
		t.Fatalf("offer row: %d %s %v", fees, status, err)
	}
	var events string
	if err := e.pool.QueryRow(ctx, "SELECT string_agg(kind, ',' ORDER BY id) FROM trade_events WHERE offer_id = $1", id).Scan(&events); err != nil || events != "created,accepted" {
		t.Fatalf("events: %q %v", events, err)
	}
	// Terminal rows never change (trigger).
	if _, err := e.pool.Exec(ctx, "UPDATE trade_offers SET status = 'cancelled' WHERE id = $1", id); err == nil {
		t.Fatal("trigger let an accepted offer change status")
	}
	h, err := e.svc.History(ctx, a.user)
	if err != nil || len(h) != 1 || h[0].Status != "accepted" || h[0].SenderFee != 5 || h[0].Direction != "outgoing" {
		t.Fatalf("history: %+v %v", h, err)
	}
}

func TestSwapIsAtomicAndReverifiedAtAccept(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	a, b := e.ok(t), e.ok(t)
	// A offers 40 linh thảo (value 200) for B's 150 stones.
	id := e.offer(t, a, b, herbs(40), stones(150))
	// The receiver now lacks the stones: accept fails, nothing moves, the offer stays pending.
	if _, err := e.pool.Exec(ctx, "UPDATE currency_accounts SET balance = 100 WHERE character_id = $1", b.char); err != nil {
		t.Fatal(err)
	}
	var lack *trade.InsufficientError
	if _, err := e.svc.Accept(ctx, b.user, id, true, key()); !errors.As(err, &lack) || lack.Side != "self" {
		t.Fatalf("receiver short: %v", err)
	}
	if e.herbs(t, a.char) != 50 || e.herbs(t, b.char) != 50 {
		t.Fatal("partial move")
	}
	e.grant(t, b.char, 900, 0)
	if _, err := e.svc.Accept(ctx, b.user, id, true, key()); err != nil {
		t.Fatal(err)
	}
	// B paid 150 + fee 8 (ceil 7.5); A gave 40 herbs, no fee on items.
	if e.herbs(t, a.char) != 10 || e.herbs(t, b.char) != 90 || e.stones(t, a.char) != 1150 || e.stones(t, b.char) != 1000-150-8 {
		t.Fatalf("after swap: herbs %d %d stones %d %d", e.herbs(t, a.char), e.herbs(t, b.char), e.stones(t, a.char), e.stones(t, b.char))
	}
}

func TestSenderSpendsBetweenOfferAndAccept(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	a, b := e.player(t, 30, 2, 200, 0), e.ok(t)
	id := e.offer(t, a, b, stones(150), nil) // needs 150 + 8 fee
	// A spends the stones elsewhere (e.g. the shop) before B accepts.
	if err := postgres.InTx(ctx, e.pool, func(tx pgx.Tx) error {
		_, err := economy.Credit(ctx, db.New(tx), a.char, economy.SpiritStone, -100, economy.Ref{Reason: "shop_buy", IdemKey: "spent-1"})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	var lack *trade.InsufficientError
	if _, err := e.svc.Accept(ctx, b.user, id, true, key()); !errors.As(err, &lack) || lack.Side != "other" {
		t.Fatalf("want sender insufficient, got %v", err)
	}
	if e.stones(t, a.char) != 100 || e.stones(t, b.char) != 1000 {
		t.Fatal("stones moved on a failed accept")
	}
	// Exactly enough except the fee → still refused (the fee is part of the deal).
	e.grant(t, a.char, 50, 0)
	if _, err := e.svc.Accept(ctx, b.user, id, true, key()); !errors.As(err, &lack) || lack.AssetID != trade.SpiritStone {
		t.Fatalf("fee not covered: %v", err)
	}
	e.grant(t, a.char, 8, 0)
	if _, err := e.svc.Accept(ctx, b.user, id, true, key()); err != nil {
		t.Fatalf("now covered: %v", err)
	}
	if e.stones(t, a.char) != 0 {
		t.Fatalf("sender left %d", e.stones(t, a.char))
	}
}

func TestConcurrentAcceptsMoveOnce(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	a, b := e.ok(t), e.ok(t)
	id := e.offer(t, a, b, stones(200), nil)
	var wg sync.WaitGroup
	errs := make([]error, 8)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = e.svc.Accept(ctx, b.user, id, true, key())
		}()
	}
	wg.Wait()
	okN := 0
	for _, err := range errs {
		if err == nil {
			okN++
		} else if !errors.Is(err, trade.ErrOfferGone) {
			t.Fatalf("unexpected: %v", err)
		}
	}
	if okN != 1 || e.stones(t, b.char) != 1200 || e.stones(t, a.char) != 1000-200-10 {
		t.Fatalf("ok=%d stones a=%d b=%d", okN, e.stones(t, a.char), e.stones(t, b.char))
	}
}

func TestAcceptVersusCancel(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	for range 5 {
		a, b := e.ok(t), e.ok(t)
		id := e.offer(t, a, b, stones(100), nil)
		var wg sync.WaitGroup
		var accErr, canErr error
		wg.Add(2)
		go func() { defer wg.Done(); _, accErr = e.svc.Accept(ctx, b.user, id, true, key()) }()
		go func() { defer wg.Done(); _, canErr = e.svc.Cancel(ctx, a.user, id, key()) }()
		wg.Wait()
		if (accErr == nil) == (canErr == nil) {
			t.Fatalf("exactly one must win: accept=%v cancel=%v", accErr, canErr)
		}
		var status string
		if err := e.pool.QueryRow(ctx, "SELECT status FROM trade_offers WHERE id = $1", id).Scan(&status); err != nil {
			t.Fatal(err)
		}
		if accErr == nil {
			if status != "accepted" || e.stones(t, b.char) != 1100 {
				t.Fatalf("accept won but %s / %d", status, e.stones(t, b.char))
			}
		} else if status != "cancelled" || e.stones(t, b.char) != 1000 || e.stones(t, a.char) != 1000 {
			t.Fatalf("cancel won but %s / %d / %d", status, e.stones(t, a.char), e.stones(t, b.char))
		}
	}
}

// A→B and B→A answered at the same time: both lock both parties in character_id order, so they serialize.
func TestCrossTradesDoNotDeadlock(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	for range 5 {
		a, b := e.ok(t), e.ok(t)
		ab := e.offer(t, a, b, stones(100), herbs(5))
		ba := e.offer(t, b, a, herbs(10), stones(50))
		total := e.totalStones(t)
		done := make(chan [2]error, 1)
		go func() {
			var wg sync.WaitGroup
			var r [2]error
			wg.Add(2)
			go func() { defer wg.Done(); _, r[0] = e.svc.Accept(ctx, b.user, ab, true, key()) }()
			go func() { defer wg.Done(); _, r[1] = e.svc.Accept(ctx, a.user, ba, true, key()) }()
			wg.Wait()
			done <- r
		}()
		select {
		case r := <-done:
			if r[0] != nil || r[1] != nil {
				t.Fatalf("cross accepts: %v / %v", r[0], r[1])
			}
		case <-time.After(15 * time.Second):
			t.Fatal("cross trades deadlocked")
		}
		// ab: a gives 100 (+5 fee) for b's 5 herbs; ba: b gives 10 herbs for a's 50 (+3 fee, ceil 2.5).
		// a: 1000−105−53 = 842 stones, 50+5+10 = 65 herbs; b: 1000+100+50 = 1150 stones, 50−5−10 = 35 herbs.
		if e.stones(t, a.char) != 842 || e.herbs(t, a.char) != 65 || e.stones(t, b.char) != 1150 || e.herbs(t, b.char) != 35 {
			t.Fatalf("a %d/%d b %d/%d", e.stones(t, a.char), e.herbs(t, a.char), e.stones(t, b.char), e.herbs(t, b.char))
		}
		if burned := total - e.totalStones(t); burned != 5+3 {
			t.Fatalf("burned %d", burned)
		}
	}
}

func TestCapsAtTheBoundary(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	a, b, c := e.ok(t), e.ok(t), e.ok(t)
	// Static cap at creation: a stranger may never receive more than 300 value in one day.
	if _, err := e.svc.Create(ctx, a.user, b.char, stones(301), nil, key()); limitCode(err) != "other:"+trade.LimitReceived {
		t.Fatalf("over the stranger cap at create: %v", err)
	}
	first := e.offer(t, a, b, stones(200), nil)
	second := e.offer(t, c, b, stones(101), nil)
	third := e.offer(t, c, b, stones(100), nil)
	if _, err := e.svc.Accept(ctx, b.user, first, true, key()); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Accept(ctx, b.user, second, true, key()); limitCode(err) != "self:"+trade.LimitReceived {
		t.Fatalf("201 > 300 remaining: %v", err)
	}
	if _, err := e.svc.Accept(ctx, b.user, third, true, key()); err != nil {
		t.Fatalf("exactly 300: %v", err)
	}
	// Same faction → 600 cap; đạo lữ → 1000.
	d, f := e.ok(t), e.ok(t)
	for _, p := range []player{d, f} {
		if _, err := e.pool.Exec(ctx, "INSERT INTO character_faction (character_id, faction_id, joined_at) VALUES ($1, 'huyen_khung_mon', now())", p.char); err != nil {
			t.Fatal(err)
		}
	}
	pv, err := e.svc.Peer(ctx, d.user, f.char)
	if err != nil || pv.Relation != trade.RelFaction || pv.ReceiveCap != 600 {
		t.Fatalf("peer: %+v %v", pv, err)
	}
	id := e.offer(t, d, f, stones(600), nil)
	if _, err := e.svc.Accept(ctx, f.user, id, true, key()); err != nil {
		t.Fatalf("faction 600: %v", err)
	}
	// Offers per day (5) at creation.
	g := e.player(t, 30, 2, 5000, 0)
	for i := range 5 {
		if _, err := e.svc.Create(ctx, g.user, e.ok(t).char, stones(10), nil, key()); err != nil {
			t.Fatalf("offer %d: %v", i, err)
		}
	}
	if _, err := e.svc.Create(ctx, g.user, b.char, stones(1), nil, key()); limitCode(err) != "self:"+trade.LimitOffersPerDay && !errors.Is(err, trade.ErrTooManyOpen) {
		t.Fatalf("6th offer: %v", err)
	}
	// Next game day the counters reset.
	e.advance(24 * time.Hour)
	if _, err := e.svc.Create(ctx, g.user, b.char, stones(1), nil, key()); err != nil {
		t.Fatalf("next day: %v", err)
	}
}

func TestGatesAndUntradables(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	a := e.ok(t)
	newAcc := e.player(t, 2, 2, 0, 0)
	lowTier := e.player(t, 30, 1, 0, 0)
	var gate *trade.GateError
	if _, err := e.svc.Create(ctx, a.user, newAcc.char, stones(10), nil, key()); !errors.As(err, &gate) || gate.Side != "other" ||
		gate.Action != "receive" || strings.Join(gate.Blockers, ",") != "account_age,character_age" {
		t.Fatalf("new account receives: %v", err)
	}
	if _, err := e.svc.Create(ctx, a.user, lowTier.char, stones(10), nil, key()); !errors.As(err, &gate) || strings.Join(gate.Blockers, ",") != "tier" {
		t.Fatalf("low tier receives: %v", err)
	}
	// A brand-new account cannot send either (send gate) — and in a swap a new account cannot receive the taken side.
	fresh := e.player(t, 0, 2, 500, 0)
	if _, err := e.svc.Create(ctx, fresh.user, a.char, stones(10), nil, key()); !errors.As(err, &gate) || gate.Side != "self" || gate.Action != "send" {
		t.Fatalf("fresh sender: %v", err)
	}
	// Crafted requests with untradable ids: artifact, requirement item, shop item, premium, unknown.
	if err := postgres.InTx(ctx, e.pool, func(tx pgx.Tx) error {
		_, err := economy.GrantItem(ctx, db.New(tx), a.char, "cuu_tieu_hoan", 1, economy.Ref{Reason: "admin_grant", IdemKey: "art-1"})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	b := e.ok(t)
	for _, id := range []string{"cuu_tieu_hoan", "truc_co_dan", "tu_linh_dan", "premium", "khong_co"} {
		var le *trade.LineError
		if _, err := e.svc.Create(ctx, a.user, b.char, []trade.Line{{AssetID: id, Quantity: 1}}, nil, key()); !errors.As(err, &le) || le.Code != "not_tradable" {
			t.Fatalf("%s: %v", id, err)
		}
		if _, err := e.svc.Create(ctx, a.user, b.char, stones(1), []trade.Line{{AssetID: id, Quantity: 1}}, key()); !errors.As(err, &le) || le.Code != "not_tradable" {
			t.Fatalf("take %s: %v", id, err)
		}
	}
	if _, err := e.svc.Create(ctx, a.user, a.char, stones(1), nil, key()); !errors.Is(err, trade.ErrSelf) {
		t.Fatalf("self: %v", err)
	}
	// An offer whose line was smuggled in by SQL is refused at accept (re-validated against the catalogue).
	id := e.offer(t, a, b, stones(10), nil)
	if _, err := e.pool.Exec(ctx, "INSERT INTO trade_lines (offer_id, side, asset_kind, asset_id, quantity) VALUES ($1, 'give', 'item', 'cuu_tieu_hoan', 1)", id); err != nil {
		t.Fatal(err)
	}
	var le *trade.LineError
	if _, err := e.svc.Accept(ctx, b.user, id, true, key()); !errors.As(err, &le) || le.AssetID != "cuu_tieu_hoan" {
		t.Fatalf("smuggled artifact: %v", err)
	}
}

func TestIdempotencyAndExpiry(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	a, b := e.ok(t), e.ok(t)
	k := key()
	if _, err := e.svc.Create(ctx, a.user, b.char, stones(10), nil, k); err != nil {
		t.Fatal(err)
	}
	v, err := e.svc.Create(ctx, a.user, b.char, stones(10), nil, k)
	if err != nil || !v.Replayed || len(v.Outgoing) != 1 || v.Limits.OffersLeft != e.cb.Trade.Daily.OffersPerDay-1 {
		t.Fatalf("create replay: %d offers, left %d, %v", len(v.Outgoing), v.Limits.OffersLeft, err)
	}
	c := e.ok(t)
	if _, err := e.svc.Create(ctx, a.user, c.char, stones(10), nil, k); !errors.Is(err, trade.ErrKeyReused) {
		t.Fatalf("key reuse: %v", err)
	}
	if _, err := e.svc.Create(ctx, a.user, b.char, stones(11), nil, k); !errors.Is(err, trade.ErrKeyReused) {
		t.Fatalf("key reuse with edited lines: %v", err)
	}
	id := v.Outgoing[0].ID
	// Decline replay.
	dk := key()
	if _, err := e.svc.Decline(ctx, b.user, id, dk); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Decline(ctx, b.user, id, dk); err != nil {
		t.Fatalf("decline replay: %v", err)
	}
	if _, err := e.svc.Accept(ctx, b.user, id, true, key()); !errors.Is(err, trade.ErrOfferGone) {
		t.Fatalf("accept after decline: %v", err)
	}
	// Not your offer looks like a missing one.
	id2 := e.offer(t, a, b, stones(10), nil)
	if _, err := e.svc.Accept(ctx, c.user, id2, true, key()); !errors.Is(err, trade.ErrOfferGone) {
		t.Fatalf("stranger accept: %v", err)
	}
	if _, err := e.svc.Cancel(ctx, b.user, id2, key()); !errors.Is(err, trade.ErrOfferGone) {
		t.Fatalf("receiver cancel: %v", err)
	}
	// Expiry: lazily on touch, and by the worker sweep.
	id3 := e.offer(t, a, c, stones(10), nil)
	e.advance(e.cb.Trade.Expiry())
	if _, err := e.svc.Accept(ctx, b.user, id2, true, key()); !errors.Is(err, trade.ErrOfferGone) {
		t.Fatalf("expired accept: %v", err)
	}
	n, err := e.svc.ExpireDue(ctx, 100)
	if err != nil || n != 1 {
		t.Fatalf("sweep: %d %v", n, err)
	}
	var s2, s3 string
	_ = e.pool.QueryRow(ctx, "SELECT status FROM trade_offers WHERE id = $1", id2).Scan(&s2)
	_ = e.pool.QueryRow(ctx, "SELECT status FROM trade_offers WHERE id = $1", id3).Scan(&s3)
	if s2 != "expired" || s3 != "expired" || e.stones(t, b.char) != 1000 {
		t.Fatalf("statuses %s %s", s2, s3)
	}
	if n, _ := e.svc.ExpireDue(ctx, 100); n != 0 {
		t.Fatal("sweep is not idempotent")
	}
}

func TestNetFlagReportAndAdminFlows(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	a, b, c, d := e.ok(t), e.ok(t), e.ok(t), e.ok(t)
	for _, p := range []player{a, b} {
		if err := e.net.Record(ctx, p.user, "203.0.113.7"); err != nil {
			t.Fatal(err)
		}
	}
	id := e.offer(t, a, b, stones(300), nil) // flagged, not blocked
	if _, err := e.svc.Accept(ctx, b.user, id, true, key()); err != nil {
		t.Fatal(err)
	}
	var flag bool
	var flagEvents int
	_ = e.pool.QueryRow(ctx, "SELECT net_flag FROM trade_offers WHERE id = $1", id).Scan(&flag)
	_ = e.pool.QueryRow(ctx, "SELECT count(*) FROM trade_events WHERE offer_id = $1 AND kind = 'net_flag'", id).Scan(&flagEvents)
	if !flag || flagEvents != 1 {
		t.Fatalf("net flag %v / %d", flag, flagEvents)
	}
	// Funnel: c and d also feed b on the next days.
	for _, from := range []player{c, d} {
		e.advance(24 * time.Hour)
		x := e.offer(t, from, b, stones(250), nil)
		if _, err := e.svc.Accept(ctx, b.user, x, true, key()); err != nil {
			t.Fatal(err)
		}
	}
	f, err := e.svc.Flows(ctx, 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.TopNetReceivers) == 0 || f.TopNetReceivers[0].CharacterID != b.char || f.TopNetReceivers[0].Net != 800 || f.TopNetReceivers[0].DistinctSenders != 3 {
		t.Fatalf("top receivers: %+v", f.TopNetReceivers)
	}
	if len(f.Funnels) != 1 || f.Funnels[0].CharacterID != b.char || len(f.NetFlags) != 1 || f.NetFlags[0].OfferID != id {
		t.Fatalf("funnels %+v flags %+v", f.Funnels, f.NetFlags)
	}
	if len(f.LopsidedPairs) != 0 { // each pair moved < 600 value
		t.Fatalf("pairs: %+v", f.LopsidedPairs)
	}
	// Report: a party reports the other about the offer; a non-party cannot.
	rep := &report.Service{Pool: e.pool, Cat: e.cb.Moderation, Now: func() time.Time { return *e.clock }}
	r, err := rep.SubmitTrade(ctx, b.user, id, "mua bán ngoài game", "rep-trade-1")
	if err != nil || r.TargetCharacterID != a.char || r.Reason != report.ReasonTradeAbuse || r.TradeOfferID == nil || *r.TradeOfferID != id {
		t.Fatalf("report: %+v %v", r, err)
	}
	if _, err := rep.SubmitTrade(ctx, c.user, id, "", "rep-trade-2"); !errors.Is(err, report.ErrInvalidReport) {
		t.Fatalf("non-party report: %v", err)
	}
	if _, err := rep.Submit(ctx, c.user, a.char, report.ReasonTradeAbuse, "", "rep-trade-3", 0); !errors.Is(err, report.ErrInvalidReport) {
		t.Fatalf("trade_abuse without offer: %v", err)
	}
}
