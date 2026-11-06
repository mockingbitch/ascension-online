---
name: data-structure-design
description: Design the data model for a new game system (sect, technique, cave abode, alchemy, achievements, market, events) across catalogue YAML, Go types, PostgreSQL tables, Redis keys and TS types before coding. Use at the start of any new feature.
---

# Data structure design

Produce a short design (in chat, or `docs/systems/<system>.md` if the user wants it kept) with these sections,
then get confirmation before implementing.

## 1. Concept inventory
List nouns and verbs from the GDD section (`docs/design/tien-dao-online-gdd-architecture.md`).
For each noun classify:

| Noun | Kind | Lives in |
|---|---|---|
| definition tuned by designers (e.g. facility levels) | catalogue | `content/catalog/<x>.yaml` |
| per-player durable state (e.g. facility level owned) | state | Postgres table |
| history / audit / idempotency (e.g. upgrade log) | log | append-only table, BIGSERIAL, UNIQUE idem key |
| value that can be recomputed (e.g. total rate) | derived | compute on read / Redis cache |
| live fan-out (e.g. sect chat) | ephemeral | WebSocket + Redis, TTL |

## 2. Shapes
For each catalogue entity: YAML snippet with units. For each table: columns, types, constraints, indexes.
For each Go type: struct with yaml/json tags. For each API payload: camelCase JSON example. For Redis: key pattern,
type (ZSET/HASH/STRING), TTL, rebuild source.

## 3. Invariants
Write them as testable statements, e.g. "a character is in at most one sect", "contribution never negative",
"bonus from facilities ≤ 30%". Mark where each is enforced: DB constraint, row lock + check, validator, test.

## 4. Time & settlement
Anything that "grows over time" (garden, furnace, contribution decay) → store `*_started_at`/`last_*_at` and
settle on access. No timers per player. State the offline cap / overflow policy.

## 5. Flows
For each mutation: intent payload → validations → writes (rows) → outbox events. Concurrency: what row is locked.

## 6. Evolution
Which IDs are permanent, which fields are likely to change, how to migrate. Prefer additive changes.

Delegate deep DB review to the `db-architect` agent and number tuning to `game-balancer`.
