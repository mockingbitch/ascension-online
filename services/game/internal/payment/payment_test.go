package payment

import (
	"bytes"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"tiendao/game/internal/vip"
)

func TestShippedPaymentsCatalogue(t *testing.T) {
	c, err := LoadFile("../../../../content/catalog/payments.yaml")
	if err != nil {
		t.Fatal(err)
	}
	tbl, err := vip.LoadFile("../../../../content/catalog/vip.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Bind(tbl.PointsForVND, vip.MaxGrantDelta); err != nil {
		t.Fatal(err)
	}
	for _, p := range c.Packages {
		// No bonus: points are exactly the vip.yaml rate.
		if p.Points != p.AmountVND/1000*tbl.PointsPer1000VND {
			t.Errorf("package %s: %d points for %dđ", p.ID, p.Points, p.AmountVND)
		}
	}
	if got := c.EnabledProviders("prod"); len(got) != 0 {
		t.Errorf("prod must have no enabled provider until a real adapter ships, got %v", got)
	}
	if got := c.EnabledProviders("dev"); len(got) != 1 || got[0].ID != ProviderSandbox {
		t.Errorf("dev providers: %v", got)
	}
}

const base = `version: 1
currency: VND
min_amount_vnd: 10000
max_amount_vnd: 2000000
order_expiry_minutes: 30
max_open_orders_per_user: 3
reconcile_after_minutes: 5
reconcile_every_minutes: 5
reconcile_late_hours: 24
sweep_batch: 100
`

