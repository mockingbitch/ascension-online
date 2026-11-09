-- Tutorial progress (decisions 2026-11-09 "Ngoại hình, hướng dẫn viên…", GĐ-B): which of Vân Khê's "lần đầu" steps a
-- character has seen, so it carries over to another device. Steps are ids from content/story/tutorial/*.yaml (no FK).
-- No gameplay effect: nothing here is read by cultivation, breakthrough, ranking, economy or Tiên Duyên, and a step
-- never grants anything.
CREATE TABLE tutorial_progress (
    character_id    UUID        NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    step_id         TEXT        NOT NULL CHECK (step_id ~ '^[a-z][a-z0-9_]*$'),
    status          TEXT        NOT NULL CHECK (status IN ('done', 'skipped')),
    idempotency_key TEXT        NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (character_id, step_id),
    UNIQUE (character_id, idempotency_key)
);

-- "Không nhắc lại": the guide stops offering steps on her own (the "?" button can still summon her). The row is also
-- the per-character lock of every tutorial write (Mark / skip-all / reset serialise on it).
CREATE TABLE tutorial_prefs (
    character_id UUID        PRIMARY KEY REFERENCES characters (id) ON DELETE CASCADE,
    muted        BOOLEAN     NOT NULL DEFAULT false,
    -- Idempotency of the settings actions (skip-all / reset): a retry with the last key replays instead of re-applying
    -- (a lost reset answer must not wipe steps marked since on another device).
    last_key     TEXT,
    version      BIGINT      NOT NULL DEFAULT 0,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
