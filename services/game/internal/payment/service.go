package payment

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"tiendao/game/internal/db"
	"tiendao/game/internal/platform/postgres"
	"tiendao/game/internal/vip"
)

// providerTimeout bounds every outbound provider call (never inside a transaction).
const providerTimeout = 10 * time.Second

// MaxWebhookBytes bounds a stored webhook body (the handler reads at most this much).
const MaxWebhookBytes = 16 << 10

// Service owns payment orders. Lock order: payment_orders (FOR UPDATE) → vip_accounts → character_appearance.
type Service struct {
	Pool      *pgxpool.Pool
	Cat       *Catalogue
	VIP       *vip.Table
	Providers map[string]Provider // enabled for this environment only
	ReturnURL string              // browser return page (+ order id); display only, never credits
	Log       *slog.Logger
	Now       func() time.Time
}

func (s *Service) now() time.Time {
	if s.Now == nil {
		return time.Now()
	}
	return s.Now()
}

func (s *Service) log() *slog.Logger {
	if s.Log == nil {
		return slog.Default()
	}
	return s.Log
}

// ───── views ─────

// CheckoutView is where the player pays (only while the order is open).
type CheckoutView struct {
	URL string `json:"url"`
	QR  string `json:"qr,omitempty"`
}

// OrderView is a player's order.
type OrderView struct {
	ID            uuid.UUID     `json:"id"`
	PackageID     string        `json:"packageId"`
	PackageName   string        `json:"packageName"`
	AmountVND     int64         `json:"amountVnd"`
	Points        int64         `json:"points"`
	Provider      string        `json:"provider"`
	Status        string        `json:"status"`
	Checkout      *CheckoutView `json:"checkout,omitempty"`
	FailureReason string        `json:"failureReason,omitempty"`
	CreatedAt     time.Time     `json:"createdAt"`
	ExpiresAt     time.Time     `json:"expiresAt"`
	PaidAt        *time.Time    `json:"paidAt,omitempty"`
	ClosedAt      *time.Time    `json:"closedAt,omitempty"`
	RefundedAt    *time.Time    `json:"refundedAt,omitempty"`
}

// AdminOrderView adds the operator fields.
type AdminOrderView struct {
	OrderView
	UserID         uuid.UUID  `json:"userId"`
	ProviderRef    string     `json:"providerRef,omitempty"`
	VIPLedgerID    int64      `json:"vipLedgerId,omitempty"`
	RefundLedgerID int64      `json:"refundLedgerId,omitempty"`
	CheckCount     int32      `json:"checkCount"`
	LastCheckedAt  *time.Time `json:"lastCheckedAt,omitempty"`
}

// EventView is one stored provider event (admin).
type EventView struct {
	ID          int64      `json:"id"`
	Provider    string     `json:"provider"`
	DedupeKey   string     `json:"dedupeKey"`
	Source      string     `json:"source"`
	SignatureOK bool       `json:"signatureOk"`
	OrderID     *uuid.UUID `json:"orderId,omitempty"`
	Kind        string     `json:"kind"`
	AmountVND   *int64     `json:"amountVnd,omitempty"`
	Outcome     string     `json:"outcome"`
	Detail      string     `json:"detail,omitempty"`
	ReceivedAt  time.Time  `json:"receivedAt"`
}

// OrderResult is the response of order reads and creation.
type OrderResult struct {
	Order    OrderView   `json:"order"`
	VIP      *vip.Status `json:"vip,omitempty"`
	Replayed bool        `json:"replayed,omitempty"`
}

func (s *Service) view(o db.PaymentOrder) OrderView {
	v := OrderView{ID: o.ID, PackageID: o.PackageID, PackageName: o.PackageID, AmountVND: o.AmountVnd, Points: o.Points,
		Provider: o.Provider, Status: o.Status, FailureReason: o.FailureReason.String, CreatedAt: o.CreatedAt,
		ExpiresAt: o.ExpiresAt, PaidAt: o.PaidAt, ClosedAt: o.ClosedAt, RefundedAt: o.RefundedAt}
	if p, ok := s.Cat.Package(o.PackageID); ok {
		v.PackageName = p.Name
	}
	if Open(o.Status) && o.CheckoutUrl.Valid && o.ExpiresAt.After(s.now()) {
		v.Checkout = &CheckoutView{URL: o.CheckoutUrl.String, QR: o.CheckoutQr.String}
	}
	return v
}

