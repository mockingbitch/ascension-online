//go:build integration

package announce_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"

	"tiendao/game/internal/announce"
	"tiendao/game/internal/content"
	"tiendao/game/internal/db"
	"tiendao/game/internal/platform/redisx"
	"tiendao/game/internal/platform/testdb"
	"tiendao/game/internal/realtime"
)

type sink struct {
	mu   sync.Mutex
	envs []realtime.Envelope
}

func (s *sink) publish(_ context.Context, e realtime.Envelope) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.envs = append(s.envs, e)
	return nil
}

var published = map[*announce.Service]*sink{}

func setup(t *testing.T) (*announce.Service, *redis.Client, func(name string) uuid.UUID, *time.Time) {
	t.Helper()
	ctx := context.Background()
	pool := testdb.New(t)
	url := os.Getenv("TEST_REDIS_URL")
	if url == "" {
		url = "redis://localhost:2379/15"
	}
	rdb, err := redisx.Connect(ctx, url)
	if err != nil {
		t.Fatalf("%v (run `make db-up`)", err)
	}
	cb, err := content.LoadAll("../../../../content/catalog")
	if err != nil {
		t.Fatal(err)
	}
	tpl, err := realtime.LoadTemplates("../../../../content/story/announcements/major-breakthrough.yaml")
	if err != nil {
		t.Fatal(err)
	}
	prefix := "test:" + uuid.NewString() + ":"
	t.Cleanup(func() {
		keys, _ := rdb.Keys(context.Background(), prefix+"*").Result()
		if len(keys) > 0 {
			rdb.Del(context.Background(), keys...)
		}
		rdb.Close()
	})
	now := time.Date(2026, 11, 9, 5, 0, 1, 0, time.UTC)
	out := &sink{}
	// never broadcast from a test: pub/sub is server-wide and a dev API may be subscribed
	svc := &announce.Service{Pool: pool, Redis: rdb, Tpl: tpl, Realms: cb.Realms, Cfg: cb.Announce, KeyPrefix: prefix,
		Now: func() time.Time { return now }, PublishFn: out.publish}
	published[svc] = out
	mk := func(name string) uuid.UUID {
		u, c := uuid.New(), uuid.New()
		if _, err := pool.Exec(ctx, "INSERT INTO users (id, email) VALUES ($1, $2)", u, u.String()+"@t"); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, "INSERT INTO characters (id, user_id, name) VALUES ($1, $2, $3)", c, u, name); err != nil {
			t.Fatal(err)
		}
		return c
	}
	return svc, rdb, mk, &now
}

// Exactly one character is ever first to a realm id, even when two commits race.
func TestRealmFirstIsRaceFree(t *testing.T) {
	svc, _, mk, _ := setup(t)
	ctx := context.Background()
	a, b := mk("Thứ Nhất"), mk("Thứ Nhì")
	var wg sync.WaitGroup
	wins := make(chan int64, 2)
	for _, c := range []uuid.UUID{a, b} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n, err := db.New(svc.Pool).InsertRealmFirst(ctx, db.InsertRealmFirstParams{RealmID: "truc_co", CharacterID: pgtype.UUID{Bytes: c, Valid: true}, Now: time.Now()})
			if err != nil {
				t.Error(err)
			}
			wins <- n
		}()
	}
	wg.Wait()
	close(wins)
	total := int64(0)
	for n := range wins {
		total += n
	}
	if total != 1 {
		t.Fatalf("winners = %d, want exactly 1", total)
	}
	// per realm id: the Vu realm of the same tier has its own first
	if n, _ := db.New(svc.Pool).InsertRealmFirst(ctx, db.InsertRealmFirstParams{RealmID: "vu_2", CharacterID: pgtype.UUID{Bytes: a, Valid: true}, Now: time.Now()}); n != 1 {
		t.Fatal("vu_2 must have its own first")
	}
}

func event(t *testing.T, c uuid.UUID, name, realmID string, trib, first bool) []byte {
	t.Helper()
	b, _ := json.Marshal(announce.Event{CharacterID: c.String(), Name: name, RealmID: realmID, StageID: realmID + ".so_ky", Path: "tien", Tribulation: trib, First: first})
	return b
}

