-- Salted IP-hash network signal (internal/netsig). Never stores or returns a raw IP.

-- name: UpsertNetSignal :exec
INSERT INTO net_signals (user_id, key_id, ip_hash, first_seen_at, last_seen_at)
VALUES (@user_id, @key_id, @ip_hash, @seen_at, @seen_at)
ON CONFLICT (user_id, key_id, ip_hash) DO UPDATE SET last_seen_at = GREATEST(net_signals.last_seen_at, EXCLUDED.last_seen_at);

-- name: NetSignalsShared :one
-- Do two accounts share any network hash (same salt key) both seen since @since?
SELECT EXISTS (
    SELECT 1 FROM net_signals a JOIN net_signals b ON b.key_id = a.key_id AND b.ip_hash = a.ip_hash
    WHERE a.user_id = @user_a AND b.user_id = @user_b AND a.last_seen_at >= @since AND b.last_seen_at >= @since
)::BOOLEAN AS shared;

-- name: ListNetSignalGroups :many
-- Admin: network hashes seen by ≥ @min_accounts accounts since @since. The hash itself is only a grouping handle
-- inside the service and is never returned by the API.
SELECT key_id, ip_hash, array_agg(DISTINCT user_id ORDER BY user_id)::uuid[] AS users, max(last_seen_at)::timestamptz AS last_seen_at
FROM net_signals
WHERE last_seen_at >= @since
GROUP BY key_id, ip_hash
HAVING count(DISTINCT user_id) >= @min_accounts::BIGINT
ORDER BY count(DISTINCT user_id) DESC, max(last_seen_at) DESC
LIMIT @lim;

-- name: ListUserBriefs :many
-- Account → character (if any) for the admin cluster view.
SELECT u.id AS user_id, u.email, c.id AS character_id, c.name AS character_name
FROM users u LEFT JOIN characters c ON c.user_id = u.id
WHERE u.id = ANY(@ids::uuid[]);

-- name: PurgeNetSignals :execrows
-- Delete at most @lim rows unseen since @cutoff (30-day retention; the worker loops in bounded batches).
DELETE FROM net_signals n
USING (SELECT s.user_id, s.key_id, s.ip_hash FROM net_signals s WHERE s.last_seen_at < @cutoff ORDER BY s.last_seen_at LIMIT @lim) old
WHERE n.user_id = old.user_id AND n.key_id = old.key_id AND n.ip_hash = old.ip_hash;

-- name: NetSignalsSharedByCharacters :one
-- SharedRecently for two characters (their accounts), e.g. hộ pháp pairs.
SELECT EXISTS (
    SELECT 1 FROM characters ca JOIN characters cb ON cb.id = @character_b
    JOIN net_signals a ON a.user_id = ca.user_id
    JOIN net_signals b ON b.user_id = cb.user_id AND b.key_id = a.key_id AND b.ip_hash = a.ip_hash
    WHERE ca.id = @character_a AND ca.user_id <> cb.user_id AND a.last_seen_at >= @since AND b.last_seen_at >= @since
)::BOOLEAN AS shared;
