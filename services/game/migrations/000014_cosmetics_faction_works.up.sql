-- Recurring spirit-stone sinks (decisions 2026-11-07 "Linh thạch sinks"): cosmetics ownership + equipped look, and
-- faction works donations. None of these tables is read by cultivation, breakthrough, ranking, merit or Tiên Duyên.

-- Owned cosmetics (cosmetics.yaml "<kind>:<slug>"). One row = one ownership = one purchase (PK: owned at most once).
-- Tiên Duyên perk cosmetics have no row: ownership is derived from the current level (cosmetic.Owns).
CREATE TABLE character_cosmetics (
    character_id    UUID        NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    cosmetic_id     TEXT        NOT NULL CHECK (cosmetic_id ~ '^(title|avatar_frame|chat_aura|cave_decoration):[a-z][a-z0-9_]*$'),
    source          TEXT        NOT NULL CHECK (source IN ('purchase', 'migrated', 'admin')),
    price           BIGINT      NOT NULL CHECK (price >= 0),
    idempotency_key TEXT        NOT NULL,   -- purchase: the request key (currency ledger key "cosmetic:<key>")
    acquired_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (character_id, cosmetic_id),
    UNIQUE (character_id, idempotency_key),
    CHECK ((source = 'purchase') = (price > 0))
);

-- Equipped look, one row per character (read by GET /character, chat send, leaderboards). Ids only.
CREATE TABLE character_appearance (
    character_id        UUID        PRIMARY KEY REFERENCES characters (id) ON DELETE CASCADE,
    title_id            TEXT        CHECK (title_id LIKE 'title:%'),
    avatar_frame_id     TEXT        CHECK (avatar_frame_id LIKE 'avatar_frame:%'),
    chat_aura_id        TEXT        CHECK (chat_aura_id LIKE 'chat_aura:%'),
    cave_decoration_ids TEXT[]      NOT NULL DEFAULT '{}' CHECK (cardinality(cave_decoration_ids) <= 12),
    version             BIGINT      NOT NULL DEFAULT 0,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Equip log: idempotency (same key = replay, other loadout = 409) + audit of what was worn when.
CREATE TABLE cosmetic_equips (
    id                  BIGSERIAL   PRIMARY KEY,
    character_id        UUID        NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    title_id            TEXT,
    avatar_frame_id     TEXT,
    chat_aura_id        TEXT,
    cave_decoration_ids TEXT[]      NOT NULL,
    idempotency_key     TEXT        NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (character_id, idempotency_key)
);

-- Chat name effect snapshot at send time (others see the effect the sender wore when posting).
ALTER TABLE chat_messages ADD COLUMN name_fx TEXT CHECK (name_fx IS NULL OR name_fx LIKE 'chat_aura:%');

-- Faction works: shared pool per faction (catalogue slug, no FK). Progress/stages are derived from donated_total.
CREATE TABLE faction_treasuries (
    faction_id    TEXT        PRIMARY KEY,
    donated_total BIGINT      NOT NULL DEFAULT 0 CHECK (donated_total >= 0),
    version       BIGINT      NOT NULL DEFAULT 0,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One row per donation; the key also keys the currency ledger row ("donate:<key>").
CREATE TABLE faction_donations (
    id              BIGSERIAL   PRIMARY KEY,
    character_id    UUID        NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    faction_id      TEXT        NOT NULL,
    amount          BIGINT      NOT NULL CHECK (amount > 0),
    total_after     BIGINT      NOT NULL CHECK (total_after >= amount),
    game_day        DATE        NOT NULL,
    idempotency_key TEXT        NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (character_id, idempotency_key)
);
CREATE INDEX faction_donations_faction_time_idx ON faction_donations (faction_id, created_at DESC);

-- Daily cap per character (across factions): conditional upsert, like shop_daily_counts.
CREATE TABLE faction_donation_days (
    character_id UUID   NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    game_day     DATE   NOT NULL,
    amount       BIGINT NOT NULL CHECK (amount >= 0),
    PRIMARY KEY (character_id, game_day)
);

-- Công đức bảng (display only — never an input of merit, ranking or điểm duyên).
CREATE TABLE faction_donors (
    faction_id   TEXT        NOT NULL,
    character_id UUID        NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    total        BIGINT      NOT NULL CHECK (total > 0),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (faction_id, character_id)
);
CREATE INDEX faction_donors_board_idx ON faction_donors (faction_id, total DESC, character_id);

-- The two Phase 2.5 shop cosmetics move to the cosmetics catalogue (items.yaml entries removed). Each owner keeps ONE
-- copy as a cosmetic; the inventory row goes to 0 with an inventory_transactions row; extra units (unique items, so
-- only via admin grants) are compensated at the old shop price (3,000 / 5,000) with a currency_ledger credit
-- (reason migrate_000014_refund). Ledger keys are unique per run (txid) so up → down → up never collides, and
-- quantities and ledger rows always move together (no silent ON CONFLICT).
DO $$
DECLARE
    run_key  TEXT := 'migrate:000014:up:' || txid_current();
    r        RECORD;
    cos_id   TEXT;
    price    BIGINT;
    extra    BIGINT;
    bal      BIGINT;
    moved    INTEGER := 0;
    extras   BIGINT := 0;
    refunded BIGINT := 0;
BEGIN
    FOR r IN SELECT character_id, item_id, quantity FROM inventory_items
             WHERE item_id IN ('avatar_frame_ngoc_bich', 'cave_deco_tung_hac') AND quantity > 0
             ORDER BY character_id, item_id FOR UPDATE
    LOOP
        IF r.item_id = 'avatar_frame_ngoc_bich' THEN cos_id := 'avatar_frame:ngoc_bich'; price := 3000;
        ELSE cos_id := 'cave_decoration:tung_hac'; price := 5000; END IF;
        INSERT INTO character_cosmetics (character_id, cosmetic_id, source, price, idempotency_key)
        VALUES (r.character_id, cos_id, 'migrated', 0, run_key || ':' || r.item_id);
        UPDATE inventory_items SET quantity = 0 WHERE character_id = r.character_id AND item_id = r.item_id;
        INSERT INTO inventory_transactions (character_id, item_id, delta, quantity_after, reason, ref_type, ref_id, idempotency_key)
        VALUES (r.character_id, r.item_id, -r.quantity, 0, 'cosmetic_migrate', 'migration', '000014', run_key);
        moved := moved + 1;
        extra := r.quantity - 1;
        IF extra > 0 THEN
            INSERT INTO currency_accounts (character_id, currency, balance) VALUES (r.character_id, 'spirit_stone', 0)
            ON CONFLICT (character_id, currency) DO NOTHING;   -- account may simply not exist yet (no value involved)
            UPDATE currency_accounts SET balance = balance + extra * price, version = version + 1
            WHERE character_id = r.character_id AND currency = 'spirit_stone'
            RETURNING balance INTO bal;
            INSERT INTO currency_ledger (character_id, currency, delta, balance_after, reason, ref_type, ref_id, idempotency_key)
            VALUES (r.character_id, 'spirit_stone', extra * price, bal, 'migrate_000014_refund', 'item', r.item_id,
                    run_key || ':refund:' || r.item_id);
            extras := extras + extra;
            refunded := refunded + extra * price;
        END IF;
    END LOOP;
    RAISE NOTICE '000014: % legacy cosmetic item rows converted; % extra units above 1 written off and refunded (% spirit stones)',
        moved, extras, refunded;
END $$;
