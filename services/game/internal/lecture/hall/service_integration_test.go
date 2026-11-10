//go:build integration

package hall_test

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"tiendao/game/internal/content"
	"tiendao/game/internal/cultivation"
	"tiendao/game/internal/dailytask"
	"tiendao/game/internal/lecture/hall"
	"tiendao/game/internal/platform/redisx"
	"tiendao/game/internal/platform/testdb"
	"tiendao/game/internal/vip"
)

type published struct {
	mu     sync.Mutex
	events []string
	topics []string // "<topic>|<type>:<id>"
}

func (p *published) fn(_ context.Context, topic, typ, id string, _ any) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, typ+":"+id)
	p.topics = append(p.topics, topic+"|"+typ+":"+id)
	return nil
}

func (p *published) hasTopic(e string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, x := range p.topics {
		if x == e {
			return true
		}
	}
	return false
}

func (p *published) has(e string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, x := range p.events {
		if x == e {
			return true
		}
	}
	return false
}

func TestLectureFlow(t *testing.T) {
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
	clean := func() {
		keys, _ := rdb.Keys(ctx, "lecture:*").Result()
		if len(keys) > 0 {
			rdb.Del(ctx, keys...)
		}
	}
	clean()
	t.Cleanup(func() { clean(); rdb.Close() })

	cb, err := content.LoadAll("../../../../../content/catalog")
	if err != nil {
		t.Fatal(err)
	}
	cfg := cb.Lectures
	// Start 1 minute into a slot's closed period: slot 100 opened 3 minutes ago… set explicitly below.
	slotStart := cfg.Epoch.Add(100 * cfg.InsightEvery)
	now := slotStart.Add(cfg.InsightWindow + time.Minute) // window closed
	clock := func() time.Time { return now }
	cult := &cultivation.Service{Pool: pool, Realms: cb.Realms, Contribution: cb.Contribution, VIP: cb.VIP, Tasks: cb.DailyTasks, Now: clock}
	pub := &published{}
	svc := &hall.Service{Pool: pool, Hall: &hall.Hall{R: rdb, Cfg: cfg}, Cfg: cfg, Cultivation: cult, VIP: cb.VIP, Now: clock, Publish: pub.fn}

	mk := func(name string) (uuid.UUID, uuid.UUID) {
		t.Helper()
		u, ch := uuid.New(), uuid.New()
		for _, q := range []struct {
			sql  string
			args []any
		}{
			{"INSERT INTO users (id, email) VALUES ($1, $2)", []any{u, u.String() + "@t"}},
			{"INSERT INTO characters (id, user_id, name) VALUES ($1, $2, $3)", []any{ch, u, name}},
			{"INSERT INTO character_progressions (character_id, realm_id, stage_id, last_cultivated_at, stage_reached_at) VALUES ($1, 'luyen_khi', 'luyen_khi.tang_1', $2, $2)", []any{ch, now}},
		} {
			if _, err := pool.Exec(ctx, q.sql, q.args...); err != nil {
				t.Fatal(err)
			}
		}
		return u, ch
	}
	alice, aliceCh := mk("Lâm Thanh Vân")
	bob, bobCh := mk("Mộ Dung Tuyết")

	// Enter: lowest free seats, idempotent, announced once.
	st, err := svc.Enter(ctx, alice)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Me.InHall || st.Me.Seat != 0 || st.Me.HallID != 1 || st.GlobalListeners != 1 || st.HallID != 1 || st.Count != 1 || st.Capacity != cfg.Seats {
		t.Fatalf("alice enter: %+v", st.Me)
	}
	if st, _ = svc.Enter(ctx, alice); st.Me.Seat != 0 {
		t.Fatalf("re-enter moved seat: %+v", st.Me)
	}
	if st, err = svc.Enter(ctx, bob); err != nil || st.Me.Seat != 1 || len(st.Seated) != 2 {
		t.Fatalf("bob enter: %+v %v", st, err)
	}
	if !pub.hasTopic("hall:1|lecture.enter:" + aliceCh.String()) {
		t.Fatal("enter not published on the điện topic")
	}

	// Ngộ đạo outside the window is refused.
	if _, err := svc.Insight(ctx, alice); !errors.Is(err, hall.ErrInsightClosed) {
		t.Fatalf("closed window: %v", err)
	}

	// Heartbeats every 30 s until the next slot opens: ~7 minutes listened.
	// Carol arrives 2 minutes before the moment: she may take it, but it is not "Nghe giảng" (attend_min_seconds).
	carol, carolCh := mk("Tô Mộc Lan")
	next := cfg.NextSlot(now)
	carolIn := false
	for now.Before(next.OpensAt) {
		now = now.Add(30 * time.Second)
		if _, err := svc.Beat(ctx, alice); err != nil {
			t.Fatal(err)
		}
		if !carolIn && !now.Before(next.OpensAt.Add(-2*time.Minute)) {
			if _, err := svc.Enter(ctx, carol); err != nil {
				t.Fatal(err)
			}
			carolIn = true
		} else if carolIn {
			if _, err := svc.Beat(ctx, carol); err != nil {
				t.Fatal(err)
			}
		}
	}
	// A long gap (tab asleep) counts nothing and does not sweep while within this test's flow: bob never beats,
	// so the sweep drops him.
	if _, err := svc.Beat(ctx, bob); !errors.Is(err, hall.ErrNotPresent) {
		t.Fatalf("bob should be swept: %v", err)
	}
	if !pub.hasTopic("hall:1|lecture.leave:" + bobCh.String()) {
		t.Fatal("sweep not published")
	}

	res, err := svc.Insight(ctx, alice)
	if err != nil {
		t.Fatal(err)
	}
	wantSecs := int64(next.OpensAt.Sub(slotStart.Add(cfg.InsightWindow+time.Minute)) / time.Second)
	if res.PresenceSeconds != wantSecs || res.RawMilli != cfg.Pay(wantSecs, 0) || res.EarnedMilli != res.RawMilli || res.Replayed {
		t.Fatalf("insight: %+v want secs %d", res, wantSecs)
	}
	// Same slot again: stored result, nothing paid twice.
	again, err := svc.Insight(ctx, alice)
	if err != nil || !again.Replayed || again.RawMilli != res.RawMilli {
		t.Fatalf("replay: %+v %v", again, err)
	}
	var credits int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM cultivation_credits WHERE character_id = $1 AND source = 'lecture'", aliceCh).Scan(&credits); err != nil || credits != 1 {
		t.Fatalf("credits %d %v", credits, err)
	}
	// Daily task "Nghe giảng" (lecture_attend) completed by the paid ngộ đạo, in the same transaction; the replay
	// did not count twice (progress is capped at the target anyway, so check the row exists with progress 1).
	var ngheGiang int
	if err := pool.QueryRow(ctx, "SELECT progress FROM daily_task_progress WHERE character_id = $1 AND task_id = 'nghe_giang'", aliceCh).Scan(&ngheGiang); err != nil || ngheGiang != 1 {
		t.Fatalf("nghe_giang progress %d %v", ngheGiang, err)
	}
	if _, err := (&dailytask.Service{Pool: pool, Cat: cb.DailyTasks, VIP: cb.VIP, Now: clock}).Claim(ctx, alice, "nghe_giang"); err != nil {
		t.Fatalf("claim nghe_giang: %v", err)
	}
	if cr, err := svc.Insight(ctx, carol); err != nil || cr.PresenceSeconds >= cfg.AttendMinSeconds || cr.RawMilli == 0 {
		t.Fatalf("carol insight: %+v %v", cr, err)
	}
	var carolTask int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM daily_task_progress WHERE character_id = $1 AND task_id = 'nghe_giang'", carolCh).Scan(&carolTask); err != nil || carolTask != 0 {
		t.Fatalf("short listen counted as Nghe giảng: %d %v", carolTask, err)
	}

	// Nothing accrued yet in the next slot → refused, not a zero row.
	now = cfg.NextSlot(now).OpensAt
	if _, err := svc.Beat(ctx, alice); err == nil { // gap > max gap: present again but nothing accrued
		if _, err := svc.Insight(ctx, alice); !errors.Is(err, hall.ErrNothingToPay) && !errors.Is(err, hall.ErrNotPresent) {
			t.Fatalf("nothing to pay: %v", err)
		}
	}

	// Listen through many slots: the daily cap holds and both điểm duyên milestones pay once.
	if _, err := svc.Enter(ctx, alice); err != nil {
		t.Fatal(err)
	}
	for range 20 {
		slot := cfg.NextSlot(now)
		for now.Before(slot.OpensAt) {
			now = now.Add(30 * time.Second)
			if _, err := svc.Beat(ctx, alice); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := svc.Insight(ctx, alice); err != nil {
			t.Fatal(err)
		}
	}
	st, err = svc.State(ctx, alice)
	if err != nil {
		t.Fatal(err)
	}
	if st.Today.Milli > cfg.DailyCapMilli {
		t.Fatalf("daily cap broken: %d", st.Today.Milli)
	}
	var vipPts int64
	if err := pool.QueryRow(ctx, "SELECT COALESCE(SUM(delta),0) FROM vip_point_ledger WHERE user_id = $1 AND reason = 'activity' AND ref_type = 'lecture_insight'", alice).Scan(&vipPts); err != nil {
		t.Fatal(err)
	}
	act, _ := cb.VIP.Activity(vip.ActLecture)
	if want := act.Points * int64(len(cfg.VIPMilestonesMilli)); st.Today.Milli == cfg.DailyCapMilli && vipPts != want {
		t.Fatalf("điểm duyên %d, want %d", vipPts, want)
	}

	// Leave frees the seat.
	if err := svc.Leave(ctx, alice); err != nil {
		t.Fatal(err)
	}
	if st, _ = svc.State(ctx, alice); st.Me.InHall || st.GlobalListeners != 0 || len(st.Halls) != 0 {
		t.Fatalf("after leave: %+v", st)
	}
}
