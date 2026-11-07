package payment

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"tiendao/game/internal/db"
)

// SandboxSignatureHeader carries "t=<unix seconds>,v1=<hex HMAC-SHA256(secret, "<t>.<raw body>")>".
const SandboxSignatureHeader = "X-Sandbox-Signature"

// SandboxTolerance is how far a webhook timestamp may be from server time (replay window; dedupe covers the rest).
const SandboxTolerance = 5 * time.Minute

// MinSecretBytes is the minimum sandbox signing secret length.
const MinSecretBytes = 32

// Charge is what the simulated gateway knows about one order.
type Charge struct {
	Status    string // paid | failed | refunded
	AmountVND int64
	EventID   string
}

// ChargeStore is the sandbox gateway's own state (payment_sandbox_charges).
type ChargeStore interface {
	Get(ctx context.Context, orderID uuid.UUID) (Charge, bool, error)
	Put(ctx context.Context, orderID uuid.UUID, c Charge, now time.Time) error
}

// Sandbox is the simulated gateway: HMAC-SHA256 signed webhooks, a dev checkout page, state in ChargeStore.
type Sandbox struct {
	Secret       []byte
	CheckoutBase string // e.g. "http://localhost:8000/tien-duyen/sandbox/" (+ order id)
	Charges      ChargeStore
	Now          func() time.Time // nil → time.Now
}

// NewSandbox validates the secret.
func NewSandbox(secret []byte, checkoutBase string, charges ChargeStore) (*Sandbox, error) {
	if len(secret) < MinSecretBytes {
		return nil, fmt.Errorf("payment: sandbox secret must be ≥ %d bytes", MinSecretBytes)
	}
	return &Sandbox{Secret: secret, CheckoutBase: checkoutBase, Charges: charges}, nil
}

// ID implements Provider.
func (s *Sandbox) ID() string { return ProviderSandbox }

// SandboxRef is the sandbox provider reference of an order.
func SandboxRef(orderID uuid.UUID) string {
	return "sbx_" + strings.ReplaceAll(orderID.String(), "-", "")
}

// CreateCheckout implements Provider (no I/O: the checkout page is our own dev route).
func (s *Sandbox) CreateCheckout(_ context.Context, req CheckoutRequest) (Checkout, error) {
	if req.AmountVND <= 0 {
		return Checkout{}, ErrAmountOutOfBounds
	}
	return Checkout{ProviderRef: SandboxRef(req.OrderID), RedirectURL: s.CheckoutBase + req.OrderID.String()}, nil
}

// sandboxBody is the webhook JSON.
type sandboxBody struct {
	EventID     string    `json:"eventId"`
	OrderID     uuid.UUID `json:"orderId"`
	ProviderRef string    `json:"providerRef"`
	Status      string    `json:"status"`
	AmountVND   int64     `json:"amountVnd"`
	OccurredAt  time.Time `json:"occurredAt"`
}

func (s *Sandbox) mac(ts string, body []byte) []byte {
	m := hmac.New(sha256.New, s.Secret)
	m.Write([]byte(ts))
	m.Write([]byte{'.'})
	m.Write(body)
	return m.Sum(nil)
}

// Sign returns the signature header value for body at time now.
func (s *Sandbox) Sign(body []byte, now time.Time) string {
	ts := strconv.FormatInt(now.Unix(), 10)
	return "t=" + ts + ",v1=" + hex.EncodeToString(s.mac(ts, body))
}

// BuildWebhook builds a correctly signed webhook (dev endpoint, tests).
func (s *Sandbox) BuildWebhook(ev Event, now time.Time) (body []byte, sig string) {
	body, _ = json.Marshal(sandboxBody{EventID: ev.EventID, OrderID: ev.OrderID, ProviderRef: ev.ProviderRef, Status: ev.Kind,
		AmountVND: ev.AmountVND, OccurredAt: ev.OccurredAt.UTC()})
	return body, s.Sign(body, now)
}

