---
name: new-migration
description: Add or change PostgreSQL schema for Tiên Đạo (new table, column, index, constraint) with golang-migrate + sqlc. Use whenever a task needs persistent data or a new query.
---

# New migration

Follow `.claude/rules/database.md`. Steps:

1. **Design first.** Decide what is durable state vs catalogue (YAML) vs derived (Redis/compute). If unsure,
   use the `data-structure-design` skill or the `db-architect` agent.
2. **Create files:** `make migrate-new name=<verb_noun>` (e.g. `add_sects`). Never edit an applied migration.
3. **Write the up migration** using the conventions:
   ```sql
   CREATE TABLE sect_members (
       sect_id      UUID        NOT NULL REFERENCES sects (id) ON DELETE CASCADE,
       character_id UUID        NOT NULL UNIQUE REFERENCES characters (id) ON DELETE CASCADE, -- 1 sect / char
       role         TEXT        NOT NULL CHECK (role IN ('leader', 'officer', 'member')),
       joined_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
       PRIMARY KEY (sect_id, character_id)
   );
   CREATE INDEX sect_members_sect_role_idx ON sect_members (sect_id, role);
   ```
   Checklist: amounts BIGINT + CHECK, `*_bp` CHECK 0..10000, TIMESTAMPTZ, TEXT+CHECK enums, catalogue refs as
   TEXT (no FK), idempotency UNIQUE on mutation logs, ledger table if it holds value, index for each query pattern.
4. **Write the down migration** reversing everything (reverse order).
5. **Queries:** add to `services/game/db/queries/<domain>.sql` with `-- name: VerbNoun :one|:many|:exec`.
   Locking reads → `FOR UPDATE`; queues → `FOR UPDATE SKIP LOCKED`.
6. **Generate:** `make sqlc`. Never hand-edit `internal/db/`.
7. **Verify:**
   ```bash
   make db-up && make migrate-up && make migrate-down && make migrate-up   # up/down/up round-trip
   cd services/game && go build ./... && go test ./...
   ```
8. Report: tables/columns added, invariants enforced by constraints, indexes and the query each serves.
