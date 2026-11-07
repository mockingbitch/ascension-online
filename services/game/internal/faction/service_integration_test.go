//go:build integration

package faction_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"tiendao/game/internal/content"
	"tiendao/game/internal/cultivation"
	"tiendao/game/internal/db"
	"tiendao/game/internal/faction"
	"tiendao/game/internal/gameday"
	"tiendao/game/internal/platform/redisx"
	"tiendao/game/internal/platform/testdb"
	"tiendao/game/internal/world"
)

type fixture struct {
	t    *testing.T
	ctx  context.Context
	pool *pgxpool.Pool
	cb   *content.Bundle
	now  time.Time
	svc  *faction.Service
	n    int
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	cb, err := content.LoadAll("../../../../content/catalog")
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{t: t, ctx: context.Background(), pool: testdb.New(t), cb: cb, now: time.Date(2027, 1, 10, 3, 0, 0, 0, time.UTC)}
	f.svc = &faction.Service{Pool: f.pool, Cat: cb.Factions, Realms: cb.Realms, World: cb.World, Now: func() time.Time { return f.now }}
	return f
}

func (f *fixture) exec(sql string, args ...any) {
	f.t.Helper()
	if _, err := f.pool.Exec(f.ctx, sql, args...); err != nil {
		f.t.Fatalf("%s: %v", sql, err)
	}
}

func (f *fixture) scalar(sql string, dst any, args ...any) {
	f.t.Helper()
	if err := f.pool.QueryRow(f.ctx, sql, args...).Scan(dst); err != nil {
		f.t.Fatalf("%s: %v", sql, err)
	}
}

func (f *fixture) count(sql string, args ...any) int {
	f.t.Helper()
	var n int
	f.scalar(sql, &n, args...)
	return n
}

// player creates a user + character at Luyện Khí tầng 1 (tiên) and returns (user, character).
func (f *fixture) player() (uuid.UUID, uuid.UUID) {
	f.t.Helper()
	f.n++
	u, c := uuid.New(), uuid.New()
	f.exec("INSERT INTO users (id, email) VALUES ($1, $2)", u, u.String()+"@t")
	f.exec("INSERT INTO vip_accounts (user_id) VALUES ($1)", u)
	f.exec("INSERT INTO characters (id, user_id, name) VALUES ($1, $2, $3)", c, u, fmt.Sprintf("Đạo Hữu %d", f.n))
	f.exec("INSERT INTO character_progressions (character_id, realm_id, stage_id, cultivation_milli) VALUES ($1, 'luyen_khi', 'luyen_khi.tang_1', $2)", c, int64(f.n)*1000)
	return u, c
}

// member puts a character straight into a faction (fixture shortcut for factions not open in the current era).
func (f *fixture) member(factionID string) (uuid.UUID, uuid.UUID) {
	u, c := f.player()
	f.exec("INSERT INTO character_faction (character_id, faction_id, joined_at) VALUES ($1, $2, $3)", c, factionID, f.now)
	return u, c
}

func (f *fixture) factionOf(c uuid.UUID) string {
	f.t.Helper()
	var id string
	f.scalar("SELECT faction_id FROM character_faction WHERE character_id=$1", &id, c)
	return id
}

func (f *fixture) setEra(era string) {
	f.exec("UPDATE world_state SET era_id=$1, era_started_at=$2", era, f.now)
}

func key() string { return "k" + uuid.NewString()[:12] }

