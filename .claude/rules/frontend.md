---
paths:
  - "apps/web/**"
---

# Frontend rules (SvelteKit + TS + Tailwind v4)

- Svelte 5 runes only (`$state`, `$derived`, `$props`, `$effect`); no legacy `export let`/stores for new code.
- TS strict, `noUncheckedIndexedAccess`. No `any`; types for API live in `src/lib/game/types.ts`.
- All HTTP through `src/lib/api/client.ts`. Mutations send `Idempotency-Key`; responses *replace* local state.
- Never compute rewards/results client-side. Allowed: display-only interpolation (e.g. animate the qi bar
  between server snapshots using `rateMilli`), clearly labeled and corrected by the next server response.
- Load data in `+page.ts` / `+layout.ts` with the provided `fetch`.
- No per-frame updates of large component trees. Use CSS animations/transitions or one `requestAnimationFrame`
  loop in a small component; respect `prefers-reduced-motion`.
- MVP rendering = DOM + CSS + SVG. **Do not add PixiJS/canvas engines** without an approved design decision.
  Exception: plain `<canvas>` 2D (no engine) inside `src/lib/minigames/` for the approved mini games.
- UI text is Vietnamese; keep strings in components for now (i18n later), no hardcoded numbers from game config.
- Styling via tokens in `src/app.css` — see `graphic-design.md`.
- Before finishing: `npm run check && npm test && npm run build`.
