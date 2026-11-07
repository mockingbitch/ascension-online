# Decision log

Product/architecture decisions that refine or deviate from the GDD
([design/tien-dao-online-gdd-architecture.md](design/tien-dao-online-gdd-architecture.md)). Newest first.

## 2026-11-07 — Chủ dự án: tăng tốc đầu game bằng cách dồn điểm về sau
- **What.** `realms.yaml`: Luyện Khí 300 → **100** points, split by new `stage_weights: [1…9]` (tầng 1–9 =
  2/4/6/8/11/14/16/18/21); Trúc Cơ 650 → **550** (entry 100); Kim Đan 1,100 → **1,400** (entry 650). Nguyên Anh
  entry (2,050) and every later tier unchanged → the ~330-day arc is untouched. Realm lengths still strictly grow
  (100 < 550 < 1,400 < 1,450).
- **Why.** At ~2 points/hour idle, the old 33-point tầng 1 meant a new player saw 1–2 stages in the first session;
  now the first hour already passes tầng 1 and the first day passes most of Luyện Khí.
- **Mechanism.** Optional `stage_weights` on a tier (one per stage, ≥ 1, non-decreasing; every stage ≥ 1 point);
  absent = the old even split. Validated in `realm.Load`.
- **`make balance` (300 runs/profile), median day to Trúc Cơ / Kim Đan / Nguyên Anh, before → after:** S3 3/10/21 →
  **1/7/21**; A3 4/12/26 → **1/8/26**; I1 13/40/84 → **5/28/84**; G 2/8/17 → **16 h/6/17**. Chuẩn Thánh unchanged
  (S3 326, A3 406, I1 1313). Fairness checks OK; retry-gap accepted exception leads 1.0/2.3 → **1.0/2.0** days (≤ 3).
- **Chat gates.** "Luận Đạo" (`tu_tieu_cung`) posting lowered to `luyen_khi.tang_1` (every character); world chat
  keeps `luyen_khi.tang_3` — which now takes 6 points (~3 h idle) instead of 66, a weaker anti-spam gate.
- Not launched yet: stage IDs are unchanged, no migration. Existing dev/test characters in Luyện Khí–Kim Đan may
  hold cultivation above the new requirements — reset those databases rather than rely on auto-advance.

## 2026-11-07 — Chủ dự án đảo ngược D2: dùng tên theo cot-truyen.md
Supersedes the "World & name (D2)" bullet of "Cốt truyện thế giới chung" (2026-11-06), the "original faction names"
note of "Game UI round 1" and every later entry that names the old stand-ins (kept below as history).
- **Rule.** Every in-game world/canon name — figures, factions, places, eras, milestones — is used exactly as in
  [design/cot-truyen.md](design/cot-truyen.md). World = **Hồng Hoang** ("Hồng Hoang giới" only where grammar needs it).
  Where the doc is silent, only classic mythology / public-domain classics, flagged for owner review (list below).
- **Product name kept: "Tiên Đạo Online"** (page titles, logo text, splash, login, domain, `CLAUDE.md`). Only in-game
  names changed.
- **Slugs kept** (permanent IDs; no migration, no data change): `huyen_khung_mon`, `kim_sa_mon`, `thuong_lan_dinh`,
  `tinh_ha_yeu_dinh`, `vu_toc`, `nhan_toc`, `thu_nhat_vien`, `lap_cuc_vien`, `van_luu_vien`, `kim_sa_giao`, every era /
  milestone / realm id. Only `name`, `motto`, `pitch` and lore text changed. The slug → name table is the header of
  `content/catalog/factions.yaml`. All `name_placeholder` flags on factions and Vu realms are removed (the Vu path
  name "Vu Đạo" keeps its flag: the doc does not name the path).

| Slug | Was | Now (cot-truyen) | Head (NPC) |
|---|---|---|---|
| `huyen_khung_mon` | Huyền Khung Môn | **Tam Thanh** | Thái Thanh Lão Tử, Nguyên Thủy Thiên Tôn, Thông Thiên Giáo Chủ |
| `kim_sa_mon` | Kim Sa Môn | **Tây Phương** (phe Chuẩn Đề) | Chuẩn Đề, Tiếp Dẫn |
| `thuong_lan_dinh` | Thương Lan Tiên Đình | **Tiên Đình** | Đông Vương Công |
| `tinh_ha_yeu_dinh` | Tinh Hà Yêu Đình | **Thiên Đình Yêu Tộc** | Đế Tuấn, Thái Nhất |
| `vu_toc` | Vu Tộc | Vu Tộc (unchanged) | Tổ Vu |
| `nhan_toc` | Nhân Tộc | Nhân Tộc (unchanged) | Tam Hoàng Ngũ Đế |
| `thu_nhat_vien` | Thủ Nhất Viện (tĩnh tu) | **Nhân Giáo** | Thái Thanh Lão Tử |
| `lap_cuc_vien` | Lập Cực Viện (quy củ, thứ bậc) | **Xiển Giáo** | Nguyên Thủy Thiên Tôn |
| `van_luu_vien` | Vạn Lưu Viện (mở cho mọi người) | **Tiệt Giáo** (doc spelling; "Triệt Giáo" also blocked as a name) | Thông Thiên Giáo Chủ |
| `kim_sa_giao` | Kim Sa Giáo | **Tây Phương Giáo** | Chuẩn Đề, Tiếp Dẫn |

  Branch mapping follows each branch's identity: quiet/long seclusion → Nhân Giáo (Thái Thanh, thanh tĩnh vô vi);
  rules/teaching by rank → Xiển Giáo; open to all → Tiệt Giáo ("hữu giáo vô loại").
- **Eras / milestones.** `dai_chien` Đại chiến Huyết Tinh → **Đại chiến Vu Yêu** (era + milestone); `luan_hoi` Luân hồi
  sơ chuyển → **Hậu Thổ hóa luân hồi** (Địa Phủ khai mở in text); `thanh_nhan` Thánh vị định tọa → **Thánh nhân xuất
  thế**; `nhan_toc` Nhân tộc hưng khởi → **Tam Hoàng Ngũ Đế**; `phong_than` → **Phong Thần Lượng Kiếp**. Realm `vu_15`
  Vu Cực → **Tổ Vu**. Opening scenes renamed to §10 (Bàn Cổ vẫn lạc, Long Hán Sơ Kiếp, Các đại năng xuất hiện, Tử Tiêu
  Cung mở) and scene 6 names the đại năng. Faction split UI copy says "nhánh" instead of "viện".
- **Moderation.** `moderation.yaml` (category `story`) now also blocks Thái Thanh Lão Tử, Thái Thượng, Lão Quân,
  Nguyên Thủy, Thông Thiên, Tiếp Dẫn Đạo Nhân, Đông Hoa Đế Quân, Côn Bằng, the classic Tổ Vu roster, Tam Hoàng Ngũ Đế
  names, Hạo Thiên, Dao Trì, Khương Tử Nha, Nhiên Đăng and the faction names (Tiên Đình, Thiên Đình, Địa Phủ, Nhân
  Giáo, Xiển/Tiệt/Triệt Giáo, Tây Phương Giáo), on top of the existing Hồng Quân / Tam Thanh / Chuẩn Đề / Nữ Oa… rules.
- **Chosen where the doc is silent (⚠ owner review):** display name "Tây Phương" for the Chuẩn Đề faction (doc calls it
  "Chuẩn Đề (phe Tây phương)"); "Thiên Đình Yêu Tộc" for "Thiên Đình của Yêu tộc"; Đế Tuấn + Thái Nhất as its heads;
  Côn Lôn as Tam Thanh's seat; biển Đông / đảo tiên for Tiên Đình; Bất Chu Sơn in lore for Vu Tộc; era name "Hậu Thổ
  hóa luân hồi"; no individual Tổ Vu named in content (only blocked as player names); Tiên Đình's fate after the war
  ("cầu sương đứt") is project prose.
- **Risk.** Worshipped figures (Tam Thanh / Lão Tử, Chuẩn Đề, Tiếp Dẫn) are now playable factions and faction heads.
  Content keeps a respectful tone (no jokes, no fighting them, no prayers or scripture; emblems use generic motifs),
  but **legal and religious review is still recommended before public launch** (`cot-truyen.md` §8).

## 2026-11-07 — Trao đổi vật phẩm (Phase 5.2)
Implements the owner's "Luật trao đổi" (entry "Chủ dự án chốt trước Phase 4–5" below), plan §3.12, checklist §5.2.
Migration `000018_trades`; catalogue `content/catalog/trade.yaml`; `internal/trade` (loader + pure rules + service).
**Every number below is a placeholder for closed-beta tuning (⚠ owner sign-off), in particular the fee.**
- **What moves.** Only `trade.yaml` assets: linh thạch and plain materials (today Linh Thảo, value 5). Refused by the
  loader/content cross-check AND again at create and accept: pháp bảo, breakthrough requirement items, anything
  `shop.yaml` sells, cosmetics, items with effect/odds fields, premium currency, anything `tradable: false`. Every
  `tradable: true` item must have a trade.yaml value, so the inventory's "Không thể trao đổi" lock equals the server rule.
- **Model.** A *gift* (`give` only) or a *swap* (`give` for the receiver's `take`; the same asset can't be on both sides).
  The receiver must accept — with an explicit "không hoàn tác" acknowledgement (`acknowledgeIrreversible`). Nothing is
  escrowed: at accept everything is re-verified (catalogue, gates, relation, caps, balances) and both sides move in ONE
  transaction through `currency_ledger` / `inventory_transactions` (reasons `trade_send` / `trade_receive` / `trade_fee`,
  ref `trade:<offer>`); a failure moves nothing and the offer stays pending. Lock order: offer → both `trade_days` →
  both stone accounts → both inventories, each in character_id order (data-model §2.3). Status machine
  `pending → accepted | declined | cancelled | expired`, terminal rows frozen by a trigger. Idempotency keys on
  create/accept/decline/cancel (a create key reused with other lines → 409). Offers expire after **24 h** (read-time
  filter, lazily on touch, and the worker sweep ≤ 10 × 500 per reconcile tick, SKIP LOCKED).
- **Caps (per character, game day).** **5** offers created, ≤ **5** open, **5** completed trades (either side), sent value
  ≤ **1,500**, received value ≤ **300** from a stranger / **600** same faction / **1,000** đạo lữ (relation checked at
  accept; one running total, so stacking relations never exceeds 1,000). Creating an offer whose value already exceeds
  the relation cap is refused up front.
- **Relationship default: any player may trade**, with stricter caps between strangers (where real-money deals happen).
- **Gates.** Receive: account ≥ **7 days**, character ≥ **3 days**, ≥ **Trúc Cơ** (tier 2). Send: account ≥ **3 days**,
  character ≥ **1 day**. In a swap both parties pass both gates.
- **Fee (sink).** **5%** of the linh thạch *given* (ceil, min 1), paid on top by the giver and destroyed; items pay none.
  ⚠ Owner sign-off: it is small next to existing sinks (≤ 75/day per giver at the sent cap) and mostly exists to tax
  stone funnels.
