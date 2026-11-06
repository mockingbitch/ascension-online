---
paths:
  - "services/game/**/*.go"
---

# Go backend rules

## Layout
- `cmd/<binary>/main.go`: wiring only (config, pools, signal handling). No game logic.
- `internal/<domain>/`: `<domain>.go` pure types/rules, `service.go` transaction owner, `*_test.go` table tests.
- `internal/db/` is sqlc output. **Never edit**; change `db/queries/*.sql` or migrations then `make sqlc`.
- `internal/platform/*`: infrastructure adapters (postgres, redis, metrics).

## Style
- Go ≥ 1.25, stdlib first: `net/http` method routing, `log/slog`, `math/rand/v2`, `errors.Join`.
- Errors: wrap with `%w` and context (`fmt.Errorf("lock progression: %w", err)`); sentinel errors for
  domain cases (`ErrNoCharacter`), mapped to HTTP codes only in `httpapi`.
- `context.Context` first arg on anything that does I/O. Respect cancellation.
- No package-level mutable state. Inject `Now func() time.Time` and `RNG` for determinism.
- No `float64` for quantities. Use `int64` + `fixedpoint` helpers (`MulBP`, `MulDiv`, `SatAdd`).
- Logs: `slog` structured keys (`"characterId", id`). Never log tokens, password hashes, or full request bodies.

## Gameplay mutations
Follow the `game-transaction` skill. Minimum: `InTx` → `SELECT … FOR UPDATE` on the hot row → idempotency
check → validate → mutate + ledger/log row → outbox row → commit. External calls (Redis, HTTP) never inside the tx.

## Testing
- Pure rule functions: table-driven tests incl. edge cases (zero, cap, overflow, clock skew, retries).
- Invariant tests: "split settlement ≤ single settlement", "pity never exceeds cap", etc.
- Content tests load the real `content/catalog/*.yaml` (they are the content CI gate).
- `go test -race ./...` must pass. DB integration tests go behind `//go:build integration`.

## Performance
- No DB query per tick/frame; batch in worker. Bounded goroutines (worker pools), bounded channels.
- Set server timeouts (`ReadHeaderTimeout` etc.). Use `pprof` + load tests before optimizing.
