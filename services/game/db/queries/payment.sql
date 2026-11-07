-- Top-up framework (000016). Lock order: payment_orders (FOR UPDATE) → vip_accounts → character_appearance.

-- name: LockPaymentUser :exec
-- Serializes order creation per user (max open orders) without touching a hot game row.
SELECT pg_advisory_xact_lock(7310016, hashtext(sqlc.arg(user_key)::text));

-- name: GetPaymentOrderByKey :one
SELECT * FROM payment_orders WHERE user_id = $1 AND idempotency_key = $2;

-- name: CountOpenPaymentOrders :one
SELECT COUNT(*)::bigint FROM payment_orders
WHERE user_id = @user_id AND status IN ('created', 'pending') AND expires_at > @now;

-- name: InsertPaymentOrder :one
INSERT INTO payment_orders (user_id, package_id, amount_vnd, points, provider, idempotency_key, created_at, updated_at, expires_at)
VALUES (@user_id, @package_id, @amount_vnd, @points, @provider, @idempotency_key, @now, @now, @expires_at)
RETURNING *;

-- name: GetPaymentOrder :one
SELECT * FROM payment_orders WHERE id = $1;

-- name: GetPaymentOrderForUpdate :one
SELECT * FROM payment_orders WHERE id = $1 FOR UPDATE;

-- name: ClaimPaymentCheckout :one
-- Only one caller creates the provider checkout of an order (review M2): a fresh order has version 0 and the claim
-- bumps it; a claim older than @stale (crashed winner) can be taken over. No row → someone else is creating it.
UPDATE payment_orders SET version = version + 1, updated_at = @now
WHERE id = @id AND status = 'created' AND checkout_url IS NULL AND (version = 0 OR updated_at < @stale)
RETURNING *;

-- name: SetPaymentCheckout :one
-- Records the provider checkout once; moves created → pending (a fast webhook may already have paid it: status kept).
UPDATE payment_orders SET
    provider_ref = COALESCE(provider_ref, @provider_ref),
    checkout_url = @checkout_url,
    checkout_qr  = sqlc.narg('checkout_qr'),
    status       = CASE WHEN status = 'created' THEN 'pending' ELSE status END,
    updated_at   = @now, version = version + 1
WHERE id = @id AND checkout_url IS NULL
RETURNING *;

-- name: MarkPaymentCheckoutFailed :one
-- The provider refused the checkout: fail the order only while no checkout was ever recorded.
UPDATE payment_orders SET status = 'failed', closed_at = @now, failure_reason = 'checkout_error', updated_at = @now, version = version + 1
WHERE id = @id AND status = 'created' AND checkout_url IS NULL
RETURNING *;

-- name: MarkPaymentPaid :one
UPDATE payment_orders SET status = 'paid', paid_at = @now, vip_ledger_id = @vip_ledger_id,
    provider_ref = COALESCE(provider_ref, sqlc.narg('provider_ref')), closed_at = NULL, failure_reason = NULL,
    updated_at = @now, version = version + 1
WHERE id = @id AND status IN ('created', 'pending', 'failed', 'expired')
RETURNING *;

-- name: MarkPaymentClosed :one
-- created/pending → failed | expired.
UPDATE payment_orders SET status = @status, closed_at = @now, failure_reason = @reason, updated_at = @now, version = version + 1
WHERE id = @id AND status IN ('created', 'pending')
RETURNING *;

-- name: MarkPaymentRefunded :one
UPDATE payment_orders SET status = 'refunded', refunded_at = @now, refund_ledger_id = @refund_ledger_id,
    updated_at = @now, version = version + 1
WHERE id = @id AND status = 'paid'
RETURNING *;

-- name: ExpirePaymentOrders :many
-- Bounded expiry sweep; SKIP LOCKED leaves orders being settled right now to the next pass.
UPDATE payment_orders SET status = 'expired', closed_at = @now, failure_reason = 'expired', updated_at = @now, version = version + 1
WHERE id IN (SELECT o.id FROM payment_orders o
             WHERE o.status IN ('created', 'pending') AND o.expires_at <= @now
             ORDER BY o.expires_at LIMIT @batch FOR UPDATE SKIP LOCKED)
RETURNING id;

-- name: ListReconcileCandidates :many
-- Pending orders older than reconcile_after, and orders expired within reconcile_late, not queried recently.
SELECT * FROM payment_orders
WHERE status IN ('pending', 'expired') AND created_at <= @older_than AND provider_ref IS NOT NULL
  AND (status = 'pending' OR closed_at >= @late_since)
  AND (last_checked_at IS NULL OR last_checked_at <= @checked_before)
ORDER BY created_at
LIMIT @batch;

-- name: TouchPaymentChecked :exec
UPDATE payment_orders SET last_checked_at = @now, check_count = check_count + 1 WHERE id = @id;

-- name: ListUserPaymentOrders :many
SELECT * FROM payment_orders WHERE user_id = @user_id ORDER BY created_at DESC, id LIMIT @lim;

-- name: AdminListPaymentOrders :many
-- Filters are optional (NULL = any): status, user, exact provider_ref.
SELECT * FROM payment_orders
WHERE (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text)
  AND (sqlc.narg('user_id')::uuid IS NULL OR user_id = sqlc.narg('user_id')::uuid)
  AND (sqlc.narg('provider_ref')::text IS NULL OR provider_ref = sqlc.narg('provider_ref')::text)
ORDER BY created_at DESC, id
LIMIT @lim;

-- name: InsertPaymentEvent :one
-- Returns no row when the provider event was already received (dedupe → replay no-op).
INSERT INTO payment_events (provider, dedupe_key, source, signature_ok, order_id, kind, amount_vnd, provider_ref, payload, received_at)
VALUES (@provider, @dedupe_key, @source, @signature_ok, sqlc.narg('order_id'), @kind, sqlc.narg('amount_vnd'),
        sqlc.narg('provider_ref'), @payload, @now)
ON CONFLICT (provider, dedupe_key) DO NOTHING
RETURNING id;

-- name: FinishPaymentEvent :exec
UPDATE payment_events SET outcome = @outcome, detail = sqlc.narg('detail'), order_id = sqlc.narg('order_id'), processed_at = @now
WHERE id = @id;

-- name: GetPaymentEventByDedupe :one
SELECT * FROM payment_events WHERE provider = $1 AND dedupe_key = $2;

-- name: ListPaymentEventsForOrder :many
SELECT * FROM payment_events WHERE order_id = $1 ORDER BY id;

-- name: ListPaymentAlerts :many
-- Rejected / ignored events for admin review, newest first.
SELECT * FROM payment_events WHERE outcome IN ('rejected', 'ignored') ORDER BY id DESC LIMIT @lim;

-- name: UpsertSandboxCharge :exec
INSERT INTO payment_sandbox_charges (order_id, status, amount_vnd, event_id, updated_at)
VALUES (@order_id, @status, @amount_vnd, @event_id, @now)
ON CONFLICT (order_id) DO UPDATE SET status = EXCLUDED.status, amount_vnd = EXCLUDED.amount_vnd,
    event_id = EXCLUDED.event_id, updated_at = EXCLUDED.updated_at;

-- name: GetSandboxCharge :one
SELECT * FROM payment_sandbox_charges WHERE order_id = $1;