func (s *Service) adminView(o db.PaymentOrder) AdminOrderView {
	return AdminOrderView{OrderView: s.view(o), UserID: o.UserID, ProviderRef: o.ProviderRef.String, VIPLedgerID: o.VipLedgerID.Int64,
		RefundLedgerID: o.RefundLedgerID.Int64, CheckCount: o.CheckCount, LastCheckedAt: o.LastCheckedAt}
}

func eventView(e db.PaymentEvent) EventView {
	v := EventView{ID: e.ID, Provider: e.Provider, DedupeKey: e.DedupeKey, Source: e.Source, SignatureOK: e.SignatureOk, Kind: e.Kind,
		Outcome: e.Outcome, Detail: e.Detail.String, ReceivedAt: e.ReceivedAt}
	if e.OrderID.Valid {
		id := uuid.UUID(e.OrderID.Bytes)
		v.OrderID = &id
	}
	if e.AmountVnd.Valid {
		v.AmountVND = &e.AmountVnd.Int64
	}
	return v
}

// PackagesView is GET /payments/packages.
type PackagesView struct {
	Currency           string        `json:"currency"`
	PointName          string        `json:"pointName"`
	PointsPer1000VND   int64         `json:"pointsPer1000Vnd"`
	MinAmountVND       int64         `json:"minAmountVnd"`
	MaxAmountVND       int64         `json:"maxAmountVnd"`
	OrderExpiryMinutes int64         `json:"orderExpiryMinutes"`
	Packages           []Package     `json:"packages"`
	Providers          []ProviderDef `json:"providers"` // enabled here; empty → top-up closed
}

// Packages lists the catalogue and the providers enabled on this server.
func (s *Service) Packages() PackagesView {
	v := PackagesView{Currency: s.Cat.Currency, PointName: s.VIP.PointName, PointsPer1000VND: s.VIP.PointsPer1000VND,
		MinAmountVND: s.Cat.MinAmountVND, MaxAmountVND: s.Cat.MaxAmountVND, OrderExpiryMinutes: s.Cat.OrderExpiryMinutes,
		Packages: s.Cat.Packages, Providers: []ProviderDef{}}
	for _, p := range s.Cat.Providers {
		if _, ok := s.Providers[p.ID]; ok {
			v.Providers = append(v.Providers, p)
		}
	}
	return v
}

func (s *Service) defaultProvider() string {
	for _, p := range s.Cat.Providers {
		if _, ok := s.Providers[p.ID]; ok {
			return p.ID
		}
	}
	return ""
}

func tsPtr(t time.Time) *time.Time { return &t }

func text(v string) pgtype.Text { return pgtype.Text{String: v, Valid: v != ""} }

// ───── create order ─────

// CreateOrder opens an order for a catalogue package (intent only: amount and points come from the catalogue).
// Idempotent per (user, key): the same key returns the same order; another package under that key → ErrKeyReuse.
// The provider checkout is created after the insert commits (no network call inside the tx); a crash in between
// leaves a 'created' order that a retry with the same key completes.
func (s *Service) CreateOrder(ctx context.Context, userID uuid.UUID, packageID, providerID, idemKey string) (OrderResult, error) {
	if providerID == "" {
		providerID = s.defaultProvider()
	}
	prov, ok := s.Providers[providerID]
	if !ok {
		return OrderResult{}, ErrUnknownProvider
	}
	pkg, ok := s.Cat.Package(packageID)
	if !ok {
		return OrderResult{}, ErrUnknownPackage
	}
	if pkg.AmountVND < s.Cat.MinAmountVND || pkg.AmountVND > s.Cat.MaxAmountVND || pkg.Points != s.VIP.PointsForVND(pkg.AmountVND) {
		return OrderResult{}, ErrAmountOutOfBounds
	}
	now := s.now()
	var o db.PaymentOrder
	replayed := false
	err := postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		if err := q.LockPaymentUser(ctx, userID.String()); err != nil {
			return fmt.Errorf("lock payment user: %w", err)
		}
		prev, err := q.GetPaymentOrderByKey(ctx, db.GetPaymentOrderByKeyParams{UserID: userID, IdempotencyKey: idemKey})
		if err == nil {
			if prev.PackageID != packageID || prev.Provider != providerID {
				return ErrKeyReuse
			}
			o, replayed = prev, true
			return nil
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		n, err := q.CountOpenPaymentOrders(ctx, db.CountOpenPaymentOrdersParams{UserID: userID, Now: now})
		if err != nil {
			return err
		}
		if n >= s.Cat.MaxOpenOrdersPerUser {
			return ErrTooManyOpen
		}
		o, err = q.InsertPaymentOrder(ctx, db.InsertPaymentOrderParams{UserID: userID, PackageID: pkg.ID, AmountVnd: pkg.AmountVND,
			Points: pkg.Points, Provider: providerID, IdempotencyKey: idemKey, Now: now, ExpiresAt: now.Add(s.Cat.OrderExpiry())})
		return err
	})
	if err != nil {
		return OrderResult{}, err
	}
	if o.Status == StatusCreated && !o.CheckoutUrl.Valid {
		if o, err = s.checkout(ctx, prov, o); err != nil {
			return OrderResult{}, err
		}
	}
	return OrderResult{Order: s.view(o), Replayed: replayed}, nil
}

