# Bộ prompt ảnh 2D tiên hiệp — Tiên Đạo

Bộ prompt để tạo ảnh 2D cho bản đồ, menu/UI, vật phẩm và nhân vật, đồng bộ với palette hiện tại của game (xanh ngọc, vàng kim, nền tối).

> **Tích hợp vào game:** xem [mục 7](#7-tích-hợp-vào-game-đối-chiếu-repo). Tóm tắt: thả file vào
> `apps/web/src/lib/assets/art/<đường dẫn>` đúng tên trong bảng là game tự dùng; thiếu file thì giữ hình SVG hiện có.

## Cách dùng

Mỗi prompt được ghép theo công thức:

```
[STYLE CHUNG] + prompt riêng + [tham số]
```

- Prompt viết bằng tiếng Anh vì các công cụ tạo ảnh hiểu tiếng Anh tốt hơn.
- Thay `[STYLE]` trong từng prompt bằng đoạn **Style chung** ở mục 0.
- Giữ cùng style và seed cho mọi ảnh: `--sref` với Midjourney, LoRA hoặc IP-Adapter với Stable Diffusion/Flux.
- Ảnh cần nền trong suốt: nếu công cụ không xuất được alpha, tạo trên nền xanh lá thuần màu rồi tách nền bằng remove.bg hoặc rembg.

---

## 0. Style chung

Dán vào đầu mọi prompt:

```
2D xianxia fantasy game art, Chinese ink-wash painting blended with clean cel-shading, painterly brush texture, elegant and mystical, deep teal and jade green palette with antique gold accents, soft moonlight, glowing spirit qi particles, misty atmosphere, high detail, consistent art style, professional mobile game asset
```

### Negative prompt

Dùng cho SD/Flux. Với Midjourney thì đưa vào `--no ...`.

```
text, watermark, logo, signature, UI, frame, border, photorealistic, 3D render, low quality, blurry, jpeg artifacts, extra limbs, deformed hands, modern buildings, western castle, anime chibi, oversaturated
```

---

## 1. Bản đồ Thế Giới

Bản đồ gồm 4 lớp parallax, tỉ lệ 16:9. Mỗi lớp tạo riêng; từ lớp 2 trở đi cần nền trong suốt.

### Lớp 1 — Bầu trời

```
[STYLE], night sky background only, full pale golden moon upper right, faint stars, thin drifting clouds, gradient from deep navy-teal at top to misty jade at horizon, no mountains, no ground, wide panoramic --ar 16:9
```

### Lớp 2 — Núi xa

```
[STYLE], distant layered mountain silhouettes in the style of Chinese shan shui painting, sharp misty peaks fading into fog, muted teal tones, isolated on plain flat green background for easy cutout, no sky, no foreground --ar 16:9
```

### Lớp 3 — Núi gần, mây và thác

```
[STYLE], mid-ground jagged jade cliffs with pine trees, waterfalls, rolling sea of clouds between peaks, a few floating rocks, isolated on plain flat green background, no sky --ar 16:9
```

### Lớp 4 — Mặt đất

```
[STYLE], foreground rolling misty hills and stone paths winding across the scene, empty open spaces for placing buildings, scattered peach blossom trees, glowing grass, viewed from slightly elevated 3/4 angle, isolated on plain flat green background --ar 16:9
```

### Phương án một ảnh duy nhất (không parallax)

```
[STYLE], top-down 3/4 view fantasy world map of a cultivation realm, floating celestial palace on a cloud island at center, sect mountain with red flag on the right, cave dwelling glowing on the left, peach blossom pavilion, ancient battlefield with broken spears, sealed secret realm gate with purple barrier, stone paths connecting all locations, mountains and sea of clouds, night with full moon, empty clear areas around each landmark for labels --ar 16:9
```

---

## 2. Công trình trên bản đồ

Tỉ lệ 1:1, nền trong suốt. Mẫu chung:

```
[STYLE], isolated single building, 3/4 isometric view, centered, plain flat green background, soft rim light, [MÔ TẢ] --ar 1:1
```

| Địa điểm | [MÔ TẢ] |
|---|---|
| Tử Tiêu Cung | `majestic multi-tiered celestial palace on a floating disc of rock and clouds, golden roofs, jade pillars, beam of divine light from below, purple-gold aura` |
| Động Phủ | `cultivator's cave dwelling carved into a mossy cliff, round stone door glowing with teal qi, small spirit spring, talismans hanging at entrance` |
| Đào Hoa Đình | `elegant wooden pavilion under blooming peach blossom trees, pink petals floating, small lantern, romantic serene mood` |
| Thế Lực | `sect gate and main hall on a mountain, tall red banner flag on roof, stone stairs, guardian lion statues` |
| Chiến Trường | `ancient battlefield with broken spears and swords stuck in scorched ground, war banners, red smoky aura, ruined stone altar` |
| Bí Cảnh | `mysterious sealed realm gate, two ancient stone pillars with swirling purple vortex portal between them, floating runes, chains` |

### Bản bị khoá

Thêm vào cuối prompt:

```
, sealed by grey stone chains and faded colors, desaturated, mysterious fog
```

Cách nhanh hơn: dùng chính ảnh gốc với CSS `filter: grayscale(1) brightness(.6)`.

---

## 3. Menu và UI

Chữ và nút nên giữ bằng code (SVG/CSS). Chỉ tạo ảnh cho khung, nền panel và icon.

### Nền panel / khung cuộn

```
[STYLE], ornate empty UI panel frame, dark jade lacquered wood with thin antique gold cloud-pattern trim on corners, center is plain dark teal texture, symmetrical, flat front view, no text --ar 4:3
```

### Nút bấm

Tạo 2 bản: teal và gold.

```
[STYLE], single horizontal game button, polished jade stone with gold engraved cloud edges, glossy, empty center, flat front view, isolated on plain flat green background --ar 4:1
```

### Thanh tiến trình tu vi

```
[STYLE], horizontal progress bar frame, gold dragon-scale border, empty dark inner groove, with separate glowing teal qi liquid fill, flat front view, isolated --ar 8:1
```

### Màn hình đăng nhập / splash

```
[STYLE], epic key art, lone cultivator in flowing robes standing on a sword flying above a sea of clouds toward a floating celestial palace, full moon, dramatic composition, large empty sky area at top for game title --ar 9:16
```

### Icon bottom nav

Tạo cả bộ trong một ảnh rồi cắt ra.

```
[STYLE], game icon set of 6 icons in a grid, consistent style, gold line art on dark jade round medallions: folded map, lotus, cultivator silhouette, cloth pouch, sect banner flag, jade tablet ranking plaque, flat front view, evenly spaced --ar 3:2
```

### Icon vật phẩm

Mẫu chung:

```
[STYLE], single game item icon, centered, dark jade circular backdrop with gold rim, [VẬT PHẨM] --ar 1:1
```

| Vật phẩm | [VẬT PHẨM] |
|---|---|
| Linh thạch | `glowing faceted spirit stone crystal, teal light` |
| Pháp bảo | `floating ancient bronze flying sword with runes` |
| Đan dược | `round golden elixir pill with swirling aura in a small porcelain bowl` |
| Linh dược | `rare spirit herb with glowing leaves and dew` |
| Tụ Linh Phù | `yellow paper talisman with red cinnabar runes, glowing` |

---

## 4. Nhân vật

### Ảnh mẫu gốc (character sheet)

Làm ảnh này trước. Mọi ảnh nhân vật sau dùng nó làm reference để giữ đúng gương mặt.

**Nam:**

```
[STYLE], character reference sheet of a young male Chinese cultivator, long black hair tied in a topknot with a gold hairpin crown, calm handsome face, dark jade green layered hanfu robes with gold trim, white inner collar, jade pendant at waist, front view, side view, back view, full body, neutral pose, plain light background --ar 16:9
```

**Nữ:** thay phần mô tả nhân vật bằng:

```
young female cultivator, long black hair half-up with jade hairpins, flowing light teal hanfu with gold embroidery, silk ribbons
```

### Ngồi thiền (tab Tu Luyện)

```
[STYLE], same character, sitting in lotus meditation pose on a glowing circular rune array on the floor, eyes closed, holding a small glowing spirit orb, teal qi energy swirling around body, front view, full body, isolated on plain flat green background --ar 1:1
```

### Chân dung avatar

```
[STYLE], same character, bust portrait, facing slightly to the side, serene expression, soft gold rim light, dark teal gradient background, centered for circular crop --ar 1:1
```

### Toàn thân theo cảnh giới (tab Nhân Vật)

Mẫu chung:

```
[STYLE], same character, full body standing heroic pose, robes flowing in wind, [KHÍ THẾ], isolated on plain flat green background --ar 2:3
```

| Cảnh giới | [KHÍ THẾ] |
|---|---|
| Luyện Khí | `faint wisps of teal qi, simple robes, humble aura` |
| Trúc Cơ | `steady glowing teal aura, wooden sword on back` |
| Kim Đan | `golden core glowing at the dantian, gold aura, flying sword hovering beside` |
| Nguyên Anh | `small luminous nascent soul figure above head, powerful teal-gold aura` |
| Hoá Thần | `divine light halo, robes of starlight, clouds gathering beneath feet, celestial presence` |

### Bộ ảnh theo dáng người (`characters/<body_id>/…`)

Mỗi dáng người trong `appearances.yaml` có character sheet riêng (lưu ở `art-work/refs/<body_id>-sheet.png`), mọi ảnh
sau gọi kèm `--ref` sheet đó. Đã làm đủ 12 dáng người (2026-11-09).

- **Sheet:** `character reference sheet of one original adult [mô tả từ traits/temperament + màu default_robe], same
  person shown three times in a row: front view, side view, back view, full body, neutral standing pose, plain light
  parchment background, no text, no labels`. Nhân vật nữ thêm `modest non-revealing clothing`, `modest high closed collar`.
- Mọi ảnh sau mở đầu bằng `exactly the same character as in the reference sheet (same face, hair, hair ornament and
  robe)`, rồi:
  - **avatar:** `bust portrait of head and shoulders, face in gentle three-quarter view…, soft antique-gold rim light,
    dark teal ink-wash gradient background, head centred with generous space around it so it survives a circular crop`
  - **meditate:** `sitting cross-legged in full lotus meditation pose, hands resting together in the lap, eyes closed,
    seated above a glowing circular formation of concentric jade and antique-gold rings with abstract cloud-pattern
    engravings (no letters, no religious symbols)…, front view, whole figure and formation centred with margin`
  - **listener:** `seen from directly behind, seated cross-legged on a round dark jade meditation cushion, back view
    only, face not visible, … about 70 percent of the frame height, clear simple silhouette`
  - **body-luyen_khi:** `full body standing from head to feet, calm poised stance, [đạo cụ], robes flowing gently,
    faint wisps of teal qi, humble beginner-cultivator aura, front three-quarter view, … margin above and below`
- **Nền tách:** áo/khí màu ngọc gặp nền xanh lá sẽ bị tách mất, nên các slot alpha của nhân vật dùng nền **magenta**
  phẳng (#FF00FF) rồi tách magenta; khói mờ được khử về xám mực.
- **body-b1…b6 (bậc hiệu ứng):** cùng mẫu toàn thân, thêm theo bậc. Tiên: b1 `simple plain outfit, faint wisps of
  qi` · b2 `slightly finer, steady glowing aura, faint jade ring on the ground` · b3 `finer layered, more gold trim,
  glowing core at the lower belly` · b4 `richer embroidery, flowing outer mantle, small pale lightning sparks` · b5
  `ornate gold-embroidered outer robe, sashes, five-coloured auspicious clouds at the feet` · b6 `woven with faint
  starlight, ring of small stars, five-coloured clouds`. Vu: thay hào quang bằng `blood-qi steam`, hình xăm sáng, giáp đồng,
  áo lông (b5), `primordial titan … bronze and starlight` (b6). **Vu b2:** viết rõ `the ground under the feet cracked
  with faint glowing ember-red earth veins (no circle, no star, no symbols, no runes, no letters)`. Viết "ring of
  geometric patterns" thì model vẽ ngôi sao năm cánh kiểu phù thuỷ phương Tây.
- **Thời Vu:** meditate = `deep wide horse stance` (trung_binh_tan) hoặc `sitting on a flat boulder` (toa_thach), hơi
  huyết khí đỏ trầm, không dùng trận pháp ngọc của phe tiên.
- **Nữ:** luôn thêm `modest non-revealing clothing fully covering the body, adult proportions`. Mô tả "mặt tròn, mắt
  cười" dễ ra dáng trẻ con: phải ghi rõ tuổi (`about twenty-five years old, adult height, not a child, not chibi`).
- **Avatar:** dặn rõ `EYES OPEN`, `no halo, no ring or circle behind the head`. Nhân vật trầm tĩnh (Hành Giả) dễ bị vẽ
  nhắm mắt, còn ánh viền vàng dễ thành vầng hào quang.
- **Nền tách:** áo ngọc/mặc/thanh lam/bạch và Lực Sĩ (áo đồng xanh) dùng `--key magenta`, áo huyết/thổ dùng `--key green`.
- **`avatar-robe`:** không sinh bằng AI (không khớp từng pixel). Tách từ chính avatar theo màu áo. Chỉ làm được khi
  màu áo khác hẳn nền teal (vàng, thổ, huyết, bạch); áo ngọc/mặc tối trên nền teal thì bỏ qua.

### Bộ trang phục vẽ sẵn (`characters/<body_id>/<outfit_id>/…`, decisions 2026-11-09 "Bộ trang phục vẽ sẵn")

Mỗi dáng người có 3 bộ trong `appearances.yaml` (`outfits[]`). Bộ mặc định (`default_outfit`) chính là bộ ảnh hiện có
trong `characters/<body_id>/` — không vẽ lại. Hai bộ còn lại vẽ vào thư mục con theo id bộ, cùng tên file và kích thước
(`avatar`, `meditate` = `default_pose`, `meditate-<pose>` cho các tư thế khác của đạo đồ, `listener`, `body-b1…b6`).
- **Sheet theo bộ:** sheet của dáng người làm `--ref`, prompt `exactly the same person as in the reference sheet (same
  face, body, hairstyle) now wearing: [trường art của bộ]`; giữ đúng mặt/tóc, chỉ đổi y phục + phụ kiện tóc của bộ
  (`hair_accessory`; `khong` = không trang sức tóc). Lưu `art-work/refs/<body_id>-<outfit_id>-sheet.png`, rồi các ảnh
  sau gọi kèm sheet bộ đó với các mẫu avatar/meditate/listener/body-b ở trên.
- Màu chủ đạo của bộ = `robe_palette` (token `--color-robe-<id>`); ba bộ của một dáng người luôn khác màu, nên ảnh phải
  khác rõ khi đặt cạnh nhau. Nữ (trừ Vân Khê): vẫn `modest non-revealing clothing fully covering the body`, cổ kín.
- **Đã làm (2026-11-09):** 24 sheet + 255 ảnh. Mỗi slot gọi 2 ref: sheet bộ (đồ, màu) + ảnh cùng slot của bộ mặc định
  (`use the SECOND reference image ONLY for camera framing, pose, scale and position, IGNORE its clothing and colours`).
  Nền tách theo màu áo: magenta cho lam/bạch/ngọc/mặc/tử/đồng, green cho thổ/huyết/kim.
- **Lỗi hay gặp:** sheet bộ mới hay giữ lại trang sức tóc của sheet gốc (dây đỏ của Nữ Kiếm Khách, kim quan của Công
  Tử), nên phải ghi rõ `NO red cord…` / `NO gold crown…`. Ở b6, màu áo dễ trôi về teal (Y Sư bộ thổ), nên phải ghi
  `the robe stays WARM EARTH-OCHRE in every image`.

### Hướng dẫn viên Vân Khê (`characters/guide/<expr>`)

Nhân vật nguyên bản (`content/story/characters/van-khe.md`). Làm character sheet trước, rồi 6 biểu cảm cùng khung hình
(nửa người, đầu ở cùng độ cao, nền trong suốt, 768×1024).

**Ngoại hình đang dùng: phương án C2** (chủ dự án chọn ngày 2026-11-09, sửa từ C "mị lực"). Sheet:
`art-work/refs/van_khe-sheet-c2.png`, ảnh khóa khung: `art-work/refs/van_khe-c2-neutral.png`. Để khôi phục: bản C cất ở
`art-work/old-guide-c/`, bản kín đáo ban đầu ở `art-work/old-guide/`.

```
[STYLE], character reference sheet of the same woman as in the reference image (same face, low bun at the nape with the
peach-wood hairpin and a few silver strands, brush and bamboo notebook at the hip, ink-black silk shawl draped at the
elbows), clearly an adult woman in her late twenties with natural average adult proportions (modest natural bust, not
exaggerated), tasteful elegant xianxia fashion illustration, all fabric fully opaque, non-explicit: an opaque smoky violet
silk gown with a shallow V / sweetheart neckline showing only a slight hint of cleavage, embroidered violet-and-gold belt
with a thin gold waist chain and small gold tassels, jade-and-gold drop earrings, fine gold necklace with a small jade
pendant, gold upper-arm cuff, thin jade bracelets, gold step-shake hairpin (buyao) beside the peach-wood pin, red lips,
calm subtle smile, violet black and antique gold palette, front view, side view, back view, full body --ar 16:9
```

Mẹo: khi gọi kèm ảnh `neutral` cũ để khóa khung hình, model chép luôn bộ áo cũ. Phải ghi rõ `use the SECOND reference
image ONLY for camera framing; IGNORE its clothing. Copy the outfit and every accessory from the FIRST`. Các chi tiết nhỏ
(chấm giữa mày, hoa tai) phải liệt kê trong mọi prompt biểu cảm, nếu không sẽ lúc có lúc mất.

Giới hạn cứng (giữ nguyên dù đổi phong cách): rõ là người trưởng thành, không khỏa thân, vải không xuyên thấu, không
phóng đại hình thể, không tư thế gợi dục, không có gì lộ liễu. Ảnh trong game chỉ cắt nửa người, nên đường xẻ tà không
xuất hiện.

Biểu cảm (thêm vào mẫu nửa người `[STYLE], same character, half body, facing the viewer, isolated on plain flat green
background --ar 3:4`): `neutral` (bình thản, tay khép trước bụng) · `smile` (mắt cong cười) · `point` (tay phải chỉ lên
góc trên phải) · `worried` (mày hơi chau, tay đặt lên cuốn sổ) · `proud` (tay đặt trước ngực, cằm hơi ngẩng) ·
`surprised` (mắt tròn, miệng hé). Không trẻ hóa, không biểu tượng tôn giáo.

Cách làm đã dùng (2026-11-09): sheet → `neutral` (bản C2 ở `art-work/refs/van_khe-c2-neutral.png`; lần sinh đầu ra
khung 3/4 người, nên phải gọi kèm `neutral` cũ làm ref thứ hai để khóa khung hình) → 5 biểu cảm gọi kèm
**hai** ref (sheet + neutral) với câu `the same framing, scale and position as the second reference image (head at
exactly the same height…); only the expression and hands change`. Đầu lệch nhau ≤ ~2% chiều cao. `blink`: model hay vẽ
nháy một mắt hoặc đổi tư thế, nên chỉ lấy dải mắt của ảnh nhắm mắt (căn khớp với neutral, mép làm mềm) dán lên neutral.

### Hào quang theo bậc (`fx/aura-1…6`)

Nền **đen** (`--key black`): alpha lấy theo độ sáng, nên ảnh chỉ có ánh sáng. Chừa trống phần giữa dưới và đường giữa
phía trên (đầu nhân vật nằm ở đó). Chọn aura-1, rồi aura-2, làm `--ref` cho các bậc sau. Các bậc: 1 sương ngọc · 2 vòng
ngọc dưới đất · 3 + hai linh châu nhỏ thấp ở hai bên (để ở giữa thì thành vầng hào quang sau đầu, nên không làm thế) ·
4 + tia sét nhỏ · 5 + mây ngũ sắc · 6 + vòng sao và pháp tướng là bóng người tu sĩ bằng ánh sao (`faceless, no hood, no
halo, no lotus, no hand gesture`). Không vẽ tượng thần, không làm như người đội mũ trùm.

### Đối thủ / NPC

Chèn mô tả vào cùng mẫu toàn thân ở trên, ví dụ:

- `elder sect master with long white beard and horsetail whisk`
- `demonic cultivator in black and crimson robes with red eyes`

---

## 5. Mẹo để ảnh đồng bộ

- **Midjourney:** dùng ảnh map đầu tiên đạt yêu cầu làm `--sref <url>` cho mọi ảnh sau, kèm `--cref <url character sheet>` cho nhân vật. Thêm `--style raw --stylize 150` để bớt "tự chế".
- **SD/Flux:** cố định seed, dùng ControlNet hoặc IP-Adapter với ảnh tham chiếu, kèm một LoRA xianxia/guofeng.
- **Xuất file:** đặt `--ar` đúng như trên, upscale lên 2x, xuất WebP (khoảng 80% quality). Ảnh cần nền trong suốt thì xuất WebP hoặc PNG kèm alpha.
- **Chỗ trống cho nhãn:** khi sinh map, luôn thêm cụm `empty clear areas for labels` để nhãn không đè lên chi tiết.

## 6. Kích thước xuất gợi ý

| Asset | Kích thước | Định dạng |
|---|---|---|
| Lớp bản đồ | 1920×1080 (thêm bản 2x nếu cần) | WebP, alpha từ lớp 2 |
| Công trình | 512×512 | WebP/PNG alpha |
| Nhân vật thiền, nền động phủ | 1024×1024 | WebP |
| Avatar | 256×256 | WebP |
| Toàn thân | 1024×1536 | WebP/PNG alpha |
| Icon vật phẩm | 128×128 | WebP/PNG alpha |
| Huy hiệu / banner Thế Lực | 512×512 | WebP/PNG alpha |

---

## 7. Tích hợp vào game (đối chiếu repo)

### 7.1. Thả file vào đâu

Thư mục: `apps/web/src/lib/assets/art/`. Bảng tên file đầy đủ ở
[README trong thư mục đó](../../apps/web/src/lib/assets/art/README.md).

- Định dạng `.avif`, `.webp` hoặc `.png`. Bản 2x đặt tên `ten@2x.webp`.
- Lúc build, Vite liệt kê các file đang có (`apps/web/src/lib/game/art.ts`). Game chỉ trỏ tới ảnh có thật, nên
  không có request 404. Tên file được gắn hash, nên cache không giữ bản cũ.
- Ảnh nào chưa có thì giữ hình SVG/CSS hiện tại. Vì vậy có thể làm dần từng ảnh.

| Mục prompt | File | Hiện ở đâu |
|---|---|---|
| §1 bốn lớp | `world/sky`, `world/far`, `world/mid`, `world/ground` | Bản đồ Thế Giới (parallax theo chuột). Có `world/sky` là bật chế độ lớp |
| §1 một ảnh | `world/map` | Bản đồ Thế Giới khi không có `world/sky` |
| §2 công trình | `landmarks/tu-tieu-cung`, `dong-phu`, `the-luc`, `dao-hoa-dinh`, `bi-canh`, `chien-truong` | Đặt trên bản đồ (chỉ ở chế độ 4 lớp). Chỗ bị khóa tự chuyển xám bằng CSS, không cần làm bản khóa |
| §3 splash | `ui/splash` | Màn đăng nhập trên mobile dọc. Desktop vẫn dùng bản đồ |
| §3 icon vật phẩm | `items/<item_id>`, `items/linh_thach` | Túi Đồ. Đặt tên theo ID trong `content/catalog/items.yaml` (`tu_linh_phu`, `truc_co_dan`, `ket_dan_qua`, `linh_thao`) |
| §4 avatar | `characters/avatar` | Mọi ảnh tròn: HUD, thẻ người chơi, Thiên Bảng |
| §4 ngồi thiền | `characters/meditate` | Tu Luyện, Đột Phá. Thay hình SVG và pháp trận vì ảnh đã có trận riêng |
| §4 toàn thân | `characters/body-<realm_id>` | Nhân Vật. Thiếu thì lùi về cảnh giới thấp hơn gần nhất, rồi về avatar |
| Huy hiệu thế lực | `factions/<faction_id>` | Thẻ chọn thế lực, màn Thế Lực |

### 7.1b. Tạo ảnh tự động

Agent `art-generator` (`.claude/agents/art-generator.md`) chạy toàn bộ quy trình:
- soạn prompt từ tài liệu này;
- gọi API tạo ảnh qua `tools/artgen/artgen.py`: **Meta Muse Image** (dev.meta.ai, `MODEL_API_KEY`, $0.01/ảnh,
  không có nền trong suốt nên script tạo nền xanh rồi tự tách) hoặc **OpenAI gpt-image-1** (`OPENAI_API_KEY`,
  có nền trong suốt thật). Có `MODEL_API_KEY` thì dùng Meta; `ART_PROVIDER` để ép chọn;
- xem và chấm từng ảnh theo checklist, tạo lại nếu chưa đạt;
- tách nền, cắt, xuất WebP rồi đặt vào slot.

Mỗi lần gọi API được ghi vào `tools/artgen/ledger.jsonl` (prompt, model, tham số). Ảnh gốc nằm ở `art-work/`
(không commit). Trước khi dùng ảnh cho bản phát hành, đọc điều khoản sử dụng và quyền sở hữu ảnh đầu ra của nhà
cung cấp. Tài liệu ảnh của Meta không nói rõ hai điểm này.

### 7.2. Prompt nên bỏ qua (giữ bằng code)

- **Icon bottom nav, nút bấm, thanh tiến trình (§3).** Giữ SVG/CSS theo `.claude/rules/graphic-design.md`.
  Lý do: đổi màu theo trạng thái được (đang chọn, bị khóa, có huy hiệu), nét sắc ở mọi kích thước,
  hỗ trợ đọc màn hình, chữ trên nút đổi được mà không phải vẽ lại.
- **Nền panel (§3):** chưa nối. Nếu muốn, sau này dùng làm `border-image` 9-slice cho `.panel`. Tạo ảnh
  không có chữ, các góc đối xứng.

### 7.3. Chỗ còn thiếu hoặc cần quyết

1. **Dáng người (GĐ-A, 2026-11-09).** Người chơi chọn 1 trong 12 dáng người của `content/catalog/appearances.yaml`
   (tiên 4 nam + 4 nữ, Vu 2 + 2). Mỗi dáng người có thư mục ảnh riêng `characters/<body_id>/` (avatar, meditate,
   body-<realm_id>, listener — xem README ô ảnh), làm **character sheet riêng trước** rồi mới làm các ảnh khác. Mô tả
   khuôn mặt, tóc, vóc dáng lấy từ trường `traits` / `temperament` của dáng người; kiểu tóc gắn với dáng người, phụ kiện
   tóc và màu áo là lớp phủ / mặt nạ tô màu. Thiếu ảnh thì game vẽ SVG theo đúng ngoại hình đã chọn. Bộ ảnh dùng chung
   ở trên chỉ còn dùng cho dữ liệu chưa có ngoại hình.
2. **Cảnh giới.** Game có 15 cảnh giới (`realms.yaml`), bảng §4 mới có 5. Thiếu thì tự lùi về ảnh cảnh giới
   thấp hơn. Đạo đồ Vu cần bộ riêng (luyện thể, huyết khí), không dùng ảnh ngồi thiền của phe tiên.
3. **Huy hiệu thế lực** chưa có prompt. Dùng mẫu sau, phần mô tả lấy từ mục "Huy hiệu" trong
   `content/story/factions/<id>.md`:
   `[STYLE], circular sect emblem medallion, gold relief on dark jade, [mô tả huy hiệu], flat front view, isolated on plain flat green background --ar 1:1`.
   Thế lực mang tên theo `cot-truyen.md` (Tam Thanh, Tây Phương, Tiên Đình, Thiên Đình Yêu Tộc, Vu Tộc…; đảo ngược D2
   2026-11-07), nhưng huy hiệu vẫn chỉ dùng họa tiết chung (mây, sóng, sao, núi): không dùng biểu tượng hay hình
   tượng thờ cúng có thật (thái cực, bát quái, hoa sen, pháp luân, chữ vạn, tượng thần/Phật). File đặt theo slug
   vĩnh viễn trong `factions.yaml` (vd. `factions/huyen_khung_mon` là Tam Thanh).
4. **Pháp bảo (§3)** chưa có vật phẩm tương ứng trong catalogue. Đặt tên file theo ID khi catalogue thêm.
5. **Bản quyền.** Kiểm tra điều khoản thương mại của công cụ tạo ảnh. Lưu lại prompt và seed của từng ảnh đã
   dùng. Không đưa tên họa sĩ hay tên game có thật vào prompt (quy tắc nguyên bản của dự án).
