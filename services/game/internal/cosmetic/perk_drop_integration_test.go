//go:build integration

package cosmetic_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"tiendao/game/internal/character"
	"tiendao/game/internal/chat"
	"tiendao/game/internal/content"
	"tiendao/game/internal/cosmetic"
	"tiendao/game/internal/cultivation"
	"tiendao/game/internal/leaderboard"
	"tiendao/game/internal/platform/redisx"
	"tiendao/game/internal/platform/testdb"
	"tiendao/game/internal/vip"
)

// H1: a Tiên Duyên perk worn at TD12 disappears everywhere others see it once a refund drops the level: vip strips it
// from character_appearance in the refund tx, and every read filters by the current level (defence in depth).
func TestPerkClearedWhenLevelDrops(t *testing.T) {
	ctx := context.Background()
	pool := testdb.New(t)
	cb, err := content.LoadAll("../../../../content/catalog")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 7, 3, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	user, char, admin := uuid.New(), uuid.New(), uuid.New()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{"INSERT INTO users (id, email) VALUES ($1, $2), ($3, $4)", []any{user, user.String() + "@t", admin, admin.String() + "@t"}},
		{"INSERT INTO characters (id, user_id, name) VALUES ($1, $2, 'Chí Tôn Thử')", []any{char, user}},
		{"INSERT INTO character_progressions (character_id, realm_id, stage_id, last_cultivated_at) VALUES ($1, 'luyen_khi', 'luyen_khi.tang_3', $2)", []any{char, now}},
		{"INSERT INTO currency_accounts (character_id, currency, balance) VALUES ($1, 'spirit_stone', 10000)", []any{char}},
	} {
		if _, err := pool.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatal(err)
		}
	}
	vipSvc := &vip.Service{Pool: pool, Table: cb.VIP, Now: clock}
	cos := &cosmetic.Service{Pool: pool, Cat: cb.Cosmetics, VIP: cb.VIP, Eras: cb.World, Now: clock}
	cult := &cultivation.Service{Pool: pool, Realms: cb.Realms, Contribution: cb.Contribution, VIP: cb.VIP, Now: clock}
	chars := &character.Service{Pool: pool, Cultivation: cult, Cosmetics: cb.Cosmetics, VIP: cb.VIP}
	chats := &chat.Service{Pool: pool, Cat: cb.Chat, Realms: cb.Realms, Mod: cb.Moderation, Now: clock, NameFx: cb.Cosmetics.NameFxFilter(cb.VIP.TierForPoints)}

	// Top up to TD12 (500,000 điểm duyên), wear TD12 + TD10 perks and a bought frame.
	if _, err := vipSvc.Topup(ctx, user, 50_000_000, "order-perk-drop-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := cos.Buy(ctx, user, "avatar_frame:truc_moc", "perk-buy-0001"); err != nil {
		t.Fatal(err)
	}
	full := cosmetic.Appearance{TitleID: "title:chi_ton", AvatarFrameID: "avatar_frame:thien", ChatAuraID: "chat_aura:tu_quang",
		CaveDecorationIDs: []string{"cave_decoration:van_hac"}}
	if _, err := cos.Equip(ctx, user, full, "perk-equip-001"); err != nil {
		t.Fatal(err)
	}
	if v, err := chars.Get(ctx, user); err != nil || !v.Appearance.Equal(full) {
		t.Fatalf("before refund: %+v %v", v.Appearance, err)
	}

	// Refund the top-up → level 0 (well, whatever free points remain: none) → every perk stripped in the same tx.
	var ledgerID int64
	if err := pool.QueryRow(ctx, "SELECT id FROM vip_point_ledger WHERE user_id=$1 AND reason='purchase'", user).Scan(&ledgerID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := vipSvc.Refund(ctx, admin, ledgerID, "chargeback", "perk-refund-01"); err != nil {
		t.Fatal(err)
	}
	var stored string
	if err := pool.QueryRow(ctx, "SELECT concat_ws('|', title_id, avatar_frame_id, chat_aura_id, array_to_string(cave_decoration_ids, ',')) FROM character_appearance WHERE character_id=$1", char).Scan(&stored); err != nil || stored != "" {
		t.Fatalf("stored look after refund %q %v", stored, err)
	}
	v, err := chars.Get(ctx, user)
	if err != nil || v.Appearance.TitleID != "" || v.Appearance.AvatarFrameID != "" || v.Appearance.ChatAuraID != "" || len(v.Appearance.CaveDecorationIDs) != 0 {
		t.Fatalf("GET /character after refund: %+v %v", v.Appearance, err)
	}
	m, err := chats.Send(ctx, user, "world", "sau khi hoàn tiền", "perk-chat-0001")
	if err != nil || m.NameFx != "" {
		t.Fatalf("chat after refund: %+v %v", m, err)
	}

	// Defence in depth: even if a stale perk row survived (simulated), reads hide it.
	if _, err := pool.Exec(ctx, "UPDATE character_appearance SET title_id='title:chi_ton', avatar_frame_id='avatar_frame:thien', chat_aura_id='chat_aura:tu_quang' WHERE character_id=$1", char); err != nil {
		t.Fatal(err)
	}
	if v, _ := chars.Get(ctx, user); v.Appearance.TitleID != "" || v.Appearance.AvatarFrameID != "" {
		t.Fatalf("read filter: %+v", v.Appearance)
	}
	if m, err := chats.Send(ctx, user, "world", "lần nữa", "perk-chat-0002"); err != nil || m.NameFx != "" {
		t.Fatalf("chat filter: %+v %v", m, err)
	}
	url := os.Getenv("TEST_REDIS_URL")
	if url == "" {
		url = "redis://localhost:2379/15"
	}
	rdb, err := redisx.Connect(ctx, url)
	if err != nil {
		t.Fatalf("%v (run `make db-up`)", err)
	}
	key := "test:leaderboard:" + uuid.NewString()
	t.Cleanup(func() { rdb.Del(context.Background(), key); rdb.Close() })
	lb := &leaderboard.Service{Pool: pool, Redis: rdb, Realms: cb.Realms, Key: key, Look: cb.Cosmetics.BoardFilter(cb.VIP.TierForPoints)}
	if err := lb.Refresh(ctx, []uuid.UUID{char}); err != nil {
		t.Fatal(err)
	}
	page, _, err := lb.Page(ctx, 0, 10)
	if err != nil || len(page) != 1 || page[0].TitleID != "" || page[0].AvatarFrameID != "" {
		t.Fatalf("leaderboard after refund: %+v %v", page, err)
	}

	// Equipping other (owned) slots works again; a perk above the level is refused.
	if _, err := cos.Equip(ctx, user, cosmetic.Appearance{AvatarFrameID: "avatar_frame:truc_moc"}, "perk-equip-002"); err != nil {
		t.Fatalf("equip bought frame: %v", err)
	}
	if _, err := cos.Equip(ctx, user, cosmetic.Appearance{TitleID: "title:chi_ton"}, "perk-equip-003"); err == nil {
		t.Fatal("perk above level accepted")
	}
	if v, _ := chars.Get(ctx, user); v.Appearance.AvatarFrameID != "avatar_frame:truc_moc" {
		t.Fatalf("after re-equip: %+v", v.Appearance)
	}
}

