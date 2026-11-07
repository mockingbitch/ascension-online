-- Trao đổi vật phẩm (internal/trade). Lock order: trade_offers → trade_days (by character_id) → currency_accounts →
-- inventory_items (migration 000018).

-- name: GetTradeParty :one
-- A character with its account (age gates), stage (tier gate) and faction (relation).
SELECT c.id, c.user_id, c.name, c.created_at, u.created_at AS account_created_at, p.stage_id,
       COALESCE(f.faction_id, '')::text AS faction_id
FROM characters c
JOIN users u ON u.id = c.user_id
JOIN character_progressions p ON p.character_id = c.id
LEFT JOIN character_faction f ON f.character_id = c.id
WHERE c.id = $1;

-- name: TradeDaoLuBonded :one
-- Are the two characters each other's active đạo lữ?
SELECT EXISTS (
    SELECT 1 FROM dao_lu_members a JOIN dao_lu_members b ON b.bond_id = a.bond_id AND b.active
    WHERE a.character_id = @a AND a.active AND b.character_id = @b
)::BOOLEAN AS bonded;

-- name: InsertTradeOffer :one
INSERT INTO trade_offers (kind, sender_character_id, receiver_character_id, relation, net_flag, value_to_receiver, value_to_sender,
                          idempotency_key, game_day, created_at, expires_at)
VALUES (@kind, @sender_character_id, @receiver_character_id, @relation, @net_flag, @value_to_receiver, @value_to_sender,
        @idempotency_key, @game_day, @created_at, @expires_at)
RETURNING *;

-- name: InsertTradeLine :exec
INSERT INTO trade_lines (offer_id, side, asset_kind, asset_id, quantity) VALUES (@offer_id, @side, @asset_kind, @asset_id, @quantity);

-- name: GetTradeOfferByKey :one
SELECT * FROM trade_offers WHERE sender_character_id = $1 AND idempotency_key = $2;

-- name: GetTradeOffer :one
SELECT * FROM trade_offers WHERE id = $1;

-- name: GetTradeOfferForUpdate :one
SELECT * FROM trade_offers WHERE id = $1 FOR UPDATE;

-- name: ListTradeLines :many
SELECT * FROM trade_lines WHERE offer_id = ANY(@ids::uuid[]) ORDER BY offer_id, side, asset_id;

-- name: CountOpenTradeOffers :one
SELECT count(*)::INT FROM trade_offers WHERE sender_character_id = @character_id AND status = 'pending' AND expires_at > @now;

-- name: RespondTradeOffer :execrows
-- decline / cancel / expire (accept uses AcceptTradeOffer).
UPDATE trade_offers SET status = @status, responded_at = @responded_at, respond_key = sqlc.narg(respond_key)
WHERE id = @id AND status = 'pending';

-- name: AcceptTradeOffer :execrows
UPDATE trade_offers SET status = 'accepted', responded_at = @responded_at, respond_key = @respond_key, accepted_day = @accepted_day,
       relation = @relation, value_to_receiver = @value_to_receiver, value_to_sender = @value_to_sender,
       sender_fee = @sender_fee, receiver_fee = @receiver_fee
WHERE id = @id AND status = 'pending';

-- name: ExpireTradeOffers :many
-- Worker: turn at most @lim lapsed pending offers into 'expired' (SKIP LOCKED: an offer being answered is left alone).
UPDATE trade_offers o SET status = 'expired', responded_at = o.expires_at
FROM (SELECT t.id FROM trade_offers t WHERE t.status = 'pending' AND t.expires_at <= @now ORDER BY t.expires_at LIMIT @lim FOR UPDATE SKIP LOCKED) due
WHERE o.id = due.id
RETURNING o.id;

-- name: UpsertTradeDay :exec
INSERT INTO trade_days (character_id, game_day) VALUES (@character_id, @game_day) ON CONFLICT DO NOTHING;

