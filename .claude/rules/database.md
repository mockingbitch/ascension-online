---
paths:
  - "services/game/migrations/**"
  - "services/game/db/**"
  - "services/game/sqlc.yaml"
  - "services/game/internal/platform/postgres/**"
---

# Database rules (PostgreSQL + Redis)

## Migrations (golang-migrate)
- Create with `make migrate-new name=<verb_noun>` → `NNNNNN_name.up.sql` + `.down.sql`. Both required.
- **Never edit a migration that has been merged/applied.** Add a new one.
- Down migration must fully revert the up (drop in reverse dependency order).
- Zero-downtime mindset: add nullable column → backfill → add constraint in a later migration.
  `CREATE INDEX CONCURRENTLY` for big tables (in its own migration, no transaction).
- After any schema/query change: `make sqlc` and commit generated code.

## Naming & types
- Tables: plural `snake_case` (`breakthrough_attempts`). PK `id`. FK `<entity>_id`.
- Entity PKs: `UUID DEFAULT gen_random_uuid()`. Append-only logs: `BIGSERIAL`.
- Amounts: `BIGINT` with `CHECK (x >= 0)` where applicable. Rates `*_milli BIGINT`, chances `*_bp INTEGER CHECK 0..10000`.
- Time: `TIMESTAMPTZ NOT NULL DEFAULT now()`; app passes server time explicitly when it matters for game math.
- Catalogue references (`realm_id`, `stage_id`, `item_id`) are `TEXT` slugs — no FK (catalogue lives in YAML).
- Enums: `TEXT` + `CHECK (x IN (...))`, not PG enum types (easier to evolve).
- JSONB only for audit payloads / snapshots — never for data you filter or aggregate on.

## Invariants enforced in the DB
- Idempotency: `UNIQUE (character_id, idempotency_key)` on every mutation log table.
- Balances never negative: `CHECK (balance >= 0)`.
- Every currency/item change = 1 row in `currency_ledger` / `inventory_transactions` in the same tx,
  with `balance_after`/`quantity_after`.
- Hot rows (`character_progressions`, `currency_accounts`) carry `version BIGINT` bumped on update.

## Queries (sqlc)
- One file per domain in `db/queries/`. Name: `-- name: VerbNoun :one|:many|:exec`.
- Locking reads end with `FOR UPDATE`; queue claims use `FOR UPDATE SKIP LOCKED`.
- Every new WHERE/ORDER BY pattern on a large table needs an index; partial indexes for queues
  (`WHERE published_at IS NULL`).
- No `SELECT *` in hot paths returning wide rows to clients; fine for internal row loads.

## Redis
- Keys: `<domain>:<scope>:<name>` e.g. `leaderboard:global:realm`, `presence:char:<id>`. Rate limits: `rate:ratelimit:<name>:<ip|user>` (redis_rate GCRA). Pub/sub channel `realtime:broadcast` (worker → API instances).
- Every key must be rebuildable from Postgres or safe to lose. Set TTLs on ephemeral keys.
- Leaderboard writes happen in the worker from committed outbox events; periodic reconcile from Postgres.
- No Redis distributed locks for gameplay correctness — use Postgres row locks.
