-- Rolling back removes in-game điểm duyên: subtract it from balances first so ledger sums still match vip_accounts.
UPDATE vip_accounts a SET points = GREATEST(0, a.points - x.total)
FROM (SELECT user_id, SUM(delta) AS total FROM vip_point_ledger WHERE reason = 'activity' GROUP BY user_id) x
WHERE a.user_id = x.user_id;
DELETE FROM vip_point_ledger WHERE reason = 'activity';
ALTER TABLE vip_point_ledger DROP CONSTRAINT vip_point_ledger_reason_check;
ALTER TABLE vip_point_ledger ADD CONSTRAINT vip_point_ledger_reason_check
    CHECK (reason IN ('purchase', 'admin_grant', 'admin_revoke', 'refund', 'promo'));
DROP TABLE IF EXISTS lecture_insights;
