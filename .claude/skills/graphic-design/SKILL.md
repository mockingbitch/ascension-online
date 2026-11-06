---
name: graphic-design
description: Design and build Tiên Đạo visuals in the web app - design tokens, Svelte UI components, realm badges and aura effects, breakthrough celebration, SVG icons/emblems, layout of game panels. Use for any UI look-and-feel or visual asset work.
---

# Graphic design

Rules: `.claude/rules/graphic-design.md` (palette, tokens, motion, a11y) and `.claude/rules/frontend.md`.

## Workflow
1. **Intent**: what the player must understand in < 2 seconds (state, next action). Write it in one sentence.
2. **Hierarchy**: one focal element per panel (usually realm badge or qi bar), one primary action (jade button).
3. **Tokens**: use existing tokens; if missing, add to `@theme` in `apps/web/src/app.css` with a semantic name
   (`--color-aura-thunder`, not `--color-blue-2`). Then use as Tailwind utility (`bg-aura-thunder`) or `var()`.
4. **Build** a component in `apps/web/src/lib/components/<Name>.svelte`:
   - Svelte 5 runes, typed `interface Props`, sensible defaults, no fetches inside primitives
   - CSS/SVG only; animate transform/opacity; reduced-motion respected (global)
5. **Check**: `npm run check && npm run build`; view at 360px and desktop; text contrast ≥ 4.5:1.

## Recipes

**Aura glow** (realm-colored breathing halo):
```svelte
<span class="absolute inset-0 -z-10 animate-qi-breathe rounded-full blur-md"
      style:background="color-mix(in oklab, var(--color-aura-{aura}) 25%, transparent)"></span>
```

**Major breakthrough celebration** (≤ 3s, skippable): full-screen overlay `bg-ink-950/80`, realm name in
`font-display text-5xl` with aura color, a radial gradient burst via `@keyframes` scale 0.6→1.4 + opacity 1→0,
then static badge. Close on click/Esc; `aria-live="polite"` announces "Đột phá thành công: <realm>".

**SVG icon**: 24×24 viewBox, `stroke="currentColor"`, `stroke-width="1.5"`, round caps/joins, no fills except
accents, ≤ 1KB, `aria-hidden="true"` when decorative (pair with text).

**Sect emblem**: 64×64, circular frame + one central glyph-like motif (original, not real calligraphy of a
trademark), 2 colours max from tokens.

## Art direction prompts (for external image generation / artists)
"Original ink-wash (thủy mặc) xianxia illustration, misty mountains, jade and gold accents on deep ink background,
soft qi glow, minimal, no text, no logos" — append the subject. Never request imitation of a named game/studio/artist.

## Performance
Many animated nodes → prefer one CSS animation on a container or a single SVG with `<animate>` over N elements.
No canvas/PixiJS in MVP.
