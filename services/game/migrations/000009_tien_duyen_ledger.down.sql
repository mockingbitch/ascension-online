-- Data is kept: refund rows stay as reason 'refund' (allowed by the 000008 CHECK); the link column is dropped but
-- ref_type 'vip_ledger' + ref_id keep the purchase id, so 000009 up restores it (and the activity attribution, from
-- ref_type).
DROP INDEX IF EXISTS vip_point_ledger_purchase_order_uq;
DROP INDEX IF EXISTS vip_point_ledger_user_day_idx;
ALTER TABLE vip_point_ledger
    DROP CONSTRAINT IF EXISTS vip_point_ledger_refund_chk,
    DROP CONSTRAINT IF EXISTS vip_point_ledger_activity_chk,
    DROP CONSTRAINT IF EXISTS vip_point_ledger_source_check,
    DROP COLUMN IF EXISTS reverses_ledger_id,
    DROP COLUMN IF EXISTS game_day,
    DROP COLUMN IF EXISTS activity_id,
    DROP COLUMN IF EXISTS source;
