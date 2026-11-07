//go:build integration

package factionwork_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"tiendao/game/internal/content"
	"tiendao/game/internal/factionwork"
	"tiendao/game/internal/platform/testdb"
)

type env struct {
	pool  *pgxpool.Pool
	svc   *factionwork.Service
	clock *time.Time
	ctx   context.Context
	cap   int64
}

func setup(t *testing.T) *env {
	t.Helper()
	pool := testdb.New(t)
	cb, err := content.LoadAll("../../../../content/catalog")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 7, 3, 0, 0, 0, time.UTC)
	e := &env{pool: pool, clock: &now, ctx: context.Background(), cap: cb.FactionWorks.Settings.DailyCapPerCharacter}
	e.svc = &factionwork.Service{Pool: pool, Cat: cb.FactionWorks, Now: func() time.Time { return *e.clock }}
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

// player creates a character with stones, optionally in a faction (with a daily_contribution row as merit input).
func (e *env) player(t *testing.T, stones int64, factionID string) (uuid.UUID, uuid.UUID) {
	t.Helper()
	u, c := uuid.New(), uuid.New()
	e.exec(t, "INSERT INTO users (id, email) VALUES ($1, $2)", u, u.String()+"@t")
	e.exec(t, "INSERT INTO characters (id, user_id, name) VALUES ($1, $2, $3)", c, u, "Đệ Tử "+c.String()[:6])
	e.exec(t, "INSERT INTO character_progressions (character_id, realm_id, stage_id, last_cultivated_at, cultivation_milli) VALUES ($1, 'luyen_khi', 'luyen_khi.tang_1', $2, 12345)", c, *e.clock)
	e.exec(t, "INSERT INTO currency_accounts (character_id, currency, balance) VALUES ($1, 'spirit_stone', $2)", c, stones)
	if factionID != "" {
		e.exec(t, "INSERT INTO character_faction (character_id, faction_id, joined_at) VALUES ($1, $2, $3)", c, factionID, *e.clock)
	}
	return u, c
}

func (e *env) balance(t *testing.T, c uuid.UUID) int64 {
	return e.scalar(t, "SELECT balance FROM currency_accounts WHERE character_id=$1 AND currency='spirit_stone'", c)
}

func key() string { return "don" + uuid.NewString()[:12] }

func TestDonateIdempotentLedgered(t *testing.T) {
	e := setup(t)
	u, c := e.player(t, 1000, "huyen_khung_mon")
	r, err := e.svc.Donate(e.ctx, u, 300, "donate-key-0001")
	if err != nil {
		t.Fatal(err)
	}
	if r.Replayed || r.State.Mine == nil || r.State.Mine.Progress.Total != 300 || r.State.TodayDonated != 300 || r.State.Balance != 700 ||
		len(r.State.Mine.Donors) != 1 || r.State.Mine.MyTotal != 300 {
		t.Fatalf("donate: %+v mine %+v", r, r.State.Mine)
	}
	again, err := e.svc.Donate(e.ctx, u, 300, "donate-key-0001")
	if err != nil || !again.Replayed || again.DonationID != r.DonationID {
		t.Fatalf("replay: %+v %v", again, err)
	}
	if _, err := e.svc.Donate(e.ctx, u, 200, "donate-key-0001"); !errors.Is(err, factionwork.ErrKeyReuse) {
		t.Fatalf("key reuse: %v", err)
	}
	if e.balance(t, c) != 700 || e.scalar(t, "SELECT donated_total FROM faction_treasuries WHERE faction_id='huyen_khung_mon'") != 300 ||
		e.scalar(t, "SELECT COUNT(*) FROM currency_ledger WHERE character_id=$1 AND reason='faction_donation' AND delta=-300 AND balance_after=700", c) != 1 ||
		e.scalar(t, "SELECT COUNT(*) FROM faction_donations") != 1 {
		t.Fatal("double effect or missing ledger")
	}
}

