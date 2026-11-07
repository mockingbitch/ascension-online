//go:build integration

package cultivation_test

import (
	"errors"
	"testing"
	"time"

	"tiendao/game/internal/cultivation"
)

func (e *env) pills(t *testing.T) int64 {
	return e.scalar(t, "SELECT COALESCE((SELECT quantity FROM inventory_items WHERE character_id=$1 AND item_id='tu_linh_dan'),0)", e.char)
}

func (e *env) gameDay() time.Time {
	ict := e.clock.In(time.FixedZone("ICT", 7*3600))
	return time.Date(ict.Year(), ict.Month(), ict.Day(), 0, 0, 0, 0, time.UTC)
}

// Tụ Linh Đan (§4.5): +5 points per pill through CreditLocked(source item), max 2 uses per game day, idempotent per key,
// clipped by the speed cap / activity bucket / 150 ceiling; a pill that would credit nothing is kept.
func TestSpeedItemLimitsAndIdempotency(t *testing.T) {
	e := setup(t)
	e.exec(t, "INSERT INTO inventory_items (character_id, item_id, quantity) VALUES ($1, 'tu_linh_dan', 6)", e.char)
	use := func(key string) (cultivation.SpeedItemResult, error) {
		return e.svc.UseSpeedItem(e.ctx, e.cb.Items, e.user, "tu_linh_dan", key)
	}
	r, err := use("pill-key-0001")
	if err != nil {
		t.Fatal(err)
	}
	if r.EarnedMilli != 5000 || r.UsesToday != 1 || r.MaxUsesPerDay != 2 || r.Remaining != 5 || r.Cultivation.Today.SpeedItems.Milli != 5000 {
		t.Fatalf("first pill: %+v", r)
	}
	again, err := use("pill-key-0001")
	if err != nil || !again.Replayed || again.EarnedMilli != 5000 || again.Remaining != 5 || again.UsesToday != 1 {
		t.Fatalf("replay: %+v %v", again, err)
	}
	if _, err := use("pill-key-0002"); err != nil {
		t.Fatal(err)
	}
	if _, err := use("pill-key-0003"); !errors.Is(err, cultivation.ErrItemDailyLimit) {
		t.Fatalf("third pill: %v", err)
	}
	if e.pills(t) != 4 {
		t.Fatalf("pills %d (the refused third pill must be kept)", e.pills(t))
	}
	e.checkC3(t)
	if got := e.scalar(t, "SELECT speed_item_milli FROM daily_contribution WHERE character_id=$1", e.char); got != 10_000 {
		t.Fatalf("speed_item_milli %d", got)
	}

	// Next day: activity bucket almost full → not even 1 milli short is accepted (all or nothing, M1).
	*e.clock = e.clock.Add(24 * time.Hour)
	e.exec(t, "UPDATE character_progressions SET last_cultivated_at=$2 WHERE character_id=$1", e.char, *e.clock)
	e.exec(t, "INSERT INTO daily_contribution (character_id, game_day, activity_milli) VALUES ($1, $2, 44000)", e.char, e.gameDay())
	kept := e.pills(t)
	if _, err := use("pill-key-0004"); !errors.Is(err, cultivation.ErrNoRoom) {
		t.Fatalf("4/5 room: %v", err)
	}
	e.exec(t, "UPDATE daily_contribution SET activity_milli=40001 WHERE character_id=$1 AND game_day=$2", e.char, e.gameDay())
	if _, err := use("pill-key-0004b"); !errors.Is(err, cultivation.ErrNoRoom) {
		t.Fatalf("1 milli short: %v", err)
	}
	if e.pills(t) != kept || e.scalar(t, "SELECT COUNT(*) FROM item_daily_uses WHERE character_id=$1 AND game_day=$2", e.char, e.gameDay()) != 0 {
		t.Fatal("refused pill was consumed")
	}
	e.exec(t, "UPDATE daily_contribution SET activity_milli=40000 WHERE character_id=$1 AND game_day=$2", e.char, e.gameDay())
	if r, err = use("pill-key-0004c"); err != nil || r.EarnedMilli != 5000 || r.Cultivation.Today.Buckets["activity"].Milli != 45000 {
		t.Fatalf("exact fit: %+v %v", r, err)
	}

	// Day 3: the 150 ceiling is reached → nothing to credit → refused, pill and daily use kept.
	*e.clock = e.clock.Add(24 * time.Hour)
	e.exec(t, "UPDATE character_progressions SET last_cultivated_at=$2 WHERE character_id=$1", e.char, *e.clock)
	e.exec(t, "INSERT INTO daily_contribution (character_id, game_day, idle_milli, activity_milli, community_milli) VALUES ($1, $2, 75000, 45000, 30000)", e.char, e.gameDay())
	before := e.pills(t)
	if _, err := use("pill-key-0005"); !errors.Is(err, cultivation.ErrNoRoom) {
		t.Fatalf("at ceiling: %v", err)
	}
	if e.pills(t) != before || e.scalar(t, "SELECT COUNT(*) FROM item_daily_uses WHERE character_id=$1 AND game_day=$2", e.char, e.gameDay()) != 0 {
		t.Fatal("refused pill was consumed")
	}
	if got := e.scalar(t, "SELECT points_milli FROM daily_contribution WHERE character_id=$1 AND game_day=$2", e.char, e.gameDay()); got != 150_000 {
		t.Fatalf("ceiling broken: %d", got)
	}

	// Day 4: speed-item cap 1 milli short of a pill (gifted pills of another kind used 5001) → refused.
	*e.clock = e.clock.Add(24 * time.Hour)
	e.exec(t, "UPDATE character_progressions SET last_cultivated_at=$2 WHERE character_id=$1", e.char, *e.clock)
	e.exec(t, "INSERT INTO daily_contribution (character_id, game_day, activity_milli, speed_item_milli) VALUES ($1, $2, 5001, 5001)", e.char, e.gameDay())
	if _, err := use("pill-key-0006"); !errors.Is(err, cultivation.ErrNoRoom) {
		t.Fatalf("speed cap 1 milli short: %v", err)
	}

	// Day 5: bình cảnh room 1 milli short of a pill (last Luyện Khí stage) → ErrBottleneckFull, pill kept.
	*e.clock = e.clock.Add(24 * time.Hour)
	last := e.cb.Realms.First(e.cb.Realms.DefaultPath())
	for n := e.cb.Realms.Next(last); n != nil && n.Tier == 1; n = e.cb.Realms.Next(n) {
		last = n
	}
	room := e.cb.Realms.BottleneckRoomMilli(last, 0)
	e.exec(t, "UPDATE character_progressions SET stage_id=$2, cultivation_milli=$3, last_cultivated_at=$4 WHERE character_id=$1",
		e.char, last.ID, room-4999, *e.clock)
	kept = e.pills(t)
	if _, err := use("pill-key-0007"); !errors.Is(err, cultivation.ErrBottleneckFull) {
		t.Fatalf("bình cảnh 1 milli short: %v", err)
	}
	if e.pills(t) != kept {
		t.Fatal("pill consumed at bình cảnh")
	}
}
