//go:build integration

package bond_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"tiendao/game/internal/content"
	"tiendao/game/internal/contribution"
	"tiendao/game/internal/cultivation"
	"tiendao/game/internal/daolu/bond"
	"tiendao/game/internal/db"
	"tiendao/game/internal/netsig"
	"tiendao/game/internal/platform/postgres"
	"tiendao/game/internal/platform/redisx"
	"tiendao/game/internal/platform/testdb"
	"tiendao/game/internal/protector"
)

type player struct{ user, char uuid.UUID }

type env struct {
	pool  *pgxpool.Pool
	rdb   *redis.Client
	cb    *content.Bundle
	cult  *cultivation.Service
	svc   *bond.Service
	net   *netsig.Store
	clock *time.Time
	n     int
}

func setup(t *testing.T, withRedis bool) *env {
	t.Helper()
	pool := testdb.New(t)
	cb, err := content.LoadAll("../../../../../content/catalog")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 6, 1, 3, 0, 0, 0, time.UTC) // 10:00 game time (UTC+7)
	clock := &now
	nowFn := func() time.Time { return *clock }
	cult := &cultivation.Service{Pool: pool, Realms: cb.Realms, Contribution: cb.Contribution, VIP: cb.VIP, Now: nowFn}
	ns := &netsig.Store{Pool: pool, Hasher: &netsig.Hasher{Keys: []netsig.Key{{ID: "t1", Secret: []byte("test-salt-test-salt-test-salt-test-salt")}}}, Now: nowFn}
	e := &env{pool: pool, cb: cb, cult: cult, net: ns, clock: clock,
		svc: &bond.Service{Pool: pool, Cfg: cb.DaoLu, Realms: cb.Realms, Cultivation: cult, VIP: cb.VIP, Net: ns, Now: nowFn}}
	if withRedis {
		url := os.Getenv("TEST_REDIS_URL")
		if url == "" {
			url = "redis://localhost:2379/15"
		}
		rdb, err := redisx.Connect(context.Background(), url)
		if err != nil {
			t.Fatalf("%v (run `make db-up`)", err)
		}
		t.Cleanup(func() { rdb.Close() })
		e.rdb = rdb
		e.svc.Session = &bond.Session{R: rdb, Cfg: cb.DaoLu}
	}
	return e
}

// player creates an account `accountDays` old with a character at `tier` (stage 1 of that realm, default path).
func (e *env) player(t *testing.T, accountDays, tier int) player {
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
	return player{user, char}
}

func (e *env) ok(t *testing.T) player { return e.player(t, 30, 2) }

func key() string { return strings.ReplaceAll(uuid.NewString(), "-", "") }

func (e *env) bond(t *testing.T, a, b player) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	v, err := e.svc.Propose(ctx, a.user, b.char, true, key())
	if err != nil || len(v.Outgoing) != 1 {
		t.Fatalf("propose: %+v %v", v.Outgoing, err)
	}
	w, err := e.svc.Accept(ctx, b.user, v.Outgoing[0].ProposalID, true, key())
	if err != nil || w.Bond == nil {
		t.Fatalf("accept: %v", err)
	}
	return w.Bond.BondID
}

func gateBlockers(err error) []string {
	var g *bond.GateError
	if errors.As(err, &g) {
		return g.Blockers
	}
	return nil
}

