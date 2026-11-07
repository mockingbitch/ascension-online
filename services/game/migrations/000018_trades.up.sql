-- Phase 5.2: trao đổi vật phẩm — player-to-player gifts (one-way) and swaps (two-sided), decisions 2026-11-07
-- "Chủ dự án chốt trước Phase 4–5" (luật trao đổi) and "Trao đổi vật phẩm (Phase 5.2)"; content/catalog/trade.yaml;
-- internal/trade.
--
-- Nothing is escrowed: an offer is a promise checked again at accept time. Ownership moves ONLY at accept, through the
-- existing ledgers (currency_ledger / inventory_transactions rows on both sides, reasons trade_send | trade_receive |
-- trade_fee, ref_type 'trade', ref_id = offer id) in ONE transaction.
--
-- Lock order (data-model §2.3, new branch — no other flow touches trade_* tables):
--   trade_offers (FOR UPDATE, the offer being answered)
--   → trade_days (both parties, upsert + FOR UPDATE, ORDER BY character_id)
--   → currency_accounts (both parties, spirit_stone, ORDER BY character_id)
--   → inventory_items (both parties ORDER BY character_id, then item_id)
-- so two accepts sharing a character (incl. a cross-trade A→B and B→A at once) serialize instead of deadlocking.
-- Creating an offer locks only the sender's trade_days row (serializes the sender's daily/open-offer counts).

-- One offer. status machine: pending → accepted | declined | cancelled | expired (terminal states never change; the
-- trigger below enforces it besides the conditional UPDATEs in Go). Values are stone-equivalents from trade.yaml
-- (computed live while pending, frozen at accept with the fees actually burned).
CREATE TABLE trade_offers (
    id                    UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    kind                  TEXT        NOT NULL CHECK (kind IN ('gift', 'swap')),
    sender_character_id   UUID        NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    receiver_character_id UUID        NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    status                TEXT        NOT NULL DEFAULT 'pending'
                                      CHECK (status IN ('pending', 'accepted', 'declined', 'cancelled', 'expired')),
    relation              TEXT        NOT NULL CHECK (relation IN ('stranger', 'faction', 'dao_lu')), -- at create; re-set at accept
    net_flag              BOOLEAN     NOT NULL DEFAULT false, -- accounts shared a network hash (netsig, 30 days) at create
    value_to_receiver     BIGINT      NOT NULL DEFAULT 0 CHECK (value_to_receiver >= 0),
    value_to_sender       BIGINT      NOT NULL DEFAULT 0 CHECK (value_to_sender >= 0),
    sender_fee            BIGINT      NOT NULL DEFAULT 0 CHECK (sender_fee >= 0),   -- linh thạch burned from the sender
    receiver_fee          BIGINT      NOT NULL DEFAULT 0 CHECK (receiver_fee >= 0), -- linh thạch burned from the receiver (swap)
    idempotency_key       TEXT        NOT NULL,              -- key of the create call
    game_day              DATE        NOT NULL,              -- creation day (offers_per_day)
    created_at            TIMESTAMPTZ NOT NULL,
    expires_at            TIMESTAMPTZ NOT NULL,
    responded_at          TIMESTAMPTZ,
    respond_key           TEXT,                              -- key of the accept/decline/cancel call (replays)
    accepted_day          DATE,                              -- game day of the accept (daily caps)
    UNIQUE (sender_character_id, idempotency_key),
    CHECK (sender_character_id <> receiver_character_id),
    CHECK (expires_at > created_at),
    CHECK ((status = 'pending') = (responded_at IS NULL)),
    CHECK (respond_key IS NULL OR responded_at IS NOT NULL),
    CHECK ((status = 'accepted') = (accepted_day IS NOT NULL)),
    CHECK (kind = 'swap' OR (value_to_sender = 0 AND receiver_fee = 0)),
    CHECK (status = 'accepted' OR (sender_fee = 0 AND receiver_fee = 0))
);
CREATE INDEX trade_offers_receiver_idx ON trade_offers (receiver_character_id, created_at DESC);
CREATE INDEX trade_offers_sender_idx ON trade_offers (sender_character_id, created_at DESC);
CREATE INDEX trade_offers_pending_exp_idx ON trade_offers (expires_at) WHERE status = 'pending';
CREATE INDEX trade_offers_accepted_idx ON trade_offers (responded_at) WHERE status = 'accepted';
CREATE INDEX trade_offers_flag_idx ON trade_offers (created_at DESC) WHERE net_flag;

