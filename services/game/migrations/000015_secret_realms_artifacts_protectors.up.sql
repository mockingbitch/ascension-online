-- Phase 3.4: bí cảnh thử luyện + pháp bảo + hộ pháp (docs/design/bi-canh-luat-choi.md; content/catalog/secret_realms.yaml,
-- items.yaml artifacts, protector.yaml; decisions 2026-11-07 "Bí cảnh thử luyện + pháp bảo").
--
-- Lock order (data-model §2.3, extended):
--   character_progressions → daily_contribution → trial_runs → secret_realm_runs → secret_realm_days
--   → protector_invites → protector_days (by character_id) → item_daily_uses → character_faction → vip_accounts
--   → currency_accounts → inventory_items (by item_id) → shop_daily_counts.
-- A bí cảnh submit credits điểm duyên (vip_accounts) BEFORE the loot (currency, then items).
-- character_artifacts is locked only by the equip call (alone) and read by the breakthrough under the progression lock.

-- One row per server-issued bí cảnh run. The client submits its movement log once; the server replays it, writes the
-- verdict and grants the loot (currency_ledger / inventory_transactions, idempotency key 'sr:<run id>') in the same tx.
CREATE TABLE secret_realm_runs (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    character_id UUID        NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    realm_id     TEXT        NOT NULL,                       -- secret_realms.yaml id
    seed         BIGINT      NOT NULL CHECK (seed BETWEEN 1 AND 4294967295),
    params       JSONB       NOT NULL,                       -- rules snapshot at issue time (replay + judge use this)
    issue_key    TEXT        NOT NULL,                       -- Idempotency-Key of the start
    issued_at    TIMESTAMPTZ NOT NULL,
    expires_at   TIMESTAMPTZ NOT NULL,
    game_day     DATE        NOT NULL,                       -- game day of the start (runs/day)
    submit_key   TEXT,
    submitted_at TIMESTAMPTZ,
    claim        JSONB,                                      -- audit: moves + claimed end state as received
    result       JSONB,                                      -- audit: the replay outcome + cadence telemetry
    verdict      TEXT        CHECK (verdict IN ('pass', 'fail', 'rejected', 'expired')),
    reason       TEXT,
    verdict_at   TIMESTAMPTZ,
    band         TEXT,                                       -- result band (NULL = no reward)
    loot         JSONB,                                      -- granted loot after caps (+ rolled, for audit)
    UNIQUE (character_id, issue_key),
    CHECK (expires_at > issued_at),
    CHECK ((verdict IS NULL) = (verdict_at IS NULL)),
    CHECK ((verdict IS NULL) = (reason IS NULL)),
    CHECK (submitted_at IS NULL OR verdict IS NOT NULL),
    CHECK ((submit_key IS NULL) = (submitted_at IS NULL)),
    CHECK (band IS NULL OR verdict IN ('pass', 'fail')),
    CHECK ((band IS NULL) = (loot IS NULL))
);
-- At most one open run per character and realm.
CREATE UNIQUE INDEX secret_realm_runs_open_uq ON secret_realm_runs (character_id, realm_id) WHERE verdict IS NULL;
CREATE INDEX secret_realm_runs_char_idx ON secret_realm_runs (character_id, issued_at DESC);

-- Per character and game day ACROSS ALL REALMS: runs started (runs/day, at start) and rewards granted (daily caps, at
-- submit). The realm being played supplies the limits; the totals are per character (secret_realms.yaml).
CREATE TABLE secret_realm_days (
    character_id UUID    NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    game_day     DATE    NOT NULL,
    runs         INTEGER NOT NULL DEFAULT 0 CHECK (runs >= 0),
    spirit_stone BIGINT  NOT NULL DEFAULT 0 CHECK (spirit_stone >= 0),
    materials    BIGINT  NOT NULL DEFAULT 0 CHECK (materials >= 0),
    artifacts    INTEGER NOT NULL DEFAULT 0 CHECK (artifacts >= 0),
    PRIMARY KEY (character_id, game_day)
);

-- The ONE pháp bảo a character has equipped (counts toward the tribulation odds while still owned).
CREATE TABLE character_artifacts (
    character_id UUID        PRIMARY KEY REFERENCES characters (id) ON DELETE CASCADE,
    item_id      TEXT        NOT NULL,                       -- items.yaml artifact id
    equipped_at  TIMESTAMPTZ NOT NULL
);

-- Hộ pháp: the breakthrough-er opens an invite; online fellow faction members accept within the window; the next
-- tribulation attempt counts the accepted protectors who are still online and in the faction, then uses the invite.
CREATE TABLE protector_invites (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    character_id    UUID        NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    faction_id      TEXT        NOT NULL,
    stage_id        TEXT        NOT NULL,                    -- the tribulation stage the invite is for
    idempotency_key TEXT        NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL,
    expires_at      TIMESTAMPTZ NOT NULL,
    used_at         TIMESTAMPTZ,                             -- consumed by a tribulation attempt
    used_ref        TEXT,                                    -- 'bt:<breakthrough idempotency key>'
    UNIQUE (character_id, idempotency_key),
    CHECK (expires_at > created_at),
    CHECK ((used_at IS NULL) = (used_ref IS NULL))
);
CREATE INDEX protector_invites_open_idx ON protector_invites (faction_id, expires_at DESC) WHERE used_at IS NULL;
CREATE INDEX protector_invites_char_idx ON protector_invites (character_id, created_at DESC);

CREATE TABLE protector_assists (
    id                     BIGSERIAL   PRIMARY KEY,
    invite_id              UUID        NOT NULL REFERENCES protector_invites (id) ON DELETE CASCADE,
    protector_character_id UUID        NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    game_day               DATE        NOT NULL,             -- day the use was spent (protector_days)
    idempotency_key        TEXT        NOT NULL,
    accepted_at            TIMESTAMPTZ NOT NULL,
    counted_at             TIMESTAMPTZ,                      -- counted by the attempt that used the invite
    UNIQUE (invite_id, protector_character_id),              -- the same protector counts once
    UNIQUE (protector_character_id, idempotency_key)
);
CREATE INDEX protector_assists_protector_idx ON protector_assists (protector_character_id, accepted_at DESC);

-- One row per protector and game day: the lock that serializes a protector's accepts. A "use" is an assist accepted that
-- day that was COUNTED or is still pending on an open invite (uncounted / expired assists are not charged); `uses` is
-- that count as of the last accept (audit).
CREATE TABLE protector_days (
    character_id UUID    NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    game_day     DATE    NOT NULL,
    uses         INTEGER NOT NULL CHECK (uses >= 0),
    PRIMARY KEY (character_id, game_day)
);
