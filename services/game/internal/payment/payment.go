package payment

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// Order statuses (payment_orders.status).
const (
	StatusCreated  = "created"  // row written, provider checkout not yet created
	StatusPending  = "pending"  // checkout created, waiting for the provider
	StatusPaid     = "paid"     // verified payment, điểm duyên credited (terminal except refund)
	StatusFailed   = "failed"   // provider said failed/cancelled, or the checkout could not be created
	StatusExpired  = "expired"  // unpaid past expires_at (worker) or provider said expired
	StatusRefunded = "refunded" // admin refund, purchase row reversed (terminal)
)

// Normalized event kinds a provider reports (payment_events.kind).
const (
	KindPaid     = "paid"
	KindFailed   = "failed"
	KindExpired  = "expired"
	KindRefunded = "refunded" // provider-side refund/chargeback: recorded + alerted, refunded by an admin
	KindPending  = "pending"  // Query only: nothing final yet
)

// ProviderSandbox is the simulated gateway's id.
const ProviderSandbox = "sandbox"

// CanTransition is the order status machine (mirrored by the 000016 trigger):
// created → pending|paid|failed|expired; pending → paid|failed|expired; failed|expired → paid (a verified late
// payment: the money was received, it is honoured); paid → refunded. Same status is not a transition.
func CanTransition(from, to string) bool {
	switch from {
	case StatusCreated:
		return to == StatusPending || to == StatusPaid || to == StatusFailed || to == StatusExpired
	case StatusPending:
		return to == StatusPaid || to == StatusFailed || to == StatusExpired
	case StatusFailed, StatusExpired:
		return to == StatusPaid
	case StatusPaid:
		return to == StatusRefunded
	}
	return false
}

// Open reports whether an order still waits for a payment.
func Open(status string) bool { return status == StatusCreated || status == StatusPending }

// ───── provider adapter contract ─────

// CheckoutRequest is what the service asks a provider to charge. Amount comes from the order row (catalogue), never
// from the client.
type CheckoutRequest struct {
	OrderID     uuid.UUID
	AmountVND   int64
	Description string // shown by the provider (ASCII-safe; some gateways limit it)
	ReturnURL   string // browser comes back here; it NEVER credits anything
	ExpiresAt   time.Time
}

// Checkout is the provider's answer: where to send the player.
type Checkout struct {
	ProviderRef string // provider's id for this checkout (unique per provider)
	RedirectURL string // hosted payment page
	QRData      string // optional VietQR / EMVCo payload
}

// Event is a normalized provider event (webhook or query result).
type Event struct {
	EventID     string    // provider event id: the dedupe key (payment_events UNIQUE)
	OrderID     uuid.UUID // our order id as echoed by the provider
	ProviderRef string
	Kind        string // KindPaid | KindFailed | KindExpired | KindRefunded | KindPending
	AmountVND   int64  // amount the provider says was paid (checked against the order)
	OccurredAt  time.Time
}

// Provider is one payment gateway adapter. A real adapter (PayOS / VNPay / MoMo) must:
//   - CreateCheckout: create the hosted checkout for exactly req.AmountVND, binding req.OrderID so webhooks/queries
//     echo it; be idempotent per order id (a retried create returns the same checkout or a provider error).
//   - VerifyWebhook: verify the provider signature over the RAW body (constant-time compare), reject stale
//     timestamps where the provider sends one, and only then parse; return ErrSignatureMissing / ErrBadSignature /
//     ErrStaleSignature / ErrBadPayload. It must not do I/O (it runs before anything else on a public route).
//   - Query: ask the provider for the order's current state (reconciliation); KindPending when nothing is final.
//   - Refund: refund through the provider when supported, idempotent per order; else ErrRefundUnsupported.
type Provider interface {
	ID() string
	CreateCheckout(ctx context.Context, req CheckoutRequest) (Checkout, error)
	VerifyWebhook(h http.Header, body []byte, now time.Time) (Event, error)
	Query(ctx context.Context, orderID uuid.UUID, providerRef string) (Event, error)
	Refund(ctx context.Context, orderID uuid.UUID, providerRef string, amountVND int64) error
}

