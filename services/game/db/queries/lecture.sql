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

-- name: GetLectureAffinity :one
-- Hall assignment hints (decisions 2026-11-10): the character's faction and active đạo lữ ('' when none).
SELECT COALESCE((SELECT f.faction_id FROM character_faction f WHERE f.character_id = @character_id), '')::TEXT AS faction_id,
       COALESCE((SELECT (CASE WHEN b.character_a = @character_id THEN b.character_b ELSE b.character_a END)::TEXT
                 FROM dao_lu_members m JOIN dao_lu_bonds b ON b.id = m.bond_id
                 WHERE m.character_id = @character_id AND m.active AND b.ended_at IS NULL LIMIT 1), '')::TEXT AS partner_id;
