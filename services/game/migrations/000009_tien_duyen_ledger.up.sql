-- Tiên Duyên (VIP v3, data-model §7.2): every điểm duyên row records its source, the earning activity, the game day
-- (for the per-day caps) and, for a refund, the exact ledger row it reverses (UNIQUE → refunded at most once).
-- Reasons keep 'activity' (000008) for in-game earning instead of the doc's 'earn' (same meaning, no data rewrite).
ALTER TABLE vip_point_ledger
    ADD COLUMN source             TEXT,
    ADD COLUMN activity_id        TEXT,
    ADD COLUMN game_day           DATE,
    ADD COLUMN reverses_ledger_id BIGINT UNIQUE REFERENCES vip_point_ledger (id);

-- Backfill, derived only from columns that survive a down → up cycle (reason, ref_type, ref_id), so re-running this
-- after 000009 down restores the same attribution and refund links:
--   * a refund written by vip.RefundLocked has ref_type 'vip_ledger', ref_id = the purchase id → keep it linked;
--     any other refund row (none existed before 000009) cannot point at a purchase and becomes an admin revoke;
--   * in-game rows ('activity') map by the ref_type their writer uses: settlement → online cultivation,
--     daily_task → daily-task claim, lecture_insight → lecture; anything unknown keeps its ref_type as activity id
--     under source 'event' so it stays an (uncapped-history) in-game row rather than being relabelled.
UPDATE vip_point_ledger l SET reason = 'admin_revoke'
WHERE l.reason = 'refund'
  AND NOT (l.ref_type = 'vip_ledger' AND l.ref_id ~ '^[0-9]{1,18}$'
           AND EXISTS (SELECT 1 FROM vip_point_ledger p
                       WHERE p.id = (CASE WHEN l.ref_id ~ '^[0-9]{1,18}$' THEN l.ref_id::bigint END)
                         AND p.reason = 'purchase' AND p.user_id = l.user_id));
UPDATE vip_point_ledger SET
    game_day           = ((created_at AT TIME ZONE 'UTC') + INTERVAL '7 hours')::date,   -- internal/gameday: UTC+7
    source             = CASE WHEN reason IN ('purchase', 'refund') THEN 'topup'
                              WHEN reason = 'activity' THEN CASE ref_type WHEN 'settlement' THEN 'cultivation'
                                                                          WHEN 'daily_task' THEN 'task'
                                                                          WHEN 'lecture_insight' THEN 'lecture'
                                                                          ELSE 'event' END
                              ELSE 'admin' END,
    activity_id        = CASE WHEN reason = 'activity' THEN CASE ref_type WHEN 'settlement' THEN 'online_cultivation'
                                                                          WHEN 'daily_task' THEN 'daily_task_claim'
                                                                          WHEN 'lecture_insight' THEN 'lecture_attendance'
                                                                          ELSE COALESCE(ref_type, 'unknown') END END,
    reverses_ledger_id = CASE WHEN reason = 'refund' THEN ref_id::bigint END;

ALTER TABLE vip_point_ledger
    ALTER COLUMN source SET NOT NULL,
    ALTER COLUMN game_day SET NOT NULL,
    ADD CONSTRAINT vip_point_ledger_source_check
        CHECK (source IN ('topup', 'cultivation', 'lecture', 'interaction', 'task', 'trial', 'event', 'admin')),
    ADD CONSTRAINT vip_point_ledger_activity_chk
        CHECK ((source IN ('topup', 'admin')) = (activity_id IS NULL)),
    ADD CONSTRAINT vip_point_ledger_refund_chk
        CHECK ((reason = 'refund') = (reverses_ledger_id IS NOT NULL) AND (reason <> 'refund' OR delta < 0));

CREATE INDEX vip_point_ledger_user_day_idx ON vip_point_ledger (user_id, game_day);
-- A payment order credits điểm duyên at most once, whatever idempotency key the caller used (vip.TopupLocked).
CREATE UNIQUE INDEX vip_point_ledger_purchase_order_uq ON vip_point_ledger (ref_type, ref_id) WHERE reason = 'purchase';
