//go:build integration

package leaderboard_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"tiendao/game/internal/content"
	"tiendao/game/internal/leaderboard"
	"tiendao/game/internal/platform/redisx"
	"tiendao/game/internal/platform/testdb"
)

func TestRebuildPageRefresh(t *testing.T) {
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
	key := "test:leaderboard:" + uuid.NewString()
	t.Cleanup(func() { rdb.Del(context.Background(), key); rdb.Close() })
	svc := &leaderboard.Service{Pool: pool, Redis: rdb, Realms: cb.Realms, Key: key}

	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	mk := func(name, stage string, cult int64, reached time.Time) uuid.UUID {
		u, c := uuid.New(), uuid.New()
		for _, q := range []struct {
			sql  string
			args []any
		}{
			{"INSERT INTO users (id, email) VALUES ($1, $2)", []any{u, u.String() + "@t"}},
			{"INSERT INTO characters (id, user_id, name) VALUES ($1, $2, $3)", []any{c, u, name}},
			{"INSERT INTO character_progressions (character_id, realm_id, stage_id, cultivation_milli, stage_reached_at) VALUES ($1, split_part($2,'.',1), $2, $3, $4)", []any{c, stage, cult, reached}},
		} {
			if _, err := pool.Exec(ctx, q.sql, q.args...); err != nil {
				t.Fatal(err)
			}
		}
		return c
	}
	low := mk("Hạ Vân", "luyen_khi.tang_3", 0, t0)
	early := mk("Sớm Đạt", "truc_co.so_ky", 0, t0) // tie on score, reached first
	late := mk("Muộn Đạt", "truc_co.so_ky", 0, t0.Add(time.Hour))
	top := mk("Đệ Nhất" /* Vu path: score is path-agnostic */, "vu_3.so_ky", 10_000, t0)

	// Equipped frame/title ride along on the page (display only; the order below proves the score ignores them).
	if _, err := pool.Exec(ctx, "INSERT INTO character_appearance (character_id, avatar_frame_id, title_id) VALUES ($1, 'avatar_frame:xich_ha', 'title:thai_hu')", low); err != nil {
		t.Fatal(err)
	}
	if n, err := svc.Rebuild(ctx); err != nil || n != 4 {
		t.Fatalf("rebuild %d %v", n, err)
	}
	page, total, err := svc.Page(ctx, 0, 10)
	if err != nil || total != 4 {
		t.Fatalf("page %v total %d", err, total)
	}
	want := []uuid.UUID{top, early, late, low}
	for i, e := range page {
		if e.CharacterID != want[i] || e.Rank != int64(i+1) {
			t.Fatalf("rank %d = %s (%s), want %s", i+1, e.Name, e.CharacterID, want[i])
		}
	}
	if p := page[3]; p.AvatarFrameID != "avatar_frame:xich_ha" || p.TitleID != "title:thai_hu" || page[0].AvatarFrameID != "" {
		t.Fatalf("appearance on page: %+v", page)
	}
	// Low player jumps to Hóa Thần → Refresh re-reads Postgres.
	if _, err := pool.Exec(ctx, "UPDATE character_progressions SET realm_id='hoa_than', stage_id='hoa_than.so_ky_1' WHERE character_id=$1", low); err != nil {
		t.Fatal(err)
	}
	if err := svc.Refresh(ctx, []uuid.UUID{low}); err != nil {
		t.Fatal(err)
	}
	if r, _, _ := svc.RankOf(ctx, low); r != 1 {
		t.Fatalf("rank after refresh = %d", r)
	}
}
