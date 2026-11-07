-- Phase 5.1: đạo lữ song tu + the salted IP-hash anti-multi-account signal (decisions 2026-11-07 "Chủ dự án chốt trước
-- Phase 4–5" and "Đạo lữ song tu (Phase 5.1)"; content/catalog/dao_lu.yaml; internal/daolu, internal/netsig).
--
-- Lock order (data-model §2.3, extended):
--   character_progressions → daily_contribution → dao_lu_proposals → dao_lu_bonds → dao_lu_members (by character_id)
--   → vip_accounts.
-- A song tu claim locks the claimer's progression, credits (CreditLocked) then điểm duyên (EarnLocked) — the lecture
-- order. Accepting a proposal locks the proposal, then the bond rows it creates (members inserted in character_id order
-- so two concurrent accepts sharing a character never deadlock: the second one waits on the partial unique index and
-- fails cleanly).

-- ───── network signal (netsig) ─────
-- HMAC-SHA256(secret salt, client IP) truncated to 16 bytes, per user and salt key, refreshed at Google login and session
-- refresh. NEVER the raw IP. Rows unseen for 30 days are deleted by the worker (bounded batches). Used only to warn
-- admins about accounts on the same network (song tu, gifts/trades, hộ pháp) — never shown to players.
-- key_id names the salt (NETSIG_SALTS "id:secret,…"): while two salts are configured during a rotation every login is
-- hashed under both, so matching keeps working; comparisons only ever match hashes of the same key_id.
CREATE TABLE net_signals (
    user_id       UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    key_id        TEXT        NOT NULL CHECK (key_id ~ '^[a-z0-9]{1,16}$'),
    ip_hash       BYTEA       NOT NULL CHECK (octet_length(ip_hash) = 16),
    first_seen_at TIMESTAMPTZ NOT NULL,
    last_seen_at  TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (user_id, key_id, ip_hash),
    CHECK (last_seen_at >= first_seen_at)
);
CREATE INDEX net_signals_hash_idx ON net_signals (key_id, ip_hash, last_seen_at);
CREATE INDEX net_signals_seen_idx ON net_signals (last_seen_at);

-- ───── đạo lữ ─────
-- A proposal from one character to another. 'pending' rows past expires_at are expired (read lazily; turned into
-- 'expired' when they would block a new proposal). net_flag = the two accounts shared a network hash within 30 days at
-- proposal time (admin signal only, never blocks).
CREATE TABLE dao_lu_proposals (
    id                    UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    proposer_character_id UUID        NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    target_character_id   UUID        NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    status                TEXT        NOT NULL DEFAULT 'pending'
                                      CHECK (status IN ('pending', 'accepted', 'declined', 'cancelled', 'superseded', 'expired')),
    net_flag              BOOLEAN     NOT NULL DEFAULT false,
    idempotency_key       TEXT        NOT NULL,              -- key of the propose call
    game_day              DATE        NOT NULL,              -- daily proposal limit
    created_at            TIMESTAMPTZ NOT NULL,
    expires_at            TIMESTAMPTZ NOT NULL,
    responded_at          TIMESTAMPTZ,
    respond_key           TEXT,                              -- key of the accept/decline/cancel call (replays)
    UNIQUE (proposer_character_id, idempotency_key),
    CHECK (proposer_character_id <> target_character_id),
    CHECK (expires_at > created_at),
    CHECK ((status = 'pending') = (responded_at IS NULL)),
    CHECK (respond_key IS NULL OR responded_at IS NOT NULL)
);
-- One open proposal per (proposer, target); the service also allows only one open outgoing proposal per proposer.
CREATE UNIQUE INDEX dao_lu_proposals_open_pair_uq ON dao_lu_proposals (proposer_character_id, target_character_id) WHERE status = 'pending';
CREATE INDEX dao_lu_proposals_target_open_idx ON dao_lu_proposals (target_character_id, created_at DESC) WHERE status = 'pending';
CREATE INDEX dao_lu_proposals_proposer_idx ON dao_lu_proposals (proposer_character_id, created_at DESC);
CREATE INDEX dao_lu_proposals_flag_idx ON dao_lu_proposals (created_at DESC) WHERE net_flag;

-- A bond (permanent; ends only by luân hồi / Phong Thần, an admin after a report, or 90 days of absence).
CREATE TABLE dao_lu_bonds (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    proposal_id UUID        NOT NULL UNIQUE REFERENCES dao_lu_proposals (id) ON DELETE CASCADE,
    character_a UUID        NOT NULL REFERENCES characters (id) ON DELETE CASCADE, -- proposer
    character_b UUID        NOT NULL REFERENCES characters (id) ON DELETE CASCADE, -- accepter
    net_flag    BOOLEAN     NOT NULL DEFAULT false,
    formed_at   TIMESTAMPTZ NOT NULL,
    ended_at    TIMESTAMPTZ,
    end_reason  TEXT        CHECK (end_reason IN ('luan_hoi', 'phong_than', 'admin', 'absence')),
    CHECK (character_a <> character_b),
    CHECK ((ended_at IS NULL) = (end_reason IS NULL))
);
CREATE INDEX dao_lu_bonds_active_idx ON dao_lu_bonds (formed_at) WHERE ended_at IS NULL;

-- Both sides of every bond. THE invariant: a character is in at most one active bond, whichever side
-- (a cross-column rule a single index on dao_lu_bonds cannot express).
CREATE TABLE dao_lu_members (
    bond_id      UUID        NOT NULL REFERENCES dao_lu_bonds (id) ON DELETE CASCADE,
    character_id UUID        NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    active       BOOLEAN     NOT NULL DEFAULT true,
    ended_at     TIMESTAMPTZ,
    PRIMARY KEY (bond_id, character_id),
    CHECK (active = (ended_at IS NULL))
);
CREATE UNIQUE INDEX dao_lu_members_active_uq ON dao_lu_members (character_id) WHERE active;
CREATE INDEX dao_lu_members_history_idx ON dao_lu_members (character_id, ended_at DESC);

-- History / audit of every step (append-only).
CREATE TABLE dao_lu_events (
    id                 BIGSERIAL   PRIMARY KEY,
    bond_id            UUID        REFERENCES dao_lu_bonds (id) ON DELETE CASCADE,
    proposal_id        UUID        REFERENCES dao_lu_proposals (id) ON DELETE CASCADE,
    kind               TEXT        NOT NULL CHECK (kind IN ('proposed', 'declined', 'cancelled', 'bonded', 'ended', 'net_flag')),
    actor_character_id UUID        REFERENCES characters (id) ON DELETE SET NULL,
    admin_user_id      UUID        REFERENCES users (id),
    reason             TEXT        NOT NULL DEFAULT '',
    note               TEXT        NOT NULL DEFAULT '' CHECK (char_length(note) <= 1000),
    report_id          UUID        REFERENCES reports (id) ON DELETE SET NULL,
    created_at         TIMESTAMPTZ NOT NULL
);
CREATE INDEX dao_lu_events_bond_idx ON dao_lu_events (bond_id, id);
CREATE INDEX dao_lu_events_flag_idx ON dao_lu_events (created_at DESC) WHERE kind = 'net_flag';

-- Song tu payments. One row per claim; Σ seconds per (character, game day) ≤ dao_lu.yaml song_tu.window_seconds.
-- seconds_after = the character's paid seconds that day after this claim (raw = pay(seconds_after) − pay(before)).
CREATE TABLE dao_lu_song_tu_claims (
    id              BIGSERIAL   PRIMARY KEY,
    character_id    UUID        NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    bond_id         UUID        NOT NULL REFERENCES dao_lu_bonds (id) ON DELETE CASCADE,
    game_day        DATE        NOT NULL,
    seconds         INTEGER     NOT NULL CHECK (seconds > 0),
    seconds_after   INTEGER     NOT NULL CHECK (seconds_after >= seconds),
    raw_milli       BIGINT      NOT NULL CHECK (raw_milli >= 0),
    credited_milli  BIGINT      NOT NULL CHECK (credited_milli >= 0),
    vip_points      INTEGER     NOT NULL DEFAULT 0 CHECK (vip_points >= 0),
    idempotency_key TEXT        NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL,
    UNIQUE (character_id, idempotency_key)
);
CREATE INDEX dao_lu_song_tu_claims_day_idx ON dao_lu_song_tu_claims (character_id, game_day);
