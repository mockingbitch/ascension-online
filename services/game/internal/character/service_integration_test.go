//go:build integration

package character_test

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
	"tiendao/game/internal/faction"
	"tiendao/game/internal/platform/testdb"
)

type env struct {
	t     *testing.T
	ctx   context.Context
	pool  *pgxpool.Pool
	chars *character.Service
	now   time.Time
}

func newEnv(t *testing.T) *env {
	t.Helper()
	cb, err := content.LoadAll("../../../../content/catalog")
	if err != nil {
		t.Fatal(err)
	}
	e := &env{t: t, ctx: context.Background(), pool: testdb.New(t), now: time.Date(2027, 1, 10, 3, 0, 0, 0, time.UTC)}
	nowFn := func() time.Time { return e.now }
	cult := &cultivation.Service{Pool: e.pool, Realms: cb.Realms, Contribution: cb.Contribution, VIP: cb.VIP, Tasks: cb.DailyTasks, Now: nowFn}
	fac := &faction.Service{Pool: e.pool, Cat: cb.Factions, Realms: cb.Realms, World: cb.World, Now: nowFn, Settle: cult.SettleLockedRow}
	e.chars = &character.Service{Pool: e.pool, Cultivation: cult, Names: cb.Moderation, Factions: fac}
	return e
}

func (e *env) user() uuid.UUID {
	e.t.Helper()
	u := uuid.New()
	if _, err := e.pool.Exec(e.ctx, "INSERT INTO users (id, email) VALUES ($1, $2)", u, u.String()+"@t"); err != nil {
		e.t.Fatal(err)
	}
	return u
}

func (e *env) count(sql string, args ...any) int {
	e.t.Helper()
	var n int
	if err := e.pool.QueryRow(e.ctx, sql, args...).Scan(&n); err != nil {
		e.t.Fatalf("%s: %v", sql, err)
	}
	return n
}

// §9.6: one intent creates the character on the faction's path and joins it, all in one transaction.
func TestCreateOnboarded(t *testing.T) {
	e := newEnv(t)
	u := e.user()
	o := character.Onboarding{IdempotencyKey: "onb-key-0001", FactionID: "vu_toc"}

	v, err := e.chars.CreateOnboarded(e.ctx, u, "Thạch Nham", o)
	if err != nil {
		t.Fatal(err)
	}
	if v.Replayed || v.FactionID == nil || *v.FactionID != "vu_toc" {
		t.Fatalf("view: replayed=%v faction=%v", v.Replayed, v.FactionID)
	}
	if v.RealmID != "vu_1" {
		t.Fatalf("Vu Tộc starts on the vu path, got realm %s", v.RealmID)
	}
	// Created directly on the path: no conversion event, one join history row (no from/to path), one outbox row.
	if n := e.count("SELECT count(*) FROM progression_events WHERE character_id=$1 AND kind='path_converted'", v.ID); n != 0 {
		t.Fatalf("path_converted events = %d", n)
	}
	if n := e.count("SELECT count(*) FROM faction_history WHERE character_id=$1 AND reason='join' AND to_path IS NULL", v.ID); n != 1 {
		t.Fatalf("history rows = %d", n)
	}
	if n := e.count("SELECT count(*) FROM outbox_events WHERE topic=$1 AND aggregate_id=$2", faction.TopicChanged, v.ID.String()); n != 1 {
		t.Fatalf("faction.changed rows = %d", n)
	}
	// The first join starts the switch cooldown (Phase 2.2).
	if n := e.count("SELECT count(*) FROM character_faction WHERE character_id=$1 AND last_switch_at IS NOT NULL", v.ID); n != 1 {
		t.Fatal("first join must start the cooldown")
	}

	// Replay: same key, name, faction → same state, nothing new written.
	r, err := e.chars.CreateOnboarded(e.ctx, u, "Thạch Nham", o)
	if err != nil || !r.Replayed || r.ID != v.ID {
		t.Fatalf("replay: %+v %v", r.Replayed, err)
	}
	if n := e.count("SELECT count(*) FROM faction_history WHERE character_id=$1", v.ID); n != 1 {
		t.Fatalf("history rows after replay = %d", n)
	}
	// Same key for another faction → reused; a fresh key or another name → the account already has a character.
	if _, err := e.chars.CreateOnboarded(e.ctx, u, "Thạch Nham", character.Onboarding{IdempotencyKey: o.IdempotencyKey, FactionID: "kim_sa_mon"}); !errors.Is(err, faction.ErrIdempotencyReused) {
		t.Fatalf("reused key: %v", err)
	}
	if _, err := e.chars.CreateOnboarded(e.ctx, u, "Thạch Nham", character.Onboarding{IdempotencyKey: "onb-key-0002", FactionID: "vu_toc"}); !errors.Is(err, character.ErrAlreadyExists) {
		t.Fatalf("new key: %v", err)
	}
	if _, err := e.chars.CreateOnboarded(e.ctx, u, "Tên Khác", o); !errors.Is(err, character.ErrAlreadyExists) {
		t.Fatalf("other name: %v", err)
	}
	// GET view carries the faction (the web guard routes on it).
	if g, err := e.chars.Get(e.ctx, u); err != nil || g.FactionID == nil || *g.FactionID != "vu_toc" {
		t.Fatalf("get: %v %v", g.FactionID, err)
	}
}

func TestCreateOnboardedRefusalsWriteNothing(t *testing.T) {
	e := newEnv(t)
	cases := []struct {
		o    character.Onboarding
		want error
	}{
		{character.Onboarding{IdempotencyKey: "onb-k-closed", FactionID: "nhan_toc"}, faction.ErrFactionClosed},      // opens at tao_nhan
		{character.Onboarding{IdempotencyKey: "onb-k-later1", FactionID: "thu_nhat_vien"}, faction.ErrFactionClosed}, // opens with thanh_nhan
		{character.Onboarding{IdempotencyKey: "onb-k-unknown", FactionID: "khong_co"}, faction.ErrUnknownFaction},
		{character.Onboarding{IdempotencyKey: "onb-k-path01", FactionID: "vu_toc", Path: "tien"}, faction.ErrPathNotAllowed},
	}
	for _, c := range cases {
		u := e.user()
		if _, err := e.chars.CreateOnboarded(e.ctx, u, "Lục Vân Phi", c.o); !errors.Is(err, c.want) {
			t.Fatalf("%s: got %v, want %v", c.o.FactionID, err, c.want)
		}
		if n := e.count("SELECT count(*) FROM characters WHERE user_id=$1", u); n != 0 {
			t.Fatalf("%s: a refused onboarding left a character behind", c.o.FactionID)
		}
	}
	// The name is still free afterwards, and an explicit allowed path is accepted.
	u := e.user()
	v, err := e.chars.CreateOnboarded(e.ctx, u, "Lục Vân Phi", character.Onboarding{IdempotencyKey: "onb-k-ok0001", FactionID: "huyen_khung_mon", Path: "tien"})
	if err != nil || v.RealmID != "luyen_khi" || v.FactionID == nil || *v.FactionID != "huyen_khung_mon" {
		t.Fatalf("ok: %v %v", v.RealmID, err)
	}
	// Without a faction service the onboarding intent is refused (legacy Create still works).
	plain := &character.Service{Pool: e.pool, Cultivation: e.chars.Cultivation}
	if _, err := plain.CreateOnboarded(e.ctx, e.user(), "Ai Đó", character.Onboarding{IdempotencyKey: "onb-k-nofac", FactionID: "vu_toc"}); !errors.Is(err, character.ErrNoFactions) {
		t.Fatalf("no factions: %v", err)
	}
}