- **Anti-RMT.** netsig `SharedRecently` at create → `net_flag` + `trade_events` row + warn log (never blocks). `GET
  /admin/trades/flows?days=` (default 7): top net receivers, lopsided pairs (≥ 600 value, ≥ 90% one way), many-to-one
  funnels (≥ 3 senders), flagged offers; audited as `trade.flows.read`. A party can report a trade (`POST /reports`
  reason `trade_abuse` + `tradeOfferId`; the offer is the evidence). Rate limit `trade` 1 per 3 s, burst 6. Create and
  accept count as activity (`markActive`); declines, cancels and replays don't.
- **Balance (`make balance`, new "Trao đổi" section; all earlier sections byte-identical).** Trades create nothing:
  2,000 random trades change world stones by exactly −Σ fees. Fee at the caps: 15 / 30 / 50 / 75 stones per day. Points:
  received stones only buy speed items, so a receiver gains at most the speed cap (I1$ TD0: 29 → 35 points/day; every
  profile already at the speed cap: +0); speed cap 10, buckets and the 150 ceiling unchanged. Requirement-item
  breakthroughs (report only): free idle@1 fed 300 or 1,000/day reaches Trúc Cơ day 9 / Kim Đan day 29 (vs 14 / 44 without,
  free@3 = 6 / 19) — the shop's 3-a-day limit binds, so gifts never beat an active free player.
- **Web.** Túi Đồ › Trao đổi (inbox, confirm screen with both sides + fees, compose from a player card's "Trao đổi" —
  chat names, Thiên Bảng, faction members — history with "Báo cáo"); lock badge on untradable items.
- **Anti-cheat review** (no Critical/High/Medium). Fixed: create key reuse with other lines now 409; a same-key create racing
  the game-day boundary replays instead of 500. Open (Low): relation/gates are read before the row locks (a faction switch
  committing mid-accept could apply the faction cap once — the 7-day switch cooldown makes it negligible);
  `trade_offers` cascades on character delete (no delete path exists; decide before adding one — RESTRICT or soft
  delete); `GET /trades/peer/{id}` shows whether any character can receive (age/tier blockers) — accepted for the
  composer, could be reduced to a boolean.
- **Open:** a second tradable material (alchemy) needs a value in trade.yaml; WS push of incoming offers (inbox loads on
  open); admin UI for flows (API only); automatic alerting (Prometheus) on flows — today a pull report.

## 2026-11-07 — Đạo lữ song tu + băm IP chống đa tài khoản (Phase 5.1)
Implements cot-truyen §5.6, plan §3.11, checklist §5.1 and the owner's "Chống đa tài khoản" decision below. Migration
`000017_dao_lu_net_signals`; catalogue `content/catalog/dao_lu.yaml` (`internal/daolu` = loader + pure rules,
`internal/daolu/bond` = runtime, split so `content` never imports `cultivation`). **Every number below is a placeholder
for closed-beta tuning (⚠ owner sign-off).**
- **Network signal (`internal/netsig`, reusable for 5.2 trades / hộ pháp).** At Google login and session refresh the API
  stores HMAC-SHA256(salt, client IP) truncated to 16 bytes per (user, salt key) with first/last seen — never the raw IP.
  Client IP = the existing `clientIP` (`TRUST_PROXY` → rightmost X-Forwarded-For). IPv4 hashed whole, IPv6 by /64 (a
  household shares a /64 while devices rotate the interface id). Salt from `NETSIG_SALTS="id:secret[,id:secret]"`
  (secret ≥ 32 B; first = current; required outside dev, dev falls back to a fixed non-secret salt). Rotation: list the
  new key first and keep the old one; every login is hashed under all listed keys and only same-key hashes compare;
  drop the old key after 30 days. Worker deletes rows unseen 30 days (≤ 10 × 1,000 per reconcile tick). API:
  `Store.SharedRecently(a, b)` (30-day window), `Store.Clusters` → `GET /admin/netsig/clusters?days=&min=` (accounts +
  character, no hash, no IP). Only ever a warning to admins; nothing auto-blocks. `/chinh-sach-du-lieu` updated.
- **Bond.** Propose from a player card (`POST /dao-lu/proposals`, proposer acknowledges "vĩnh viễn"), target accepts with
  an explicit second acknowledgement (typed name in the UI; `acknowledgePermanent` required by the server) or declines;
  proposer may cancel. Gates for BOTH sides at propose and again at accept: account (Google user) ≥ **14 days**, character
  ≥ **tier 2** (Trúc Cơ, any path), not bonded, not in the **7-day** rebond cooldown. Proposal expiry **72 h**, **3**
  proposals/game day, one open outgoing proposal; dedicated rate limit (1 per 3 s, burst 6). One active bond per
  character = partial unique index on `dao_lu_members(character_id) WHERE active` (two rows per bond — a single index on
  `dao_lu_bonds` cannot cover both columns). Accept locks every pending proposal of both characters in id order, then
  bonds and supersedes the rest (two concurrent accepts sharing a character serialize instead of deadlocking; a race
  past the lock fails cleanly with `already_bonded`). History in `dao_lu_events` (+ `admin_audit_logs` for admin ends).
- **No voluntary divorce.** cot-truyen §5.6 says "đã kết đạo lữ thì không thể bỏ nhau"; endings are only: lượng kiếp
  (`bond.EndForCharacterLocked(ctx, q, char, luan_hoi|phong_than, now)` — hook for G4 Phase 6 / Phong Thần Phase 8, call
  it inside their transaction), admin after a report by or about a partner (`POST /admin/dao-lu/bonds/{id}/terminate`,
  note required, audited), and **90 days** without play by either side (worker; "play" = latest of settlement time,
  session refresh, character creation; ≤ 100 bonds per tick). Every ending starts the cooldown for both.
- **Song tu (no ticker).** Either partner starts today's session while BOTH are *active* (3.4 rule: a successful
  authenticated mutation within `activity_window_seconds` 600; the song tu heartbeat — like the ping — does NOT refresh
  activity — see H1 below). Heartbeat every **30 s** while the panel is open; Redis Lua on the server clock adds the gap since the last
  accrual (≤ max_gap **75 s**) only when both are active and both beat within 75 s, capped at **7,200 s/game day**
  (shared overlap, one HASH per bond and day, TTL 26 h, safe to lose). Each partner claims separately (`POST
  /dao-lu/song-tu/claim`, Idempotency-Key): Redis take → tx (progression lock → Σ paid today ≤ window → `CreditLocked`
  source `dual_cultivation`, community bucket, 150 ceiling → claim row) → seconds restored if the tx failed. Rate
  **5 points/h → 10 points** for the full window; pay is cumulative (`pay(after) − pay(before)`, splitting never
  gains/loses a milli). Unclaimed overlap is lost at the game-day reset (UI says so). `vip.yaml dual_cultivation`
  (decision D 2 × 1) is now **live**: the claim that brings the day's paid seconds to ≥ **1,800 s** earns it.
- **Balance (`make balance`).** Song tu is modelled for the S profiles as one full window/day (10 points, community) and
  the generic community assumption drops 10 → 5 points/login so S2/S3 keep the same 20/day: **S3 100.8 pts/day, Chuẩn
  Thánh day 326 (unchanged)**; S2 unchanged (356); S1 55.4 → 60.4 (594 → 545: a 1-login player who also does the 2 h
  song tu). Fairness (TD_L@1 ≤ free@3), gift gap, ceiling (max day 125 ≤ 150) and retry-gap checks unchanged/OK.
  Điểm duyên/day S1–S3 +2 (S3 13 → 15, level 4 by day 90). A full window always fits the community bucket (loader check).
- **Anti-cheat review fixes (H1, H2, M1, M2, L1–L7).**
  - **H1 — "active" is an explicit allowlist** (replaces "any 2xx POST/PUT except the ping", which a heartbeat or a
    repeated cheap POST could sustain). `httpapi.markActive` is called only after a non-replayed success of: settle that
    credited > 0, a paid ngộ đạo, a judged trial / bí cảnh submit, a breakthrough attempt, a shop / cosmetic buy, a daily
    task claim, incense, a fortune draw, a chat send, faction join/switch (Phase 5.2 trade create/accept: call it there).
    Never: the presence ping, lecture enter/heartbeat, any `/dao-lu/song-tu/*`, hộ pháp invite/accept (they only refresh
    presence now), GETs, replays. `TestMarkActiveAllowlist` pins the callers. Hộ pháp semantics unchanged otherwise:
    a ping marks online only while active; a protector who just hit the idle cap (settle credits 0) must do one of the
    listed actions. **Residual:** a script can still settle every 10 min (credited > 0 while idle room is left) or chat;
    bounded by the daily caps (song tu 10 points, hộ pháp 2 assists) and flagged by netsig.
  - **H2/M1 — client IP.** nginx takes the client from `CF-Connecting-IP` only when the peer is a Cloudflare range
    (`deployments/cloudflare-ips.conf`, refresh command inside; origin firewalled to Cloudflare — deployments/README) and
    sends `X-Forwarded-For: $remote_addr` (also on `/ws`). The API honours XFF only when `TRUST_PROXY` and the direct peer
    is in `TRUSTED_PROXY_CIDRS` (required outside dev; dev = loopback + private ranges). Dev compose publishes the API
    on `127.0.0.1:8080` only (k6 uses `api:8080` on the compose network, unaffected).
  - **M2 — policy & retention.** `/chinh-sach-du-lieu`: "không thể dịch ngược nếu không có khóa bí mật của máy chủ",
    IPv6 hashed by /64, flags also deleted after 30 days, admin reads logged. Hộ pháp is wired: `GET
    /admin/protector/pairs` carries `sharedNetwork` (netsig, character → account) and an accept between accounts on the
    same network is logged — flag only. The worker clears `net_flag` and deletes `dao_lu_events kind='net_flag'` older
    than 30 days (≤ 1,000 per tick) next to the netsig purge.
  - **L1** Propose locks both characters' pending proposals in id order before the single lazy-expiry update (same order
    as accept); deadlock/serialization errors map to 409 `try_again`. **L2** `decline_cooldown_days` **7** per (proposer,
    target) after a decline; `max_incoming_pending` **10** per target. **L3** admin termination needs a report that is
    not dismissed and is either between the two partners (one reported the other) or about a partner and already
    actioned — an outsider's open report is not enough. **L4** the claim re-checks the bond under the progression lock.
    **L5** target-side blockers answer a generic "Đối phương hiện chưa thể nhận lời cầu thân" (no reasons). **L6** taken
    song tu seconds are restored only when the post-failure check proves the claim row absent. **L7** reads of
    `/admin/netsig/clusters`, `/admin/dao-lu/flags` and `/admin/protector/pairs` write `admin_audit_logs` (`*.read`).
- **Not done / open:** device fingerprinting (only network); player-visible "same network" warning (deliberately admin
  only — telling two strangers they share a network would leak it); WS push of proposals (screen polls 30 s); admin UI
  for flags/clusters (API only); add `NETSIG_SALTS` and `TRUSTED_PROXY_CIDRS` to `.env.example` and the staging/prod env
  (the agent could not edit `.env.example`).

