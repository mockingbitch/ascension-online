-- Top-up framework (Phase 4, docs/decisions.md 2026-11-07 "Khung nạp Tiên Duyên"): payment orders, verified provider
-- events, sandbox charge state. Money is BIGINT VND. The order is the idempotency anchor of the credit
-- (vip_point_ledger ref_type 'topup_order', ref_id = order id, partial unique index from 000009).

CREATE TABLE payment_orders (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- Financial record: kept when an account goes away (no cascade); account deletion must anonymize instead.
    user_id          UUID        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    package_id       TEXT        NOT NULL,                            -- payments.yaml slug (no FK)
    amount_vnd       BIGINT      NOT NULL CHECK (amount_vnd > 0),
    points           BIGINT      NOT NULL CHECK (points > 0),         -- điểm duyên promised at order time
    provider         TEXT        NOT NULL,
    status           TEXT        NOT NULL DEFAULT 'created'
                     CHECK (status IN ('created', 'pending', 'paid', 'failed', 'expired', 'refunded')),
    provider_ref     TEXT,                                            -- provider's checkout / transaction id
    checkout_url     TEXT,
    checkout_qr      TEXT,
    failure_reason   TEXT,
    idempotency_key  TEXT        NOT NULL,
    vip_ledger_id    BIGINT REFERENCES vip_point_ledger (id),         -- purchase row once paid
    refund_ledger_id BIGINT REFERENCES vip_point_ledger (id),         -- refund row once refunded
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at       TIMESTAMPTZ NOT NULL,
    paid_at          TIMESTAMPTZ,
    closed_at        TIMESTAMPTZ,                                     -- failed / expired
    refunded_at      TIMESTAMPTZ,
    last_checked_at  TIMESTAMPTZ,                                     -- last reconciliation query
    check_count      INTEGER     NOT NULL DEFAULT 0 CHECK (check_count >= 0),
    version          BIGINT      NOT NULL DEFAULT 0,
    CONSTRAINT payment_orders_user_key_uq UNIQUE (user_id, idempotency_key),
    CONSTRAINT payment_orders_provider_ref_uq UNIQUE (provider, provider_ref),
    CONSTRAINT payment_orders_expiry_chk CHECK (expires_at > created_at),
    -- paid and refunded orders carry their purchase row; nothing else does.
    CONSTRAINT payment_orders_paid_chk
        CHECK ((status IN ('paid', 'refunded')) = (paid_at IS NOT NULL AND vip_ledger_id IS NOT NULL)),
    CONSTRAINT payment_orders_refunded_chk
        CHECK ((status = 'refunded') = (refunded_at IS NOT NULL AND refund_ledger_id IS NOT NULL)),
    CONSTRAINT payment_orders_closed_chk CHECK ((status IN ('failed', 'expired')) = (closed_at IS NOT NULL))
);
-- Own history (GET /payments/orders) and admin search by user.
CREATE INDEX payment_orders_user_created_idx ON payment_orders (user_id, created_at DESC);
-- Admin list by status, newest first.
CREATE INDEX payment_orders_status_created_idx ON payment_orders (status, created_at DESC);
-- Worker: expiry sweep and open-order count.
CREATE INDEX payment_orders_open_idx ON payment_orders (expires_at) WHERE status IN ('created', 'pending');
-- Worker: reconciliation candidates (pending, or expired recently — late payments are honoured).
CREATE INDEX payment_orders_reconcile_idx ON payment_orders (created_at) WHERE status IN ('pending', 'expired');
-- One order per purchase / refund ledger row.
CREATE UNIQUE INDEX payment_orders_vip_ledger_uq ON payment_orders (vip_ledger_id) WHERE vip_ledger_id IS NOT NULL;
CREATE UNIQUE INDEX payment_orders_refund_ledger_uq ON payment_orders (refund_ledger_id) WHERE refund_ledger_id IS NOT NULL;

-- Status machine, defence in depth (internal/payment.CanTransition is the primary check):
--   created → pending | paid | failed | expired
--   pending → paid | failed | expired
--   failed | expired → paid            (verified late payment: money was received, honour it)
--   paid → refunded                    (admin)
--   refunded: terminal
CREATE FUNCTION payment_orders_transition_guard() RETURNS trigger AS $$
BEGIN
    IF NEW.status = OLD.status THEN
        RETURN NEW;
    END IF;
    IF (OLD.status = 'created' AND NEW.status IN ('pending', 'paid', 'failed', 'expired'))
        OR (OLD.status = 'pending' AND NEW.status IN ('paid', 'failed', 'expired'))
        OR (OLD.status IN ('failed', 'expired') AND NEW.status = 'paid')
        OR (OLD.status = 'paid' AND NEW.status = 'refunded') THEN
        RETURN NEW;
    END IF;
    RAISE EXCEPTION 'payment order %: illegal status transition % -> %', OLD.id, OLD.status, NEW.status
        USING ERRCODE = 'check_violation';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER payment_orders_transition_trg
    BEFORE UPDATE OF status ON payment_orders
    FOR EACH ROW EXECUTE FUNCTION payment_orders_transition_guard();

-- Normalized provider events: signature-verified webhooks and reconciliation query results. Webhooks failing the
-- signature check are rejected before any write (no table spam from anonymous callers).
CREATE TABLE payment_events (
    id           BIGSERIAL PRIMARY KEY,
    provider     TEXT        NOT NULL,
    dedupe_key   TEXT        NOT NULL,                                -- provider event id (replays hit the UNIQUE)
    source       TEXT        NOT NULL CHECK (source IN ('webhook', 'reconcile')),
    signature_ok BOOLEAN     NOT NULL,                                -- webhook signature verified (false for reconcile)
    order_id     UUID REFERENCES payment_orders (id) ON DELETE RESTRICT,
    kind         TEXT        NOT NULL CHECK (kind IN ('paid', 'failed', 'expired', 'refunded', 'pending')),
    amount_vnd   BIGINT CHECK (amount_vnd >= 0),
    provider_ref TEXT,
    outcome      TEXT        NOT NULL DEFAULT 'received'
                 CHECK (outcome IN ('received', 'applied', 'replayed', 'ignored', 'rejected')),
    detail       TEXT,                                                -- why ignored / rejected (amount_mismatch, ...)
    payload      TEXT        NOT NULL,                                -- raw body as received (bounded by the handler)
    received_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at TIMESTAMPTZ,
    CONSTRAINT payment_events_dedupe_uq UNIQUE (provider, dedupe_key)
);
CREATE INDEX payment_events_order_idx ON payment_events (order_id, id);
-- Admin alert list: rejected / ignored events, newest first.
CREATE INDEX payment_events_outcome_idx ON payment_events (outcome, id DESC) WHERE outcome IN ('rejected', 'ignored');

-- Sandbox gateway's own state (what the simulated provider "knows"), read by its Query for reconciliation.
-- Written only by the dev sandbox endpoint; unused by real providers.
CREATE TABLE payment_sandbox_charges (
    order_id   UUID PRIMARY KEY REFERENCES payment_orders (id) ON DELETE CASCADE,
    status     TEXT        NOT NULL CHECK (status IN ('paid', 'failed', 'refunded')),
    amount_vnd BIGINT      NOT NULL CHECK (amount_vnd >= 0),
    event_id   TEXT        NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
