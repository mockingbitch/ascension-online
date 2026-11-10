//go:build integration

package realtime

import (
	"context"
	"log/slog"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"tiendao/game/internal/platform/redisx"
)

// Through Redis: PublishTopic on one "instance" reaches only the followers of that topic on another.
func TestRedisTopicFanout(t *testing.T) {
	url := os.Getenv("TEST_REDIS_URL")
	if url == "" {
		url = "redis://localhost:2379/15"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rdb, err := redisx.Connect(ctx, url)
	if err != nil {
		t.Fatalf("%v (run `make db-up`)", err)
	}
	defer rdb.Close()
	h := NewHub(slog.New(slog.DiscardHandler), []string{"*"}, 10)
	h.TopicOK = func(s string) bool { return strings.HasPrefix(s, "hall:") }
	go h.Subscribe(ctx, rdb)
	srv := httptest.NewServer(h)
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	// Pub/sub is not scoped by Redis db: use topic names no real điện has, and never the broadcast channel (a dev
	// API on the same Redis would relay it to real sockets).
	t1, t2 := "hall:test-"+strconv.FormatInt(time.Now().UnixNano(), 36)+"-1", "hall:test-"+strconv.FormatInt(time.Now().UnixNano(), 36)+"-2"
	one := dialSub(t, ctx, h, wsURL, t1)
	defer one.CloseNow()
	two := dialSub(t, ctx, h, wsURL, t2)
	defer two.CloseNow()
	time.Sleep(200 * time.Millisecond) // PSUBSCRIBE settles
	if err := PublishTopic(ctx, rdb, t2, Envelope{Type: "lecture.enter", ID: "x"}); err != nil {
		t.Fatal(err)
	}
	if err := PublishTopic(ctx, rdb, t1, Envelope{Type: "chat.message", ID: "w"}); err != nil {
		t.Fatal(err)
	}
	_, b, err := one.Read(ctx)
	if err != nil || !strings.Contains(string(b), `"chat.message"`) {
		t.Fatalf("điện 1 got %q %v (must skip the điện 2 event)", b, err)
	}
	_, b, err = two.Read(ctx)
	if err != nil || !strings.Contains(string(b), `"lecture.enter"`) {
		t.Fatalf("điện 2 got %q %v", b, err)
	}
}
