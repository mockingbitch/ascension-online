-- Tử Tiêu Cung lectures (content/catalog/lectures.yaml). Presence/seats live in Redis (rebuildable); only paid
-- ngộ đạo moments are durable. One row per character per ngộ đạo slot = the idempotency guard.
CREATE TABLE lecture_insights (
    character_id      UUID        NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    slot_no           BIGINT      NOT NULL CHECK (slot_no >= 0),          -- floor((t − epoch) / every_seconds)
    game_day          DATE        NOT NULL,
    lecture_id        TEXT        NOT NULL DEFAULT '',                    -- script being taught (Đạo ngộ notebook)
    presence_seconds  INTEGER     NOT NULL CHECK (presence_seconds >= 0), -- accrued listening paid by this row
    raw_milli         BIGINT      NOT NULL CHECK (raw_milli >= 0),        -- after the lecture daily cap
    credited_milli    BIGINT      NOT NULL CHECK (credited_milli >= 0),   -- after bucket/ceiling/bottleneck
    vip_points        INTEGER     NOT NULL DEFAULT 0 CHECK (vip_points >= 0),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (character_id, slot_no)
);
CREATE INDEX lecture_insights_day_idx ON lecture_insights (character_id, game_day);

-- Điểm duyên earned in game (decision D1b): reason 'activity', the activity in ref_type (e.g. 'lecture_insight').
ALTER TABLE vip_point_ledger DROP CONSTRAINT vip_point_ledger_reason_check;
ALTER TABLE vip_point_ledger ADD CONSTRAINT vip_point_ledger_reason_check
    CHECK (reason IN ('purchase', 'admin_grant', 'admin_revoke', 'refund', 'promo', 'activity'));