CREATE FUNCTION trade_offers_status_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.status <> 'pending' AND (NEW.status <> OLD.status OR NEW.sender_fee <> OLD.sender_fee
        OR NEW.receiver_fee <> OLD.receiver_fee OR NEW.value_to_receiver <> OLD.value_to_receiver
        OR NEW.value_to_sender <> OLD.value_to_sender) THEN
        RAISE EXCEPTION 'trade offer % is %, terminal', OLD.id, OLD.status USING ERRCODE = 'check_violation';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER trade_offers_status_guard BEFORE UPDATE ON trade_offers
    FOR EACH ROW EXECUTE FUNCTION trade_offers_status_guard();

-- The lines of an offer. side 'give' = sender → receiver, 'take' = receiver → sender (swap only). asset_id is
-- 'spirit_stone' (kind currency) or an items.yaml id (kind item); only trade.yaml assets are accepted (Go + content).
CREATE TABLE trade_lines (
    offer_id   UUID   NOT NULL REFERENCES trade_offers (id) ON DELETE CASCADE,
    side       TEXT   NOT NULL CHECK (side IN ('give', 'take')),
    asset_kind TEXT   NOT NULL CHECK (asset_kind IN ('currency', 'item')),
    asset_id   TEXT   NOT NULL,
    quantity   BIGINT NOT NULL CHECK (quantity > 0),
    PRIMARY KEY (offer_id, side, asset_id),
    CHECK (asset_kind = 'item' OR asset_id = 'spirit_stone')
);

-- Per character and game day (UTC+7): the counters every daily limit reads. offers_created counts at creation (by the
-- creation day); completed / value_* count at accept (by the accept day), for BOTH parties.
CREATE TABLE trade_days (
    character_id   UUID    NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    game_day       DATE    NOT NULL,
    offers_created INTEGER NOT NULL DEFAULT 0 CHECK (offers_created >= 0),
    completed      INTEGER NOT NULL DEFAULT 0 CHECK (completed >= 0),
    value_received BIGINT  NOT NULL DEFAULT 0 CHECK (value_received >= 0),
    value_sent     BIGINT  NOT NULL DEFAULT 0 CHECK (value_sent >= 0),
    PRIMARY KEY (character_id, game_day)
);

-- Full log (append-only): every step, net flags and reports. Ledger rows carry the actual movements.
CREATE TABLE trade_events (
    id                 BIGSERIAL   PRIMARY KEY,
    offer_id           UUID        NOT NULL REFERENCES trade_offers (id) ON DELETE CASCADE,
    kind               TEXT        NOT NULL CHECK (kind IN ('created', 'accepted', 'declined', 'cancelled', 'expired', 'net_flag')),
    actor_character_id UUID        REFERENCES characters (id) ON DELETE SET NULL,
    detail             JSONB,
    created_at         TIMESTAMPTZ NOT NULL
);
CREATE INDEX trade_events_offer_idx ON trade_events (offer_id, id);

-- A trade can be reported (reason trade_abuse, by one party about the other; the offer is the evidence).
ALTER TABLE reports DROP CONSTRAINT reports_reason_check;
ALTER TABLE reports ADD CONSTRAINT reports_reason_check CHECK (reason IN ('offensive_name', 'harassment', 'trade_abuse'));
ALTER TABLE reports ADD COLUMN trade_offer_id UUID REFERENCES trade_offers (id) ON DELETE SET NULL;
ALTER TABLE reports ADD CONSTRAINT reports_trade_reason_check CHECK (trade_offer_id IS NULL OR reason = 'trade_abuse');
