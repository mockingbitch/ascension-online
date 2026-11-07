package httpapi

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"tiendao/game/internal/content"
	"tiendao/game/internal/payment"
)

// paymentServer has a payment service with a sandbox provider but NO database: every request that reaches the DB
// would panic, which proves the webhook rejects bad signatures before any DB work.
func paymentServer(t *testing.T, dev bool) (*payment.Sandbox, *httptest.Server) {
	t.Helper()
	cb, err := content.LoadAll(filepath.Join("..", "..", "..", "..", "content", "catalog"))
	if err != nil {
		t.Fatal(err)
	}
	sb, err := payment.NewSandbox(bytes.Repeat([]byte("w"), 32), "http://x/", nil)
	if err != nil {
		t.Fatal(err)
	}
	svc := &payment.Service{Cat: cb.Payments, VIP: cb.VIP, Providers: map[string]payment.Provider{payment.ProviderSandbox: sb}}
	h := New(Deps{Log: slog.New(slog.DiscardHandler), Content: cb, Payments: svc, PaymentDev: dev, AuthDevHeader: true})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return sb, srv
}

func TestPaymentPackagesPublic(t *testing.T) {
	_, srv := paymentServer(t, false)
	res, err := srv.Client().Get(srv.URL + "/api/v1/payments/packages")
	if err != nil || res.StatusCode != 200 {
		t.Fatalf("status %v %v", res, err)
	}
	var body payment.PackagesView
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil || len(body.Packages) == 0 || len(body.Providers) != 1 ||
		body.Packages[0].Points != body.Packages[0].AmountVND/1000*body.PointsPer1000VND {
		t.Fatalf("body %+v %v", body, err)
	}
}

func TestPaymentWebhookSignatureFirst(t *testing.T) {
	sb, srv := paymentServer(t, false)
	body, sig := sb.BuildWebhook(payment.Event{EventID: "e1", OrderID: uuid.New(), Kind: payment.KindPaid, AmountVND: 20000}, time.Now())
	cases := []struct {
		name, path, sig string
		body            []byte
		want            int
	}{
		{"missing signature", "/sandbox", "", body, 401},
		{"invalid signature", "/sandbox", "t=1,v1=" + strings.Repeat("ab", 32), body, 401},
		{"tampered body", "/sandbox", sig, bytes.Replace(body, []byte("20000"), []byte("90000"), 1), 401},
		{"stale replay", "/sandbox", sb.Sign(body, time.Now().Add(-time.Hour)), body, 401},
		{"unknown provider", "/paypal", sig, body, 404},
		{"oversized", "/sandbox", sig, bytes.Repeat([]byte("x"), payment.MaxWebhookBytes+10), 413},
	}
	for _, c := range cases {
		req := httptest.NewRequest("POST", srv.URL+"/api/v1/payments/webhook"+c.path, bytes.NewReader(c.body))
		req.RequestURI = ""
		if c.sig != "" {
			req.Header.Set(payment.SandboxSignatureHeader, c.sig)
		}
		res, err := srv.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != c.want {
			t.Errorf("%s: status %d, want %d", c.name, res.StatusCode, c.want)
		}
	}
}

func TestPaymentRoutesAuthAndDevGate(t *testing.T) {
	_, prod := paymentServer(t, false)
	for _, p := range []string{"/api/v1/payments/orders", "/api/v1/admin/payments/orders/" + uuid.NewString() + "/refund"} {
		res, err := prod.Client().Post(prod.URL+p, "application/json", strings.NewReader(`{}`))
		if err != nil || res.StatusCode != 401 {
			t.Errorf("%s without auth: %v %v", p, res.StatusCode, err)
		}
	}
	// Admin routes refuse players.
	req := httptest.NewRequest("GET", prod.URL+"/api/v1/admin/payments/orders", nil)
	req.RequestURI = ""
	req.Header.Set("X-Dev-User-Id", uuid.NewString())
	if res, err := prod.Client().Do(req); err != nil || res.StatusCode != 403 {
		t.Errorf("admin as player: %v %v", res.StatusCode, err)
	}
	devPath := "/api/v1/dev/payments/sandbox/" + uuid.NewString()
	do := func(srvURL string) int {
		req := httptest.NewRequest("POST", srvURL+devPath, strings.NewReader(`{"outcome":"paid"}`))
		req.RequestURI = ""
		res, err := prod.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return res.StatusCode
	}
	if got := do(prod.URL); got != 404 && got != 405 {
		t.Errorf("sandbox endpoint registered outside dev: %d", got)
	}
	_, dev := paymentServer(t, true)
	if got := do(dev.URL); got != 401 {
		t.Errorf("dev sandbox endpoint without auth: %d", got)
	}
}