// VerifyWebhook implements Provider: signature first (constant time), timestamp window, then parse.
func (s *Sandbox) VerifyWebhook(h http.Header, body []byte, now time.Time) (Event, error) {
	raw := h.Get(SandboxSignatureHeader)
	if raw == "" {
		return Event{}, ErrSignatureMissing
	}
	var ts, v1 string
	for part := range strings.SplitSeq(raw, ",") {
		k, v, _ := strings.Cut(strings.TrimSpace(part), "=")
		switch k {
		case "t":
			ts = v
		case "v1":
			v1 = v
		}
	}
	got, err := hex.DecodeString(v1)
	if ts == "" || err != nil || len(got) != sha256.Size {
		return Event{}, ErrBadSignature
	}
	if !hmac.Equal(got, s.mac(ts, body)) {
		return Event{}, ErrBadSignature
	}
	sec, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return Event{}, ErrBadSignature
	}
	if d := now.Sub(time.Unix(sec, 0)); d > SandboxTolerance || d < -SandboxTolerance {
		return Event{}, ErrStaleSignature
	}
	var b sandboxBody
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&b); err != nil {
		return Event{}, fmt.Errorf("%w: %v", ErrBadPayload, err)
	}
	switch {
	case b.EventID == "" || len(b.EventID) > 128 || b.OrderID == uuid.Nil || b.AmountVND < 0:
		return Event{}, ErrBadPayload
	case b.Status != KindPaid && b.Status != KindFailed && b.Status != KindExpired && b.Status != KindRefunded:
		return Event{}, fmt.Errorf("%w: status %q", ErrBadPayload, b.Status)
	}
	return Event{EventID: b.EventID, OrderID: b.OrderID, ProviderRef: b.ProviderRef, Kind: b.Status, AmountVND: b.AmountVND,
		OccurredAt: b.OccurredAt}, nil
}

// Query implements Provider from the sandbox charge state (no charge yet → pending).
func (s *Sandbox) Query(ctx context.Context, orderID uuid.UUID, providerRef string) (Event, error) {
	ev := Event{OrderID: orderID, ProviderRef: providerRef, Kind: KindPending}
	if s.Charges == nil {
		return ev, nil
	}
	c, ok, err := s.Charges.Get(ctx, orderID)
	if err != nil || !ok {
		return ev, err
	}
	ev.Kind, ev.AmountVND, ev.EventID = c.Status, c.AmountVND, c.EventID
	return ev, nil
}

// Refund implements Provider: the simulated gateway records the refund.
func (s *Sandbox) Refund(ctx context.Context, orderID uuid.UUID, _ string, amountVND int64) error {
	if s.Charges == nil {
		return ErrRefundUnsupported
	}
	return s.Charges.Put(ctx, orderID, Charge{Status: KindRefunded, AmountVND: amountVND, EventID: "sbx_refund_" + orderID.String()}, s.now())
}

func (s *Sandbox) now() time.Time {
	if s.Now == nil {
		return time.Now()
	}
	return s.Now()
}

// PGCharges stores sandbox charges in payment_sandbox_charges.
type PGCharges struct{ Pool *pgxpool.Pool }

// Get implements ChargeStore.
func (p PGCharges) Get(ctx context.Context, orderID uuid.UUID) (Charge, bool, error) {
	c, err := db.New(p.Pool).GetSandboxCharge(ctx, orderID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Charge{}, false, nil
	} else if err != nil {
		return Charge{}, false, err
	}
	return Charge{Status: c.Status, AmountVND: c.AmountVnd, EventID: c.EventID}, true, nil
}

// Put implements ChargeStore.
func (p PGCharges) Put(ctx context.Context, orderID uuid.UUID, c Charge, now time.Time) error {
	return db.New(p.Pool).UpsertSandboxCharge(ctx, db.UpsertSandboxChargeParams{OrderID: orderID, Status: c.Status,
		AmountVnd: c.AmountVND, EventID: c.EventID, Now: now})
}
