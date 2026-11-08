# render3d — reserved for the GĐ7 spike (no code, no dependency)

Status: **not started**. `frontend.md` forbids 3D engines until the project owner records a GĐ7 decision in
`docs/decisions.md`. This file is the proposal that decision would approve or change.

## Contract

A 3D renderer implements the same `SceneRenderer` interface as `render2d/Render2D.ts` (`../types.ts`) and draws the
same `SceneDef` data (`../../scene/types.ts`). Nothing outside `lib/engine/render/` may know which renderer is active:

- UI, FX and chat bubbles ask `anchorToScreen(id)`; a 3D renderer projects the anchor (`x, y` plus optional `z`)
  through its camera. Anchors stay normalised to the scene frame so the 2D def keeps working.
- `FxLayer` (canvas particles) stays in screen space and is reused unchanged.
- `setCamera(preset, ms)` maps `CameraPreset` to a camera position/target; `pause()`/`resume()` stop the render loop
  (subscribe to `engine/loop.ts`, never a second rAF).
- Selected by the `renderer` setting / `?renderer=3d` per scene, with 2D as the fallback when WebGL2 is missing or
  the device is weak.

## Proposed stack

- Three.js through **Threlte** (Svelte-native), dynamically imported only when `renderer=3d` — zero bytes for 2D users.
- Assets: glTF 2.0 (`.glb`) with **KTX2/Basis** textures and **Draco/meshopt** geometry; environment `env.hdr` (small,
  prefiltered). Per scene: `assets/scenes/<id>/model.glb`, `env.hdr`; characters `characters/<id>.glb` with clips
  `idle`, `meditate`, `cast`, `breakthrough`.

## Budgets (mid-range Android)

| Item | Budget |
|---|---|
| Triangles on screen | ≤ 150k (scene) + ≤ 20k per character |
| Draw calls | ≤ 120 |
| Textures | ≤ 48 MB GPU, 2048² max, KTX2 |
| JS added (lazy chunk) | ≤ 250 kB gz, never in the initial `/play` bundle |
| Frame rate | 60 fps idle, ≥ 45 fps during big effects (same as 2D) |

## Spike steps

1. Behind `renderer=3d`, render Tử Tiêu Cung only: static hall model + Hồng Quân placeholder + seat anchors projected.
2. Verify `anchorToScreen` parity with the 2D def (bubbles, qi stream origin) at 375 px and 1440 px.
3. Measure: bundle (lazy chunk), fps idle/effects, battery drain over 10 min, memory (DevTools heap) on Android + laptop.
4. Fallback: no WebGL2 / context lost / fps < 40 for 5 s → switch the scene back to 2D and remember it.
5. Write the result and the go/no-go into `docs/decisions.md`.

## Risks

Bundle size and first-load time; battery and heat on phones; weak/old GPUs and context loss; art pipeline cost
(3D assets must be original or licensed — never copied from existing games); accessibility (reduced motion must
still mean a static scene).
