-- World chat (content/catalog/chat.yaml). Append-only log; the worker deletes rows older than retention_days.
-- body is what players see (masked); raw_body keeps the original only when masking changed it (moderation evidence).
CREATE TABLE chat_messages (
    id              BIGSERIAL PRIMARY KEY,
    channel         TEXT        NOT NULL CHECK (channel ~ '^[a-z][a-z0-9_]*$'),   -- chat.yaml channel id
    user_id         UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    character_id    UUID        NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    character_name  TEXT        NOT NULL,                                        -- snapshot at send time
    stage_id        TEXT        NOT NULL,                                        -- snapshot (realm colour)
    body            TEXT        NOT NULL CHECK (char_length(body) BETWEEN 1 AND 500),
    raw_body        TEXT        CHECK (raw_body IS NULL OR char_length(raw_body) <= 500),
    idempotency_key TEXT        NOT NULL,
    hidden_at       TIMESTAMPTZ,
    hidden_by       UUID        REFERENCES users (id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, idempotency_key),
    CHECK ((hidden_at IS NULL) = (hidden_by IS NULL))
);
-- History page per channel (newest first, keyset on id), visible messages only.
CREATE INDEX chat_messages_channel_visible_idx ON chat_messages (channel, id DESC) WHERE hidden_at IS NULL;
-- Duplicate-text check and admin lookups per sender.
CREATE INDEX chat_messages_user_created_idx ON chat_messages (user_id, created_at DESC);
-- Retention sweep.
CREATE INDEX chat_messages_created_idx ON chat_messages (created_at);

-- One row per muted account; muted_until in the past = not muted (rows are overwritten, history is in admin_audit_logs).
CREATE TABLE chat_mutes (
    user_id     UUID        PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    muted_until TIMESTAMPTZ NOT NULL,
    reason      TEXT        NOT NULL DEFAULT '' CHECK (char_length(reason) <= 1000),
    muted_by    UUID        NOT NULL REFERENCES users (id),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Reports may point at a chat message; evidence snapshots its text so it survives hiding and retention.
ALTER TABLE reports ADD COLUMN chat_message_id BIGINT REFERENCES chat_messages (id) ON DELETE SET NULL;
ALTER TABLE reports ADD COLUMN evidence TEXT NOT NULL DEFAULT '' CHECK (char_length(evidence) <= 1000);