// H1 (admin revoke): only the perks above the NEW level are stripped; a perk the level still grants stays.
func TestRevokeStripsOnlyPerksAboveNewLevel(t *testing.T) {
	ctx := context.Background()
	pool := testdb.New(t)
	cb, err := content.LoadAll("../../../../content/catalog")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 7, 3, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	user, char := uuid.New(), uuid.New()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{"INSERT INTO users (id, email) VALUES ($1, $2)", []any{user, user.String() + "@t"}},
		{"INSERT INTO characters (id, user_id, name) VALUES ($1, $2, 'Thu Hồi Thử')", []any{char, user}},
		{"INSERT INTO character_progressions (character_id, realm_id, stage_id, last_cultivated_at) VALUES ($1, 'luyen_khi', 'luyen_khi.tang_1', $2)", []any{char, now}},
	} {
		if _, err := pool.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatal(err)
		}
	}
	vipSvc := &vip.Service{Pool: pool, Table: cb.VIP, Now: clock}
	cos := &cosmetic.Service{Pool: pool, Cat: cb.Cosmetics, VIP: cb.VIP, Eras: cb.World, Now: clock}
	if _, err := vipSvc.Grant(ctx, nil, user, 2500, "admin_grant", "test", "g1", "perk-grant-001"); err != nil { // TD5
		t.Fatal(err)
	}
	look := cosmetic.Appearance{TitleID: "title:vip_5", AvatarFrameID: "avatar_frame:ngoc", ChatAuraID: "chat_aura:linh_quang", CaveDecorationIDs: []string{}}
	if _, err := cos.Equip(ctx, user, look, "perk-equip-101"); err != nil {
		t.Fatal(err)
	}
	if _, err := vipSvc.Grant(ctx, nil, user, -2000, "admin_revoke", "test", "r1", "perk-revoke-01"); err != nil { // 500 → TD3
		t.Fatal(err)
	}
	v, err := cos.View(ctx, user)
	if err != nil || v.Tier != 3 || v.Equipped.TitleID != "" || v.Equipped.ChatAuraID != "" || v.Equipped.AvatarFrameID != "avatar_frame:ngoc" {
		t.Fatalf("after revoke: tier %d %+v %v", v.Tier, v.Equipped, err)
	}
}