func TestJoinSwitchRules(t *testing.T) {
	f := newFixture(t)
	u, c := f.player()

	k := key()
	res, err := f.svc.Join(f.ctx, u, k, "kim_sa_mon", "")
	if err != nil || res.Replayed || res.Membership.FactionID == nil || *res.Membership.FactionID != "kim_sa_mon" {
		t.Fatalf("join: %+v %v", res, err)
	}
	// first_join_free: false — the first switch waits a full cooldown too (review H2: fresh alts can't hop).
	if res.Membership.CanSwitchAt == nil || !res.Membership.CanSwitchAt.Equal(f.now.Add(7*24*time.Hour)) {
		t.Fatalf("first join must start the cooldown: %v", res.Membership.CanSwitchAt)
	}
	// Idempotent replay: same key → same state, one history row, one outbox row.
	if res, err = f.svc.Join(f.ctx, u, k, "kim_sa_mon", ""); err != nil || !res.Replayed {
		t.Fatalf("replay: %+v %v", res, err)
	}
	if n := f.count("SELECT count(*) FROM faction_history WHERE character_id=$1", c); n != 1 {
		t.Fatalf("history rows after replay = %d", n)
	}
	if n := f.count("SELECT count(*) FROM outbox_events WHERE topic=$1", faction.TopicChanged); n != 1 {
		t.Fatalf("faction.changed rows = %d", n)
	}
	// The credit path stamps a new day's contribution row with the member's faction (merit attribution).
	if err := db.New(f.pool).EnsureDailyContribution(f.ctx, db.EnsureDailyContributionParams{CharacterID: c,
		GameDay: pgtype.Date{Time: time.Date(2027, 1, 11, 0, 0, 0, 0, time.UTC), Valid: true}}); err != nil {
		t.Fatal(err)
	}
	if n := f.count("SELECT count(*) FROM daily_contribution WHERE character_id=$1 AND faction_id='kim_sa_mon'", c); n != 1 {
		t.Fatal("daily_contribution not stamped with the faction")
	}
	if _, err := f.svc.Join(f.ctx, u, key(), "vu_toc", ""); !errors.Is(err, faction.ErrAlreadyMember) {
		t.Fatalf("second join: %v", err)
	}
	// L1: a key is bound to its operation and target.
	if _, err := f.svc.Join(f.ctx, u, k, "vu_toc", ""); !errors.Is(err, faction.ErrIdempotencyReused) {
		t.Fatalf("same key, other target: %v", err)
	}
	if _, err := f.svc.Switch(f.ctx, u, k, "kim_sa_mon", ""); !errors.Is(err, faction.ErrIdempotencyReused) {
		t.Fatalf("same key, other operation: %v", err)
	}
	if _, err := f.svc.Switch(f.ctx, u, key(), "vu_toc", ""); !errors.Is(err, faction.ErrCooldown) {
		t.Fatalf("switch right after the first join: %v", err)
	}
	f.now = f.now.Add(7*24*time.Hour + time.Second)
	// Wrong path / closed factions.
	if _, err := f.svc.Switch(f.ctx, u, key(), "thuong_lan_dinh", "vu"); !errors.Is(err, faction.ErrPathNotAllowed) {
		t.Fatalf("wrong path: %v", err)
	}
	if _, err := f.svc.Switch(f.ctx, u, key(), "nhan_toc", ""); !errors.Is(err, faction.ErrFactionClosed) {
		t.Fatalf("closed nhan_toc: %v", err)
	}
	if _, err := f.svc.Switch(f.ctx, u, key(), "thu_nhat_vien", ""); !errors.Is(err, faction.ErrFactionClosed) {
		t.Fatalf("branch before its era: %v", err)
	}

	// Joining Vu Tộc converts the path to vu at the same tier/stage (F3).
	res, err = f.svc.Switch(f.ctx, u, key(), "vu_toc", "")
	if err != nil || res.Membership.Path != "vu" {
		t.Fatalf("switch to vu: %+v %v", res, err)
	}
	var stage string
	var cult int64
	f.scalar("SELECT stage_id FROM character_progressions WHERE character_id=$1", &stage, c)
	f.scalar("SELECT cultivation_milli FROM character_progressions WHERE character_id=$1", &cult, c)
	if want := f.cb.Realms.First("vu").ID; stage != want || cult != 1000 {
		t.Fatalf("converted stage %s cult %d, want %s 1000", stage, cult, want)
	}
	if n := f.count("SELECT count(*) FROM faction_history WHERE character_id=$1 AND from_path='tien' AND to_path='vu'", c); n != 1 {
		t.Fatal("history must record the path conversion")
	}
	if n := f.count("SELECT count(*) FROM progression_events WHERE character_id=$1 AND kind='path_converted'", c); n != 1 {
		t.Fatal("missing path_converted event")
	}

	// Cooldown (F4): 7 days from the last player switch.
	if res.Membership.CanSwitchAt == nil || !res.Membership.CanSwitchAt.Equal(f.now.Add(7*24*time.Hour)) {
		t.Fatalf("canSwitchAt = %v", res.Membership.CanSwitchAt)
	}
	if _, err := f.svc.Switch(f.ctx, u, key(), "thuong_lan_dinh", ""); !errors.Is(err, faction.ErrCooldown) {
		t.Fatalf("cooldown: %v", err)
	}
	f.now = f.now.Add(7*24*time.Hour + time.Second)
	if res, err = f.svc.Switch(f.ctx, u, key(), "thuong_lan_dinh", ""); err != nil || res.Membership.Path != "tien" {
		t.Fatalf("switch after cooldown: %+v %v", res, err)
	}
	if got := f.factionOf(c); got != "thuong_lan_dinh" {
		t.Fatalf("faction = %s", got)
	}

	// Era moves on (set directly, no world tick → no split scheduled here): thuong_lan_dinh is closed from luan_hoi
	// (nobody joins; its split is TestCloseSplitThuongLan); huyen_khung_mon closes at thanh_nhan.
	f.setEra("thanh_nhan")
	u2, _ := f.player()
	if _, err := f.svc.Join(f.ctx, u2, key(), "huyen_khung_mon", ""); !errors.Is(err, faction.ErrFactionClosed) {
		t.Fatalf("closed huyen_khung_mon: %v", err)
	}
	if _, err := f.svc.Join(f.ctx, u2, key(), "lap_cuc_vien", ""); err != nil {
		t.Fatalf("branch open in thanh_nhan: %v", err)
	}
	if m, err := f.svc.Me(f.ctx, u); err != nil || m.FactionID == nil || *m.FactionID != "thuong_lan_dinh" || len(m.History) != 3 {
		t.Fatalf("me: %+v %v", m, err)
	}
}

