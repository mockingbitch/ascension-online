package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"tiendao/game/internal/auth"

	"tiendao/game/internal/content"
)

func testServer(t *testing.T) http.Handler {
	t.Helper()
	cb, err := content.LoadAll(filepath.Join("..", "..", "..", "..", "content", "catalog"))
	if err != nil {
		t.Fatal(err)
	}
	return New(Deps{Log: slog.New(slog.DiscardHandler), Content: cb})
}

func TestRealmsEndpoint(t *testing.T) {
	rec := httptest.NewRecorder()
	testServer(t).ServeHTTP(rec, httptest.NewRequest("GET", "/api/v1/realms", nil))
	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	var body struct{ Realms []map[string]any }
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil || len(body.Realms) == 0 {
		t.Fatalf("bad body: %v", err)
	}
}

func TestSettleRequiresAuth(t *testing.T) {
	rec := httptest.NewRecorder()
	testServer(t).ServeHTTP(rec, httptest.NewRequest("POST", "/api/v1/cultivation/settle", nil))
	if rec.Code != 401 {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestVIPTiersEndpoint(t *testing.T) {
	rec := httptest.NewRecorder()
	testServer(t).ServeHTTP(rec, httptest.NewRequest("GET", "/api/v1/vip/tiers", nil))
	var body struct{ Tiers []map[string]any }
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil || len(body.Tiers) != 13 {
		t.Fatalf("want 13 tiers, got %d (%v)", len(body.Tiers), err)
	}
}

func TestBearerAuthAndAdminGuard(t *testing.T) {
	cb, _ := content.LoadAll(filepath.Join("..", "..", "..", "..", "content", "catalog"))
	iss, _ := auth.NewIssuer([]byte(strings.Repeat("s", 32)), time.Minute, time.Now)
	h := New(Deps{Log: slog.New(slog.DiscardHandler), Content: cb, Issuer: iss})

	player, _ := iss.Issue(auth.Principal{UserID: uuid.New(), Role: "player"})
	req := httptest.NewRequest("POST", "/api/v1/admin/vip/grant", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+player)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 403 {
		t.Fatalf("player on admin route: %d", rec.Code)
	}

	req = httptest.NewRequest("GET", "/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer garbage")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Fatalf("bad token: %d", rec.Code)
	}
}

func TestDevHeaderIgnoredUnlessEnabled(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/character", nil)
	req.Header.Set("X-Dev-User-Id", uuid.NewString())
	rec := httptest.NewRecorder()
	testServer(t).ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Fatalf("dev header must be ignored when disabled: %d", rec.Code)
	}
}

func TestGoogleStartWithoutConfig(t *testing.T) {
	rec := httptest.NewRecorder()
	testServer(t).ServeHTTP(rec, httptest.NewRequest("GET", "/api/v1/auth/google/start", nil))
	if rec.Code != 503 {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestReportRoutesGuarded(t *testing.T) {
	cb, _ := content.LoadAll(filepath.Join("..", "..", "..", "..", "content", "catalog"))
	iss, _ := auth.NewIssuer([]byte(strings.Repeat("s", 32)), time.Minute, time.Now)
	h := New(Deps{Log: slog.New(slog.DiscardHandler), Content: cb, Issuer: iss})
	player, _ := iss.Issue(auth.Principal{UserID: uuid.New(), Role: "player"})
	for _, tc := range []struct {
		method, path, tok string
		want              int
	}{
		{"POST", "/api/v1/reports", "", 401},
		{"GET", "/api/v1/admin/reports", "", 401},
		{"GET", "/api/v1/admin/reports", player, 403},
		{"POST", "/api/v1/admin/reports/" + uuid.NewString() + "/resolve", player, 403},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{}`))
		if tc.tok != "" {
			req.Header.Set("Authorization", "Bearer "+tc.tok)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Errorf("%s %s: %d, want %d", tc.method, tc.path, rec.Code, tc.want)
		}
	}
}

func TestNameBlockedMessageHidesTerm(t *testing.T) {
	cb, err := content.LoadAll(filepath.Join("..", "..", "..", "..", "content", "catalog"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Địt Mẹ", "Admin Lâm", "Hồng Quân", "Quan Âm Bồ Tát"} {
		msg, ok := nameBlockedMessage(cb.Moderation.CheckName(name))
		if !ok || msg == "" || strings.Contains(strings.ToLower(msg), strings.ToLower(name)) {
			t.Errorf("%q → %q %v", name, msg, ok)
		}
	}
	if _, ok := nameBlockedMessage(nil); ok {
		t.Error("nil error must not map")
	}
}

func TestWorldRoutesGuarded(t *testing.T) {
	cb, _ := content.LoadAll(filepath.Join("..", "..", "..", "..", "content", "catalog"))
	iss, _ := auth.NewIssuer([]byte(strings.Repeat("s", 32)), time.Minute, time.Now)
	h := New(Deps{Log: slog.New(slog.DiscardHandler), Content: cb, Issuer: iss})
	player, _ := iss.Issue(auth.Principal{UserID: uuid.New(), Role: "player"})
	for _, tc := range []struct {
		method, path, tok string
		want              int
	}{
		{"GET", "/api/v1/world", "", 503}, // public; no world service wired in this test
		{"GET", "/api/v1/admin/world", "", 401},
		{"GET", "/api/v1/admin/world", player, 403},
		{"PUT", "/api/v1/admin/world/milestones/giang_dao_2", player, 403},
		{"POST", "/api/v1/admin/world/pause", player, 403},
		{"POST", "/api/v1/admin/world/resume", "", 401},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{}`))
		if tc.tok != "" {
			req.Header.Set("Authorization", "Bearer "+tc.tok)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Errorf("%s %s: %d, want %d", tc.method, tc.path, rec.Code, tc.want)
		}
	}
}

