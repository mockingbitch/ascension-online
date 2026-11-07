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
| `items/<item_id>` | 128×128 | Túi Đồ | e.g. `items/tu_linh_phu`, `items/truc_co_dan` |
| `items/linh_thach` | 128×128 | Túi Đồ | Spirit-stone currency |
| `factions/<faction_id>` | 512×512 | faction cards, Thế Lực | e.g. `factions/vu_toc`; replaces the SVG emblem. IDs are the permanent legacy slugs: `huyen_khung_mon` = Tam Thanh, `kim_sa_mon` = Tây Phương, `thuong_lan_dinh` = Tiên Đình, `tinh_ha_yeu_dinh` = Thiên Đình Yêu Tộc, `thu_nhat_vien`/`lap_cuc_vien`/`van_luu_vien` = Nhân/Xiển/Tiệt Giáo, `kim_sa_giao` = Tây Phương Giáo (full table in `factions.yaml`). Generic motifs only, no worship iconography |
| `ui/splash` | 1080×1920 | login on portrait screens | 9:16 key art; landscape screens keep the map |
| `scenes/tu-tieu-cung` | 1920×1080 | Tử Tiêu Cung hall | Interior; dais top-centre for Hồng Quân, open floor below for the seat grid. No cushions painted in (seats are drawn by the game) |
| `characters/hong-quan` | 768×768 | Tử Tiêu Cung hall | Alpha. Hồng Quân seated cross-legged, front view, sits on the dais |
| `characters/listener` | 256×256 | Tử Tiêu Cung seats | Alpha. One disciple seen from behind, seated on a cushion; reused for every seat |
| `cosmetics/<slug>` (avatar frame) | 320×320 | every `Portrait` with an equipped frame, Trang phục tab | Alpha ring only, transparent centre; drawn at 128% of the portrait so it sits around the circle. e.g. `cosmetics/ngoc_bich`. Fallback: CSS ring (`.frame-*` in app.css) |
| `cosmetics/<slug>` (cave decoration) | 256×256 | Động Phủ scene (4 slots: 2 wall, 2 floor), Trang phục tab | Alpha, object only, front view, soft ground shadow. e.g. `cosmetics/binh_phong_tuyet`. Fallback: `DecorationGlyph` SVG |
| `cosmetics/<slug>` (faction decoration) | 256×256 | Thế Lực → Công trình | Alpha, e.g. `cosmetics/su_tu_da`, `cosmetics/bai_phuong` (slugs from `faction_works.yaml`). Fallback: `DecorationGlyph` SVG |

Cosmetic slugs come from `content/catalog/cosmetics.yaml` / `faction_works.yaml` (unique across both files, loader-checked).
Titles and chat name effects are text styles, not images. Placeholders (CSS/SVG) ship today; no raster cosmetics yet.
