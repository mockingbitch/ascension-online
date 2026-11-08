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
- MVP rendering = DOM + CSS + SVG. **Do not add PixiJS/Phaser/Three or any canvas/3D engine** without an approved
  design decision. **3D (GĐ7, approved 2026-11-08 in `docs/decisions.md`):** Three.js is allowed **only** inside
  `src/lib/engine/render/render3d/`, reached **only** through a dynamic import behind the `renderer=3d` flag
  (`render/index.ts`); nothing else may import `three` or `render3d/*` statically (the initial `/play` bundle stays
  3D-free). 2D stays the default; a 3D renderer implements `SceneRenderer`, renders on `gameLoop()` (never
  `setAnimationLoop`/its own rAF), takes colours from tokens, disposes every GPU resource on unmount and falls back to
  2D (no WebGL2 / context lost / fps < 40 for 5 s, remembered in settings).
  Exceptions (plain Canvas 2D, no engine): `src/lib/minigames/` for the approved mini games, and the client engine in
  `src/lib/engine/` (decisions.md 2026-11-07 "Client webgame"):
  - **One** `requestAnimationFrame` loop: subscribe to `engine/loop.ts` (`gameLoop()`); never start another rAF.
    It pauses on `visibilitychange` and explicit pauses.
  - Particles only via `engine/fx/ParticleSystem` (pooled, ≤ 300 full / ≤ 80 lite / 0 off, DPR ≤ 2), driven by a
    `ParticleField` (`fx/layer.ts`, on the loop only while particles live or an emitter runs): `fxLayer` = the
    full-screen FxLayer canvas (a manual popover, re-raised above modal windows when an effect starts); the scene field
    = `scene.fx` in a scene view (box-local px, under the UI, cleared/paused while the scene is covered or moving).
    Ambient spawning = `scene.fx.addEmitter(every(ms, spawn))` in a view `$effect` (returns its removal). No DOM
    "particles" (many small animated nodes).
  - Effects are planned by `fx/plan.ts` (pure: event + quality → commands, unit-tested) and run by the lazy
    `fx/director.ts` (positions: anchors via the renderer, HUD/bar targets via `data-fx-target="qi"`,
    `data-bar="inventory"`, `data-chip="stones"`, the last pointer press). HUD numbers use `components/CountUp.svelte`
    (held via `fx/hud.svelte.ts` while orbs/items fly to them, always ending on the server value).
  - Scene positions only via `SceneDef.anchors` + `SceneRenderer.anchorToScreen` — no hard-coded px/% for scene points.
  - Scenes are routes (`engine/scene/routes.ts`), drawn by the always-mounted `SceneLayer` + a lazy scene view
    (`engine/scene/views/`); pages hold UI only and mark the scene area with `<SceneSlot>`. Never re-draw scene art in
    a page. Windows over a scene stay shallow routing (`lib/game/windows.ts`) — the scene pauses, never unmounts.
  - Layout loads in `(game)` must not read `url` (it re-runs them on every navigation); guards read `parent()`.
  - Stores emit semantic events on `fxBus` **after** a server response, with server values; effects/audio only listen
    (never call the API, never compute rewards).
  - Effects/audio/renderer preferences live in `engine/settings.svelte.ts` (`settings.fx` = full | lite | off,
    defaulting to off under `prefers-reduced-motion`). Scenes, mini games and audio load lazily (dynamic import);
    initial `/play` JS may grow ≤ 60 kB gz per refactor phase.
- **Windows (GĐ3).** Every menu is a window, never a `/play/*` route: add a kind to `lib/game/windowStack.ts`
  (`WindowState`, deep-link params), a lazy content component in `lib/windows/` + its entry in `lib/windows/registry.ts`
  (title, icon, size, tabs, help topic). Open with `openWindow()` (stack ≤ 2 in `page.state.wins`; Back/Esc/swipe =
  `history.back()`), never `goto`. Frame = `components/GameWindow.svelte` only (no ad-hoc overlays/dialogs); a modal
  inside a window or page uses `<GameWindow local>`. Former screen URLs stay deep links via `[screen]/+page.ts`.
