//go:build integration

package hall_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"tiendao/game/internal/content"
	"tiendao/game/internal/cultivation"
	"tiendao/game/internal/lecture"
	"tiendao/game/internal/lecture/hall"
	"tiendao/game/internal/platform/redisx"
	"tiendao/game/internal/platform/testdb"
)

// redisForTest connects to TEST_REDIS_URL (default db 15) and wipes lecture:* before and after.
func redisForTest(t *testing.T) *redis.Client {
	t.Helper()
	ctx := context.Background()
	url := os.Getenv("TEST_REDIS_URL")
	if url == "" {
		url = "redis://localhost:2379/15"
	}
	rdb, err := redisx.Connect(ctx, url)
	if err != nil {
		t.Fatalf("%v (run `make db-up`)", err)
	}
	clean := func() {
		keys, _ := rdb.Keys(ctx, "lecture:*").Result()
		if len(keys) > 0 {
			rdb.Del(ctx, keys...)
		}
	}
	clean()
	t.Cleanup(func() { clean(); rdb.Close() })
	return rdb
}

// smallHall: the shipped config with `seats` cushions per điện and merge threshold `below`.
func smallHall(t *testing.T, rdb *redis.Client, seats, below int) *hall.Hall {
	t.Helper()
	cfg, err := lecture.LoadFile("../../../../../content/catalog/lectures.yaml")
	if err != nil {
		t.Fatal(err)
	}
	c := *cfg
	c.Seats, c.MergeBelow = seats, below
	return &hall.Hall{R: rdb, Cfg: &c}
}

var t0 = time.Date(2026, 11, 10, 8, 0, 0, 0, time.UTC)

func enter(t *testing.T, h *hall.Hall, id string, aff hall.Affinity) (int, int) {
	t.Helper()
	hid, seat, _, err := h.Enter(context.Background(), hall.Listener{CharacterID: id, Name: id}, aff, t0)
	if err != nil {
		t.Fatalf("enter %s: %v", id, err)
	}
	return hid, seat
}

func counts(t *testing.T, h *hall.Hall) map[int]int {
	t.Helper()
	_, hs, err := h.Counts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	m := map[int]int{}
	for _, x := range hs {
		m[x.ID] = x.Count
	}
	return m
}

func TestAssignFullestThenOverflow(t *testing.T) {
	h := smallHall(t, redisForTest(t), 3, 0)
	for i := range 3 {
		if hid, seat := enter(t, h, fmt.Sprint("a", i), hall.Affinity{}); hid != 1 || seat != i {
			t.Fatalf("a%d → điện %d seat %d", i, hid, seat)
		}
	}
	// điện 1 full → a new điện opens; nobody is ever outside
	if hid, seat := enter(t, h, "a3", hall.Affinity{}); hid != 2 || seat != 0 {
		t.Fatalf("overflow → %d/%d", hid, seat)
	}
	// re-enter keeps the cushion
	if hid, seat := enter(t, h, "a1", hall.Affinity{}); hid != 1 || seat != 1 {
		t.Fatalf("re-enter moved: %d/%d", hid, seat)
	}
	// a seat frees in 1; a newcomer goes to the fullest non-full điện: 1 (2 listeners) beats 2 (1 listener)
	if _, ok, err := h.Leave(context.Background(), "a2"); err != nil || !ok {
		t.Fatal(err)
	}
	if hid, _ := enter(t, h, "a4", hall.Affinity{}); hid != 1 {
		t.Fatalf("fullest: went to %d", hid)
	}
	// lowest free number is reused when a điện closes
	if _, _, err := h.Leave(context.Background(), "a3"); err != nil {
		t.Fatal(err)
	}
	if c := counts(t, h); c[2] != 0 || len(c) != 1 {
		t.Fatalf("điện 2 should close: %v", c)
	}
	if hid, _ := enter(t, h, "a5", hall.Affinity{}); hid != 2 {
		t.Fatalf("reopen 2: %d", hid)
	}
}

