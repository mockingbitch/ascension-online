//go:build integration

package secretrealm_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"tiendao/game/internal/character"
	"tiendao/game/internal/content"
	"tiendao/game/internal/cultivation"
	"tiendao/game/internal/db"
	"tiendao/game/internal/platform/postgres"
	"tiendao/game/internal/platform/testdb"
	"tiendao/game/internal/secretrealm"
	"tiendao/game/internal/secretrealm/bicanh/bicanhtest"
	"tiendao/game/internal/vip"
)

// winSeed: the fast reference player survives this seed with the shipped params (golden case bot_fast).
const winSeed uint32 = 20261107

// zeroRNG always draws 0: minimum stones/items, the artifact chance always hits, the first artifact is picked.
type zeroRNG struct{}

func (zeroRNG) IntN(int) int { return 0 }

// maxRNG always draws n−1: maximum stones/items and the artifact chance never hits.
type maxRNG struct{}

func (maxRNG) IntN(n int) int { return n - 1 }

type env struct {
	pool  *pgxpool.Pool
	cb    *content.Bundle
	chars *character.Service
	svc   *secretrealm.Service
	user  uuid.UUID
	char  uuid.UUID
	clock *time.Time
	rng   *secretrealm.RNG
}

func (e env) advance(d time.Duration) { *e.clock = e.clock.Add(d) }

func setup(t *testing.T) env {
	t.Helper()
	pool := testdb.New(t)
	cb, err := content.LoadAll("../../../../content/catalog")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 6, 1, 3, 0, 0, 0, time.UTC) // 10:00 UTC+7: the game day does not roll during a test
	clock := &now
	nowFn := func() time.Time { return *clock }
	cult := &cultivation.Service{Pool: pool, Realms: cb.Realms, Contribution: cb.Contribution, VIP: cb.VIP, Now: nowFn}
	var rng secretrealm.RNG = maxRNG{}
	e := env{pool: pool, cb: cb, chars: &character.Service{Pool: pool, Cultivation: cult}, clock: clock, rng: &rng}
	e.svc = &secretrealm.Service{Pool: pool, Cat: cb.SecretRealms, Realms: cb.Realms, Project: cult.ProjectedStage, VIP: cb.VIP, Now: nowFn,
		Seed: func() uint32 { return winSeed }, RNG: func() secretrealm.RNG { return *e.rng }}
	e.user, e.char = e.newCharacter(t, "Lâm Phong")
	return e
}

func (e env) newCharacter(t *testing.T, name string) (uuid.UUID, uuid.UUID) {
	t.Helper()
	user := uuid.New()
	if _, err := e.pool.Exec(context.Background(), "INSERT INTO users (id, email) VALUES ($1, $2)", user, user.String()+"@t"); err != nil {
		t.Fatal(err)
	}
	v, err := e.chars.Create(context.Background(), user, name)
	if err != nil {
		t.Fatal(err)
	}
	return user, v.ID
}

func (e env) scalar(t *testing.T, sql string, args ...any) (v int64) {
	t.Helper()
	if err := e.pool.QueryRow(context.Background(), sql, args...).Scan(&v); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return
}

func (e env) realm(t *testing.T) *secretrealm.Realm {
	r, ok := e.cb.SecretRealms.Get("hoang_co")
	if !ok {
		t.Fatal("no realm")
	}
	return r
}

// honest plays the seed with the fast reference player; returns the claim and the game time.
func (e env) honest(t *testing.T, seed uint32) (secretrealm.Claim, time.Duration) {
	t.Helper()
	p := e.realm(t).BiCanh()
	moves, r := bicanhtest.AutoPlay(p, seed, bicanhtest.Skills[0], 0)
	if !r.Won {
		t.Fatalf("reference player fell on seed %d", seed)
	}
	return secretrealm.Claim{EndTick: r.EndTick, Kills: r.Kills, Score: r.Score, StateHash: r.StateHash, Moves: moves},
		time.Duration(r.EndTick*p.TickMS) * time.Millisecond
}

func (e env) start(t *testing.T, user uuid.UUID, key string) secretrealm.StartView {
	t.Helper()
	v, err := e.svc.Start(context.Background(), user, "hoang_co", key)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	return v
}

