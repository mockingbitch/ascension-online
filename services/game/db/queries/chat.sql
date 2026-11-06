-- name: GetChatMessageByKey :one
SELECT * FROM chat_messages WHERE user_id = $1 AND idempotency_key = $2;

-- name: GetChatSender :one
-- Sender snapshot + posting gate input (caller holds LockUser, so sends serialise per player without
-- touching the hot progression row's lock).
SELECT c.id AS character_id, c.name, p.stage_id
FROM characters c JOIN character_progressions p ON p.character_id = c.id
WHERE c.user_id = $1;

-- name: GetChatMute :one
SELECT muted_until FROM chat_mutes WHERE user_id = $1;

-- name: RecentSameText :one
SELECT EXISTS (
    SELECT 1 FROM chat_messages
    WHERE user_id = @user_id AND created_at >= @since AND body = @body
) AS dup;

-- name: InsertChatMessage :one
INSERT INTO chat_messages (channel, user_id, character_id, character_name, stage_id, body, raw_body, idempotency_key, created_at)
VALUES (@channel, @user_id, @character_id, @character_name, @stage_id, @body, @raw_body, @idempotency_key, @now)
RETURNING *;

-- name: ListChatMessages :many
-- Newest first; @before_id = 0 means "from the newest".
SELECT id, channel, character_id, character_name, stage_id, body, created_at
FROM chat_messages
WHERE channel = @channel AND hidden_at IS NULL AND (@before_id::bigint = 0 OR id < @before_id::bigint)
ORDER BY id DESC
LIMIT @lim;

-- name: GetChatMessage :one
SELECT * FROM chat_messages WHERE id = $1;

-- name: GetChatMessageForUpdate :one
SELECT * FROM chat_messages WHERE id = $1 FOR UPDATE;

-- name: HideChatMessage :exec
UPDATE chat_messages SET hidden_at = @now, hidden_by = @admin WHERE id = @id AND hidden_at IS NULL;

-- name: GetChatMuteForUpdate :one
SELECT * FROM chat_mutes WHERE user_id = $1 FOR UPDATE;

-- name: UpsertChatMute :exec
INSERT INTO chat_mutes (user_id, muted_until, reason, muted_by, updated_at)
VALUES (@user_id, @muted_until, @reason, @muted_by, @now)
ON CONFLICT (user_id) DO UPDATE
SET muted_until = EXCLUDED.muted_until, reason = EXCLUDED.reason, muted_by = EXCLUDED.muted_by, updated_at = EXCLUDED.updated_at;

-- name: DeleteOldChatMessages :execrows
-- Retention sweep in bounded batches (the worker loops until fewer than @lim rows are deleted).
DELETE FROM chat_messages d WHERE d.id IN (
    SELECT o.id FROM chat_messages o WHERE o.created_at < @before ORDER BY o.created_at LIMIT @lim
);