func TestAssignDaoLuThenFaction(t *testing.T) {
	h := smallHall(t, redisForTest(t), 3, 0)
	// điện 1: x0 x1 (faction A), x2 → full; điện 2: y0 (faction B); điện 3: z0, z1 (no faction)
	enter(t, h, "x0", hall.Affinity{FactionID: "a"})
	enter(t, h, "x1", hall.Affinity{FactionID: "a"})
	enter(t, h, "x2", hall.Affinity{})
	if hid, _ := enter(t, h, "y0", hall.Affinity{FactionID: "b"}); hid != 2 {
		t.Fatalf("y0 → %d", hid)
	}
	if hid, _ := enter(t, h, "z0", hall.Affinity{FactionID: "b"}); hid != 2 {
		t.Fatalf("same faction → %d", hid)
	}
	// điện 2 has two of faction B; a B newcomer still joins them (not full)
	if hid, _ := enter(t, h, "y1", hall.Affinity{FactionID: "b"}); hid != 2 {
		t.Fatalf("faction b → %d", hid)
	}
	// now 1 and 2 are full: faction A newcomer opens 3
	if hid, _ := enter(t, h, "x3", hall.Affinity{FactionID: "a"}); hid != 3 {
		t.Fatalf("x3 → %d", hid)
	}
	enter(t, h, "w0", hall.Affinity{}) // fullest non-full = 3
	// đạo lữ first: free one cushion in 1 and one in 3 → 3 has (x3, w0) = 2, 1 has 2 too.
	h.Leave(context.Background(), "x2")
	// a faction-B player whose đạo lữ sits in 1 goes to 1 even though faction B is in 2 (full anyway) and 3 ties
	if hid, _ := enter(t, h, "p0", hall.Affinity{PartnerID: "x0", FactionID: "b"}); hid != 1 {
		t.Fatalf("đạo lữ: → %d", hid)
	}
	// đạo lữ's điện full → falls back to faction/fullest (3 has a free cushion)
	if hid, _ := enter(t, h, "p1", hall.Affinity{PartnerID: "x0", FactionID: "a"}); hid != 3 {
		t.Fatalf("đạo lữ full: → %d", hid)
	}
	// faction preference among non-full: open 4 & 5 … a fresh layout
	h2 := smallHall(t, redisForTest(t), 4, 0)
	enter(t, h2, "m0", hall.Affinity{})
	enter(t, h2, "m1", hall.Affinity{})
	enter(t, h2, "m2", hall.Affinity{})
	enter(t, h2, "m3", hall.Affinity{})               // điện 1 full
	enter(t, h2, "f0", hall.Affinity{FactionID: "k"}) // 2
	enter(t, h2, "n0", hall.Affinity{})               // 2 (fullest non-full)
	enter(t, h2, "n1", hall.Affinity{})               // 2
	enter(t, h2, "n2", hall.Affinity{})               // 2 → full
	enter(t, h2, "g0", hall.Affinity{FactionID: "k"}) // 3
	enter(t, h2, "o0", hall.Affinity{})               // 3
	enter(t, h2, "o1", hall.Affinity{})               // 3 (3 listeners)
	h2.Leave(context.Background(), "n0")              // 2 has 3, one of faction k; 3 has 3, one of faction k
	h2.Leave(context.Background(), "o1")              // 3 has 2 (g0 k, o0)
	if hid, _ := enter(t, h2, "k1", hall.Affinity{FactionID: "k"}); hid != 2 {
		t.Fatalf("faction tie → fullest: %d", hid) // both have one k; 2 is fuller
	}
	h2.Leave(context.Background(), "f0") // 2 now has no k
	if hid, _ := enter(t, h2, "k2", hall.Affinity{FactionID: "k"}); hid != 2 {
		// 2 has k1 (k) and 3 has g0 (k): tie on faction (1 each), 2 is fuller (3 vs 2)
		t.Fatalf("k2 → %d", hid)
	}
	h2.Leave(context.Background(), "k1")
	h2.Leave(context.Background(), "k2") // 2: n1 n2 (no k, 2) ; 3: g0 o0 (one k, 2)
	enter(t, h2, "n3", hall.Affinity{})  // fullest tie → lower number: 2 (3 listeners)
	if hid, _ := enter(t, h2, "k3", hall.Affinity{FactionID: "k"}); hid != 3 {
		t.Fatalf("faction beats fullest: %d", hid) // 3 has a k member, 2 is fuller but has none
	}
}

