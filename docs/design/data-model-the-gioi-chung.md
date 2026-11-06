# Data model: thế giới chung (Phase 1–2)

Status: **design, not implemented** (Phase 0 deliverable of [lo-trinh-checklist.md](lo-trinh-checklist.md)).
Revision 3 (2026-11-06) — **final Phase 1–2 implementation reference.** Aligned with the binding decisions
"Phase 0: số liệu & mô hình dữ liệu thế giới chung" and "Phase 0 (bổ sung): số liệu còn lại" in
[decisions.md](../decisions.md) and with [balance-the-gioi-chung.md](balance-the-gioi-chung.md) §11.

Binding inputs: [decisions.md](../decisions.md) (the four 2026-11-06 world-story entries);
[balance-the-gioi-chung.md](balance-the-gioi-chung.md); [ke-hoach-trien-khai-hong-hoang.md](ke-hoach-trien-khai-hong-hoang.md)
§2–3; [cot-truyen.md](cot-truyen.md) §5 (§6.1 and §6.4 are **superseded** by the decision entry).
Rules: `.claude/rules/{data-structures,database,architecture,economy-balance}.md`.

All numbers are **design defaults** from the balance doc; `game-balancer` keeps tuning them with `make balance`.
Every number lives in YAML. Go/TS only hold structural limits (`bp ≤ 10000`, `MaxTier`, …).

Contents

1. Concept inventory
2. Cross-cutting choices (point scale, game day, lock order, credit pipeline)
3. `realms.yaml` v2 (paths, tiers, stage generation, bottleneck, tribulation)
4. `contribution.yaml` + `daily_contribution` + `cultivation_credits` (+ speed items, catch-up, late-joiner grant)
5. `world_milestones.yaml` + world tables (per-active-player progress)
6. `factions.yaml` + `character_faction` / `faction_history`
7. Tiên Duyên (VIP v3) + `vip_point_ledger` sources, refunds
8. `shop.yaml` + purchase ledger (+ item effect fields)
9. Other Phase 1–2 data items (lectures, presence, rituals, fortunes, moderation, onboarding)
10. Redis keys
11. API shapes (camelCase JSON)
12. Mutation flows
13. Migration plan, sqlc queries
14. Evolution / permanence
15. File-by-file implementation plan
16. Resolved questions (decision log)
17. Open questions

---

## 1. Concept inventory

| Concept | Kind | Lives in |
|---|---|---|
| Path (tiên / Vu), tier, realm, stage generation, bottleneck, tribulation rule | catalogue | `content/catalog/realms.yaml` v2 |
| Daily ceiling, bucket caps 75/45/30, idle rate, catch-up ×2, late-joiner grant, speed-item cap | catalogue | `content/catalog/contribution.yaml` |
| Era, story milestone (target day, per-active-player threshold), effect | catalogue | `content/catalog/world_milestones.yaml` |
| Faction, allowed paths, era window, switch cooldown | catalogue | `content/catalog/factions.yaml` |
| Tiên Duyên ladder, top-up rate, per-activity điểm duyên | catalogue | `content/catalog/vip.yaml` v3 |
| Shop listing, price, per-day purchase limit | catalogue | `content/catalog/shop.yaml` |
| Item effects incl. per-day use limit | catalogue | `content/catalog/items.yaml` |
| Character progression (stage, stored cultivation, carry, streak, cooldown, grant reserve) | state | `character_progressions` (existing, changed) |
| Points credited per character per game day, per bucket | state (aggregate, hot) | `daily_contribution` |
| Every points credit (source, raw, multiplier, clipped) | log | `cultivation_credits` |
| Late-joiner one-time grant | log + state | `late_join_grants` + `character_progressions.grant_reserve_milli` |
| Item uses per day (speed-item cap) | state (aggregate) | `item_daily_uses` |
| Settlement time accounting | log | `cultivation_settlements` (existing, changed) |
| Tribulation attempt + loss | log | `breakthrough_attempts` (+ columns) |
| World clock (`started_at`), era, world progress, server median | state (singleton) | `world_state` |
| Per game day: credited total, DAU, per-capita, median position | derived, stored | `world_daily_totals` (worker recompute, then closed) |
| Milestone reached | log (idempotent by PK) | `world_milestones_reached` |
| Scheduled milestone effect | state/queue | `world_effects` |
| Admin milestone shift | state + audit | `world_milestone_overrides` + `admin_audit_logs` |
| Character's faction, lock | state | `character_faction` |
| Faction changes | log | `faction_history` |
| Tiên Duyên points / changes | state / log | `vip_accounts` / `vip_point_ledger` (+ `source`, `activity_id`, `game_day`, `reverses_ledger_id`) |
| Shop purchase / per-day count | log / state | `shop_purchases` / `shop_daily_counts` |
| Path of a character | **derived** from `stage_id` | no column (§3.7) |
| Ladder position (cumulative points) | derived | `Stage.CumulativeBeforeMilli + cultivation_milli` |
| % to next milestone, ETA | derived | computed on read in `GET /world` |
| Faction ranking, faction member ranking | derived | Redis ZSET, rebuilt from Postgres |
| Presence (lecture room, hộ pháp) | ephemeral | Redis with TTL |

---

## 2. Cross-cutting choices

### 2.1 Point scale

- **1 point (điểm tu vi) = 1000 milli.** All stored cultivation is `*_milli BIGINT`. Maximum per character ≈ 3.3e7
  milli; server sums stay far below int64.
- YAML authors write **whole points** where designers think in points (`entry_points`, `daily_cap_points`,
  `reference_points_per_active_day`) and `*_milli` where sub-point precision matters (idle rate, item instant points).
  Loaders convert once (`×1000`); Go types hold `…Milli` fields only; JSON exposes `…Milli`; `format.ts` renders points.
- **Idle rate** = `rate_milli_per_hour: 2083` (50 points per 24 h of eligible time; base window 8 h = Tiên Duyên 0
  offline cap). Sub-milli remainder is carried in `character_progressions.rate_carry INTEGER CHECK (0..3599)`:
  `earned_milli = (rate × eligible_seconds + rate_carry) / 3600`, `rate_carry = … % 3600`.
- **Vu uses the same idle rate, buckets and ladder numbers as tiên** (decision). Nothing in `contribution.yaml` is
  per path.
- Stored cultivation stays **per stage** (carry-over on breakthrough). Minor stages are auto-advanced (§3.4).

### 2.2 Game day and server day

- Game day = `gameday.Of(now)` (UTC+7) for every daily cap (contribution, item uses, shop, Tiên Duyên earn, lecture).
- **A credit counts for the game day on which it happens** (decision), never split across midnight.
- **Server day** = whole game days since `gameday.Of(world_state.started_at)`; day 0 = launch day.
- Milestone safety windows use exact seconds since `started_at` (`target_day × 86400 × pct_bp / 10000`).

### 2.3 Global lock order (deadlock rule)

```
character_progressions (FOR UPDATE)            -- serializes everything that changes cultivation
  → daily_contribution (upsert + FOR UPDATE)
  → item_daily_uses (conditional upsert)
  → character_faction (FOR UPDATE)
  → vip_accounts (FOR UPDATE)
  → currency_accounts (FOR UPDATE, by currency)
  → inventory_items (FOR UPDATE, ORDER BY item_id)
  → shop_daily_counts (conditional upsert)
```

`world_state` is **read without lock** on hot paths (or from an in-process snapshot ≤ 30 s old); only the worker and
admin lock it.

### 2.4 One choke point for points: `cultivation.CreditLocked`

Every daily source of tu vi (idle settle, lecture, daily task, ritual, fortune, trial, speed item, faction, hộ pháp,
song tu, event) calls the same helper **inside its own transaction after locking the progression row**:

```
raw_milli (source-specific)
  → × catch-up multiplier (if eligible and the source's bucket is in applies_to)   = boosted
  → min(boosted, bucket room today, total room today)                              = allowed   (ceiling clip)
  → min(allowed, realm bottleneck room)                                            = credited  (bình cảnh clip, §3.5)
  → cultivation_milli += credited; auto-advance minor stages (§3.4)
  → UPSERT daily_contribution[bucket] += credited
  → INSERT cultivation_credits (raw, multiplier, credited, both clips, idem key)
  → release late-joiner reserve into remaining bottleneck room (§4.6; not counted in daily_contribution)
  → outbox leaderboard.dirty (if anything changed)
```

Consequences: ×2 is applied **before** the 150 ceiling; bottleneck-clipped points consume neither the ceiling nor
world progress; **world progress counts credited points only, excluding the ×2 portion and capped at 100 per character-day** (decision B, §5.1).

---

## 3. `realms.yaml` v2

### 3.1 Structure

Numbers are defined **once per tier**; each path only supplies names, titles and visuals. Equal tiers hold by
construction and path conversion is lossless.

```yaml
# content/catalog/realms.yaml — v2. Loaded by internal/realm.
version: 2

settings:
  overflow_cap_bp: 12000          # bình cảnh — applies ONLY on the last stage of each realm (§3.5)
  pity_max_bonus_bp: 3000         # major breakthroughs before tribulation

paths:
  - { id: tien, name: Tiên Đạo, default: true }
  - { id: vu,   name: Vu Đạo, name_placeholder: true }

# Tribulation (D3). Applies to the MAJOR breakthrough LEAVING `from_tier` and every later major one.
tribulation:
  from_tier: 5                    # leaving Hóa Thần (tier 5) → Luyện Hư
  base_bp: 8000
  step_bp: 300                    # −3% per tier above from_tier
  floor_bp: 5000
  loss_bp: 2500                   # of cultivation_milli stored in the CURRENT stage (decision 2)
  cooldown_seconds: 86400
  guarantee_after: 3              # 3 consecutive fails → next attempt 100%
  surplus_bonus:                  # "dư tu vi so với ngưỡng" (decision F)
    bp_per_surplus_bp: 2500       # each 1% stored above requirement → +0.25%; 20% surplus (bình cảnh) → +5%
    max_bp: 500
  item_bonus_max_bp: 700          # pháp bảo + đan hộ thân (Phase 3; bí cảnh drops only, never sold)
  protector_bonus_per_member_bp: 100   # hộ pháp: +1% per online fellow faction member (Phase 3)
  protector_bonus_max_bp: 300          # max 3 members
  max_total_bonus_bp: 1200        # surplus + items + protector together
  max_success_bp: 9000            # final odds cap; only the guaranteed attempt reaches 10000
  names: { tien: Lôi Kiếp, vu: Huyết Kiếp }   # vu name placeholder (D2)

# Stage generators. A tier picks one: an explicit list (`set`) or `phases × layers`.
stage_sets:
  chin_tang:                      # Luyện Khí: 9 tầng, names prefixed with the realm name
    prefix_realm: true
    stages: [{ id: tang_1, name: Tầng 1 }, …, { id: tang_9, name: Tầng 9 }]
phase_sets:
  bon_ky:                         # 4 kỳ; × layers → "Sơ Kỳ Nhất Trọng" … ; ids "<phase>_<layer>"
    phases:
      - { id: so_ky,    name: Sơ Kỳ }
      - { id: trung_ky, name: Trung Kỳ }
      - { id: hau_ky,   name: Hậu Kỳ }
      - { id: vien_man, name: Viên Mãn }
    layer_names: [Nhất Trọng, Nhị Trọng, Tam Trọng, Tứ Trọng, Ngũ Trọng, Lục Trọng, Thất Trọng, Bát Trọng,
                  Cửu Trọng, Thập Trọng, Thập Nhất Trọng, Thập Nhị Trọng, Thập Tam Trọng, Thập Tứ Trọng,
                  Thập Ngũ Trọng, Thập Lục Trọng]   # layers = 1 → layer name omitted

minor_breakthrough: { auto: true }   # default for all tiers: 100%, applied automatically on credit (§3.4)

tiers:                            # entry_points = cumulative points to ENTER the tier (balance doc §3.2)
  - tier: 1                       # Luyện Khí
    entry_points: 0
    stages: { set: chin_tang }
    major_breakthrough:
      success_bp: 7000
      pity_step_bp: 1000
      cooldown_seconds: 1800
      requirements: [{ item_id: truc_co_dan, quantity: 1 }]
  - tier: 2                       # Trúc Cơ
    entry_points: 300
    stages: { phases: bon_ky, layers: 1 }
    major_breakthrough:
      success_bp: 6000
      pity_step_bp: 800
      cooldown_seconds: 3600
      requirements: [{ item_id: ket_dan_qua, quantity: 1 }]
  - { tier: 3,  entry_points: 950,   stages: { phases: bon_ky, layers: 1 }, major_breakthrough: { … } }  # Kim Đan
  - { tier: 4,  entry_points: 2050,  stages: { phases: bon_ky, layers: 2 }, major_breakthrough: { … } }  # Nguyên Anh
  - { tier: 5,  entry_points: 3500,  stages: { phases: bon_ky, layers: 2 }, major_breakthrough: { requirements: [] } } # Hóa Thần → 1st tribulation
  - { tier: 6,  entry_points: 5400,  stages: { phases: bon_ky, layers: 2 } }   # Luyện Hư
  - { tier: 7,  entry_points: 7500,  stages: { phases: bon_ky, layers: 2 } }   # Hợp Thể
  - { tier: 8,  entry_points: 9750,  stages: { phases: bon_ky, layers: 3 } }   # Đại Thừa
  - { tier: 9,  entry_points: 12250, stages: { phases: bon_ky, layers: 3 } }   # Độ Kiếp
  - { tier: 10, entry_points: 14900, stages: { phases: bon_ky, layers: 3 } }   # Nhân Tiên
  - { tier: 11, entry_points: 17750, stages: { phases: bon_ky, layers: 3 } }   # Địa Tiên
  - { tier: 12, entry_points: 20900, stages: { phases: bon_ky, layers: 3 } }   # Kim Tiên
  - { tier: 13, entry_points: 24350, stages: { phases: bon_ky, layers: 4 } }   # Thái Ất Kim Tiên
  - { tier: 14, entry_points: 28200, stages: { phases: bon_ky, layers: 4 } }   # Đại La Kim Tiên
  - tier: 15                      # Chuẩn Thánh — last playable tier
    entry_points: 32450
    span_points: 19200            # decision C: 32,450 → 51,650
    stages: { phases: bon_ky, layers: 16 }  # 64 stages × 300 points
    extra_stages: []              # append-only tail (e.g. vien_man_17…) if Tây Du comes late; each needs required_points
    major_breakthrough: { success_bp: 0 }   # Thánh Nhân reserved: no player breakthrough
  - tier: 16
    reserved: true                # Thánh Nhân: events/NPCs only; no player may hold a stage here
    stages: { set: dac_dao }      # single stage "Đắc Đạo"

realms:
  # ── tiên path (IDs reuse shipped slugs) ──
  - { id: luyen_khi,        path: tien, tier: 1,  name: Luyện Khí,        title: Luyện Khí Sĩ,      visual: { badge: realm-luyen-khi, aura: mist } }
  - { id: truc_co,          path: tien, tier: 2,  name: Trúc Cơ,          title: Trúc Cơ Tu Sĩ,     visual: { badge: realm-truc-co, aura: jade } }
  - { id: kim_dan,          path: tien, tier: 3,  name: Kim Đan, … }
  - { id: nguyen_anh,       path: tien, tier: 4,  name: Nguyên Anh, … }
  - { id: hoa_than,         path: tien, tier: 5,  name: Hóa Thần, … }
  - { id: luyen_hu,         path: tien, tier: 6,  name: Luyện Hư, … }
  - { id: hop_the,          path: tien, tier: 7,  name: Hợp Thể, … }
  - { id: dai_thua,         path: tien, tier: 8,  name: Đại Thừa, … }
  - { id: do_kiep,          path: tien, tier: 9,  name: Độ Kiếp, … }
  - { id: nhan_tien,        path: tien, tier: 10, name: Nhân Tiên, … }
  - { id: dia_tien,         path: tien, tier: 11, name: Địa Tiên, … }
  - { id: kim_tien,         path: tien, tier: 12, name: Kim Tiên, … }
  - { id: thai_at_kim_tien, path: tien, tier: 13, name: Thái Ất Kim Tiên, … }
  - { id: dai_la_kim_tien,  path: tien, tier: 14, name: Đại La Kim Tiên, … }
  - { id: chuan_thanh,      path: tien, tier: 15, name: Chuẩn Thánh, … }
  - { id: thanh_nhan,       path: tien, tier: 16, name: Thánh Nhân, … }
  # ── Vu path: IDs permanent and neutral; names placeholder until the D2 lore table ──
  - { id: vu_1,  path: vu, tier: 1, name: Khai Huyết, name_placeholder: true, title: …, visual: { badge: realm-vu-1, aura: blood } }
  - { id: vu_2,  path: vu, tier: 2, name: Luyện Cốt,  name_placeholder: true, … }
  # … vu_3 Tẩy Tủy · vu_4 Huyết Hải · vu_5 Vu Hồn · vu_6 Tiểu Vu · vu_7 Trung Vu · vu_8 Thượng Vu · vu_9 Chân Vu ·
  #   vu_10 Vu Tướng · vu_11 Vu Vương · vu_12 Vu Tôn · vu_13 Vu Hoàng · vu_14 Đại Vu ·
  #   vu_15 (placeholder for "Tổ Vu" — must get an original name, D2)
```

