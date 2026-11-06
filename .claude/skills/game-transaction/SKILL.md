---
name: game-transaction
description: Implement a state-changing gameplay operation (settle, breakthrough, claim reward, craft, donate, buy) safely in Go - row lock, idempotency, ledger, outbox. Use for any mutation of progression, currency or inventory.
---

# Game transaction pattern

Reference implementation: `services/game/internal/cultivation/service.go` (`Service.Settle`).

## Shape

```go
func (s *Service) DoThing(ctx context.Context, userID uuid.UUID, idemKey string, in Input) (State, error) {
    var out State
    err := postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
        q := db.New(tx)
        // 1. Resolve the actor's character (ownership check is implicit: lookup by userID).
        // 2. Lock the hot row(s):  q.GetProgressionForUpdate / currency account FOR UPDATE.
        //    Lock order when several: progression → currency_accounts (by currency) → inventory (by item_id).
        // 3. Idempotency: look up <log_table> by (character_id, idemKey). Found → return current state, Replayed=true.
        // 4. Validate with PURE functions (rules package) using server time s.Now() and catalogue data.
        //    Return typed domain errors (ErrNotReady, ErrInsufficient...) — no partial writes.
        // 5. Resolve randomness server-side via injected RNG; persist roll + effective chance.
        // 6. Mutate: UPDATE hot row (version = version + 1); INSERT ledger/transactions rows with *_after values.
        // 7. INSERT the operation log row with idemKey (UNIQUE guards concurrent duplicates).
        // 8. INSERT outbox_events for side effects (leaderboard.dirty, announcement.major, notification...).
        out = State{...}  // full authoritative state
        return nil
    })
    return out, err
}
```

## Rules
- No network calls (Redis, HTTP, payment) inside the tx. Use the outbox; worker handlers must be idempotent.
- Keep tx short: load catalogue data before/without DB; compute in memory.
- A unique-violation on the idempotency key (concurrent duplicate) → roll back and return current state.
- Never trust input amounts/results; input is only *intent* (+ choices like which pill to use).
- Rate-limit the endpoint (per user) in `httpapi`.

## Tests to write
1. Pure rule table tests (ready / not ready / each missing requirement / caps / boundaries).
2. Invariant tests (retries don't double-award; split ≤ single; balances never negative).
3. Integration (`//go:build integration`): same key twice → one effect; two different keys concurrently →
   serialized, consistent totals.

## Wire-up checklist
- Handler in `internal/httpapi` requiring `Idempotency-Key` (regex `^[A-Za-z0-9_-]{8,64}$`).
- Error → HTTP mapping (400 validation, 404 missing, 409 not ready/conflict, 429 rate limit).
- `packages/contracts/openapi.yaml` + `apps/web/src/lib/game/types.ts` + `api` client method.
- Use the `anti-cheat-reviewer` agent on the diff before finishing.