func TestConcurrentEntersNeverExceedCapacity(t *testing.T) {
	h := smallHall(t, redisForTest(t), 60, 0)
	const n = 400
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, _, err := h.Enter(context.Background(), hall.Listener{CharacterID: fmt.Sprint("c", i)}, hall.Affinity{FactionID: fmt.Sprint("f", i%3)}, t0)
			if err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	total, hs, err := h.Counts(context.Background())
	if err != nil || total != n {
		t.Fatalf("total %d %v", total, err)
	}
	sum := 0
	for _, x := range hs {
		if x.Count > 60 || x.Count < 1 {
			t.Fatalf("điện %d has %d", x.ID, x.Count)
		}
		sum += x.Count
	}
	if sum != n || len(hs) != 7 { // ⌈400/60⌉ — fullest-first never leaves a half-empty điện behind
		t.Fatalf("sum %d over %d điện", sum, len(hs))
	}
	// every cushion is unique: each listener's (điện, seat) is distinct
	seen := map[[2]int]bool{}
	for i := range n {
		hid, seat, ok, err := h.Where(context.Background(), fmt.Sprint("c", i))
		if err != nil || !ok || seen[[2]int{hid, seat}] {
			t.Fatalf("c%d: %d/%d %v %v", i, hid, seat, ok, err)
		}
		seen[[2]int{hid, seat}] = true
	}
}

func TestSwitch(t *testing.T) {
	h := smallHall(t, redisForTest(t), 2, 0)
	enter(t, h, "s0", hall.Affinity{})
	enter(t, h, "s1", hall.Affinity{}) // 1 full
	enter(t, h, "s2", hall.Affinity{}) // 2
	ctx := context.Background()
	if _, _, _, err := h.Switch(ctx, "s2", 1); !errors.Is(err, hall.ErrHallFull) {
		t.Fatalf("switch to full: %v", err)
	}
	if _, _, _, err := h.Switch(ctx, "s2", 9); !errors.Is(err, hall.ErrHallUnknown) {
		t.Fatalf("switch to closed: %v", err)
	}
	if _, _, _, err := h.Switch(ctx, "ghost", 2); !errors.Is(err, hall.ErrNotPresent) {
		t.Fatalf("switch absent: %v", err)
	}
	// same điện: idempotent no-op
	if _, seat, moved, err := h.Switch(ctx, "s2", 2); err != nil || moved || seat != 0 {
		t.Fatalf("same: %v %v %d", err, moved, seat)
	}
	from, seat, moved, err := h.Switch(ctx, "s0", 2)
	if err != nil || !moved || from.HallID != 1 || from.Seat != 0 || seat != 1 {
		t.Fatalf("move: %+v %d %v %v", from, seat, moved, err)
	}
	if c := counts(t, h); c[1] != 1 || c[2] != 2 {
		t.Fatalf("counts %v", c)
	}
	// seats lowered under a live Redis (điện 2 holds more than the new cap): the switch is refused and nobody loses
	// their cushion
	low := smallHall(t, h.R, 1, 0)
	enter(t, h, "s3", hall.Affinity{}) // 1 has s1 only → s3 to 1 (cap 2)
	h.Leave(ctx, "s3")
	if _, _, _, err := low.Switch(ctx, "s1", 2); !errors.Is(err, hall.ErrHallFull) {
		t.Fatalf("lowered cap: %v", err)
	}
	if hid, seat, ok, _ := h.Where(ctx, "s1"); !ok || hid != 1 || seat != 1 {
		t.Fatalf("s1 lost its cushion: %d/%d %v", hid, seat, ok)
	}
	// moving the last listener out closes the điện
	if _, _, _, err := h.Switch(ctx, "s1", 2); !errors.Is(err, hall.ErrHallFull) {
		t.Fatalf("2 is full now: %v", err)
	}
}

