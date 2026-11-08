# render3d — the GĐ7 3D spike (Tử Tiêu Cung behind `renderer=3d`)

Status: **spike done (2026-11-08)**, approved by the owner the same day (`docs/decisions.md` "GĐ7 3D spike approved by
owner 2026-11-08"); result and go/no-go in the decision "GĐ7 spike result". 2D stays the default. 3D code may live only in
this folder and is reached only through the dynamic import in `../index.ts` (`.claude/rules/frontend.md`).

## How to try it

- URL: `/play/tu-tieu-cung?renderer=3d` (`?renderer=2d` forces 2D). Only Tử Tiêu Cung has a 3D version; every other
  scene stays 2D under the flag.
- Cài đặt › Hiển thị › Kết xuất: "2D (mặc định)" / "3D thử nghiệm (Tử Tiêu Cung)". Persisted in
  `tiendao.settings.v1` (`renderer: '3d'`; absent = 2D).
- `?fpsguard=0` turns off the fps watchdog (headless tests on software GL only).

## Files

| File | What |
|---|---|
| `Render3D.ts` | `SceneRenderer` in plain Three.js: canvas in the scene box, camera from `layout.ts`, `gameLoop()` rendering, quality, pause, fallbacks, disposal |
| `hall.ts` | procedural hall (floor platform, pillars, facade + eaves, dais, Hồng Quân placeholder, instanced cushions on the seat anchors, cloud sprites, fog, gold key light) |
| `mapping.ts` | pure anchor ↔ 3D math (reference-camera rays, projection, presets → view offset, drift) — unit-tested |
| `layout.ts` | reference camera + hall dimensions |
| `palette.ts` | token → sRGB for materials (read at mount through the cascade) — unit-tested |
| `watchdog.ts` | pure fps watchdog (< 40 fps for 5 s after a 2 s grace) — unit-tested |
| `assets.ts` | asset slots: `src/lib/assets/scenes/<id>/model.glb`, `env.hdr`, `src/lib/assets/characters/<id>.glb` |
| `failure.ts` | `RendererFailure(reason)` thrown by `mount` |

Selection lives outside (initial bundle, a few hundred bytes): `../select.ts` (`rendererFor(scene, prefs)`) and
`settings.rendererFor()`; `SceneLayer.svelte` swaps renderers per scene and handles fallbacks.

## Contract (unchanged `SceneRenderer`)

- **Anchors.** The reference camera's full image *is* the def's 16:9 frame, placed like the 2D frame (`frameRect`,
  cover/centre-top) via `PerspectiveCamera.setViewOffset`; presets map through `viewRect` (= the 2D
  `cameraTransform`). A normalised anchor is the ray through `(x, y)` meeting the plane `y = anchor.z` (height above
  the floor in hall units, default 0). The hall is **built at those points** (cushions on `seat.*`, the sage on
  `hong_quan`, `dais.z` puts the qi source on his lap), so: at rest `anchorToScreen` equals the 2D result exactly
  (unit test + measured ≤ 0.5 px), and with the idle drift the anchors follow the 3D objects. Layout listeners are
  told when a probe anchor moved ≥ 0.5 px (≈ 3–4 times a second during drift), not every frame.
- **HallView** keeps the DOM listeners, name cards, bubbles and the qi stream; in 3D it hides the DOM sage (kept
  invisible as a measurable point) and the DOM cushions of seats the def knows. FxLayer and the scene particle field
  are unchanged (screen space).
- **One loop.** `renderer.setAnimationLoop(null)`; frames come from `gameLoop()` only while visible and not paused.
- **Quality.** full: drift + cloud motion + shadows (PCF 1024²), DPR ≤ 2, 18 clouds. lite: DPR ≤ 1.5, 7 clouds, no
  shadows, smaller drift. off / reduced motion: static — one frame per layout change (resize, preset), no drift.
- **Pause.** A window/mode covering the scene → `pause()` → off the loop (last frame stays on screen).
- **Fallback.** No WebGL2 (`mount` throws) / `webglcontextlost` / watchdog trip → `settings.recordFallback3d(reason)`,
  toast "Thiết bị không hỗ trợ 3D, đã chuyển về 2D", the scene re-mounts in 2D; the fallback is remembered until the
  player picks 3D again in Cài đặt (which shows the reason).
- **Disposal.** `unmount` disposes every geometry/material/texture (incl. anything a slot loaded), the shadow map and
  render lists, then `forceContextLoss()` so contexts never pile up (20× mount/unmount measured).

## Stack choice

Plain Three.js inside a class, **not Threlte**: the contract is an imperative class (`mount`/`unmount`/`projectPoint`)
driven by the shared loop; Threlte brings its own component tree and render scheduler (`useTask`) that we would have to
switch off, plus extra bytes, for no gain at this size. Revisit if 3D scenes grow into many interactive components.

## Budgets vs measured (spike) — see decisions.md for the numbers

| Item | Budget | Spike |
|---|---|---|
| Triangles | ≤ 150k + 20k/character | ~21k (full, with shadow pass) |
| Draw calls | ≤ 120 | ~66 (lite/off); full adds the shadow pass |
| Textures | ≤ 48 MB, KTX2 | one 64² canvas puff (+ 1024² shadow map at full) |
| Lazy JS | ≤ 250 kB gz | three r187 + renderer: 151 kB gz (611 kB raw), one chunk |
| Initial `/play` JS | ≤ +1 kB gz | +0.9 kB gz (flag check, settings, SceneLayer swap) |
| fps | 60 idle | not measurable headless (SwiftShader); real devices: owner |

## Next steps (not in the spike)

1. Real art: an original/licensed `model.glb` for the hall and `characters/hong-quan.glb` (clips `idle`, `meditate`,
   `cast`, `breakthrough`) — the slots already load them (`assets.ts`) and fall back to the procedural hall.
2. Asset pipeline: gltf-transform → KTX2/Basis textures (+ `KTX2Loader` with the Basis transcoder served from
   `static/`), Draco or meshopt geometry (`DRACOLoader`/`MeshoptDecoder`) — lazy, only when a file uses them; a small
   prefiltered `env.hdr`.
3. Instanced listener figures in 3D (today DOM sitters over 3D cushions) and 3D speech bubble anchors on heads.
4. Real-device measurements: mid-range Android + laptop, idle/effects fps, battery over 10 min, heap.
5. Other scenes (Động Phủ, Thế Giới) only after the hall passes on real devices.

## Risks (observed)

Bundle (151 kB gz lazy is fine on Wi-Fi, ~1 s more on 3G for players who opt in); battery/heat (continuous rendering
at full — lite/off reduce it, off renders only on layout); weak/old GPUs and context loss (handled by the fallback, but
a device that is slow only sometimes may flip to 2D); art pipeline cost (3D assets must be original or licensed);
accessibility (reduced motion = static render, verified).
