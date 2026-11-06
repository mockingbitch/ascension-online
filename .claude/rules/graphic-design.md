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
- **No raw hex/rgb in components.** Add a token first. Inline `style:` may reference `var(--color-…)`.

## Components
- Reuse `RealmBadge`, `QiBar`; new primitives go to `src/lib/components/` with typed `Props`.
- Semantic colour use: jade = can act / progress; gold = achieved / premium-free prestige; cinnabar = failure,
  never for VIP upsell. VIP visuals must not imply power advantage.
- Numbers: format via `format.ts` (vạn/ức, %); never show raw milli/bp values.

## Motion
- Durations 150–700ms, easing `--ease-qi`. Ambient loops (`animate-qi-breathe`) only on 1–3 focal elements.
- Major breakthrough celebration ≤ 3s, skippable, then settle to static state.
- All motion disabled under `prefers-reduced-motion` (global rule in app.css — don't override).
- Animate `transform`/`opacity` only; avoid animating layout/box-shadow on many nodes.

## Accessibility
- Contrast ≥ 4.5:1 for text (mist-100/300 on ink-900/950 OK; check gold on ink for small text).
- Never convey state by colour alone — pair with text/icon (e.g. "Bình cảnh — có thể đột phá").
- Progress bars use `role="progressbar"` + aria values. Focus ring = gold outline (global).
- Layout works from 360px width; tap targets ≥ 40px.

## Assets
- Icons/badges: inline SVG using `currentColor`, viewBox 0 0 24 24 (icons) or 64 64 (emblems), optimized (no editor
  metadata). Raster art: WebP/AVIF, sized ×1 and ×2, stored in `static/` (later R2).
- Don't imitate a specific existing game's art, logos or UI; the style must be original.
