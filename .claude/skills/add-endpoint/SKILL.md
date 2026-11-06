---
name: add-endpoint
description: Add a REST endpoint end-to-end in Tiên Đạo - SQL query, Go service, httpapi handler, OpenAPI contract, TS types and Svelte client. Use when exposing new data or actions to the web app.
---

# Add endpoint (vertical slice)

1. **Contract first** — add path + schemas to `packages/contracts/openapi.yaml` under `/api/v1`.
   JSON camelCase; error envelope `{ "error": { "code", "message" } }`.
2. **Data** — new table/query? → `new-migration` skill. Then `make sqlc`.
3. **Domain** — pure functions + `Service` method in `services/game/internal/<domain>/`.
   Mutations follow the `game-transaction` skill. Reads may skip tx but must not lock.
4. **Handler** — `services/game/internal/httpapi/`:
   - register in `New()` with method pattern: `mux.Handle("POST /api/v1/x", d.auth(http.HandlerFunc(d.x)))`
   - decode with size limit (`http.MaxBytesReader`, 64KB), validate, call service, `writeJSON`/`writeError`
   - map domain errors to status codes in the handler only
   - add `httptest` tests (auth required, validation errors, happy path with fake service if DB-free)
5. **Frontend** — mirror types in `apps/web/src/lib/game/types.ts`, add a method to `api` in
   `src/lib/api/client.ts`, use it from `+page.ts` (reads) or event handlers (mutations; replace state with response).
6. **Verify** — `make lint && make test`; run `make api` + `make web` and exercise the endpoint.

Public catalogue reads (`/realms`) may set `Cache-Control`; player-specific responses must not be cached.