## 2026-11-07 — Vận hành closed beta: metrics, backup, load test, reset (Phase 4)
Implements the owner's "Vận hành" decision below. Runbook: [../deployments/README.md](../deployments/README.md).
- **Metrics:** official `prometheus/client_golang` only (no OpenTelemetry/exporter sidecars yet). Each process builds its
  own registry in `cmd/*` (no global state) and serves it on an **internal listener** (`METRICS_ADDR`, api `:9090`,
  worker `:9091`; compose `expose`, never published); nginx also 404s `/metrics`. Labels are bounded: ServeMux route
  pattern (`Request.Pattern`), status class, catalogue/code enums folded through a bounded set (`other` beyond it).
  Game counters live at the handler boundary and skip idempotent replays; economy flows (linh thạch created/destroyed by
  reason, điểm duyên by source) are aggregated by the worker from the ledgers with an id cursor and a 30 s commit lag.
  Dead letters are counted (bad payloads, unknown topics, rejected hộ pháp rewards — `protector.RewardBatch` now returns
  the count). `tiendao_db_up` / `tiendao_redis_up` are pinged at scrape time (no postgres/redis exporters needed yet).
- **Alerts:** Prometheus rules only, no Alertmanager until someone is on call (open item in the checklist). Grafana and
  Prometheus bind to 127.0.0.1 (SSH tunnel on a server).
- **Backup:** `pg_dump -Fc` run inside the postgres container (matching major version, no host tools), verified with
  `pg_restore --list`, retention by count. The drill restores into a scratch DB and compares migrations, tables, indexes
  and key row counts. Redis is never backed up: restore runbook flushes it and lets the worker reconcile rebuild it.
- **Load tests:** k6 in docker against the api on the compose network, dev-only `X-Dev-User-Id` users from
  `loadtest/seed.sql` (deterministic UUIDs, `@loadtest.invalid`), per-VU `X-Forwarded-For` so per-IP limits model many
  clients (works only in dev: `TRUST_PROXY` + direct api access). One VU = one user so per-user limits stay real.
- **Reset tool:** `cmd/opsctl` (in the game image) with raw-SQL operator code in `internal/ops` (kept out of sqlc).
  Every public table must be classified keep/reset — the reset refuses unclassified tables, so each new migration has
  to decide. Kept: accounts, identities, refresh tokens, admin audit, chat mutes, `net_signals`, **payment records**
  (financial; Tiên Duyên rows they reference are then emptied with DELETE, which fails loudly if a paid order still
  points at one). Tiên Duyên is reset by default (data-model Q21) but top-up rows require `--keep-duyen`, or
  `--wipe-purchases` when every order is sandbox. Typed confirmation `reset <db>`, prod refused without
  `--allow-prod`, audit row `ops.reset_beta`; then Redis FLUSHDB + ranking rebuild.

## 2026-11-07 — Khung nạp Tiên Duyên: cổng giả lập, webhook có chữ ký, đối soát (Phase 4)
Implements the owner's "Thanh toán (Phase 4)" decision below. Migration `000016_payments`, `content/catalog/payments.yaml`,
`internal/payment`. **No real provider yet** — the sandbox stands in until a contract + legal review exist.
- **Catalogue.** Packages author only `amount_vnd` (20k / 50k / 100k / 200k / 500k / 1M); points are computed by the loader
  with `vip.PointsForVND` (the function `TopupLocked` credits with), so a package can never promise a bonus. **No bonus
  points, no first-top-up doubling** — that needs an owner decision. Per-order bounds 10.000đ–2.000.000đ, expiry 30 min,
  ≤ 3 open orders per user, reconcile pending orders older than 5 min every 5 min, keep querying expired orders 24 h.
  Loader: amounts multiples of 1000 within bounds and strictly increasing; a provider may be `enabled` only if it has an
  adapter; **sandbox may never list `prod`**; max amount ≤ `vip.MaxGrantDelta`. Prod has no enabled provider today →
  the web shows "Kênh nạp chưa mở".
- **Tables.** `payment_orders` (BIGINT VND, points promised at order time, provider ref UNIQUE per provider,
  `UNIQUE (user_id, idempotency_key)`, `vip_ledger_id` / `refund_ledger_id`, CHECKs tying paid/refunded/closed status to
  their columns, `ON DELETE RESTRICT` on the user — a financial record outlives the account). Status machine
  `created → pending → paid → refunded`, `created|pending → failed|expired`, **`failed|expired → paid`** (a verified late
  payment means the money arrived: honoured and alerted), enforced in Go (`CanTransition`, conditional UPDATEs) and by a
  `BEFORE UPDATE` trigger. `payment_events` stores every **verified** event (raw body ≤ 16 KiB, `signature_ok`, source
  webhook|reconcile, outcome, detail) with `UNIQUE (provider, dedupe_key)`. `payment_sandbox_charges` is the simulated
  gateway's own state (what its `Query` reads); unused by real providers.
- **Webhook = signature first.** `POST /payments/webhook/{provider}` is public, per-IP rate-limited, body ≤ 16 KiB. Unknown
  provider → 404 and a bad/missing/stale signature → 401 **before any DB access** (invalid calls are logged, not stored —
  no table spam from anonymous callers). Then one transaction: insert event (duplicate id → replay no-op) → lock the order
  → pure `Decide` → on paid `vip.TopupLocked(order id)` + mark paid. Amount ≠ order or points ≠ what the vip rate gives
  now → `rejected` + operator alert (log "payment alert" + `GET /admin/payments/alerts`), nothing credited. failed/expired
  after paid → ignored. A provider-side refund/chargeback event is recorded and alerted, never auto-applied.
- **Exactly once.** Webhook and reconciliation share `apply`; both lock the order row, the second sees `paid` and replays;
  `TopupLocked` is itself order-anchored (`topup:<order>`, partial unique index). Lock order: `payment_orders` →
  `vip_accounts` → `character_appearance`. No provider call inside a transaction (checkout is created after the insert
  commits; a crash leaves a `created` order that a retry with the same Idempotency-Key completes).
- **Worker.** Every `PAYMENT_SWEEP_SECONDS` (60): reconcile (bounded batch, `Query`, settle final answers) then expire
  (bounded `UPDATE … FOR UPDATE SKIP LOCKED`). Idempotent across workers; no per-order timer.
- **Refund.** `POST /admin/payments/orders/{id}/refund {reason}`: provider refund first if the adapter supports it (outside
  the tx, idempotent per order), then in one tx `vip.RefundTx` (new: `vip.Refund` minus its own transaction — level drop,
  perk strip, `vip.refund` audit), order → refunded, `payment.refund` audit. Ledger key `refund:<order>` → any retry is one
  refund. A purchase already reversed through `/admin/vip/refund` is synced, not reversed twice. Known limitation (L1,
  unchanged): claimed gifts / used offline time are not clawed back.
- **Sandbox.** HMAC-SHA256 over `"<unix>.<raw body>"`, header `X-Sandbox-Signature: t=…,v1=…`, ±5 min window (dedupe covers
  replays inside it), secret `PAYMENT_SANDBOX_SECRET` ≥ 32 bytes (dev: random per process if empty; staging: required).
  Dev-only `POST /dev/payments/sandbox/{orderId} {outcome, deliver, amountVnd}` (registered only when `APP_ENV=dev`,
  owner/admin) records the charge and sends a correctly signed webhook through the real verification path;
  `deliver:false` simulates a lost webhook for reconciliation. Web: `/tien-duyen/sandbox/{id}` checkout page.
- **Return URL never credits.** The web opens the checkout in a new tab and polls `GET /payments/orders/{id}` (own orders
  only) every 2.5 s while open and visible; success shows the new level from the server.
- **What a real adapter must implement** (`payment.Provider`): `CreateCheckout` (exact amount, bind our order id, idempotent
  per order, return hosted URL and/or VietQR payload); `VerifyWebhook` (provider signature over the raw body,
  constant-time, timestamp/replay window if the provider has one, NO I/O, map to `paid|failed|expired|refunded` with the
  provider's event id as dedupe key and the paid amount); `Query` (status of an order for reconciliation, `pending` when
  not final); `Refund` (or `ErrRefundUnsupported`). Plus: provider IP allow-list if offered, the provider's expected ACK
  body (current handler answers JSON 200), secrets via env, add the id to `adapters` and enable it in `payments.yaml`.
- **Anti-cheat review (no Critical/High), fixed:** M1 the refund checks the balance covers the purchase (and syncs an
  already-reversed purchase) **before** calling the provider refund; M2 one checkout per order — a fresh order
  (`version 0`) is claimed by exactly one caller (stale claims > 20 s can be taken over), `SetPaymentCheckout` writes
  only while no URL exists, a provider error fails the order only if no checkout was ever recorded; M3 a NEW webhook
  event id saying `paid` on a settled order is `ignored` + alert `duplicate_payment` (player paid twice → manual
  refund), while reconciliation re-reporting it stays a silent replay; L5 vip ledger inconsistencies on refund → 409;
  L6 reconciliation validates adapter output before applying.
- **Open:** provider choice + contract; legal text for the refund policy (web shows a draft placeholder); per-user daily/
  monthly spend limits (consumer protection) — none yet; invoices/VAT; account deletion must anonymize orders (FK is
  RESTRICT); whether a provider chargeback should auto-refund. From the review: (L1) a vip.yaml rate change while
  orders are open makes their payments `points_mismatch` (rejected, alerted) — needs an audited admin "settle order"
  action or crediting the order's stored points; (L2) `payment_events.payload` keeps raw bodies forever — real gateways
  include payer name/account: allow-list fields or add retention before go-live; (L3) real adapters must return
  sanitized errors (no signed URLs / bodies) since errors are logged; (L4) sandbox is enabled in staging — staging's
  `PAYMENT_SANDBOX_SECRET` can mint điểm duyên there, so staging data must never reach prod; (L7) an order failed by a
  checkout timeout is not reconciled (`provider_ref` unknown) — webhook only.

## 2026-11-07 — Chủ dự án chốt trước Phase 4–5
- **Luật trao đổi (Phase 5.2):** người chơi tặng/đổi được **linh thạch và nguyên liệu**; có trần giá trị nhận mỗi ngày,
  giới hạn số lần, tài khoản/nhân vật mới không nhận được. **Không trao đổi:** pháp bảo (tăng tỉ lệ lôi kiếp), đan đột
  phá, vật phẩm mua ở cửa hàng, trang phục — để giao dịch không thành kênh mua tỉ lệ/tiến độ bằng tiền thật ngoài game.
  Ghi đè `cot-truyen.md` §5.5 về pháp bảo. Nhật ký đầy đủ, cảnh báo luồng chuyển một chiều bất thường cho admin.
- **Thanh toán (Phase 4):** xây khung nạp chung (đơn nạp, webhook có chữ ký + idempotent, đối soát, hoàn tiền nối với
  `vip.Refund`) với **cổng giả lập (sandbox)**; chọn cổng thật (PayOS/VNPay/MoMo) sau khi có hợp đồng và ý kiến pháp lý.
- **Chống đa tài khoản:** lưu **băm IP có muối** (không lưu IP gốc) khi đăng nhập/làm mới phiên, tự xóa sau 30 ngày; chỉ
  dùng để cảnh báo admin các cặp tài khoản trùng mạng (song tu, tặng đồ, hộ pháp). Cập nhật `/chinh-sach-du-lieu`.
- **Vận hành:** `/metrics` cho api/worker, Prometheus + Grafana + cảnh báo cơ bản như một profile tùy chọn trong
  docker-compose; script backup/khôi phục Postgres có kiểm thử; load test k6. Không K8s.

## 2026-11-07 — Phase 3.4 anti-cheat review fixes (M1–M5, L1–L5; M2 policy chosen by the coordinator)
- **M1** A replayed bí cảnh submit never credits điểm duyên again (even on a later day with room): it reports the stored
  `vip_point_ledger` row of key `sr:<run>` (0 if the original was capped).