Notes:
- Stage counts (decision C): 9 / 4 / 4 / 8 / 8 / 8 / 8 / 12 / 12 / 12 / 12 / 12 / 16 / 16 / 64 = **205 stages**; top of
  the ladder 51,650 points (51,650,000 milli). Order inside a realm is phase-major: Sơ Kỳ 1…k → Trung Kỳ 1…k → Hậu Kỳ
  1…k → Viên Mãn 1…k; the tribulation/major breakthrough sits on **Viên Mãn k**. Độ Kiếp may use display names
  "Đệ N Trọng Thiên Kiếp" via a per-realm `stage_name_override` (display only).
- **Stage IDs are append-only** (decision C). Generated IDs (`so_ky_1` … `vien_man_16`) are fixed at launch; `layers`
  must not change after launch for any realm. Lengthening Chuẩn Thánh later = add entries to `extra_stages`
  (`{ id: vien_man_17, name: …, required_points: 300 }`) after the generated stages, which never renumbers an existing
  stage, so no data migration is needed (planned together with G7/Tây Du).
- Chuẩn Thánh has no further breakthrough: at its last stage the 120% bottleneck holds points, and Thiên Bảng orders
  ties by `stage_reached_at`.
- `pham_nhan` and `phi_thang` are **removed** (pre-launch reset, §13). New characters start at `<path>.tier1.stage1`.
- `base_rate_milli` and `curve` are removed from realms; pace lives in `contribution.yaml`.
- Phase 3 adds `requires_trial: tam_ma` / `hoa_pham` on the tier 3/4 major rules (additive field).

### 3.2 Stage requirement generation

For a playable tier `t` with `n` stages:

Requirements are **whole points**, split evenly with the remainder spread one point each over the **last** stages
(balance doc §11.1):

```
span(t)        = entry_points[t+1] − entry_points[t]                # last playable tier: span_points
base, r        = span / n, span % n                                  # integer division
required(i)    = base       for i = 1 .. n−r
               = base + 1   for i = n−r+1 .. n                       # Σ == span exactly
required_milli = required × 1000
cap_milli      = required_milli(n) × overflow_cap_bp / 10000 on stage n only (§3.5); 0 elsewhere
cumulative_before(i) = (entry_points[t] + Σ required(1..i−1)) × 1000
extra_stages   : appended after stage n with their own required_points; the realm's last stage (and cap) moves to
                 the last extra stage
```

| Tier | Realm | Entry | Span | Structure | Stages | Requirement per stage (points × count, in order) | Last-stage cap (120%) |
|---|---|---|---|---|---|---|---|
| 1 | Luyện Khí | 0 | 300 | 9 tầng | 9 | 33×6, 34×3 | 40.8 |
| 2 | Trúc Cơ | 300 | 650 | 4 kỳ × 1 | 4 | 162×2, 163×2 | 195.6 |
| 3 | Kim Đan | 950 | 1,100 | 4 kỳ × 1 | 4 | 275×4 | 330 |
| 4 | Nguyên Anh | 2,050 | 1,450 | 4 kỳ × 2 | 8 | 181×6, 182×2 | 218.4 |
| 5 | Hóa Thần | 3,500 | 1,900 | 4 kỳ × 2 | 8 | 237×4, 238×4 | 285.6 |
| 6 | Luyện Hư | 5,400 | 2,100 | 4 kỳ × 2 | 8 | 262×4, 263×4 | 315.6 |
| 7 | Hợp Thể | 7,500 | 2,250 | 4 kỳ × 2 | 8 | 281×6, 282×2 | 338.4 |
| 8 | Đại Thừa | 9,750 | 2,500 | 4 kỳ × 3 | 12 | 208×8, 209×4 | 250.8 |
| 9 | Độ Kiếp | 12,250 | 2,650 | 4 kỳ × 3 | 12 | 220×2, 221×10 | 265.2 |
| 10 | Nhân Tiên | 14,900 | 2,850 | 4 kỳ × 3 | 12 | 237×6, 238×6 | 285.6 |
| 11 | Địa Tiên | 17,750 | 3,150 | 4 kỳ × 3 | 12 | 262×6, 263×6 | 315.6 |
| 12 | Kim Tiên | 20,900 | 3,450 | 4 kỳ × 3 | 12 | 287×6, 288×6 | 345.6 |
| 13 | Thái Ất Kim Tiên | 24,350 | 3,850 | 4 kỳ × 4 | 16 | 240×6, 241×10 | 289.2 |
| 14 | Đại La Kim Tiên | 28,200 | 4,250 | 4 kỳ × 4 | 16 | 265×6, 266×10 | 319.2 |
| 15 | Chuẩn Thánh | 32,450 | 19,200 | 4 kỳ × 16 | 64 | 300×64 | 360 |

`TestShippedCatalogueIsValid` asserts this exact table (counts, per-stage values, 205 total, top = 51,650).

### 3.3 Go types (sketch)

```go
package realm

type PathID string // "tien" | "vu"

type Tribulation struct {
    FromTier        int               `yaml:"from_tier" json:"fromTier"`
    BaseBP          int64             `yaml:"base_bp" json:"baseBp"`
    StepBP          int64             `yaml:"step_bp" json:"stepBp"`
    FloorBP         int64             `yaml:"floor_bp" json:"floorBp"`
    LossBP          int64             `yaml:"loss_bp" json:"lossBp"`
    CooldownSeconds int64             `yaml:"cooldown_seconds" json:"cooldownSeconds"`
    GuaranteeAfter  int               `yaml:"guarantee_after" json:"guaranteeAfter"`
    SurplusBonus    SurplusBonus      `yaml:"surplus_bonus" json:"surplusBonus"`
    ItemBonusMaxBP  int64             `yaml:"item_bonus_max_bp" json:"itemBonusMaxBp"`
    ProtectorPerBP  int64             `yaml:"protector_bonus_per_member_bp" json:"protectorBonusPerMemberBp"`
    ProtectorMaxBP  int64             `yaml:"protector_bonus_max_bp" json:"protectorBonusMaxBp"`
    MaxTotalBonusBP int64             `yaml:"max_total_bonus_bp" json:"maxTotalBonusBp"`
    MaxSuccessBP    int64             `yaml:"max_success_bp" json:"maxSuccessBp"`
    Names           map[PathID]string `yaml:"names" json:"names"`
}

type Stage struct {
    ID                    string           `json:"id"`        // "<realm_id>.<stage_id>"
    RealmID               string           `json:"realmId"`
    Path                  PathID           `json:"path"`
    Tier                  int              `json:"tier"`
    StageOrder            int              `json:"stageOrder"` // 1-based inside realm
    StageCount            int              `json:"stageCount"`
    Index                 int              `json:"index"`      // 0-based ordinal within its path
    Name                  string           `json:"name"`
    RequiredMilli         int64            `json:"requiredMilli"`
    CapMilli              int64            `json:"capMilli"`   // > 0 only on a realm's last stage (§3.5)
    CumulativeBeforeMilli int64            `json:"cumulativeBeforeMilli"`
    NextKind              BreakthroughKind `json:"nextKind,omitempty"` // auto | major | ""
    Tribulation           bool             `json:"tribulation"`
    Breakthrough          BreakthroughRule `json:"breakthrough"`       // major rule (last stage only)
}

func (c *Catalogue) Paths() []Path
func (c *Catalogue) First(path PathID) *Stage
func (c *Catalogue) Next(s *Stage) *Stage                          // within s.Path; nil on last playable stage
func (c *Catalogue) StageAt(path PathID, tier, stageOrder int) (*Stage, bool)
func (c *Catalogue) Convert(s *Stage, to PathID) (*Stage, error)  // same tier + stageOrder
func (c *Catalogue) LastOfRealm(s *Stage) *Stage
func (c *Catalogue) BottleneckRoomMilli(s *Stage, stored int64) int64 // §3.5
func (c *Catalogue) Advance(s *Stage, stored int64) (*Stage, int64, []*Stage) // §3.4: auto steps; returns passed stages
func (c *Catalogue) Position(s *Stage, stored int64) int64        // ladder position (milli)
func (c *Catalogue) Tribulation() Tribulation
func (c *Catalogue) TribulationBaseBP(s *Stage) int64            // max(floor, base − step·(tier−from_tier))
```

### 3.4 Minor stages: automatic

**Confirmed by decision A** ("Phase 0 (bổ sung)"): minor breakthroughs are **automatic**; realm (major) breakthroughs
and tribulations stay player-initiated. Inside `CreditLocked`, after adding credited points, `realm.Advance` loops
`while stage is not the realm's last && stored ≥ required: stored −= required; stage = next`. Each passed stage writes
a `progression_events(kind='stage_reached')` row; `stage_reached_at` is updated. No roll, no cooldown, no item cost,
so nothing is lost relative to the old minor policy ("failure keeps cultivation" stays trivially true). Major
breakthroughs (last stage → next realm) stay player-triggered via `POST /breakthrough/attempt`.

`breakthrough_attempts.kind` keeps `'minor'` in its CHECK for historical rows only; v2 never writes it.

### 3.5 Bottleneck (bình cảnh) only on the last stage

