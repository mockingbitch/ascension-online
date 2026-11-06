DROP TABLE IF EXISTS daily_task_progress;
ALTER TABLE character_progressions
    DROP COLUMN IF EXISTS offline_bonus_used_day,
    DROP COLUMN IF EXISTS offline_bonus_seconds;