- **M2 hộ pháp vs alts + ping scripts.** Protectors must be ≥ `min_tier` 2 (Trúc Cơ, any path) and their character
  ≥ `min_character_age_days` 7 old (`protector.yaml`, loader-validated, checked at accept). Presence must be backed by
  play — chosen option: **a ping counts only when the same user made a successful authenticated game mutation within
  `activity_window_seconds` 600** (any POST/PUT through `authn` with a 2xx except the ping itself refreshes
  `presence:active:<user>`; reads, failures and pings do not). Simpler and more robust than tying pings to a WS link
  (the realtime hub is an unauthenticated broadcast). Admin review: `GET /admin/protector/pairs?min=&days=` (inviter →
  protector pairs, assists, counted, first/last day; `adminLimit`).
- **M3** `POST /breakthrough/attempt` accepts `acceptSuccessBp` (compare-only): odds lower than the player saw → 409
  `quote_changed` with a fresh preview, nothing written. The web sends the preview's `successBp` and explains the drop.
- **M4** Every reward band needs real play: `min_qi` + `min_kills` besides `min_score` (Hạ 450/200/180, Trung 650/250/220,
  Thượng 900/300/250). 100 seeds: blind scripts (idle, fixed 8-direction loops 10–100 ticks, back-and-forth, square,
  diagonal) 0/100 rewarded; reference player ≥ Hạ 99–100/100. Review flags `mostly_idle | low_change | periodic` in the
  run's telemetry (blind 100/100 flagged, reference 0/100). Engine params unchanged → golden vector unchanged.
- **M5** The worker credits protectors in per-event savepoints, AFTER every Redis call of the batch (vip row locks held
  only until the commit). Permanent errors (bad payload, unknown activity, key reuse, integrity class 23) are logged as
  dead letters and the event is marked published; transient errors keep the batch pending.
- **L1** Runs/day and daily caps are per character across all realms (`secret_realm_days` PK (character, day) — 000015
  edited before shipping). **L2** Inviting needs a Ready tribulation (preview Ready: no cooldown/trial/cultivation gap);
  a protector's daily use = an assist of that day that was counted or is still pending on an open invite (uncounted or
  expired ones are free; `protector_days` is now the per-day lock). **L3** A guaranteed attempt keeps the invite and pays
  nobody. **L4** Bí cảnh submit credits điểm duyên before the loot (vip → currency → inventory). **L5** The replay runs
  outside the transaction; the run is then locked and re-checked.

## 2026-11-07 — Bí cảnh thử luyện + pháp bảo + hộ pháp (Phase 3.4)
Rules: [design/bi-canh-luat-choi.md](design/bi-canh-luat-choi.md). Migration `000015_secret_realms_artifacts_protectors`.
**Every number below is a placeholder for closed-beta tuning (⚠ owner sign-off).**
- **Separate catalogue, same verification.** `content/catalog/secret_realms.yaml` + `internal/secretrealm` (own tables
  `secret_realm_runs`, `secret_realm_days`; routes `/secret-realms…`) rather than a `trials.yaml` kind: a trial is a
  breakthrough gate (pass, gate tier, lose cooldown, consumed by the success), a bí cảnh is a daily resource run
  (runs/day, loot, caps) — sharing a table would special-case most of the trial code. Reused unchanged: server-issued
  seed, params snapshot per run (incl. `minElapsedBp`, `minRewardTicks`), deterministic integer replay in Go with a
  bit-identical TS twin, golden vector (Go + vitest, path via `import.meta.url`, required), recorder that applies the
  human-cadence rule, test-only reference player, telemetry + `slowSubmit` flag, judge order late → invalid →
  implausible → too fast → implausible input → mismatch → outcome.
- **The game (original survivor-like).** 640² sealed realm, 150 s, 6 hộ thể; tà khí (Âm Hồn, Hỏa Linh, Thạch Quỷ,
  Tà Tướng at 60/105/135 s) drift in from the edges; an auto-aimed phi kiếm and, with kiếm ý from collected linh khí,
  1–3 orbiting kiếm trận blades. Input = 8-way direction per tick, RLE `[dir, ticks]`. Human cadence: ≤ 8 direction
  changes per 1 s window (`implausible_input`); min-run floor 30 s for any reward; expiry **240 s** (seed retried locally
  ≤ ~1.6×, accepted as for Tâm Ma/Hóa Phàm). Reference player (40 seeds): survive 100% / 98% / 72% (fast/medium/slow).
- **Runs and loot.** 3 runs/day counted at START (abandon does not refund — no seed shopping), open from Luyện Khí.
  Bands by replayed score: Hạ ≥ 150 (10–20 LT, 1–3 Linh Thảo), Trung ≥ 500 (25–35, 2–4, pháp bảo 0.5%), Thượng ≥ 900
  (35–50, 3–5, 1.5%); survived +10 LT and +1.5% pháp bảo; a pháp bảo already owned → duplicate stones (30…200). Daily caps
  (submit day): **120 linh thạch, 30 materials, 1 pháp bảo**, clipped and disclosed. Loot rolled server-side at submit
  (injected RNG; crypto-seeded PCG in prod), stored in `secret_realm_runs.loot`, granted through the ledgers in the same
  tx (key `sr:<run id>`). Survived also earns vip.yaml `trial_pass` (balance §11.3 "qua bí cảnh hoặc mini game"),
  capped there. Requirement items (Trúc Cơ Đan, Kết Đan Quả) are **not** dropped: the docs allow them in the shop, not as
  a bí cảnh source, and a drop would change the accepted retry-gap analysis. Linh Thảo drops although it has no sink yet
  (it buys nothing — future alchemy input; capped 30/day).
- **Pháp bảo.** 5 artifacts in `items.yaml`: Thanh Đồng Hộ Tâm Kính +1% (linh, w50), Tị Lôi Châu +2% (huyền, w28), Lôi
  Văn Bảo Phù +3% (địa, w14), Huyền Vũ Thuẫn +4% (thiên, w6), Cửu Tiêu Hoàn +5% (tiên, w2). **One equipped** (chosen in
  Túi Đồ, `character_artifacts`, `POST /artifacts/equip`) — visible, deliberate, named in the odds breakdown; not
  "best held counts". Item loader: artifacts never tradable, exactly one effect (`tribulation_bonus_bp > 0`), and **only
  artifacts may raise odds**. `content.LoadAll`: bonus ≤ realms.yaml `item_bonus_max_bp` (700), every artifact drops
  from a bí cảnh (its only source), bí cảnh band items are plain materials, no daily task gives an odds item; the shop
  already refuses artifacts. Top tier stops at +5% so +2% stays for a future đan hộ thân (balance §11.4).
- **Decision F completed (1.3 placeholder).** `TribulationOdds` = base + surplus (≤ 5%) + items (≤ 7%) + protector
  (+1%/member, ≤ 3 members) → total bonus ≤ 12% → final ≤ 90% unless guaranteed. The odds array now **sums to
  successBp**: bonus parts uncapped, then `bonus_cap` (≤ 0) and `max_cap` (≤ 0), `guarantee` on the guaranteed attempt;
  `items.detail` = artifact id, `protector.detail` = counted members; preview `caps` lists every cap. The attempt snapshots
  the same array in `breakthrough_attempts.odds`.
- **Hộ pháp flow (minimal).** `protector.yaml`: invite window 30 min, 2 accepts/protector/game day, presence TTL 120 s,
  ping 45 s. Online = `POST /presence/ping` within the TTL (Redis `presence:char:<id>` + faction ZSET; the play layout
  pings while visible; no ticker, no DB write). The breakthrough-er (faction member, last stage of a tribulation realm)
  opens an invite; fellow members tap "Hộ pháp" (spends a daily use at accept; same invite twice = one use); the next
  tribulation attempt reads presence **before** its tx, locks the invite, counts accepted protectors still online and
  still in the faction (acceptance order, ≤ 3), uses the invite (either outcome), stamps `protector_assists.counted_at`
  and enqueues `protector.assist_counted`. The worker credits vip.yaml `protector_assist` (2 điểm duyên, ≤ 2/day, now
  `live: true`) on the attempt's game day, idempotent per assist, sorted by user — never inside the attempt tx (crossing
  into other players' `vip_accounts` there could deadlock two mutual protectors). Lock order (data-model §2.3) extended:
  … trial_runs → secret_realm_runs → secret_realm_days → protector_invites → protector_days → … → vip_accounts …
- **Not done (open):** community-bucket tu vi for protectors (would move S3 pacing; needs a balance pass); đan hộ thân;
  WS push of invites (members see them on Đột Phá, polled 30 s); trading of pháp bảo (cot-truyen §5.5 says tradable,
  kept non-tradable until Phase 5 trade rules exist).
- **Balance (`make balance`, new "Bí cảnh" tables, 2,000 players × 60 days):** stone income **120/day** (cap binds;
  slow players 117), materials ~11/day; first pháp bảo median day 9–12, ≥ +3% by day 60 for 56–67%, the +5% for 8–10%,
  ~2.3 distinct by day 60. Stone budget (active profiles play every run): surplus A1 +50 → +170, A3/S3 +200 → +320,
  TD12 +500 → +620; every 60-day surplus (≤ 37,200) is below what the existing sinks absorb (on-sale cosmetics 173,100
  + donations 30,000); once the whole on-sale collection is owned (≥ ~280 days) the steady residual rises from 0 to
  **+120/day** for TD12 active profiles only (owner: accept, or add sinks/lower the cap). Progression unchanged (bí cảnh
  gives no tu vi; speed items were already at their cap): S3 100.8 points/day, Chuẩn Thánh **326**; fairness, gift gap
  and retry-gap checks unchanged. Tribulation effect: expected failed tribulations 4.68 → 3.76 with a +5% pháp bảo,
  3.28 with +5% and 3 hộ pháp (≤ 1.4 days saved over the ladder). Tiên Duyên has no input into runs, loot or caps
  (loader + integration test + balancesim check).

## 2026-11-07 — Hóa Phàm: mini game thứ hai, chắn Nguyên Anh → Hóa Thần (Phase 3.3)
Rules: [design/hoa-pham-luat-choi.md](design/hoa-pham-luat-choi.md). Reuses the Tâm Ma framework unchanged
(`trial_runs`, start/submit, `gate.go`, pass kept until a successful breakthrough, 7-day validity, last-stage opening,
resume keeps the seed, abandon = loss, 30 min cooldown, `trial_pass` 2 điểm duyên as the only reward). **No migration.**
- **Original slicing game.** Ink "trần duyên" knots (red fate-thread 1 pt, lotus fibre 2 pt, karmic net 3 pt needing two
  strokes) are thrown up from a pond line and sink back; "chân tâm" paper lanterns rise steadily and must not be cut
  (3 tâm đăng; the 3rd cut fails, `lanterns_out`). A stroke cuts only when a tick's segment is ≥ `min_cut_step`
  (no drag-cutting); the k-th knot of one stroke adds +min(3, k−1) (liên đoạn); 12 cuts without an escape trigger
  5 s of tâm tĩnh (world at half speed). No fruit, no bombs, a drying brush stroke instead of a blade trail.