// checkoutClaimStale: a checkout claim older than this (crashed caller) may be taken over by a retry.
const checkoutClaimStale = 2 * providerTimeout

// checkout creates the provider checkout once (review M2): the caller first claims the fresh order row (version 0 → 1), so
// concurrent retries of the same key never create two checkouts; a loser returns the order as is (the client polls).
// A provider error fails the order only while no checkout was ever recorded.
func (s *Service) checkout(ctx context.Context, prov Provider, o db.PaymentOrder) (db.PaymentOrder, error) {
	q := db.New(s.Pool)
	now := s.now()
	claimed, err := q.ClaimPaymentCheckout(ctx, db.ClaimPaymentCheckoutParams{Now: now, ID: o.ID, Stale: now.Add(-checkoutClaimStale)})
	if errors.Is(err, pgx.ErrNoRows) {
		return q.GetPaymentOrder(ctx, o.ID) // another request is creating it (or it is done)
	} else if err != nil {
		return o, err
	}
	cctx, cancel := context.WithTimeout(ctx, providerTimeout)
	co, cerr := prov.CreateCheckout(cctx, CheckoutRequest{OrderID: o.ID, AmountVND: o.AmountVnd,
		Description: "TienDao " + o.PackageID, ReturnURL: s.ReturnURL + o.ID.String(), ExpiresAt: o.ExpiresAt})
	cancel()
	now = s.now()
	if cerr != nil {
		s.log().Error("payment checkout failed", "provider", prov.ID(), "orderId", o.ID, "err", cerr)
		if _, err := q.MarkPaymentCheckoutFailed(ctx, db.MarkPaymentCheckoutFailedParams{Now: tsPtr(now), ID: o.ID}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return claimed, err
		}
		return claimed, ErrProviderFailed
	}
	out, err := q.SetPaymentCheckout(ctx, db.SetPaymentCheckoutParams{ProviderRef: text(co.ProviderRef), CheckoutUrl: text(co.RedirectURL),
		CheckoutQr: text(co.QRData), Now: now, ID: o.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		return q.GetPaymentOrder(ctx, o.ID)
	}
	return out, err
}

// ───── reads ─────

// GetOrder returns one of the user's own orders (another user's id → ErrOrderNotFound) with the current Tiên Duyên.
func (s *Service) GetOrder(ctx context.Context, userID, orderID uuid.UUID) (OrderResult, error) {
	q := db.New(s.Pool)
	o, err := q.GetPaymentOrder(ctx, orderID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && o.UserID != userID) {
		return OrderResult{}, ErrOrderNotFound
	} else if err != nil {
		return OrderResult{}, err
	}
	st, err := s.VIP.StatusTx(ctx, q, userID, s.now())
	if err != nil {
		return OrderResult{}, err
	}
	return OrderResult{Order: s.view(o), VIP: &st}, nil
}

// ListOrders is the user's own history, newest first.
func (s *Service) ListOrders(ctx context.Context, userID uuid.UUID, limit int32) ([]OrderView, error) {
	rows, err := db.New(s.Pool).ListUserPaymentOrders(ctx, db.ListUserPaymentOrdersParams{UserID: userID, Lim: limit})
	if err != nil {
		return nil, err
	}
	out := make([]OrderView, 0, len(rows))
	for _, o := range rows {
		out = append(out, s.view(o))
	}
	return out, nil
}

// AdminFilter narrows the admin order list (zero values = any).
type AdminFilter struct {
	Status      string
	UserID      uuid.UUID
	ProviderRef string
	OrderID     uuid.UUID
	Limit       int32
}

// AdminList searches orders.
func (s *Service) AdminList(ctx context.Context, f AdminFilter) ([]AdminOrderView, error) {
	q := db.New(s.Pool)
	out := []AdminOrderView{}
	if f.OrderID != uuid.Nil {
		o, err := q.GetPaymentOrder(ctx, f.OrderID)
		if errors.Is(err, pgx.ErrNoRows) {
			return out, nil
		} else if err != nil {
			return nil, err
		}
		return append(out, s.adminView(o)), nil
	}
	p := db.AdminListPaymentOrdersParams{Status: text(f.Status), ProviderRef: text(f.ProviderRef), Lim: f.Limit}
	if f.UserID != uuid.Nil {
		p.UserID = pgtype.UUID{Bytes: f.UserID, Valid: true}
	}
	rows, err := q.AdminListPaymentOrders(ctx, p)
	if err != nil {
		return nil, err
	}
	for _, o := range rows {
		out = append(out, s.adminView(o))
	}
	return out, nil
}

// AdminOrder is one order with its provider events.
type AdminOrder struct {
	Order  AdminOrderView `json:"order"`
	Events []EventView    `json:"events"`
}

// AdminGet returns an order and its events.
func (s *Service) AdminGet(ctx context.Context, orderID uuid.UUID) (AdminOrder, error) {
	q := db.New(s.Pool)
	o, err := q.GetPaymentOrder(ctx, orderID)
	if errors.Is(err, pgx.ErrNoRows) {
		return AdminOrder{}, ErrOrderNotFound
	} else if err != nil {
		return AdminOrder{}, err
	}
	evs, err := q.ListPaymentEventsForOrder(ctx, pgtype.UUID{Bytes: orderID, Valid: true})
	if err != nil {
		return AdminOrder{}, err
	}
	out := AdminOrder{Order: s.adminView(o), Events: make([]EventView, 0, len(evs))}
	for _, e := range evs {
		out.Events = append(out.Events, eventView(e))
	}
	return out, nil
}

// Alerts lists rejected / ignored provider events, newest first.
func (s *Service) Alerts(ctx context.Context, limit int32) ([]EventView, error) {
	rows, err := db.New(s.Pool).ListPaymentAlerts(ctx, limit)
	if err != nil {
		return nil, err
	}
	out := make([]EventView, 0, len(rows))
	for _, e := range rows {
		out = append(out, eventView(e))
	}
	return out, nil
}

// ───── webhook + settlement ─────

// WebhookResult is what applying one provider event did.
type WebhookResult struct {
	Outcome string    `json:"outcome"`
	Detail  string    `json:"detail,omitempty"`
	OrderID uuid.UUID `json:"orderId"`
	Status  string    `json:"status,omitempty"` // order status after the event
}

// HandleWebhook verifies the provider signature BEFORE anything else (no DB access for a bad signature), then
// applies the normalized event. Replays (same provider event id) are no-ops.
func (s *Service) HandleWebhook(ctx context.Context, providerID string, h http.Header, body []byte) (WebhookResult, error) {
	prov, ok := s.Providers[providerID]
	if !ok {
		return WebhookResult{}, ErrUnknownProvider
	}
	ev, err := prov.VerifyWebhook(h, body, s.now())
	if err != nil {
		return WebhookResult{}, err
	}
	return s.apply(ctx, providerID, ev, "webhook", true, string(body))
}

// apply settles one verified event in ONE transaction: dedupe row → lock order → Decide → (paid) vip.TopupLocked
// anchored on the order + mark paid. Concurrent webhook and reconciliation for the same order serialize on the order
// row; the second sees 'paid' and replays, and TopupLocked is itself order-anchored (exactly-once credit).
func (s *Service) apply(ctx context.Context, providerID string, ev Event, source string, sigOK bool, payload string) (WebhookResult, error) {
	dedupe := ev.EventID
	if dedupe == "" {
		dedupe = fmt.Sprintf("%s:%s:%s", source, ev.OrderID, ev.Kind)
	}
	if len(payload) > MaxWebhookBytes {
		payload = payload[:MaxWebhookBytes]
	}
	res := WebhookResult{OrderID: ev.OrderID}
	var dec Decision
	var userID uuid.UUID
	err := postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		now := s.now()
		evID, err := q.InsertPaymentEvent(ctx, db.InsertPaymentEventParams{Provider: providerID, DedupeKey: dedupe, Source: source,
			SignatureOk: sigOK, Kind: ev.Kind, AmountVnd: pgtype.Int8{Int64: ev.AmountVND, Valid: true},
			ProviderRef: text(ev.ProviderRef), Payload: payload, Now: now})
		if errors.Is(err, pgx.ErrNoRows) {
			dec = Decision{Outcome: OutcomeReplayed, Detail: "duplicate_event"}
			if o, err := q.GetPaymentOrder(ctx, ev.OrderID); err == nil {
				res.Status = o.Status
			}
			return nil
		} else if err != nil {
			return fmt.Errorf("insert payment event: %w", err)
		}
		finish := func(orderID pgtype.UUID) error {
			return q.FinishPaymentEvent(ctx, db.FinishPaymentEventParams{Outcome: dec.Outcome, Detail: text(dec.Detail), OrderID: orderID,
				Now: tsPtr(now), ID: evID})
		}
		o, err := q.GetPaymentOrderForUpdate(ctx, ev.OrderID)
		if errors.Is(err, pgx.ErrNoRows) {
			dec = Decision{Outcome: OutcomeRejected, Detail: "unknown_order", Alert: true}
			return finish(pgtype.UUID{})
		} else if err != nil {
			return err
		}
		userID = o.UserID
		oid := pgtype.UUID{Bytes: o.ID, Valid: true}
		switch {
		case o.Provider != providerID:
			dec = Decision{Outcome: OutcomeRejected, Detail: "provider_mismatch", Alert: true}
		case ev.ProviderRef != "" && o.ProviderRef.Valid && o.ProviderRef.String != ev.ProviderRef:
			dec = Decision{Outcome: OutcomeRejected, Detail: "provider_ref_mismatch", Alert: true}
		default:
			dec = Decide(o.Status, o.AmountVnd, o.Points, ev, s.VIP.PointsForVND(o.AmountVnd), source == "webhook")
		}
		res.Status = o.Status
		if dec.Outcome == OutcomeApplied {
			switch dec.To {
			case StatusPaid:
				if _, _, err := s.VIP.TopupLocked(ctx, q, o.UserID, o.AmountVnd, o.ID.String(), now); errors.Is(err, vip.ErrOrderConflict) {
					dec = Decision{Outcome: OutcomeRejected, Detail: "ledger_conflict", Alert: true}
					return finish(oid)
				} else if err != nil {
					return fmt.Errorf("topup: %w", err)
				}
				row, err := vip.PurchaseByOrder(ctx, q, o.ID.String())
				if err != nil {
					return fmt.Errorf("purchase row: %w", err)
				}
				if o, err = q.MarkPaymentPaid(ctx, db.MarkPaymentPaidParams{Now: tsPtr(now), VipLedgerID: pgtype.Int8{Int64: row.ID, Valid: true},
					ProviderRef: text(ev.ProviderRef), ID: o.ID}); err != nil {
					return fmt.Errorf("mark paid: %w", err)
				}
			default:
				if o, err = q.MarkPaymentClosed(ctx, db.MarkPaymentClosedParams{Status: dec.To, Now: tsPtr(now),
					Reason: text("provider_" + ev.Kind), ID: o.ID}); err != nil {
					return fmt.Errorf("mark %s: %w", dec.To, err)
				}
			}
			res.Status = o.Status
		}
		return finish(oid)
	})
	if err != nil {
		return WebhookResult{}, err
	}
	res.Outcome, res.Detail = dec.Outcome, dec.Detail
	if dec.Alert {
		// Operator alert (admin: GET /admin/payments/alerts). Never logs payloads or secrets.
		s.log().Error("payment alert", "detail", dec.Detail, "outcome", dec.Outcome, "provider", providerID, "source", source,
			"orderId", ev.OrderID, "userId", userID, "eventAmountVnd", ev.AmountVND)
	}
	return res, nil
}