// playOne starts, waits the game time and submits an honest log.
func (e env) playOne(t *testing.T, user uuid.UUID, n int) secretrealm.SubmitView {
	t.Helper()
	v := e.start(t, user, "start-key-"+string(rune('a'+n)))
	c, d := e.honest(t, v.Seed)
	e.advance(d + time.Second)
	out, err := e.svc.Submit(context.Background(), user, v.RunID, "submit-key-"+string(rune('a'+n)), c)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	return out
}

func TestRunsPerDayAndIdempotentLoot(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	r := e.realm(t)
	first := e.playOne(t, e.user, 0)
	if first.Verdict != secretrealm.VerdictPass || first.Band == nil || first.Loot == nil || first.Loot.SpiritStone <= 0 {
		t.Fatalf("honest survived run: %+v", first)
	}
	if first.VIPPoints != 2 {
		t.Fatalf("surviving should pay trial_pass điểm duyên, got %d", first.VIPPoints)
	}
	stones := e.scalar(t, "SELECT COALESCE(sum(delta),0) FROM currency_ledger WHERE character_id=$1 AND reason='secret_realm'", e.char)
	if stones != first.Loot.SpiritStone {
		t.Fatalf("ledger %d != loot %d", stones, first.Loot.SpiritStone)
	}
	// Same key → same verdict and loot, no second grant; another key → 409.
	again, err := e.svc.Submit(ctx, e.user, first.RunID, "submit-key-a", secretrealm.Claim{})
	if err != nil || !again.Replayed || again.Loot == nil || again.Loot.SpiritStone != first.Loot.SpiritStone || again.VIPPoints != first.VIPPoints {
		t.Fatalf("replay: %+v %v", again, err)
	}
	if _, err := e.svc.Submit(ctx, e.user, first.RunID, "other-key-xyz", secretrealm.Claim{}); !errors.Is(err, secretrealm.ErrAlreadySubmitted) {
		t.Fatalf("other key: %v", err)
	}
	if n := e.scalar(t, "SELECT count(*) FROM currency_ledger WHERE character_id=$1 AND reason='secret_realm'", e.char); n != 1 {
		t.Fatalf("ledger rows %d", n)
	}
	// Start resumes an open run with the same seed (not a new run).
	v := e.start(t, e.user, "start-key-b")
	if res := e.start(t, e.user, "start-key-b2"); !res.Resumed || res.RunID != v.RunID {
		t.Fatalf("open run should resume: %+v", res)
	}
	// Abandon it: it already counted. One more run fits, then the daily limit.
	e.advance(time.Duration(r.ExpireAfterSeconds+1) * time.Second)
	e.playOne(t, e.user, 2) // closes the abandoned run, third run of the day
	if n := e.scalar(t, "SELECT count(*) FROM secret_realm_runs WHERE character_id=$1 AND verdict='expired' AND reason='abandoned'", e.char); n != 1 {
		t.Fatalf("abandoned runs %d", n)
	}
	_, err = e.svc.Start(ctx, e.user, "hoang_co", "start-key-d")
	var blocked *secretrealm.ErrBlocked
	if !errors.As(err, &blocked) || blocked.Code != secretrealm.BlockDailyLimit || blocked.Status.RunsLeft != 0 {
		t.Fatalf("4th run: %v", err)
	}
	// Next game day: runs reset.
	e.advance(24 * time.Hour)
	e.start(t, e.user, "start-key-e")
}

func TestDailyCapsClipLoot(t *testing.T) {
	e := setup(t)
	r := e.realm(t)
	// Pre-fill today's totals right under the caps; the next run is clipped to what is left.
	if _, err := e.pool.Exec(context.Background(), `INSERT INTO secret_realm_days (character_id, game_day, runs, spirit_stone, materials, artifacts)
		VALUES ($1, $2, 0, $3, $4, $5)`, e.char, "2026-06-01", r.DailyCaps.SpiritStone-5, r.DailyCaps.Materials, r.DailyCaps.Artifacts); err != nil {
		t.Fatal(err)
	}
	var z secretrealm.RNG = zeroRNG{} // artifact chance hits
	*e.rng = z
	out := e.playOne(t, e.user, 0)
	l := out.Loot
	if l == nil || l.SpiritStone != 5 || l.Materials() != 0 || l.Artifact != "" || !l.ArtifactCapped || !l.Clipped {
		t.Fatalf("clipped loot: %+v", l)
	}
	if got := e.scalar(t, "SELECT spirit_stone FROM secret_realm_days WHERE character_id=$1", e.char); got != r.DailyCaps.SpiritStone {
		t.Fatalf("day total %d", got)
	}
	if n := e.scalar(t, "SELECT count(*) FROM inventory_transactions WHERE character_id=$1 AND reason='secret_realm'", e.char); n != 0 {
		t.Fatalf("no items should be granted past the caps, got %d rows", n)
	}
}

