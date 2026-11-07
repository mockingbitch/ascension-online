-- Removes every load-test fixture created by seed.sql (and everything the runs wrote for those characters, via
-- ON DELETE CASCADE). World per-day aggregates (world_daily_totals) keep the test traffic: dev data only.
-- Then restart the worker so its startup reconcile drops the test characters from the Redis rankings.
\set ON_ERROR_STOP on
BEGIN;
DELETE FROM outbox_events o USING characters c JOIN users u ON u.id = c.user_id
WHERE u.email LIKE 'lt%@loadtest.invalid' AND o.aggregate_id = c.id::text AND o.published_at IS NULL;
-- Payment records are RESTRICT on users (financial data); test users' orders are dev fixtures, removed explicitly.
DO $$
BEGIN
  IF to_regclass('public.payment_orders') IS NOT NULL THEN
    DELETE FROM payment_events e USING payment_orders o, users u
      WHERE e.order_id = o.id AND o.user_id = u.id AND u.email LIKE 'lt%@loadtest.invalid';
    DELETE FROM payment_orders o USING users u WHERE o.user_id = u.id AND u.email LIKE 'lt%@loadtest.invalid';
  END IF;
END $$;
DELETE FROM users WHERE email LIKE 'lt%@loadtest.invalid';
COMMIT;
SELECT count(*) AS loadtest_users_left FROM users WHERE email LIKE 'lt%@loadtest.invalid';