// ───── worker jobs ─────

// Expire closes open orders past expires_at (one bounded batch; SKIP LOCKED).
func (s *Service) Expire(ctx context.Context) (int, error) {
	ids, err := db.New(s.Pool).ExpirePaymentOrders(ctx, db.ExpirePaymentOrdersParams{Now: tsPtr(s.now()), Batch: s.Cat.SweepBatch})
	return len(ids), err
}

// ReconcileResult counts one reconciliation pass.
type ReconcileResult struct {
	Checked, Settled, Failed int
}

// Reconcile asks the provider about pending orders older than reconcile_after (and orders expired within
// reconcile_late) and settles final answers through apply — the same exactly-once path as the webhook.
func (s *Service) Reconcile(ctx context.Context) (ReconcileResult, error) {
	var out ReconcileResult
	now := s.now()
	q := db.New(s.Pool)
	rows, err := q.ListReconcileCandidates(ctx, db.ListReconcileCandidatesParams{OlderThan: now.Add(-s.Cat.ReconcileAfter()),
		LateSince: tsPtr(now.Add(-s.Cat.ReconcileLate())), CheckedBefore: tsPtr(now.Add(-s.Cat.ReconcileEvery())), Batch: s.Cat.SweepBatch})
	if err != nil {
		return out, err
	}
	for _, o := range rows {
		if ctx.Err() != nil {
			return out, ctx.Err()
		}
		prov, ok := s.Providers[o.Provider]
		if !ok {
			continue
		}
		if err := q.TouchPaymentChecked(ctx, db.TouchPaymentCheckedParams{Now: tsPtr(now), ID: o.ID}); err != nil {
			return out, err
		}
		out.Checked++
		qctx, cancel := context.WithTimeout(ctx, providerTimeout)
		ev, err := prov.Query(qctx, o.ID, o.ProviderRef.String)
		cancel()
		if err != nil {
			out.Failed++
			s.log().Warn("payment reconcile query", "provider", o.Provider, "orderId", o.ID, "err", err)
			continue
		}
		if ev.Kind == KindPending {
			continue
		}
		if ev.AmountVND < 0 || len(ev.EventID) > 128 || (ev.Kind != KindPaid && ev.Kind != KindFailed && ev.Kind != KindExpired && ev.Kind != KindRefunded) {
			out.Failed++
			s.log().Error("payment reconcile: adapter returned an invalid result", "provider", o.Provider, "orderId", o.ID, "kind", ev.Kind)
			continue
		}
		ev.OrderID = o.ID // the order we asked about, whatever the provider echoes
		payload, _ := json.Marshal(map[string]any{"query": true, "eventId": ev.EventID, "kind": ev.Kind, "amountVnd": ev.AmountVND,
			"providerRef": ev.ProviderRef})
		res, err := s.apply(ctx, o.Provider, ev, "reconcile", false, string(payload))
		if err != nil {
			out.Failed++
			s.log().Error("payment reconcile apply", "orderId", o.ID, "err", err)
			continue
		}
		if res.Outcome == OutcomeApplied {
			out.Settled++
		}
	}
	return out, nil
}

