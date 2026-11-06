---
paths:
  - "content/catalog/**"
  - "services/game/internal/realm/**"
  - "services/game/internal/vip/**"
  - "services/game/internal/fixedpoint/**"
  - "apps/web/src/lib/game/**"
  - "packages/contracts/**"
---

# Data structure rules

## Three representations, one meaning
Each game concept has up to three shapes; keep them aligned in the same change:
1. **Catalogue YAML** (`content/catalog/*.yaml`) — `snake_case`, designer-facing, units in field names.
2. **Go types** (`internal/<domain>`) — yaml tags for loading, json tags (camelCase) for API.
3. **TS types** (`apps/web/src/lib/game/types.ts`) + `packages/contracts/openapi.yaml` — camelCase mirror of JSON.

## Units are part of the name
| Suffix | Meaning | Example |
|---|---|---|
| `_bp` | basis points, 10000 = 100% | `success_bp: 7000` |
| `_milli` | value × 1000 (per second for rates) | `base_rate_milli: 3000` = 3 tu vi/s |
| `_seconds`, `_hours` | durations | `cooldown_seconds: 1800` |
| `_at` | timestamp (TIMESTAMPTZ / RFC3339) | `last_cultivated_at` |
| (none, integer) | count / amount in base units | `required_cultivation: 1500` |

No floats in catalogue or state. Display formatting (vạn/ức, %) only in `apps/web/src/lib/game/format.ts`.

## IDs
- Stable ASCII slugs `^[a-z][a-z0-9_]*$`, Vietnamese without diacritics (`truc_co_dan`). Display names carry diacritics.
- Composite IDs use `.`: stage `"<realm_id>.<stage_id>"`.
- **Shipped IDs are permanent.** Renaming = new ID + data migration of every DB row referencing it.

## Catalogue files
- Top-level `version: N`. Loader uses `KnownFields(true)` → typos fail loudly.
- Every file has a Go loader that validates: ID format/uniqueness, ranges (`0..10000` bp), monotonic curves,
  hard caps (VIP `hard_max_offline_cap_hours`, perk whitelist). Cross-file references (e.g. realm requirement
  `item_id` exists in `items.yaml`) are checked in `internal/content.LoadAll` — the ONLY way processes load content.
  New catalogue file → add its loader there and extend `crossCheck`.
- Each loader has a `TestShipped…` test reading the real file. Content changes must pass `make content-check`.
- Catalogue is loaded once at startup and is immutable; share pointers, never mutate.
- Prefer compact generators (curve: `first_required` + `growth_bp`) with optional per-entry overrides over
  long hand-written tables.

## Source of truth vs derived
- Store the minimum: e.g. progression stores `stage_id`, `cultivation`, `last_cultivated_at`.
  Derive `power_score`, leaderboard score, rate-with-modifiers on read.
- If a derived value must be stored (for sorting/perf), document how it is recomputed and reconciled.

## Modifiers (spirit root, technique, cave, sect…)
- Expressed as `*_bonus_bp`, combined additively within a category then multiplied across categories,
  each category with a documented cap. Implement in one place (`cultivation.EffectiveRateMilli`).