- **Gate = `trials.yaml` `hoa_pham`, `gate.tier: 4`** (the major breakthrough leaving Nguyên Anh; Vu tier 4 → 5 too —
  same open question as tâm ma about a Vu-flavoured skin). The existing loader cross-check verifies every path has it.
- **Verification** = deterministic Go replay (`internal/trial/hoapham`, TS twin `lib/minigames/hoa-pham/sim.ts`):
  integer physics in 1/32 grid units, one xorshift32 stream, FNV-1a state hash, integer segment–circle test (all
  products < 2^53 by loader bounds). Claim adds `strokes`: per pressed period `[start, x0, y0, dx1, dy1, …]`, one point
  per tick, grid-quantized, delta-encoded (idle ticks free); `lines` = knots severed; `inputs` must be empty (and
  `strokes` empty for tâm ma). Body limit stays `trial.MaxClaimBytes` 96 KiB; the loader checks
  `time_limit_ticks × 9 B + max_strokes × 24 B` fits (≈ 35 KiB today). `Outcome` gains omitempty telemetry fields
  (strokes, maxStep, maxWindowTravel, maxWindowStrokes, lanternsCut, escaped, bestCombo); tâm ma JSON is unchanged.
- **Human-plausibility rules = game rules** applied by the client recorder (so honest logs never trip them) and checked
  by the server (`implausible_input`): brush ≤ `max_step` 48 units/tick (a longer step = teleport); in any 1 s window
  ≤ 720 units of travel and ≤ 8 stroke starts; ≥ 2 unpressed ticks (80 ms) between strokes; pass needs the score
  **and** `min_pass_ticks` 60 s (with the 90% server-clock check ≥ ~54 s real time). Soft skill check, not bot-proof —
  same stance as Tâm Ma v2.
- **Seed foreknowledge (anti-cheat review M1).** The seed is sent at start and a run can be resumed with the same seed
  until it expires, so a player can retry the known layout locally before submitting. Bounded by expiry:
  `hoa_pham.expire_after_seconds` **300** (90 s game + slack → the seed is seen at most ~3×, accepted residual);
  `tam_ma` trimmed **900 → 480** (180 s game + 5 min → ~2.5×). Submits later than game time + 60 s
  (`trial.SlowSubmitSlackMillis`) are **flagged, never rejected** (`slowSubmit` + `elapsedMillis` in the hóa phàm
  outcome/`trial_runs.result`) so review can spot replay-shopping; pauses and slow networks stay legitimate.
- **Review fixes (L1–L3).** A stroke start ≥ `endTick` is rejected before computing its end (a start near MaxInt64
  overflowed `start + points` and slipped past the order checks; Go and TS). Cross-kind logs are rejected both ways
  (`inputs` on hóa phàm, `strokes` on tâm ma). `min_elapsed_bp` is now **snapshotted** into each run's params
  (`minElapsedBp`) for both kinds and `Judge` reads it from there; runs issued before the change fall back to the
  catalogue value.
- **Difficulty (⚠ beta tuning):** field 120×160, 90 s, pass 75 points by ≥ 60 s, waves every 1.8 s → 0.9 s of 1 → 4
  objects, knot weights 6/3/1 + lantern 2. Reference player (`hoaphamtest`, test-only; 8-direction swipe planner with
  hand aim error, recorded through the same rules), 40 seeds: fast (320 ms reaction, ±4 aim) 100%, medium (480 ms,
  ±5) 90%, slow (720 ms, ±7) 82.5%; every failure was `lanterns_out`, none for lack of points. First attempt (pass 60,
  sparse waves) let every profile pass on score alone — waves were made denser so skill shows. Target in beta: ~80–90%
  of decent players within 3 tries.
- **Web:** `/play/hoa-pham` (canvas 2D, one rAF loop, pointer events with capture, `touch-action: none`, mouse works),
  practice "không tính", `prefers-reduced-motion` drops flying shards, rotation and tick interpolation. Đột Phá links
  it via `nav.ts` `trialRoutes`; the Đột Phá trial card picks its copy by kind (the "hóa phàm announced only" fallback
  is gone). Open: an alternative for players who cannot swipe/drag (keyboard/switch); audio (must be original).

## 2026-11-07 — Linh thạch sinks: ba chỗ tiêu định kỳ không tăng tiến độ (chủ dự án chốt cả 3)
Closes the Phase 2.5 open item "no recurring non-progress sink". Owner chose all three; none may speed progress.
Migration `000014_cosmetics_faction_works`. Smallest safe option where the brief left room:
- **One cosmetic id space.** New `content/catalog/cosmetics.yaml`; ids are `"<kind>:<slug>"` with kind `title |
  avatar_frame | chat_aura | cave_decoration` — the same strings as `vip.yaml` perks. `content.LoadAll` binds them:
  every perk of those kinds must resolve to a `source: tien_duyen` entry, every tien_duyen entry must be granted by
  exactly one tier, a perk may not point at a purchasable cosmetic. Tiên Duyên cosmetics have **no row**: owned while
  the level reaches the perk tier (derived). `breakthrough_fx:chi_ton` is not an equippable kind here and stays
  unchecked (open below). Slugs are unique across the file and faction decorations: web art slot `cosmetics/<slug>`.
- **The two Phase 2.5 shop cosmetics moved** (`khung_ngoc_bich` → `avatar_frame:ngoc_bich` 3,000, `dong_phu_tung_hac` →
  `cave_decoration:tung_hac` 5,000; same prices). Their `items.yaml` entries and shop listings are removed; 000014 turns
  any owned copy into a `character_cosmetics` row (`source migrated`) and zeroes the inventory row with an
  `inventory_transactions` row (`cosmetic_migrate`); units above 1 (admin grants only) are refunded at the old shop
  price (`currency_ledger reason migrate_000014_refund`) and counted in a `RAISE NOTICE`. Ledger keys are per run
  (`migrate:000014:up|down:<txid>`) and quantity + ledger always move together, so up → down → up → down → up stays
  consistent (verified on a throwaway DB). **Rollback is pre-launch only** (as 000010/000011): the down migration
  refuses once any non-legacy cosmetic was bought or any faction donation exists; otherwise it gives the two legacy
  cosmetics back as one item each. The shop's "≥ 1 cosmetic
  listing" rule (decision G) moved to the cosmetics loader (≥ 1 shop cosmetic); `content.LoadAll` now **refuses** a
  shop listing of a `cosmetic` item, and the item loader refuses effect fields on `cosmetic` items.
- **No gameplay effect, enforced by the loaders:** cosmetics.yaml and faction_works.yaml reject any effect-like key
  (`*_bp`, `*_milli`, `*_hours`, `*_seconds`, `*_points`, every items.yaml effect key, `merit`, `rank_points`…) with
  "is a gameplay effect". Prices must **strictly increase within a shop series** (escalating ladder).
- **Rotation:** a series may carry `season` (xuan/ha/thu/dong = game-day months, UTC+7) and/or `from_era`/`until_era`
  (world eras; before launch the first era applies). Outside the window it is not sold; owned items stay usable.
- **Ownership & equip** (`character_cosmetics` PK = owned once; `character_appearance` one row per character;
  `cosmetic_equips` log). `POST /cosmetics/buy {cosmeticId}`: lock currency → idempotency → quote (catalogue price,
  window, `min_tier` gate — cosmetics only, allowed by non-negotiable 3) → debit (`currency_ledger cosmetic_purchase`,
  key `cosmetic:<key>`) → ownership row. `POST /cosmetics/equip` sets the **whole loadout** (one title, one frame, one
  chat aura, ≤ `cave_slots` = 4 decorations): ensure + lock the appearance row → idempotency (same key + same loadout =
  replay, other loadout = 409) → every id owned and of the slot's kind. No outbox (nothing else changes).
- **Others see it (ids only, client resolves via public `GET /cosmetics/catalog`, cached 5 min):** `GET /character`
  `appearance`; chat messages carry `nameFx` = the chat aura worn **at send time** (`chat_messages.name_fx` snapshot);
  global leaderboard and faction member pages carry `avatarFrameId` / `titleId` (LEFT JOIN, display only — the score
  ignores them). `GET /cosmetics/catalog` is rate-limited per IP like `GET /world`.
- **Tiên Duyên level drop (anti-cheat H1):** `vip.Refund` and a negative `vip.Grant` unequip every perk cosmetic of a
  tier above the new level in the same transaction (vip computes the ids from its own table — no `cosmetic` import;
  lock order `vip_accounts` → `character_appearance`). Defence in depth: `GET /character`, `GET /cosmetics`, chat send
  (the `nameFx` snapshot) and leaderboard / faction-member rows hide a perk above the current level
  (`cosmetic.Visible`, fed by `vip_accounts.points`). The loader also refuses a cosmetic perk on tier 0. Web: frames on `Portrait`, title under the name, name effects in chat/profile, decorations in the Động
  Phủ scene; CSS/SVG placeholders until raster art lands in `cosmetics/<slug>` (no images generated).
- **Faction donation** (`faction_works.yaml`, `POST /factions/donate {amount}` + key): every faction builds the same
  works in order; stage thresholds unlock purely visual decorations; progress is derived from
  `faction_treasuries.donated_total` (never stored). Daily cap **per character across factions** (switching doesn't
  reset it), `min_amount` 10. Lock order (data-model §2.3 extended): `character_faction` (FOR UPDATE) →
  `currency_accounts` → `faction_donation_days` (conditional upsert) → `faction_donors` → `faction_treasuries` +
  donation row in **one final statement** (the shared hot row is held only until the commit right after; the response
  view is read after the commit — anti-cheat M3). The công đức bảng (`faction_donors`, top 10) is display only. **The service writes nothing that
  feeds merit (`daily_contribution`), ranking (no outbox), điểm duyên (`vip_*`) or tu vi** — stated in the package doc,
  integration-tested (`TestDonateFeedsNoProgressMeritRankOrDuyen`). Donations stay with the faction they were given to.
- **Pools across faction changes (L3):** on `migrate_faction` (a rename) the pool and công đức bảng move to the new id
  in the worker step that finishes the move (`faction.Effects`, tested); on a **split** the source pool stays under the
  old id as archived history (shown read-only, "đã đóng, lưu trữ") and the branches start from zero.
- **Balance** (`make balance`, new "Recurring non-progress sinks" table; assumptions `-donateshare 3000` = 30% of the
  surplus donated while cosmetics remain on sale, `-reserve 2000` kept as a retry buffer, cheapest-first purchases,
  120 days; corrected after anti-cheat M4 — only the CURRENT season's set is on sale, unbought pieces leave with their
  season): surplus before **+0…+550/day**; at day 120, 77–93% of it is absorbed for every profile with a surplus except
  A1 free (+50/day: 50%, its 3,000 wallet is the reserve) and A1 TD12 (86%, was 93% before the fix); wallets ≈ 2–6k. At
  720 days (`-sinkdays 720`): 87–99% absorbed, but idle TD12 wallets drift to ≈ 11–22k as the year-round stock runs out
  and only one season set (10,500) rotates in per quarter. Once the on-sale collection is owned the donation cap leaves a
  steady residual of **≤ +50/day** (only I3 TD12, 550/day) — **if** players then donate up to the cap; players who
  stop donating keep up to their full surplus (the honest upper bound). Progression output is byte-identical (S3
  100.8 points/day, Chuẩn Thánh day **326**; fairness, gift gap, retry gap unchanged) — the sinks are not inputs of the
  progression model.
