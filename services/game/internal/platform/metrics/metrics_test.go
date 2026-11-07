package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestInstrumentUsesRoutePatternAndStatusClass(t *testing.T) {
	reg := NewRegistry()
	m := NewAPI(reg)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/items/{id}", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(404) })
	mux.HandleFunc("POST /api/v1/settle", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	h := m.Instrument(mux)

	for _, p := range []string{"/api/v1/items/a", "/api/v1/items/b", "/api/v1/items/c"} {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", p, nil))
	}
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/api/v1/settle", nil))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/nope/12345", nil))

	if got := testutil.ToFloat64(m.requests.WithLabelValues("GET /api/v1/items/{id}", "4xx")); got != 3 {
		t.Fatalf("items 4xx = %v, want 3 (raw paths must collapse into the pattern)", got)
	}
	if got := testutil.ToFloat64(m.requests.WithLabelValues("POST /api/v1/settle", "2xx")); got != 1 {
		t.Fatalf("settle 2xx = %v", got)
	}
	if got := testutil.ToFloat64(m.requests.WithLabelValues("unmatched", "4xx")); got != 1 {
		t.Fatalf("unmatched = %v", got)
	}
	if n := testutil.CollectAndCount(m.requests); n != 3 {
		t.Fatalf("series = %d, want 3", n)
	}
}

func TestNilRecordersAreNoops(t *testing.T) {
	var a *API
	var w *Worker
	a.Settle(true)
	a.Breakthrough(true, true, false)
	a.TrialVerdict("tam_ma", "pass", false)
	a.SecretRealmVerdict("x", "fail", false)
	a.NgoDao(10, false)
	a.Purchase("shop", false)
	a.ChatMessage("world")
	a.RateLimited("game")
	w.Job("world_tick", time.Now(), nil)
	w.Outbox(1, 0)
	w.DeadLetter("x")
	w.Handled("x")
	w.Stone("r", 1, 1)
	w.Duyen("s", 1, 1)
	h := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	if got := a.Instrument(h); got == nil {
		t.Fatal("nil Instrument must return the handler")
	}
}

func TestBoundedFoldsOverflowAndJunk(t *testing.T) {
	b := NewBounded(2)
	if b.Value("a") != "a" || b.Value("b") != "b" || b.Value("a") != "a" {
		t.Fatal("known values must pass")
	}
	if b.Value("c") != "other" {
		t.Fatal("overflow must fold into other")
	}
	if NewBounded(5).Value("Bad Value!") != "other" || NewBounded(5).Value(strings.Repeat("x", 49)) != "other" {
		t.Fatal("junk must fold into other")
	}
}

func TestDomainCountersSkipReplays(t *testing.T) {
	m := NewAPI(NewRegistry())
	m.Breakthrough(true, true, false)
	m.Breakthrough(false, true, true)
	m.Purchase("shop", true)
	m.TrialVerdict("tam_ma", "pass", true)
	if got := testutil.ToFloat64(m.breakthrough.WithLabelValues("success", "true")); got != 1 {
		t.Fatalf("success = %v", got)
	}
	if got := testutil.ToFloat64(m.breakthrough.WithLabelValues("replayed", "true")); got != 1 {
		t.Fatalf("replayed = %v", got)
	}
	if n := testutil.CollectAndCount(m.purchases); n != 0 {
		t.Fatalf("replayed purchase counted: %d", n)
	}
	if n := testutil.CollectAndCount(m.trials); n != 0 {
		t.Fatalf("replayed verdict counted: %d", n)
	}
}
