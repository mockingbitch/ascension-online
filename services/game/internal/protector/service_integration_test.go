//go:build integration

package protector_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"tiendao/game/internal/artifact"
	"tiendao/game/internal/breakthrough"
	"tiendao/game/internal/character"
	"tiendao/game/internal/content"
	"tiendao/game/internal/cultivation"
	"tiendao/game/internal/db"
	"tiendao/game/internal/platform/postgres"
	"tiendao/game/internal/platform/testdb"
	"tiendao/game/internal/protector"
	"tiendao/game/internal/realm"
)

type fixedRNG int

func (f fixedRNG) IntN(int) int { return int(f) }

// fakePresence: online = in the set (tests flip it).
type fakePresence struct {
	mu sync.Mutex
	on map[uuid.UUID]bool
}

func (f *fakePresence) MarkActive(context.Context, uuid.UUID) error { return nil }

func (f *fakePresence) Ping(_ context.Context, id, _ uuid.UUID, _ string, _ time.Time) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.on[id] = true
	return true, nil
}

func (f *fakePresence) Online(_ context.Context, ids []uuid.UUID, _ time.Time) (map[uuid.UUID]bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[uuid.UUID]bool{}
	for _, id := range ids {
		if f.on[id] {
			out[id] = true
		}
	}
	return out, nil
}

func (f *fakePresence) FactionOnline(_ context.Context, _ string, _ time.Time, _ int64) ([]uuid.UUID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []uuid.UUID
	for id, on := range f.on {
		if on {
			out = append(out, id)
		}
	}
	return out, nil
}

func (f *fakePresence) set(id uuid.UUID, on bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.on[id] = on
}

type player struct{ user, char uuid.UUID }

type env struct {
	pool  *pgxpool.Pool
	cb    *content.Bundle
	cult  *cultivation.Service
	chars *character.Service
	prot  *protector.Service
	bt    *breakthrough.Service
	arts  *artifact.Service
	pres  *fakePresence
	clock *time.Time
	n     int
}

func setup(t *testing.T) *env {
	t.Helper()
	pool := testdb.New(t)
	cb, err := content.LoadAll("../../../../content/catalog")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 6, 1, 3, 0, 0, 0, time.UTC)
	clock := &now
	nowFn := func() time.Time { return *clock }
	cult := &cultivation.Service{Pool: pool, Realms: cb.Realms, Contribution: cb.Contribution, VIP: cb.VIP, Now: nowFn}
	pres := &fakePresence{on: map[uuid.UUID]bool{}}
	prot := &protector.Service{Pool: pool, Cfg: cb.Protector, Realms: cb.Realms, Presence: pres, Project: cult.ProjectedStage, Now: nowFn}
	e := &env{pool: pool, cb: cb, cult: cult, chars: &character.Service{Pool: pool, Cultivation: cult}, prot: prot, pres: pres, clock: clock,
		bt:   &breakthrough.Service{Pool: pool, Cultivation: cult, RNG: fixedRNG(9999), Items: cb.Items, Protectors: prot},
		arts: &artifact.Service{Pool: pool, Items: cb.Items, Now: nowFn}}
	prot.Ready = func(ctx context.Context, userID uuid.UUID) (bool, error) {
		pv, err := e.bt.Preview(ctx, userID)
		return pv.Ready && pv.Tribulation != nil, err
	}
	return e
}

