---
paths:
  - "apps/web/src/**/*.svelte"
  - "apps/web/src/app.css"
  - "apps/web/static/**"
---

# Graphic design rules — "Mực tàu & Ngọc bích"

## Visual direction
Thủy mặc (ink-wash) meets modern UI: dark ink surfaces, jade for action/progress, gold for achievement/prestige,
cinnabar (chu sa) for danger/failure. Calm, spacious, readable; effects are subtle "linh khí" glows, not neon.

## Tokens (single source: `apps/web/src/app.css` `@theme`)
- Colours: `ink-*` surfaces, `mist-*` text, `jade-*` primary, `gold-*` prestige, `cinnabar-500` danger (`cinnabar-300` for failure text), `spirit-cyan`/`spirit-violet` for mystical effects. Panels/buttons: `.panel`, `.panel-title`, `.btn .btn-jade|.btn-gold|.btn-ghost`, `.eyebrow` in app.css.
- Realm aura: `--color-aura-<visual.aura>` from `realms.yaml` (none, mist, jade, golden, azure, violet, void,
  crimson, radiant, thunder, celestial). Higher realm = more saturated / luminous.
- Rarity: `--color-rarity-<rarity>` (pham → linh → huyen → dia → thien → tien).
- Fonts: `font-display` (Cormorant Garamond) for titles & realm names; `font-sans` (Be Vietnam Pro) for UI & numbers
  (`tabular-nums` for counters). Both fully support Vietnamese diacritics — check any new font does too.
- **No raw hex/rgb in components.** Add a token first. Inline `style:` may reference `var(--color-…)`. Sole exception:
  `static/manifest.webmanifest`, `<meta name="theme-color">` and the generated PWA icons (they cannot read CSS
  variables) mirror `--color-ink-950` — a test keeps them equal.

## Components
- Reuse `RealmBadge`, `QiBar`; new primitives go to `src/lib/components/` with typed `Props`.
- Semantic colour use: jade = can act / progress; gold = achieved / premium-free prestige; cinnabar = failure,
  never for VIP upsell. VIP visuals must not imply power advantage.
- Numbers: format via `format.ts` (vạn/ức, %); never show raw milli/bp values.

## Motion
- Tokens (app.css): `--dur-instant` 90ms (press), `--dur-fast` 160ms (tab/hover), `--dur-base` 240ms (window),
  `--dur-slow` 420ms (reward fly, count-up), `--dur-scene` 650ms (scene transition), `--dur-window` 220ms (window
  open), `--stagger-step` 30ms (list stagger, max 10 — put `.stagger` on the list); easings `--ease-qi` (default),
  `--ease-out-back` (pop/overshoot: badges), `--ease-in-out-sine` (camera, breathing). Use tokens, not literal ms.
- Every effect has three levels from Cài đặt (`settings.fx`): **Đầy đủ** (full), **Nhẹ** (lite: fewer particles,
  shorter/smaller motion), **Tắt** (off: static end state, no decorative motion). Default = Tắt under
  `prefers-reduced-motion`, else Đầy đủ. Information must never depend on an animation playing.
- Particles/auras: Canvas2D `ParticleSystem` only, colours from tokens (`var(--color-aura-*)`, `--color-rarity-*`,
  jade/cyan for qi), additive blend, ≤ 300 live (≤ 80 Nhẹ); counts scale with log of the server value (e.g.
  `motesForEarned`), never with client-computed amounts. Aura colour follows the realm `visual.aura`.
- Durations 150–700ms, easing `--ease-qi`. Ambient loops (`animate-qi-breathe`) only on 1–3 focal elements.
- Major breakthrough celebration ≤ 3s, skippable, then settle to static state. Breakthrough / lôi kiếp beats come
  from `engine/fx/breakthrough.ts` (minor: gather 0.8 s → flash → aura + stage; major: formation spin 1.2 s → pillar →
  calligraphy name → clouds part; lôi kiếp: darken → 3 strikes with flash + light shake; failure: frosted crack +
  200 ms shake + the server's loss text). Lite = ×0.6 time, fewer particles, no shake; off = static card; reduced
  motion = never flash/shake. Mode results carry a badge with icon + text (never colour alone).
- All motion disabled under `prefers-reduced-motion` (global rule in app.css — don't override).
- Animate `transform`/`opacity` only; avoid animating layout/box-shadow on many nodes.
- Interaction: press = scale 0.96 (`.btn`, `.press`), desktop hover = lift 2px + faint gold border, disabled never
  animates. CSS that depends on the effects level reads `:root[data-fx='full'|'lite'|'off']` (set by the shell).
- `data-fx='off'` has a global rule in app.css (like reduced motion): every CSS animation/transition jumps to its end
  state. So write animations whose **end state is the information** (never base `opacity: 0` revealed only by an
  animation), and still gate loops per level (`:global(:root[data-fx='full']) .x { animation: … }`).
- New badges: `.badge-pop` (0.6 → 1.1 → 1 + glint), re-keyed (`{#key count}`) to replay. Number changes: `CountUp`
  (jade flash on increase = a jade copy fading, opacity only). Reward/announcement/float visuals live in
  `engine/fx/FxOverlay.svelte`; colours of canvas particles are token strings in `engine/fx/presets.ts`.

## Sound
- Sound mirrors the visuals: cues come from the same event and timeline (thunder on each lôi kiếp strike, the outcome
  on the reveal). One cue per event (a reward = its rarest item), never one per particle. Calm and soft by default —
  bells, jade, wood, wind; no harsh alarms. Information never depends on sound (audio is off until a tap, and muted
  players see everything). Original/licensed only (`lib/assets/audio/README.md`).

## Accessibility
- Contrast ≥ 4.5:1 for text (mist-100/300 on ink-900/950 OK; check gold on ink for small text).
- Never convey state by colour alone — pair with text/icon (e.g. "Bình cảnh — có thể đột phá").
- Progress bars use `role="progressbar"` + aria values. Focus ring = gold outline (global).
- Layout works from 360px width and in landscape phones (812×375) and tablets (768/1024).
- Tap targets ≥ 44×44 px on touch (`--tap`), ≥ 8 px apart (`--tap-gap`); icon-only buttons use `var(--tap)` boxes.
  A control too small to grow (a tiny chip) moves somewhere it can be a real target (phones: Tiên Duyên via "Thêm").
- **Sending state:** a button that sends a request shows it at once (< 100 ms): disabled + `aria-busy` + a small
  spinner/dim (the primary action has `busy`); the result still appears only after the server answers.

## Layers
- One z scale in `app.css` (`:root`): `--z-scene` 0 < `--z-scene-ui` 10 < `--z-stage` 20 < `--z-chat` 30 <
  `--z-bar` 40 < `--z-hud` 50 < `--z-wipe` 60 < `--z-mode` 70 < `--z-toast` 80 < `--z-boot` 90; above them the top
  layer (windows, HUD menu, FxLayer, toasts, FPS overlay). Inside a component/region: `--z-under`, `--z-local-1..3`.
  No literal z numbers anywhere else (a test fails on them); depth sorting from data (hall seats by scene y) is fine.

## Assets
- Icons/badges: inline SVG using `currentColor`, viewBox 0 0 24 24 (icons) or 64 64 (emblems), optimized (no editor
  metadata). Raster art: WebP/AVIF/PNG, ×1 and `@2x`, in `apps/web/src/lib/assets/art/` (slots + SVG fallback via
  `lib/game/art.ts`; list in its README, prompts in docs/design/art-prompts.md). Later R2.
- Don't imitate a specific existing game's art, logos or UI; the style must be original.
