# Architecture rules (always on)

Source: GDD §14–29. These apply to every change.

## Authority boundary
- Go server decides: cultivation amount, breakthrough result, loot, currency, inventory, ranking score.
- Client (SvelteKit) only renders state and sends *intents* (`settle`, `attempt breakthrough`, `join sect`).
  Requests never carry amounts, results, timestamps or odds the server would trust.
- Every state-changing response returns the full authoritative state; the client replaces, never merges.

## Time
- Use server time (`Service.Now()` injected; `time.Now` only in `cmd/*` wiring). Never accept client clocks.
- Progress = f(timestamps). No per-player ticker, timer or goroutine. Settle at login/action boundaries.

## Data placement
| Kind | Where | Example |
|---|---|---|
| Design config (tuned by designers) | `content/catalog/*.yaml` → loaded, validated, immutable in memory | realm curve, VIP caps, item defs |
| Durable player state | PostgreSQL | progression, inventory, ledger |
| Derived / rebuildable | Redis or computed on read | leaderboard ZSET, presence, rate limits |
| Narrative | `content/story/**` | lore, announcement templates |

Never make Redis the only copy of anything valuable. Never hardcode game numbers in Go or TS — put them in catalog YAML.

## Module boundaries (services/game/internal)
- One package per domain (`cultivation`, `breakthrough`, `vip`, `sect`, ...). Pure rule functions (no I/O) +
  a `Service` that owns the transaction.
- `httpapi` is thin: decode → service → encode. No SQL, no game math in handlers.
- Domains talk via function calls in-process now; side effects to other domains go through `outbox_events`.
- Don't add a new process/service, message broker, or K8s without measurements in the PR description.

## Scope guard (GDD §12)
Out of MVP: realtime action combat, open-world simulation, physics, large PvP, PixiJS, Protobuf, microservices.
Exception (decisions.md 2026-11-06): the four story mini games (tâm ma, hóa phàm, bí cảnh, caro PvP) are in scope —
server-issued seeded sessions, server-verified results, capped rewards.
If a task drifts into these, stop and ask.