-- name: GetTradeDayForUpdate :one
SELECT * FROM trade_days WHERE character_id = @character_id AND game_day = @game_day FOR UPDATE;

-- name: GetTradeDay :one
SELECT * FROM trade_days WHERE character_id = @character_id AND game_day = @game_day;

-- name: BumpTradeOffersCreated :exec
UPDATE trade_days SET offers_created = offers_created + 1 WHERE character_id = @character_id AND game_day = @game_day;

-- name: AddTradeDayCompleted :exec
UPDATE trade_days SET completed = completed + 1, value_received = value_received + @received, value_sent = value_sent + @sent
WHERE character_id = @character_id AND game_day = @game_day;

-- name: InsertTradeEvent :exec
INSERT INTO trade_events (offer_id, kind, actor_character_id, detail, created_at)
VALUES (@offer_id, @kind, sqlc.narg(actor_character_id), sqlc.narg(detail), @created_at);

-- name: ListTradeOffersOpen :many
-- Pending, unexpired offers where the character is the receiver (@incoming) or the sender (NOT @incoming).
SELECT o.*, c.name AS counterpart_name, p.stage_id AS counterpart_stage_id
FROM trade_offers o
JOIN characters c ON c.id = CASE WHEN @incoming::boolean THEN o.sender_character_id ELSE o.receiver_character_id END
JOIN character_progressions p ON p.character_id = c.id
WHERE o.status = 'pending' AND o.expires_at > @now
  AND ((@incoming::boolean AND o.receiver_character_id = @character_id) OR (NOT @incoming::boolean AND o.sender_character_id = @character_id))
ORDER BY o.created_at DESC LIMIT @lim;

-- name: ListTradeHistory :many
-- Answered or lapsed offers of the character (either side), newest first.
SELECT o.*, c.name AS counterpart_name, p.stage_id AS counterpart_stage_id
FROM trade_offers o
JOIN characters c ON c.id = CASE WHEN o.sender_character_id = @character_id THEN o.receiver_character_id ELSE o.sender_character_id END
JOIN character_progressions p ON p.character_id = c.id
WHERE (o.sender_character_id = @character_id OR o.receiver_character_id = @character_id)
  AND (o.status <> 'pending' OR o.expires_at <= @now)
ORDER BY o.created_at DESC LIMIT @lim;

-- name: GetTradeOfferBrief :one
-- Report: the parties of an offer.
SELECT o.id, o.kind, o.status, o.sender_character_id, o.receiver_character_id, cs.user_id AS sender_user_id, cr.user_id AS receiver_user_id
FROM trade_offers o JOIN characters cs ON cs.id = o.sender_character_id JOIN characters cr ON cr.id = o.receiver_character_id
WHERE o.id = $1;

-- ───── admin: abnormal one-way flows (accepted trades since @since, stone-equivalent values) ─────

-- name: TradeTopNetReceivers :many
WITH flows AS (
    SELECT t.receiver_character_id AS to_char, t.sender_character_id AS from_char, t.value_to_receiver AS v
    FROM trade_offers t WHERE t.status = 'accepted' AND t.responded_at >= @since AND t.value_to_receiver > 0
    UNION ALL
    SELECT t2.sender_character_id, t2.receiver_character_id, t2.value_to_sender
    FROM trade_offers t2 WHERE t2.status = 'accepted' AND t2.responded_at >= @since AND t2.value_to_sender > 0
), per AS (
    SELECT to_char AS character_id, v AS received, 0::BIGINT AS sent, from_char AS peer, 1 AS n_in FROM flows
    UNION ALL
    SELECT from_char, 0::BIGINT, v, NULL::uuid, 0 FROM flows
)
SELECT per.character_id, c.name, sum(per.received)::BIGINT AS received, sum(per.sent)::BIGINT AS sent,
       (sum(per.received) - sum(per.sent))::BIGINT AS net, count(DISTINCT per.peer)::INT AS distinct_senders,
       sum(per.n_in)::INT AS trades_in
