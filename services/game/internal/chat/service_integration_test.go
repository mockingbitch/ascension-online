//go:build integration

package chat_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"tiendao/game/internal/chat"
	"tiendao/game/internal/content"
	"tiendao/game/internal/platform/testdb"
	"tiendao/game/internal/report"
)

type published struct {
	mu     sync.Mutex
	events []string
	topics []string // topic of each event ("" = broadcast)
}

func (p *published) fn(_ context.Context, topic, typ, id string, _ any) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, typ+":"+id)
	p.topics = append(p.topics, topic)
	return nil
}

func TestChatFlow(t *testing.T) {
	ctx := context.Background()
	pool := testdb.New(t)
	cb, err := content.LoadAll("../../../../content/catalog")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 6, 5, 0, 0, 0, time.UTC)
	pub := &published{}
	svc := &chat.Service{Pool: pool, Cat: cb.Chat, Realms: cb.Realms, Mod: cb.Moderation, Now: func() time.Time { return now }, Publish: pub.fn}

	mk := func(name, stage string) (uuid.UUID, uuid.UUID) {
		t.Helper()
		u, ch := uuid.New(), uuid.New()
		for _, q := range []struct {
			sql  string
			args []any
		}{
			{"INSERT INTO users (id, email) VALUES ($1, $2)", []any{u, u.String() + "@t"}},
			{"INSERT INTO characters (id, user_id, name) VALUES ($1, $2, $3)", []any{ch, u, name}},
			{"INSERT INTO character_progressions (character_id, realm_id, stage_id) VALUES ($1, split_part($2,'.',1), $2)", []any{ch, stage}},
		} {
			if _, err := pool.Exec(ctx, q.sql, q.args...); err != nil {
				t.Fatal(err)
			}
		}
		return u, ch
	}
	alice, aliceCh := mk("Lâm Thanh Vân", "luyen_khi.tang_3")
	newbie, _ := mk("Tân Thủ", "luyen_khi.tang_1")
	admin, _ := mk("Quản Sự", "truc_co.so_ky")
	noChar := uuid.New()
	if _, err := pool.Exec(ctx, "INSERT INTO users (id, email) VALUES ($1, 'x@t')", noChar); err != nil {
		t.Fatal(err)
	}

	// Gates.
	var low *chat.StageTooLowError
	if _, err := svc.Send(ctx, newbie, "world", "xin chào", "k-newbie-1"); !errors.As(err, &low) {
		t.Fatalf("stage gate: %v", err)
	}
	if _, err := svc.Send(ctx, noChar, "world", "xin chào", "k-nochar-1"); !errors.Is(err, chat.ErrNoCharacter) {
		t.Fatalf("no character: %v", err)
	}
	if _, err := svc.Send(ctx, alice, "sect", "xin chào", "k-alice-0"); !errors.Is(err, chat.ErrUnknownChannel) {
		t.Fatalf("channel: %v", err)
	}
	if _, err := svc.Send(ctx, alice, "world", " ​ ", "k-alice-00"); !errors.Is(err, chat.ErrInvalidText) {
		t.Fatalf("empty: %v", err)
	}

	// Send + masking + idempotent replay (no second publish).
	m1, err := svc.Send(ctx, alice, "world", "  địt mẹ,   ai đột phá chưa? ", "k-alice-1")
	if err != nil || m1.Text != "***, ai đột phá chưa?" || m1.Name != "Lâm Thanh Vân" || m1.StageID != "luyen_khi.tang_3" {
		t.Fatalf("send %+v %v", m1, err)
	}
	if again, err := svc.Send(ctx, alice, "world", "địt mẹ, ai đột phá chưa?", "k-alice-1"); err != nil || again.ID != m1.ID {
		t.Fatalf("replay %+v %v", again, err)
	}
	if _, err := svc.Send(ctx, alice, "world", "câu khác", "k-alice-1"); !errors.Is(err, chat.ErrKeyReuse) {
		t.Fatalf("key reuse: %v", err)
	}
	var raw *string
	if err := pool.QueryRow(ctx, "SELECT raw_body FROM chat_messages WHERE id=$1", m1.ID).Scan(&raw); err != nil || raw == nil || *raw != "địt mẹ, ai đột phá chưa?" {
		t.Fatalf("raw evidence %v %v", raw, err)
	}

	// Duplicate text within the window, allowed after it.
	if _, err := svc.Send(ctx, alice, "world", "đạo hữu khỏe", "k-alice-2"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Send(ctx, alice, "world", "đạo hữu khỏe", "k-alice-3"); !errors.Is(err, chat.ErrDuplicate) {
		t.Fatalf("dup: %v", err)
	}
	now = now.Add(31 * time.Second)
	m3, err := svc.Send(ctx, alice, "world", "đạo hữu khỏe", "k-alice-4")
	if err != nil {
		t.Fatal(err)
	}

	// History: oldest first, paging by id.
	h, err := svc.History(ctx, "world", 0, 0)
	if err != nil || len(h) != 3 || h[0].ID != m1.ID || h[2].ID != m3.ID {
		t.Fatalf("history %+v %v", h, err)
	}
	if older, _ := svc.History(ctx, "world", 0, m3.ID); len(older) != 2 {
		t.Fatalf("paged %+v", older)
	}

	// Report a chat message: evidence is the original text; message must belong to the target.
	rep := &report.Service{Pool: pool, Cat: cb.Moderation, Now: func() time.Time { return now }}
	r, err := rep.Submit(ctx, newbie, aliceCh, report.ReasonHarassment, "", "rep-key-01", m1.ID)
	if err != nil || r.ChatMessageID == nil || *r.ChatMessageID != m1.ID {
		t.Fatalf("report %+v %v", r, err)
	}
	if list, _ := rep.List(ctx, report.StatusOpen, 10, 0); len(list) != 1 || list[0].Evidence != "địt mẹ, ai đột phá chưa?" {
		t.Fatalf("evidence %+v", list)
	}
	_, other := mk("Người Khác", "luyen_khi.tang_3")
	if _, err := rep.Submit(ctx, newbie, other, report.ReasonHarassment, "", "rep-key-02", m1.ID); !errors.Is(err, report.ErrInvalidReport) {
		t.Fatalf("message/target mismatch: %v", err)
	}

	// Hide (idempotent, one broadcast) and mute.
	if err := svc.Hide(ctx, admin, m1.ID, "tục"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Hide(ctx, admin, m1.ID, "tục"); err != nil {
		t.Fatal(err)
	}
	if h, _ := svc.History(ctx, "world", 0, 0); len(h) != 2 {
		t.Fatalf("hidden still listed: %+v", h)
	}
	if _, err := svc.Mute(ctx, admin, alice, 24, "spam"); err != nil {
		t.Fatal(err)
	}
	var muted *chat.MutedError
	if _, err := svc.Send(ctx, alice, "world", "còn nói được không", "k-alice-5"); !errors.As(err, &muted) || !muted.Until.Equal(now.Add(24*time.Hour)) {
		t.Fatalf("muted: %v", err)
	}
	if _, err := svc.Mute(ctx, admin, alice, 0, "ân xá"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Send(ctx, alice, "world", "cảm tạ", "k-alice-6"); err != nil {
		t.Fatalf("after unmute: %v", err)
	}
	if _, err := svc.Mute(ctx, admin, alice, 100000, ""); !errors.Is(err, chat.ErrInvalidMute) {
		t.Fatalf("mute range: %v", err)
	}
	var audits int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM admin_audit_logs WHERE admin_user_id=$1", admin).Scan(&audits); err != nil || audits != 3 {
		t.Fatalf("audits %d %v (hide + 2 mutes)", audits, err)
	}

	// Publishes: 4 messages sent (m1, m2, m3, after unmute) + 1 hide; replays and the 2nd hide publish nothing.
	if len(pub.events) != 5 {
		t.Fatalf("published %v", pub.events)
	}

	// Retention sweep; the report keeps its evidence after the message is gone.
	now = now.Add(91 * 24 * time.Hour)
	if n, err := svc.Sweep(ctx); err != nil || n != 4 {
		t.Fatalf("sweep %d %v", n, err)
	}
	if list, _ := rep.List(ctx, report.StatusOpen, 10, 0); len(list) != 1 || list[0].Evidence == "" || list[0].ChatMessageID != nil {
		t.Fatalf("report after sweep %+v", list)
	}
}

// The chat aura equipped at send time is snapshotted on the message (others see it in live delivery and history);
// changing the look later does not rewrite old messages.
func TestChatNameEffectSnapshot(t *testing.T) {
	ctx := context.Background()
	pool := testdb.New(t)
	cb, err := content.LoadAll("../../../../content/catalog")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 7, 5, 0, 0, 0, time.UTC)
	svc := &chat.Service{Pool: pool, Cat: cb.Chat, Realms: cb.Realms, Mod: cb.Moderation, Now: func() time.Time { return now }}
	u, ch := uuid.New(), uuid.New()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{"INSERT INTO users (id, email) VALUES ($1, $2)", []any{u, u.String() + "@t"}},
		{"INSERT INTO characters (id, user_id, name) VALUES ($1, $2, 'Mặc Khách')", []any{ch, u}},
		{"INSERT INTO character_progressions (character_id, realm_id, stage_id) VALUES ($1, 'luyen_khi', 'luyen_khi.tang_3')", []any{ch}},
		{"INSERT INTO character_appearance (character_id, chat_aura_id) VALUES ($1, 'chat_aura:mac_huong')", []any{ch}},
	} {
		if _, err := pool.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatal(err)
		}
	}
	m, err := svc.Send(ctx, u, "world", "hương mực", "k-fx-0001")
	if err != nil || m.NameFx != "chat_aura:mac_huong" {
		t.Fatalf("send: %+v %v", m, err)
	}
	if _, err := pool.Exec(ctx, "UPDATE character_appearance SET chat_aura_id = NULL WHERE character_id=$1", ch); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	m2, err := svc.Send(ctx, u, "world", "không hiệu ứng", "k-fx-0002")
	if err != nil || m2.NameFx != "" {
		t.Fatalf("send 2: %+v %v", m2, err)
	}
	hist, err := svc.History(ctx, "world", 0, 0)
	if err != nil || len(hist) != 2 || hist[0].NameFx != "chat_aura:mac_huong" || hist[1].NameFx != "" {
		t.Fatalf("history: %+v %v", hist, err)
	}
}