func TestLockBlocksSwitch(t *testing.T) {
	f := newFixture(t)
	u, c := f.player()
	admin := uuid.New()
	f.exec("INSERT INTO users (id, email, role) VALUES ($1, 'admin@t', 'admin')", admin)
	if _, err := f.svc.SetLock(f.ctx, admin, c, "admin", 0, "test"); !errors.Is(err, faction.ErrNotMember) {
		t.Fatalf("lock non-member: %v", err)
	}
	if _, err := f.svc.Join(f.ctx, u, key(), "kim_sa_mon", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.SetLock(f.ctx, admin, c, "bogus", 0, "test"); !errors.Is(err, faction.ErrInvalidLock) {
		t.Fatalf("bad reason: %v", err)
	}
	v, err := f.svc.SetLock(f.ctx, admin, c, "fengshen", 0, "Phong Thần")
	if err != nil || v.LockReason != "fengshen" || v.LockedUntil != nil {
		t.Fatalf("lock: %+v %v", v, err)
	}
	if _, err := f.svc.Switch(f.ctx, u, key(), "vu_toc", ""); !errors.Is(err, faction.ErrLocked) {
		t.Fatalf("locked switch: %v", err)
	}
	if m, _ := f.svc.Me(f.ctx, u); !m.Locked || m.LockReason != "fengshen" {
		t.Fatalf("me locked: %+v", m)
	}
	if _, err := f.svc.SetLock(f.ctx, admin, c, "", 0, "hết kiếp"); err != nil {
		t.Fatal(err)
	}
	f.now = f.now.Add(8 * 24 * time.Hour) // past the first-join cooldown
	if _, err := f.svc.Switch(f.ctx, u, key(), "vu_toc", ""); err != nil {
		t.Fatalf("unlocked switch: %v", err)
	}
	if n := f.count("SELECT count(*) FROM admin_audit_logs WHERE action IN ('faction.lock','faction.unlock') AND target_id=$1", c.String()); n != 2 {
		t.Fatalf("audit rows = %d", n)
	}
}

func TestConcurrentJoins(t *testing.T) {
	f := newFixture(t)
	u, c := f.player()
	targets := []string{"huyen_khung_mon", "kim_sa_mon", "thuong_lan_dinh", "tinh_ha_yeu_dinh", "vu_toc", "kim_sa_mon", "vu_toc", "huyen_khung_mon"}
	var wg sync.WaitGroup
	errs := make([]error, len(targets))
	for i, tg := range targets {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = f.svc.Join(f.ctx, u, key(), tg, "")
		}()
	}
	wg.Wait()
	ok := 0
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case !errors.Is(err, faction.ErrAlreadyMember):
			t.Fatalf("unexpected: %v", err)
		}
	}
	if ok != 1 || f.count("SELECT count(*) FROM faction_history WHERE character_id=$1", c) != 1 {
		t.Fatalf("%d joins succeeded", ok)
	}
	// Same key from several tabs at once: one change, the rest replay.
	u2, c2 := f.player()
	k := key()
	replayed := 0
	var mu sync.Mutex
	for range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := f.svc.Join(f.ctx, u2, k, "vu_toc", "")
			if err != nil {
				t.Errorf("same-key join: %v", err)
				return
			}
			if res.Replayed {
				mu.Lock()
				replayed++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if replayed != 5 || f.count("SELECT count(*) FROM faction_history WHERE character_id=$1", c2) != 1 {
		t.Fatalf("replayed %d", replayed)
	}
}

// reachEffect records milestone thanh_nhan as reached at f.now with its catalogue effects pending, like the world
// tick does (world.Service.reach), and sets the era.
func (f *fixture) reachThanhNhan() map[world.EffectKind]int64 {
	f.t.Helper()
	m, ok := f.cb.World.Milestone("thanh_nhan")
	if !ok {
		f.t.Fatal("no thanh_nhan milestone")
	}
	f.exec(`INSERT INTO world_milestones_reached (milestone_id, seq, reached_at, reason, server_day, progress_milli, threshold_milli, target_day)
	        VALUES ($1, $2, $3, 'admin', 140, 0, 0, 140)`, m.ID, m.Seq, f.now)
	ids := map[world.EffectKind]int64{}
	for i, e := range m.Effects {
		if e.Kind != world.EffectSplitFaction && e.Kind != world.EffectMigrateFaction {
			continue
		}
		snap, _ := json.Marshal(e)
		due := f.now.Add(time.Duration(e.NoticeDays) * 24 * time.Hour)
		var deadline *time.Time
		if e.Kind == world.EffectSplitFaction {
			d := due.Add(time.Duration(e.ChooseDays) * 24 * time.Hour)
			deadline = &d
		}
		var id int64
		f.scalar(`INSERT INTO world_effects (milestone_id, effect_index, kind, due_at, deadline_at, snapshot)
		          VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`, &id, m.ID, i, string(e.Kind), due, deadline, snap)
		ids[e.Kind] = id
	}
	f.setEra("thanh_nhan")
	return ids
}

func (f *fixture) runEffects(fx *faction.Effects) faction.EffectsResult {
	f.t.Helper()
	res, err := fx.Run(f.ctx)
	if err != nil {
		f.t.Fatal(err)
	}
	return res
}

func (f *fixture) announced(key string) int {
	return f.count("SELECT count(*) FROM outbox_events WHERE topic=$1 AND payload->>'key'=$2", faction.TopicAnnouncement, key)
}

