# Raster art slots

Drop a file at the path below and the game uses it on the next build or dev reload. No code change is needed.
Anything missing falls back to the original SVG art. Prompts and art direction are in
[docs/design/art-prompts.md](../../../../../../docs/design/art-prompts.md).

- **Formats:** `.avif`, `.webp` or `.png`, tried in that order.
- **Retina:** add `name@2x.ext` next to `name.ext` and it is used through `srcset`.
- **Transparency:** layers 2–4, landmarks, characters and items need an alpha channel.
- **Names:** lowercase, exactly as listed. Item, realm and faction IDs come from `content/catalog/*.yaml` and the
  faction drafts.

| File | Size (1x) | Used in | Notes |
|---|---|---|---|
| `world/sky` | 1920×1080 | Thế Giới map, login | Layer 1. When this file exists the map switches to parallax layers |
| `world/far` | 1920×1080 | Thế Giới map | Layer 2, alpha |
| `world/mid` | 1920×1080 | Thế Giới map | Layer 3, alpha |
| `world/ground` | 1920×1080 | Thế Giới map | Layer 4, alpha, leave open ground where the landmarks sit |
| `world/map` | 1920×1080 | Thế Giới map | Single-image option, used only when there is no `world/sky` |
| `landmarks/tu-tieu-cung` | 512×512 | map (layered mode) | Placed above the Tử Tiêu Cung plaque |
| `landmarks/dong-phu` | 512×512 | map (layered mode) | |
| `landmarks/the-luc` | 512×512 | map (layered mode) | |
| `landmarks/dao-hoa-dinh` | 512×512 | map (layered mode) | |
| `landmarks/bi-canh` | 512×512 | map (layered mode) | Greyed out automatically while locked |
| `landmarks/chien-truong` | 512×512 | map (layered mode) | Greyed out automatically while locked |
| `characters/avatar` | 256×256 | every Portrait (HUD, chat card, ranking) | Cropped to a circle |
| `characters/meditate` | 1024×1024 | Tu Luyện, Đột Phá scene | Replaces the SVG figure and formation; the art includes its own rune array |
| `characters/body-<realm_id>` | 1024×1536 | Nhân Vật | e.g. `body-luyen_khi`; falls back to the closest lower realm, then to the portrait |
| `characters/<body_id>/avatar` | 256×256 | every `Portrait` of a character with that body (HUD, chat card, Thiên Bảng, thế lực, đạo lữ, hồ sơ) | **Per body** (`appearances.yaml` ids, e.g. `characters/tien_nu_y_su/avatar`). Lazy-loaded. Fallback: the parameterised SVG `CultivatorFigure` of the look — never another body's art. P0 (GĐ-E) |
| `characters/<body_id>/avatar-robe` | 256×256 | `Portrait` | Optional grey mask (alpha) of the robe area: the robe colour token is blended over it (`mix-blend-mode: color`). Fallback: the painted robe colour. P1 |
| `characters/<body_id>/meditate` | 1024×1024 | Động Phủ, Đột Phá scene, look preview | Alpha; includes its own rune array. Fallback: SVG figure (pose, robe, hair) + formation. P0 |
| `characters/<body_id>/meditate-<pose_id>` | 1024×1024 | same | Pose variant (e.g. `meditate-ban_gia`), tried before `meditate`. P2 |
| `characters/<body_id>/body-<realm_id>` | 1024×1536 | Nhân Vật, realm achievement card | Alpha. Optional painting of one exact realm; wins over the tier painting below |
| `characters/<body_id>/body-b<1..6>` | 1024×1536 | Nhân Vật, realm achievement card | Alpha. Full body per **effect tier** (`realms.yaml visual.fx_tier`, decision 2026-11-09 #3): robes and presence grow with the tier. Chain: `body-<realm_id>` → `body-b<tier>` → lower tiers → lower realms → SVG figure. Lazy-loaded. P1 (b1–b3 first) |
| `characters/<body_id>/listener` | 256×256 | Tử Tiêu Cung seats | Alpha, back view on a cushion. Fallback: a small still SVG figure of the look. P2 |
| `characters/<body_id>/<outfit_id>/{avatar, meditate, meditate-<pose_id>, listener, body-b1 … body-b6}` | same sizes as the body slots above (256² · 1024² · 1024² · 256² · 1024×1536) | everywhere the body slots are used | **Per outfit** (decision 2026-11-09 "Bộ trang phục vẽ sẵn"): each body has 3 painted outfits in `appearances.yaml` (`outfits[]`: robe colour + hair accessory baked in, brief in `art`). The **default outfit** (`default_outfit`) IS the body folder above — no copy, no sub-folder. The 2 alternates get a sub-folder named by the outfit id, e.g. `characters/tien_nu_y_su/y_su_tu_yen/avatar`. Same rules as the body slots: alpha except avatar, `meditate` = the body's `default_pose`, `meditate-<pose>` for every other pose of the path, optional `@2x`. Chain in game: outfit folder → body folder (default outfit; `avatar-robe` mask recolours it to the outfit's robe) → SVG figure in the outfit's colours. The look picker (onboarding "Ngoại hình", Nhân Vật › Trang phục) uses only the exact outfit's art (else the SVG), so every choice visibly changes the preview. Lazy-loaded (same index). Per outfit: 1 avatar + 3 (tiên) / 2 (Vu) meditate + 1 listener + 6 body = 11 / 10 images. P0 avatar + meditate, P1 body-b1–b3, P2 the rest |
| `characters/acc/<hair_id>` | 128×128 | (reserved) | Alpha hair-accessory overlay for raster bodies; placed at a per-body head anchor added with the first painted bodies (GĐ-E). Until then accessories show on the SVG figure only. P1 |
| `fx/aura-<1..6>` | 512×512 | Động Phủ / Đột Phá scene figure (persistent realm aura) | Optional alpha overlay per effect tier, centred on the figure; replaces the SVG/CSS layers of `RealmAura` (ring, nascent shadow, sparks, clouds, star orbit). Comes from the realm only — never sold. P2 |
| `characters/guide/<expr>` | 768×1024 | Vân Khê's panel (tutorial, "?" summon, onboarding) | Alpha, half body, the guide only (`content/story/characters/van-khe.md`): `neutral`, `smile`, `point`, `worried`, `proud`, `surprised`. Same framing in all six (head at the same height) so lines swap without a jump. Lazy-loaded. Fallback: SVG `GuideFigure`. Character sheet first. P0 (GĐ-E) |
| `characters/guide/blink` | 768×1024 | same | Optional: the `neutral` pose with eyes closed, flashed for a blink at effects full only. P2 |
| `items/<item_id>` | 128×128 | Túi Đồ | e.g. `items/tu_linh_phu`, `items/truc_co_dan` |
| `items/linh_thach` | 128×128 | Túi Đồ | Spirit-stone currency |
| `factions/<faction_id>` | 512×512 | faction cards, Thế Lực | e.g. `factions/vu_toc`; replaces the SVG emblem. IDs are the permanent legacy slugs: `huyen_khung_mon` = Tam Thanh, `kim_sa_mon` = Tây Phương, `thuong_lan_dinh` = Tiên Đình, `tinh_ha_yeu_dinh` = Thiên Đình Yêu Tộc, `thu_nhat_vien`/`lap_cuc_vien`/`van_luu_vien` = Nhân/Xiển/Tiệt Giáo, `kim_sa_giao` = Tây Phương Giáo (full table in `factions.yaml`). Generic motifs only, no worship iconography |
| `ui/splash` | 1080×1920 | login on portrait screens | 9:16 key art; landscape screens keep the map |
| `scenes/dong-phu` | 1600×1200 (4:3) | Động Phủ scene backdrop (GĐ2 slot, optional) | Cave interior behind the meditating cultivator; floor line at the bottom edge, centre left open. Until it exists the scene shows a jade glow |
| `scenes/tu-tieu-cung` | 1920×1080 | Tử Tiêu Cung hall | Interior; dais top-centre for Hồng Quân, open floor below for the seat grid. No cushions painted in (seats are drawn by the game) |
| `characters/hong-quan` | 768×768 | Tử Tiêu Cung hall | Alpha. Hồng Quân seated cross-legged, front view, sits on the dais |
| `characters/listener` | 256×256 | Tử Tiêu Cung seats | Alpha. One disciple seen from behind, seated on a cushion; reused for every seat |
| `cosmetics/<slug>` (avatar frame) | 320×320 | every `Portrait` with an equipped frame, Trang phục tab | Alpha ring only, transparent centre; drawn at 128% of the portrait so it sits around the circle. e.g. `cosmetics/ngoc_bich`. Fallback: CSS ring (`.frame-*` in app.css) |
| `cosmetics/<slug>` (cave decoration) | 256×256 | Động Phủ scene (4 slots: 2 wall, 2 floor), Trang phục tab | Alpha, object only, front view, soft ground shadow. e.g. `cosmetics/binh_phong_tuyet`. Fallback: `DecorationGlyph` SVG |
| `cosmetics/<slug>` (faction decoration) | 256×256 | Thế Lực → Công trình | Alpha, e.g. `cosmetics/su_tu_da`, `cosmetics/bai_phuong` (slugs from `faction_works.yaml`). Fallback: `DecorationGlyph` SVG |

