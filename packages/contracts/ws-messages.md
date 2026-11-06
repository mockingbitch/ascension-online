# WebSocket messages (selective live features — GDD §23)

Endpoint: `GET /ws` (same origin as the web app; Origin is checked against `PUBLIC_BASE_URL`).
Read-only for clients: the server ignores client frames. Server pings every 30s; slow clients are dropped.
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
| `leaderboard.moved` (planned) | S→C | `{ characterId, oldRank, newRank }` |
| `sect.chat` (planned) | C↔S | `{ sectId, text }` — would need authenticated sockets + per-user rate limit |

Rules: never send state the client could treat as authoritative; no per-second broadcast to all players.
