//go:build integration

package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
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

// Hóa phàm over HTTP: the stroke log decodes, and a body above trial.MaxClaimBytes is refused before any judging.
func TestHoaPhamSubmitBodyLimit(t *testing.T) {
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
	v, err := chars.Create(ctx, user, "Hóa Phàm Thử")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE character_progressions SET stage_id='nguyen_anh.vien_man_2', realm_id='nguyen_anh', last_cultivated_at=now() WHERE character_id=$1", v.ID); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(httpapi.New(httpapi.Deps{Log: slog.New(slog.DiscardHandler), Pool: pool, Content: cb, Now: time.Now,
		Cultivation: cult, AuthDevHeader: true,
		Trials: &trial.Service{Pool: pool, Cat: cb.Trials, Realms: cb.Realms, Project: cult.ProjectedStage, VIP: cb.VIP, Now: time.Now}}))
	defer srv.Close()
	do := func(method, path, key string, body []byte) (int, map[string]any) {
		t.Helper()
		req, _ := http.NewRequest(method, srv.URL+"/api/v1"+path, bytes.NewReader(body))
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
	code, run := do("POST", "/trials/hoa_pham/start", "hp-http-start-01", nil)
	if code != 201 || run["kind"] != "hoa_pham" || run["params"].(map[string]any)["maxStep"] == nil {
		t.Fatalf("start %d %v", code, run)
	}
	runID := run["runId"].(string)
	// ~120 KiB of strokes: refused by the body limit (400), the run stays open.
	big := `{"endTick":2250,"lines":0,"score":0,"stateHash":1,"strokes":[` + strings.Repeat("[1,1,1,0,0,0,0,0,0,0,0],", 11000) + `[5,5,5]]}`
	if len(big) <= trial.MaxClaimBytes {
		t.Fatal("test body too small")
	}
	if code, e := do("POST", "/trials/"+runID+"/submit", "hp-http-sub-001", []byte(big)); code != 400 || e["error"].(map[string]any)["code"] != "invalid_body" {
		t.Fatalf("oversized %d %v", code, e)
	}
	// A small stroke log decodes and is judged (here: too fast → rejected).
	small := `{"endTick":400,"lines":0,"score":0,"stateHash":1,"inputs":[],"strokes":[[0,10,10,5,5]]}`
	if code, e := do("POST", "/trials/"+runID+"/submit", "hp-http-sub-002", []byte(small)); code != 200 || e["verdict"] != "rejected" || e["reason"] != "too_fast" {
		t.Fatalf("small %d %v", code, e)
	}
}