- `CapMilli = RequiredMilli × overflow_cap_bp / 10000` on the **last stage of each realm**; `CapMilli = 0` (no cap)
  on all other stages, because they auto-advance.
- Bottleneck room for a credit, from stage `s` with stored `x`:
  `room = (Σ RequiredMilli of s … last−1) − x + last.CapMilli` (≥ 0). `CreditLocked` clips to it.
- At the last stage, stored can reach 120% of its requirement; points beyond are clipped (`clipped_bottleneck_milli`).
- On a successful major breakthrough the carry-over (`stored − required`, ≤ 20% of the last stage) moves into the next
  realm's first stage and is auto-advanced if large enough.

### 3.6 Tribulation rule (pure, `internal/breakthrough`)

For a last stage with `Tribulation == true` (tier ≥ `from_tier`):

```
if fail_streak ≥ guarantee_after:  success = 10000, guaranteed = true
else:
  base      = max(floor_bp, base_bp − step_bp × (tier − from_tier))          # floor applies to the BASE
  surplus   = min(surplus.max_bp, (stored − required) × 10000 / required × bp_per_surplus_bp / 10000)
  items     = min(item_bonus_max_bp, Σ item tribulation_bonus_bp)            # Phase 3; 0 in Phase 1
  protector = min(protector_bonus_max_bp, online_helpers × protector_bonus_per_member_bp)  # Phase 3; 0 in Phase 1
  bonus     = min(max_total_bonus_bp, surplus + items + protector)
  success   = min(max_success_bp, base + bonus)
loss_milli = stored × loss_bp / 10000         (applied on failure only)
```

| Tribulation (leaving) | Base | + 20% surplus (+5%) | Max with items + hộ pháp (cap 90%) |
|---|---|---|---|
| Hóa Thần | 80% | 85% | 90% |
| Luyện Hư | 77% | 82% | 89% |
| Hợp Thể | 74% | 79% | 86% |
| Đại Thừa | 71% | 76% | 83% |
| Độ Kiếp | 68% | 73% | 80% |
| Nhân Tiên | 65% | 70% | 77% |
| Địa Tiên | 62% | 67% | 74% |
| Kim Tiên | 59% | 64% | 71% |
| Thái Ất Kim Tiên | 56% | 61% | 68% |
| Đại La Kim Tiên | 53% | 58% | 65% |

On failure: `cultivation_milli −= loss_milli` (stage/realm never change), streak +1, cooldown `now + 24h`.
Requirement items are consumed on both outcomes (existing policy). Example: Luyện Hư last stage requires 263
points, stored ≤ 315.6 → loss ≤ 78.9 points.

**Preview = attempt** (decision): the preview evaluates the *same* settlement the attempt will run first
(`cultivation.ProjectLocked`-equivalent pure path including bucket caps, catch-up, bottleneck and today's
`daily_contribution`), so `lossMilli` is computed on post-settlement stored. Because time passes between preview and
attempt (stored can only grow up to the cap), the attempt request carries an optional **guard**
`acceptLossMilli`: the server never uses it as a value, it only refuses with `409 quote_changed` (+ fresh preview) when
the loss it would apply exceeds the guard. Shown loss therefore always ≥ applied loss.

### 3.7 Path is derived, not stored (decision)

`stage_id` identifies the realm, which belongs to one path. No `characters.path`. Conversion rewrites
`realm_id`/`stage_id` atomically under the progression lock.

### 3.8 Path conversion

`to = StageAt(toPath, s.Tier, s.StageOrder)`; requirements are identical, so `cultivation_milli`, `rate_carry`,
`grant_reserve_milli`, fail streak and cooldown are **kept** (no tribulation-cooldown dodging). Logged in
`faction_history (from_path, to_path)` and `progression_events(kind='path_converted')`.

### 3.9 Leaderboard score

`Score = tier × 1e9 + stage_order × 1e6 + progress_ppm` (path-agnostic; max stage_order 64 + extras < 1000 fits).

### 3.10 Loader invariants (`realm.Load`, `errors.Join`, `TestShippedCatalogueIsValid`)

1. `version == 2`; `KnownFields(true)`.
2. IDs match `^[a-z][a-z0-9_]*$`; path, realm, stage-set, phase IDs unique; global stage IDs unique.
3. Exactly one `default: true` path.
4. Tiers `1..N` contiguous; tier 1 `entry_points == 0`; `entry_points` strictly increasing over playable tiers.
5. Exactly one last playable tier, with `span_points > 0`; no other tier sets `span_points`.
6. Reserved tiers follow all playable tiers; no points, no breakthrough rules.
7. **Every path has exactly one realm per playable tier.**
8. `stages` is exactly one of `{set}` or `{phases, layers}`; `1 ≤ layers ≤ len(layer_names)`; `span ≥ n` (every
   stage requires ≥ 1 point); generated requirements sum to the span. `extra_stages` only on the last playable tier,
   IDs unique, `required_points > 0`.
9. Realm durations strictly increase is **not** enforced by the loader (that's a balance property checked in
   balancesim), but spans must be > 0.
10. bp fields `0..10000`; cooldowns ≥ 0; requirement items unique per rule, quantity > 0.
11. `minor_breakthrough.auto == true` (v2 supports only automatic minors).
12. `tribulation`: `1 ≤ from_tier < last playable tier`; `floor_bp ≤ base_bp ≤ 10000`; `0 < loss_bp ≤ 5000`;
    `cooldown_seconds > 0`; `guarantee_after ≥ 1`; `surplus_bonus.max_bp ≤ max_total_bonus_bp`;
    `item_bonus_max_bp ≤ max_total_bonus_bp`; `protector_bonus_per_member_bp ≤ protector_bonus_max_bp ≤
    max_total_bonus_bp`; `max_success_bp < 10000`; `base_bp + max_total_bonus_bp ≥ max_success_bp` (the cap is
    reachable on the first tribulation); `names` covers every path.
13. Major rules of tiers `≥ from_tier` must not set `success_bp`/`pity_step_bp`/`cooldown_seconds`.
14. The last playable tier's major rule has `success_bp: 0`.
15. `overflow_cap_bp ∈ [10000, MaxOverflowCapBP]`.
16. Cross-check (`content.LoadAll`): requirement `item_id`s exist.
17. Release gate (`CONTENT_RELEASE=1` test): no `name_placeholder: true` remains.

---

## 4. Contribution: `contribution.yaml`, tables, catch-up, grant, speed items

### 4.1 YAML

```yaml
# content/catalog/contribution.yaml — daily points pacing (D5 + Phase 0 decision). Loaded by internal/contribution.
version: 1

daily_ceiling_points: 150

buckets:                              # decision 8: caps kept
  idle:      { target_points: 50, daily_cap_points: 75 }
  activity:  { target_points: 30, daily_cap_points: 45 }   # lectures, tasks, ritual, fortune, trial, speed items
  community: { target_points: 20, daily_cap_points: 30 }   # faction merit, hộ pháp, song tu, shared room

idle:
  rate_milli_per_hour: 2083           # 50 points / 24 h eligible; window = Tiên Duyên offline cap (8 h at level 0)

speed_items:                          # decision 5
  daily_cap_points: 10                # Σ instant points from speed items used per character per game day

catch_up:                             # late joiner ×2, applied BEFORE the ceiling
  multiplier_bp: 20000
  until: server_median                # eligible while ladder position < world_state.median_position_milli
  min_server_day: 3
  applies_to: [idle, activity, community]

late_join_grant:                      # decision 6
  after_server_day: 14                # characters created on server day > 14
  median_share_bp: 5000               # one-time grant = 50% of the server median ladder position
  per_account: 1                      # one grant per Google account, ever

sources:                              # bucket mapping (amounts live in the owning catalogue)
  settle:           { bucket: idle }
  lecture:          { bucket: activity }
  daily_task:       { bucket: activity }
  ritual:           { bucket: activity }
  fortune:          { bucket: activity }
  trial:            { bucket: activity }
  item:             { bucket: activity }
  faction:          { bucket: community }
  protector:        { bucket: community }
  dual_cultivation: { bucket: community }
  event:            { bucket: community }
```

### 4.2 Go (sketch)

```go
package contribution

type Bucket string
type Source string

type Config struct {
    DailyCeilingMilli    int64
    Buckets              map[Bucket]BucketCfg // TargetMilli, CapMilli
    IdleRateMilliPerHour int64
    SpeedItemDailyMilli  int64
    CatchUp              CatchUp       // MultiplierBP, MinServerDay, AppliesTo
    Grant                LateJoinGrant // AfterServerDay, MedianShareBP
    Sources              map[Source]Bucket
}

type Day struct{ Idle, Activity, Community, SpeedItem int64 } // today's credited milli

type Applied struct {
    Bucket                                         Bucket
    RawMilli, MultiplierBP, BoostedMilli           int64
    CreditedMilli                                  int64
    CatchUpBonusMilli                              int64 // = CreditedMilli − min(CreditedMilli, RawMilli)
    ClippedCeilingMilli, ClippedBottleneckMilli    int64
}

func (c *Config) Apply(src Source, raw int64, day Day, catchUp bool, bottleneckRoom int64) Applied // pure
func (c *Config) CatchUpEligible(positionMilli, medianMilli int64, serverDay int) bool
func (c *Config) GrantMilli(medianMilli int64, serverDay int) int64 // 0 when serverDay ≤ AfterServerDay
```

`cultivation.CreditLocked(ctx, q, p *db.CharacterProgression, src, rawMilli, ref, now)` owns the writes (§2.4).

### 4.3 Tables

```sql
CREATE TABLE daily_contribution (
    character_id         UUID        NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    game_day             DATE        NOT NULL,
    idle_milli           BIGINT      NOT NULL DEFAULT 0 CHECK (idle_milli >= 0),
    activity_milli       BIGINT      NOT NULL DEFAULT 0 CHECK (activity_milli >= 0),
    community_milli      BIGINT      NOT NULL DEFAULT 0 CHECK (community_milli >= 0),
    points_milli         BIGINT GENERATED ALWAYS AS (idle_milli + activity_milli + community_milli) STORED,
    speed_item_milli     BIGINT      NOT NULL DEFAULT 0 CHECK (speed_item_milli >= 0), -- subset of activity_milli
    catch_up_bonus_milli BIGINT      NOT NULL DEFAULT 0 CHECK (catch_up_bonus_milli >= 0), -- ×2 portion (§4.4)
    faction_id           TEXT,        -- faction at first credit of the day (Phase 2)
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (character_id, game_day),
    CHECK (speed_item_milli <= activity_milli),
    CHECK (catch_up_bonus_milli <= idle_milli + activity_milli + community_milli)
);
CREATE INDEX daily_contribution_day_idx ON daily_contribution (game_day) WHERE points_milli > 0;
CREATE INDEX daily_contribution_faction_day_idx ON daily_contribution (faction_id, game_day) WHERE faction_id IS NOT NULL;

CREATE TABLE cultivation_credits (
    id                       BIGSERIAL PRIMARY KEY,
    character_id             UUID        NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    game_day                 DATE        NOT NULL,
    source                   TEXT        NOT NULL CHECK (source IN ('settle','lecture','daily_task','ritual','fortune',
                                                     'trial','item','faction','protector','dual_cultivation','event',
                                                     'late_join_grant')),
    bucket                   TEXT        CHECK (bucket IN ('idle','activity','community')),  -- NULL only for grant
    stage_id                 TEXT        NOT NULL,
    raw_milli                BIGINT      NOT NULL CHECK (raw_milli >= 0),
    multiplier_bp            INTEGER     NOT NULL CHECK (multiplier_bp BETWEEN 10000 AND 50000),
    credited_milli           BIGINT      NOT NULL CHECK (credited_milli >= 0),
    catch_up_bonus_milli     BIGINT      NOT NULL DEFAULT 0 CHECK (catch_up_bonus_milli >= 0),
    clipped_ceiling_milli    BIGINT      NOT NULL DEFAULT 0 CHECK (clipped_ceiling_milli >= 0),
    clipped_bottleneck_milli BIGINT      NOT NULL DEFAULT 0 CHECK (clipped_bottleneck_milli >= 0),
    ref_type                 TEXT,
    ref_id                   TEXT,
    idempotency_key          TEXT        NOT NULL,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (character_id, idempotency_key),
    CHECK ((source = 'late_join_grant') = (bucket IS NULL)),
    CHECK (catch_up_bonus_milli <= credited_milli)
);
CREATE INDEX cultivation_credits_char_day_idx ON cultivation_credits (character_id, game_day);

-- Per-day item uses (speed items and any item with max_use_per_game_day).
CREATE TABLE item_daily_uses (
    character_id UUID    NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    game_day     DATE    NOT NULL,
    item_id      TEXT    NOT NULL,
    uses         INTEGER NOT NULL CHECK (uses >= 0),
    PRIMARY KEY (character_id, game_day, item_id)
);

-- Late-joiner grant: at most one per Google account (survives character deletion).
CREATE TABLE late_join_grants (
    user_id               UUID        PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    character_id          UUID        REFERENCES characters (id) ON DELETE SET NULL,
    server_day            INTEGER     NOT NULL CHECK (server_day >= 0),
    median_position_milli BIGINT      NOT NULL CHECK (median_position_milli >= 0),
    median_game_day       DATE        NOT NULL,  -- world_daily_totals row the median came from
    grant_milli           BIGINT      NOT NULL CHECK (grant_milli > 0),
    granted_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE character_progressions
    ADD COLUMN grant_reserve_milli BIGINT NOT NULL DEFAULT 0 CHECK (grant_reserve_milli >= 0);
```

