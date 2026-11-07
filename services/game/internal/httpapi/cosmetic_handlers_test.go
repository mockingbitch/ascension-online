package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"tiendao/game/internal/auth"
	"tiendao/game/internal/content"
)

func TestCosmeticCatalogPublicAndCached(t *testing.T) {
	rec := httptest.NewRecorder()
	testServer(t).ServeHTTP(rec, httptest.NewRequest("GET", "/api/v1/cosmetics/catalog", nil))
	if rec.Code != 200 || !strings.Contains(rec.Header().Get("Cache-Control"), "max-age") {
		t.Fatalf("status %d cache %q", rec.Code, rec.Header().Get("Cache-Control"))
	}
	var body struct {
		Settings  struct{ CaveSlots int }
		Cosmetics []struct {
			ID, Kind, Source string
			PerkTier         int
		}
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil || body.Settings.CaveSlots < 1 || len(body.Cosmetics) == 0 {
		t.Fatalf("body: %+v %v", body, err)
	}
	for _, c := range body.Cosmetics {
		if c.Source == "tien_duyen" && c.PerkTier == 0 {
			t.Errorf("%s: perk tier not bound", c.ID)
		}
	}
}

func TestCosmeticAndDonationRoutesGuarded(t *testing.T) {
	cb, _ := content.LoadAll(filepath.Join("..", "..", "..", "..", "content", "catalog"))
	iss, _ := auth.NewIssuer([]byte(strings.Repeat("s", 32)), time.Minute, time.Now)
	h := New(Deps{Log: slog.New(slog.DiscardHandler), Content: cb, Issuer: iss})
	player, _ := iss.Issue(auth.Principal{UserID: uuid.New(), Role: "player"})
	for _, tc := range []struct {
		method, path, tok string
		want              int
	}{
		{"GET", "/api/v1/cosmetics", "", 401},
		{"POST", "/api/v1/cosmetics/buy", "", 401},
		{"POST", "/api/v1/cosmetics/equip", "", 401},
		{"GET", "/api/v1/factions/works", "", 401},
		{"POST", "/api/v1/factions/donate", "", 401},
		{"GET", "/api/v1/cosmetics", player, 503}, // no services wired in this test
		{"POST", "/api/v1/cosmetics/buy", player, 503},
		{"POST", "/api/v1/cosmetics/equip", player, 503},
		{"GET", "/api/v1/factions/works", player, 503},
		{"POST", "/api/v1/factions/donate", player, 503},
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
