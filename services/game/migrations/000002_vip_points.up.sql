-- VIP v2: permanent level derived from cumulative points (content/catalog/vip.yaml thresholds).
-- Level is NOT stored: it is derived from points so a ladder retune never needs a data migration.
CREATE TABLE vip_accounts (
    user_id    UUID PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    points     BIGINT      NOT NULL DEFAULT 0 CHECK (points >= 0),
    version    BIGINT      NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Append-only. Every points change inserts exactly one row in the same tx.
CREATE TABLE vip_point_ledger (
    id              BIGSERIAL PRIMARY KEY,
    user_id         UUID        NOT NULL REFERENCES vip_accounts (user_id) ON DELETE CASCADE,
    delta           BIGINT      NOT NULL CHECK (delta <> 0),
    points_after    BIGINT      NOT NULL CHECK (points_after >= 0),
    reason          TEXT        NOT NULL CHECK (reason IN ('purchase', 'admin_grant', 'admin_revoke', 'refund', 'promo')),
    ref_type        TEXT,
    ref_id          TEXT,
    idempotency_key TEXT        NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, idempotency_key)
);
CREATE INDEX vip_point_ledger_user_idx ON vip_point_ledger (user_id, created_at DESC);

-- Daily gift claims: one per user per server-day (UTC+7 business day computed by the app).
CREATE TABLE vip_daily_gift_claims (
    user_id    UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    claim_day  DATE        NOT NULL,
    tier       SMALLINT    NOT NULL CHECK (tier BETWEEN 0 AND 12),
    amount     BIGINT      NOT NULL CHECK (amount >= 0),
    claimed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, claim_day)
);

-- Monthly-card style time-boxed entitlements remain possible via vip_entitlements; widen tier range.
ALTER TABLE vip_entitlements DROP CONSTRAINT vip_entitlements_tier_check;
ALTER TABLE vip_entitlements ADD CONSTRAINT vip_entitlements_tier_check CHECK (tier BETWEEN 1 AND 12);