**Per-body folders load lazily.** `characters/<body_id>/…` and `characters/<body_id>/<outfit_id>/…` (and `characters/acc/…`, `characters/guide/…`) are NOT in
the eager glob of `lib/game/art.ts`; `lib/game/charArt.svelte.ts` fetches their URL index on first use, so neither the
files nor their URLs are in the initial `/play` bundle. Body ids, accessories, robe colours and poses come from
`content/catalog/appearances.yaml` (decisions 2026-11-09 "Ngoại hình…"); make one character sheet per body first
(`docs/design/art-prompts.md` §4). The shared top-level `characters/avatar|meditate|listener|body-*` slots are only
used for data without a look.

**Painted art (phase E, 2026-11-09, Meta Muse Image; `tools/artgen/artgen.py`, refs in `art-work/refs/`; prompts in
`docs/design/art-prompts.md` §4 "Bộ ảnh theo dáng người" / "Vân Khê" / "Hào quang"):**

| Set | Files |
|---|---|
| All 12 bodies | `avatar` (+@2x), `meditate`, `listener` (+@2x), `body-b1` … `body-b6` |
| Alternate outfits | all 24 (2 per body): `characters/<body_id>/<outfit_id>/{avatar (+@2x), meditate, meditate-<pose>, listener (+@2x), body-b1…b6}`; refs `art-work/refs/<body_id>-<outfit_id>-sheet.png`. Missing: `tien_nu_y_su/y_su_hoang_tho/body-b6` (falls back to b5) |
| Pose variants | every body has art for every pose of its path: `meditate` = the body's `default_pose`, `meditate-<pose>` for the others (tiên: `kiet_gia`, `ban_gia`, `kiem_ngang`; Vu: `trung_binh_tan`, `toa_thach`). No `meditate-<default>` copy is needed: `meditateSlots()` falls back to `meditate` |
| `avatar-robe` | `tien_nu_tieu_thu`, `tien_nam_cong_tu`, `vu_nu_tho_san`, `vu_nam_chien_si` only. The others are skipped because their robe can't be separated from the teal background or the skin by colour, so their robe colour stays the painted one |
| `characters/guide/` | `neutral`, `smile`, `point`, `worried`, `proud`, `surprised`, `blink` (`blink` = `neutral` with the eye band of a closed-eyes render). Look = **option C2** (owner's pick, 2026-11-09): opaque strapless smoky-violet silk gown with a sweetheart neckline, embroidered violet-and-gold belt with waist chain and tassels, gold arm cuff, jade bracelets, jade-and-gold drop earrings, gold necklace with jade pendant, gold buyao beside the peach-wood pin, small beauty dot between the brows, black shawl at the elbows; adult, non-explicit. Ref `art-work/refs/van_khe-sheet-c2.png` (+ `van_khe-c2-neutral.png` for framing). Rollback: `art-work/old-guide-c/` (option C), `art-work/old-guide/` (original modest set) |
| `fx/` | `aura-1` … `aura-6` (+@2x), made on black with alpha from luminance; tier 6 holds the faint pháp tướng (starlight silhouette, generic) |

`meditate` / `body-*` / `guide/*` have no @2x (the source is 1600² / 1280×1920, below 2× the slot). The pilot
`body-luyen_khi` became `body-b1` (the tier chain covers it).

Cosmetic slugs come from `content/catalog/cosmetics.yaml` / `faction_works.yaml` (unique across both files, loader-checked).
Titles and chat name effects are text styles, not images. Placeholders (CSS/SVG) ship today; no raster cosmetics yet.

## Scenes (engine, GĐ1+)

Scene data lives in `src/lib/engine/scene/defs/<id>.ts` (`SceneDef`): each layer names an art slot above (e.g. the
world uses `world/sky|far|mid|ground`, else `world/map`; Tử Tiêu Cung uses `scenes/tu-tieu-cung`; Động Phủ
`scenes/dong-phu`), and positions are `anchors` normalised 0..1 in the scene's art frame — never pixels. Since GĐ2 the
SceneLayer draws them; characters and props (`characters/meditate`, `characters/hong-quan`, `characters/listener`,
`landmarks/*`) are placed by the scene views (`scene/views/*View.svelte`) at anchors. Depth-0 layers sit exactly on
the frame (anchors match the painting); depth > 0 layers get a 3% parallax overscan, so keep their edges disposable. Future per-scene folders follow
`scenes/<id>/layers/*.webp` (2D) with room for `model.glb` / `env.hdr` (3D, see
`src/lib/engine/render/render3d/README.md`); characters reserve `characters/<id>.glb`.

## PWA / app icons (GĐ6)

`static/icons/icon-192.png`, `icon-512.png`, `icon-maskable-512.png` (mark inside the 80 % safe zone on
`--color-ink-950`) and `apple-touch-icon.png` (180) are **generated** from the logo `static/favicon.svg` by
`node apps/web/tools/make-icons.mjs` — re-run it when the logo changes (the script refuses if the logo's shapes no
longer match what it draws). Do not hand-edit them; a future painted app icon would replace the script's output.