func TestProposalFlowGatesExpiryDecline(t *testing.T) {
	e := setup(t, false)
	ctx := context.Background()
	a, b := e.ok(t), e.ok(t)
	young, low := e.player(t, 3, 2), e.player(t, 30, 1)

	if _, err := e.svc.Propose(ctx, a.user, b.char, false, key()); !errors.Is(err, bond.ErrNotAcknowledged) {
		t.Fatalf("no ack: %v", err)
	}
	if _, err := e.svc.Propose(ctx, a.user, a.char, true, key()); !errors.Is(err, bond.ErrSelf) {
		t.Fatalf("self: %v", err)
	}
	if _, err := e.svc.Propose(ctx, a.user, young.char, true, key()); fmt.Sprint(gateBlockers(err)) != "[account_age]" {
		t.Fatalf("young target: %v", err)
	}
	if _, err := e.svc.Propose(ctx, low.user, a.char, true, key()); fmt.Sprint(gateBlockers(err)) != "[tier]" {
		t.Fatalf("low realm proposer: %v", err)
	}
	// Propose → idempotent replay → one open outgoing at a time.
	k := key()
	v, err := e.svc.Propose(ctx, a.user, b.char, true, k)
	if err != nil || len(v.Outgoing) != 1 || v.ProposalsLeft != e.cb.DaoLu.Bond.DailyProposals-1 {
		t.Fatalf("propose: %+v %v", v, err)
	}
	if v2, err := e.svc.Propose(ctx, a.user, b.char, true, k); err != nil || len(v2.Outgoing) != 1 {
		t.Fatalf("replay: %v", err)
	}
	if _, err := e.svc.Propose(ctx, a.user, b.char, true, k[:len(k)-1]+"x"); !errors.Is(err, bond.ErrOpenProposal) {
		t.Fatalf("second open: %v", err)
	}
	c := e.ok(t)
	if _, err := e.svc.Propose(ctx, a.user, c.char, true, k); !errors.Is(err, bond.ErrKeyReused) {
		t.Fatalf("key reuse: %v", err)
	}
	// B sees it; accept needs the second confirmation; B declines (idempotent).
	bv, _ := e.svc.State(ctx, b.user)
	if len(bv.Incoming) != 1 || bv.Incoming[0].CharacterID != a.char.String() {
		t.Fatalf("incoming: %+v", bv.Incoming)
	}
	pid := bv.Incoming[0].ProposalID
	if _, err := e.svc.Accept(ctx, b.user, pid, false, key()); !errors.Is(err, bond.ErrNotAcknowledged) {
		t.Fatalf("accept without ack: %v", err)
	}
	if _, err := e.svc.Accept(ctx, c.user, pid, true, key()); !errors.Is(err, bond.ErrNotYourProposal) {
		t.Fatalf("stranger accept: %v", err)
	}
	dk := key()
	if _, err := e.svc.Decline(ctx, b.user, pid, dk); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Decline(ctx, b.user, pid, dk); err != nil {
		t.Fatalf("decline replay: %v", err)
	}
	if _, err := e.svc.Accept(ctx, b.user, pid, true, key()); !errors.Is(err, bond.ErrProposalGone) {
		t.Fatalf("accept declined: %v", err)
	}
	// Review L2: the same proposer waits decline_cooldown_days before proposing to the same target again.
	if _, err := e.svc.Propose(ctx, a.user, b.char, true, key()); !errors.Is(err, bond.ErrDeclinedRecently) {
		t.Fatalf("re-propose after decline: %v", err)
	}
	*e.clock = e.clock.Add(e.cb.DaoLu.DeclineCooldown())
	// Expiry: a new proposal lapses after proposal_expiry_hours; accept fails, the proposer may propose again.
	v, err = e.svc.Propose(ctx, a.user, b.char, true, key())
	if err != nil {
		t.Fatal(err)
	}
	*e.clock = e.clock.Add(e.cb.DaoLu.ProposalExpiry() + time.Second)
	if _, err := e.svc.Accept(ctx, b.user, v.Outgoing[0].ProposalID, true, key()); !errors.Is(err, bond.ErrProposalGone) {
		t.Fatalf("expired accept: %v", err)
	}
	if bv, _ := e.svc.State(ctx, b.user); len(bv.Incoming) != 0 {
		t.Fatal("expired proposal still listed")
	}
	// Daily limit (counted per game day, any outcome): cancel to free the open slot each time.
	for i := 0; ; i++ {
		v, err := e.svc.Propose(ctx, a.user, b.char, true, key())
		if errors.Is(err, bond.ErrDailyLimit) {
			if i != int(e.cb.DaoLu.Bond.DailyProposals) {
				t.Fatalf("limit after %d", i)
			}
			break
		}
		if err != nil || i > 10 {
			t.Fatalf("propose %d: %v", i, err)
		}
		if _, err := e.svc.Cancel(ctx, a.user, v.Outgoing[0].ProposalID, key()); err != nil {
			t.Fatal(err)
		}
	}
}