// ───── refund (admin) ─────

// RefundResult is the outcome of an admin refund.
type RefundResult struct {
	Order    AdminOrderView `json:"order"`
	Refund   vip.Refund     `json:"refund"`
	VIP      vip.Status     `json:"vip"`
	Replayed bool           `json:"replayed"`
}

// RefundKey is the ledger idempotency key of an order's refund: the order is the anchor (any retry is one refund).
func RefundKey(orderID uuid.UUID) string { return "refund:" + orderID.String() }

// Refund marks a paid order refunded and reverses its purchase row through vip.RefundTx (level may drop, perks above
// the new level are unequipped, vip.refund audit row) plus a payment.refund audit row, in one transaction. The
// provider refund (if the adapter supports it) runs first, outside the transaction, idempotent per order.
func (s *Service) Refund(ctx context.Context, adminID, orderID uuid.UUID, reason string) (RefundResult, error) {
	o, err := db.New(s.Pool).GetPaymentOrder(ctx, orderID)
	if errors.Is(err, pgx.ErrNoRows) {
		return RefundResult{}, ErrOrderNotFound
	} else if err != nil {
		return RefundResult{}, err
	}
	if o.Status != StatusPaid && o.Status != StatusRefunded {
		return RefundResult{}, ErrNotRefundable
	}
	callProvider := o.Status == StatusPaid
	if o.Status == StatusPaid {
		// Review M1: never return money at the provider for a reversal the ledger would refuse (balance below the
		// purchase after an admin revoke) — check before the external call. A purchase already reversed through
		// /admin/vip/refund skips the provider and is only synced below.
		q := db.New(s.Pool)
		_, err := q.GetVIPLedgerReversal(ctx, pgtype.Int8{Int64: o.VipLedgerID.Int64, Valid: true})
		switch {
		case err == nil:
			callProvider = false
		case !errors.Is(err, pgx.ErrNoRows):
			return RefundResult{}, err
		default:
			pts, err := q.GetVIPPoints(ctx, o.UserID)
			if err != nil {
				return RefundResult{}, err
			}
			if pts < o.Points {
				return RefundResult{}, fmt.Errorf("%w: balance %d below purchase %d", vip.ErrInvalidGrant, pts, o.Points)
			}
		}
	}
	if prov, ok := s.Providers[o.Provider]; ok && callProvider {
		pctx, cancel := context.WithTimeout(ctx, providerTimeout)
		err := prov.Refund(pctx, o.ID, o.ProviderRef.String, o.AmountVnd)
		cancel()
		if err != nil && !errors.Is(err, ErrRefundUnsupported) {
			s.log().Error("payment provider refund", "provider", o.Provider, "orderId", o.ID, "err", err)
			return RefundResult{}, ErrProviderFailed
		}
	}
	var out RefundResult
	err = postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		now := s.now()
		o, err := q.GetPaymentOrderForUpdate(ctx, orderID)
		if err != nil {
			return err
		}
		switch o.Status {
		case StatusRefunded:
			out.Replayed = true
			out.Refund = vip.Refund{UserID: o.UserID, LedgerID: o.VipLedgerID.Int64, RefundLedgerID: o.RefundLedgerID.Int64,
				PointsRemoved: o.Points, Replayed: true}
		case StatusPaid:
			r, err := s.VIP.RefundTx(ctx, q, adminID, o.VipLedgerID.Int64, "order "+o.ID.String()+": "+reason, RefundKey(o.ID), now)
			if errors.Is(err, vip.ErrAlreadyRefunded) {
				// Reversed earlier through POST /admin/vip/refund: sync the order to the existing refund row.
				rev, rerr := q.GetVIPLedgerReversal(ctx, pgtype.Int8{Int64: o.VipLedgerID.Int64, Valid: true})
				if rerr != nil {
					return rerr
				}
				r = vip.Refund{UserID: o.UserID, LedgerID: o.VipLedgerID.Int64, RefundLedgerID: rev.ID, PointsRemoved: -rev.Delta,
					PointsBefore: rev.PointsAfter - rev.Delta, PointsAfter: rev.PointsAfter, Replayed: true}
			} else if err != nil {
				return err
			}
			out.Refund = r
			if o, err = q.MarkPaymentRefunded(ctx, db.MarkPaymentRefundedParams{Now: tsPtr(now),
				RefundLedgerID: pgtype.Int8{Int64: r.RefundLedgerID, Valid: true}, ID: o.ID}); err != nil {
				return fmt.Errorf("mark refunded: %w", err)
			}
			before, _ := json.Marshal(map[string]any{"status": StatusPaid, "amountVnd": o.AmountVnd, "points": o.Points})
			after, _ := json.Marshal(map[string]any{"status": StatusRefunded, "refundLedgerId": r.RefundLedgerID,
				"pointsBefore": r.PointsBefore, "pointsAfter": r.PointsAfter})
			if err := q.InsertAdminAudit(ctx, db.InsertAdminAuditParams{AdminUserID: adminID, Action: "payment.refund",
				TargetType: "payment_order", TargetID: o.ID.String(), Before: before, After: after, Reason: text(reason)}); err != nil {
				return err
			}
		default:
			return ErrNotRefundable
		}
		out.Order = s.adminView(o)
		out.VIP, err = s.VIP.StatusTx(ctx, q, o.UserID, now)
		return err
	})
	return out, err
}

