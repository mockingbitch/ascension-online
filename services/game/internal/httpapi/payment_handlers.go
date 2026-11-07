package httpapi

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"tiendao/game/internal/payment"
	"tiendao/game/internal/vip"
)

// paymentPackages: GET /payments/packages — public catalogue + providers enabled on this server.
func (d Deps) paymentPackages(w http.ResponseWriter, _ *http.Request) {
	if d.Payments == nil {
		unavailable(w)
		return
	}
	writeJSON(w, 200, d.Payments.Packages())
}

// paymentCreateOrder: POST /payments/orders {packageId, provider?} — intent only; amount/points from the catalogue.
func (d Deps) paymentCreateOrder(w http.ResponseWriter, r *http.Request) {
	if d.Payments == nil {
		unavailable(w)
		return
	}
	key, ok := idempotencyKey(w, r)
	if !ok {
		return
	}
	var body struct {
		PackageID string `json:"packageId"`
		Provider  string `json:"provider"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	res, err := d.Payments.CreateOrder(r.Context(), userFrom(r.Context()), body.PackageID, body.Provider, key)
	switch {
	case errors.Is(err, payment.ErrUnknownPackage):
		writeError(w, 400, "unknown_package", "gói nạp không tồn tại")
	case errors.Is(err, payment.ErrUnknownProvider):
		writeError(w, 409, "topup_closed", "kênh nạp chưa mở")
	case errors.Is(err, payment.ErrAmountOutOfBounds):
		writeError(w, 400, "invalid_amount", "số tiền ngoài giới hạn")
	case errors.Is(err, payment.ErrTooManyOpen):
		writeError(w, 409, "too_many_open_orders", "bạn đang có quá nhiều đơn chưa thanh toán — hãy hoàn tất hoặc chờ đơn cũ hết hạn")
	case errors.Is(err, payment.ErrKeyReuse):
		writeError(w, 409, "idempotency_key_reused", err.Error())
	case errors.Is(err, payment.ErrProviderFailed):
		writeError(w, 502, "provider_unavailable", "cổng thanh toán không phản hồi, thử lại sau")
	case err != nil:
		d.internal(w, r, "create payment order", err)
	default:
		writeJSON(w, 201, res)
	}
}

func pathUUID(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue(name))
	if err != nil {
		writeError(w, 404, "order_not_found", "order not found")
		return uuid.Nil, false
	}
	return id, true
}

// paymentGetOrder: GET /payments/orders/{orderId} — own orders only (another user's id answers 404).
func (d Deps) paymentGetOrder(w http.ResponseWriter, r *http.Request) {
	if d.Payments == nil {
		unavailable(w)
		return
	}
	id, ok := pathUUID(w, r, "orderId")
	if !ok {
		return
	}
	res, err := d.Payments.GetOrder(r.Context(), userFrom(r.Context()), id)
	switch {
	case errors.Is(err, payment.ErrOrderNotFound):
		writeError(w, 404, "order_not_found", "order not found")
	case err != nil:
		d.internal(w, r, "get payment order", err)
	default:
		writeJSON(w, 200, res)
	}
}

// paymentListOrders: GET /payments/orders — own history, newest first (≤ 50).
func (d Deps) paymentListOrders(w http.ResponseWriter, r *http.Request) {
	if d.Payments == nil {
		unavailable(w)
		return
	}
	list, err := d.Payments.ListOrders(r.Context(), userFrom(r.Context()), 50)
	if err != nil {
		d.internal(w, r, "list payment orders", err)
		return
	}
	writeJSON(w, 200, map[string]any{"orders": list})
}

// paymentWebhook: POST /payments/webhook/{provider} — PUBLIC. Signature first (no DB work for a bad one), then the
// idempotent apply. Any non-2xx makes a real provider retry; replays answer 200.
func (d Deps) paymentWebhook(w http.ResponseWriter, r *http.Request) {
	if d.Payments == nil {
		unavailable(w)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, payment.MaxWebhookBytes+1))
	if err != nil || len(body) > payment.MaxWebhookBytes {
		writeError(w, 413, "payload_too_large", "webhook body too large")
		return
	}
	provider := r.PathValue("provider")
	res, err := d.Payments.HandleWebhook(r.Context(), provider, r.Header, body)
	switch {
	case errors.Is(err, payment.ErrUnknownProvider):
		writeError(w, 404, "unknown_provider", "unknown provider")
	case errors.Is(err, payment.ErrSignatureMissing), errors.Is(err, payment.ErrBadSignature), errors.Is(err, payment.ErrStaleSignature):
		d.Log.Warn("payment webhook rejected", "provider", provider, "reason", err.Error())
		writeError(w, 401, "invalid_signature", "signature invalid")
	case errors.Is(err, payment.ErrBadPayload):
		d.Log.Warn("payment webhook bad payload", "provider", provider)
		writeError(w, 400, "invalid_payload", "payload invalid")
	case err != nil:
		d.internal(w, r, "payment webhook", err)
	default:
		writeJSON(w, 200, res)
	}
}

// ───── admin ─────

// adminPaymentOrders: GET /admin/payments/orders?status=&userId=&q=&limit= — q is an order id or a provider ref.
func (d Deps) adminPaymentOrders(w http.ResponseWriter, r *http.Request) {
	if d.Payments == nil {
		unavailable(w)
		return
	}
	qs := r.URL.Query()
	f := payment.AdminFilter{Status: qs.Get("status"), Limit: 50}
	if n, err := strconv.Atoi(qs.Get("limit")); err == nil && n > 0 && n <= 200 {
		f.Limit = int32(n)
	}
	if v := qs.Get("userId"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			writeError(w, 400, "invalid_user_id", "userId must be a UUID")
			return
		}
		f.UserID = id
	}
	if v := strings.TrimSpace(qs.Get("q")); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			f.OrderID = id
		} else {
			f.ProviderRef = v
		}
	}
	list, err := d.Payments.AdminList(r.Context(), f)
	if err != nil {
		d.internal(w, r, "admin payment orders", err)
		return
	}
	writeJSON(w, 200, map[string]any{"orders": list})
}

// adminPaymentOrder: GET /admin/payments/orders/{orderId} — order + provider events.
func (d Deps) adminPaymentOrder(w http.ResponseWriter, r *http.Request) {
	if d.Payments == nil {
		unavailable(w)
		return
	}
	id, ok := pathUUID(w, r, "orderId")
	if !ok {
		return
	}
	res, err := d.Payments.AdminGet(r.Context(), id)
	switch {
	case errors.Is(err, payment.ErrOrderNotFound):
		writeError(w, 404, "order_not_found", "order not found")
	case err != nil:
		d.internal(w, r, "admin payment order", err)
	default:
		writeJSON(w, 200, res)
	}
}

// adminPaymentAlerts: GET /admin/payments/alerts — rejected / ignored provider events (amount mismatch, unknown
// order, provider refund, out-of-order), newest first.
func (d Deps) adminPaymentAlerts(w http.ResponseWriter, r *http.Request) {
	if d.Payments == nil {
		unavailable(w)
		return
	}
	list, err := d.Payments.Alerts(r.Context(), 100)
	if err != nil {
		d.internal(w, r, "admin payment alerts", err)
		return
	}
	writeJSON(w, 200, map[string]any{"events": list})
}

// adminPaymentRefund: POST /admin/payments/orders/{orderId}/refund {reason} — Idempotency-Key required (the order is
// the anchor: any retry is one refund). Reverses the purchase row via vip; audited.
func (d Deps) adminPaymentRefund(w http.ResponseWriter, r *http.Request) {
	if d.Payments == nil {
		unavailable(w)
		return
	}
	if _, ok := idempotencyKey(w, r); !ok {
		return
	}
	id, ok := pathUUID(w, r, "orderId")
	if !ok {
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if strings.TrimSpace(body.Reason) == "" || len(body.Reason) > 500 {
		writeError(w, 400, "invalid_refund", "a reason (≤ 500 chars) is required")
		return
	}
	res, err := d.Payments.Refund(r.Context(), userFrom(r.Context()), id, body.Reason)
	switch {
	case errors.Is(err, payment.ErrOrderNotFound):
		writeError(w, 404, "order_not_found", "order not found")
	case errors.Is(err, payment.ErrNotRefundable):
		writeError(w, 409, "not_refundable", "only paid orders can be refunded")
	case errors.Is(err, payment.ErrProviderFailed):
		writeError(w, 502, "provider_unavailable", "provider refund failed")
	case errors.Is(err, vip.ErrInvalidGrant):
		writeError(w, 409, "insufficient_points", "balance no longer covers the refund (manual decision needed)")
	case errors.Is(err, vip.ErrNotRefundable), errors.Is(err, vip.ErrLedgerNotFound), errors.Is(err, vip.ErrKeyReuse):
		writeError(w, 409, "ledger_inconsistent", err.Error())
	case err != nil:
		d.internal(w, r, "admin payment refund", err)
	default:
		writeJSON(w, 200, res)
	}
}

// ───── dev only (registered only when APP_ENV=dev) ─────

// devSandboxComplete: POST /dev/payments/sandbox/{orderId} {outcome: paid|failed, deliver?: bool, amountVnd?: int}
// — the simulated gateway's "pay now / fail" button. deliver defaults to true; false simulates a lost webhook.
func (d Deps) devSandboxComplete(w http.ResponseWriter, r *http.Request) {
	if d.Payments == nil {
		unavailable(w)
		return
	}
	id, ok := pathUUID(w, r, "orderId")
	if !ok {
		return
	}
	var body struct {
		Outcome   string `json:"outcome"`
		Deliver   *bool  `json:"deliver"`
		AmountVND int64  `json:"amountVnd"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	deliver := body.Deliver == nil || *body.Deliver
	p := principalFrom(r.Context())
	res, err := d.Payments.SandboxComplete(r.Context(), p.UserID, p.IsAdmin(), id, body.Outcome, deliver, body.AmountVND)
	switch {
	case errors.Is(err, payment.ErrOrderNotFound):
		writeError(w, 404, "order_not_found", "order not found")
	case errors.Is(err, payment.ErrUnknownProvider):
		writeError(w, 409, "not_sandbox", "order is not a sandbox order")
	case errors.Is(err, payment.ErrBadPayload):
		writeError(w, 400, "invalid_outcome", "outcome must be paid or failed")
	case err != nil:
		d.internal(w, r, "sandbox complete", err)
	default:
		writeJSON(w, 200, res)
	}
}
