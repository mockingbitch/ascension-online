-- name: GetLectureInsight :one
SELECT * FROM lecture_insights WHERE character_id = $1 AND slot_no = $2;

-- name: SumLectureDay :one
SELECT COALESCE(SUM(raw_milli), 0)::BIGINT AS raw_milli FROM lecture_insights WHERE character_id = $1 AND game_day = $2;

-- name: InsertLectureInsight :exec
INSERT INTO lecture_insights (character_id, slot_no, game_day, lecture_id, presence_seconds, raw_milli, credited_milli, vip_points, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: ListLectureNotebook :many
-- Distinct lectures a character has had a paid ngộ đạo in, first time first.
SELECT lecture_id, MIN(created_at)::TIMESTAMPTZ AS first_at, COUNT(*)::BIGINT AS times
FROM lecture_insights WHERE character_id = $1 AND lecture_id <> ''
GROUP BY lecture_id ORDER BY MIN(created_at);