func TestWorldEffectsMigrateAndSplit(t *testing.T) {
	f := newFixture(t)
	fx := &faction.Effects{Pool: f.pool, Cat: f.cb.Factions, World: f.cb.World, Log: slog.New(slog.DiscardHandler), Now: func() time.Time { return f.now }, Batch: 3}

	var kimSa, huyen []uuid.UUID
	for range 7 {
		_, c := f.member("kim_sa_mon")
		kimSa = append(kimSa, c)
	}
	users := map[uuid.UUID]uuid.UUID{}
	for range 7 {
		u, c := f.member("huyen_khung_mon")
		huyen = append(huyen, c)
		users[c] = u
	}
	_, other := f.member("vu_toc")
	// Members who joined branches directly once they opened: thu_nhat_vien 2, lap_cuc_vien 1, van_luu_vien 0.
	f.member("thu_nhat_vien")
	f.member("thu_nhat_vien")
	f.member("lap_cuc_vien")

	if res := f.runEffects(fx); res.Steps != 0 {
		t.Fatalf("nothing scheduled yet: %+v", res)
	}
	ids := f.reachThanhNhan()
	reached := f.now

	// Tick at reach: both notices go out once, nobody moves.
	res := f.runEffects(fx)
	if res.Notices != 2 || res.Moved != 0 || f.announced(faction.KeySplitNotice) != 1 || f.announced(faction.KeyMigrateNotice) != 1 {
		t.Fatalf("notices: %+v", res)
	}
	if res := f.runEffects(fx); res.Steps != 0 {
		t.Fatalf("re-run during notice did something: %+v", res)
	}
	m, err := f.svc.Me(f.ctx, users[huyen[0]])
	if err != nil || m.Upcoming == nil || m.Upcoming.Kind != world.EffectSplitFaction || len(m.MustChoose) != 0 {
		t.Fatalf("upcoming split: %+v %v", m, err)
	}

	// Due (3 days): migrate moves everyone in batches of 3; split marks everyone pending and opens the choice.
	f.now = reached.Add(3*24*time.Hour + time.Minute)
	res = f.runEffects(fx)
	if res.Moved != 7 || res.Pending != 7 || res.Applied != 2 {
		t.Fatalf("due: %+v", res)
	}
	for _, c := range kimSa {
		if got := f.factionOf(c); got != "kim_sa_giao" {
			t.Fatalf("migrated member in %s", got)
		}
	}
	if f.count("SELECT count(*) FROM faction_history WHERE reason='auto_migrate' AND world_effect_id=$1", ids[world.EffectMigrateFaction]) != 7 ||
		f.announced(faction.KeyMigrateDone) != 1 || f.announced(faction.KeySplitDone) != 1 {
		t.Fatal("migrate history / announcements")
	}
	if got := f.factionOf(other); got != "vu_toc" {
		t.Fatalf("bystander moved to %s", got)
	}
	if n := f.count("SELECT count(*) FROM outbox_events WHERE topic=$1", faction.TopicBulkChanged); n == 0 {
		t.Fatal("bulk moves must enqueue faction.bulk_changed")
	}

	// Choice window: members pick a branch (no cooldown, replay-safe); invalid branches are refused.
	m, err = f.svc.Me(f.ctx, users[huyen[0]])
	if err != nil || len(m.MustChoose) != 3 || m.ChooseDeadline == nil {
		t.Fatalf("must choose: %+v %v", m, err)
	}
	if _, err := f.svc.ChooseSplit(f.ctx, users[huyen[0]], key(), "vu_toc"); !errors.Is(err, faction.ErrNotABranch) {
		t.Fatalf("not a branch: %v", err)
	}
	k := key()
	for range 2 {
		if r, err := f.svc.ChooseSplit(f.ctx, users[huyen[0]], k, "lap_cuc_vien"); err != nil || *r.Membership.FactionID != "lap_cuc_vien" {
			t.Fatalf("choose: %+v %v", r, err)
		}
	}
	if _, err := f.svc.ChooseSplit(f.ctx, users[huyen[0]], key(), "thu_nhat_vien"); !errors.Is(err, faction.ErrNoPendingSplit) {
		t.Fatalf("second choice: %v", err)
	}
	if _, err := f.svc.ChooseSplit(f.ctx, users[huyen[1]], key(), "lap_cuc_vien"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ChooseSplit(f.ctx, users[huyen[2]], key(), "thu_nhat_vien"); err != nil {
		t.Fatal(err)
	}
	// L1: a split-choice key is bound to its branch.
	if _, err := f.svc.ChooseSplit(f.ctx, users[huyen[0]], k, "van_luu_vien"); !errors.Is(err, faction.ErrIdempotencyReused) {
		t.Fatalf("reused key: %v", err)
	}
	// M3: a locked member can't choose; the deadline default places them.
	f.exec("UPDATE character_faction SET lock_reason='fengshen', locked_at=$2 WHERE character_id=$1", huyen[3], f.now)
	if _, err := f.svc.ChooseSplit(f.ctx, users[huyen[3]], key(), "thu_nhat_vien"); !errors.Is(err, faction.ErrLocked) {
		t.Fatalf("locked choice: %v", err)
	}
	if res := f.runEffects(fx); res.Steps != 0 {
		t.Fatalf("window open, nothing to do: %+v", res)
	}

	// Deadline (3 + 7 days). Placed BY THIS SPLIT: lap 2, thu 1, van 0 — the 3 direct branch members (alts/switchers)
	// don't count (H2). The 4 undecided fill the smallest branch one by one in character_id order (ties → YAML order):
	// van, thu, van, thu → final placements thu 3, lap 2, van 2.
	f.now = reached.Add(10*24*time.Hour + time.Minute)
	if _, err := f.svc.ChooseSplit(f.ctx, users[huyen[4]], key(), "thu_nhat_vien"); !errors.Is(err, faction.ErrChoiceClosed) {
		t.Fatalf("after deadline: %v", err)
	}
	undecided := slices.Clone(huyen[3:])
	slices.SortFunc(undecided, func(a, b uuid.UUID) int { return slices.Compare(a[:], b[:]) })
	res = f.runEffects(fx)
	if res.Moved != 4 || res.Finalized != 1 {
		t.Fatalf("finalize: %+v", res)
	}
	for i, want := range []string{"van_luu_vien", "thu_nhat_vien", "van_luu_vien", "thu_nhat_vien"} {
		if got := f.factionOf(undecided[i]); got != want {
			t.Fatalf("undecided[%d] → %s, want %s", i, got, want)
		}
	}
	for b, want := range map[string]int{"thu_nhat_vien": 3, "lap_cuc_vien": 2, "van_luu_vien": 2} {
		if n := f.count("SELECT count(*) FROM faction_history WHERE world_effect_id=$1 AND to_faction_id=$2", ids[world.EffectSplitFaction], b); n != want {
			t.Fatalf("placed in %s = %d, want %d", b, n, want)
		}
	}
	var targets []string
	f.scalar("SELECT ARRAY(SELECT jsonb_array_elements_text(payload->'targets')) FROM outbox_events WHERE topic=$1 AND payload->>'key'=$2",
		&targets, faction.TopicAnnouncement, faction.KeySplitFinal)
	if !slices.Equal(targets, []string{"thu_nhat_vien", "lap_cuc_vien", "van_luu_vien"}) {
		t.Fatalf("split_final targets %v", targets)
	}
	if n := f.count("SELECT count(*) FROM faction_history WHERE reason='split_default'"); n != 4 {
		t.Fatalf("split_default rows %d", n)
	}
	if n := f.count("SELECT count(*) FROM character_faction WHERE faction_id IN ('huyen_khung_mon','kim_sa_mon') OR pending_split_effect_id IS NOT NULL"); n != 0 {
		t.Fatalf("%d members left in closed factions", n)
	}
	// Idempotent re-run: nothing left, no extra rows.
	if res := f.runEffects(fx); res.Steps != 0 {
		t.Fatalf("re-run: %+v", res)
	}
	if f.announced(faction.KeySplitFinal) != 1 || f.count("SELECT count(*) FROM faction_history") != 7+4+3 {
		t.Fatal("re-run wrote rows")
	}
	if m, _ := f.svc.Me(f.ctx, users[huyen[3]]); m.Upcoming != nil || len(m.MustChoose) != 0 {
		t.Fatalf("after finalize: %+v", m)
	}
}

func TestRankingRebuild(t *testing.T) {
	f := newFixture(t)
	url := os.Getenv("TEST_REDIS_URL")
	if url == "" {
		url = "redis://localhost:2379/15"
	}
	rdb, err := redisx.Connect(f.ctx, url)
	if err != nil {
		t.Fatalf("%v (run `make db-up`)", err)
	}
	prefix := "test:faction:" + uuid.NewString()
	t.Cleanup(func() {
		keys, _ := rdb.Keys(context.Background(), prefix+"*").Result()
		if len(keys) > 0 {
			rdb.Del(context.Background(), keys...)
		}
		rdb.Close()
	})
	rk := &faction.Ranking{Pool: f.pool, Redis: rdb, Realms: f.cb.Realms, Cat: f.cb.Factions, KeyPrefix: prefix,
		CapMilli: f.cb.World.Settings.WorldCreditCapMilli}
	f.svc.Ranking = rk
	f.exec("UPDATE world_state SET started_at=$1", f.now.Add(-24*time.Hour))
	today := f.now.In(time.FixedZone("ICT", 7*3600)).Format(time.DateOnly)

	uA, a := f.player()
	uB, b := f.player()
	uC, cc := f.player()
	// A credited before joining: joining stamps today's row for the new faction.
	f.exec("INSERT INTO daily_contribution (character_id, game_day, idle_milli) VALUES ($1, $2, 50000)", a, today)
	for _, j := range []struct {
		u uuid.UUID
		f string
	}{{uA, "kim_sa_mon"}, {uB, "kim_sa_mon"}, {uC, "vu_toc"}} {
		if _, err := f.svc.Join(f.ctx, j.u, key(), j.f, ""); err != nil {
			t.Fatal(err)
		}
	}
	// H3: the ×2 catch-up bonus never counts and a day counts at most the world cap (100 points), like world progress.
	f.exec("INSERT INTO daily_contribution (character_id, game_day, idle_milli, catch_up_bonus_milli, faction_id) VALUES ($1, $2, 70000, 30000, 'vu_toc')", cc, today)
	f.exec("INSERT INTO daily_contribution (character_id, game_day, idle_milli, activity_milli, faction_id) VALUES ($1, $2, 75000, 45000, 'kim_sa_mon')", b, today)
	// Before the era start: never counted.
	f.exec("INSERT INTO daily_contribution (character_id, game_day, idle_milli, faction_id) VALUES ($1, '2026-12-01', 99000, 'kim_sa_mon')", b)
	f.exec("UPDATE character_progressions SET cultivation_milli=900000 WHERE character_id=$1", b)

	if n, err := rk.Rebuild(f.ctx); err != nil || n != 3 {
		t.Fatalf("rebuild %d %v", n, err)
	}
	v, err := f.svc.List(f.ctx)
	if err != nil || !v.RankingAvailable {
		t.Fatalf("list %v %+v", err, v)
	}
	byID := map[string]faction.FactionView{}
	for _, fv := range v.Factions {
		byID[fv.ID] = fv
	}
	if k := byID["kim_sa_mon"]; k.Members != 2 || k.MeritMilli != 50000+100000 || k.MeritRank != 1 {
		t.Fatalf("kim_sa_mon %+v", k)
	}
	if vu := byID["vu_toc"]; vu.Members != 1 || vu.MeritMilli != 40000 || vu.MeritRank != 2 {
		t.Fatalf("vu_toc %+v", vu)
	}
	if byID["nhan_toc"].MeritRank != 0 || byID["nhan_toc"].Open {
		t.Fatal("closed empty faction must be unranked")
	}
	page, err := f.svc.Members(f.ctx, "kim_sa_mon", "")
	if err != nil || page.Total != 2 || page.Entries[0].CharacterID != b || page.Entries[1].CharacterID != a || page.NextCursor != nil {
		t.Fatalf("members %+v %v", page, err)
	}
	// A switch moves the member between ZSETs on refresh; counts follow via a requested rebuild (worker, M2).
	f.now = f.now.Add(8 * 24 * time.Hour)
	if _, err := f.svc.Switch(f.ctx, uB, key(), "thuong_lan_dinh", ""); err != nil {
		t.Fatal(err)
	}
	if err := rk.RefreshMembers(f.ctx, []uuid.UUID{b}); err != nil {
		t.Fatal(err)
	}
	if err := rk.RequestRebuild(f.ctx, false); err != nil {
		t.Fatal(err)
	}
	if did, err := rk.RunRequested(f.ctx); err != nil || did != "counts" {
		t.Fatalf("requested rebuild %q %v", did, err)
	}
	if did, _ := rk.RunRequested(f.ctx); did != "" {
		t.Fatalf("marker not cleared: %q", did)
	}
	if v, _ := f.svc.List(f.ctx); v.Factions[2].ID != "thuong_lan_dinh" || v.Factions[2].Members != 1 {
		t.Fatalf("counts after request: %+v", v.Factions[2])
	}
	if p, _ := f.svc.Members(f.ctx, "kim_sa_mon", ""); p.Total != 1 {
		t.Fatalf("old faction still lists the member: %+v", p)
	}
	if p, _ := f.svc.Members(f.ctx, "thuong_lan_dinh", ""); p.Total != 1 || p.Entries[0].CharacterID != b {
		t.Fatalf("new faction: %+v", p)
	}
	// Losing Redis: a full rebuild restores everything from Postgres.
	keys, _ := rdb.Keys(f.ctx, prefix+"*").Result()
	rdb.Del(f.ctx, keys...)
	if _, err := rk.Rebuild(f.ctx); err != nil {
		t.Fatal(err)
	}
	if p, _ := f.svc.Members(f.ctx, "thuong_lan_dinh", ""); p.Total != 1 {
		t.Fatalf("after rebuild: %+v", p)
	}
	if _, err := f.svc.Members(f.ctx, "khong_co", ""); !errors.Is(err, faction.ErrUnknownFaction) {
		t.Fatalf("unknown: %v", err)
	}
}

// H1: the worker was down (or the tick kept failing) for 10 days after the milestone. The notice still goes out with
// its full period and the choice window is still choose_days long, counted from when it actually opens.
func TestWorldEffectsLateRecovery(t *testing.T) {
	f := newFixture(t)
	fx := &faction.Effects{Pool: f.pool, Cat: f.cb.Factions, World: f.cb.World, Log: slog.New(slog.DiscardHandler), Now: func() time.Time { return f.now }}
	u, c := f.member("huyen_khung_mon")
	_, k := f.member("kim_sa_mon")
	ids := f.reachThanhNhan()
	reached := f.now

	f.now = reached.Add(10 * 24 * time.Hour)
	if res := f.runEffects(fx); res.Notices != 2 || res.Moved != 0 || res.Pending != 0 {
		t.Fatalf("late notices: %+v", res)
	}
	var due time.Time
	f.scalar("SELECT due_at FROM world_effects WHERE id=$1", &due, ids[world.EffectSplitFaction])
	if !due.Equal(f.now.Add(3 * 24 * time.Hour)) {
		t.Fatalf("split due %v, want now + 3 days", due)
	}
	if res := f.runEffects(fx); res.Steps != 0 {
		t.Fatalf("notice period: %+v", res)
	}
	if got := f.factionOf(k); got != "kim_sa_mon" {
		t.Fatal("migrated before the rescheduled notice ended")
	}
	// The move happens 3 days after the (late) notice; it opens late too (2 more days) → window from then.
	f.now = due.Add(2 * 24 * time.Hour)
	if res := f.runEffects(fx); res.Applied != 2 || res.Moved != 1 || res.Pending != 1 {
		t.Fatalf("apply: %+v", res)
	}
	m, err := f.svc.Me(f.ctx, u)
	if err != nil || m.ChooseDeadline == nil || !m.ChooseDeadline.Equal(f.now.Add(7*24*time.Hour)) {
		t.Fatalf("deadline %v, want now + 7 days (%+v %v)", m.ChooseDeadline, m, err)
	}
	f.now = f.now.Add(7*24*time.Hour - time.Minute)
	if _, err := f.svc.ChooseSplit(f.ctx, u, key(), "lap_cuc_vien"); err != nil {
		t.Fatalf("choice inside the full window: %v", err)
	}
	if got := f.factionOf(c); got != "lap_cuc_vien" {
		t.Fatal(got)
	}
}

// M1: a faction change settles first, so offline accrual is credited (and stamped) to the faction it was earned in.
func TestSettleBeforeSwitch(t *testing.T) {
	f := newFixture(t)
	cult := &cultivation.Service{Pool: f.pool, Realms: f.cb.Realms, Contribution: f.cb.Contribution, VIP: f.cb.VIP, Tasks: f.cb.DailyTasks,
		Now: func() time.Time { return f.now }}
	f.svc.Settle = cult.SettleLockedRow
	u, c := f.player()
	f.exec("UPDATE character_progressions SET last_cultivated_at=$2 WHERE character_id=$1", c, f.now)
	if _, err := f.svc.Join(f.ctx, u, key(), "kim_sa_mon", ""); err != nil {
		t.Fatal(err)
	}
	// Offline 5 h on a later game day (cooldown over), then switch: today's row must belong to kim_sa_mon.
	f.now = f.now.Add(8*24*time.Hour + 5*time.Hour)
	if _, err := f.svc.Switch(f.ctx, u, key(), "thuong_lan_dinh", ""); err != nil {
		t.Fatal(err)
	}
	day := gameday.Of(f.now)
	var fid string
	var pts int64
	f.scalar("SELECT faction_id FROM daily_contribution WHERE character_id=$1 AND game_day=$2", &fid, c, day)
	f.scalar("SELECT points_milli FROM daily_contribution WHERE character_id=$1 AND game_day=$2", &pts, c, day)
	if fid != "kim_sa_mon" || pts <= 0 {
		t.Fatalf("today's row faction %q points %d, want kim_sa_mon > 0", fid, pts)
	}
	if n := f.count("SELECT count(*) FROM cultivation_settlements WHERE character_id=$1 AND idempotency_key LIKE 'faction:%'", c); n != 1 { // the join settle had no elapsed time
		t.Fatalf("settlements %d", n)
	}
}

// reachLuanHoi records the dai_chien milestone as reached and schedules the era-start effects of luan_hoi exactly as
// world.Service.setEra does when the dai_chien event's on_end moves the era (effect_index = base + index), then sets
// the era. Returns the thuong_lan_dinh split effect id.
func (f *fixture) reachLuanHoi() int64 {
	f.t.Helper()
	m, ok := f.cb.World.Milestone("dai_chien")
	if !ok {
		f.t.Fatal("no dai_chien milestone")
	}
	f.exec(`INSERT INTO world_milestones_reached (milestone_id, seq, reached_at, reason, server_day, progress_milli, threshold_milli, target_day)
	        VALUES ($1, $2, $3, 'admin', 100, 0, 0, 100)`, m.ID, m.Seq, f.now)
	seq, _ := f.cb.World.EraSeq("luan_hoi")
	var id int64
	for _, ee := range f.cb.World.EraEffects(seq) {
		e := ee.Effect
		snap, _ := json.Marshal(e)
		due := f.now.Add(time.Duration(e.NoticeDays) * 24 * time.Hour)
		deadline := due.Add(time.Duration(e.ChooseDays) * 24 * time.Hour)
		var got int64
		f.scalar(`INSERT INTO world_effects (milestone_id, effect_index, kind, due_at, deadline_at, snapshot)
		          VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`, &got, m.ID, world.EraEffectIndexBase+ee.Index, string(e.Kind), due, deadline, snap)
		if e.From == "thuong_lan_dinh" {
			id = got
		}
	}
	if id == 0 {
		f.t.Fatal("luan_hoi schedules no thuong_lan_dinh split")
	}
	f.setEra("luan_hoi")
	return id
}

// Decision 2026-11-07: Tiên Đình (slug thuong_lan_dinh) closes at luan_hoi and its members are split into the tiên
// factions still open (computed: huyen_khung_mon, kim_sa_mon, tinh_ha_yeu_dinh — not vu_toc (path), not nhan_toc (opens at
// tao_nhan)) with decision 9's rules: 3-day notice (late notice keeps its full period, H1), 7-day choice, a locked
// member can't choose, defaults to the branch with the fewest split-placed members (H2), ties in YAML order.
func TestCloseSplitThuongLan(t *testing.T) {
	f := newFixture(t)
	fx := &faction.Effects{Pool: f.pool, Cat: f.cb.Factions, World: f.cb.World, Log: slog.New(slog.DiscardHandler), Now: func() time.Time { return f.now }}
	users := map[uuid.UUID]uuid.UUID{}
	var tl []uuid.UUID
	for range 6 {
		u, c := f.member("thuong_lan_dinh")
		tl = append(tl, c)
		users[c] = u
	}
	_, by := f.member("huyen_khung_mon") // a direct member of a branch: doesn't count for the default (H2)
	id := f.reachLuanHoi()
	reached := f.now
	branches := []string{"huyen_khung_mon", "kim_sa_mon", "tinh_ha_yeu_dinh"}

	// The worker runs a day late: the notice still gets its full 3 days (H1).
	f.now = reached.Add(24 * time.Hour)
	if res := f.runEffects(fx); res.Notices != 1 || res.Moved != 0 || f.announced(faction.KeySplitNotice) != 1 {
		t.Fatalf("notice: %+v", res)
	}
	var due time.Time
	f.scalar("SELECT due_at FROM world_effects WHERE id=$1", &due, id)
	if !due.Equal(f.now.Add(3 * 24 * time.Hour)) {
		t.Fatalf("due %v, want late notice + 3 days", due)
	}
	if m, err := f.svc.Me(f.ctx, users[tl[0]]); err != nil || m.Upcoming == nil || !slices.Equal(m.Upcoming.Into, branches) {
		t.Fatalf("upcoming: %+v %v", m.Upcoming, err)
	}
	// Nobody can join the closed faction any more.
	u0, _ := f.player()
	if _, err := f.svc.Join(f.ctx, u0, key(), "thuong_lan_dinh", ""); !errors.Is(err, faction.ErrFactionClosed) {
		t.Fatalf("join closed: %v", err)
	}

	f.now = due.Add(time.Minute)
	if res := f.runEffects(fx); res.Pending != 6 || res.Applied != 1 {
		t.Fatalf("due: %+v", res)
	}
	m, err := f.svc.Me(f.ctx, users[tl[0]])
	if err != nil || !slices.Equal(m.MustChoose, branches) || m.ChooseDeadline == nil || !m.ChooseDeadline.Equal(f.now.Add(7*24*time.Hour)) {
		t.Fatalf("must choose: %+v %v", m, err)
	}
	if _, err := f.svc.ChooseSplit(f.ctx, users[tl[0]], key(), "vu_toc"); !errors.Is(err, faction.ErrNotABranch) {
		t.Fatalf("vu_toc: %v", err)
	}
	if _, err := f.svc.ChooseSplit(f.ctx, users[tl[0]], key(), "huyen_khung_mon"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ChooseSplit(f.ctx, users[tl[1]], key(), "tinh_ha_yeu_dinh"); err != nil {
		t.Fatal(err)
	}
	f.exec("UPDATE character_faction SET lock_reason='admin', locked_at=$2 WHERE character_id=$1", tl[2], f.now)
	if _, err := f.svc.ChooseSplit(f.ctx, users[tl[2]], key(), "kim_sa_mon"); !errors.Is(err, faction.ErrLocked) {
		t.Fatalf("locked: %v", err)
	}

	// Deadline: placed by this split huyen 1, kim_sa 0, tinh_ha 1 (the direct huyen member doesn't count). The 4
	// undecided (incl. the locked one), in character_id order: kim_sa, huyen (tie → YAML order), kim_sa, tinh_ha.
	f.now = m.ChooseDeadline.Add(time.Minute)
	undecided := slices.Clone(tl[2:])
	slices.SortFunc(undecided, func(a, b uuid.UUID) int { return slices.Compare(a[:], b[:]) })
	if res := f.runEffects(fx); res.Moved != 4 || res.Finalized != 1 {
		t.Fatalf("finalize: %+v", res)
	}
	for i, want := range []string{"kim_sa_mon", "huyen_khung_mon", "kim_sa_mon", "tinh_ha_yeu_dinh"} {
		if got := f.factionOf(undecided[i]); got != want {
			t.Fatalf("undecided[%d] → %s, want %s", i, got, want)
		}
	}
	if got := f.factionOf(by); got != "huyen_khung_mon" {
		t.Fatalf("bystander moved to %s", got)
	}
	if n := f.count("SELECT count(*) FROM character_faction WHERE faction_id='thuong_lan_dinh'"); n != 0 {
		t.Fatalf("%d members left in the closed faction", n)
	}
	if res := f.runEffects(fx); res.Steps != 0 {
		t.Fatalf("re-run after finalize: %+v", res)
	}
}

// The computed branches include factions that close in a LATER era (huyen_khung_mon, kim_sa_mon at thanh_nhan).
// If thanh_nhan starts during the choice window, those branches take nobody new — a member placed there after their
// own split/migrate ran would be stranded — so choices are refused and defaults go to the branches still open.
func TestCloseSplitSkipsBranchesClosedByLaterEra(t *testing.T) {
	f := newFixture(t)
	fx := &faction.Effects{Pool: f.pool, Cat: f.cb.Factions, World: f.cb.World, Log: slog.New(slog.DiscardHandler), Now: func() time.Time { return f.now }}
	var tl []uuid.UUID
	users := map[uuid.UUID]uuid.UUID{}
	for range 3 {
		u, c := f.member("thuong_lan_dinh")
		tl = append(tl, c)
		users[c] = u
	}
	f.reachLuanHoi()
	f.runEffects(fx) // notice
	f.now = f.now.Add(3*24*time.Hour + time.Minute)
	if res := f.runEffects(fx); res.Pending != 3 {
		t.Fatalf("due: %+v", res)
	}
	f.setEra("thanh_nhan")
	m, err := f.svc.Me(f.ctx, users[tl[0]])
	if err != nil || !slices.Equal(m.MustChoose, []string{"tinh_ha_yeu_dinh"}) {
		t.Fatalf("must choose after thanh_nhan: %v %v", m.MustChoose, err)
	}
	for _, b := range []string{"huyen_khung_mon", "kim_sa_mon"} {
		if _, err := f.svc.ChooseSplit(f.ctx, users[tl[0]], key(), b); !errors.Is(err, faction.ErrFactionClosed) {
			t.Fatalf("choose %s after its era closed: %v", b, err)
		}
	}
	f.now = f.now.Add(7*24*time.Hour + time.Minute)
	if res := f.runEffects(fx); res.Moved != 3 || res.Finalized != 1 {
		t.Fatalf("finalize: %+v", res)
	}
	for _, c := range tl {
		if got := f.factionOf(c); got != "tinh_ha_yeu_dinh" {
			t.Fatalf("default into %s", got)
		}
	}
}