FROM per JOIN characters c ON c.id = per.character_id
GROUP BY per.character_id, c.name
HAVING sum(per.received) - sum(per.sent) > 0
ORDER BY net DESC, per.character_id
LIMIT @lim;

-- name: TradeLopsidedPairs :many
-- Pairs whose total value ≥ @min_value and whose larger direction is ≥ @min_bp of the total.
WITH flows AS (
    SELECT t.sender_character_id AS from_char, t.receiver_character_id AS to_char, t.value_to_receiver AS v
    FROM trade_offers t WHERE t.status = 'accepted' AND t.responded_at >= @since AND t.value_to_receiver > 0
    UNION ALL
    SELECT t2.receiver_character_id, t2.sender_character_id, t2.value_to_sender
    FROM trade_offers t2 WHERE t2.status = 'accepted' AND t2.responded_at >= @since AND t2.value_to_sender > 0
), pr AS (
    SELECT LEAST(from_char, to_char)::uuid AS a, GREATEST(from_char, to_char)::uuid AS b,
           COALESCE(sum(v) FILTER (WHERE from_char < to_char), 0)::BIGINT AS a_to_b,
           COALESCE(sum(v) FILTER (WHERE from_char > to_char), 0)::BIGINT AS b_to_a,
           count(*)::INT AS trades
    FROM flows GROUP BY 1, 2
), px AS (
    SELECT pr.*, (pr.a_to_b + pr.b_to_a) AS total,
           (GREATEST(pr.a_to_b, pr.b_to_a) * 10000 / GREATEST(pr.a_to_b + pr.b_to_a, 1))::BIGINT AS major_bp FROM pr
)
SELECT px.a, ca.name AS a_name, px.b, cb.name AS b_name, px.a_to_b, px.b_to_a, px.trades
FROM px JOIN characters ca ON ca.id = px.a JOIN characters cb ON cb.id = px.b
WHERE px.total >= @min_value::BIGINT AND px.major_bp >= @min_bp::BIGINT
ORDER BY px.total DESC, px.a
LIMIT @lim;

-- name: TradeFunnels :many
-- Many-to-one: receivers fed by ≥ @min_senders distinct characters.
WITH flows AS (
    SELECT t.receiver_character_id AS to_char, t.sender_character_id AS from_char, t.value_to_receiver AS v
    FROM trade_offers t WHERE t.status = 'accepted' AND t.responded_at >= @since AND t.value_to_receiver > 0
    UNION ALL
    SELECT t2.sender_character_id, t2.receiver_character_id, t2.value_to_sender
    FROM trade_offers t2 WHERE t2.status = 'accepted' AND t2.responded_at >= @since AND t2.value_to_sender > 0
)
SELECT flows.to_char AS character_id, c.name, count(DISTINCT flows.from_char)::INT AS distinct_senders,
       sum(flows.v)::BIGINT AS received, count(*)::INT AS trades
FROM flows JOIN characters c ON c.id = flows.to_char
GROUP BY flows.to_char, c.name
HAVING count(DISTINCT flows.from_char) >= @min_senders::INT
ORDER BY distinct_senders DESC, received DESC, flows.to_char
LIMIT @lim;

-- name: ListTradeFlags :many
-- Offers between accounts that shared a network hash (netsig) when created, newest first.
SELECT o.id, o.kind, o.status, o.created_at, o.value_to_receiver, o.value_to_sender,
       o.sender_character_id, cs.name AS sender_name, o.receiver_character_id, cr.name AS receiver_name
FROM trade_offers o JOIN characters cs ON cs.id = o.sender_character_id JOIN characters cr ON cr.id = o.receiver_character_id
WHERE o.net_flag AND o.created_at >= @since
ORDER BY o.created_at DESC LIMIT @lim;
