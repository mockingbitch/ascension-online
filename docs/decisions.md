# Decision log

Product/architecture decisions that refine or deviate from the GDD
([design/tien-dao-online-gdd-architecture.md](design/tien-dao-online-gdd-architecture.md)). Newest first.

## 2026-11-06 — Phase 0 (bổ sung): số liệu còn lại
Source: [design/balance-the-gioi-chung.md](design/balance-the-gioi-chung.md) §11. Owner accepted every proposal.
- **A. Minor stages auto-advance** inside the single credit path once their requirement is met (they were 100%
  anyway). **Realm (major) breakthroughs and tribulations stay player-initiated.** Refines CLAUDE.md "người chơi tự
  kích hoạt đột phá" to mean realm-level breakthroughs.
- **B. Amends decision #4 below:** world progress per game day = Σ min(100, credited points **excluding the ×2
  late-joiner bonus**) ÷ DAU; reference a_ref = 67 points/active player/day; each milestone threshold =
  target_day × 67 (Giảng đạo 2: 2,010 · 3: 4,020 · Vu Yêu: 6,700 · Nữ Oa: 8,710 · Thánh nhân: 9,380 · Nhân tộc: 10,050 ·
  Phong Thần: 16,080 · Thiên Đình: 20,770 · Tây Du: 22,110). 70%/150% clamp kept as safety net. Re-measure a_ref in beta.
- **C. Chuẩn Thánh** spans 19,200 points (32,450 → 51,650) as 4 kỳ × 16 trọng = 64 stages of 300. Stage counts:
  Luyện Khí 9 · Trúc Cơ 4 · Kim Đan 4 · Nguyên Anh…Hợp Thể 8 · Đại Thừa…Kim Tiên 12 · Thái Ất, Đại La 16 · Chuẩn
  Thánh 64 (205 total). Stage IDs are append-only.
- **D. Điểm duyên in game** ≤ 20/day across non-top-up sources: online cultivation 1×3 (settles ≥1h apart),
  lecture 3×2, daily task claim 1×4, hộ pháp 2×2, song tu 2×1, faction merit 1×3, trial 2×2, event 3×1.
- **E. Tiên Duyên thresholds:** 30 / 150 / 500 / 1,200 / 2,500 / 4,500 / 10,000 / 25,000 / 50,000 / 100,000 /
  250,000 / 500,000 (levels 1–12). Daily gifts unchanged.
- **F. Tribulation bonuses:** surplus +0.25% per 1% stored above requirement (max +5%); items max +7%; hộ pháp +1% per
  online fellow member (max +3%); all bonuses max +12%; final odds ≤ 90% except the guaranteed attempt after 3 fails.
- **G. Spirit-stone sinks** that do not speed progress (cosmetics, cave decoration, breakthrough-requirement items)
  ship with the shop, before bí cảnh adds more stones.
- **H.** Faction merit counts within the current era; milestones fire strictly in order with no minimum gap.