func TestLoaderRejects(t *testing.T) {
	cases := map[string]string{
		"sandbox in prod":     "providers:\n  - { id: sandbox, name: S, enabled: true, envs: [dev, prod] }\npackages:\n  - { id: a, name: A, amount_vnd: 20000 }\n",
		"enabled no adapter":  "providers:\n  - { id: payos, name: P, enabled: true, envs: [prod] }\npackages:\n  - { id: a, name: A, amount_vnd: 20000 }\n",
		"unknown provider":    "providers:\n  - { id: paypal, name: P, enabled: false, envs: [prod] }\npackages:\n  - { id: a, name: A, amount_vnd: 20000 }\n",
		"not multiple":        "providers: []\npackages:\n  - { id: a, name: A, amount_vnd: 20500 }\n",
		"below min":           "providers: []\npackages:\n  - { id: a, name: A, amount_vnd: 5000 }\n",
		"above max":           "providers: []\npackages:\n  - { id: a, name: A, amount_vnd: 3000000 }\n",
		"not increasing":      "providers: []\npackages:\n  - { id: a, name: A, amount_vnd: 50000 }\n  - { id: b, name: B, amount_vnd: 20000 }\n",
		"authored points":     "providers: []\npackages:\n  - { id: a, name: A, amount_vnd: 20000, points: 999 }\n",
		"duplicate package":   "providers: []\npackages:\n  - { id: a, name: A, amount_vnd: 20000 }\n  - { id: a, name: B, amount_vnd: 50000 }\n",
		"no packages":         "providers: []\npackages: []\n",
		"bad id":              "providers: []\npackages:\n  - { id: Gói, name: A, amount_vnd: 20000 }\n",
		"unknown env":         "providers:\n  - { id: sandbox, name: S, enabled: false, envs: [qa] }\npackages:\n  - { id: a, name: A, amount_vnd: 20000 }\n",
		"duplicate providers": "providers:\n  - { id: sandbox, name: S, enabled: false, envs: [dev] }\n  - { id: sandbox, name: S, enabled: false, envs: [dev] }\npackages:\n  - { id: a, name: A, amount_vnd: 20000 }\n",
	}
	for name, tail := range cases {
		if _, err := Load(strings.NewReader(base + tail)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	c, err := Load(strings.NewReader(base + "providers: []\npackages:\n  - { id: a, name: A, amount_vnd: 20000 }\n"))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Bind(func(int64) int64 { return 0 }, vip.MaxGrantDelta); err == nil {
		t.Error("zero-point package accepted")
	}
	if err := c.Bind(func(v int64) int64 { return v }, vip.MaxGrantDelta); err == nil {
		t.Error("max amount above MaxGrantDelta accepted")
	}
}

func TestCanTransition(t *testing.T) {
	all := []string{StatusCreated, StatusPending, StatusPaid, StatusFailed, StatusExpired, StatusRefunded}
	allowed := map[[2]string]bool{
		{StatusCreated, StatusPending}: true, {StatusCreated, StatusPaid}: true, {StatusCreated, StatusFailed}: true,
		{StatusCreated, StatusExpired}: true, {StatusPending, StatusPaid}: true, {StatusPending, StatusFailed}: true,
		{StatusPending, StatusExpired}: true, {StatusFailed, StatusPaid}: true, {StatusExpired, StatusPaid}: true,
		{StatusPaid, StatusRefunded}: true,
	}
	for _, a := range all {
		for _, b := range all {
			if got := CanTransition(a, b); got != allowed[[2]string{a, b}] {
				t.Errorf("%s → %s: %v", a, b, got)
			}
		}
	}
}

func TestDecide(t *testing.T) {
	paid := func(amount int64) Event { return Event{Kind: KindPaid, AmountVND: amount} }
	cases := []struct {
		name, status string
		ev           Event
		expect       int64
		outcome, to  string
		alert        bool
	}{
		{"paid pending", StatusPending, paid(20000), 200, OutcomeApplied, StatusPaid, false},
		{"paid created (fast webhook)", StatusCreated, paid(20000), 200, OutcomeApplied, StatusPaid, false},
		{"second paid webhook (paid twice)", StatusPaid, paid(20000), 200, OutcomeIgnored, "", true},
		{"paid webhook after refund", StatusRefunded, paid(20000), 200, OutcomeIgnored, "", true},
		{"amount mismatch", StatusPending, paid(10000), 200, OutcomeRejected, "", true},
		{"amount mismatch on a settled order is a duplicate", StatusPaid, paid(1), 200, OutcomeIgnored, "", true},
		{"rate changed since order", StatusPending, paid(20000), 100, OutcomeRejected, "", true},
		{"late payment after expiry", StatusExpired, paid(20000), 200, OutcomeApplied, StatusPaid, true},
		{"late payment after fail", StatusFailed, paid(20000), 200, OutcomeApplied, StatusPaid, true},
		{"failed pending", StatusPending, Event{Kind: KindFailed}, 200, OutcomeApplied, StatusFailed, false},
		{"failed after paid (out of order)", StatusPaid, Event{Kind: KindFailed}, 200, OutcomeIgnored, "", false},
		{"expired after paid", StatusPaid, Event{Kind: KindExpired}, 200, OutcomeIgnored, "", false},
		{"failed twice", StatusFailed, Event{Kind: KindFailed}, 200, OutcomeReplayed, "", false},
		{"provider refund", StatusPaid, Event{Kind: KindRefunded}, 200, OutcomeIgnored, "", true},
		{"pending query", StatusPending, Event{Kind: KindPending}, 200, OutcomeIgnored, "", false},
	}
	for _, c := range cases {
		d := Decide(c.status, 20000, 200, c.ev, c.expect, true)
		if d.Outcome != c.outcome || d.To != c.to || d.Alert != c.alert {
			t.Errorf("%s: %+v", c.name, d)
		}
		// Reconciliation re-reporting a settled order is a silent replay.
		if q := Decide(c.status, 20000, 200, c.ev, c.expect, false); (c.status == StatusPaid || c.status == StatusRefunded) &&
			c.ev.Kind == KindPaid && q.Outcome != OutcomeReplayed {
			t.Errorf("%s via reconcile: %+v", c.name, q)
		}
		if d.Outcome == OutcomeApplied && !CanTransition(c.status, d.To) {
			t.Errorf("%s: Decide applies an illegal transition %s → %s", c.name, c.status, d.To)
		}
	}
}

func testSandbox(t *testing.T) *Sandbox {
	t.Helper()
	sb, err := NewSandbox(bytes.Repeat([]byte("k"), 32), "http://x/tien-duyen/sandbox/", nil)
	if err != nil {
		t.Fatal(err)
	}
	return sb
}

func TestSandboxSignature(t *testing.T) {
	sb := testSandbox(t)
	now := time.Date(2026, 11, 7, 3, 0, 0, 0, time.UTC)
	ev := Event{EventID: "evt_1", OrderID: uuid.New(), ProviderRef: "sbx_1", Kind: KindPaid, AmountVND: 20000, OccurredAt: now}
	body, sig := sb.BuildWebhook(ev, now)
	hdr := func(v string) http.Header {
		h := http.Header{}
		if v != "" {
			h.Set(SandboxSignatureHeader, v)
		}
		return h
	}

	got, err := sb.VerifyWebhook(hdr(sig), body, now.Add(time.Minute))
	if err != nil || got.EventID != "evt_1" || got.OrderID != ev.OrderID || got.AmountVND != 20000 || got.Kind != KindPaid {
		t.Fatalf("valid: %+v %v", got, err)
	}
	if _, err := sb.VerifyWebhook(hdr(""), body, now); !errors.Is(err, ErrSignatureMissing) {
		t.Errorf("missing: %v", err)
	}
	tampered := bytes.Replace(body, []byte("20000"), []byte("2000000"), 1)
	if _, err := sb.VerifyWebhook(hdr(sig), tampered, now); !errors.Is(err, ErrBadSignature) {
		t.Errorf("tampered body: %v", err)
	}
	other, _ := NewSandbox(bytes.Repeat([]byte("z"), 32), "", nil)
	_, otherSig := other.BuildWebhook(ev, now)
	if _, err := sb.VerifyWebhook(hdr(otherSig), body, now); !errors.Is(err, ErrBadSignature) {
		t.Errorf("wrong secret: %v", err)
	}
	for _, bad := range []string{"garbage", "t=1", "v1=abcd", "t=x,v1=" + strings.Repeat("0", 64), "t=1,v1=zz"} {
		if _, err := sb.VerifyWebhook(hdr(bad), body, now); !errors.Is(err, ErrBadSignature) {
			t.Errorf("malformed %q: %v", bad, err)
		}
	}
	// Replayed later than the tolerance → stale (dedupe covers replays inside the window).
	if _, err := sb.VerifyWebhook(hdr(sig), body, now.Add(SandboxTolerance+time.Second)); !errors.Is(err, ErrStaleSignature) {
		t.Errorf("replay after window: %v", err)
	}
	if _, err := sb.VerifyWebhook(hdr(sig), body, now.Add(-SandboxTolerance-time.Second)); !errors.Is(err, ErrStaleSignature) {
		t.Errorf("future timestamp: %v", err)
	}
	// A correctly signed but malformed payload is still rejected.
	junk := []byte(`{"eventId":"","orderId":"` + ev.OrderID.String() + `","status":"paid","amountVnd":1}`)
	if _, err := sb.VerifyWebhook(hdr(sb.Sign(junk, now)), junk, now); !errors.Is(err, ErrBadPayload) {
		t.Errorf("empty event id: %v", err)
	}
	weird := []byte(`{"eventId":"e","orderId":"` + ev.OrderID.String() + `","status":"chargeback","amountVnd":1}`)
	if _, err := sb.VerifyWebhook(hdr(sb.Sign(weird, now)), weird, now); !errors.Is(err, ErrBadPayload) {
		t.Errorf("unknown status: %v", err)
	}
	if _, err := NewSandbox([]byte("short"), "", nil); err == nil {
		t.Error("short secret accepted")
	}
}

func TestBuildProviders(t *testing.T) {
	c, err := LoadFile("../../../../content/catalog/payments.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if p, err := BuildProviders(c, "dev", nil, "", nil, nil); err != nil || p[ProviderSandbox] == nil {
		t.Fatalf("dev: %v %v", p, err)
	}
	if _, err := BuildProviders(c, "staging", nil, "", nil, nil); err == nil {
		t.Error("staging without a sandbox secret accepted")
	}
	if p, err := BuildProviders(c, "prod", nil, "", nil, nil); err != nil || len(p) != 0 {
		t.Errorf("prod: %v %v", p, err)
	}
}