- **Modes (GĐ5).** Full-screen modes render in the shell's ModeLayer, which hides HUD/action bar/chat/windows
  (`visibility` + `inert`; the scene stays mounted, covered and paused). Mini games stay routes listed in
  `engine/scene/routes.ts` `modeRoutes`; their page wraps everything in `engine/mode/ModeShell.svelte` driven by a
  `ModeController` (pure rules in `engine/mode/machine.ts`, unit-tested): lobby → 3-2-1 countdown (`countdownMs(fx)`,
  off = instant "Bắt đầu") → play → waiting (gather loop) → result (`ModeResult.svelte`: badge icon + text, server
  stats, reward lines as `data-fx-source` with a `data-bar="inventory"` bag) / soft error with retry. Esc and Back go
  through one rule: pause, then confirm, then a confirmed exit gives up (the board's log is judged like any end). The
  result shows only after the server answer; submissions use `lib/game/inflight.ts` (`RESPONSE_TIMEOUT_MS`, a retry
  joins the request in flight, a late answer still lands). Boards take `paused` (bindable), `hold`, `countdownMs` —
  presentation only; never touch sims, seeds, input logs. The breakthrough sequence is not a route: fxBus
  `breakthroughStart` / `breakthrough` / `breakthroughStalled` / `breakthroughCancelled` → `planFx` (timeline in
  `fx/breakthrough.ts`) → director → `engine/mode/breakthrough.svelte.ts` → lazy `BreakthroughMode.svelte` (own
  `page.state.mode` history entry: Back skips, then closes).
- **Audio (GĐ6).** Only `engine/audio/AudioManager.ts` (`audio()`) touches Web Audio: no AudioContext before a gesture
  or while muted, suspended while hidden; BGM/ambience = `playLoop` (1 s crossfade) from `SceneDef.music/ambience`.
  New sounds: a slot in `engine/audio/manifest.ts` `AUDIO_SLOTS` (+ a `synth.ts` recipe if it gets a placeholder) and
  a cue in the pure `engine/audio/sfxMap.ts` (unit-tested) — components never call `play()` for game events; they
  emit on `fxBus` and the lazy `router.ts` plays. Buttons click through one delegated listener (opt out with
  `data-sfx="off"`). Audio files = original/licensed only, listed in `lib/assets/audio/README.md`.
- **PWA (GĐ6).** `src/service-worker.ts` caches static assets only; its rules live in the pure `lib/sw/routes.ts`
  (`classifyRequest`, unit-tested). Never cache or answer `/api/*` (incl. `/api/v1/auth/google/*`), `/ws`,
  `__data.json` or HTML navigations (network-only; the prerendered `/offline` page only when the network fails).
  `static/manifest.webmanifest` / `<meta name="theme-color">` are the only places with a literal hex (= `--color-ink-950`,
  checked by `lib/sw/manifest.test.ts`); icons come from `tools/make-icons.mjs`. Fullscreen via
  `engine/fullscreen.svelte.ts` (hidden where unsupported); install hint via `engine/install.svelte.ts`. Inside
  `/play` the shell sets `data-play` (no pull-to-refresh, no double-tap zoom; pinch-zoom stays).
- **Action bar / primary.** Navigation lives in `components/ActionBar.svelte` (items in `lib/game/nav.ts`). A scene's
  main action is registered by its page with `usePrimaryAction(() => …)` (calls the page/store handler; no logic in
  the bar). Chat is `components/FloatingChat.svelte` only — no fixed chat panels in pages.
- **Regions & layers (UI polish, decisions.md 2026-11-09).** The `/play` shell is a grid of regions that it owns:
  HUD (+ ticker) / STAGE / CHAT DOCK / BAR (phones: rows hud·stage·chat·bar; ≥ 640: hud over stage + right rail, the
  chat dock a fixed box in the stage's bottom-left). One ResizeObserver in the shell measures them and publishes
  `--hud-h`, `--bar-h`, `--rail-w`, `--chat-w`, `--chat-reserve`, `--stage-inset-*` on `<html>` (pure math in
  `engine/layout/regions.ts`, store `regions.svelte.ts`). The **stage is the only place for a scene page's UI**: it
  clips, and a page has **one scroller** (`overflow-y: auto; overscroll-behavior: contain`) — never let UI run under
  the bar/chat, never patch with `margin-bottom`/`--chat-lift`. Wide pages keep bottom UI clear of the chat dock with
  `--chat-w` (e.g. `margin-left: max(calc(var(--chat-w) + …), …)`). Must-reach actions (Đột phá) sit in a
  `position: sticky; bottom: 0` row of the scroller. Scene-anchored UI (map plaques) is clamped into
  `regions.stage` and kept off page cards / the dock (`relaxLabels`, avoid boxes marked `data-scene-avoid`).
  Pointer: regions take the pointer; the stage lets empty space through to the scene only when the page's
  `<SceneSlot passthrough>` asks (map hotspots, seat hover cards) — then its UI blocks opt back in with `.ui` /
  `data-ui`. Layers: only `--z-*` tokens from `app.css` (`src/lib/zindex.test.ts` fails on a bare `z-index: N` or
  `z-N` class in `src/lib` / `routes/(game)`); each region is a stacking context, inner z uses `--z-local-*`. The
  top layer belongs to `GameWindow` (`<dialog>`), popovers (HUD menu `popover="auto"`, FxLayer, Toaster, FpsOverlay —
  manual popovers re-shown when they need to be above a window); its order = opening order. Windows prefetch their
  chunk (`prefetchWindow(kind)` on hover/focus/pointerdown of the opener; common ones idle after boot). Tu vi has one
  display value: `idleClock` (`game/idle.svelte.ts`, pure `game/qi.ts`) through `CountUp` — HUD and pages never
  interpolate on their own. Budgets: INP ≤ 200 ms, no long task > 50 ms on main interactions, CLS ≈ 0 in `/play`
  (the scene box hides while it moves under the transition cover).
- **Less text.** Rules/explanations go to `lib/game/help.ts` behind a `<HelpButton topic>` ("?"); empty lists use
  `components/EmptyState.svelte` with an action hint.
- UI text is Vietnamese; keep strings in components for now (i18n later), no hardcoded numbers from game config.
- Styling via tokens in `src/app.css` — see `graphic-design.md`.
- Before finishing: `npm run check && npm test && npm run build`.
