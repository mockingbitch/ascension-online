-- World announcements by effect tier (decisions 2026-11-09 #7 #8 #12, GĐ-D). Display only.

-- First character to reach each realm id (per realm id: tiên and Vu each have their own first). Written in the
-- breakthrough transaction with INSERT … ON CONFLICT DO NOTHING, so exactly one character ever wins (race-free).
CREATE TABLE realm_firsts (
    realm_id     TEXT        PRIMARY KEY CHECK (realm_id ~ '^[a-z][a-z0-9_]*$'),
    -- SET NULL: a deleted first character keeps the slot taken (the next arrival is not "first" again)
    character_id UUID        REFERENCES characters (id) ON DELETE SET NULL,
    reached_at   TIMESTAMPTZ NOT NULL
);

-- Short history of what the worker announced (GET /announcements/recent, the notification list, chat system lines
-- after a reconnect). Rows are rendered texts + the public fields they name (đạo hiệu, realm, rank) — never personal
-- data. source_event_id = the outbox event that produced it (replays are no-ops); digests and rank news have none.
-- Pruned by the worker (announcements.yaml history_keep_days).
CREATE TABLE world_announcements (
    id              BIGSERIAL   PRIMARY KEY,
    source_event_id BIGINT      UNIQUE,
    kind            TEXT        NOT NULL CHECK (kind IN ('realm', 'tribulation', 'first_to_realm', 'rank_top10', 'digest')),
    tier            SMALLINT    NOT NULL DEFAULT 0 CHECK (tier BETWEEN 0 AND 6),
    character_id    UUID        REFERENCES characters (id) ON DELETE SET NULL,
    name            TEXT        NOT NULL DEFAULT '',
    realm_id        TEXT,
    stage_id        TEXT,
    path            TEXT,
    rank            INTEGER     CHECK (rank IS NULL OR rank > 0),
    count           INTEGER     CHECK (count IS NULL OR count > 0),
    text            TEXT        NOT NULL CHECK (char_length(text) BETWEEN 1 AND 200),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX world_announcements_time_idx ON world_announcements (created_at DESC, id DESC);