func TestArtifactDropAndDuplicate(t *testing.T) {
	e := setup(t)
	var z secretrealm.RNG = zeroRNG{}
	*e.rng = z
	r := e.realm(t)
	first := e.playOne(t, e.user, 0)
	if first.Loot.Artifact != r.Artifacts[0].ItemID {
		t.Fatalf("artifact: %+v", first.Loot)
	}
	if q := e.scalar(t, "SELECT quantity FROM inventory_items WHERE character_id=$1 AND item_id=$2", e.char, r.Artifacts[0].ItemID); q != 1 {
		t.Fatalf("artifact quantity %d", q)
	}
	// Next game day (fresh caps): the same pháp bảo again turns into duplicate stones.
	e.advance(24 * time.Hour)
	second := e.playOne(t, e.user, 1)
	if second.Loot.Artifact != "" || second.Loot.DuplicateOf != r.Artifacts[0].ItemID || second.Loot.RolledStone < r.Artifacts[0].DuplicateStones {
		t.Fatalf("duplicate: %+v", second.Loot)
	}
	if q := e.scalar(t, "SELECT quantity FROM inventory_items WHERE character_id=$1 AND item_id=$2", e.char, r.Artifacts[0].ItemID); q != 1 {
		t.Fatalf("duplicate must not stack: %d", q)
	}
}

func TestConcurrentSubmitsGrantOnce(t *testing.T) {
	e := setup(t)
	v := e.start(t, e.user, "start-key-a")
	c, d := e.honest(t, v.Seed)
	e.advance(d + time.Second)
	var wg sync.WaitGroup
	results := make([]error, 4)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, results[i] = e.svc.Submit(context.Background(), e.user, v.RunID, "concurrent-"+string(rune('a'+i))+"-key", c)
		}(i)
	}
	wg.Wait()
	ok := 0
	for _, err := range results {
		switch {
		case err == nil:
			ok++
		case !errors.Is(err, secretrealm.ErrAlreadySubmitted):
			t.Fatalf("unexpected: %v", err)
		}
	}
	if ok != 1 {
		t.Fatalf("%d submissions accepted", ok)
	}
	if n := e.scalar(t, "SELECT count(*) FROM currency_ledger WHERE character_id=$1 AND reason='secret_realm'", e.char); n != 1 {
		t.Fatalf("ledger rows %d", n)
	}
}

func TestTamperedAndForeignRunsEarnNothing(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	v := e.start(t, e.user, "start-key-a")
	c, d := e.honest(t, v.Seed)
	// Too fast.
	e.advance(d / 3)
	out, err := e.svc.Submit(ctx, e.user, v.RunID, "submit-key-a", c)
	if err != nil || out.Verdict != secretrealm.VerdictRejected || out.Reason != secretrealm.ReasonTooFast || out.Loot != nil || out.Outcome != nil {
		t.Fatalf("too fast: %+v %v", out, err)
	}
	// Another character can't submit (or see) this run.
	other, _ := e.newCharacter(t, "Tô Vân")
	if _, err := e.svc.Submit(ctx, other, v.RunID, "submit-key-o", c); !errors.Is(err, secretrealm.ErrRunNotFound) {
		t.Fatalf("foreign run: %v", err)
	}
	// Inflated score.
	v2 := e.start(t, e.user, "start-key-b")
	c2, d2 := e.honest(t, v2.Seed)
	c2.Score += 10
	e.advance(d2 + time.Second)
	out, err = e.svc.Submit(ctx, e.user, v2.RunID, "submit-key-b", c2)
	if err != nil || out.Verdict != secretrealm.VerdictRejected || out.Loot != nil {
		t.Fatalf("inflated: %+v %v", out, err)
	}
	if n := e.scalar(t, "SELECT count(*) FROM currency_ledger WHERE character_id=$1 AND reason='secret_realm'", e.char); n != 0 {
		t.Fatalf("rejected runs granted %d rows", n)
	}
	// Late.
	v3 := e.start(t, e.user, "start-key-c")
	c3, _ := e.honest(t, v3.Seed)
	e.advance(time.Duration(e.realm(t).ExpireAfterSeconds+5) * time.Second)
	if out, err = e.svc.Submit(ctx, e.user, v3.RunID, "submit-key-c", c3); err != nil || out.Verdict != secretrealm.VerdictExpired {
		t.Fatalf("late: %+v %v", out, err)
	}
}