func TestMergeNearEmpty(t *testing.T) {
	h := smallHall(t, redisForTest(t), 10, 4)
	ctx := context.Background()
	for i := range 20 {
		enter(t, h, fmt.Sprint("m", i), hall.Affinity{})
	}
	// 1: m0..m9, 2: m10..m19 → leave to shape 1:7, 2:3
	for _, id := range []string{"m7", "m8", "m9", "m13", "m14", "m15", "m16", "m17", "m18", "m19"} {
		h.Leave(ctx, id)
	}
	if c := counts(t, h); c[1] != 7 || c[2] != 3 {
		t.Fatalf("shape %v", c)
	}
	moves, err := h.Merge(ctx, 7)
	if err != nil {
		t.Fatal(err)
	}
	// 2 (3 < 4) folds into 1, which has room for all three
	if c := counts(t, h); len(c) != 1 || c[1] != 10 {
		t.Fatalf("after merge %v", c)
	}
	if len(moves) != 3 || moves[0].From != 2 || moves[0].To != 1 {
		t.Fatalf("moves %+v", moves)
	}
	// once per lecture number (fully merged → marker set)
	enter(t, h, "late", hall.Affinity{}) // 1 is full → opens 2 with one listener
	if again, _ := h.Merge(ctx, 7); len(again) != 0 {
		t.Fatalf("merged twice for one lecture: %+v", again)
	}
	// nowhere to go (1 full): stays
	if m, _ := h.Merge(ctx, 8); len(m) != 0 {
		t.Fatalf("merged into a full điện: %+v", m)
	}
	if c := counts(t, h); c[2] != 1 {
		t.Fatalf("late moved: %v", c)
	}
	// a seat in 1 frees → next lecture the lone listener joins it
	h.Leave(ctx, "m0")
	if m, _ := h.Merge(ctx, 9); len(m) != 1 || m[0].CharacterID != "late" || m[0].To != 1 || m[0].ToSeat != 0 {
		t.Fatalf("late merge %+v", m)
	}
	if hid, seat, ok, _ := h.Where(ctx, "late"); !ok || hid != 1 || seat != 0 {
		t.Fatalf("late at %d/%d", hid, seat)
	}
}