- **⚠️ PLACEHOLDERS for owner sign-off** (marked in the YAML):

  | Series | Prices (linh thạch) |
  |---|---|
  | Đạo hiệu (titles) | 1,500 · 3,000 · 6,000 · 10,000 · 16,000 · 25,000 |
  | Khung chân dung | 2,000 · 3,000 (Ngọc Bích) · 6,000 · 12,000 · 20,000 |
  | Danh quang (chat) | 2,500 · 5,000 · 10,000 · 18,000 |
  | Động phủ — Tùng Hạc (cả năm) | 1,200 · 2,400 · 5,000 (Bình Phong Tùng Hạc) |
  | Động phủ — mỗi bộ theo mùa (Đào Hoa, Liên Trì, Thu Nguyệt, Mai Tuyết) | 1,500 · 3,000 · 6,000 |
  | Động phủ — Tử Tiêu (khai_giang → luan_hoi) / Phong Thần (từ phong_than) | 2,000 · 4,000 · 8,000 / 4,000 · 8,000 · 14,000 |
  | Cúng góp thế lực | trần 500/ngày/nhân vật, tối thiểu 10; công trình 0.5 M · 3 M · 10 M · 30 M (3 mốc mỗi công trình) |

  Totals: 230,600 for the full shop catalogue, 173,100 on sale at launch.
- **Open (owner):** (1) prices/cap/goals above, and whether more seasonal/era stock is needed for the long run
  (see the 720-day drift); (2) — closed (H1 above); (3) `breakthrough_fx:*` perks have no catalogue yet; (4) completed works could announce in world chat (not built);
  (5) Tiên Duyên names of the perk titles (Duyên Khởi … Chí Tôn Tiên Duyên) are placeholders.

## 2026-11-07 — Chủ dự án chốt: ngoại lệ retry gap, nhịp 330 ngày, Thương Lan chia ở luan_hoi, Nhân Tộc mở ở Nữ Oa
Four owner decisions closing open items of Phase 2.1 / 2.2 / 2.5.
1. **Ngoại lệ công bằng — retry gap ACCEPTED.** What: with requirement items bought from spare stones, idle Tiên Duyên
   12 @1 login/day reaches Trúc Cơ on day 5 and Kim Đan on day 17, vs free @3 logins/day on days 6 and 19.3 (TD8 @1:
   Kim Đan 19.0 vs 19.3); active profiles pass. Why: the lead comes only from the daily gift's linh thạch paying for
   Trúc Cơ Đan / Kết Đan Quả retries sooner (anti-cheat L3); it is 1–2.3 days early in a 330-day arc, buys no odds, no
   rate and no rank points, and points/day fairness (TD_L@1 ≤ free@3) still holds everywhere. Boundary: requirement
   items stay buyable with linh thạch (never with money directly, never gated by Tiên Duyên level); the exception
   covers ONLY this lead, for the idle+stones profile, up to **3 days** on any requirement-item breakthrough.
   `balancesim` keeps the check, labels the rows "accepted exception (decisions 2026-11-07)", stays report-only within
   the bound and **exits 1 if any lead exceeds 3 days** (or if the active profile ever leads). Rule:
   `.claude/rules/economy-balance.md`.
2. **Pacing back to ~330 days.** Incense streak 1 / 1.5 / 2 → **0.2 / 0.3 / 0.4** points (streak 1 / 3 / 7); gieo
   quẻ buffs Đại Cát +20% / Cát +10% / Tiểu Cát +5% → **+5% / +2.5% / +1%** (Bình 0); odds unchanged and disclosed
   (5 / 20 / 45 / 30 %, Σ 100%), still no negative outcome. `make balance`: S3 103.7 → **100.8 points/day**, Chuẩn
   Thánh day 317 → **326**; A1 47.0 → 45.4, A3 83.7 → 80.8; idle profiles, gift gap and stone budget unchanged; all
   fairness checks pass except the accepted exception above (leads unchanged: 1.0 / 2.3 days). The rituals remain a
   small daily habit, not a pacing lever.
