-- Phase 1.5: player reports (offensive name, harassment) + admin resolution. Every resolution is audited in
-- admin_audit_logs; target_name snapshots the name at report time (the character may be renamed later).
CREATE TABLE reports (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reporter_user_id    UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    target_character_id UUID        NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    target_name         TEXT        NOT NULL,
    reason              TEXT        NOT NULL CHECK (reason IN ('offensive_name', 'harassment')),
    note                TEXT        NOT NULL DEFAULT '' CHECK (char_length(note) <= 1000),
    status              TEXT        NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'dismissed', 'actioned')),
    resolution_note     TEXT        NOT NULL DEFAULT '' CHECK (char_length(resolution_note) <= 1000),
    resolved_by         UUID        REFERENCES users (id),
    resolved_at         TIMESTAMPTZ,
    idempotency_key     TEXT        NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (reporter_user_id, idempotency_key),
    CHECK ((status = 'open') = (resolved_at IS NULL)),
    CHECK ((status = 'open') = (resolved_by IS NULL))
);
-- One OPEN report per reporter/target/reason (repeat submissions return the existing one).
CREATE UNIQUE INDEX reports_open_dedupe_uq ON reports (reporter_user_id, target_character_id, reason) WHERE status = 'open';
-- Admin queue (by status, oldest first) and per-reporter daily count.
CREATE INDEX reports_status_created_idx ON reports (status, created_at, id);
CREATE INDEX reports_reporter_created_idx ON reports (reporter_user_id, created_at);
-- Open reports against one target (admin "N reports" badge, bulk resolve after rename).
CREATE INDEX reports_target_open_idx ON reports (target_character_id, reason) WHERE status = 'open';