// Service-level: the đạo lữ and faction hints come from Postgres; switching announces on both topics.
func TestServiceAffinityAndSwitch(t *testing.T) {
	ctx := context.Background()
	pool := testdb.New(t)
	rdb := redisForTest(t)
	cb, err := content.LoadAll("../../../../../content/catalog")
	if err != nil {
		t.Fatal(err)
	}
	cfg := *cb.Lectures
	cfg.Seats = 2
	now := t0
	clock := func() time.Time { return now }
	cult := &cultivation.Service{Pool: pool, Realms: cb.Realms, Contribution: cb.Contribution, VIP: cb.VIP, Tasks: cb.DailyTasks, Now: clock}
	pub := &published{}
	svc := &hall.Service{Pool: pool, Hall: &hall.Hall{R: rdb, Cfg: &cfg}, Cfg: &cfg, Cultivation: cult, VIP: cb.VIP, Now: clock, Publish: pub.fn}
	mk := func(name, faction string) (uuid.UUID, uuid.UUID) {
		t.Helper()
		u, ch := uuid.New(), uuid.New()
		qs := []string{
			"INSERT INTO users (id, email) VALUES ($1, $2::text || '@t' || md5($3))",
			"INSERT INTO characters (id, user_id, name) VALUES ($2, $1, $3)",
			"INSERT INTO character_progressions (character_id, realm_id, stage_id, last_cultivated_at, stage_reached_at) SELECT $2, 'luyen_khi', 'luyen_khi.tang_1', now(), now() WHERE $1::uuid IS NOT NULL AND $3::text IS NOT NULL",
		}
		for _, q := range qs {
			if _, err := pool.Exec(ctx, q, u, ch, name); err != nil {
				t.Fatal(q, err)
			}
		}
		if faction != "" {
			if _, err := pool.Exec(ctx, "INSERT INTO character_faction (character_id, faction_id, joined_at) VALUES ($1, $2, now())", ch, faction); err != nil {
				t.Fatal(err)
			}
		}
		return u, ch
	}
	fac := cb.Factions.Factions[0].ID
	a, aCh := mk("Thanh Vân", "")
	b, _ := mk("Lục Bình", "")
	c, cCh := mk("Bạch Lộ", fac) // fills 2 with … below
	d, dCh := mk("Đạo Lữ", "")
	// a, b → điện 1 (full); c → 2
	for _, u := range []uuid.UUID{a, b, c} {
		if _, err := svc.Enter(ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	// d is a's đạo lữ but 1 is full → 2. Then a moves to 2? No: 2 has c + d = full. Bond first.
	if _, err := pool.Exec(ctx, `WITH p AS (INSERT INTO dao_lu_proposals (proposer_character_id, target_character_id, status, idempotency_key, game_day, created_at, expires_at, responded_at)
		VALUES ($1, $2, 'accepted', 'k-bond-1', CURRENT_DATE, now(), now() + interval '1 day', now()) RETURNING id),
		bnd AS (INSERT INTO dao_lu_bonds (proposal_id, character_a, character_b, formed_at) SELECT id, $1, $2, now() FROM p RETURNING id)
		INSERT INTO dao_lu_members (bond_id, character_id) SELECT id, x FROM bnd, unnest(ARRAY[$1::uuid, $2::uuid]) AS x`, cCh, dCh); err != nil {
		t.Fatal(err)
	}
	st, err := svc.Enter(ctx, d)
	if err != nil || st.Me.HallID != 2 {
		t.Fatalf("đạo lữ's điện: %+v %v", st.Me, err)
	}
	if st.Me.PartnerID != cCh.String() || st.Me.FactionID != "" {
		t.Fatalf("display hints %+v", st.Me)
	}
	for _, l := range st.Seated {
		if l.CharacterID == cCh.String() && l.FactionID != fac {
			t.Fatalf("listener faction %+v", l)
		}
	}
	if st.Count != 2 || st.GlobalListeners != 4 || len(st.Halls) != 2 || st.Capacity != 2 {
		t.Fatalf("state %+v", st)
	}
	// switch into a full điện is refused; a frees a cushion in 1 by leaving and d switches there
	if _, err := svc.Switch(ctx, a, 2); !errors.Is(err, hall.ErrHallFull) {
		t.Fatalf("switch full: %v", err)
	}
	if err := svc.Leave(ctx, b); err != nil {
		t.Fatal(err)
	}
	st, err = svc.Switch(ctx, d, 1)
	if err != nil || st.Me.HallID != 1 || st.HallID != 1 || len(st.Seated) != 2 {
		t.Fatalf("switch: %+v %v", st, err)
	}
	if !pub.hasTopic("hall:2|lecture.leave:"+dCh.String()) || !pub.hasTopic("hall:1|lecture.enter:"+dCh.String()) {
		t.Fatalf("switch events %v", pub.topics)
	}
	if !pub.hasTopic("hall:1|lecture.enter:" + aCh.String()) {
		t.Fatal("enter topic")
	}
	if _, err := svc.Switch(ctx, d, 0); !errors.Is(err, hall.ErrHallUnknown) {
		t.Fatalf("switch 0: %v", err)
	}
}

// A large backlog is spread over calls: each call moves at most a bounded number of điện, the marker is set only
// when nothing qualifies.
func TestMergeBudget(t *testing.T) {
	h := smallHall(t, redisForTest(t), 2, 2)
	ctx := context.Background()
	// 20 điện of 2, then everyone in odd seats leaves → 20 điện with 1 listener each
	for i := range 40 {
		enter(t, h, fmt.Sprint("b", i), hall.Affinity{})
	}
	for i := 1; i < 40; i += 2 {
		h.Leave(ctx, fmt.Sprint("b", i))
	}
	calls := 0
	for {
		m, err := h.Merge(ctx, 1)
		if err != nil {
			t.Fatal(err)
		}
		if len(m) == 0 {
			break
		}
		if calls++; calls > 10 {
			t.Fatal("merge never finished")
		}
		if len(m) > 8 {
			t.Fatalf("one call moved %d (budget 8 điện × 1)", len(m))
		}
	}
	if c := counts(t, h); len(c) != 10 || calls < 2 {
		t.Fatalf("after %d calls: %v", calls, c)
	}
}

func TestValidTopic(t *testing.T) {
	cfg := &lecture.Config{MaxHalls: 500}
	for topic, want := range map[string]bool{"hall:1": true, "hall:500": true, "hall:501": false, "hall:0": false, "hall:01": false,
		"hall:": false, "hall:-1": false, "hall:1x": false, "world": false, "hall:99999999": false} {
		if hall.ValidTopic(cfg, topic) != want {
			t.Errorf("%q: want %v", topic, want)
		}
	}
}
