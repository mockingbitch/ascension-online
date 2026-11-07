-- Load-test fixtures (DEV ONLY): N users + characters the k6 scripts address through the dev-only X-Dev-User-Id header.
-- Run with: make loadtest-seed [LT_USERS=4000]   (psql -v n=4000 -f seed.sql). Idempotent; remove with cleanup.sql.
--
-- User i (1-based) has id 00000000-0000-4000-8000-<i as 12 hex digits> (loadtest/lib.js userId), e-mail
-- lt<i>@loadtest.invalid, a character "LT<i>" at luyen_khi.tang_3 (world chat allowed), last cultivated 2 hours ago
-- (settle has something to credit), in faction huyen_khung_mon.
\set ON_ERROR_STOP on
\if :{?n}
\else
  \set n 4000
\endif

BEGIN;
INSERT INTO users (id, email)
SELECT ('00000000-0000-4000-8000-' || lpad(to_hex(i), 12, '0'))::uuid, 'lt' || i || '@loadtest.invalid'
FROM generate_series(1, :n) AS i
ON CONFLICT (id) DO NOTHING;

INSERT INTO characters (user_id, name)
SELECT u.id, 'LT' || substr(u.email, 3, position('@' IN u.email) - 3)
FROM users u
WHERE u.email LIKE 'lt%@loadtest.invalid'
  AND NOT EXISTS (SELECT 1 FROM characters c WHERE c.user_id = u.id);

INSERT INTO character_progressions (character_id, realm_id, stage_id, last_cultivated_at, stage_reached_at)
SELECT c.id, 'luyen_khi', 'luyen_khi.tang_3', now() - interval '2 hours', now() - interval '2 hours'
FROM characters c JOIN users u ON u.id = c.user_id
WHERE u.email LIKE 'lt%@loadtest.invalid'
ON CONFLICT (character_id) DO NOTHING;

INSERT INTO character_faction (character_id, faction_id, joined_at)
SELECT c.id, 'huyen_khung_mon', now() - interval '2 hours'
FROM characters c JOIN users u ON u.id = c.user_id
WHERE u.email LIKE 'lt%@loadtest.invalid'
ON CONFLICT (character_id) DO NOTHING;

INSERT INTO currency_accounts (character_id, currency)
SELECT c.id, 'spirit_stone' FROM characters c JOIN users u ON u.id = c.user_id
WHERE u.email LIKE 'lt%@loadtest.invalid'
ON CONFLICT DO NOTHING;
COMMIT;

SELECT count(*) AS loadtest_users FROM users WHERE email LIKE 'lt%@loadtest.invalid';
