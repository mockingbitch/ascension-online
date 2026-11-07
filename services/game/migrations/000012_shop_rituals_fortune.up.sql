-- Phase 2.5: shop (spirit-stone sink), morning incense streak, gieo quẻ fortune + its idle-rate buff.
-- docs/design/data-model-the-gioi-chung.md §8.3, §9.2–9.4.

-- One row per purchase; the idempotency key also keys the currency/inventory ledger rows ("shop:<key>").
CREATE TABLE shop_purchases (
    id              BIGSERIAL   PRIMARY KEY,
    character_id    UUID        NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    listing_id      TEXT        NOT NULL,                       -- shop.yaml listing id
    item_id         TEXT        NOT NULL,                       -- items.yaml id
    lots            INTEGER     NOT NULL CHECK (lots > 0),
    item_quantity   BIGINT      NOT NULL CHECK (item_quantity > 0),
    currency        TEXT        NOT NULL CHECK (currency IN ('spirit_stone')),
    unit_price      BIGINT      NOT NULL CHECK (unit_price > 0),
    total_price     BIGINT      NOT NULL CHECK (total_price = unit_price * lots),
    game_day        DATE        NOT NULL,
    idempotency_key TEXT        NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (character_id, idempotency_key)
);
CREATE INDEX shop_purchases_char_time_idx ON shop_purchases (character_id, created_at DESC);

-- Lots bought per (character, game day, listing); conditional upsert enforces daily_limit_lots (S2).
CREATE TABLE shop_daily_counts (
    character_id UUID    NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    game_day     DATE    NOT NULL,
    listing_id   TEXT    NOT NULL,
    lots         INTEGER NOT NULL CHECK (lots >= 0),
    PRIMARY KEY (character_id, game_day, listing_id)
);

-- Morning incense ("dâng hương tĩnh tâm") streak. last_day = game day of the last offering (once per day).
CREATE TABLE ritual_streaks (
    character_id UUID        PRIMARY KEY REFERENCES characters (id) ON DELETE CASCADE,
    last_day     DATE        NOT NULL,
    streak       INTEGER     NOT NULL CHECK (streak >= 1),
    best         INTEGER     NOT NULL,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (best >= streak)
);

-- Gieo quẻ: one draw per (character, game day). roll_bp is the server roll in [0, 10000).
CREATE TABLE daily_fortunes (
    character_id UUID        NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    game_day     DATE        NOT NULL,
    outcome_id   TEXT        NOT NULL,                            -- fortunes.yaml outcome id
    roll_bp      INTEGER     NOT NULL CHECK (roll_bp BETWEEN 0 AND 9999),
    bonus_bp     INTEGER     NOT NULL CHECK (bonus_bp BETWEEN 0 AND 10000),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (character_id, game_day)
);

-- Timed idle-rate buffs. Settlement applies bonus_bp only to eligible seconds inside [starts_at, ends_at).
CREATE TABLE character_buffs (
    character_id UUID        NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    slot         TEXT        NOT NULL CHECK (slot IN ('fortune')),
    bonus_bp     INTEGER     NOT NULL CHECK (bonus_bp BETWEEN 0 AND 10000),
    starts_at    TIMESTAMPTZ NOT NULL,
    ends_at      TIMESTAMPTZ NOT NULL,
    source_ref   TEXT        NOT NULL,
    PRIMARY KEY (character_id, slot),
    CHECK (ends_at > starts_at)
);
