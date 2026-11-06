-- Tiên Đạo Online — initial schema (MVP P0).
-- Conventions: see .claude/rules/database.md
--   * snake_case, plural table names, UUID PKs for entities, BIGSERIAL for append-only logs
--   * all resource/currency amounts are BIGINT (fixed-point, never float)
--   * all timestamps are TIMESTAMPTZ and come from the server clock
--   * catalogue IDs (realm, stage, item) are stable TEXT slugs owned by content/catalog/*.yaml

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- ───────────────────────── Identity ─────────────────────────
CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT        NOT NULL,
    password_hash TEXT        NOT NULL,
    role          TEXT        NOT NULL DEFAULT 'player' CHECK (role IN ('player', 'admin')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX users_email_lower_uq ON users (lower(email));

CREATE TABLE refresh_tokens (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash  TEXT        NOT NULL UNIQUE,
    expires_at  TIMESTAMPTZ NOT NULL,
    revoked_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX refresh_tokens_user_idx ON refresh_tokens (user_id);

-- ───────────────────────── Character ─────────────────────────
CREATE TABLE characters (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID        NOT NULL UNIQUE REFERENCES users (id) ON DELETE CASCADE, -- MVP: 1 character / account
    name                TEXT        NOT NULL CHECK (char_length(name) BETWEEN 2 AND 24),
    spirit_root_type    TEXT,                     -- content/catalog/spirit_roots.yaml id (P1)
    spirit_root_quality SMALLINT    CHECK (spirit_root_quality BETWEEN 0 AND 100),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX characters_name_lower_uq ON characters (lower(name));

-- Hot row: one per character, locked FOR UPDATE by settlement/breakthrough.
CREATE TABLE character_progressions (
    character_id                UUID PRIMARY KEY REFERENCES characters (id) ON DELETE CASCADE,
    realm_id                    TEXT        NOT NULL,
    stage_id                    TEXT        NOT NULL,          -- global stage id "<realm>.<stage>"
    cultivation                 BIGINT      NOT NULL DEFAULT 0 CHECK (cultivation >= 0), -- progress inside current stage
    remainder_milli             INTEGER     NOT NULL DEFAULT 0 CHECK (remainder_milli BETWEEN 0 AND 999),
    last_cultivated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    breakthrough_fail_streak    INTEGER     NOT NULL DEFAULT 0 CHECK (breakthrough_fail_streak >= 0),
    breakthrough_cooldown_until TIMESTAMPTZ,
    stage_reached_at            TIMESTAMPTZ NOT NULL DEFAULT now(), -- leaderboard tie-break
    version                     BIGINT      NOT NULL DEFAULT 0,
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ───────────────────────── Cultivation & breakthrough logs ─────────────────────────
CREATE TABLE cultivation_settlements (
    id                  BIGSERIAL PRIMARY KEY,
    character_id        UUID        NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    idempotency_key     TEXT        NOT NULL,
    stage_id            TEXT        NOT NULL,
    elapsed_seconds     BIGINT      NOT NULL CHECK (elapsed_seconds >= 0),
    eligible_seconds    BIGINT      NOT NULL CHECK (eligible_seconds >= 0),
    rate_milli          BIGINT      NOT NULL CHECK (rate_milli >= 0),
    offline_cap_seconds BIGINT      NOT NULL CHECK (offline_cap_seconds >= 0),
    earned              BIGINT      NOT NULL CHECK (earned >= 0),
    cultivation_after   BIGINT      NOT NULL,
    settled_at          TIMESTAMPTZ NOT NULL,
    UNIQUE (character_id, idempotency_key)
);
CREATE INDEX cultivation_settlements_char_time_idx ON cultivation_settlements (character_id, settled_at DESC);

CREATE TABLE breakthrough_attempts (
    id              BIGSERIAL PRIMARY KEY,
    character_id    UUID        NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    idempotency_key TEXT        NOT NULL,
    from_stage_id   TEXT        NOT NULL,
    to_stage_id     TEXT        NOT NULL,
    kind            TEXT        NOT NULL CHECK (kind IN ('minor', 'major')),
    success_bp      INTEGER     NOT NULL CHECK (success_bp BETWEEN 0 AND 10000), -- effective chance incl. pity
    roll_bp         INTEGER     NOT NULL CHECK (roll_bp BETWEEN 0 AND 9999),     -- auditable server roll
    success         BOOLEAN     NOT NULL,
    consumed        JSONB       NOT NULL DEFAULT '[]'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (character_id, idempotency_key)
);

-- Immutable milestone history (achievements, "first to reach", feed).
CREATE TABLE progression_events (
    id           BIGSERIAL PRIMARY KEY,
    character_id UUID        NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    kind         TEXT        NOT NULL,   -- stage_reached | realm_reached | ...
    stage_id     TEXT,
    payload      JSONB       NOT NULL DEFAULT '{}'::jsonb,
    occurred_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX progression_events_char_idx ON progression_events (character_id, occurred_at DESC);
CREATE INDEX progression_events_stage_first_idx ON progression_events (stage_id, occurred_at) WHERE kind = 'realm_reached';

-- ───────────────────────── VIP ─────────────────────────
CREATE TABLE vip_entitlements (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    tier        SMALLINT    NOT NULL CHECK (tier BETWEEN 1 AND 10),
    starts_at   TIMESTAMPTZ NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL CHECK (expires_at > starts_at),
    source      TEXT        NOT NULL,  -- purchase | admin_grant | promo
    source_ref  TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX vip_entitlements_active_idx ON vip_entitlements (user_id, expires_at DESC);

-- ───────────────────────── Economy ─────────────────────────
CREATE TABLE currency_accounts (
    character_id UUID   NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    currency     TEXT   NOT NULL CHECK (currency IN ('spirit_stone', 'premium')),
    balance      BIGINT NOT NULL DEFAULT 0 CHECK (balance >= 0),
    version      BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (character_id, currency)
);

-- Append-only. Every balance change MUST insert exactly one ledger row in the same tx.
CREATE TABLE currency_ledger (
    id              BIGSERIAL PRIMARY KEY,
    character_id    UUID        NOT NULL,
    currency        TEXT        NOT NULL,
    delta           BIGINT      NOT NULL CHECK (delta <> 0),
    balance_after   BIGINT      NOT NULL CHECK (balance_after >= 0),
    reason          TEXT        NOT NULL,   -- settlement | breakthrough_cost | daily_reward | admin_grant ...
    ref_type        TEXT,
    ref_id          TEXT,
    idempotency_key TEXT        NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (character_id, currency) REFERENCES currency_accounts (character_id, currency),
    UNIQUE (character_id, currency, idempotency_key)
);

CREATE TABLE inventory_items (
    character_id UUID   NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    item_id      TEXT   NOT NULL,          -- content/catalog/items.yaml id
    quantity     BIGINT NOT NULL CHECK (quantity >= 0),
    PRIMARY KEY (character_id, item_id)
);

CREATE TABLE inventory_transactions (
    id              BIGSERIAL PRIMARY KEY,
    character_id    UUID        NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    item_id         TEXT        NOT NULL,
    delta           BIGINT      NOT NULL CHECK (delta <> 0),
    quantity_after  BIGINT      NOT NULL CHECK (quantity_after >= 0),
    reason          TEXT        NOT NULL,
    ref_type        TEXT,
    ref_id          TEXT,
    idempotency_key TEXT        NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (character_id, item_id, idempotency_key)
);

-- ───────────────────────── Operations ─────────────────────────
-- Transactional outbox: written in the same tx as the state change, published by the worker.
CREATE TABLE outbox_events (
    id           BIGSERIAL PRIMARY KEY,
    topic        TEXT        NOT NULL,   -- progression.stage_reached | leaderboard.dirty | announcement.major ...
    aggregate_id TEXT        NOT NULL,
    payload      JSONB       NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ,
    attempts     INTEGER     NOT NULL DEFAULT 0
);
CREATE INDEX outbox_events_pending_idx ON outbox_events (id) WHERE published_at IS NULL;

CREATE TABLE admin_audit_logs (
    id            BIGSERIAL PRIMARY KEY,
    admin_user_id UUID        NOT NULL REFERENCES users (id),
    action        TEXT        NOT NULL,
    target_type   TEXT        NOT NULL,
    target_id     TEXT        NOT NULL,
    before        JSONB,
    after         JSONB,
    reason        TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
