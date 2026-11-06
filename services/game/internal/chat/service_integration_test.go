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
}

func (p *published) fn(_ context.Context, typ, id string, _ any) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, typ+":"+id)
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
	h, err := svc.History(ctx, "world", 0)
	if err != nil || len(h) != 3 || h[0].ID != m1.ID || h[2].ID != m3.ID {
		t.Fatalf("history %+v %v", h, err)
	}
	if older, _ := svc.History(ctx, "world", m3.ID); len(older) != 2 {
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
	if h, _ := svc.History(ctx, "world", 0); len(h) != 2 {
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