### 4.4 Catch-up (×2)

Eligible when `started_at` is set, server day ≥ `min_server_day`, and the character's ladder position
(`realm.Position`) < `world_state.median_position_milli`. The median comes from the worker (§5.6), recomputed at each
day close. ×2 is applied to raw before bucket caps and the 150 ceiling.

**The ×2 portion is stored separately** because world progress must exclude it (decision B). For every credit,
`Apply` attributes the clipped amount to the base first and the bonus last:

```
credited             = min(raw × multiplier, bucket room, total room, bottleneck room)
catch_up_bonus_milli = credited − min(credited, raw)        # 0 when multiplier = 10000 or clipping ate the bonus
```

The value is written on the `cultivation_credits` row and added to `daily_contribution.catch_up_bonus_milli` in the
same statement as the bucket column. The world-countable amount of a character-day is then
`min(world_credit_cap, points_milli − catch_up_bonus_milli)` (§5.1) — computable from `daily_contribution` alone, so
the worker never has to scan `cultivation_credits`.

### 4.5 Speed items (≤ 10 points/day)

- Item effect `instant_points_milli` + `max_use_per_game_day` (e.g. Tụ Linh Đan +5 points, max 2/day; §8.1).
- Use flow: conditional upsert `item_daily_uses` (`uses + 1 ≤ max_use_per_game_day`, else `daily_limit`), then
  `CreditLocked(source=item)`, which additionally clips by `speed_items.daily_cap_points − daily_contribution.speed_item_milli`.
  Uses are counted **per character per game day of use**, so gifted/traded pills can't bypass the cap.
- Points still go to the activity bucket (≤ 45) and the 150 ceiling.

### 4.6 Late-joiner one-time grant

- **When:** in the character-creation transaction, if `started_at` is set and server day > `after_server_day`.
- **Amount:** `grant = median_position_milli × median_share_bp / 10000`, using `world_state.median_position_milli`
  (from the latest **closed** game day; `late_join_grants.median_game_day` records which).
- **Idempotency:** `late_join_grants` PK `user_id` → one grant per Google account, even if the character is deleted
  and recreated. Insert `ON CONFLICT DO NOTHING RETURNING`; no row → no grant.
- **Delivery without skipping gates:** the grant is stored as `character_progressions.grant_reserve_milli`. Every
  `CreditLocked` (and every successful major breakthrough) releases `min(reserve, bottleneck room)` into
  `cultivation_milli`, auto-advancing minor stages. So the grant fills realms instantly but **never** skips a major
  breakthrough, its item requirement, trial (tâm ma, hóa phàm) or tribulation. Each release writes a
  `cultivation_credits` row with `source='late_join_grant'`, `bucket=NULL`, `multiplier_bp=10000`.
- **Excluded from world progress and daily rankings:** releases are **not** added to `daily_contribution` (so not to
  DAU, `world_daily_totals`, faction merit or any per-day ranking) and do not consume the daily ceiling. They do raise
  the ladder position, which is the point. Thiên Bảng (position-based) reflects it.

### 4.7 Invariants

| # | Invariant | Enforced by |
|---|---|---|
| C1 | `daily_contribution.points_milli ≤ daily_ceiling` | `Apply` (table test), worker audit query |
| C2 | each bucket ≤ its daily cap; `speed_item_milli ≤ speed_items.daily_cap` | `Apply` |
| C3 | Σ `cultivation_credits.credited_milli` of (char, day) **excluding `late_join_grant`** == `daily_contribution.points_milli` | same tx; integration test |
| C4 | `cultivation_milli` only changes via `CreditLocked` / grant release / tribulation failure | review + caller test |
| C5 | ×2 applied before the ceiling | `Apply` |
| C6 | Bottleneck-clipped points count neither for the ceiling nor world progress | `Apply` |
| C6b | `catch_up_bonus_milli` = Σ of the credits' bonus portions; ≤ `points_milli`; 0 when no catch-up | `Apply` (table test) + CHECKs |
| C7 | Credits idempotent per `(character_id, idempotency_key)` | UNIQUE + check after lock |
| C8 | No catch-up / no grant before `started_at`; no grant on server day ≤ 14 | `CatchUpEligible`, `GrantMilli` |
| C9 | ≤ 1 late-join grant per user; grant never skips a major breakthrough | PK `user_id`; release clipped to bottleneck room |
| C10 | item uses per (char, day, item) ≤ `max_use_per_game_day` | conditional upsert |

Loader invariants (`contribution.Load`): version 1; ceiling > 0; buckets exactly `{idle, activity, community}`;
`0 < target ≤ daily_cap ≤ ceiling`; Σ targets ≤ ceiling; `rate × 24 ≤ idle.daily_cap × 1000`;
`0 ≤ speed_items.daily_cap_points ≤ activity.daily_cap_points`; `10000 ≤ multiplier_bp ≤ 50000`;
`until == server_median`; `applies_to ⊆ buckets`; `after_server_day ≥ 0`; `0 < median_share_bp ≤ 10000`;
`per_account == 1`; every Go `Source` constant has a mapping, no unknown keys.

---

## 5. World: `world_milestones.yaml` + tables

### 5.1 Progress definition (decision 4 as amended by decision B)

- **DAU(d)** = number of characters with `daily_contribution.points_milli > 0` on game day `d` (≥ 1 credited point
  that day, ×2 included). Late-joiner grant releases don't count (they aren't in `daily_contribution`).
- **World-countable(c, d)** = `min(world_credit_cap_points × 1000, points_milli − catch_up_bonus_milli)`, i.e.
  credited points **excluding the ×2 late-joiner bonus**, capped at **100 points** per character per day (the
  personal ceiling stays 150).
- **Per-capita(d)** = `Σ_c world_countable(c, d) / DAU(d)` (integer division; 0 if DAU = 0).
- **World progress** = Σ per-capita over **closed** game days since launch = the points an average active player
  would have accumulated at standard pace.
- A milestone is reached when `world_progress ≥ threshold_points_per_active` **and** the 70% floor has passed, or when
  the 150% ceiling has passed regardless. Thresholds = `target_day × 67` (a_ref; re-measured in closed beta).

### 5.2 YAML

```yaml
# content/catalog/world_milestones.yaml — server story clock. Loaded by internal/world.
version: 1

settings:
  progress_mode: per_active_capita        # only mode supported
  world_credit_cap_points: 100            # per character per game day, toward world progress only
  exclude_catch_up_bonus: true            # ×2 portion never counts (grant never reaches daily_contribution anyway)
  reference_points_per_active_day: 67     # a_ref; documentation + default threshold = target_day × a_ref
  min_pct_bp: 7000                        # never before 70% of target day
  max_pct_bp: 15000                       # always by 150% of target day
  day_close_grace_seconds: 300            # a game day is closed 5 min after midnight (late commits)

eras:
  - { id: khai_giang, name: Tử Tiêu khai giảng }
  - { id: dai_chien,  name: Đại chiến, name_placeholder: true }   # G2 framing renamed per D2
  - { id: luan_hoi,   name: Luân hồi khai mở, name_placeholder: true }
  - { id: tao_nhan,   name: Nữ Oa tạo người }
  - { id: thanh_nhan, name: Thánh nhân xuất thế, name_placeholder: true }
  - { id: phong_than, name: Phong Thần lượng kiếp }
  # …

milestones:          # strictly ordered, no minimum gap (decision H); thresholds written explicitly (decision B)
  - id: khai_giang
    name: Hồng Quân giảng đạo lần 1
    target_day: 0
    threshold_points_per_active: 0        # reached at launch
    announce: world.khai_giang
    effects: [{ kind: set_era, era_id: khai_giang }]
  - id: giang_dao_2
    name: Hồng Quân giảng đạo lần 2
    target_day: 30
    threshold_points_per_active: 2010
    announce: world.giang_dao_2
    effects: [{ kind: start_event, event_id: giang_dao_2, duration_days: 1 }]
  - { id: giang_dao_3, target_day: 60,  threshold_points_per_active: 4020,
      effects: [{ kind: start_event, event_id: giang_dao_3, duration_days: 1 }] }
  - id: dai_chien                         # story window day 100–115
    target_day: 100
    threshold_points_per_active: 6700
    effects:
      - { kind: set_era, era_id: dai_chien }
      - { kind: start_event, event_id: dai_chien, duration_days: 15 }
  - id: nu_oa
    target_day: 130
    threshold_points_per_active: 8710
    effects: [{ kind: set_era, era_id: tao_nhan }]          # opens nhan_toc
  - id: thanh_nhan                        # story window day 140–170
    target_day: 140
    threshold_points_per_active: 9380
    effects:
      - { kind: set_era, era_id: thanh_nhan }
      - { kind: start_event, event_id: thanh_nhan, duration_days: 30 }
      - kind: split_faction
        from: huyen_mon
        into: [huyen_mon_a, huyen_mon_b, huyen_mon_c]   # names/IDs from story-creator (O5)
        notice_days: 3                    # decision 9
        choose_days: 7                    # decision 9; default = branch with fewest members (no field)
      - { kind: migrate_faction, from: tay_phuong_mon, to: tay_phuong_giao, notice_days: 3 }
  - { id: nhan_toc,   target_day: 150, threshold_points_per_active: 10050,   # window day 150–210
      effects: [{ kind: start_event, event_id: tam_hoang, duration_days: 60 }] }
  - { id: phong_than, target_day: 240, threshold_points_per_active: 16080,   # window day 240–300
      effects: [{ kind: set_era, era_id: phong_than }, { kind: start_event, event_id: phong_than, duration_days: 60 }] }
  - { id: thien_dinh, target_day: 310, threshold_points_per_active: 20770 }
  - { id: tay_du,     target_day: 330, threshold_points_per_active: 22110 }
```

| Milestone | Target day | Threshold / active player | Simulated day (balance §11.2, realistic launch) |
|---|---|---|---|
| Giảng đạo lần 2 | 30 | 2,010 | 31–32 |
| Giảng đạo lần 3 | 60 | 4,020 | 61–63 |
| Đại chiến | 100 | 6,700 | 101–104 |
| Nữ Oa tạo người | 130 | 8,710 | 130–135 |
| Thánh nhân xuất thế | 140 | 9,380 | 140–145 |
| Nhân tộc · Tam Hoàng | 150 | 10,050 | 150–155 |
| Phong Thần | 240 | 16,080 | 238–245 |
| Thiên Đình tái lập | 310 | 20,770 | 305–314 |
| Tây Du | 330 | 22,110 | 325–333 |

Ranged story periods fire at the start of the range and run via `duration_days`.

### 5.3 Reach rule (pure, `internal/world`)

```
elapsed   = now − started_at (seconds)
earliest  = target_day × 86400 × min_pct_bp / 10000
latest    = target_day × 86400 × max_pct_bp / 10000
threshold = (override ?? threshold_points_per_active) × 1000
reached ⇔ previous milestone reached
        ∧ ( (elapsed ≥ earliest ∧ world_progress_milli ≥ threshold) → reason "threshold"
          ∨  elapsed ≥ latest                                       → reason "max_time" )
```

Admin override can replace threshold/target day, pause, or force (`reason "admin"`). At most one milestone per tick.

### 5.4 Tables

