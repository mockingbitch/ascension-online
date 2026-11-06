ALTER TABLE vip_entitlements DROP CONSTRAINT vip_entitlements_tier_check;
ALTER TABLE vip_entitlements ADD CONSTRAINT vip_entitlements_tier_check CHECK (tier BETWEEN 1 AND 10);
DROP TABLE IF EXISTS vip_daily_gift_claims;
DROP TABLE IF EXISTS vip_point_ledger;
DROP TABLE IF EXISTS vip_accounts;