// Hall chat (decisions 2026-11-10): stored with the sender's điện, delivered on that điện's topic, back-filled per
// điện; posting needs a cushion; world chat is still broadcast.
func TestHallChatPerDien(t *testing.T) {
	ctx := context.Background()
	pool := testdb.New(t)
	cb, err := content.LoadAll("../../../../content/catalog")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 10, 5, 0, 0, 0, time.UTC)
	pub := &published{}
	halls := map[uuid.UUID]int{}
	svc := &chat.Service{Pool: pool, Cat: cb.Chat, Realms: cb.Realms, Mod: cb.Moderation, Now: func() time.Time { return now }, Publish: pub.fn,
		HallChannel: cb.Lectures.ChatChannel,
		HallOf: func(_ context.Context, id uuid.UUID) (int, bool, error) {
			h, ok := halls[id]
			return h, ok, nil
		},
		HallTopic: func(h int) string { return "hall:" + string(rune('0'+h)) },
	}
	mk := func(name string) (uuid.UUID, uuid.UUID) {
		t.Helper()
		u, ch := uuid.New(), uuid.New()
		for _, q := range []string{
			"INSERT INTO users (id, email) VALUES ($1, $2::text || '@t' || md5($3))",
			"INSERT INTO characters (id, user_id, name) VALUES ($2, $1, $3)",
			"INSERT INTO character_progressions (character_id, realm_id, stage_id) SELECT $2, 'luyen_khi', 'luyen_khi.tang_3' WHERE $1::uuid IS NOT NULL AND $3::text IS NOT NULL",
		} {
			if _, err := pool.Exec(ctx, q, u, ch, name); err != nil {
				t.Fatal(q, err)
			}
		}
		return u, ch
	}
	a, aCh := mk("Điện Nhất")
	b, bCh := mk("Điện Nhị")
	c, _ := mk("Ngoài Cung")
	halls[aCh], halls[bCh] = 1, 2
	hc := cb.Lectures.ChatChannel

	if _, err := svc.Send(ctx, c, hc, "cho vào với", "k-c-1"); !errors.Is(err, chat.ErrNotInHall) {
		t.Fatalf("no cushion: %v", err)
	}
	m1, err := svc.Send(ctx, a, hc, "điện một", "k-a-1")
	if err != nil || m1.HallID != 1 {
		t.Fatalf("a: %+v %v", m1, err)
	}
	m2, err := svc.Send(ctx, b, hc, "điện hai", "k-b-1")
	if err != nil || m2.HallID != 2 {
		t.Fatalf("b: %+v %v", m2, err)
	}
	if _, err := svc.Send(ctx, a, "world", "thế giới", "k-a-2"); err != nil {
		t.Fatal(err)
	}
	if got := pub.topics; len(got) != 3 || got[0] != "hall:1" || got[1] != "hall:2" || got[2] != "" {
		t.Fatalf("topics %v", got)
	}
	// A replay after leaving the hall still answers with the stored message (and does not re-publish).
	delete(halls, aCh)
	if again, err := svc.Send(ctx, a, hc, "điện một", "k-a-1"); err != nil || again.ID != m1.ID {
		t.Fatalf("replay: %+v %v", again, err)
	}
	if len(pub.topics) != 3 {
		t.Fatal("replay re-published")
	}
	h1, err := svc.History(ctx, hc, 1, 0)
	if err != nil || len(h1) != 1 || h1[0].ID != m1.ID {
		t.Fatalf("history 1: %+v %v", h1, err)
	}
	if h2, _ := svc.History(ctx, hc, 2, 0); len(h2) != 1 || h2[0].ID != m2.ID {
		t.Fatalf("history 2: %+v", h2)
	}
	if _, err := svc.History(ctx, hc, 0, 0); !errors.Is(err, chat.ErrHallRequired) {
		t.Fatalf("hall required: %v", err)
	}
	// Hiding a hall message is broadcast (a client may show another điện's backfill).
	adm, _ := mk("Quản Sự")
	if err := svc.Hide(ctx, adm, m2.ID, "test"); err != nil {
		t.Fatal(err)
	}
	if last := pub.topics[len(pub.topics)-1]; last != "" {
		t.Fatalf("hide topic %q", last)
	}
}