// Webhook verification errors (HTTP 401/400, nothing written).
var (
	ErrSignatureMissing  = errors.New("payment: webhook signature missing")
	ErrBadSignature      = errors.New("payment: webhook signature invalid")
	ErrStaleSignature    = errors.New("payment: webhook timestamp outside tolerance")
	ErrBadPayload        = errors.New("payment: webhook payload invalid")
	ErrRefundUnsupported = errors.New("payment: provider refund not supported")
)

// Service errors.
var (
	ErrUnknownPackage    = errors.New("payment: unknown package")
	ErrUnknownProvider   = errors.New("payment: unknown or disabled provider")
	ErrTooManyOpen       = errors.New("payment: too many open orders")
	ErrKeyReuse          = errors.New("payment: idempotency key reused with a different package")
	ErrOrderNotFound     = errors.New("payment: order not found")
	ErrNotRefundable     = errors.New("payment: order is not paid")
	ErrProviderFailed    = errors.New("payment: provider checkout failed")
	ErrAmountOutOfBounds = errors.New("payment: amount outside catalogue bounds")
)

// Outcomes of applying one event (payment_events.outcome).
const (
	OutcomeApplied  = "applied"
	OutcomeReplayed = "replayed"
	OutcomeIgnored  = "ignored"
	OutcomeRejected = "rejected"
)

// Decision is what apply() does with an event for an order in a given state (pure).
type Decision struct {
	Outcome string // applied | replayed | ignored | rejected
	To      string // target status when applied
	Detail  string // reason for ignored/rejected, or "late_payment"
	Alert   bool   // log at error level for an operator
}

// Decide is the pure event rule. order fields: status, amount, points; expectPoints = vip rate now for the amount.
//   - paid: on paid/refunded → replayed (exactly once) — but a NEW webhook event id on a settled order means the
//     provider reports a second payment (e.g. the player paid twice): ignored + alert "duplicate_payment" (review M3;
//     same-id redeliveries never get here, dedupe catches them); amount ≠ order or points ≠ promised → rejected + alert;
//     else applied (from failed/expired: late payment, alerted so support knows).
//   - failed/expired: on an open order → applied; otherwise ignored (e.g. failed after paid: out of order).
//   - refunded (provider side): ignored + alert — an admin refunds through POST /admin/payments/orders/{id}/refund.
//   - pending: ignored (nothing final).
func Decide(status string, orderAmount, orderPoints int64, ev Event, expectPoints int64, webhook bool) Decision {
	switch ev.Kind {
	case KindPaid:
		switch {
		case (status == StatusPaid || status == StatusRefunded) && webhook:
			return Decision{Outcome: OutcomeIgnored, Detail: "duplicate_payment", Alert: true}
		case status == StatusPaid || status == StatusRefunded:
			return Decision{Outcome: OutcomeReplayed, Detail: "already_settled"}
		case ev.AmountVND != orderAmount:
			return Decision{Outcome: OutcomeRejected, Detail: "amount_mismatch", Alert: true}
		case expectPoints != orderPoints:
			return Decision{Outcome: OutcomeRejected, Detail: "points_mismatch", Alert: true}
		case status == StatusFailed || status == StatusExpired:
			return Decision{Outcome: OutcomeApplied, To: StatusPaid, Detail: "late_payment", Alert: true}
		default:
			return Decision{Outcome: OutcomeApplied, To: StatusPaid}
		}
	case KindFailed, KindExpired:
		to := StatusFailed
		if ev.Kind == KindExpired {
			to = StatusExpired
		}
		if Open(status) {
			return Decision{Outcome: OutcomeApplied, To: to}
		}
		if status == to {
			return Decision{Outcome: OutcomeReplayed, Detail: "already_closed"}
		}
		return Decision{Outcome: OutcomeIgnored, Detail: "out_of_order_" + ev.Kind + "_after_" + status}
	case KindRefunded:
		return Decision{Outcome: OutcomeIgnored, Detail: "provider_refund_needs_admin", Alert: true}
	}
	return Decision{Outcome: OutcomeIgnored, Detail: "not_final"}
}
