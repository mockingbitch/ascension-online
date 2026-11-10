# WebSocket messages (selective live features — GDD §23)

Endpoint: `GET /ws` (same origin as the web app; Origin is checked against `PUBLIC_BASE_URL`).
Read-only for game state: the only client frame is a **topic subscription** (below); anything else is ignored. Players *send* chat over REST
(`POST /api/v1/chat/{channel}/messages`: auth, rate limit, masking, persistence), then the API publishes the stored
message here — the socket itself never needs authentication. Server pings every 30s; slow clients are dropped.
Clients reconnect with capped exponential backoff + jitter (`apps/web/src/lib/api/realtime.svelte.ts`).

Topology: worker → Redis `PUBLISH realtime:broadcast` → every API instance → its connected sockets.
Delivery is at-most-once (a missed announcement is acceptable); clients de-duplicate by `id`.

**Topics (Tử Tiêu Cung điện, decisions 2026-11-10).** A socket follows at most one topic. Client → server text frames:

```json
{ "op": "sub", "topic": "hall:3" }      // follow điện 3 (replaces any previous topic)
{ "op": "unsub" }                        // follow nothing (broadcasts only)
```

Valid topics: `hall:<n>` with 1 ≤ n ≤ lectures.yaml `hall.max_halls` (no leading zero); others are ignored. Topics are
public rooms (seats and hall chat are visible to any player), so no authentication; > 30 frames per minute closes the
socket (policy violation). Clients send `sub` after every (re)connect and whenever their điện changes (`hallId` from
`GET /lecture/state`, or a `lecture.leave` about themselves with `movedTo`). Topic envelopes travel on Redis channel
`realtime:topic:<topic>` (API instances `PSUBSCRIBE realtime:topic:*`) and reach only that topic's sockets on every
instance; `lecture.count` is computed by each API instance itself and pushed to its local hall sockets (no Redis hop).
Everything not marked "topic" below is still broadcast to every socket.

Envelope (JSON; Protobuf only after measurement):

```json
{ "v": 1, "type": "announcement.major_breakthrough", "id": "123", "ts": "2026-11-06T10:00:00Z", "data": { } }
```