// ───── dev sandbox ─────

// SandboxResult is what the dev "pay now / fail" action did.
type SandboxResult struct {
	EventID   string         `json:"eventId"`
	Delivered bool           `json:"delivered"`
	Webhook   *WebhookResult `json:"webhook,omitempty"`
	Order     OrderView      `json:"order"`
}

// SandboxComplete simulates the player paying (or failing) at the sandbox gateway (DEV ONLY, the route is registered
// only when APP_ENV=dev): records the charge on the simulated provider and, when deliver, sends a correctly signed
// webhook through HandleWebhook (full verification). deliver=false simulates a lost webhook (reconciliation picks it
// up). amountVND > 0 overrides the paid amount (mismatch testing). Only the order owner (or an admin) may act.
func (s *Service) SandboxComplete(ctx context.Context, userID uuid.UUID, isAdmin bool, orderID uuid.UUID, outcome string, deliver bool, amountVND int64) (SandboxResult, error) {
	sb, ok := s.Providers[ProviderSandbox].(*Sandbox)
	if !ok {
		return SandboxResult{}, ErrUnknownProvider
	}
	if outcome != KindPaid && outcome != KindFailed {
		return SandboxResult{}, ErrBadPayload
	}
	o, err := db.New(s.Pool).GetPaymentOrder(ctx, orderID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && o.UserID != userID && !isAdmin) {
		return SandboxResult{}, ErrOrderNotFound
	} else if err != nil {
		return SandboxResult{}, err
	}
	if o.Provider != ProviderSandbox {
		return SandboxResult{}, ErrUnknownProvider
	}
	if amountVND <= 0 {
		amountVND = o.AmountVnd
	}
	var rnd [8]byte
	_, _ = rand.Read(rnd[:])
	now := s.now()
	ev := Event{EventID: "sbx_evt_" + hex.EncodeToString(rnd[:]), OrderID: o.ID, ProviderRef: o.ProviderRef.String, Kind: outcome,
		AmountVND: amountVND, OccurredAt: now}
	if sb.Charges != nil {
		if err := sb.Charges.Put(ctx, o.ID, Charge{Status: outcome, AmountVND: amountVND, EventID: ev.EventID}, now); err != nil {
			return SandboxResult{}, err
		}
	}
	out := SandboxResult{EventID: ev.EventID, Delivered: deliver}
	if deliver {
		body, sig := sb.BuildWebhook(ev, now)
		h := http.Header{}
		h.Set(SandboxSignatureHeader, sig)
		res, err := s.HandleWebhook(ctx, ProviderSandbox, h, body)
		if err != nil {
			return SandboxResult{}, err
		}
		out.Webhook = &res
	}
	if o, err = db.New(s.Pool).GetPaymentOrder(ctx, orderID); err != nil {
		return SandboxResult{}, err
	}
	out.Order = s.view(o)
	return out, nil
}

