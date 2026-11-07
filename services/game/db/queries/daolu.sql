-- Đạo lữ song tu (internal/daolu).

-- name: GetDaoLuParty :one
-- A character with its account (age gate) and stage.
SELECT c.id, c.user_id, c.name, c.created_at, u.created_at AS account_created_at, p.stage_id
FROM characters c JOIN users u ON u.id = c.user_id JOIN character_progressions p ON p.character_id = c.id
WHERE c.id = $1;

-- name: InsertDaoLuProposal :one
INSERT INTO dao_lu_proposals (proposer_character_id, target_character_id, net_flag, idempotency_key, game_day, created_at, expires_at)
VALUES (@proposer_character_id, @target_character_id, @net_flag, @idempotency_key, @game_day, @created_at, @expires_at)
RETURNING *;

-- name: GetDaoLuProposalByKey :one
SELECT * FROM dao_lu_proposals WHERE proposer_character_id = $1 AND idempotency_key = $2;

-- name: GetDaoLuProposal :one
SELECT * FROM dao_lu_proposals WHERE id = $1;

-- name: GetDaoLuProposalForUpdate :one
SELECT * FROM dao_lu_proposals WHERE id = $1 FOR UPDATE;

-- name: ExpireDaoLuProposals :exec
-- Turn the characters' lapsed pending proposals (either side) into 'expired' (lazy expiry). Callers lock the pending rows
-- of the same characters first (LockDaoLuPendingFor, id order).
UPDATE dao_lu_proposals SET status = 'expired', responded_at = expires_at
WHERE status = 'pending' AND expires_at <= @now
  AND (proposer_character_id = ANY(@ids::uuid[]) OR target_character_id = ANY(@ids::uuid[]));

-- name: CountDaoLuOpenOutgoing :one
SELECT count(*)::INT FROM dao_lu_proposals WHERE proposer_character_id = @character_id AND status = 'pending' AND expires_at > @now;

-- name: CountDaoLuProposalsDay :one
SELECT count(*)::INT FROM dao_lu_proposals WHERE proposer_character_id = @character_id AND game_day = @game_day;

-- name: RespondDaoLuProposal :execrows
UPDATE dao_lu_proposals SET status = @status, responded_at = @responded_at, respond_key = @respond_key
WHERE id = @id AND status = 'pending';

-- name: SupersedeDaoLuProposals :exec
-- Once two characters bond, every other pending proposal involving either of them is void.
UPDATE dao_lu_proposals SET status = 'superseded', responded_at = @now
WHERE status = 'pending' AND id <> @except_id
  AND (proposer_character_id = ANY(@ids::uuid[]) OR target_character_id = ANY(@ids::uuid[]));

-- name: ListDaoLuIncoming :many
SELECT pr.id, pr.proposer_character_id AS character_id, c.name, p.stage_id, pr.created_at, pr.expires_at
FROM dao_lu_proposals pr
JOIN characters c ON c.id = pr.proposer_character_id
JOIN character_progressions p ON p.character_id = pr.proposer_character_id
WHERE pr.target_character_id = @character_id AND pr.status = 'pending' AND pr.expires_at > @now
ORDER BY pr.created_at DESC LIMIT @lim;

-- name: ListDaoLuOutgoing :many
SELECT pr.id, pr.target_character_id AS character_id, c.name, p.stage_id, pr.created_at, pr.expires_at
FROM dao_lu_proposals pr
JOIN characters c ON c.id = pr.target_character_id
JOIN character_progressions p ON p.character_id = pr.target_character_id
WHERE pr.proposer_character_id = @character_id AND pr.status = 'pending' AND pr.expires_at > @now
ORDER BY pr.created_at DESC LIMIT @lim;

-- name: InsertDaoLuBond :one
INSERT INTO dao_lu_bonds (proposal_id, character_a, character_b, net_flag, formed_at)
VALUES (@proposal_id, @character_a, @character_b, @net_flag, @formed_at)
RETURNING *;

-- name: InsertDaoLuMember :exec
INSERT INTO dao_lu_members (bond_id, character_id) VALUES (@bond_id, @character_id);

-- name: GetActiveDaoLuBond :one
-- The character's active bond (at most one: dao_lu_members_active_uq).
SELECT b.* FROM dao_lu_members m JOIN dao_lu_bonds b ON b.id = m.bond_id
WHERE m.character_id = $1 AND m.active;

-- name: GetDaoLuBondForUpdate :one
SELECT * FROM dao_lu_bonds WHERE id = $1 FOR UPDATE;

-- name: EndDaoLuBond :execrows
UPDATE dao_lu_bonds SET ended_at = @ended_at, end_reason = @end_reason WHERE id = @id AND ended_at IS NULL;

-- name: EndDaoLuMembers :exec
UPDATE dao_lu_members SET active = false, ended_at = @ended_at WHERE bond_id = @bond_id AND active;

-- name: LastDaoLuEndedAt :one
-- When the character's most recent bond ended ('epoch' = never).
SELECT COALESCE(max(ended_at), 'epoch'::timestamptz)::timestamptz AS last_ended
FROM dao_lu_members WHERE character_id = $1 AND NOT active;

