-- Reverts the schema of 000005. Data is NOT restored (the up migration reset character/Tiên Duyên data);
-- character rows are truncated again so the v1 columns come back consistent.
DROP TABLE IF EXISTS world_daily_totals;
DROP TABLE IF EXISTS world_state;
DROP TABLE IF EXISTS late_join_grants;
DROP TABLE IF EXISTS item_daily_uses;
DROP TABLE IF EXISTS cultivation_credits;
DROP TABLE IF EXISTS daily_contribution;

TRUNCATE characters CASCADE;
DELETE FROM outbox_events;

ALTER TABLE breakthrough_attempts
    DROP CONSTRAINT breakthrough_attempts_guaranteed_chk,
    DROP CONSTRAINT breakthrough_attempts_loss_chk,
    DROP COLUMN odds,
    DROP COLUMN loss_milli,
    DROP COLUMN cultivation_before_milli,
    DROP COLUMN guaranteed,
    DROP COLUMN tribulation;

ALTER TABLE cultivation_settlements
    DROP COLUMN game_day,
    DROP COLUMN multiplier_bp,
    DROP COLUMN raw_milli;
ALTER TABLE cultivation_settlements RENAME COLUMN cultivation_after_milli TO cultivation_after;
ALTER TABLE cultivation_settlements RENAME COLUMN credited_milli TO earned;
ALTER TABLE cultivation_settlements RENAME COLUMN rate_milli_per_hour TO rate_milli;

ALTER TABLE character_progressions
    DROP COLUMN grant_reserve_milli,
    DROP COLUMN rate_carry,
    ADD COLUMN remainder_milli INTEGER NOT NULL DEFAULT 0 CHECK (remainder_milli BETWEEN 0 AND 999);
ALTER TABLE character_progressions RENAME COLUMN cultivation_milli TO cultivation;
