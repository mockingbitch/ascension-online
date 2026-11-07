package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"tiendao/game/internal/auth"
	"tiendao/game/internal/content"
	"tiendao/game/internal/trade"
)

func TestTradeRoutesGuarded(t *testing.T) {
	cb, _ := content.LoadAll(filepath.Join("..", "..", "..", "..", "content", "catalog"))
	iss, _ := auth.NewIssuer([]byte(strings.Repeat("s", 32)), time.Minute, time.Now)
	h := New(Deps{Log: slog.New(slog.DiscardHandler), Content: cb, Issuer: iss})
	player, _ := iss.Issue(auth.Principal{UserID: uuid.New(), Role: "player"})
	id := uuid.NewString()
	for _, tc := range []struct {
		method, path, tok string
		want              int
	}{
		{"GET", "/api/v1/trades", "", 401},
		{"POST", "/api/v1/trades", "", 401},
		{"POST", "/api/v1/trades/" + id + "/accept", "", 401},
		{"GET", "/api/v1/admin/trades/flows", "", 401},
		{"GET", "/api/v1/admin/trades/flows", player, 403}, // admin only
		{"GET", "/api/v1/trades", player, 503},             // no service wired in this test
		{"GET", "/api/v1/trades/history", player, 503},
		{"GET", "/api/v1/trades/peer/" + id, player, 503},
		{"POST", "/api/v1/trades", player, 503},
		{"POST", "/api/v1/trades/" + id + "/decline", player, 503},
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

func TestTradeErrorMapping(t *testing.T) {
	d := Deps{Log: slog.New(slog.DiscardHandler)}
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{&trade.LineError{Code: "not_tradable", AssetID: "cuu_tieu_hoan"}, 400, "invalid_lines"},
		{&trade.GateError{Side: "other", Action: "receive", Blockers: []string{"account_age"}}, 409, "not_eligible"},
		{&trade.LimitError{Side: "self", Code: trade.LimitReceived, Cap: 300, Used: 200, Requested: 101}, 409, "limit_reached"},
		{&trade.LimitError{Side: "self", Code: trade.LimitOffersPerDay, Cap: 5, Used: 5, Requested: 1}, 429, "limit_reached"},
		{&trade.InsufficientError{Side: "other", AssetID: "spirit_stone"}, 409, "insufficient"},
		{trade.ErrNotAcknowledged, 400, "irreversible_not_acknowledged"},
		{trade.ErrOfferGone, 409, "offer_gone"},
		{trade.ErrSelf, 409, "self"},
		{trade.ErrTooManyOpen, 409, "too_many_open"},
		{trade.ErrKeyReused, 409, "idempotency_key_reused"},
		{errors.New("boom"), 500, "internal"},
	} {
		rec := httptest.NewRecorder()
		d.writeTradeError(rec, httptest.NewRequest("POST", "/api/v1/trades", nil), "test", tc.err)
		var body struct{ Error apiError }
		_ = json.NewDecoder(rec.Body).Decode(&body)
		if rec.Code != tc.status || body.Error.Code != tc.code {
			t.Errorf("%v: %d %q, want %d %q", tc.err, rec.Code, body.Error.Code, tc.status, tc.code)
		}
	}
}