| type | direction | data |
|---|---|---|
| `announcement.major_breakthrough` | S→C | `WorldAnnouncement` (openapi): `{ id, kind: realm\|tribulation, tier, characterId, characterName, realmId, stageId, path, tribulation, text, createdAt }` — a realm breakthrough (never a minor stage-up) or a passed tribulation. `tier` = the realm's effect tier (realms.yaml visual.fx_tier); `text` from `content/story/announcements/major-breakthrough.yaml` (`realm.t<k>`, `tribulation.<path>.t<k>`), ≤ 90 chars. Envelope `id` = history row id. |
| `announcement.first_to_realm` | S→C | `WorldAnnouncement` with `kind: first_to_realm` — the first character of the world to reach that realm id (race-free `realm_firsts`). Never folded into a digest. |
| `announcement.rank_top10` | S→C | `WorldAnnouncement` with `kind: rank_top10`, `rank` — an already-ranked character entered the top of Thiên Bảng or reached rank 1; at most once per player per `rank_cooldown_seconds` (6 h). |
| `announcement.digest` | S→C | `WorldAnnouncement` with `kind: digest`, `characterName` (the first) and `count` (the others) — more than `max_separate_per_window` (3) breakthroughs in one `window_seconds` (10 s) window are folded into one line. |
| `chat.message` | S→C | `ChatMessage` (openapi): `{ id, channel, characterId, name, stageId, text, sentAt, nameFx?, look?, hallId? }` — **topic `hall:<hallId>`** for the Tử Tiêu Cung channel (`hallId` = the sender's điện, stored with the message; backfill `GET /chat/tu_tieu_cung/messages?hall=N`), broadcast for every other channel (`look` = sender look at send time, display only: `{ bodyId, outfitId, hairId, robePaletteId, poseId }`; snapshots older than outfits have no `outfitId` — clients derive it from `hairId` + `robePaletteId`, else the body's default outfit). Envelope `id` = message id. Published by the API after commit. |
| `chat.hidden` | S→C | `{ id, channel }` — an admin hid message `id`; clients remove it. Topic `hall:<n>` for a hall message, else broadcast. |
| `lecture.enter` | S→C, **topic `hall:<hallId>`** | `Listener` (openapi): `{ characterId, name, stageId, hallId, seat, factionId?, look? }` (`factionId` display only;  `look` = the listener's chosen look, resolved: body + outfit + pose, display only) — someone sat down in this điện (new arrival, switch or merge). Everyone in a điện has a cushion (`seat` 0…capacity-1). Envelope `id` = characterId. |
| `lecture.leave` | S→C, **topic `hall:<hallId>`** | `{ characterId, hallId, seat, movedTo? }` — someone left this điện: left, was swept (no heartbeat for `max_gap_seconds`; sweeps run on hall requests and in each API instance's presence loop), or moved (`movedTo` = new điện, by `POST /lecture/hall` or a merge). A client receiving its **own** leave with `movedTo` refetches `GET /lecture/state` and subscribes to the new topic. |
| `lecture.count` | S→C, **topic `hall:<hallId>`** | `{ listeners, halls, hallId, count }` — every `hall.count_push_seconds` (5 s) from each API instance to its hall sockets: `listeners` = everyone in every điện ("đạo hữu đang nghe khắp Hồng Hoang"), `halls` = open điện, `count` = this điện. Envelope `id` = unix seconds. |
| `world.milestone_reached` | S→C | `{ milestoneId, name, seq, reason, serverDay, eraId, eraName, text }` — a story milestone was reached (`reason` threshold \| max_time \| admin). `text` rendered from `content/story/announcements/world-milestones.yaml` key `world.<milestoneId>` (`{day}`, `{era}`). Published by the worker outbox drain after the world tick commits. Clients refetch `GET /world` for state. |
| `world.event_ended` | S→C | `{ eventId, milestoneId, serverDay, eraId, eraName, text }` — a ranged event (`start_event … duration_days`) ended; `eraId` is the era after its `on_end` effects. `text` from key `world.event_end.<eventId>`. |
| `faction.announcement` | S→C | `{ key, kind, from, targets, dueAt, deadlineAt?, text }` — a faction world effect moved a step. `key`: `faction.split_notice` / `faction.migrate_notice` (milestone reached, notice period starts), `faction.split_done` (choice window open until `deadlineAt`), `faction.migrate_done` (members moved), `faction.split_final` (deadline passed; `targets` = branches that received members, undecided members spread to the smallest). `kind` split_faction \| migrate_faction. `text` from `world-milestones.yaml` (`{faction}`, `{branches}`, `{target}`, `{days}`). Published by the worker outbox drain. Clients refetch `GET /factions/me` (+ `GET /factions`). |
| `leaderboard.moved` (planned) | S→C | `{ characterId, oldRank, newRank }` |
| sect/faction chat (planned) | S→C | same as `chat.message` with another `channel` (Phase 2); can reuse the topic mechanism (`faction:<id>`), which allows one topic per socket today. |

Announcements (GĐ-D, decisions 2026-11-09 #7 #8 #12): the worker renders, paces (content/catalog/announcements.yaml),
stores a short history (`world_announcements`, `GET /api/v1/announcements/recent` for reconnects / reloads) and
publishes. Clients show them by tier: tier 2 → ticker; 3–4, top-10 and digests → sliding banner; 5–6, first-to-realm and
rank 1 → large banner with sound; never their own news as a banner (their own celebration already played). Banners and
their sound can be turned off in Cài đặt (the ticker and the notification list always stay); they are also shown as
client-side system lines in the world chat (not stored as chat). Only the đạo hiệu names a player — no personal data.

Rules: never send state the client could treat as authoritative; no per-second broadcast to all players.
