-- name: GetRitualStreak :one
SELECT * FROM ritual_streaks WHERE character_id = $1;

-- name: UpsertRitualStreak :exec
-- Called under the progression lock (serializes the once-per-day offering).
INSERT INTO ritual_streaks (character_id, last_day, streak, best, updated_at)
VALUES (@character_id, @last_day, @streak, @best, @updated_at)
ON CONFLICT (character_id) DO UPDATE
SET last_day = EXCLUDED.last_day, streak = EXCLUDED.streak, best = EXCLUDED.best, updated_at = EXCLUDED.updated_at;

-- name: GetDailyFortune :one
SELECT * FROM daily_fortunes WHERE character_id = $1 AND game_day = $2;

-- name: InsertDailyFortune :exec
INSERT INTO daily_fortunes (character_id, game_day, outcome_id, roll_bp, bonus_bp, created_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetCharacterBuff :one
SELECT * FROM character_buffs WHERE character_id = $1 AND slot = $2;

-- name: UpsertCharacterBuff :exec
INSERT INTO character_buffs (character_id, slot, bonus_bp, starts_at, ends_at, source_ref)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (character_id, slot) DO UPDATE
SET bonus_bp = EXCLUDED.bonus_bp, starts_at = EXCLUDED.starts_at, ends_at = EXCLUDED.ends_at, source_ref = EXCLUDED.source_ref;