func TestDonateCapsAndRefusals(t *testing.T) {
	e := setup(t)
	u, c := e.player(t, 100_000, "huyen_khung_mon")
	if _, err := e.svc.Donate(e.ctx, u, e.cap, key()); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Donate(e.ctx, u, 10, key()); !errors.Is(err, factionwork.ErrDailyCap) {
		t.Fatalf("over cap: %v", err)
	}
	if _, err := e.svc.Donate(e.ctx, u, 5, key()); !errors.Is(err, factionwork.ErrBadAmount) {
		t.Fatalf("below min: %v", err)
	}
	// Switching faction does not reset the cap (per character, per game day).
	e.exec(t, "UPDATE character_faction SET faction_id='kim_sa_mon' WHERE character_id=$1", c)
	if _, err := e.svc.Donate(e.ctx, u, 10, key()); !errors.Is(err, factionwork.ErrDailyCap) {
		t.Fatalf("cap after switch: %v", err)
	}
	// Next game day: cap resets; the donation goes to the CURRENT faction.
	*e.clock = e.clock.Add(24 * time.Hour)
	if r, err := e.svc.Donate(e.ctx, u, 100, key()); err != nil || r.State.Mine.FactionID != "kim_sa_mon" {
		t.Fatalf("next day: %+v %v", r, err)
	}
	poor, pc := e.player(t, 50, "huyen_khung_mon")
	if _, err := e.svc.Donate(e.ctx, poor, 60, key()); !errors.Is(err, factionwork.ErrInsufficient) {
		t.Fatalf("insufficient: %v", err)
	}
	if e.balance(t, pc) != 50 || e.scalar(t, "SELECT COUNT(*) FROM faction_donation_days WHERE character_id=$1", pc) != 0 {
		t.Fatal("partial write on refusal")
	}
	loner, _ := e.player(t, 1000, "")
	if _, err := e.svc.Donate(e.ctx, loner, 100, key()); !errors.Is(err, factionwork.ErrNoFaction) {
		t.Fatalf("no faction: %v", err)
	}
	if v, err := e.svc.View(e.ctx, loner); err != nil || v.Mine != nil || len(v.Factions) != 2 {
		t.Fatalf("loner view: %+v %v", v, err)
	}
}

// Concurrency: 10 donations of 100 with different keys against a 500 cap → exactly 5 succeed; the same key ×8 → one.
// Many members of one faction donating at once → the pool equals the sum of their ledger rows.
func TestDonateConcurrent(t *testing.T) {
	e := setup(t)
	u, c := e.player(t, 100_000, "huyen_khung_mon")
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok := 0
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := e.svc.Donate(e.ctx, u, 100, key())
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				ok++
			} else if !errors.Is(err, factionwork.ErrDailyCap) {
				t.Errorf("race: %v", err)
			}
		}()
	}
	wg.Wait()
	if want := int(e.cap / 100); ok != want || e.balance(t, c) != 100_000-e.cap {
		t.Fatalf("ok %d (want %d) balance %d", ok, want, e.balance(t, c))
	}
	u2, c2 := e.player(t, 1000, "huyen_khung_mon")
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := e.svc.Donate(e.ctx, u2, 200, "same-key-race1"); err != nil {
				t.Errorf("same key: %v", err)
			}
		}()
	}
	wg.Wait()
	if e.balance(t, c2) != 800 {
		t.Fatalf("same key double debit: %d", e.balance(t, c2))
	}
	for i := 0; i < 12; i++ {
		mu2, _ := e.player(t, 1000, "huyen_khung_mon")
		wg.Add(1)
		go func(u uuid.UUID) {
			defer wg.Done()
			if _, err := e.svc.Donate(e.ctx, u, 250, key()); err != nil {
				t.Errorf("member: %v", err)
			}
		}(mu2)
	}
	wg.Wait()
	pool := e.scalar(t, "SELECT donated_total FROM faction_treasuries WHERE faction_id='huyen_khung_mon'")
	ledger := e.scalar(t, "SELECT -SUM(delta) FROM currency_ledger WHERE reason='faction_donation'")
	donors := e.scalar(t, "SELECT SUM(total) FROM faction_donors WHERE faction_id='huyen_khung_mon'")
	if pool != ledger || pool != donors || pool != e.cap+200+12*250 {
		t.Fatalf("pool %d ledger %d donors %d", pool, ledger, donors)
	}
}

// INVARIANT (decisions 2026-11-07): a donation never changes faction merit (daily_contribution), ranking (outbox /
// leaderboard events), điểm duyên (vip_accounts, vip_point_ledger) or tu vi (character_progressions).
func TestDonateFeedsNoProgressMeritRankOrDuyen(t *testing.T) {
	e := setup(t)
	u, c := e.player(t, 10_000, "huyen_khung_mon")
	e.exec(t, "INSERT INTO vip_accounts (user_id, points) VALUES ($1, 42)", u)
	snapshot := func() string {
		var s string
		q := `SELECT concat_ws('|',
			(SELECT COUNT(*) FROM daily_contribution),
			(SELECT COALESCE(SUM(points_milli),0) FROM daily_contribution),
			(SELECT COUNT(*) FROM outbox_events),
			(SELECT COUNT(*) FROM vip_point_ledger),
			(SELECT points FROM vip_accounts WHERE user_id=$1),
			(SELECT concat_ws(',', stage_id, cultivation_milli, version) FROM character_progressions WHERE character_id=$2),
			(SELECT concat_ws(',', faction_id, version) FROM character_faction WHERE character_id=$2))`
		if err := e.pool.QueryRow(e.ctx, q, u, c).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	before := snapshot()
	for i := 0; i < 3; i++ {
		if _, err := e.svc.Donate(e.ctx, u, 150, fmt.Sprintf("noeffect-key-%d", i)); err != nil {
			t.Fatal(err)
		}
	}
	if after := snapshot(); after != before {
		t.Fatalf("donation changed progress/merit/rank/duyên state:\nbefore %s\nafter  %s", before, after)
	}
}
