---
name: content-catalog
description: Add or edit data-driven game content in content/catalog/*.yaml (realms, stages, VIP tiers, items, spirit roots, techniques, cave facilities) including the Go loader/validator and TS mirror. Use when adding game content or a new catalogue type.
---

# Content catalogue

Rules: `.claude/rules/data-structures.md`, `.claude/rules/economy-balance.md`.

## Editing existing catalogue
1. Edit the YAML. Keep `snake_case`, unit suffixes (`_bp`, `_milli`, `_seconds`), stable IDs.
2. `make content-check` (loader tests on shipped files). Fix every validation error — never relax the validator
   to make content pass without a design reason.
3. Numbers changed? Run `balance-sim` (before/after) and include it in the summary.
4. If a field is visible in UI, check `apps/web` renders it (badge/aura tokens exist for new `visual.*` keys —
   add `--color-aura-<key>` in `app.css` for a new aura).

## Adding a new catalogue type (e.g. `techniques.yaml`)
1. Write the YAML with `version: 1` and a short header comment: purpose, units, ID rules.
2. Create `services/game/internal/<domain>/load.go`:
   - private `file*` structs with yaml tags; `dec.KnownFields(true)`
   - `build()` collects ALL errors via `errors.Join` (ID regex, duplicates, ranges, monotonicity, cross-refs)
   - expose an immutable public type with lookup methods; json tags camelCase
3. `load_test.go`: `TestShipped<Type>IsValid` (reads real file) + rejection tests for bad input.
4. Wire into `cmd/api/main.go` (load at startup, fail fast) and add to `make content-check`.
5. If players can own it → DB table referencing the slug (`new-migration`).
6. Mirror in `apps/web/src/lib/game/types.ts` and the OpenAPI contract if served.

## Template for an entry
```yaml
  - id: thanh_mong_quyet          # permanent slug
    name: Thanh Mộng Quyết        # display (diacritics)
    rarity: huyen                 # pham|linh|huyen|dia|thien|tien
    rate_bonus_bp: 300            # +3% cultivation rate, capped by category rule
    unlock_stage: truc_co.so_ky   # cross-ref validated
    description: Công pháp thủy hệ, tâm như mặt hồ, tu vi tích lũy đều đặn.
```
Story/description text follows `.claude/rules/story-writing.md` (the `story-creator` agent can write it).