// ───── wiring ─────

// BuildProviders returns the adapters enabled for env (payments.yaml envs + enabled). The sandbox needs a signing
// secret: outside dev a missing/short secret is an error; in dev an empty one is replaced by a random per-process
// secret (webhooks are then only signable by this process — fine for the dev endpoint).
func BuildProviders(cat *Catalogue, env string, sandboxSecret []byte, checkoutBase string, pool *pgxpool.Pool, now func() time.Time) (map[string]Provider, error) {
	out := map[string]Provider{}
	for _, def := range cat.EnabledProviders(env) {
		switch def.ID {
		case ProviderSandbox:
			if env == "prod" { // the loader already refuses it; belt and braces
				return nil, errors.New("payment: sandbox provider is never enabled in prod")
			}
			secret := sandboxSecret
			if len(secret) == 0 && env == "dev" {
				secret = make([]byte, MinSecretBytes)
				_, _ = rand.Read(secret)
			}
			sb, err := NewSandbox(secret, checkoutBase, PGCharges{Pool: pool})
			if err != nil {
				return nil, fmt.Errorf("%w (set PAYMENT_SANDBOX_SECRET)", err)
			}
			sb.Now = now
			out[def.ID] = sb
		default:
			return nil, fmt.Errorf("payment: provider %q has no adapter", def.ID)
		}
	}
	return out, nil
}
