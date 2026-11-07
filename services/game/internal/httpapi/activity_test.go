package httpapi

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"tiendao/game/internal/protector"
)

type countingPresence struct {
	mu     sync.Mutex
	active int
}

func (c *countingPresence) MarkActive(context.Context, uuid.UUID) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.active++
	return nil
}
func (c *countingPresence) Ping(context.Context, uuid.UUID, uuid.UUID, string, time.Time) (bool, error) {
	return false, nil
}
func (c *countingPresence) Online(context.Context, []uuid.UUID, time.Time) (map[uuid.UUID]bool, error) {
	return nil, nil
}
func (c *countingPresence) FactionOnline(context.Context, string, time.Time, int64) ([]uuid.UUID, error) {
	return nil, nil
}

// Review H1: authentication itself never marks activity, whatever the method/status/path — not even a 2xx POST.
func TestAuthnNeverMarksActive(t *testing.T) {
	pres := &countingPresence{}
	d := Deps{Protectors: &protector.Service{Presence: pres}, AuthDevHeader: true}
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	for _, path := range []string{"/api/v1/cultivation/settle", "/api/v1/presence/ping", "/api/v1/lecture/heartbeat",
		"/api/v1/lecture/enter", "/api/v1/dao-lu/song-tu/heartbeat", "/api/v1/dao-lu/song-tu/claim", "/api/v1/shop/buy"} {
		req := httptest.NewRequest("POST", path, nil)
		req.Header.Set("X-Dev-User-Id", uuid.NewString())
		d.authn(ok).ServeHTTP(httptest.NewRecorder(), req)
	}
	if pres.active != 0 {
		t.Fatalf("authn marked %d times", pres.active)
	}
	r := httptest.NewRequest("POST", "/x", nil)
	d.markActive(r, false)
	d.markActive(r, true)
	if pres.active != 1 {
		t.Fatalf("markActive(real) should mark once, got %d", pres.active)
	}
}

// Review H1: markActive is called ONLY from the allowlisted handlers (real game actions). Adding a caller is a product
// decision: update this list (and docs/decisions.md) deliberately.
func TestMarkActiveAllowlist(t *testing.T) {
	want := []string{"breakthroughAttempt", "chatSend", "claimDailyTask", "cosmeticsBuy", "factionChange", "fortuneDraw",
		"lectureInsight", "offerIncense", "secretRealmSubmit", "settle", "shopBuy", "tradeCreate", "tradeRespond", "trialSubmit"}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var got []string
	for _, f := range files {
		if len(f) > 8 && f[len(f)-8:] == "_test.go" {
			continue
		}
		file, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || fn.Name.Name == "markActive" {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if c, ok := n.(*ast.CallExpr); ok {
					if sel, ok := c.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "markActive" && !slices.Contains(got, fn.Name.Name) {
						got = append(got, fn.Name.Name)
					}
				}
				return true
			})
		}
	}
	sort.Strings(got)
	if !slices.Equal(got, want) {
		t.Fatalf("markActive callers %v, want %v", got, want)
	}
}