// Tier templates, ≤ 3 separate per window, the rest as ONE digest; replays are no-ops; history is public data only.
func TestBreakthroughPacingDigestHistory(t *testing.T) {
	svc, _, mk, now := setup(t)
	ctx := context.Background()
	q := db.New(svc.Pool)
	names := []string{"Một", "Hai", "Ba", "Bốn", "Năm", "Sáu"}
	for i, n := range names {
		c := mk("Đạo Hữu " + n)
		if err := svc.Breakthrough(ctx, q, int64(100+i), event(t, c, "Đạo Hữu "+n, "luyen_hu", true, false)); err != nil {
			t.Fatal(err)
		}
	}
	// a replayed drain of an already announced event: no second row, no second publish
	if err := svc.Breakthrough(ctx, q, 100, event(t, uuid.New(), "Đạo Hữu Một", "luyen_hu", true, false)); err != nil {
		t.Fatal(err)
	}
	// first-to-realm is never folded, even when the window is full
	first := mk("Người Đầu Tiên")
	if err := svc.Breakthrough(ctx, q, 200, event(t, first, "Người Đầu Tiên", "kim_tien", true, true)); err != nil {
		t.Fatal(err)
	}
	*now = now.Add(15 * time.Second) // the window closed
	if err := svc.FlushDigests(ctx); err != nil {
		t.Fatal(err)
	}
	items, err := svc.Recent(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]int{}
	for _, it := range items {
		kinds[it.Kind]++
		if n := len([]rune(it.Text)); n == 0 || n > 90 {
			t.Errorf("%s: %d chars", it.Kind, n)
		}
		if strings.Contains(it.Text, "@") {
			t.Errorf("personal data in %q", it.Text)
		}
	}
	if kinds["tribulation"] != 3 || kinds["digest"] != 1 || kinds["first_to_realm"] != 1 || len(items) != 5 {
		t.Fatalf("kinds = %v (%d items)", kinds, len(items))
	}
	for _, it := range items {
		if it.Kind == "digest" && (it.Count != 2 || !strings.Contains(it.Text, "2")) {
			t.Fatalf("digest = %+v", it)
		}
		if it.Kind == "tribulation" && it.Tier != 4 {
			t.Fatalf("tier = %d", it.Tier)
		}
	}
	// published: 3 separate + first + digest (the replay published nothing), each with its WS type
	envs := published[svc].envs
	types := map[string]int{}
	for _, e := range envs {
		types[e.Type]++
	}
	if len(envs) != 5 || types["announcement.major_breakthrough"] != 3 || types["announcement.first_to_realm"] != 1 || types["announcement.digest"] != 1 {
		t.Fatalf("published %v", types)
	}
}

// Rank news: entering the top 10 (already ranked) or reaching rank 1, at most once per player per cooldown.
func TestRankNewsCooldown(t *testing.T) {
	svc, _, mk, _ := setup(t)
	ctx := context.Background()
	q := db.New(svc.Pool)
	c := mk("Lên Bảng")
	moves := []announce.RankMove{{CharacterID: c, Name: "Lên Bảng", Old: 14, New: 9}}
	if err := svc.Rank(ctx, q, moves); err != nil {
		t.Fatal(err)
	}
	if err := svc.Rank(ctx, q, []announce.RankMove{{CharacterID: c, Name: "Lên Bảng", Old: 9, New: 1}}); err != nil {
		t.Fatal(err)
	}
	newcomer := mk("Mới Vào")
	if err := svc.Rank(ctx, q, []announce.RankMove{{CharacterID: newcomer, Name: "Mới Vào", Old: 0, New: 2}}); err != nil {
		t.Fatal(err)
	}
	items, _ := svc.Recent(ctx, 0)
	if len(items) != 1 || items[0].Kind != "rank_top10" || items[0].Rank != 9 {
		t.Fatalf("items = %+v", items)
	}
}
