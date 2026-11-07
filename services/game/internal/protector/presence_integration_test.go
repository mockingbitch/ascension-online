//go:build integration

package protector_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"tiendao/game/internal/platform/redisx"
	"tiendao/game/internal/protector"
)

// M2: a ping alone keeps nobody online; it counts only after a real game action within the activity window.
func TestPresenceNeedsRecentActivity(t *testing.T) {
	ctx := context.Background()
	url := os.Getenv("TEST_REDIS_URL")
	if url == "" {
		url = "redis://localhost:2379/15"
	}
	rdb, err := redisx.Connect(ctx, url)
	if err != nil {
		t.Fatalf("%v (run `make db-up`)", err)
	}
	defer rdb.Close()
	p := &protector.RedisPresence{R: rdb, TTL: 2 * time.Second, Activity: 3 * time.Second}
	user, char := uuid.New(), uuid.New()
	now := time.Now()
	if ok, err := p.Ping(ctx, char, user, "f", now); err != nil || ok {
		t.Fatalf("ping without activity: %v %v", ok, err)
	}
	if on, _ := p.Online(ctx, []uuid.UUID{char}, now); on[char] {
		t.Fatal("online without activity")
	}
	if err := p.MarkActive(ctx, user); err != nil {
		t.Fatal(err)
	}
	if ok, err := p.Ping(ctx, char, user, "f", now); err != nil || !ok {
		t.Fatalf("ping after activity: %v %v", ok, err)
	}
	if on, _ := p.Online(ctx, []uuid.UUID{char}, now); !on[char] {
		t.Fatal("should be online")
	}
	if ids, _ := p.FactionOnline(ctx, "f", now, 10); len(ids) == 0 {
		t.Fatal("faction presence")
	}
	time.Sleep(3200 * time.Millisecond) // activity and presence expired
	if ok, _ := p.Ping(ctx, char, user, "f", time.Now()); ok {
		t.Fatal("stale activity still accepted a ping")
	}
	if on, _ := p.Online(ctx, []uuid.UUID{char}, time.Now()); on[char] {
		t.Fatal("presence did not expire")
	}
}
