package realtime

import (
	"context"
	"log/slog"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestHubBroadcast(t *testing.T) {
	h := NewHub(slog.New(slog.DiscardHandler), []string{"*"}, 10)
	srv := httptest.NewServer(h)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	for h.Count() == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	h.Broadcast([]byte(`{"type":"x"}`))
	_, msg, err := conn.Read(ctx)
	if err != nil || string(msg) != `{"type":"x"}` {
		t.Fatalf("got %q %v", msg, err)
	}
}

func TestShippedTemplates(t *testing.T) {
	tpl, err := LoadTemplates(filepath.Join("..", "..", "..", "..", "content", "story", "announcements", "major-breakthrough.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	s := tpl.Render("major_breakthrough", map[string]string{"name": "Lâm Phong", "realm": "Trúc Cơ"})
	if !strings.Contains(s, "Lâm Phong") || !strings.Contains(s, "Trúc Cơ") || strings.Contains(s, "{") {
		t.Fatalf("render: %q", s)
	}
	if n := len([]rune(s)); n > 90 {
		t.Fatalf("announcement too long: %d", n)
	}
}

// Server WriteTimeout must not kill long-lived sockets.
func TestSurvivesServerWriteTimeout(t *testing.T) {
	h := NewHub(slog.New(slog.DiscardHandler), []string{"*"}, 10)
	srv := httptest.NewUnstartedServer(h)
	srv.Config.WriteTimeout = 200 * time.Millisecond
	srv.Config.ReadTimeout = 200 * time.Millisecond
	srv.Start()
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	time.Sleep(500 * time.Millisecond)
	h.Broadcast([]byte(`late`))
	if _, msg, err := conn.Read(ctx); err != nil || string(msg) != "late" {
		t.Fatalf("socket died after server timeout: %q %v", msg, err)
	}
}

// dialSub connects and follows topic ("" = none), then waits until the hub has applied it.
func dialSub(t *testing.T, ctx context.Context, h *Hub, url, topic string) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	if topic != "" {
		if err := conn.Write(ctx, websocket.MessageText, []byte(`{"op":"sub","topic":"`+topic+`"}`)); err != nil {
			t.Fatal(err)
		}
		for !slices.Contains(h.LocalTopics(""), topic) {
			time.Sleep(5 * time.Millisecond)
		}
	}
	return conn
}

// Per-điện fan-out (decisions 2026-11-10): a socket in điện 1 never gets điện 2 events; broadcasts reach everyone.
func TestTopicFanout(t *testing.T) {
	h := NewHub(slog.New(slog.DiscardHandler), []string{"*"}, 10)
	h.TopicOK = func(s string) bool { return strings.HasPrefix(s, "hall:") && len(s) < 12 }
	srv := httptest.NewServer(h)
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	one := dialSub(t, ctx, h, url, "hall:1")
	defer one.CloseNow()
	two := dialSub(t, ctx, h, url, "hall:2")
	defer two.CloseNow()
	none := dialSub(t, ctx, h, url, "")
	defer none.CloseNow()

	// an invalid topic is ignored (the socket keeps its old one)
	if err := one.Write(ctx, websocket.MessageText, []byte(`{"op":"sub","topic":"world:9"}`)); err != nil {
		t.Fatal(err)
	}
	h.BroadcastTopic("hall:2", []byte(`two`))
	h.BroadcastTopic("hall:1", []byte(`one`))
	h.Broadcast([]byte(`all`))
	read := func(c *websocket.Conn) string {
		_, b, err := c.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	if a, b := read(one), read(one); a != "one" || b != "all" {
		t.Fatalf("điện 1 got %q %q", a, b)
	}
	if a, b := read(two), read(two); a != "two" || b != "all" {
		t.Fatalf("điện 2 got %q %q", a, b)
	}
	if a := read(none); a != "all" {
		t.Fatalf("no topic got %q", a)
	}
	got := h.LocalTopics("hall:")
	slices.Sort(got)
	if !slices.Equal(got, []string{"hall:1", "hall:2"}) {
		t.Fatalf("topics %v", got)
	}
	// switching topic moves the socket; unsub leaves only broadcasts
	if err := one.Write(ctx, websocket.MessageText, []byte(`{"op":"sub","topic":"hall:2"}`)); err != nil {
		t.Fatal(err)
	}
	for slices.Contains(h.LocalTopics(""), "hall:1") {
		time.Sleep(5 * time.Millisecond)
	}
	if err := h.PushTopic("hall:2", "lecture.count", "1", map[string]int{"listeners": 3}); err != nil {
		t.Fatal(err)
	}
	if a := read(one); !strings.Contains(a, `"lecture.count"`) || !strings.Contains(a, `"listeners":3`) {
		t.Fatalf("moved socket got %q", a)
	}
	read(two)
}

// A socket that floods subscription frames is closed.
func TestFrameFloodCloses(t *testing.T) {
	h := NewHub(slog.New(slog.DiscardHandler), []string{"*"}, 10)
	h.TopicOK = func(string) bool { return true }
	srv := httptest.NewServer(h)
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	for i := range maxFramesPerMinute + 5 {
		if err := conn.Write(ctx, websocket.MessageText, []byte(`{"op":"sub","topic":"t`+strconv.Itoa(i)+`"}`)); err != nil {
			break
		}
	}
	if _, _, err := conn.Read(ctx); websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
		t.Fatalf("flood not closed: %v", err)
	}
	for h.Count() != 0 {
		time.Sleep(5 * time.Millisecond)
	}
	if len(h.LocalTopics("")) != 0 {
		t.Fatal("dropped client still indexed")
	}
}