```sql
CREATE TABLE world_state (
    id                       SMALLINT    PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    started_at               TIMESTAMPTZ,            -- NULL = pre-launch: no milestones, catch-up or grant
    era_id                   TEXT,                   -- NULL = first era in catalogue
    last_milestone_id        TEXT,
    progress_milli           BIGINT      NOT NULL DEFAULT 0 CHECK (progress_milli >= 0), -- Σ closed per-capita
    closed_through_day       DATE,                   -- last game day folded into progress_milli
    median_position_milli    BIGINT      NOT NULL DEFAULT 0 CHECK (median_position_milli >= 0),
    median_game_day          DATE,
    paused_at                TIMESTAMPTZ,
    version                  BIGINT      NOT NULL DEFAULT 0,
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO world_state (id) VALUES (1);

-- One row per game day. Recomputed while open; frozen once closed_at is set.
CREATE TABLE world_daily_totals (
    game_day              DATE        PRIMARY KEY,
    credited_milli        BIGINT      NOT NULL CHECK (credited_milli >= 0),  -- Σ points_milli (display/audit)
    catch_up_bonus_milli  BIGINT      NOT NULL CHECK (catch_up_bonus_milli >= 0), -- Σ ×2 portions (audit)
    countable_milli       BIGINT      NOT NULL CHECK (countable_milli >= 0), -- Σ min(cap, points − bonus)
    dau                   INTEGER     NOT NULL CHECK (dau >= 0),             -- rows with points_milli > 0
    per_capita_milli      BIGINT      NOT NULL CHECK (per_capita_milli >= 0),-- countable / dau (0 if dau = 0)
    median_position_milli BIGINT,                                            -- set at close (§5.6)
    computed_at           TIMESTAMPTZ NOT NULL,
    closed_at             TIMESTAMPTZ
);

CREATE TABLE world_milestones_reached (
    milestone_id         TEXT        PRIMARY KEY,
    seq                  INTEGER     NOT NULL UNIQUE,
    reached_at           TIMESTAMPTZ NOT NULL,
    reason               TEXT        NOT NULL CHECK (reason IN ('threshold', 'max_time', 'admin')),
    server_day           INTEGER     NOT NULL CHECK (server_day >= 0),
    progress_milli       BIGINT      NOT NULL CHECK (progress_milli >= 0),
    threshold_milli      BIGINT      NOT NULL CHECK (threshold_milli >= 0),   -- effective, incl. override
    admin_user_id        UUID        REFERENCES users (id)
);

CREATE TABLE world_effects (
    id           BIGSERIAL   PRIMARY KEY,
    milestone_id TEXT        NOT NULL REFERENCES world_milestones_reached (milestone_id),
    effect_index SMALLINT    NOT NULL,
    kind         TEXT        NOT NULL CHECK (kind IN ('set_era','start_event','split_faction','migrate_faction')),
    due_at       TIMESTAMPTZ NOT NULL,          -- reached_at + notice_days
    deadline_at  TIMESTAMPTZ,                   -- split_faction: due_at + choose_days
    applied_at   TIMESTAMPTZ,
    finalized_at TIMESTAMPTZ,                   -- split_faction: defaults assigned after the deadline
    snapshot     JSONB       NOT NULL,          -- effect params at schedule time (audit only)
    UNIQUE (milestone_id, effect_index)
);
CREATE INDEX world_effects_pending_idx ON world_effects (due_at) WHERE applied_at IS NULL OR finalized_at IS NULL;

CREATE TABLE world_milestone_overrides (
    milestone_id                TEXT        PRIMARY KEY,
    threshold_points_per_active BIGINT      CHECK (threshold_points_per_active >= 0),
    target_day                  INTEGER     CHECK (target_day >= 0),
    force_reach                 BOOLEAN     NOT NULL DEFAULT false,
    updated_by                  UUID        NOT NULL REFERENCES users (id),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT now()
);  -- every change also writes admin_audit_logs ('world.milestone_override')
```

### 5.5 Go (sketch)

```go
package world

type Milestone struct {
    ID, Name, Announce string
    Seq                int
    TargetDay          int
    ThresholdMilli     int64 // resolved: override ?? threshold_points_per_active × 1000
    MinPctBP, MaxPctBP int64
    Effects            []Effect
}

type Snapshot struct {
    StartedAt     *time.Time
    ProgressMilli int64
    Reached       map[string]bool
    Paused        bool
}

func Countable(pointsMilli, catchUpBonusMilli, capMilli int64) int64 // min(cap, points − bonus), pure
func PerCapita(countableMilli int64, dau int) int64                 // pure; 0 when dau = 0
func Next(cat *Catalogue, s Snapshot, now time.Time) Decision      // pure, table-tested
func Window(m *Milestone, startedAt time.Time) (earliest, latest time.Time)
func ProgressBP(m *Milestone, progressMilli int64) int64
func Median(positions []int64) int64                                // lower median, pure
```

### 5.6 Worker algorithm (single runner: advisory lock `world` = 7_310_002)

Every `WorkerInterval`:

1. **Recompute open days.** For each game day `d` in {today, yesterday} with `closed_at IS NULL` (`@cap` =
   `world_credit_cap_points × 1000` from YAML):
   ```sql
   INSERT INTO world_daily_totals (game_day, credited_milli, catch_up_bonus_milli, countable_milli, dau,
                                   per_capita_milli, computed_at)
   SELECT @d, COALESCE(SUM(points_milli),0), COALESCE(SUM(catch_up_bonus_milli),0),
          COALESCE(SUM(LEAST(@cap, points_milli - catch_up_bonus_milli)),0), COUNT(*),
          COALESCE(SUM(LEAST(@cap, points_milli - catch_up_bonus_milli)),0) / GREATEST(COUNT(*),1), @now
   FROM daily_contribution WHERE game_day = @d AND points_milli > 0
   ON CONFLICT (game_day) DO UPDATE SET … WHERE world_daily_totals.closed_at IS NULL;
   ```
   Recompute, never increment → replay-safe. Uses `daily_contribution_day_idx`.
