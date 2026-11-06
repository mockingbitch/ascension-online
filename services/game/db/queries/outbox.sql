-- name: InsertOutboxEvent :exec
INSERT INTO outbox_events (topic, aggregate_id, payload)
VALUES ($1, $2, $3);

-- name: ClaimPendingOutbox :many
-- Use inside a tx; SKIP LOCKED lets several workers run safely.
SELECT * FROM outbox_events
WHERE published_at IS NULL
ORDER BY id
LIMIT $1
FOR UPDATE SKIP LOCKED;

-- name: MarkOutboxPublished :exec
UPDATE outbox_events SET published_at = now(), attempts = attempts + 1
WHERE id = ANY(@ids::bigint[]);
