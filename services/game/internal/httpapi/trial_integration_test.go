//go:build integration

package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"tiendao/game/internal/character"
	"tiendao/game/internal/content"
	"tiendao/game/internal/cultivation"
	"tiendao/game/internal/httpapi"
	"tiendao/game/internal/platform/testdb"
	"tiendao/game/internal/trial"
)

// HTTP mapping of the trial endpoints (the rules themselves are covered in internal/trial).
func TestTrialEndpoints(t *testing.T) {
	ctx := context.Background()
	pool := testdb.New(t)
	cb, err := content.LoadAll("../../../../content/catalog")
	if err != nil {
		t.Fatal(err)
	}
	cult := &cultivation.Service{Pool: pool, Realms: cb.Realms, Contribution: cb.Contribution, VIP: cb.VIP, Now: time.Now}
	chars := &character.Service{Pool: pool, Cultivation: cult}
	user := uuid.New()
	if _, err := pool.Exec(ctx, "INSERT INTO users (id, email) VALUES ($1, $2)", user, user.String()+"@t"); err != nil {
		t.Fatal(err)
	}
	v, err := chars.Create(ctx, user, "Tâm Ma Thử")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE character_progressions SET stage_id='kim_dan.vien_man', realm_id='kim_dan', last_cultivated_at=now() WHERE character_id=$1", v.ID); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(httpapi.New(httpapi.Deps{Log: slog.New(slog.DiscardHandler), Pool: pool, Content: cb, Now: time.Now,
		Cultivation: cult, AuthDevHeader: true,
		Trials: &trial.Service{Pool: pool, Cat: cb.Trials, Realms: cb.Realms, Project: cult.ProjectedStage, VIP: cb.VIP, Now: time.Now}}))
	defer srv.Close()

	do := func(method, path, key string, body any) (int, map[string]any) {
		t.Helper()
		var rd *bytes.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			rd = bytes.NewReader(b)
		} else {
			rd = bytes.NewReader(nil)
		}
		req, _ := http.NewRequest(method, srv.URL+"/api/v1"+path, rd)
		req.Header.Set("X-Dev-User-Id", user.String())
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var out map[string]any
		_ = json.NewDecoder(res.Body).Decode(&out)
		return res.StatusCode, out
	}

	if code, st := do("GET", "/trials", "", nil); code != 200 || st["kinds"].([]any)[0].(map[string]any)["state"] != "available" {
		t.Fatalf("status %d %v", code, st)
	}
	if code, _ := do("POST", "/trials/tam_ma/start", "", nil); code != 400 {
		t.Fatalf("start without key: %d", code)
	}
	code, run := do("POST", "/trials/tam_ma/start", "http-start-0001", nil)
	if code != 201 || run["seed"].(float64) < 1 || run["params"].(map[string]any)["width"] != float64(9) {
		t.Fatalf("start %d %v", code, run)
	}
	runID := run["runId"].(string)
	if code, again := do("POST", "/trials/tam_ma/start", "http-start-0002", nil); code != 200 || again["runId"] != runID || again["resumed"] != true {
		t.Fatalf("resume %d %v", code, again)
	}
	if code, _ := do("POST", "/trials/khong_co/start", "http-start-0003", nil); code != 404 {
		t.Fatalf("unknown kind %d", code)
	}
	// Immediately submitting a long claimed game: too fast → rejected + lose cooldown.
	claim := map[string]any{"endTick": 400, "lines": 0, "score": 0, "stateHash": 1, "inputs": [][2]int{{0, 6}}}
	code, verdict := do("POST", "/trials/"+runID+"/submit", "http-submit-001", claim)
	if code != 200 || verdict["verdict"] != "rejected" || verdict["reason"] != "too_fast" || verdict["cooldownUntil"] == nil {
		t.Fatalf("submit %d %v", code, verdict)
	}
	if code, _ := do("POST", "/trials/"+runID+"/submit", "http-submit-002", claim); code != 409 {
		t.Fatalf("second submit %d", code)
	}
	if code, e := do("POST", "/trials/tam_ma/start", "http-start-0004", nil); code != 409 || e["error"].(map[string]any)["code"] != "cooldown" {
		t.Fatalf("start during cooldown %d %v", code, e)
	}
	if code, _ := do("POST", "/trials/"+uuid.NewString()+"/submit", "http-submit-003", claim); code != 404 {
		t.Fatalf("unknown run %d", code)
	}
	if code, _ := do("POST", "/trials/not-a-uuid/submit", "http-submit-004", claim); code != 404 {
		t.Fatalf("bad run id %d", code)
	}
	if code, _ := do("POST", "/trials/"+runID+"/submit", "http-submit-005", map[string]any{"score": 1, "bogus": true}); code != 400 {
		t.Fatalf("unknown field %d", code)
	}
}
