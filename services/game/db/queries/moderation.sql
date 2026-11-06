-- name: LockUser :one
-- Serialises one reporter's submissions (daily limit check).
SELECT id FROM users WHERE id = $1 FOR UPDATE;

-- name: GetReportByKey :one
SELECT * FROM reports WHERE reporter_user_id = $1 AND idempotency_key = $2;

-- name: GetOpenReport :one
SELECT * FROM reports
WHERE reporter_user_id = $1 AND target_character_id = $2 AND reason = $3 AND status = 'open';

-- name: CountReportsSince :one
SELECT count(*) FROM reports WHERE reporter_user_id = $1 AND created_at >= @since;

-- name: InsertReport :one
INSERT INTO reports (reporter_user_id, target_character_id, target_name, reason, note, idempotency_key, created_at,
                     chat_message_id, evidence)
VALUES (@reporter_user_id, @target_character_id, @target_name, @reason, @note, @idempotency_key, @now,
        sqlc.narg(chat_message_id), @evidence)
RETURNING *;

-- name: GetReportForUpdate :one
SELECT * FROM reports WHERE id = $1 FOR UPDATE;

-- name: ResolveReport :one
UPDATE reports SET status = @status, resolution_note = @note, resolved_by = @admin, resolved_at = @now
WHERE id = @id AND status = 'open'
RETURNING *;

-- name: ResolveOpenReportsForTarget :many
-- After a rename, close every other open offensive-name report against the same character.
UPDATE reports SET status = 'actioned', resolution_note = @note, resolved_by = @admin, resolved_at = @now
WHERE target_character_id = @target AND reason = @reason AND status = 'open' AND id <> @except_id
RETURNING id;

-- name: ListReports :many
SELECT r.*, t.name AS target_current_name, t.user_id AS target_user_id,
       COALESCE(rc.name, '')::text AS reporter_name,
       (SELECT count(*) FROM reports o WHERE o.target_character_id = r.target_character_id AND o.status = 'open') AS open_against_target
FROM reports r
JOIN characters t ON t.id = r.target_character_id
LEFT JOIN characters rc ON rc.user_id = r.reporter_user_id
WHERE r.status = @status
ORDER BY r.created_at, r.id
LIMIT @lim OFFSET @off;