func activeBonds(t *testing.T, e *env, char uuid.UUID) int {
	var n int
	if err := e.pool.QueryRow(context.Background(), "SELECT count(*) FROM dao_lu_members WHERE character_id = $1 AND active", char).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// THE invariant: A proposes to B while C proposes to A; B and A accept at the same time → exactly one bond.
func TestOneActiveBondUnderConcurrency(t *testing.T) {
	e := setup(t, false)
	ctx := context.Background()
	for round := range 8 {
		a, b, c := e.ok(t), e.ok(t), e.ok(t)
		ab, err := e.svc.Propose(ctx, a.user, b.char, true, key())
		if err != nil {
			t.Fatal(err)
		}
		ca, err := e.svc.Propose(ctx, c.user, a.char, true, key())
		if err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		errs := make([]error, 2)
		start := make(chan struct{})
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			_, errs[0] = e.svc.Accept(ctx, b.user, ab.Outgoing[0].ProposalID, true, key())
		}()
		go func() {
			defer wg.Done()
			<-start
			_, errs[1] = e.svc.Accept(ctx, a.user, ca.Outgoing[0].ProposalID, true, key())
		}()
		close(start)
		wg.Wait()
		okN := 0
		for _, err := range errs {
			switch {
			case err == nil:
				okN++
			case errors.Is(err, bond.ErrProposalGone), errors.Is(err, bond.ErrAlreadyBonded), gateBlockers(err) != nil:
			default:
				t.Fatalf("round %d: unexpected error %v", round, err)
			}
		}
		if okN != 1 || activeBonds(t, e, a.char) != 1 || activeBonds(t, e, b.char)+activeBonds(t, e, c.char) != 1 {
			t.Fatalf("round %d: ok=%d errs=%v", round, okN, errs)
		}
	}
	// The same proposal accepted twice at once (two keys): one bond; the same key replays.
	a, b := e.ok(t), e.ok(t)
	v, _ := e.svc.Propose(ctx, a.user, b.char, true, key())
	pid := v.Outgoing[0].ProposalID
	k := key()
	var wg sync.WaitGroup
	errs := make([]error, 3)
	for i := range 3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			kk := key()
			if i == 0 {
				kk = k
			}
			_, errs[i] = e.svc.Accept(ctx, b.user, pid, true, kk)
		}()
	}
	wg.Wait()
	if activeBonds(t, e, a.char) != 1 || activeBonds(t, e, b.char) != 1 {
		t.Fatalf("double accept: %v", errs)
	}
	if _, err := e.svc.Accept(ctx, b.user, pid, true, k); err != nil && errs[0] == nil {
		t.Fatalf("replay of the winning key: %v", err)
	}
	// A bonded character can neither propose nor be proposed to.
	d := e.ok(t)
	if _, err := e.svc.Propose(ctx, d.user, a.char, true, key()); fmt.Sprint(gateBlockers(err)) != "[bonded]" {
		t.Fatalf("propose to bonded: %v", err)
	}
}