-- name: InsertDaoLuEvent :exec
INSERT INTO dao_lu_events (bond_id, proposal_id, kind, actor_character_id, admin_user_id, reason, note, report_id, created_at)
VALUES (@bond_id, @proposal_id, @kind, @actor_character_id, @admin_user_id, @reason, @note, @report_id, @created_at);

-- name: ListAbsentDaoLuBonds :many
-- Active bonds with a member whose last sign of play (settlement, session refresh, character creation) is before @cutoff.
SELECT DISTINCT b.id FROM dao_lu_bonds b
JOIN dao_lu_members m ON m.bond_id = b.id AND m.active
JOIN characters c ON c.id = m.character_id
JOIN character_progressions p ON p.character_id = c.id
WHERE b.ended_at IS NULL
  AND GREATEST(p.last_cultivated_at, c.created_at,
               COALESCE((SELECT max(r.created_at) FROM refresh_tokens r WHERE r.user_id = c.user_id), c.created_at)) < @cutoff
LIMIT @lim;

-- name: DaoLuLastSeen :one
-- Last sign of play of one character (same rule as ListAbsentDaoLuBonds).
SELECT GREATEST(p.last_cultivated_at, c.created_at,
                COALESCE((SELECT max(r.created_at) FROM refresh_tokens r WHERE r.user_id = c.user_id), c.created_at))::timestamptz AS last_seen
FROM characters c JOIN character_progressions p ON p.character_id = c.id WHERE c.id = $1;

-- name: ListDaoLuFlags :many
-- Admin: proposals/bonds flagged because the two accounts shared a network hash (newest first).
SELECT e.id, e.bond_id, e.proposal_id, e.created_at, e.note,
       pr.proposer_character_id, cp.name AS proposer_name, pr.target_character_id, ct.name AS target_name, pr.status
FROM dao_lu_events e
JOIN dao_lu_proposals pr ON pr.id = e.proposal_id
JOIN characters cp ON cp.id = pr.proposer_character_id
JOIN characters ct ON ct.id = pr.target_character_id
WHERE e.kind = 'net_flag' AND e.created_at >= @since
ORDER BY e.created_at DESC LIMIT @lim;

-- name: SumSongTuDay :one
SELECT COALESCE(sum(seconds), 0)::INT FROM dao_lu_song_tu_claims WHERE character_id = @character_id AND game_day = @game_day;

-- name: GetSongTuClaimByKey :one
SELECT * FROM dao_lu_song_tu_claims WHERE character_id = $1 AND idempotency_key = $2;

-- name: InsertSongTuClaim :one
INSERT INTO dao_lu_song_tu_claims (character_id, bond_id, game_day, seconds, seconds_after, raw_milli, credited_milli, vip_points,
                                   idempotency_key, created_at)
VALUES (@character_id, @bond_id, @game_day, @seconds, @seconds_after, @raw_milli, @credited_milli, @vip_points, @idempotency_key, @created_at)
RETURNING *;

-- name: GetReportParties :one
-- Admin termination must cite a report about (or by) one of the partners.
SELECT r.reporter_user_id, r.target_character_id, r.status FROM reports r WHERE r.id = $1;

-- name: LockDaoLuPendingFor :many
-- Accept locks every pending proposal of both characters in id order BEFORE bonding (it supersedes them): two
-- concurrent accepts sharing a character serialize here instead of deadlocking on each other's proposal rows.
SELECT id FROM dao_lu_proposals
WHERE status = 'pending' AND (proposer_character_id = ANY(@ids::uuid[]) OR target_character_id = ANY(@ids::uuid[]))
ORDER BY id
FOR UPDATE;

-- name: LastDaoLuDecline :one
-- When the target last declined a proposal from this proposer ('epoch' = never): per-pair cooldown (review L2).
SELECT COALESCE(max(responded_at), 'epoch'::timestamptz)::timestamptz AS last_declined
FROM dao_lu_proposals
WHERE proposer_character_id = @proposer_character_id AND target_character_id = @target_character_id AND status = 'declined';

-- name: CountDaoLuIncomingOpen :one
SELECT count(*)::INT FROM dao_lu_proposals WHERE target_character_id = @character_id AND status = 'pending' AND expires_at > @now;

-- name: DeleteOldDaoLuNetFlagEvents :execrows
-- Retention (review M2): net_flag audit events older than the netsig window go, in bounded batches.
DELETE FROM dao_lu_events WHERE id IN (
    SELECT o.id FROM dao_lu_events o WHERE o.kind = 'net_flag' AND o.created_at < @cutoff ORDER BY o.id LIMIT @lim);

-- name: ClearOldDaoLuProposalFlags :execrows
UPDATE dao_lu_proposals SET net_flag = false WHERE id IN (
    SELECT o.id FROM dao_lu_proposals o WHERE o.net_flag AND o.created_at < @cutoff LIMIT @lim);

-- name: ClearOldDaoLuBondFlags :execrows
UPDATE dao_lu_bonds SET net_flag = false WHERE id IN (
    SELECT o.id FROM dao_lu_bonds o WHERE o.net_flag AND o.formed_at < @cutoff LIMIT @lim);