func TestShopRitualRoutesGuarded(t *testing.T) {
	cb, _ := content.LoadAll(filepath.Join("..", "..", "..", "..", "content", "catalog"))
	iss, _ := auth.NewIssuer([]byte(strings.Repeat("s", 32)), time.Minute, time.Now)
	h := New(Deps{Log: slog.New(slog.DiscardHandler), Content: cb, Issuer: iss})
	player, _ := iss.Issue(auth.Principal{UserID: uuid.New(), Role: "player"})
	for _, tc := range []struct {
		method, path, tok string
		want              int
	}{
		{"GET", "/api/v1/shop", "", 401},
		{"POST", "/api/v1/shop/buy", "", 401},
		{"GET", "/api/v1/fortune", "", 401},
		{"POST", "/api/v1/fortune/draw", "", 401},
		{"POST", "/api/v1/tasks/daily/incense", "", 401},
		{"GET", "/api/v1/shop", player, 503}, // no services wired in this test
		{"POST", "/api/v1/shop/buy", player, 503},
		{"POST", "/api/v1/fortune/draw", player, 503},
		{"POST", "/api/v1/tasks/daily/incense", player, 503},
		{"POST", "/api/v1/items/tu_linh_dan/use", player, 503},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{}`))
		if tc.tok != "" {
			req.Header.Set("Authorization", "Bearer "+tc.tok)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Errorf("%s %s: %d, want %d", tc.method, tc.path, rec.Code, tc.want)
		}
	}
}

func TestTrialRoutesGuarded(t *testing.T) {
	cb, _ := content.LoadAll(filepath.Join("..", "..", "..", "..", "content", "catalog"))
	iss, _ := auth.NewIssuer([]byte(strings.Repeat("s", 32)), time.Minute, time.Now)
	h := New(Deps{Log: slog.New(slog.DiscardHandler), Content: cb, Issuer: iss})
	player, _ := iss.Issue(auth.Principal{UserID: uuid.New(), Role: "player"})
	run := uuid.NewString()
	for _, tc := range []struct {
		method, path, tok string
		want              int
	}{
		{"GET", "/api/v1/trials", "", 401},
		{"POST", "/api/v1/trials/tam_ma/start", "", 401},
		{"POST", "/api/v1/trials/" + run + "/submit", "", 401},
		{"GET", "/api/v1/trials", player, 503}, // no services wired in this test
		{"POST", "/api/v1/trials/tam_ma/start", player, 503},
		{"POST", "/api/v1/trials/" + run + "/submit", player, 503},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{}`))
		if tc.tok != "" {
			req.Header.Set("Authorization", "Bearer "+tc.tok)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Errorf("%s %s: %d, want %d", tc.method, tc.path, rec.Code, tc.want)
		}
	}
}

// Review M1: X-Forwarded-For is honoured only when TRUST_PROXY is on AND the direct peer is a trusted proxy.
func TestClientIPTrustedProxiesOnly(t *testing.T) {
	proxies := []netip.Prefix{netip.MustParsePrefix("172.16.0.0/12"), netip.MustParsePrefix("127.0.0.1/32")}
	req := func(remote, xff string) *http.Request {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = remote
		if xff != "" {
			r.Header.Set("X-Forwarded-For", xff)
		}
		return r
	}
	cases := []struct {
		remote, xff string
		proxies     []netip.Prefix
		want        string
	}{
		{"172.18.0.5:5555", "1.2.3.4, 198.51.100.7", proxies, "198.51.100.7"}, // nginx on the compose network: rightmost
		{"203.0.113.9:4444", "9.9.9.9", proxies, "203.0.113.9"},               // direct client spoofing XFF: ignored
		{"172.18.0.5:5555", "198.51.100.7", nil, "172.18.0.5"},                // TRUST_PROXY off
		{"[::ffff:127.0.0.1]:80", "198.51.100.8", proxies, "198.51.100.8"},    // IPv4-mapped loopback
		{"172.18.0.5:5555", "", proxies, "172.18.0.5"},                        // no header
	}
	for _, c := range cases {
		if got := clientIP(req(c.remote, c.xff), c.proxies); got != c.want {
			t.Errorf("clientIP(%s, %q) = %s, want %s", c.remote, c.xff, got, c.want)
		}
	}
}