// Fairness: a Tiên Duyên 12 character gets exactly the same runs/day and loot as a free one.
func TestTienDuyenDoesNotChangeRunsOrLoot(t *testing.T) {
	e := setup(t)
	vipUser, _ := e.newCharacter(t, "Hạc Vân")
	if _, err := e.pool.Exec(context.Background(), "INSERT INTO vip_accounts (user_id, points) VALUES ($1, 10000000) ON CONFLICT (user_id) DO UPDATE SET points=10000000", vipUser); err != nil {
		t.Fatal(err)
	}
	free := e.playOne(t, e.user, 0)
	rich := e.playOne(t, vipUser, 1)
	if free.Loot.RolledStone != rich.Loot.RolledStone || free.Status.RunsPerDay != rich.Status.RunsPerDay || free.Status.DailyCaps != rich.Status.DailyCaps {
		t.Fatalf("Tiên Duyên changed bí cảnh: free %+v vs TD12 %+v", free.Loot, rich.Loot)
	}
}

// M1: a replayed submit never credits điểm duyên again — not even on a later game day when the cap has room again.
func TestReplayOnLaterDayPaysNoDuyen(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	// Use up today's trial_pass activity (2 × 2 điểm) with other keys so the pass itself earns 0.
	for i := range 2 {
		err := postgres.InTx(ctx, e.pool, func(tx pgx.Tx) error {
			_, err := e.cb.VIP.EarnLocked(ctx, db.New(tx), e.user, vip.ActTrialPass, vip.Ref{Type: "test", ID: "x", IdemKey: "fill-" + string(rune('a'+i))}, *e.clock)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	v := e.start(t, e.user, "start-key-m1")
	c, d := e.honest(t, v.Seed)
	e.advance(d + time.Second)
	first, err := e.svc.Submit(ctx, e.user, v.RunID, "submit-key-m1", c)
	if err != nil || first.Verdict != secretrealm.VerdictPass || first.VIPPoints != 0 {
		t.Fatalf("pass with the cap used: %+v %v", first, err)
	}
	e.advance(24 * time.Hour)
	again, err := e.svc.Submit(ctx, e.user, v.RunID, "submit-key-m1", c)
	if err != nil || !again.Replayed || again.VIPPoints != 0 {
		t.Fatalf("replay next day: %+v %v", again, err)
	}
	if n := e.scalar(t, "SELECT count(*) FROM vip_point_ledger WHERE user_id=$1 AND idempotency_key LIKE 'sr:%'", e.user); n != 0 {
		t.Fatalf("replay credited điểm duyên: %d rows", n)
	}
	// And a pass with room reports its credit again on replay (from the stored ledger row).
	v2 := e.start(t, e.user, "start-key-m1b")
	c2, d2 := e.honest(t, v2.Seed)
	e.advance(d2 + time.Second)
	p, err := e.svc.Submit(ctx, e.user, v2.RunID, "submit-key-m1b", c2)
	if err != nil || p.VIPPoints != 2 {
		t.Fatalf("pass with room: %+v %v", p, err)
	}
	if r, err := e.svc.Submit(ctx, e.user, v2.RunID, "submit-key-m1b", c2); err != nil || r.VIPPoints != 2 || !r.Replayed {
		t.Fatalf("replay shows the stored credit: %+v %v", r, err)
	}
}