func (e *env) player(t *testing.T, faction string) player {
	t.Helper()
	e.n++
	ctx := context.Background()
	user := uuid.New()
	if _, err := e.pool.Exec(ctx, "INSERT INTO users (id, email) VALUES ($1, $2)", user, user.String()+"@t"); err != nil {
		t.Fatal(err)
	}
	v, err := e.chars.Create(ctx, user, fmt.Sprintf("Đạo Hữu %c", 'A'+e.n))
	if err != nil {
		t.Fatal(err)
	}
	if faction != "" {
		if _, err := e.pool.Exec(ctx, "INSERT INTO character_faction (character_id, faction_id, joined_at) VALUES ($1, $2, $3)", v.ID, faction, *e.clock); err != nil {
			t.Fatal(err)
		}
	}
	// Protector eligibility (M2): an established character (30 days old) at Trúc Cơ.
	if _, err := e.pool.Exec(ctx, "UPDATE characters SET created_at = $2 WHERE id = $1", v.ID, e.clock.Add(-30*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	pl := player{user, v.ID}
	e.putAt(t, pl, e.firstOf(t, "truc_co"), 0)
	return pl
}

func (e *env) firstOf(t *testing.T, realmID string) *realm.Stage {
	for _, s := range e.cb.Realms.Ladder("tien") {
		if s.RealmID == realmID {
			return s
		}
	}
	t.Fatal("no stage")
	return nil
}

func (e *env) lastOf(t *testing.T, realmID string) *realm.Stage {
	for _, s := range e.cb.Realms.Ladder("tien") {
		if s.RealmID == realmID && s.IsLastOfRealm() {
			return s
		}
	}
	t.Fatal("no stage")
	return nil
}

func (e *env) putAt(t *testing.T, p player, stage *realm.Stage, cultivation int64) {
	t.Helper()
	if _, err := e.pool.Exec(context.Background(),
		"UPDATE character_progressions SET stage_id=$2, realm_id=split_part($2,'.',1), cultivation_milli=$3, last_cultivated_at=$4 WHERE character_id=$1",
		p.char, stage.ID, cultivation, *e.clock); err != nil {
		t.Fatal(err)
	}
}

func (e *env) scalar(t *testing.T, sql string, args ...any) (v int64) {
	t.Helper()
	if err := e.pool.QueryRow(context.Background(), sql, args...).Scan(&v); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return
}

func (e *env) equip(t *testing.T, p player, item string) {
	t.Helper()
	if _, err := e.pool.Exec(context.Background(), "INSERT INTO inventory_items VALUES ($1,$2,1)", p.char, item); err != nil {
		t.Fatal(err)
	}
	if v, err := e.arts.Equip(context.Background(), p.user, item); err != nil || v.ItemID != item {
		t.Fatalf("equip: %+v %v", v, err)
	}
}

// guard: protectors accept the invite of p (all online).
func (e *env) guard(t *testing.T, inviter player, ps ...player) {
	t.Helper()
	v, err := e.prot.State(context.Background(), inviter.user)
	if err != nil || v.MyInvite == nil {
		t.Fatalf("no invite: %+v %v", v, err)
	}
	for i, p := range ps {
		e.pres.set(p.char, true)
		if _, err := e.prot.Accept(context.Background(), p.user, v.MyInvite.InviteID, fmt.Sprintf("accept-%s-%d", p.char.String()[:8], i)); err != nil {
			t.Fatalf("accept: %v", err)
		}
	}
}

func sum(parts []breakthrough.OddsPart) (s int64) {
	for _, p := range parts {
		s += p.BP
	}
	return
}

func part(parts []breakthrough.OddsPart, src string) (breakthrough.OddsPart, bool) {
	for _, p := range parts {
		if p.Source == src {
			return p, true
		}
	}
	return breakthrough.OddsPart{}, false
}

const fac = "huyen_khung_mon"

// Decision F at both caps: surplus 5% + pháp bảo 5% + 3 hộ pháp 3% = 13% raw → 12% (bonus cap) → 80% + 12% = 92% →
// 90% (final cap). Every component and cap is disclosed; the attempt records the same snapshot, spends the invite and
// enqueues one điểm duyên credit per counted protector (credited by the worker, idempotent).
func TestTribulationOddsAtEveryCap(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	me := e.player(t, fac)
	last := e.lastOf(t, "hoa_than")
	e.putAt(t, me, last, last.CapMilli)
	e.equip(t, me, "cuu_tieu_hoan")
	if _, err := e.prot.Invite(ctx, me.user, "invite-key-001"); err != nil {
		t.Fatal(err)
	}
	a, b, c, d := e.player(t, fac), e.player(t, fac), e.player(t, fac), e.player(t, fac)
	e.guard(t, me, a, b, c, d) // 4 accept; only 3 can count
	pv, err := e.bt.Preview(ctx, me.user)
	if err != nil {
		t.Fatal(err)
	}
	tr := e.cb.Realms.Tribulation()
	if pv.SuccessBP != tr.MaxSuccessBP || sum(pv.Odds) != pv.SuccessBP {
		t.Fatalf("odds %d (Σ parts %d): %+v", pv.SuccessBP, sum(pv.Odds), pv.Odds)
	}
	items, _ := part(pv.Odds, "items")
	prot, _ := part(pv.Odds, "protector")
	bonusCap, okB := part(pv.Odds, "bonus_cap")
	maxCap, okM := part(pv.Odds, "max_cap")
	if items.BP != 500 || items.Detail != "cuu_tieu_hoan" || prot.BP != tr.ProtectorMaxBP || prot.Detail != "3" || !okB || bonusCap.BP != -100 || !okM || maxCap.BP != -200 {
		t.Fatalf("breakdown %+v", pv.Odds)
	}
	if pv.Caps == nil || pv.Caps.TotalBonusMaxBP != tr.MaxTotalBonusBP || pv.Caps.ProtectorMembers != 3 || pv.Caps.ArtifactID != "cuu_tieu_hoan" {
		t.Fatalf("caps %+v", pv.Caps)
	}
	bound := pv.Tribulation.MaxLossMilli
	r, err := e.bt.Attempt(ctx, me.user, "trib-caps-0001", &bound)
	if err != nil || r.SuccessBP != pv.SuccessBP || r.Success {
		t.Fatalf("attempt %+v %v", r, err)
	}
	var raw []byte
	if err := e.pool.QueryRow(ctx, "SELECT odds FROM breakthrough_attempts WHERE character_id=$1", me.char).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var rec []breakthrough.OddsPart
	_ = json.Unmarshal(raw, &rec)
	if sum(rec) != pv.SuccessBP {
		t.Fatalf("recorded odds %s", raw)
	}
	if n := e.scalar(t, "SELECT count(*) FROM protector_assists WHERE counted_at IS NOT NULL"); n != 3 {
		t.Fatalf("counted %d", n)
	}
	if n := e.scalar(t, "SELECT count(*) FROM protector_invites WHERE used_ref='bt:trib-caps-0001'"); n != 1 {
		t.Fatal("invite not used")
	}
	// Worker step: credit the outbox events twice (replay) → 2 điểm duyên each, once.
	for range 2 {
		err := postgres.InTx(ctx, e.pool, func(tx pgx.Tx) error {
			q := db.New(tx)
			rows, err := tx.Query(ctx, "SELECT payload FROM outbox_events WHERE topic=$1", protector.TopicAssistCounted)
			if err != nil {
				return err
			}
			var evs []protector.AssistCountedEvent
			for rows.Next() {
				var p []byte
				if err := rows.Scan(&p); err != nil {
					return err
				}
				var ev protector.AssistCountedEvent
				_ = json.Unmarshal(p, &ev)
				evs = append(evs, ev)
			}
			rows.Close()
			if len(evs) != 3 {
				return fmt.Errorf("%d events", len(evs))
			}
			for _, ev := range evs {
				if _, err := protector.RewardLocked(ctx, q, e.cb.VIP, ev); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if got := e.scalar(t, "SELECT COALESCE(sum(delta),0) FROM vip_point_ledger WHERE activity_id='protector_assist'"); got != 6 {
		t.Fatalf("protector điểm duyên %d, want 3 × 2", got)
	}
	if got := e.scalar(t, "SELECT count(*) FROM vip_point_ledger WHERE activity_id='protector_assist' AND user_id=$1", d.user); got != 0 {
		t.Fatal("the 4th protector (not counted) must earn nothing")
	}
	// The invite is spent: the next preview has no protectors.
	e.advance(25 * time.Hour)
	pv, _ = e.bt.Preview(ctx, me.user)
	if p, _ := part(pv.Odds, "protector"); p.BP != 0 {
		t.Fatalf("spent invite still counts: %+v", pv.Odds)
	}
}

func (e *env) advance(d time.Duration) { *e.clock = e.clock.Add(d) }

// Below the caps every part adds up without cap lines; an offline or switched protector does not count.
func TestOfflineAndOtherFactionNotCounted(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	me := e.player(t, fac)
	last := e.lastOf(t, "hop_the")
	e.putAt(t, me, last, last.RequiredMilli) // no surplus
	e.equip(t, me, "ty_loi_chau")            // +2%
	if _, err := e.prot.Invite(ctx, me.user, "invite-key-002"); err != nil {
		t.Fatal(err)
	}
	a, b, c := e.player(t, fac), e.player(t, fac), e.player(t, fac)
	e.guard(t, me, a, b, c)
	e.pres.set(b.char, false) // went offline
	if _, err := e.pool.Exec(ctx, "UPDATE character_faction SET faction_id='kim_sa_mon' WHERE character_id=$1", c.char); err != nil {
		t.Fatal(err)
	}
	pv, err := e.bt.Preview(ctx, me.user)
	if err != nil {
		t.Fatal(err)
	}
	base := e.cb.Realms.TribulationBaseBP(last)
	prot, _ := part(pv.Odds, "protector")
	if prot.BP != 100 || pv.SuccessBP != base+200+100 || sum(pv.Odds) != pv.SuccessBP {
		t.Fatalf("odds %+v", pv.Odds)
	}
	if _, ok := part(pv.Odds, "bonus_cap"); ok {
		t.Fatal("no cap should bite")
	}
	st, _ := e.prot.State(ctx, me.user)
	if st.MyInvite.Counted != 1 || len(st.MyInvite.Protectors) != 3 {
		t.Fatalf("state %+v", st.MyInvite)
	}
}

// Each protector has daily_assists accepts per game day; accepting the same invite twice spends one; self and other
// factions are refused; inviting needs a tribulation stage and a faction.
func TestProtectorLimits(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	limit := int(e.cb.Protector.DailyAssists)
	helper := e.player(t, fac)
	e.pres.set(helper.char, true)
	var invites []uuid.UUID
	for i := 0; i <= limit; i++ {
		p := e.player(t, fac)
		e.putAt(t, p, e.lastOf(t, "hoa_than"), e.lastOf(t, "hoa_than").RequiredMilli)
		v, err := e.prot.Invite(ctx, p.user, fmt.Sprintf("invite-key-%03d", i))
		if err != nil || v.MyInvite == nil {
			t.Fatalf("invite: %+v %v", v, err)
		}
		invites = append(invites, v.MyInvite.InviteID)
		if i == 0 { // self-protect refused; a second invite returns the same
			if _, err := e.prot.Accept(ctx, p.user, v.MyInvite.InviteID, "self-accept-01"); !errors.Is(err, protector.ErrSelf) {
				t.Fatalf("self: %v", err)
			}
			if again, _ := e.prot.Invite(ctx, p.user, "invite-key-again"); again.MyInvite.InviteID != v.MyInvite.InviteID {
				t.Fatal("one open invite at a time")
			}
		}
	}
	for i := 0; i < limit; i++ {
		if _, err := e.prot.Accept(ctx, helper.user, invites[i], fmt.Sprintf("help-key-%03d", i)); err != nil {
			t.Fatalf("accept %d: %v", i, err)
		}
	}
	// Same invite again with a new key: no second use, no second row.
	if _, err := e.prot.Accept(ctx, helper.user, invites[0], "help-key-dupe"); err != nil {
		t.Fatalf("re-accept: %v", err)
	}
	if n := e.scalar(t, "SELECT count(*) FROM protector_assists WHERE protector_character_id=$1", helper.char); n != int64(limit) {
		t.Fatalf("assists %d", n)
	}
	if _, err := e.prot.Accept(ctx, helper.user, invites[limit], "help-key-over"); !errors.Is(err, protector.ErrDailyLimit) {
		t.Fatalf("over the limit: %v", err)
	}
	// Next game day the limit resets.
	e.advance(24 * time.Hour)
	if _, err := e.prot.Invite(ctx, e.mustPlayerAtTrib(t).user, "invite-key-next"); err != nil {
		t.Fatal(err)
	}
	outsider := e.player(t, "kim_sa_mon")
	if _, err := e.prot.Accept(ctx, outsider.user, invites[limit], "outsider-key-1"); !errors.Is(err, protector.ErrInviteGone) && !errors.Is(err, protector.ErrOtherFaction) {
		t.Fatalf("outsider: %v", err)
	}
	// Not in front of a tribulation / no faction / not ready (cooldown).
	cool := e.mustPlayerAtTrib(t)
	if _, err := e.pool.Exec(ctx, "UPDATE character_progressions SET breakthrough_cooldown_until=$2 WHERE character_id=$1", cool.char, e.clock.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := e.prot.Invite(ctx, cool.user, "invite-key-cool"); !errors.Is(err, protector.ErrNotReady) {
		t.Fatalf("cooldown: %v", err)
	}
	early := e.player(t, fac)
	if _, err := e.prot.Invite(ctx, early.user, "invite-key-early"); !errors.Is(err, protector.ErrNotEligible) {
		t.Fatalf("early: %v", err)
	}
	lone := e.player(t, "")
	e.putAt(t, lone, e.lastOf(t, "hoa_than"), e.lastOf(t, "hoa_than").RequiredMilli)
	if _, err := e.prot.Invite(ctx, lone.user, "invite-key-lone"); !errors.Is(err, protector.ErrNoFaction) {
		t.Fatalf("lone: %v", err)
	}
}

func (e *env) mustPlayerAtTrib(t *testing.T) player {
	p := e.player(t, fac)
	e.putAt(t, p, e.lastOf(t, "hoa_than"), e.lastOf(t, "hoa_than").RequiredMilli)
	return p
}

// Concurrent accepts of one invite by the same protector: one row, one use.
func TestConcurrentAcceptCountsOnce(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	me := e.mustPlayerAtTrib(t)
	v, err := e.prot.Invite(ctx, me.user, "invite-key-conc")
	if err != nil {
		t.Fatal(err)
	}
	h := e.player(t, fac)
	var wg sync.WaitGroup
	for i := range 4 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _ = e.prot.Accept(ctx, h.user, v.MyInvite.InviteID, fmt.Sprintf("conc-key-%03d", i))
		}(i)
	}
	wg.Wait()
	if n := e.scalar(t, "SELECT count(*) FROM protector_assists WHERE protector_character_id=$1", h.char); n != 1 {
		t.Fatalf("assists %d", n)
	}
	if n := e.scalar(t, "SELECT uses FROM protector_days WHERE character_id=$1", h.char); n != 1 {
		t.Fatalf("uses %d", n)
	}
}

// M2: protectors need min_tier and a character older than min_character_age_days.
func TestProtectorEligibility(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	me := e.mustPlayerAtTrib(t)
	v, err := e.prot.Invite(ctx, me.user, "invite-key-elig")
	if err != nil {
		t.Fatal(err)
	}
	low := e.player(t, fac)
	e.putAt(t, low, e.firstOf(t, "luyen_khi"), 0)
	if _, err := e.prot.Accept(ctx, low.user, v.MyInvite.InviteID, "low-key-0001"); !errors.Is(err, protector.ErrTooLowRealm) {
		t.Fatalf("Luyện Khí protector: %v", err)
	}
	alt := e.player(t, fac)
	if _, err := e.pool.Exec(ctx, "UPDATE characters SET created_at=$2 WHERE id=$1", alt.char, e.clock.Add(-6*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := e.prot.Accept(ctx, alt.user, v.MyInvite.InviteID, "alt-key-0001"); !errors.Is(err, protector.ErrTooNew) {
		t.Fatalf("6-day-old protector: %v", err)
	}
	ok := e.player(t, fac)
	if _, err := e.prot.Accept(ctx, ok.user, v.MyInvite.InviteID, "ok-key-00001"); err != nil {
		t.Fatalf("eligible protector: %v", err)
	}
}

// L2: an accepted but uncounted assist (offline at the attempt) does not spend the protector's daily use.
func TestUncountedAssistNotCharged(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	helper := e.player(t, fac)
	limit := int(e.cb.Protector.DailyAssists)
	me := e.mustPlayerAtTrib(t)
	if _, err := e.prot.Invite(ctx, me.user, "invite-key-unc"); err != nil {
		t.Fatal(err)
	}
	e.guard(t, me, helper)
	e.pres.set(helper.char, false) // offline when the attempt happens
	pv, _ := e.bt.Preview(ctx, me.user)
	bound := pv.Tribulation.MaxLossMilli
	if _, err := e.bt.Attempt(ctx, me.user, "trib-unc-0001", &bound); err != nil {
		t.Fatal(err)
	}
	if n := e.scalar(t, "SELECT count(*) FROM protector_assists WHERE counted_at IS NOT NULL"); n != 0 {
		t.Fatal("offline protector counted")
	}
	st, _ := e.prot.State(ctx, helper.user)
	if int(st.AssistsLeft) != limit {
		t.Fatalf("uncounted assist was charged: %d left of %d", st.AssistsLeft, limit)
	}
}

// L3: a guaranteed attempt does not spend the invite or pay protectors.
func TestGuaranteedAttemptKeepsInvite(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	me := e.mustPlayerAtTrib(t)
	if _, err := e.pool.Exec(ctx, "UPDATE character_progressions SET breakthrough_fail_streak=$2 WHERE character_id=$1", me.char,
		e.cb.Realms.Tribulation().GuaranteeAfter); err != nil {
		t.Fatal(err)
	}
	if _, err := e.prot.Invite(ctx, me.user, "invite-key-gua"); err != nil {
		t.Fatal(err)
	}
	e.guard(t, me, e.player(t, fac))
	r, err := e.bt.Attempt(ctx, me.user, "trib-gua-0001", nil)
	if err != nil || !r.Success || !r.Guaranteed {
		t.Fatalf("guaranteed: %+v %v", r, err)
	}
	if n := e.scalar(t, "SELECT count(*) FROM protector_invites WHERE used_at IS NOT NULL"); n != 0 {
		t.Fatal("guaranteed attempt used the invite")
	}
	if n := e.scalar(t, "SELECT count(*) FROM outbox_events WHERE topic=$1", protector.TopicAssistCounted); n != 0 {
		t.Fatal("guaranteed attempt paid protectors")
	}
}

// M3: odds that dropped after the preview (a protector went offline) refuse the attempt; nothing is written.
func TestAttemptRefusesLowerOdds(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	me := e.mustPlayerAtTrib(t)
	if _, err := e.prot.Invite(ctx, me.user, "invite-key-m3"); err != nil {
		t.Fatal(err)
	}
	h := e.player(t, fac)
	e.guard(t, me, h)
	pv, err := e.bt.Preview(ctx, me.user)
	if err != nil {
		t.Fatal(err)
	}
	e.pres.set(h.char, false)
	bound, seen := pv.Tribulation.MaxLossMilli, pv.SuccessBP
	_, err = e.bt.AttemptGuarded(ctx, me.user, "trib-m3-00001", breakthrough.Guard{AcceptLossMilli: &bound, AcceptSuccessBP: &seen})
	var qc *breakthrough.ErrQuoteChanged
	if !errors.As(err, &qc) || qc.Preview.SuccessBP >= seen {
		t.Fatalf("want quote_changed with lower odds, got %v", err)
	}
	if n := e.scalar(t, "SELECT count(*) FROM breakthrough_attempts WHERE character_id=$1", me.char); n != 0 {
		t.Fatal("quote_changed must write nothing")
	}
	// Confirming the new (lower) odds goes through.
	lower := qc.Preview.SuccessBP
	if _, err := e.bt.AttemptGuarded(ctx, me.user, "trib-m3-00002", breakthrough.Guard{AcceptLossMilli: &bound, AcceptSuccessBP: &lower}); err != nil {
		t.Fatal(err)
	}
}

// M5: a poisoned event (bad user id, unknown user) is dropped without stalling the batch; good events are credited.
func TestRewardBatchSkipsPoisonedEvents(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	good := e.player(t, fac)
	evs := []protector.AssistCountedEvent{
		{AssistID: 1, ProtectorUserID: "not-a-uuid", OccurredAt: *e.clock},
		{AssistID: 2, ProtectorUserID: uuid.NewString(), OccurredAt: *e.clock}, // no such user: FK violation
		{AssistID: 3, ProtectorUserID: good.user.String(), OccurredAt: *e.clock},
		{AssistID: 0, ProtectorUserID: good.user.String(), OccurredAt: *e.clock}, // bad assist id
	}
	err := postgres.InTx(ctx, e.pool, func(tx pgx.Tx) error { _, err := protector.RewardBatch(ctx, tx, e.cb.VIP, evs, nil); return err })
	if err != nil {
		t.Fatalf("poisoned batch failed: %v", err)
	}
	if got := e.scalar(t, "SELECT COALESCE(sum(delta),0) FROM vip_point_ledger WHERE activity_id='protector_assist' AND user_id=$1", good.user); got != 2 {
		t.Fatalf("good event credited %d", got)
	}
}
