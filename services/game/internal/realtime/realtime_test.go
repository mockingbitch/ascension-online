package realtime

import (
	"context"
	"log/slog"
	"net/http/httptest"
	"path/filepath"
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