func TestTerminationReasonsAndCooldown(t *testing.T) {
	e := setup(t, false)
	ctx := context.Background()
	admin := e.ok(t)
	if _, err := e.pool.Exec(ctx, "UPDATE users SET role = 'admin' WHERE id = $1", admin.user); err != nil {
		t.Fatal(err)
	}
	a, b, x := e.ok(t), e.ok(t), e.ok(t)
	bid := e.bond(t, a, b)
	// No voluntary divorce exists in the API; admin needs a report concerning a partner + a note.
	report := func(reporter player, target uuid.UUID) uuid.UUID {
		var id uuid.UUID
		if err := e.pool.QueryRow(ctx, `INSERT INTO reports (reporter_user_id, target_character_id, target_name, reason, idempotency_key)
			VALUES ($1, $2, 'x', 'harassment', $3) RETURNING id`, reporter.user, target, key()).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	unrelated := report(x, admin.char)
	if err := e.svc.AdminEnd(ctx, admin.user, bid, unrelated, "quấy rối"); !errors.Is(err, bond.ErrReportMismatch) {
		t.Fatalf("unrelated report: %v", err)
	}
	// Review L3: an outsider's report about a partner counts only once actioned; a dismissed report never counts.
	outsider := report(x, a.char)
	if err := e.svc.AdminEnd(ctx, admin.user, bid, outsider, "quấy rối"); !errors.Is(err, bond.ErrReportMismatch) {
		t.Fatalf("open outsider report: %v", err)
	}
	dismissed := report(b, a.char)
	if _, err := e.pool.Exec(ctx, "UPDATE reports SET status = 'dismissed', resolved_at = now(), resolved_by = $2 WHERE id = $1", dismissed, admin.user); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.AdminEnd(ctx, admin.user, bid, dismissed, "quấy rối"); !errors.Is(err, bond.ErrReportMismatch) {
		t.Fatalf("dismissed report: %v", err)
	}
	rep := report(b, a.char) // one partner against the other, still open: enough
	if err := e.svc.AdminEnd(ctx, admin.user, bid, rep, " "); !errors.Is(err, bond.ErrNoteRequired) {
		t.Fatalf("no note: %v", err)
	}
	if err := e.svc.AdminEnd(ctx, admin.user, bid, rep, "quấy rối kéo dài"); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.AdminEnd(ctx, admin.user, bid, rep, "lần hai"); !errors.Is(err, bond.ErrBondEnded) {
		t.Fatalf("end twice: %v", err)
	}
	var audits, ended int
	_ = e.pool.QueryRow(ctx, "SELECT count(*) FROM admin_audit_logs WHERE action = 'dao_lu.terminate' AND target_id = $1", bid.String()).Scan(&audits)
	_ = e.pool.QueryRow(ctx, "SELECT count(*) FROM dao_lu_events WHERE bond_id = $1 AND kind = 'ended' AND reason = 'admin' AND report_id = $2", bid, rep).Scan(&ended)
	if audits != 1 || ended != 1 || activeBonds(t, e, a.char) != 0 {
		t.Fatalf("audit %d, event %d", audits, ended)
	}
	// Cooldown: both wait rebond_cooldown_days.
	if _, err := e.svc.Propose(ctx, a.user, x.char, true, key()); fmt.Sprint(gateBlockers(err)) != "[cooldown]" {
		t.Fatalf("cooldown: %v", err)
	}
	if v, _ := e.svc.State(ctx, b.user); v.CooldownUntil == nil {
		t.Fatal("cooldown not shown")
	}
	*e.clock = e.clock.Add(e.cb.DaoLu.RebondCooldown())
	bond2 := e.bond(t, a, x)

	// Lượng kiếp hook (luân hồi), inside a caller's transaction.
	err := postgres.InTx(ctx, e.pool, func(tx pgx.Tx) error {
		if _, err := bond.EndForCharacterLocked(ctx, db.New(tx), x.char, bond.ReasonAdmin, *e.clock); !errors.Is(err, bond.ErrBadReason) {
			return fmt.Errorf("hook must refuse non-kiếp reasons: %v", err)
		}
		ok, err := bond.EndForCharacterLocked(ctx, db.New(tx), x.char, bond.ReasonLuanHoi, *e.clock)
		if !ok || err != nil {
			return fmt.Errorf("luân hồi hook: %v %v", ok, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var reason string
	_ = e.pool.QueryRow(ctx, "SELECT end_reason FROM dao_lu_bonds WHERE id = $1", bond2).Scan(&reason)
	if reason != bond.ReasonLuanHoi {
		t.Fatalf("reason %q", reason)
	}

	// Absence: p plays on, q disappears; after absence_days the worker ends the bond (bounded), not before.
	*e.clock = e.clock.Add(e.cb.DaoLu.RebondCooldown())
	p, q := e.ok(t), e.ok(t)
	bond3 := e.bond(t, p, q)
	*e.clock = e.clock.Add(e.cb.DaoLu.Absence() - time.Hour)
	if _, err := e.pool.Exec(ctx, "UPDATE character_progressions SET last_cultivated_at = $2 WHERE character_id = $1", p.char, *e.clock); err != nil {
		t.Fatal(err)
	}
	if n, err := e.svc.EndAbsent(ctx, 100); err != nil || n != 0 {
		t.Fatalf("too early: %d %v", n, err)
	}
	*e.clock = e.clock.Add(2 * time.Hour)
	if n, err := e.svc.EndAbsent(ctx, 100); err != nil || n != 1 {
		t.Fatalf("absence sweep: %d %v", n, err)
	}
	_ = e.pool.QueryRow(ctx, "SELECT end_reason FROM dao_lu_bonds WHERE id = $1", bond3).Scan(&reason)
	if reason != bond.ReasonAbsence {
		t.Fatalf("absence reason %q", reason)
	}
}

// ───── song tu ─────

func (e *env) setActive(t *testing.T, p player, on bool) {
	t.Helper()
	k := protector.ActiveKey(p.user)
	var err error
	if on {
		err = e.rdb.Set(context.Background(), k, 1, time.Hour).Err()
	} else {
		err = e.rdb.Del(context.Background(), k).Err()
	}
	if err != nil {
		t.Fatal(err)
	}
}

// beatBoth advances the clock in heartbeat steps for d, both partners beating (only those in `who`).
func (e *env) beat(t *testing.T, d time.Duration, who ...player) {
	t.Helper()
	step := time.Duration(e.cb.DaoLu.SongTu.HeartbeatSeconds) * time.Second
	for end := e.clock.Add(d); e.clock.Before(end); {
		*e.clock = e.clock.Add(step)
		for _, p := range who {
			if _, err := e.svc.HeartbeatSongTu(context.Background(), p.user); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestSongTuOverlapCapCeilingIdempotency(t *testing.T) {
	e := setup(t, true)
	ctx := context.Background()
	cfg := e.cb.DaoLu
	a, b := e.ok(t), e.ok(t)
	e.bond(t, a, b)
	t.Cleanup(func() {
		for _, p := range []player{a, b} {
			e.setActive(t, p, false)
		}
	})

	// Start needs BOTH active (a real game action recently — not a heartbeat).
	if _, err := e.svc.StartSongTu(ctx, a.user); !errors.Is(err, bond.ErrNotActive) {
		t.Fatalf("start inactive: %v", err)
	}
	e.setActive(t, a, true)
	if _, err := e.svc.StartSongTu(ctx, a.user); !errors.Is(err, bond.ErrPartnerNotActive) {
		t.Fatalf("partner inactive: %v", err)
	}
	if _, err := e.svc.HeartbeatSongTu(ctx, a.user); !errors.Is(err, bond.ErrNoSession) {
		t.Fatalf("beat without session: %v", err)
	}
	e.setActive(t, b, true)
	if _, err := e.svc.StartSongTu(ctx, a.user); err != nil {
		t.Fatal(err)
	}

	// Offline partner: only A beats for 10 min → nothing accrues.
	e.beat(t, 10*time.Minute, a)
	if v, _ := e.svc.HeartbeatSongTu(ctx, a.user); v.AccruedSeconds != 0 {
		t.Fatalf("offline partner accrued %d", v.AccruedSeconds)
	}
	if _, err := e.svc.ClaimSongTu(ctx, a.user, key()); !errors.Is(err, bond.ErrNothingToClaim) {
		t.Fatalf("claim nothing: %v", err)
	}
	// Both beat for 10 min (server clock) → ~600 s of overlap.
	e.beat(t, 10*time.Minute, a, b)
	v, _ := e.svc.HeartbeatSongTu(ctx, a.user)
	if v.AccruedSeconds < 570 || v.AccruedSeconds > 600 {
		t.Fatalf("overlap accrued %d, want ≈ 600", v.AccruedSeconds)
	}
	// Partner idle (no game action within the window): accrual stops although both still beat.
	e.setActive(t, b, false)
	before := v.AccruedSeconds
	e.beat(t, 5*time.Minute, a, b)
	if v, _ = e.svc.HeartbeatSongTu(ctx, a.user); v.AccruedSeconds != before {
		t.Fatalf("idle partner still accrued: %d → %d", before, v.AccruedSeconds)
	}
	e.setActive(t, b, true)

	// A claims: paid once, replay by key, nothing left; B claims the same overlap separately.
	k := key()
	r1, err := e.svc.ClaimSongTu(ctx, a.user, k)
	if err != nil || r1.Seconds != before || r1.RawMilli != cfg.PayMilli(before) || r1.Replayed {
		t.Fatalf("claim: %+v %v", r1, err)
	}
	if r2, err := e.svc.ClaimSongTu(ctx, a.user, k); err != nil || !r2.Replayed || r2.RawMilli != r1.RawMilli {
		t.Fatalf("replay: %+v %v", r2, err)
	}
	if _, err := e.svc.ClaimSongTu(ctx, a.user, key()); !errors.Is(err, bond.ErrNothingToClaim) {
		t.Fatalf("double claim: %v", err)
	}
	rb, err := e.svc.ClaimSongTu(ctx, b.user, key())
	if err != nil || rb.Seconds != r1.Seconds || rb.RawMilli != r1.RawMilli {
		t.Fatalf("partner claim: %+v %v", rb, err)
	}
	var credits int
	_ = e.pool.QueryRow(ctx, "SELECT count(*) FROM cultivation_credits WHERE source = 'dual_cultivation' AND bucket = 'community'").Scan(&credits)
	if credits != 2 {
		t.Fatalf("credits %d", credits)
	}

	// 2h cap: beat for 2.5h more → shared overlap capped at window_seconds; Σ paid per character = window.
	e.beat(t, 150*time.Minute, a, b)
	v, _ = e.svc.HeartbeatSongTu(ctx, a.user)
	if v.AccruedSeconds != cfg.SongTu.WindowSeconds {
		t.Fatalf("cap: accrued %d", v.AccruedSeconds)
	}
	// Ceiling: fill A's community bucket (another community source) so only part of the claim fits.
	cap := e.cb.Contribution.Buckets[contribution.Community].CapMilli
	err = postgres.InTx(ctx, e.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		p, err := q.GetProgressionForUpdate(ctx, a.char)
		if err != nil {
			return err
		}
		_, _, err = e.cult.CreditLocked(ctx, q, p, contribution.SourceFaction, cap-2000, cultivation.Ref{Type: "test", ID: "fill", IdemKey: "fill"}, *e.clock)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	r3, err := e.svc.ClaimSongTu(ctx, a.user, key())
	if err != nil || r3.Seconds != cfg.SongTu.WindowSeconds-r1.Seconds {
		t.Fatalf("cap claim: %+v %v", r3, err)
	}
	if room := 2000 - r1.EarnedMilli; r3.RawMilli <= room || r3.EarnedMilli != room {
		t.Fatalf("ceiling: raw %d earned %d (bucket room %d)", r3.RawMilli, r3.EarnedMilli, room)
	}
	var paid int
	_ = e.pool.QueryRow(ctx, "SELECT sum(seconds) FROM dao_lu_song_tu_claims WHERE character_id = $1", a.char).Scan(&paid)
	if int64(paid) != cfg.SongTu.WindowSeconds {
		t.Fatalf("paid %d > window", paid)
	}
	var community int64
	_ = e.pool.QueryRow(ctx, "SELECT community_milli FROM daily_contribution WHERE character_id = $1", a.char).Scan(&community)
	if community != cap {
		t.Fatalf("community %d, cap %d", community, cap)
	}
	// Điểm duyên dual_cultivation: once per day for each partner (the first claim crossing duyen_min_seconds).
	var duyen int
	_ = e.pool.QueryRow(ctx, "SELECT count(*) FROM vip_point_ledger WHERE activity_id = 'dual_cultivation'").Scan(&duyen)
	// A's first claim (~600 s) and B's (~600 s) stay below duyen_min_seconds; A's window claim crosses it → one row.
	if before >= cfg.SongTu.DuyenMinSeconds || duyen != 1 {
		t.Fatalf("điểm duyên rows %d", duyen)
	}
}

func TestNetSignal(t *testing.T) {
	e := setup(t, false)
	ctx := context.Background()
	a, b, c := e.ok(t), e.ok(t), e.ok(t)
	for _, r := range []struct {
		p  player
		ip string
	}{{a, "203.0.113.9"}, {b, "203.0.113.9"}, {c, "198.51.100.1"}, {a, "2001:db8:5:6::1"}, {c, "2001:db8:5:6::abcd"}} {
		if err := e.net.Record(ctx, r.p.user, r.ip); err != nil {
			t.Fatal(err)
		}
	}
	// Stored hashed only: 16-byte hashes, no column can hold the address text.
	var n, bad int
	_ = e.pool.QueryRow(ctx, "SELECT count(*), count(*) FILTER (WHERE octet_length(ip_hash) <> 16 OR position('203.0.113.9'::bytea in ip_hash) > 0) FROM net_signals").Scan(&n, &bad)
	if n != 5 || bad != 0 {
		t.Fatalf("rows %d bad %d", n, bad)
	}
	for _, cs := range []struct {
		x, y player
		want bool
	}{{a, b, true}, {a, c, true}, {b, c, false}, {a, a, false}} {
		if got, err := e.net.SharedRecently(ctx, cs.x.user, cs.y.user); err != nil || got != cs.want {
			t.Fatalf("shared(%v,%v) = %v %v", cs.x.char, cs.y.char, got, err)
		}
	}
	cl, err := e.net.Clusters(ctx, 30, 2, 50)
	if err != nil || len(cl) != 2 {
		t.Fatalf("clusters: %+v %v", cl, err)
	}
	for _, c := range cl {
		if len(c.Accounts) != 2 || c.Accounts[0].CharacterName == "" {
			t.Fatalf("cluster: %+v", c)
		}
	}
	// Proposals between accounts on the same network are flagged for admins, never blocked.
	if _, err := e.svc.Propose(ctx, a.user, b.char, true, key()); err != nil {
		t.Fatal(err)
	}
	admin := e.ok(t)
	flags, err := e.svc.Flags(ctx, admin.user, 30)
	if err != nil || len(flags) != 1 || flags[0].Stage != "propose" {
		t.Fatalf("flags: %+v %v", flags, err)
	}
	if err := e.net.AuditRead(ctx, admin.user, "netsig.clusters", map[string]any{"days": 30}); err != nil {
		t.Fatal(err)
	}
	var reads int
	_ = e.pool.QueryRow(ctx, "SELECT count(*) FROM admin_audit_logs WHERE admin_user_id = $1 AND target_type = 'admin_read'", admin.user).Scan(&reads)
	if reads != 2 {
		t.Fatalf("audited reads %d (review L7)", reads)
	}
	// Characters helper (hộ pháp pairs).
	if got, err := e.net.SharedRecentlyCharacters(ctx, a.char, b.char); err != nil || !got {
		t.Fatalf("shared by characters: %v %v", got, err)
	}
	// After the retention window: no longer shared; the purge deletes everything in bounded batches.
	*e.clock = e.clock.Add(netsig.Retention + time.Hour)
	if got, _ := e.net.SharedRecently(ctx, a.user, b.user); got {
		t.Fatal("still shared after 30 days")
	}
	if err := e.net.Record(ctx, c.user, "198.51.100.1"); err != nil { // c comes back: its fresh row survives
		t.Fatal(err)
	}
	if del, err := e.net.Purge(ctx, 2, 1); err != nil || del != 2 {
		t.Fatalf("bounded purge: %d %v", del, err)
	}
	if del, err := e.net.Purge(ctx, 100, 5); err != nil || del != 2 {
		t.Fatalf("purge rest: %d %v", del, err)
	}
	_ = e.pool.QueryRow(ctx, "SELECT count(*) FROM net_signals").Scan(&n)
	if n != 1 {
		t.Fatalf("left %d", n)
	}
	// Review M2: đạo lữ flags follow the same 30-day retention.
	if _, err := e.svc.PurgeNetFlags(ctx, 100); err != nil {
		t.Fatal(err)
	}
	var flagged, events int
	_ = e.pool.QueryRow(ctx, "SELECT count(*) FROM dao_lu_proposals WHERE net_flag").Scan(&flagged)
	_ = e.pool.QueryRow(ctx, "SELECT count(*) FROM dao_lu_events WHERE kind = 'net_flag'").Scan(&events)
	if flagged != 0 || events != 0 {
		t.Fatalf("flags kept past retention: %d proposals, %d events", flagged, events)
	}
}

// Review L2: a target's inbox of waiting proposals is capped (the proposer only sees "not eligible", side target).
func TestIncomingPendingCap(t *testing.T) {
	e := setup(t, false)
	ctx := context.Background()
	target := e.ok(t)
	for i := range int(e.cb.DaoLu.Bond.MaxIncomingPending) {
		p := e.ok(t)
		if _, err := e.svc.Propose(ctx, p.user, target.char, true, key()); err != nil {
			t.Fatalf("proposal %d: %v", i, err)
		}
	}
	late := e.ok(t)
	_, err := e.svc.Propose(ctx, late.user, target.char, true, key())
	var g *bond.GateError
	if !errors.As(err, &g) || g.Side != "target" || fmt.Sprint(g.Blockers) != "[inbox_full]" {
		t.Fatalf("inbox cap: %v", err)
	}
}