3. **Thương Lan Tiên Đình closes at `luan_hoi` with a split** (decision 9 rules: 3-day notice, 7-day choice, default =
   branch with the fewest members placed by this split, ties in YAML order; locked members get the default). Chosen
   mechanism: **generic era-start trigger**, not `split_faction` in `on_end`. `factions.yaml` gets `on_close: split` +
   `split: { notice_days, choose_days }`; the branches are **computed** from the catalogue (`CloseSplitTargets`: every
   other faction open in the closing era that allows all the faction's paths — today Huyền Khung Môn, Kim Sa Môn, Tinh
   Hà Yêu Đình; not Vu Tộc (path), not Nhân Tộc (opens at `tao_nhan`)). `content.LoadAll` registers them on the world
   catalogue (`SetEraEffects`); `world.setEra` schedules them as pending `world_effects` rows (`effect_index` 1000+,
   `milestone_id` = the milestone that moved the era or whose event's `on_end` did) for **every era it reaches or
   skips** — so the split still happens when `nu_oa` fires before the `dai_chien` event ends and the late `on_end` is
   skipped (era never moves backwards; scheduled exactly once). Then the existing `faction.Effects` runner executes it
   unchanged: noticed/applied/finalized lifecycle, outage-safe notice and deadline (H1), split-origin counting (H2).
   Why not `on_end`: it would need nested effect indexes and still miss the split when the `on_end` is skipped; the
   closing rule belongs with the faction. **New guard:** computed branches may close in a later era (Huyền Khung Môn,
   Kim Sa Môn at `thanh_nhan`); once that era started, choices into them are refused (`ErrFactionClosed`) and defaults /
   `mustChoose` use only branches still open (`OpenBranches`), so nobody lands in a faction whose own split/migrate
   already ran. Loader: `on_close: split` needs ≥ 2 computed branches and at least one that never closes. No migration,
   no API shape change.
4. **Nhân Tộc opens at the Nữ Oa milestone** (`open_from_era: tao_nhan`, still `name_placeholder: true`). Timeline card
   7 now says the Nhân Tộc faction, open since Nữ Oa, flourishes (web `timeline.ts` copy updated with it);
   `content/story/factions/README.md` and `world/milestone-effects.md` record both choices.

## 2026-11-07 — Tâm Ma rules v2: soft skill check, human cadence (anti-cheat review M1, L1–L5)
- **The gate is a soft skill check, not bot-proof.** The game is deterministic from a seed the client receives and
  the client holds every rule, so a solver can always play it. Defence = make cheating no faster or more rewarding
  than playing: the reward is only the gate + capped 2 điểm duyên, and v2 forces human cadence. Accepted.
- **Rules v2** (`rules_version: 2`, identical in Go and TS, params in trials.yaml, loader-validated):
  ≤ `max_inputs_per_window` 16 inputs in any `input_window_ticks` 20-tick (1 s) window, else `rejected /
  implausible_input` (replaces the 4-per-tick cap; the client throttles with a 3-press queue so honest logs never
  violate it); a hard drop / locking soft drop is **ignored** sooner than `min_lock_gap_ticks` 6 (300 ms) after the
  previous lock (a game rule, so it can't misfire on honest players); the pass needs `pass_lines` **and**
  `min_pass_ticks` 1500 (75 s) — with the 90% server-clock check a pass costs ≥ ~68 s of real time.
- **Reference player at human cadence** (2-piece look-ahead, thinks 3×gap ticks per piece, 40 seeds): 5 inputs/s
  31/40 (78%), 3.3/s 35/40 (88%), 2.2/s 36/40 (90%); passes land at 75–78 s. `min_pass_ticks` 1800 gave 70–90%,
  2400 gave 45–78% (too hard). ⚠ beta tuning.
- Cadence telemetry (inputs, inputs/s, densest window, min lock gap, claimed end tick) is stored in
  `trial_runs.result`; the response carries `outcome` only for pass/fail (rejections stay server-side).
- The reference player moved to the test-only package `internal/trial/tamma/tammatest` (not imported by any binary).
- Golden vector is now `packages/contracts/golden/tam-ma-v2.json`; **v1 removed** — pre-launch, no v1 run can be in
  flight, and the server only accepts `rules_version: 2` (a stored v1 run would be judged `invalid_log`).
- Submit has its own body limit `trial.MaxClaimBytes` (96 KiB); the loader checks `max_inputs × 12 B` fits. `Judge`
  rejects impossible claimed lines (`< 0` or `> endTick`) before computing `MaxScore`.

## 2026-11-07 — Mini game: khung thử luyện + Tâm Ma (Phase 3.1, 3.2)
Implementation choices for the trial framework and the first mini game (D6). Migration `000013_trial_runs`.
Rules: [design/tam-ma-luat-choi.md](design/tam-ma-luat-choi.md).
- **Gate lives in `trials.yaml`** (`gate.tier: 3` = the major breakthrough leaving tier 3), not as `requires_trial`
  on realms.yaml as data-model §3.1 sketched — one source per gate, cross-checked in `content.LoadAll` (the tier must
  have a major breakthrough on every path). Tiers are equal across paths, so **Vu đạo tier 3 → 4 also needs Tâm Ma**
  (open question: a Vu-flavoured name/skin later).
- **Pass kept until the breakthrough succeeds** (consumed on success, `consumed_ref = bt:<key>`), valid
  `pass_valid_seconds` = 7 days. Not consume-on-attempt: the breakthrough already has its own odds, pity and 2 h
  cooldown; making a 50% roll also cost a replay of the mini game double-punishes bad luck, and cot-truyen §5.4 says a
  lost trial only delays. The pass is a skill gate, never an odds lever. While a pass is valid no new run starts
  (409 `pass_active`), which also caps điểm duyên farming.
- **Opens only on the gated realm's last stage** (`opens_at: last_stage`): "tâm ma hiện trước bình cảnh". Eligibility
  uses the read-only projection (pending idle time counted). Practice mode is client-only (local random seed, nothing
  sent, labelled "không tính") for everyone.
- **Verification** = deterministic replay in Go of the client's input log from the run's seed and the params
  snapshot stored on the run (`trial_runs.params`), so a catalogue change never alters a run in flight. Order:
  late → invalid log → implausible score → too fast (server elapsed < simulated time × `min_elapsed_bp` 90%) →
  mismatch (replay ≠ claimed endTick/lines/score/stateHash) → the game's outcome. The client's claim is never used as
  a value. Shared PRNG xorshift32 + FNV-1a state hash; Go/TS drift caught by the golden vector
  `packages/contracts/golden/tam-ma-v1.json` (Go test + vitest).
- **Runs**: one open run per character and kind (partial unique index). Start while a run is open **resumes it**
  (same seed, game restarts at tick 0) — friendly to reloads; the residual "see the piece order twice" advantage is
  bounded by `expire_after_seconds` (15 min; trimmed to 8 min on 2026-11-07, see the Hóa Phàm entry) and accepted. A run never submitted is closed as `expired/abandoned` on
  the next start with the cooldown counted from its expiry (no seed shopping). **Every non-pass verdict** (fail,
  rejected, expired) starts `lose_cooldown_seconds` = 30 min.
- **Idempotency**: start = `Idempotency-Key` (`issue_key`, unique per character); submit = single use per run — the
  same key replays the stored verdict (no second điểm duyên), any other key → 409 `already_submitted`. Another
  character's run id → 404. Both mutations take the progression lock first; lock order (data-model §2.3):
  progression → daily_contribution → **trial_runs** → vip_accounts → inventory (the breakthrough locks the valid
  pass after its settlement, before inventory).
- **Rewards**: gate only, plus the existing vip.yaml activity `trial_pass` (2 điểm duyên, ≤ 2/day, inside the 20/day
  earn cap) now `live: true` — specified by data-model §7.1. No tu vi, stones or odds.
- **Difficulty (⚠ beta tuning)**: board 9×16, 6 original blots of 3/5 cells (weights 3,3,1,1,1,1), 12 lines in
  180 s, gravity 14 → 5 ticks/row (−1 per 20 s), a tạp niệm row rises every 20 s, tĩnh tâm after 4 lines for 8 s.
  The reference player in `tamma/bot.go` (two-piece lookahead) passes ~90% (fast) / ~85% (≈2 inputs/s); the first
  piece set (with U/S pentominoes) passed only ~10–70% and was replaced. Target in beta: 70–85% of real players
  within 3 tries.
- **Web**: `/play/tam-ma` (canvas 2D, one rAF loop, fixed tick; keyboard + touch pad; pause, auto-pause on tab hide,
  `prefers-reduced-motion` drops glow/flash/shake). The Đột Phá screen shows the gate (`preview.trial`, missing
  `trial`) with a link; the verdict screen shows only the server's verdict.

## 2026-11-07 — Mở đầu Tử Tiêu Cung: những lựa chọn khi triển khai (Phase 2.3)
Implementation choices for the new-player flow (data-model §9.6, smallest safe option each time). No migration.
- **One intent, no onboarding state on the server.** The steps (opening → Tử Tiêu Cung → faction → path → đạo hiệu)
  live in the client (`/create`, draft + Idempotency-Key in sessionStorage). The last step sends `POST /character
  {name, factionId, path}` (Idempotency-Key required when `factionId` is set): one transaction creates the character
  **directly at the first stage of the faction's path** (requested path if allowed, else the faction's first — Vu Tộc
  → `vu`, no `path_converted` event), then runs the normal join (`faction.Service.JoinTx`: same locks, era/open checks,
  `faction_history` key, outbox; the first join starts the 7-day cooldown). Any refusal rolls the character back.
  Replay = existing character with the same name + a `faction_history` row for that key and faction → 200
  `replayed: true`; a key used for another faction → 409 `idempotency_key_reused`. Old `POST /character {name}` and
  `/factions/join` stay for existing characters.
- **State is derived**: `GET /character` now returns `factionId` (from `character_faction`). The game guard
  (`lib/game/onboarding.ts guardTarget`): no character → `/create`; character without faction → `/chon-the-luc` for
  every game route (the "để sau" skip link is gone, so §9.6's "every character has a faction" holds); returning
  players skip both. `/chon-the-luc` now lands in Tử Tiêu Cung like the wizard.
- **Name last** as the spec orders it; the server checks it only at the final intent (409 `name_taken` keeps the
  player on the name step, faction errors send them back to the faction step).
- **Opening video placeholder**: the 8 scenes of cot-truyen §10 as still frames (shipped art + CSS), captions and
  Hồng Quân's 8 opening lines in `content/story/onboarding/mo-dau.md`, mirrored in `lib/game/onboarding.ts` (the docker
  web build has no `content/`; a vitest checks the copy word for word). D2: no Tam Thanh/Chuẩn Đề/Tiếp Dẫn, no Tổ Vu or
  "Long Hán"; the đại năng are evoked by the five factions' places. Auto-advances unless `prefers-reduced-motion`
  (then manual); pause, skip (button/Esc) and back on every frame; "Xem lại mở đầu" from the hall step.
- **"First lecture"** = the hall at `/play/tu-tieu-cung`. The lecture loop is on the shared server clock, so a
  newcomer joins whatever lecture is playing (not lecture 1); a toast welcomes them.
- **Path step** is shown only when the chosen faction allows several paths (none does today); otherwise the path is
  confirmed on the faction detail and the name step.

## 2026-11-07 — Hoạt động hằng ngày & cửa hàng: những lựa chọn khi triển khai (Phase 2.5)
Implementation choices where data-model §8/§9 left room (smallest safe option each time). Migration `000012`.
- **Shop** (`shop.yaml`, `POST /shop/buy {listingId, lots}` + Idempotency-Key, `GET /shop`): the API takes a
  *listing* and a lot count (data-model §11), not `{itemId, quantity}` — price, item and lot size come from the
  listing only. One transaction: lock `currency_accounts` → idempotency (`shop_purchases`) → lock inventory row →
  quote → conditional upsert `shop_daily_counts` → debit (`currency_ledger reason shop_purchase`) → grant
  (`inventory_transactions`) → purchase row; ledger key `shop:<key>`. Listings: Tụ Linh Đan 125 (2/day), Tụ Linh Phù
  300 (1/day), **Trúc Cơ Đan 400 and Kết Đan Quả 900 (3/day each — consumed on success and failure, and until now no
  in-game source existed)**, cosmetics Khung Ngọc Bích 3,000 and Bình Phong Tùng Hạc 5,000 (`unique_per_character`).
  Linh Thảo is **not** listed until something consumes it; shop speed items are `tradable: false`. A key reused with
  a different listing or lot count → 409 `idempotency_key_reused`.
- **Tiên Duyên gate (`min_tier`) only on cosmetic listings** (loader rule). §8.2's example gated Tụ Linh Phù (TD1) and
  Kết Đan Quả (TD2); dropped, because a level gate on a progress or requirement item would let Tiên Duyên decide
  progress (non-negotiable 3). No shipped listing uses `min_tier` yet.
- **Loader (S1):** `content.LoadAll` refuses any listing whose item is an `artifact` or has `breakthrough_bonus_bp` /
  `tribulation_bonus_bp`; effects must be shop-safe (`offline_cap_bonus_hours`, `instant_points_milli`,
  `max_use_per_game_day`) and need a daily limit; at least one non-progress cosmetic listing (decision G).
  `items.yaml` cross-check: Σ instant points × max uses ≤ `speed_items.daily_cap_points` (5 × 2 = 10).
- **Speed items** (`POST /items/{id}/use`): lock progression → idempotency → settle first → `item_daily_uses`
  conditional upsert → consume → `CreditLocked(item)`. **All or nothing** (anti-cheat M1): unless the pill's full
  value is credited (speed cap, activity bucket, 150 ceiling, bình cảnh room), the transaction rolls back and the pill
  and the daily use are kept (409 `no_room` / `bottleneck_full`).
- **Daily tasks:** `trung_kich` replaced by `nghe_giang` (kind `lecture_attend`, 200 stones), recorded in the ngộ đạo
  payment transaction of Tử Tiêu Cung when that insight paid **≥ `lectures.yaml reward.attend_min_seconds` (300 s)
  of listening** (real attendance; also when the lecture's daily points cap was reached).
- **Lock order** (one global order, anti-cheat M2/M3): progression → daily_contribution → item_daily_uses →
  daily_task_progress → vip_accounts → **currency_accounts → inventory_items** → shop_daily_counts. `dailytask.Claim`
  credits điểm duyên, then currency rewards, then item rewards; the lecture records its task row before the điểm
  duyên credit.
- **Incense** (`POST /tasks/daily/incense`, task `dang_huong`, "Dâng hương tĩnh tâm"): once per game day; the
  offering itself pays the streak reward (1 / 1.5 / 2 points at streak 1 / 3 / 7, `CreditLocked` source `ritual`) and
  completes + claims the task in the same tx (điểm duyên `daily_task_claim`, so the 4 daily tasks match its 1 × 4).
  No stones, no "nhận lộc" framing, not addressed to any figure. A repeat the same day replays (no key needed).
  `ritual_streaks` keeps `last_day`, `streak`, `best`; a missed day restarts at 1.
- **Gieo quẻ** (`GET /fortune`, `POST /fortune/draw`; distinct from the lecture "ngộ đạo"): `fortunes.yaml`, 4
  outcomes with disclosed odds — Đại Cát 5% (+20%), Cát 20% (+10%), Tiểu Cát 45% (+5%), Bình 30% (+0%); **no negative
  outcome**. Server roll (injected RNG, `daily_fortunes` PK = once per day). §9.4 read as: a `character_buffs` row
  (`slot fortune`) from the draw to the next reset; **settlement applies the bonus only to eligible seconds inside
  the buff window, where the eligible window is the first `offline cap` seconds after the last settlement**; the draw
  settles first and the buff starts at max(now, settled timestamp), so it is never retroactive; capped by new `contribution.yaml idle.max_buff_bonus_bp: 2000`;
  the idle bucket cap (75) and 150 ceiling still bound it. Source `fortune` (activity bucket) stays unused.
- **Balance** (`make balance`, rituals modelled with the expected fortune bonus and a full incense streak for active
  profiles): S3 100.0 → 103.7 points/day, Chuẩn Thánh day 329 → 317 (−3.6%); A1 45.0 → 47.0; idle profiles unchanged;
  TD_L@1 ≤ free@3 still holds at every level; gift-driven speed still ≤ 6/day. Speed price now read from `shop.yaml`
  (25 stones/point). The fortune buff is in the fairness matrix (active/social rows).
- **Stone budget** (balancesim, creation vs destruction per day): the only recurring sink is speed items (≤ 250/day);
  steady-state surplus after them: free I1 +100, I3 +250, A3/S3 +200, TD12 profiles +350…+550 stones/day; cosmetics
  (8,000 one-off) absorb it once. ⚠️ **Open (owner):** no recurring non-progress sink yet; requirement items are gates,
  not sinks. → **Closed** by the 2026-11-07 "Linh thạch sinks" entry.
- **Retry gap** (accepted as an exception, see the 2026-11-07 owner-decisions entry): when requirement items must be bought with spare stones, idle TD12 @1 login
  reaches Trúc Cơ on day 5 vs free @3 day 6, Kim Đan day 17 vs 19.3 (TD8 @1: Kim Đan 19.0 vs 19.3) — the gift buys
  Trúc Cơ Đan / Kết Đan Quả without pausing speed items. Points/day fairness still holds. Not tuned; owner decision.
- **Not done:** worker log of daily stone creation vs destruction (§8.2) — the ledger has the data; add with the next
  worker change.

## 2026-11-07 — Thế lực: những lựa chọn khi triển khai (Phase 2.2)
Implementation choices where data-model §6/§10 left room (smallest safe option each time):
- **Roster** = `content/story/factions/README.md`: 5 starting factions, the 3 Huyền Khung viện and `kim_sa_giao`
  (open from `thanh_nhan`), all `name_placeholder: true` until the owner approves (decision 12; `Placeholders()` is the
  release-gate list). New YAML fields: `color` (design-token suffix), `motto`/`pitch` (display text), `on_close`.
- **Canon left open, YAML-driven and closed by default:** `nhan_toc` has **no `open_from_era`** (never joinable) until
  the owner picks `tao_nhan` or `nhan_toc`. `thuong_lan_dinh` closes from `luan_hoi` with `on_close: keep_members`:
  its members stay (and may switch out) — no effect moves them until a decision exists. The loader requires every
  `on_close: effect` faction to be moved by exactly one split/migrate effect, whose targets open in the era that
  milestone sets and allow every path of the source (so the worker never converts paths).
- **Before launch** (`world_state.era_id` NULL) the first era applies: the 5 starting factions are joinable.
- **Join/switch path:** joining a faction that doesn't allow the character's path converts it (`realm.Convert`, same
  tier/stage, cultivation kept) to the requested `path` or the faction's first path; logged in `faction_history`
  (from/to path) and `progression_events(path_converted)`.
- **Cooldown:** 7 days (`switch_cooldown_seconds`) from the last *player* change, **including the first join**
  (`first_join_free: false`, anti-cheat H2: a fresh alt can't join and hop into a branch before a split). Split choices,
  defaults and migrations never touch it.
- **Locks** (`fengshen` | `admin` | `event`, optional hours) block player switches **and split choices** (anti-cheat M3):
  a locked member is placed by the deadline default. Admin lock/unlock is audited.
- **Split flow:** milestone reached → `faction.split_notice` (notice_days) → at `due_at` every source member gets
  `pending_split_effect_id` and `faction.split_done` opens the choice (choose_days) → `POST /factions/split-choice`
  → at `deadline_at` everyone still pending is placed one by one (character_id order) into the branch with the
  **fewest members placed by this split** so far — `faction_history` rows of the effect (choices + earlier defaults),
  ties → `into` order — so branches end as even as possible and alts/switchers who joined a branch directly can't steer
  it (anti-cheat H2; refines decision 9 "fewest members"). `faction.split_final` (new template) lists the branches.
  **Outages never shorten a window** (H1): a late notice moves `due_at` to notice + `notice_days` (and the deadline
  with it); applying a split sets `deadline_at = max(stored, applied + choose_days)`; the worker runs effects even when
  the world tick fails. Migrate: `faction.migrate_notice`
  → at `due_at` all members move (`auto_migrate`) → `faction.migrate_done`. Notices use `world_effects.noticed_at`.
- **Runner:** `faction.Effects` runs right after each world tick in the worker, one short tx per step under the world
  advisory lock (7_310_002), ≤ 500 members per step, ≤ 40 steps per tick; history keys `effect:<id>` + the
  noticed/applied/finalized markers make re-runs no-ops.
- **Faction ranking = merit** (data-model §10, decision H): Σ per member-day of the **world-countable** points —
  `min(world_credit_cap 100, points_milli − catch_up_bonus_milli)`, the same rule as world progress (anti-cheat H3: the
  ×2 catch-up bonus never counts; speed-item points do, as for the world) — from the first game day wholly inside the
  current era (`world_state.era_started_at`, new): the day after the era started, or the launch day for an era that
  started on it (L5, merit is per game day). A day's points belong to the faction **stamped on
  its `daily_contribution` row** (the faction at the day's first credit; a first join the same day claims that day).
  Points don't follow a switcher. Redis: `leaderboard:faction:merit` (ZSET), `leaderboard:faction:members` (HASH),
  `leaderboard:faction:<id>:realm` (members by global realm score). Worker refreshes from `faction.changed` /
  `leaderboard.dirty` (per-character ZSET moves only); member counts and bulk moves only set a Redis "rebuild
  requested" marker, executed on the next world tick under the reconcile advisory lock — never inside the outbox drain
  (M2); full rebuild (incl. merit) on reconcile. `GET /factions` falls back to Postgres counts (no merit, cached 15 s
  in-process) when Redis is down.
- **Settle first** (M1): join/switch/split-choice settle idle cultivation under the same progression lock before the
  faction changes, so offline accrual is credited and stamped to the faction it was earned in (`cultivation`'s
  `SettleLockedRow` injected from wiring).
- **Idempotency keys** are bound to operation + target (L1): reusing one for another change → 409
  `idempotency_key_reused`. Join/switch read `world_state` `FOR SHARE` (L2).
- **Milestones are append-only after launch** (`seq` = catalogue position, `TestShippedMilestoneSeqIsStable`); a seq
  conflict at reach is logged as an error (L4). Down migrations 000010/000011 refuse once the world has started (L6).
- **Faction merit điểm duyên** (decision D, `faction_merit` 1 × 3) stays `live: false`: §6 doesn't define the action
  that earns it. TODO when faction activities exist.

## 2026-11-07 — Mốc thế giới: những lựa chọn khi triển khai (Phase 2.1)
Implementation choices where data-model §5 left room (smallest safe option each time):
- **Eras** follow `story/world/milestone-effects.md` (9 eras). `set_era` added to `nhan_toc`, `thien_dinh`, `tay_du`;
  `luan_hoi` is set by the `dai_chien` event's `on_end` (only `set_era` is allowed in `on_end`). **An era never moves
  backwards at runtime**: with no minimum gap (H), `nu_oa` may fire before the 15-day `dai_chien` event ends; that
  late `on_end` is then skipped.
- **Ranged events** live in a new table `world_events` (not in §5.4); ended once by the worker tick (≤ 16 per tick),
  which runs `on_end` and writes outbox `world.event_ended`. Events still end while the world is paused.
- **Faction effects** (`split_faction`, `migrate_faction`) are validated (slug ids from `milestone-effects.md`) and
  stored as **pending `world_effects` rows** (`applied_at` NULL, `due_at` = reached + notice, `deadline_at` for split).
  Nothing executes them until Phase 2.2; faction ids are cross-checked against `factions.yaml` once it exists.
- **Milestone tick** runs in the same transaction as the day job, under the existing advisory lock: fold → end due
  events → reach rule (≤ 1 milestone). WS announcements go out from the outbox drain after commit (same path as
  major breakthroughs); clients refetch `GET /world` on `world.*` events.
- **Admin**: `PUT /admin/world/milestones/{id}` replaces the override of an unreached milestone (threshold, target day,
  `forceReach`, `paused`) with a required reason; `POST /admin/world/pause|resume` holds every milestone (days still
  close and fold). All audited. A force still respects order and lands on the next tick.
- **`GET /world`** is public (same for everyone, `max-age=15`, per-IP limit) and also returns the full `timeline`
  (names, target day, event length, reached day). Card blurbs stay in the web app (`lib/game/timeline.ts`, copied
  from the story doc) since narrative is not catalogue config.

## 2026-11-07 — Tiên Duyên v3: những lựa chọn khi triển khai (Phase 1.4)
Implementation choices where data-model §7 and the code differed (smallest safe option each time):
- **Ledger reason** stays `activity` for in-game điểm duyên (000008) instead of the doc's `earn`; migration `000009`
  adds `source`, `activity_id`, `game_day`, `reverses_ledger_id` with the §7.2 CHECKs. Back-filled rows: lecture rows →
  `source lecture`/`lecture_attendance`, everything else → `admin` (there was no top-up path).
- **Capped credit** (activity used up or 20/day reached) writes no ledger row; a credit that would cross 20 is clipped.
- **Online cultivation** counts a player-initiated settle whose elapsed time ≥ `min_elapsed_seconds` (3600) — the same
  rule as daily task `van_cong`; settles run inside other mutations never count.
- **Lecture**: the amount comes from vip.yaml `lecture_attendance`; `lectures.yaml` keeps only the trigger marks
  (`vip_milestones_points: [10, 20]`), cross-checked ≤ its `daily_max_times`. Ledger keys unchanged.
- **Refund** applies only to `purchase` rows (source `topup`) and is refused, not clamped, when the balance can no longer
  cover it (an admin revoke happened in between). `vip.MaxGrantDelta` = 500,000. A refund row also stores
  `ref_type='vip_ledger'`, `ref_id=<purchase id>`, and 000009's backfill rebuilds `reverses_ledger_id` and the activity
  attribution from `ref_type`/`ref_id` only, so a migration down → up never re-opens a refunded purchase.
- **Top-up idempotency is anchored on the payment order** (`ref_type='topup_order'`, `ref_id=<order>`): partial unique
  index `vip_point_ledger_purchase_order_uq` + ledger key `topup:<order>`. Re-sending an order replays; the same order
  for another user or amount is refused. Admin grants must match their sign (grant/promo > 0, revoke < 0).
- **Known limitation (anti-cheat L1):** a refund removes the điểm duyên only. Daily gifts already claimed and the longer
  offline window already used at the higher level are not clawed back (data-model §7.3). Revisit if refund abuse shows up
  (e.g. top up → claim gifts / bank offline time → charge back).
- **Activities not built yet** (hộ pháp, song tu, thế lực, thí luyện, sự kiện) are in vip.yaml with `live: false`;
  `EarnLocked` refuses them until flipped. The UI shows them as "sắp mở".
- **Balancesim** gift model assumes 25 linh thạch per speed point (Tụ Linh Đan 125 / 5) until the shop ships (`-speedprice`).

## 2026-11-07 — Tử Tiêu Cung: Hồng Quân giảng đạo cả ngày
Owner choices: **lecture loops all day** (replaces the 2 fixed sessions of data-model §9.1 / plan §3.5), reward =
**tu vi + điểm duyên**, hall chat = **its own channel** shown as speech bubbles over seated listeners.
- **Hall.** `/play/tu-tieu-cung` (the map's Tử Tiêu Cung plaque). Hồng Quân on the dais speaks one line at a time
  from `content/story/lectures/*.yaml` (8 lectures, ~37-minute loop) on the server clock. Online players get the
  lowest free of 36 cushions (`lectures.yaml hall.seats`); later arrivals listen "ngoài điện" with the same reward.
- **Reward.** Heartbeats (30 s) accrue listening on the server clock in Redis; gaps > 75 s count nothing. Every
  10 minutes a 2-minute **ngộ đạo** window opens (Hồng Quân asks the lecture's question); tapping it pays the accrued
  time at **10 points/hour**, at most 12 minutes per ngộ đạo and **20 points per game day** (the lectures' existing
  share of the 45-point activity bucket, so pacing/balance is unchanged — continuous listening only spreads it over
  the day), through `CreditLocked` (source `lecture`). Điểm duyên: **3 at 10 and 3 at 20 points** per day
  (decision D "lecture 3 × 2"), ledger reason `activity`, ref `lecture_insight`. One row per character per slot
  (`lecture_insights`) is the idempotency guard and feeds the **Đạo ngộ** notebook.
- **O4 resolved:** if Redis loses presence, players lose at most the unpaid listening since their last ngộ đạo and are
  re-seated on the next heartbeat; nothing durable lives only in Redis.
- **Chat.** Channel `tu_tieu_cung` in `chat.yaml`, same moderation, rate limit and Luyện Khí tầng 3 posting rule.
- **Accepted risks (anti-cheat review 2026-11-07):** (1) a script that heartbeats and taps ngộ đạo can farm the
  same capped amount as a real listener (20 points + 6 điểm duyên/day); a server-issued question challenge is the
  fix if abuse shows up. (2) `lecture.enter/leave` go to every WebSocket client (name, stage, characterId), like
  chat — presence is visible to anyone watching the socket.
- Open: the daily task `nghe_giang` (decision 13) still waits; timeline card `giang_dao_3` says "lần giảng cuối", which
  conflicts with an always-on lecture — story-creator proposes milestones 2 and 3 unlock new lecture cycles.

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
- ~~⚠️ Spirit stones have sources but no sink yet.~~ Resolved 2026-11-07 (Phase 2.5): the shop (`shop.yaml`) is the
  sink — speed items (≤ 250 stones/day), breakthrough requirement items and one-off cosmetics. See the entry above.

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