## 2026-11-06 — Phase 0: số liệu & mô hình dữ liệu thế giới chung
Sources: [design/balance-the-gioi-chung.md](design/balance-the-gioi-chung.md),
[design/data-model-the-gioi-chung.md](design/data-model-the-gioi-chung.md). Owner accepted every proposal.
1. **Idle rate:** 50 points per 24h, 8h base accumulation window (free player needs 3 logins/day for the full 50;
   Tiên Duyên's longer window only helps 1–2-login players and never beats free at 3 logins).
2. **Tribulation loss:** 25% of the **current stage's** stored cultivation.
3. **Ladder:** cumulative entry points per realm from the balance doc (Trúc Cơ 300 · Kim Đan 950 · Nguyên Anh 2,050 ·
   Hóa Thần 3,500 · Luyện Hư 5,400 · Hợp Thể 7,500 · Đại Thừa 9,750 · Độ Kiếp 12,250 · Nhân Tiên 14,900 ·
   Địa Tiên 17,750 · Kim Tiên 20,900 · Thái Ất 24,350 · Đại La 28,200 · Chuẩn Thánh 32,450), replacing
   `cot-truyen.md` §6.4. Luyện Khí 9 tầng; later realms 4 kỳ × k trọng. The 120% bottleneck applies only on a
   realm's **last** stage. Chuẩn Thánh span: to be proposed by balance.
4. **Story milestones** advance on **average progress per daily-active player** (Σ credited points ÷ DAU, per
   day), not raw server total; the 70%/150% target-day clamp stays as a safety net. Replaces `cot-truyen.md` §6.1.
5. **Speed items:** shop speed items capped at ~10 points/day per character (e.g. Tụ Linh Đan +5, max 2/day).
   Rule kept: Tiên Duyên 12 at 1 login/day ≤ free at 3 logins/day. Fix the gift-budget comment in `vip.yaml`.
6. **Late joiners:** ×2 before the 150 ceiling **plus** a one-time grant of 50% of the server median for characters
   created after day 14; the grant counts toward neither world progress nor daily rankings.
7. Low-activity players not reaching Chuẩn Thánh in year one is **intended**.
8. **Bucket caps** kept: idle 75 · lectures/tasks 45 · community 30, total 150/day.
9. **Faction split:** 3-day notice, 7-day choice window, default = branch with the fewest members.
10. **Refunds** remove the exact điểm duyên credited; a Tiên Duyên level may drop.
11. **Shop** may sell breakthrough-*requirement* items (Trúc Cơ Đan, Kết Đan Quả…); never odds items.
12. Faction roster + permanent IDs: `story-creator` names them before first deploy.
13. Daily task `trung_kich` is replaced by "attend a lecture" once lectures exist.

Technical choices (data-model open questions): world totals count **credited** points (after bottleneck);
a settlement counts for the game day it happens; path is **derived from `stage_id`** (no `characters.path`);
the worker **recomputes** daily totals instead of an outbox event per credit; Vu uses the same idle rate and
buckets as tiên; the tribulation preview must include the settlement that precedes the attempt so the shown loss
matches the applied loss.

## 2026-11-06 — Tiên Duyên (thay VIP), quà ngày, linh thạch
Refines D1/D7 of the entry below.
- **Name.** "VIP" is renamed **Tiên Duyên** in all player-facing text ("Tiên Duyên cấp 5"); its points are
  **điểm duyên**. Levels 0–12, permanent. Code/DB identifiers (`vip`, `vip_points`) may stay to avoid a risky
  migration; only display strings change.
- **Sources of điểm duyên (D1b).** In-game: online cultivation activities (giảng đạo, tu luyện online), player
  interaction (hộ pháp, song tu, thế lực), system interaction (nhiệm vụ ngày, mini game, sự kiện). Top-up:
  **1.000đ = 10 điểm duyên** (was 1.000đ = 1 point). Level thresholds must be recalculated so free players can climb
  through activity; amounts per activity go in YAML and are tuned with `make balance`.
- **Daily gift (D1a) — kept.** Higher Tiên Duyên levels keep receiving more spirit stones per day. Accepted
  consequence: because spirit stones buy items that speed cultivation, top-up indirectly helps progress. Remaining
  guards: (1) nobody exceeds **150 points/day**; (2) items that raise **breakthrough/tribulation odds are never sold
  for spirit stones or money** (`cot-truyen.md` §5.2 still applies) — they only drop from bí cảnh; (3) no direct
  rate multiplier or ranking points from Tiên Duyên.
- **Spirit-stone sink (D7).** Spirit stones are spent **buying items** from an in-game shop (catalogue-driven
  prices). This closes the "no sink" gap noted in the daily-tasks entry once the shop ships. Player-to-player
  trading rules: still open (proposal in the plan, needed by Phase 5).
- **G2/G3** must be decided by ~day 30 after open launch (Phase 6 deadline day 60).

## 2026-11-06 — Cốt truyện thế giới chung (từ `design/cot-truyen.md`)
Plan: [design/ke-hoach-trien-khai-hong-hoang.md](design/ke-hoach-trien-khai-hong-hoang.md). Supersedes the
"Progression pacing: 180 days" entry below and amends the VIP entry.

- **Monetisation (D1) — kept, reshaped** (see Tiên Duyên entry above). The paid channel stays. VIP points become earnable in game
  *and* by top-up, top-up just earns more. Levels 0–12, permanent. The VIP hard line is unchanged: no
  cultivation-rate multiplier, no breakthrough/tribulation odds, no ranking points, no paid odds boosts. Offline-cap
  extension stays a convenience: a top-tier player at 1 login/day must never out-earn a free player at 3 logins/day,
  and nobody exceeds the 150 points/day ceiling. `cot-truyen.md` §3, §5.5, §9.3 ("không thu phí") are overridden.
- **World & name (D2).** Product stays **Tiên Đạo Online**; the world stays **Thiên Nguyên Giới**. The story arc
  and mechanics of `cot-truyen.md` are kept. Characters use a **mixed** rule:
  - Keep figures from mythology and public-domain classics (*Phong Thần Diễn Nghĩa*, *Tây Du Ký*, Sơn Hải Kinh…),
    e.g. Bàn Cổ, Nữ Oa, Hồng Quân, the Phong Thần list/eras, Tây Du as an era.
  - Rename what modern web novels invented or fixed (e.g. Vu Yêu đại chiến framing, Tổ Vu roster, Hậu Thổ founding
    luân hồi, Đông Vương Công's Tiên Đình vs Yêu tộc Thiên Đình) into original Thiên Nguyên Giới equivalents.
  - Avoid **actively worshipped religious figures** as playable factions, quest givers or anything players fight or
    mock (Tam Thanh/Lão Tử, Chuẩn Đề, Tiếp Dẫn, Phật, Quan Âm…): use original stand-ins. Per-figure list to be
    written in `content/story/lore/` and reviewed before release.
- **Tribulation (D3).** From the breakthrough that *leaves* Hóa Thần (→ Luyện Hư) onward: base 80%, −3% per
  realm, floor 50%. Failure loses 25% of the cultivation currently stored (never drops realm/stage) and starts a
  24h cooldown; after 3 consecutive failures the next attempt succeeds. Loss shown before the attempt.
  Supersedes "failure keeps all cultivation" for these realms; minor breakthroughs keep the old policy.
- **Realm ladder (D4).** 15 realms Luyện Khí → Chuẩn Thánh (Thánh Nhân reserved for events/NPCs) with stages
  kept inside each realm, plus a parallel **Vu** path of equal tiers. Path conversion keeps points.
- **Pacing (D5).** ~100 points/day average (50 idle · 30 lectures/tasks · 20 community), hard ceiling
  **150 points/day/character** (late-joiner ×2 applies before the ceiling), Chuẩn Thánh at ~day 330. Stored as
  `*_milli`. Server-wide story milestones are driven by the sum of these points, clamped to 70%–150% of target day.
- **Scope (D6) — mini games are in.** Tâm ma (Tetris-like), hóa phàm (slicing), bí cảnh (survivor-like), caro PvP.
  Overrides GDD §12 "out of MVP" for these four only. Results are server-issued sessions (seed) and server-verified;
  rewards are gates or capped. Plain `<canvas>` 2D is allowed inside `apps/web/src/lib/minigames/` only; still no
  game engine (PixiJS/Phaser).
- **Phong Thần (G1).** Ứng Kiếp is opt-in. Only registered players are bound by the 15-game rule.
- **Reincarnation (G4).** Luân hồi keeps **50%** of cultivation.
- Deferred: Đại chiến Vu Yêu gameplay (G2), Yêu tộc path (G3). Still open: G5–G8 — proposals in the plan.

## 2026-11-06 — Game UI round 1 (art brief "Tu Tiên Hồng Hoang Online", reconciled)
- The brief's conflicting asks were decided by the product owner: **real API where it exists + clearly labelled
  previews elsewhere** (no client-side rewards/odds); **keep the jade/gold palette**; **original faction names**
  (D2 kept: Huyền Khung Môn, Kim Sa Môn, Thương Lan Tiên Đình, Tinh Hà Yêu Đình, Vu Tộc — drafts in
  `content/story/factions/`); Google-only sign-in and one shared world stay (no register form / server picker).
- `/play` is a persistent shell layout (HUD with tu vi + notifications, world chat, 6-item dock) and screens are
  nested routes: Thế Giới (world map, home), Tu Luyện, Nhân Vật, Túi Đồ, Thế Lực, Xếp Hạng, Đột Phá, Thiên Đạo Sử,
  Đạo Lữ; unbuilt places (Bí Cảnh, Đại Chiến) render an in-world "sealed" screen, never a dead link.
- Preview screens read `apps/web/src/lib/mock/*` and show the "Bản xem trước" ribbon; the preview faction choice
  lives only in localStorage. No invented combat stats (no combat system). Mini games, Vu body-refining UI and the
  trial realm are the next rounds (server-issued sessions per D6).

## 2026-11-06 — One-page game shell + world chat
- **Web = one game screen** (`/play`): fixed viewport, no page scroll; HUD (identity, resources, Tiên Duyên),
  cultivation scene, dock. Features open as **windows over the scene** via SvelteKit shallow routing (Back closes
  them). Shared state in `apps/web/src/lib/game/store.svelte.ts` (still server-authoritative: responses replace
  state). Out-of-game pages (landing, login, policy, public Thiên Bảng/Tiên Duyên, admin) live in `routes/(site)`;
  signed-in visitors of `/` go straight to `/play`; `/cultivate` redirects.
- **World chat** (`content/catalog/chat.yaml`): one channel "Thế Giới"; faction/sect channels with Phase 2.
  Send = REST (`POST /chat/{channel}/messages`: auth, idempotency, Redis rate limit 1/3s burst 5, duplicate-text
  window 30s) → Postgres `chat_messages` → Redis pub/sub → WS `chat.message` (socket stays receive-only and
  unauthenticated). At-most-once; clients backfill from REST on every (re)connect.
- Posting needs **Luyện Khí tầng 3** (anti-throwaway-account spam); reading is open. Profanity is **masked with
  `***`** (moderation.yaml `profanity` category, same matcher as names); the unmasked text is kept in `raw_body`
  only for moderators. Retention **90 days** (worker sweep) — ⚠️ legal to confirm.
- Moderation: reports can attach a chat message (evidence snapshot survives hiding/retention); admins hide messages
  (`chat.hidden` broadcast) and mute accounts up to 30 days. All audited.

## 2026-11-06 — Daily tasks, Tụ Linh Phù, VIP daily gift
- Game day = 00:00–24:00 Vietnam time (UTC+7, `internal/gameday`) for every daily reset.
- Daily tasks in `content/catalog/daily_tasks.yaml`; progress recorded inside the gameplay transaction that
  caused it (settle, breakthrough attempt). "Vận công" counts only settlements ≥ 1h apart (no spam-to-complete).
- **Tụ Linh Phù** comes from the *check-in* task so once-a-day players get it. Using it stores a pending +4h
  bonus (1 per game day, no stacking); it is spent only by the first settlement it actually extends
  (elapsed > VIP cap) and never lifts the window past the 24h daily ceiling.
- VIP daily gift: `POST /vip/daily-gift`, once per game day (DB primary key), amount by tier at claim time.
- ⚠️ Spirit stones now have sources (tasks ~450/day, VIP gift ≤ 300/day) but **no sink yet**. Breach of the
  "sinks before sources" rule, accepted temporarily; the first spending feature (alchemy / cave upgrades) must land
  before balance is final.

## 2026-11-06 — Realtime and shared rate limits
- World announcements over `GET /ws` (read-only). Worker publishes to Redis pub/sub; every API instance fans out
  to its sockets. At-most-once; clients de-dupe by id. Text from `content/story/announcements/*.yaml`.
- Rate limits use Redis GCRA (`redis_rate`, keys `rate:ratelimit:<name>:<key>`) shared by all API instances, with
  an in-process fallback if Redis errors (degrades to per-instance limits, never to "no limit").

## 2026-11-06 — Progression pacing: 180 days to Độ Kiếp
- **Target:** a free player logging in 3×/day (5 min each) reaches Độ Kiếp at ~day 180 (median).
- Realm durations follow a ~1.55× geometric growth so every realm takes longer than the previous one.
- Tuned via `first_required` in `content/catalog/realms.yaml`; verified with `make balance`.

| Median day reached (3×/day, free) | Trúc Cơ | Kim Đan | Nguyên Anh | Hóa Thần | Luyện Hư | Hợp Thể | Đại Thừa | **Độ Kiếp** |
|---|---|---|---|---|---|---|---|---|
| before | 4 | 5 | 7 | 10 | 18 | 35 | 80 | 207 |
| after  | 4 | 9 | 17 | 28 | 46 | 73 | 115 | **181** |

Known limitation: low-frequency players are stage-limited early (≈1 stage per login because of the 120%
bottleneck), e.g. ~11 days to Trúc Cơ at 1 login/day. Revisit if early retention for casual players suffers.

## 2026-11-06 — VIP: 12 permanent levels from cumulative points
- Deviation: GDD sketched 3 VIP tiers as time-boxed entitlements. Now **VIP 0–12**, level = f(cumulative VIP points),
  never expires. Thresholds/benefits in `content/catalog/vip.yaml`. Price per point is a placeholder (1.000đ).
- Benefits: offline cap 8h → 32h (+2h per level), daily spirit-stone gift, cosmetics (titles, frames, chat aura,
  cave decoration, breakthrough FX), convenience (alchemy slots, garden auto-harvest).
- **Hard line kept:** no cultivation-rate multiplier, no breakthrough odds, no ranking points. The loader rejects
  any other perk key. Daily ceiling: nobody banks > 24h of cultivation per day.
- Fairness target replaces "gap ≤ 25%": **VIP max at 1 login/day ≈ free at 3 logins/day** (measured: 200 vs 181
  days to Độ Kiếp). VIP lowers how often you must log in; it does not raise the ceiling.
- Free at 1 login/day is ~2.7× slower than VIP 8+ at 1 login/day. Mitigation: **Tụ Linh Phù** (`tu_linh_phu`, +4h
  offline window for one gap per day, never beyond 24h) from daily tasks → gap ~1.8×. (Daily-task system pending.)
- `vip_entitlements` (time-boxed) is kept for a future monthly card.

## 2026-11-06 — Google-only sign-in
- Deviation: GDD listed register/login. Accounts are created only via **Google OIDC** (Authorization Code + PKCE,
  ID-token verification incl. nonce). No passwords stored (`users.password_hash` nullable, unused).
- Identity key = `(provider, subject)`; email is never used to auto-link accounts (takeover risk) — a new Google
  subject with an existing email is refused (`email_taken`).
- Sessions: access JWT (HS256, 15 min) kept in memory by the SPA; refresh token (opaque, sha256 in DB) in an
  `HttpOnly; SameSite=Strict; Path=/api/v1/auth` cookie, rotated on every use; reuse of a rotated token revokes the
  whole family. `/auth/refresh` also requires `X-Requested-With: tiendao` (CSRF defence in depth).
- Web app runs in SPA mode (`ssr = false`) because the access token lives only in browser memory.

## 2026-11-06 — Breakthrough outcome policy
- Items are consumed on **success and failure**; failure keeps all cultivation, increments the pity streak
  (+`pity_step_bp`, capped by `pity_max_bonus_bp`) and starts the stage cooldown.
- Success carries cultivation above the requirement into the next stage (bounded to 20% by the overflow cap).
- Every attempt persists `success_bp` and the server roll (`roll_bp`) for audit.

## 2026-11-06 — Leaderboard availability
- Redis ZSET is the read path; Postgres is the truth. The worker re-reads progression rows for `leaderboard.dirty`
  events (never trusts event payload scores) and fully rebuilds every 10 minutes and at start.
- If Redis is down the endpoint returns 503 instead of a slow Postgres fallback (catalogue order isn't stored in
  SQL). Ties are re-ordered by `stage_reached_at` within a page only.

## 2026-11-06 — Stack adjustments vs. tech-stack doc
- GDD wins over `tien-dao-online-tech-stack.md`: no PixiJS in MVP (DOM/CSS/SVG), WebSocket only for live features,
  JSON before Protobuf.
- Nginx overwrites `X-Forwarded-For` with `$remote_addr`; behind Cloudflare, enable `real_ip` with Cloudflare ranges.
