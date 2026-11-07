# WebSocket messages (selective live features — GDD §23)

Endpoint: `GET /ws` (same origin as the web app; Origin is checked against `PUBLIC_BASE_URL`).
Read-only for clients: the server ignores client frames. Players *send* chat over REST
(`POST /api/v1/chat/{channel}/messages`: auth, rate limit, masking, persistence), then the API publishes the stored
message here — the socket itself never needs authentication. Server pings every 30s; slow clients are dropped.
Clients reconnect with capped exponential backoff + jitter (`apps/web/src/lib/api/realtime.svelte.ts`).

Topology: worker → Redis `PUBLISH realtime:broadcast` → every API instance → its connected sockets.
Delivery is at-most-once (a missed announcement is acceptable); clients de-duplicate by `id`.

Envelope (JSON; Protobuf only after measurement):

```json
{ "v": 1, "type": "announcement.major_breakthrough", "id": "123", "ts": "2026-11-06T10:00:00Z", "data": { } }
```

| type | direction | data |
|---|---|---|
| `announcement.major_breakthrough` | S→C | `{ characterId, characterName, realmId, stageId, text }` — `text` rendered from `content/story/announcements/major-breakthrough.yaml` |
| `chat.message` | S→C | `ChatMessage` (openapi): `{ id, channel, characterId, name, stageId, text, sentAt }`. Envelope `id` = message id. Published by the API after commit. |
| `chat.hidden` | S→C | `{ id, channel }` — an admin hid message `id`; clients remove it. |
| `lecture.enter` | S→C | `Listener` (openapi): `{ characterId, name, stageId, seat }` — someone sat down in Tử Tiêu Cung (`seat` -1 = outside). Envelope `id` = characterId. |
| `lecture.leave` | S→C | `{ characterId, seat }` — someone left or was swept (no heartbeat for `max_gap_seconds`). Sweeps piggyback on hall requests; no timer. |
| `world.milestone_reached` | S→C | `{ milestoneId, name, seq, reason, serverDay, eraId, eraName, text }` — a story milestone was reached (`reason` threshold \| max_time \| admin). `text` rendered from `content/story/announcements/world-milestones.yaml` key `world.<milestoneId>` (`{day}`, `{era}`). Published by the worker outbox drain after the world tick commits. Clients refetch `GET /world` for state. |
| `world.event_ended` | S→C | `{ eventId, milestoneId, serverDay, eraId, eraName, text }` — a ranged event (`start_event … duration_days`) ended; `eraId` is the era after its `on_end` effects. `text` from key `world.event_end.<eventId>`. |
| `faction.announcement` | S→C | `{ key, kind, from, targets, dueAt, deadlineAt?, text }` — a faction world effect moved a step. `key`: `faction.split_notice` / `faction.migrate_notice` (milestone reached, notice period starts), `faction.split_done` (choice window open until `deadlineAt`), `faction.migrate_done` (members moved), `faction.split_final` (deadline passed; `targets` = branches that received members, undecided members spread to the smallest). `kind` split_faction \| migrate_faction. `text` from `world-milestones.yaml` (`{faction}`, `{branches}`, `{target}`, `{days}`). Published by the worker outbox drain. Clients refetch `GET /factions/me` (+ `GET /factions`). |
| `leaderboard.moved` (planned) | S→C | `{ characterId, oldRank, newRank }` |
| sect/faction chat (planned) | S→C | same as `chat.message` with another `channel` (Phase 2); per-channel delivery may then need channel-scoped subscriptions. |

Rules: never send state the client could treat as authoritative; no per-second broadcast to all players.
