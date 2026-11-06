//go:build integration

package httpapi_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"tiendao/game/internal/auth"
	"tiendao/game/internal/breakthrough"
	"tiendao/game/internal/character"
	"tiendao/game/internal/content"
	"tiendao/game/internal/cultivation"
	"tiendao/game/internal/httpapi"
	"tiendao/game/internal/leaderboard"
	"tiendao/game/internal/platform/redisx"
	"tiendao/game/internal/platform/testdb"
	"tiendao/game/internal/vip"
)

// fakeGoogle "consents" immediately: AuthURL points straight at our callback.
type fakeGoogle struct{ callback string }

func (f fakeGoogle) AuthURL(state, nonce, verifier string) string {
	return f.callback + "?code=ok&state=" + url.QueryEscape(state)
}
func (f fakeGoogle) Exchange(_ context.Context, code, verifier, nonce string) (auth.Identity, error) {
	return auth.Identity{Provider: "google", Subject: "g-123", Email: "dao@example.com", EmailVerified: true, Name: "Đạo Hữu"}, nil
}

func TestEndToEnd(t *testing.T) {
	ctx := context.Background()
	pool := testdb.New(t)
	cb, err := content.LoadAll("../../../../content/catalog")
	if err != nil {
		t.Fatal(err)
	}
	redisURL := os.Getenv("TEST_REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379/15"
	}
	rdb, err := redisx.Connect(ctx, redisURL)
	if err != nil {
		t.Fatal(err)
	}
	lbKey := "test:lb:" + uuid.NewString()
	t.Cleanup(func() { rdb.Del(context.Background(), lbKey); rdb.Close() })

	secret := []byte(strings.Repeat("e", 32))
	iss, _ := auth.NewIssuer(secret, time.Minute, time.Now)
	cult := &cultivation.Service{Pool: pool, Realms: cb.Realms, Contribution: cb.Contribution, VIP: cb.VIP, Now: time.Now}
	lb := &leaderboard.Service{Pool: pool, Redis: rdb, Realms: cb.Realms, Key: lbKey}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()
	mux.Handle("/", httpapi.New(httpapi.Deps{
		Log: slog.New(slog.DiscardHandler), Pool: pool, Content: cb, Now: time.Now,
		Auth: &auth.Service{Pool: pool, Issuer: iss, RefreshTTL: time.Hour, Now: time.Now}, Issuer: iss,
		Google: fakeGoogle{callback: srv.URL + "/api/v1/auth/google/callback"}, Flow: auth.NewFlowSigner(secret),
		Cultivation: cult, Characters: &character.Service{Pool: pool, Cultivation: cult, Names: cb.Moderation},
		Breakthrough: &breakthrough.Service{Pool: pool, Cultivation: cult, RNG: breakthrough.NewRNG()},
		Leaderboard:  lb, VIP: &vip.Service{Pool: pool, Table: cb.VIP},
		PublicBaseURL: srv.URL,
	}))

	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			return nil
		}
		return http.ErrUseLastResponse // stop at the SPA page
	}}

	// 1. Google sign-in → redirected back to /cultivate with a refresh cookie.
	resp, err := c.Get(srv.URL + "/api/v1/auth/google/start?return=/cultivate")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if loc := resp.Header.Get("Location"); loc != srv.URL+"/cultivate" {
		t.Fatalf("login redirect = %q", loc)
	}

	// 2. Refresh → access token.
	var sess struct {
		AccessToken string `json:"accessToken"`
	}
	do(t, c, "POST", srv.URL+"/api/v1/auth/refresh", "", "", map[string]string{"X-Requested-With": "tiendao"}, 200, &sess)
	tok := sess.AccessToken

	// 3. No character yet → 404; create; duplicate → 409.
	do(t, c, "GET", srv.URL+"/api/v1/character", tok, "", nil, 404, nil)
	var blocked struct {
		Error struct{ Code, Message string }
	}
	do(t, c, "POST", srv.URL+"/api/v1/character", tok, `{"name":"Nữ Oa Nương Nương"}`, nil, 400, &blocked)
	if blocked.Error.Code != "name_not_allowed" || strings.Contains(blocked.Error.Message, "Nữ Oa") {
		t.Fatalf("blocked name: %+v", blocked)
	}
	do(t, c, "POST", srv.URL+"/api/v1/character", tok, `{"name":"Mộc Thanh"}`, nil, 201, nil)
	do(t, c, "POST", srv.URL+"/api/v1/character", tok, `{"name":"Mộc Thanh"}`, nil, 409, nil)

	// 4. Settle requires Idempotency-Key.
	do(t, c, "POST", srv.URL+"/api/v1/cultivation/settle", tok, "", nil, 400, nil)
	do(t, c, "POST", srv.URL+"/api/v1/cultivation/settle", tok, "", map[string]string{"Idempotency-Key": "e2e-settle-0001"}, 200, nil)

	// 5. Breakthrough not ready yet (fresh character) → 409 with details.
	var nr struct {
		Error struct {
			Code    string `json:"code"`
			Details struct {
				Missing []map[string]any `json:"missing"`
			} `json:"details"`
		} `json:"error"`
	}
	do(t, c, "POST", srv.URL+"/api/v1/breakthrough/attempt", tok, "", map[string]string{"Idempotency-Key": "e2e-bt-000001"}, 409, &nr)
	if nr.Error.Code != "not_ready" || len(nr.Error.Details.Missing) == 0 {
		t.Fatalf("not ready body %+v", nr)
	}

	// 6. Leaderboard after rebuild contains us at rank 1.
	if _, err := lb.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	var me struct{ Rank int64 }
	do(t, c, "GET", srv.URL+"/api/v1/leaderboards/global/me", tok, "", nil, 200, &me)
	if me.Rank != 1 {
		t.Fatalf("rank %d", me.Rank)
	}

	// 7. Logout kills the session.
	do(t, c, "POST", srv.URL+"/api/v1/auth/logout", "", "", nil, 204, nil)
	do(t, c, "POST", srv.URL+"/api/v1/auth/refresh", "", "", map[string]string{"X-Requested-With": "tiendao"}, 401, nil)
}

func do(t *testing.T, c *http.Client, method, u, tok, body string, hdr map[string]string, want int, out any) {
	t.Helper()
	req, _ := http.NewRequest(method, u, strings.NewReader(body))
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != want {
		t.Fatalf("%s %s = %d, want %d: %s", method, u, resp.StatusCode, want, b)
	}
	if out != nil {
		if err := json.Unmarshal(b, out); err != nil {
			t.Fatalf("decode: %v: %s", err, b)
		}
	}
}