2. **Close a day.** If `now ≥ midnight_after(d) + day_close_grace_seconds` and `d` is not closed:
   compute the server median ladder position over characters **active in the 7 game days ending at d** (keyset pages
   of `character_progressions` joined with `daily_contribution`, position via `realm.Position` in Go — the catalogue
   isn't in SQL), then set `median_position_milli`, `closed_at`.
3. **Fold into world_state** (`SELECT … FOR UPDATE`): `progress_milli += per_capita_milli` for every newly closed day
   after `closed_through_day` (only days ≥ `gameday.Of(started_at)`), advance `closed_through_day`, copy the latest
   median into `median_position_milli`/`median_game_day`.
4. **Evaluate** `world.Next`. If a milestone: `INSERT world_milestones_reached … ON CONFLICT DO NOTHING RETURNING`;
   if inserted: insert its `world_effects`, apply immediate `set_era`, outbox `world.milestone_reached`. Commit.
5. **Effects** (separate tx each): claim a due effect `FOR UPDATE SKIP LOCKED`, apply in chunks of 500 characters
   (history idem key `effect:<id>`), set `applied_at`, outbox `world.effect_applied`. For `split_faction` after
   `deadline_at`: count members of each `into` branch (`character_faction`), assign every still-pending member to the
   **branch with the fewest members at that moment** (ties → catalogue order; computed once, not per member, so the
   batch doesn't oscillate), reason `split_default`, then `finalized_at`.

`GET /world` shows `progress_milli` plus today's running per-capita as a provisional estimate; only closed days count
for reaching milestones.

### 5.7 Invariants

| # | Invariant | Enforced by |
|---|---|---|
| W1 | Each milestone reached at most once | PK + `ON CONFLICT DO NOTHING` |
| W2 | Milestones reached in catalogue order | `world.Next` + `seq UNIQUE` |
| W3 | Never before 70%, always by 150% (absent admin) | `world.Next` table test |
| W4 | Nothing advances while not started or paused | `world.Next` |
| W5 | Open day totals = SUM/COUNT of `daily_contribution`; closed days never change | recompute query; `closed_at` guard |
| W5b | A character contributes ≤ 100 points/day to progress and never its ×2 bonus | `LEAST(@cap, points − bonus)` + `world.Countable` test |
| W6 | `progress_milli` = Σ `per_capita_milli` of closed days since launch, each folded once | `closed_through_day` under row lock; reconcile test |
| W7 | Grant releases never appear in DAU, totals or progress | grant not in `daily_contribution` |
| W8 | Each effect applied once; default split branch decided once per effect | UNIQUE + `applied_at`/`finalized_at` |
| W9 | Admin overrides audited | same tx as `admin_audit_logs` |

Loader invariants (`world.Load`): version 1; `progress_mode == per_active_capita`; `exclude_catch_up_bonus == true`;
`0 < reference_points_per_active_day ≤ world_credit_cap_points ≤ contribution.daily_ceiling_points` (cross-check);
every milestone has `threshold_points_per_active` (the first may be 0); a test warns (not fails) when a threshold
differs from `target_day × reference_points_per_active_day`;
`0 < min_pct_bp ≤ 10000 ≤ max_pct_bp ≤ 30000`; `0 ≤ day_close_grace_seconds ≤ 3600`; IDs slug/unique; first
milestone `target_day == 0`; `target_day` and resolved threshold non-decreasing; effects have exactly their kind's
fields; `set_era` eras in non-decreasing order; `split_faction.into` non-empty and unique, `notice_days, choose_days ≥ 0`.
Cross-checks: faction IDs exist and their era windows match the effect (§6.4 F7); `announce` keys exist in
`content/story/announcements/world-milestones.yaml` (worker-start check).

---

## 6. Factions: `factions.yaml` + `character_faction` / `faction_history`

### 6.1 YAML

Roster and permanent IDs are named by `story-creator` before first deploy (decision 12). IDs below are
**placeholders to be replaced, not shipped**.

```yaml
version: 1
settings:
  switch_cooldown_seconds: 604800     # 7 days between player-initiated switches
  first_join_free: true

factions:
  - id: huyen_mon                     # stand-in for the Tam Thanh lineage (D2)
    name: Huyền Môn
    name_placeholder: true
    allowed_paths: [tien]
    open_from_era: khai_giang
    close_from_era: thanh_nhan        # split at the Thánh nhân milestone
    visual: { badge: faction-huyen-mon, color: jade }
  - { id: tay_phuong_mon, name: Tây Vực Môn,  name_placeholder: true, allowed_paths: [tien], open_from_era: khai_giang, close_from_era: thanh_nhan }
  - { id: dong_tien_dinh, name: Đông Hoa Đình, name_placeholder: true, allowed_paths: [tien], open_from_era: khai_giang, close_from_era: luan_hoi }
  - { id: yeu_dinh,       name: Yêu Đình,     name_placeholder: true, allowed_paths: [tien], open_from_era: khai_giang }
  - { id: vu_toc,         name: Vu Tộc,       allowed_paths: [vu],   open_from_era: khai_giang }
  - { id: nhan_toc,       name: Nhân Tộc,     allowed_paths: [tien], open_from_era: tao_nhan }
```

### 6.2 Tables

```sql
CREATE TABLE character_faction (
    character_id            UUID        PRIMARY KEY REFERENCES characters (id) ON DELETE CASCADE,
    faction_id              TEXT        NOT NULL,
    joined_at               TIMESTAMPTZ NOT NULL,
    last_switch_at          TIMESTAMPTZ,              -- last PLAYER-initiated switch (cooldown anchor)
    lock_reason             TEXT        CHECK (lock_reason IN ('fengshen', 'admin', 'event')),
    locked_at               TIMESTAMPTZ,
    locked_until            TIMESTAMPTZ,              -- NULL with lock_reason = permanent
    pending_split_effect_id BIGINT      REFERENCES world_effects (id),
    version                 BIGINT      NOT NULL DEFAULT 0,
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((lock_reason IS NULL) = (locked_at IS NULL))
);
CREATE INDEX character_faction_faction_idx ON character_faction (faction_id);
CREATE INDEX character_faction_pending_idx ON character_faction (pending_split_effect_id)
    WHERE pending_split_effect_id IS NOT NULL;

CREATE TABLE faction_history (
    id              BIGSERIAL   PRIMARY KEY,
    character_id    UUID        NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    from_faction_id TEXT,
    to_faction_id   TEXT        NOT NULL,
    from_path       TEXT,
    to_path         TEXT,
    reason          TEXT        NOT NULL CHECK (reason IN ('join','switch','split_choice','split_default',
                                                          'auto_migrate','reincarnation','admin')),
    world_effect_id BIGINT      REFERENCES world_effects (id),
    idempotency_key TEXT        NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (character_id, idempotency_key)
);
CREATE INDEX faction_history_char_idx ON faction_history (character_id, created_at DESC);
```

### 6.3 Go (sketch)

```go
package faction

type Faction struct {
    ID, Name        string
    NamePlaceholder bool
    AllowedPaths    []realm.PathID
    OpenFromEra     string
    CloseFromEra    string
    Visual          Visual
}

type Membership struct {
    FactionID      string     `json:"factionId"`
    JoinedAt       time.Time  `json:"joinedAt"`
    CanSwitchAt    *time.Time `json:"canSwitchAt"`
    LockReason     string     `json:"lockReason,omitempty"`
    MustChoose     []string   `json:"mustChoose,omitempty"`
    ChooseDeadline *time.Time `json:"chooseDeadline,omitempty"`
}

func CanJoin(cat *Catalogue, eras EraIndex, currentEra string, m *Membership, target string,
    curPath, wantPath realm.PathID, now time.Time) (convertTo realm.PathID, err error) // pure
func DefaultBranch(into []string, members map[string]int) string                    // fewest, ties → order
```

### 6.4 Invariants

| # | Invariant | Enforced by |
|---|---|---|
| F1 | At most one faction per character | PK |
| F2 | Player joins only factions open in the current era | `CanJoin` |
| F3 | Path ∈ faction `allowed_paths` after every change (conversion in same tx) | `CanJoin` + `realm.Convert` under progression lock |
| F4 | Player switches respect the cooldown; worker/admin moves don't touch it | `CanJoin` |
| F5 | `fengshen`-locked characters can't switch | `CanJoin` |
| F6 | One `faction_history` row per change | same tx; idem key |
| F7 | Members of a closing faction are moved by an effect or marked pending | catalogue cross-check + worker finalize |
| F8 | Default split branch = fewest members at deadline | `DefaultBranch` (pure test) |

Loader: version 1; slug/unique IDs; `allowed_paths` non-empty ⊆ realm paths; eras exist and open < close;
`switch_cooldown_seconds ≥ 0`; at least one faction open in the first era for **every** path; release gate on
placeholders.

---

## 7. Tiên Duyên (VIP v3)

Display rename only: package `vip`, tables and `/vip/*` endpoints stay. Level = f(cumulative points), per user.

### 7.1 `vip.yaml` v3

```yaml
version: 3
display_name: Tiên Duyên
point_name: điểm duyên
points_per_1000_vnd: 10
hard_max_offline_cap_hours: 36
daily_cultivation_ceiling_hours: 24
# daily_gift: spirit stones claimable per day at this tier (absolute, not cumulative).
# BUDGET (decisions 2026-11-06 "Phase 0" #5): stones turn into cultivation ONLY through speed items, capped per
#   character per game day (Tụ Linh Đan 2 × 5 = 10 points, cost 250 ≤ stone income of a free 3-login idle player).
#   ⇒ a gift's progress value ≤ 10 − 4 (what a free 1-login player self-funds) = 6 points/day, and 0 for anyone
#   doing daily tasks. Invariant checked by balancesim: tier L at 1 login/day ≤ free at 3 logins/day, every tier,
#   every activity profile. Gift stones beyond the speed-item budget need non-progress sinks (cosmetics, cave,
#   breakthrough-requirement items); track stone creation vs destruction per day.

earn:                                 # decision D: ≤ 20/day across non-top-up sources (theoretical max 29)
  daily_cap_points: 20
  activities:
    - { id: online_cultivation,  source: cultivation, points: 1, daily_max_times: 3 }  # settles ≥ 1 h apart
    - { id: lecture_attendance,  source: lecture,     points: 3, daily_max_times: 2 }
    - { id: daily_task_claim,    source: task,        points: 1, daily_max_times: 4 }
    - { id: protector_assist,    source: interaction, points: 2, daily_max_times: 2 }
    - { id: dual_cultivation,    source: interaction, points: 2, daily_max_times: 1 }
    - { id: faction_merit,       source: interaction, points: 1, daily_max_times: 3 }
    - { id: trial_pass,          source: trial,       points: 2, daily_max_times: 2 }
    - { id: event_participation, source: event,       points: 3, daily_max_times: 1 }

tiers:                                # decision E thresholds; caps/gifts/perks unchanged from vip.yaml v2
  - { tier: 0,  name: Tán Tu,         points_required: 0,      offline_cap_hours: 8,  daily_gift: 0,   perks: [] }
  - { tier: 1,  name: Tiên Duyên 1,   points_required: 30,     offline_cap_hours: 10, daily_gift: 0,   perks: ["title:vip_1"] }
  - { tier: 2,  name: Tiên Duyên 2,   points_required: 150,    offline_cap_hours: 12, daily_gift: 50,  perks: ["title:vip_2"] }
  - { tier: 3,  name: Tiên Duyên 3,   points_required: 500,    offline_cap_hours: 14, daily_gift: 50,  perks: ["title:vip_3", "avatar_frame:ngoc", alchemy_slot] }
  - { tier: 4,  name: Tiên Duyên 4,   points_required: 1200,   offline_cap_hours: 16, daily_gift: 50,  perks: ["title:vip_4", garden_auto_harvest] }
  - { tier: 5,  name: Tiên Duyên 5,   points_required: 2500,   offline_cap_hours: 18, daily_gift: 100, perks: ["title:vip_5", "chat_aura:linh_quang"] }
  - { tier: 6,  name: Tiên Duyên 6,   points_required: 4500,   offline_cap_hours: 20, daily_gift: 100, perks: ["title:vip_6", "avatar_frame:kim", alchemy_slot] }
  - { tier: 7,  name: Tiên Duyên 7,   points_required: 10000,  offline_cap_hours: 22, daily_gift: 100, perks: ["title:vip_7", "cave_decoration:van_hac"] }
  - { tier: 8,  name: Tiên Duyên 8,   points_required: 25000,  offline_cap_hours: 24, daily_gift: 200, perks: ["title:vip_8"] }
  - { tier: 9,  name: Tiên Duyên 9,   points_required: 50000,  offline_cap_hours: 26, daily_gift: 200, perks: ["title:vip_9", "avatar_frame:thien", alchemy_slot] }
  - { tier: 10, name: Tiên Duyên 10,  points_required: 100000, offline_cap_hours: 28, daily_gift: 200, perks: ["title:vip_10", "chat_aura:tu_quang"] }
  - { tier: 11, name: Tiên Duyên 11,  points_required: 250000, offline_cap_hours: 30, daily_gift: 300, perks: ["title:vip_11"] }
  - { tier: 12, name: Tiên Duyên 12,  points_required: 500000, offline_cap_hours: 32, daily_gift: 300, perks: ["title:chi_ton", "breakthrough_fx:chi_ton"] }
```

Expected free-play levels (balance §11.3): active free players reach level 3–4 by day 90 and level 6 after a year;
level 8+ is effectively top-up only. `vip.MaxGrantDelta` (currently 100,000) must be raised to ≥ 500,000 so a single
admin grant/top-up can cover level 12 — it is a typo guard, not a game number.

### 7.2 Ledger changes

```sql
ALTER TABLE vip_point_ledger DROP CONSTRAINT vip_point_ledger_reason_check;
ALTER TABLE vip_point_ledger
    ADD COLUMN source            TEXT   NOT NULL CHECK (source IN ('topup','cultivation','lecture','interaction',
                                                                   'task','trial','event','admin')),
    ADD COLUMN activity_id       TEXT,
    ADD COLUMN game_day          DATE   NOT NULL,
    ADD COLUMN reverses_ledger_id BIGINT UNIQUE REFERENCES vip_point_ledger (id), -- refund/revoke of exactly one row
    ADD CONSTRAINT vip_point_ledger_reason_check
        CHECK (reason IN ('purchase','refund','earn','admin_grant','admin_revoke','promo')),
    ADD CONSTRAINT vip_point_ledger_activity_chk
        CHECK ((source IN ('topup','admin')) = (activity_id IS NULL)),
    ADD CONSTRAINT vip_point_ledger_refund_chk
        CHECK ((reason = 'refund') = (reverses_ledger_id IS NOT NULL));
CREATE INDEX vip_point_ledger_user_day_idx ON vip_point_ledger (user_id, game_day);
```

### 7.3 Refunds (decision 10)

A refund/chargeback inserts one row with `delta = −original.delta`, `reason='refund'`, `source='topup'`,
`reverses_ledger_id = original.id` (UNIQUE → a purchase is reversed at most once). The level is recomputed from
points, **so it may drop**; offline cap and gift follow the new level from the next settlement/claim. Already-claimed
gifts are not clawed back. Points can't go negative (points are never spent), so the CHECK holds.

### 7.4 Go (sketch)

```go
func (s *Service) EarnLocked(ctx context.Context, q *db.Queries, userID uuid.UUID, activityID, idemKey string, now time.Time) (int64, error)
func (s *Service) TopupLocked(ctx context.Context, q *db.Queries, userID uuid.UUID, amountVND int64, orderRef, idemKey string) (Status, error)
func (s *Service) RefundLocked(ctx context.Context, q *db.Queries, userID uuid.UUID, originalLedgerID int64, idemKey string) (Status, error)
```

### 7.5 Invariants

| # | Invariant | Enforced by |
|---|---|---|
| V1 | Tiên Duyên never changes rate, odds or ranking points | perk whitelist; `contribution`/`breakthrough` never read VIP except offline cap |
| V2 | Non-topup earn per (user, day) ≤ `earn.daily_cap_points`; per activity ≤ `daily_max_times` | `EarnLocked` under `vip_accounts` lock |
| V3 | One ledger row per change, idempotent per (user, key) | existing UNIQUE |
| V4 | A purchase is refunded at most once, for exactly its amount | `reverses_ledger_id UNIQUE` + `RefundLocked` |
| V5 | Daily gift once per (user, day) | existing PK |
| V6 | TD12@1 login ≤ free@3 logins; gift-driven speed ≤ 6 points/day | balancesim + speed-item cap |

Loader: version 3; `points_per_1000_vnd > 0`; display names non-empty; activity IDs slug/unique; `source` ∉
`{topup, admin}`; `points > 0`; `daily_max_times ≥ 1`; `daily_cap_points > 0`; tier names don't contain "VIP".

---

## 8. Shop: `shop.yaml` + purchase ledger

### 8.1 Item effect fields (`items.yaml`, additive)

```yaml
  - id: tu_linh_dan
    name: Tụ Linh Đan
    category: pill
    rarity: linh
    tradable: true
    instant_points_milli: 5000        # +5 points via contribution (source item, activity bucket)
    max_use_per_game_day: 2           # per character per game day of USE (gifted/traded pills included)
    description: …
  # Phase 3 pháp bảo — never sellable:
  - id: loi_van_phu
    category: artifact
    tribulation_bonus_bp: 300
```

`item.Def` gains `InstantPointsMilli`, `MaxUsePerGameDay`, `BreakthroughBonusBP`, `TribulationBonusBP` and
`EffectKeys()`. Category `artifact` added. Rate-buff items are **not** introduced for the shop (they would escape the
points-based speed cap); fortune buffs (§9.4) are not shop items.

Item loader rules: `instant_points_milli > 0` ⇒ `max_use_per_game_day ≥ 1`. **Cross-check with
`contribution.yaml`:** `Σ over items with instant_points_milli of (instant_points_milli × max_use_per_game_day)` ≤
`speed_items.daily_cap_points × 1000` — so the per-item limits alone already respect the 10-point cap, and the runtime
clip (§4.5) is a second guard. Example: Tụ Linh Đan 5 × 2 = 10 ✓.

### 8.2 YAML

```yaml
version: 1
settings:
  currency: spirit_stone
  max_lots_per_order: 10
listings:
  - { id: tu_linh_dan,   item_id: tu_linh_dan,  lot_size: 1,  price: 125, daily_limit_lots: 2 }
  - { id: tu_linh_phu,   item_id: tu_linh_phu,  lot_size: 1,  price: 300, daily_limit_lots: 1, min_tier: 1 }
  - { id: truc_co_dan,   item_id: truc_co_dan,  lot_size: 1,  price: 400, daily_limit_lots: 1 }   # requirement item (decision 11)
  - { id: ket_dan_qua,   item_id: ket_dan_qua,  lot_size: 1,  price: 900, daily_limit_lots: 1, min_tier: 2 }
  - { id: linh_thao_x10, item_id: linh_thao,    lot_size: 10, price: 50,  daily_limit_lots: 5 }
  # Non-progress sinks (decision G) — ship WITH the shop, before bí cảnh adds stones:
  - { id: khung_ngoc_bich, item_id: avatar_frame_ngoc_bich, lot_size: 1, price: 3000 }   # cosmetic, one-off
  - { id: dong_phu_tung_hac, item_id: cave_deco_tung_hac,  lot_size: 1, price: 5000 }   # cave decoration
```

Prices are balance defaults (Tụ Linh Đan 125 from the balance doc; others placeholder). `daily_limit_lots` is
optional for non-progress items (no limit); cosmetics/cave items use the existing `cosmetic` category with an
optional `unique_per_character: true` item flag (buying a second copy is refused).

**Spirit-stone sinks (decision G).** Stones convert into progress only through speed items (≤ 10 points/day). An
active player has ~200–300 stones/day left over, so the shop must also sell items that **do not speed progress**:
cosmetics (frames, titles, auras), cave decorations, and breakthrough-requirement items (decision 11). Content
check: `shop.yaml` must contain at least one listing whose item has no `EffectKeys()` and category `cosmetic`
(fails `make content-check` otherwise). The worker logs daily stone creation (`currency_ledger` delta > 0) vs
destruction (delta < 0) for balance tracking.

### 8.3 Tables

```sql
CREATE TABLE shop_purchases (
    id              BIGSERIAL   PRIMARY KEY,
    character_id    UUID        NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    listing_id      TEXT        NOT NULL,
    item_id         TEXT        NOT NULL,
    lots            INTEGER     NOT NULL CHECK (lots > 0),
    item_quantity   BIGINT      NOT NULL CHECK (item_quantity > 0),
    currency        TEXT        NOT NULL CHECK (currency IN ('spirit_stone')),
    unit_price      BIGINT      NOT NULL CHECK (unit_price > 0),
    total_price     BIGINT      NOT NULL CHECK (total_price = unit_price * lots),
    game_day        DATE        NOT NULL,
    idempotency_key TEXT        NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (character_id, idempotency_key)
);
CREATE INDEX shop_purchases_char_time_idx ON shop_purchases (character_id, created_at DESC);

CREATE TABLE shop_daily_counts (
    character_id UUID    NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    game_day     DATE    NOT NULL,
    listing_id   TEXT    NOT NULL,
    lots         INTEGER NOT NULL CHECK (lots >= 0),
    PRIMARY KEY (character_id, game_day, listing_id)
);
```

Ledger rows: `currency_ledger(reason='shop_purchase', ref_type='shop_listing', ref_id=listing_id)` and
`inventory_transactions(reason='shop_purchase')`, same idempotency key.

### 8.4 Invariants

| # | Invariant | Enforced by |
|---|---|---|
| S1 | **No listing sells an item that raises breakthrough/tribulation odds** (requirement items allowed) | `content.LoadAll` cross-check |
| S2 | Lots per (char, day, listing) ≤ `daily_limit_lots` (when set) | conditional upsert |
| S7 | `unique_per_character` items can be owned at most once | inventory check under the inventory row lock |
| S3 | Balance never negative | `AddCurrency` guard + CHECK |
| S4 | One purchase = 1 `shop_purchases` + 1 currency row + 1 inventory row, same tx/key | service + integration test |
| S5 | Price from catalogue only | API takes `listingId`, `lots` |
| S6 | Speed effect ≤ 10 points/day regardless of how many pills are bought | `item_daily_uses` + `speed_item_milli` clip |

Loader (`shop.Load`): version 1; `currency == spirit_stone`; slug/unique listing IDs; `lot_size > 0`, `price > 0`,
`daily_limit_lots` absent or `1..1000` (**required** when the item has any effect key); `max_lots_per_order ≥ 1`;
`min_tier ≥ 1`; at least one non-progress `cosmetic` listing (decision G). Cross-check: `item_id` exists; category ≠
`artifact`; `BreakthroughBonusBP == 0 && TribulationBonusBP == 0`; every `EffectKeys()` entry is in the shop-safe
whitelist `{offline_cap_bonus_hours, instant_points_milli, max_use_per_game_day}`. Items without effects (requirement
items, materials) pass. `min_tier` ≤ last playable tier; `open_from_era` exists.

---

## 9. Other Phase 1–2 data items (brief)

### 9.1 Lectures (2.4)

```yaml
# content/catalog/lectures.yaml
version: 1
slots:
  - { id: sang, start_local: "07:30", duration_minutes: 20 }
  - { id: toi,  start_local: "20:30", duration_minutes: 20 }
attendance: { min_presence_bp: 8000, require_insight: true, insight_window_seconds: 120, heartbeat_seconds: 20 }
reward: { points_milli: 10000, vip_activity: lecture_attendance }   # activity bucket
```

```sql
CREATE TABLE lecture_sessions (
    id TEXT PRIMARY KEY,                 -- "<yyyymmdd>.<slot>", created idempotently by the worker
    slot_id TEXT NOT NULL, game_day DATE NOT NULL,
    starts_at TIMESTAMPTZ NOT NULL, ends_at TIMESTAMPTZ NOT NULL CHECK (ends_at > starts_at),
    insight_opens_at TIMESTAMPTZ NOT NULL, era_id TEXT, content_ref TEXT
);
CREATE TABLE lecture_attendance (
    session_id TEXT NOT NULL REFERENCES lecture_sessions (id),
    character_id UUID NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    first_seen_at TIMESTAMPTZ NOT NULL,
    presence_seconds INTEGER NOT NULL DEFAULT 0 CHECK (presence_seconds >= 0),  -- copied from Redis at claim
    insight_at TIMESTAMPTZ, credited_at TIMESTAMPTZ,
    PRIMARY KEY (session_id, character_id)
);
```

Daily task `trung_kich` is replaced by `nghe_giang` (kind `lecture_attend`, target 1) once lectures ship (decision 13).
Behaviour when Redis presence is lost mid-session: open (§17 O4).

### 9.2 Morning incense streak

`daily_tasks.yaml` kind `incense` + `streak_rewards`; new reward kind `{ points_milli: N }` (via `CreditLocked`,
source `ritual`). State `ritual_streaks(character_id PK, last_day DATE, streak INT ≥ 1, best INT ≥ streak)`.
No "nhận lộc" framing.

### 9.3 Ngộ đạo / gieo quẻ

`fortunes.yaml`: outcomes with `weight_bp` (Σ = 10000, disclosed) and `rate_bonus_bp ≤ 2000` until end of game day.
`daily_fortunes(character_id, game_day, outcome_id, roll_bp, created_at, PK(character_id, game_day))`.

### 9.4 Fortune rate buff

`character_buffs(character_id, slot CHECK IN ('fortune'), bonus_bp, starts_at, ends_at, source_ref, PK(character_id, slot))`.
Settle splits eligible time at buff boundaries; bonus capped by `contribution.yaml` `idle.max_buff_bonus_bp`
(proposed 2000); the idle bucket cap (75) still bounds the result. The `slot` CHECK can be widened later.

### 9.5 Moderation (1.5)

`content/catalog/name_filter.yaml` (normalization rules, banned substrings, reserved story names).

```sql
CREATE TABLE reports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reporter_character_id UUID NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    target_character_id UUID NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('name', 'harassment', 'other')),
    note TEXT CHECK (char_length(note) <= 500),
    status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'actioned', 'dismissed')),
    handled_by UUID REFERENCES users (id), handled_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (reporter_character_id <> target_character_id)
);
CREATE UNIQUE INDEX reports_open_uq ON reports (reporter_character_id, target_character_id, kind) WHERE status = 'open';
CREATE INDEX reports_open_idx ON reports (created_at) WHERE status = 'open';

CREATE TABLE moderation_actions (
    id BIGSERIAL PRIMARY KEY, admin_user_id UUID NOT NULL REFERENCES users (id),
    target_character_id UUID NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    action TEXT NOT NULL CHECK (action IN ('force_rename', 'mute', 'ban', 'dissolve_bond', 'warn')),
    report_id UUID REFERENCES reports (id), reason TEXT NOT NULL, until_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 9.6 Onboarding (Tử Tiêu Cung)

Client steps (video → Tử Tiêu Cung → faction → path → name) end in one intent `POST /character {name, factionId,
path}`: creates `characters`, `character_progressions` at `First(path)`, `character_faction` (reason `join`),
`faction_history`, and — if eligible — `late_join_grants` + `grant_reserve_milli` (§4.6), all in one tx.

---

## 10. Redis keys (all rebuildable or safe to lose)

| Key | Type | TTL | Writer | Rebuild source |
|---|---|---|---|---|
| `leaderboard:global:realm` | ZSET char → score | — | worker | `character_progressions` |
| `leaderboard:faction:merit` | ZSET faction_id → merit points | — | worker reconcile | `SUM(daily_contribution.points_milli) GROUP BY faction_id` over the current era (grant excluded by construction) |
| `leaderboard:faction:<faction_id>:realm` | ZSET char → global score | — | worker (`leaderboard.dirty`, `faction.changed`) | `character_progressions ⋈ character_faction` |
| `leaderboard:faction:members` | HASH faction_id → count | — | worker reconcile | `COUNT(*) GROUP BY faction_id` |
| `world:global:snapshot` | STRING (public `GET /world`) | 30 s | API read-through | `world_state` + `world_daily_totals` + catalogue |
| `presence:char:<character_id>` | STRING room id | 60 s | API (WS heartbeat) | safe to lose |
| `presence:room:<room_id>` | ZSET char → last_seen | 2 h, pruned by score | API | safe to lose |
| `lecture:<session_id>:presence` | HASH char → seconds | session end + 1 h | API (HINCRBY, min interval) | see §17 O4 |
| `rate:ratelimit:<name>:<key>` | GCRA | lib | API | safe to lose |

---

## 11. API shapes (camelCase JSON)

All mutations take `Idempotency-Key` and return full authoritative state.

### `GET /world`

```json
{
  "started": true, "startedAt": "2027-01-10T01:00:00Z", "serverDay": 12,
  "era": { "id": "khai_giang", "name": "Tử Tiêu khai giảng" },
  "progress": {
    "perActiveMilli": 742000, "closedThroughDay": "2027-01-21",
    "todayProvisionalMilli": 31000, "yesterdayDau": 4120, "referencePerDayMilli": 67000
  },
  "lastMilestone": { "id": "khai_giang", "name": "Hồng Quân giảng đạo lần 1", "reachedAt": "…" },
  "nextMilestone": {
    "id": "giang_dao_2", "name": "Hồng Quân giảng đạo lần 2", "targetDay": 30,
    "thresholdPerActiveMilli": 2010000, "progressBp": 3691,
    "earliestAt": "2027-01-31T01:00:00Z", "latestAt": "2027-02-24T01:00:00Z", "estimatedAt": "2027-02-10T00:00:00Z"
  },
  "pendingEffects": [ { "kind": "split_faction", "dueAt": "…", "from": "huyen_mon" } ],
  "paused": false
}
```

`estimatedAt` = clamp(now + remaining / avg(last 7 closed `per_capita_milli`), earliest, latest), `null` without data.

### Cultivation status — additions

```json
{
  "stageId": "luyen_hu.so_ky_2", "path": "tien", "tier": 6,
  "cultivationMilli": 112400, "requiredMilli": 262000, "capMilli": 0, "bottleneckRoomMilli": 1778200,
  "grantReserveMilli": 0,
  "idleRateMilliPerHour": 2083, "offlineCapSeconds": 28800, "lastCultivatedAt": "…",
  "earnedMilli": 16660, "stagesAdvanced": ["luyen_hu.so_ky_1"], "hitOfflineCap": false, "hitBottleneck": false,
  "today": {
    "gameDay": "2027-01-22", "pointsMilli": 82000, "ceilingMilli": 150000,
    "buckets": { "idle": { "milli": 41000, "capMilli": 75000 }, "activity": { "milli": 31000, "capMilli": 45000 },
                 "community": { "milli": 10000, "capMilli": 30000 } },
    "speedItems": { "milli": 5000, "capMilli": 10000 },
    "catchUp": { "active": true, "multiplierBp": 20000 },
    "resetsAt": "2027-01-22T17:00:00Z"
  }
}
```

### `GET /breakthrough/preview` — additions

```json
{
  "from": "hoa_than.vien_man_2", "to": "luyen_hu.so_ky_1", "kind": "major",
  "ready": true, "missing": [], "consumes": [],
  "settlementIncluded": true, "projectedCultivationMilli": 271000,
  "successBp": 8346,
  "odds": [ { "source": "base", "bp": 8000 }, { "source": "surplus", "bp": 346 },
            { "source": "items", "bp": 0 }, { "source": "protector", "bp": 0 } ],
  "tribulation": {
    "name": "Lôi Kiếp", "guaranteed": false, "failStreak": 1, "guaranteeAfter": 3,
    "lossBp": 2500, "lossMilli": 67750, "cooldownSeconds": 86400
  }
}
```

`POST /breakthrough/attempt` body `{ "acceptLossMilli": 67750 }` (optional guard, §3.6); `409 quote_changed`
returns a fresh preview. Result adds `tribulation`, `lossMilli`, `guaranteed`.

### Factions, Shop, Tiên Duyên, Admin

```
GET  /factions              → { factions: [{ id, name, allowedPaths, open, members, meritRank, meritPoints, visual }], switchCooldownSeconds }
GET  /factions/me           → Membership + { path }
POST /factions/join         { factionId, path? }  → { membership, progression }
POST /factions/choose       { factionId }         → { membership }
GET  /factions/{id}/members?cursor=               → { entries: [{ rank, characterId, name, stageId, tier }], nextCursor }

GET  /shop                  → { currency, balance, listings: [{ id, itemId, name, lotSize, price, dailyLimitLots,
                                 boughtTodayLots, available, lockedReason }], resetsAt }
POST /shop/buy              { listingId, lots }   → { purchaseId, balance, item: { itemId, quantity }, boughtTodayLots, replayed }
POST /items/use             { itemId }            → { item, usesToday, maxUsesPerDay, cultivation status }

GET  /vip/status            + displayName, pointName, earnedToday, earnDailyCap, activities[{ id, points, timesToday, dailyMaxTimes }]
GET  /vip/tiers             + pointsPer1000Vnd

POST /admin/world/start | /pause | /resume                       (audited)
PUT  /admin/world/milestones/{id}  { thresholdPointsPerActive?, targetDay?, forceReach? }   (audited)
GET  /admin/world                  → progress, last 14 world_daily_totals (dau, perCapita, median), overrides, pending effects
POST /admin/vip/refund             { ledgerId, reason }          (audited)
```

---

## 12. Mutation flows

**F1. Settle**: lock progression → idem check (`cultivation_settlements`) → eligible seconds (offline cap incl. Tụ
Linh Phù, fortune buff) → `raw_milli` + `rate_carry` → `CreditLocked(settle)` (catch-up from `world_state` median;
bucket/ceiling; bottleneck; auto-advance; reserve release) → insert `cultivation_settlements` (`raw_milli`,
`credited_milli`, `multiplier_bp`, `game_day`) → daily-task hook → `EarnLocked(online_cultivation)` if ≥ 1 h since
the previous counted settle → outbox `leaderboard.dirty`.

**F2. Other credits** (lecture, task, ritual, trial, events): lock progression → idem check on the source's own log
→ `SettleLocked` first → `CreditLocked(source)` → source writes → optional `EarnLocked` → outbox.

**F3. Speed item use**: lock progression → idem check (`inventory_transactions` key) → `SettleLocked` → conditional
upsert `item_daily_uses` (→ `daily_limit`) → lock inventory row, consume 1 → `CreditLocked(item)` with speed-cap clip.

**F4. Tribulation attempt**: lock progression → idem check (`breakthrough_attempts`) → `SettleLocked` (same pure
projection the preview used) → lock requirement inventory → `Evaluate` → guard `acceptLossMilli` → consume items →
roll → insert attempt (`tribulation`, `success_bp`, `roll_bp`, `cultivation_before_milli`, `loss_milli`,
`guaranteed`, `odds`) → success: advance + carry auto-advance + reserve release + events + outbox; failure:
`ApplyTribulationFailure` + outbox.

**F5. Join / switch faction**: lock progression → lock `character_faction` → idem check (`faction_history`) →
`CanJoin` → optional `Convert` → upsert membership → history → outbox `faction.changed`.

**F6. World tick**: §5.6.

**F7. Shop buy**: lock currency account → idem check (`shop_purchases`) → validate listing (era, `min_tier`, lots)
→ conditional upsert `shop_daily_counts` → `economy.Credit(−total)` → `economy.GrantItem` → insert purchase.

**F8. Character creation with grant**: §9.6; grant only if started, server day > 14 and no `late_join_grants` row for
the user.

**F9. Tiên Duyên earn / top-up / refund**: lock `vip_accounts` (after any progression lock) → idem check → caps
(earn) or `reverses_ledger_id` uniqueness (refund) → ledger + `AddVIPPoints`.

---

## 13. Migration plan

Pre-launch: character data **is reset**; users, identities and refresh tokens are kept. Migrations 000001–000004
are not edited.

### `000005_world_story_foundation` (Phase 1)
1. `TRUNCATE characters CASCADE`; `TRUNCATE vip_point_ledger, vip_accounts, vip_daily_gift_claims`;
   `DELETE FROM outbox_events`. Comment: *pre-launch only* (old stage IDs and the 1 point = 1.000đ scale can't be
   migrated meaningfully).
2. `character_progressions`: rename `cultivation → cultivation_milli`; drop `remainder_milli`; add `rate_carry`,
   `grant_reserve_milli`.
3. `cultivation_settlements`: rename `rate_milli → rate_milli_per_hour`, `earned → credited_milli`,
   `cultivation_after → cultivation_after_milli`; add `raw_milli`, `multiplier_bp`, `game_day`.
4. `breakthrough_attempts`: add `tribulation`, `guaranteed`, `cultivation_before_milli`, `loss_milli`
   (`CHECK (tribulation OR loss_milli = 0)`), `odds JSONB` (audit).
5. `daily_contribution`, `cultivation_credits`, `item_daily_uses`, `late_join_grants` (§4.3).
6. `world_state` (needed in Phase 1 for `started_at` and the median); `world_daily_totals` (the median is computed at
   day close, which catch-up and the grant need in Phase 1).
7. `vip_point_ledger` changes (§7.2).
8. `reports`, `moderation_actions`.

Down: drop new tables/columns, rename back, restore old CHECKs (data not restored — documented).

### `000006_world_factions` (Phase 2)
`world_milestones_reached`, `world_effects`, `world_milestone_overrides`, `character_faction`, `faction_history`,
`lecture_sessions`, `lecture_attendance`, `ritual_streaks`, `daily_fortunes`, `character_buffs`.

### `000007_shop` (Phase 2.5)
`shop_purchases`, `shop_daily_counts`.

Kept: `vip_entitlements`, `progression_events`, `outbox_events`, `admin_audit_logs`. Dropped: column `remainder_milli`
only. At open launch: reset runbook, then `POST /admin/world/start` (day 0).

### sqlc queries

- `progression.sql`: `UpdateProgressionCultivation` (`cultivation_milli`, `rate_carry`, `grant_reserve_milli`,
  `stage_id`, `realm_id`, `stage_reached_at` — auto-advance writes stage too), `ApplyTribulationFailure`,
  `ConvertProgressionPath`, `InsertSettlement`, `ListActivePositionsPage` (keyset; characters with contribution in a
  7-day window, for the median).
- `contribution.sql`: `EnsureDailyContribution` + `GetDailyContributionForUpdate`, `AddDailyContribution`,
  `InsertCultivationCredit`, `GetCultivationCreditByKey`, `SumCreditsForDay` (excl. grant), `ListContributionOverCeiling`,
  `IncrementItemDailyUse` (conditional upsert `RETURNING uses`), `InsertLateJoinGrant` (`ON CONFLICT DO NOTHING RETURNING`).
- `world.sql`: `GetWorldState`, `GetWorldStateForUpdate`, `FoldWorldProgress`, `SetWorldEra`, `StartWorld`,
  `PauseWorld`, `UpsertWorldDailyTotal` (`WHERE closed_at IS NULL` on conflict), `CloseWorldDay`,
  `ListClosedDaysAfter`, `ListRecentWorldTotals`, `InsertMilestoneReached`, `ListMilestonesReached`,
  `InsertWorldEffect`, `ClaimDueWorldEffect` (`SKIP LOCKED`), `MarkWorldEffectApplied`, `ClaimExpiredSplitEffect`,
  `MarkWorldEffectFinalized`, `UpsertMilestoneOverride`, `ListMilestoneOverrides`.
- `faction.sql`: `GetCharacterFaction(ForUpdate)`, `UpsertCharacterFaction`, `SetFactionLock`, `InsertFactionHistory`,
  `GetFactionHistoryByKey`, `MigrateFactionBatch`, `SetPendingSplitBatch`, `AssignPendingSplitBatch`,
  `CountFactionMembers`, `SumFactionMerit`, `ListFactionMembersPage`.
- `vip.sql`: `InsertVIPLedger` (+ new columns), `CountVIPActivityToday`, `SumVIPEarnToday`, `GetVIPLedgerByID`.
- `shop.sql`: `IncrementShopDailyCount`, `GetShopDailyCounts`, `InsertShopPurchase`, `GetShopPurchaseByKey`.
- `breakthrough.sql`: `InsertBreakthroughAttempt` (+ tribulation columns).
- `lecture.sql`, `ritual.sql`, `moderation.sql`: CRUD with `ON CONFLICT` idempotency.

---

## 14. Evolution / permanence

- **Permanent IDs:** paths, realm IDs (incl. `vu_1…vu_15`), stage IDs, faction IDs (after `story-creator` names them),
  era/milestone/listing/activity IDs, DB enum values.
- **Stage IDs depend on `layers`**: changing a realm's `layers` after launch renames stages (`so_ky_1` → `so_ky_2`
  shifts) and needs a data migration of `stage_id`. Retune spans/entry points freely; retune layers only pre-launch.
- **Free to change (YAML):** names, entry points, odds, tribulation numbers, bucket caps, `reference_points_per_active_day`,
  milestone target days/thresholds, prices/limits, Tiên Duyên thresholds and earn amounts.
- **Additive later:** `requires_trial`, item/protector tribulation bonuses (Phase 3), Yêu path (G3), Phong Thần tier
  cap (Phase 8), reincarnation (Phase 6, 50% via an explicit negative adjustment log), new effect kinds.

---

## 15. File-by-file implementation plan

### Phase 1
| File | Change |
|---|---|
| `content/catalog/realms.yaml` | v2 (§3) |
| `content/catalog/contribution.yaml` | new (§4.1) |
| `content/catalog/vip.yaml` | v3, new gift-budget comment (§7.1) |
| `content/catalog/items.yaml` | `tu_linh_dan`, effect fields, `artifact` |
| `content/catalog/daily_tasks.yaml` | `points_milli` reward kind |
| `content/catalog/name_filter.yaml` | new |
| `services/game/internal/realm/*` | tiers/paths/stage generation/cap/auto-advance/tribulation; tests: shipped, generation sums, `Convert` round-trip, `BottleneckRoomMilli`, tribulation table |
| `services/game/internal/contribution/*` | new: loader, `Apply`, `CatchUpEligible`, `GrantMilli` (table tests) |
| `services/game/internal/cultivation/*` | milli, `rate_carry`, `CreditLocked`, auto-advance, reserve release, item use, pure projection shared with preview |
| `services/game/internal/breakthrough/*` | tribulation branch, guard, odds breakdown |
| `services/game/internal/world/` (Phase 1 subset) | day totals, day close + median, `world_state` fold (no milestones yet) |
| `services/game/internal/leaderboard/score.go` | tier-based |
| `services/game/internal/vip/*` | v3 loader, `EarnLocked`, `TopupLocked`, `RefundLocked` |
| `services/game/internal/item/item.go` | effect fields, `EffectKeys`, use limits |
| `services/game/internal/dailytask/*` | `points_milli` reward |
| `services/game/internal/character/*` | name filter; create at `First(path)` + grant |
| `services/game/internal/moderation/` | new |
| `services/game/internal/content/content.go` | new loaders + cross-checks (speed-item cap sum, realm items) |
| `services/game/migrations/000005_*` | §13 |
| `services/game/db/queries/*` | §13; `make sqlc` |
| `services/game/cmd/balancesim/main.go` | balance-doc model (ladder, auto stages, last-stage cap, tribulation, buckets, speed items, catch-up + grant, `-world`, `-late J`) |
| `services/game/cmd/worker/main.go` | day totals/close/median job |
| `services/game/internal/httpapi/*`, `packages/contracts/openapi.yaml`, `apps/web/src/lib/game/*` | new fields, `/items/use`, `/reports`, admin start/refund |

### Phase 2
| File | Change |
|---|---|
| `content/catalog/{world_milestones,factions,lectures,fortunes,shop}.yaml` | new |
| `content/story/announcements/world-milestones.yaml` | templates |
| `services/game/internal/world/*` | milestones `Next`, effects, overrides |
| `services/game/internal/faction/*` | catalogue, `CanJoin`, `DefaultBranch`, flows |
| `services/game/internal/shop/*` | catalogue + buy |
| `services/game/internal/{lecture,ritual}/` | sessions/attendance, incense, fortune |
| `services/game/internal/leaderboard/*` | faction ZSETs |
| `services/game/cmd/worker/main.go` | milestone evaluation, effects, faction reconcile |
| `services/game/migrations/000006_*`, `000007_*`; `db/queries/{world,faction,shop,lecture,ritual}.sql` | §13 |
| `packages/contracts/{openapi.yaml,ws-messages.md}`, `apps/web/...` | `/world`, `/factions*`, `/shop*`, WS milestone + lecture room |

Reviews: `db-architect` on 000005–000007; `anti-cheat-reviewer` on F1–F9; `game-balancer` owns every default number.

---

## 16. Resolved questions (Phase 0 decisions, 2026-11-06)

| # | Question | Resolution |
|---|---|---|
| Q1 | Idle rate reading | 50 points / 24 h, 8 h base window (Phase 0 #1) |
| Q2 | Bucket caps | Kept: 75 / 45 / 30, total 150 (Phase 0 #8) |
| Q3 | What counts for the world | Credited points after bottleneck, **excluding the ×2 bonus, ≤ 100/character/day**, ÷ DAU (Phase 0 #4 + bổ sung B) |
| Q4 | Catch-up reference | Server median ladder position + one-time grant of 50% median after day 14 (Phase 0 #6) |
| Q5 | Midnight attribution | A settlement counts for the game day it happens (technical choices) |
| Q6 | Tribulation loss base | Current stage's stored cultivation (Phase 0 #2) |
| Q6a | Preview vs applied loss | Preview includes the preceding settlement; attempt guard `acceptLossMilli` (§3.6) |
| Q9 | Vu rates | Same idle rate and buckets as tiên (technical choices) |
| Q10 | Path column | Derived from `stage_id` (technical choices) |
| Q11 | Faction merit window | Current era (bổ sung H) |
| Q13 | Split timings | 3-day notice, 7-day window, default = fewest members (Phase 0 #9) |
| Q14 | Milestone spacing | Strict order, no minimum gap (bổ sung H) |
| Q15 | Thresholds vs population | Per-active-player progress, a_ref = 67, thresholds = target_day × 67 (bổ sung B) |
| Q17 | Refunds | Remove exact points; level may drop (Phase 0 #10) |
| Q18 | Shop scope | Requirement items sellable, odds items never (Phase 0 #11); non-progress sinks ship with the shop (bổ sung G) |
| Q19 | Speed-item cap | 10 points/day, Tụ Linh Đan +5 × 2 (Phase 0 #5); no rate-buff items in the shop |
| Q21 | Dev reset scope | Truncate characters + Tiên Duyên data, keep accounts |
| Q22 | Outbox per credit | Worker recomputation of daily totals (technical choices) |
| O1 | Chuẩn Thánh span | 19,200 points, 4 kỳ × 16 trọng = 64 stages × 300; extension via append-only `extra_stages` (bổ sung C) |
| O2 | Tiên Duyên earn + thresholds | Earn table and 20/day cap (bổ sung D); thresholds 30 … 500,000 (bổ sung E); gifts unchanged |
| O3 | Tribulation bonuses | Surplus +0.25%/1% (max +5%), items ≤ +7%, hộ pháp +1%/member (max +3%), total ≤ +12%, final ≤ 90% except the guarantee (bổ sung F) |
| — | Minor stages | Auto-advance on credit; realm breakthroughs and tribulations stay player-initiated (bổ sung A) |
| — | Low-activity players not reaching Chuẩn Thánh in year one | Intended (Phase 0 #7) |
| — | `trung_kich` daily task | Replaced by `nghe_giang` once lectures exist (Phase 0 #13) |

## 17. Open questions

- **O4 Lecture presence on Redis loss**: if Redis loses `lecture:<session>:presence` mid-session, does a recorded
  "ngộ đạo" interaction alone earn the credit (proposed fallback), or is the session voided? Needed by Phase 2.4.
- **O5 Faction roster names and permanent IDs**: `story-creator` delivers before first deploy (Phase 0 #12); the IDs
  in §6.1 and the split branches in §5.2 are placeholders and must not ship as-is. Needed by Phase 2.2.

Neither blocks Phase 1.
