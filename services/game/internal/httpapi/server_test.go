package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
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
