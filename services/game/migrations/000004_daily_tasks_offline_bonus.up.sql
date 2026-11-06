-- Daily tasks (content/catalog/daily_tasks.yaml) and temporary offline-window bonus (Tụ Linh Phù).

-- Pending offline bonus: consumed by the first settlement it actually extends (elapsed > base VIP cap).
ALTER TABLE character_progressions
    ADD COLUMN offline_bonus_seconds  INTEGER NOT NULL DEFAULT 0 CHECK (offline_bonus_seconds BETWEEN 0 AND 86400),
    ADD COLUMN offline_bonus_used_day DATE;   -- game day the last bonus item was used (1 per day)

-- One row per (character, game day, task). Progress is capped at the task target by the app.
CREATE TABLE daily_task_progress (
    character_id UUID        NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    game_day     DATE        NOT NULL,
    task_id      TEXT        NOT NULL,     -- daily_tasks.yaml id
    progress     INTEGER     NOT NULL DEFAULT 0 CHECK (progress >= 0),
    claimed_at   TIMESTAMPTZ,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (character_id, game_day, task_id)
);
