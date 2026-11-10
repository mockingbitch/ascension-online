# Art backlog — ô ảnh còn thiếu / nên thay

Lập ngày 2026-11-10 bằng cách đối chiếu slot mà code + README đòi hỏi với file thật trong
`apps/web/src/lib/assets/art/` (506 file, đều là WebP thật, không có LFS pointer). Tạo ảnh đang **tạm dừng** vì Meta
đã chạm hạn mức chi tiêu tháng (commit `35d28aa`, `tools/artgen/ledger.jsonl`). Đây là hàng đợi cho lúc có ngân sách lại.

Nguồn đối chiếu: `apps/web/src/lib/assets/art/README.md` (bảng slot), `apps/web/src/lib/game/looks.ts` (chuỗi slot nhân
vật), `art.ts` / `charArt.svelte.ts` / `charArtIndex.ts` (glob), `content/catalog/{appearances,cosmetics,faction_works,items,factions,realms}.yaml`,
`lib/engine/mode/BreakthroughMode.svelte` + `lib/engine/fx/breakthrough.ts` (lớp ăn mừng đột phá),
`docs/design/visual-progression-proposal.md` §6 (kế hoạch ô ảnh).

## Tóm tắt

| Ưu tiên | Số ảnh (×1, chưa tính @2x) | Gồm |
|---|---|---|
| **P0** | **8** | 1 body-b6 còn thiếu, nền Động Phủ, 1 icon đan dược bán ở shop, 5 huy hiệu thế lực mở từ đầu |
| **P1** | **38** | 3 lớp ăn mừng bậc 5, 5 icon pháp bảo, 4 lớp bản đồ parallax + 6 công trình, 5 huy hiệu thế lực mở sau, 8 khung chân dung, 7 trang trí động phủ |
| **P2** | **50** | 3 lớp fx bậc 3/6, 15 trang trí động phủ theo mùa/sự kiện, 12 trang trí thế lực, 8 `avatar-robe`, 12 `meditate-robe` |
| Tổng | **96** | |

Phần nhân vật gần như **đã đủ**: 12 dáng người × 3 trang phục × (avatar + meditate + mọi tư thế + listener + body-b1…b6)
có đủ trên đĩa, trừ đúng **một** ảnh (`tien_nu_y_su/y_su_hoang_tho/body-b6`). Hướng dẫn viên Vân Khê đủ 7 biểu cảm
(`GuideExpr` chỉ khai báo 6 + `blink`), không cần thêm.

Định nghĩa ưu tiên:
- **P0** — người chơi nào cũng thấy ngay ở đầu game (tạo nhân vật, Động Phủ, shop), hoặc là ảnh retry đã soạn sẵn prompt/ref.
- **P1** — thấy thường xuyên nhưng fallback SVG/CSS đang chấp nhận được, hoặc chỉ thấy sau vài tuần chơi.
- **P2** — cuối game, theo mùa/sự kiện, hoặc chỉ là fallback hiếm khi hiện.

## Cách tạo ảnh

- Công cụ: `tools/artgen/artgen.py` (chạy từ gốc repo bằng `uv run`), agent `art-generator` (`.claude/agents/art-generator.md`)
  điều phối: `gen` (gọi Meta Muse Image hoặc OpenAI gpt-image, ghi `ledger.jsonl`, ảnh gốc vào `art-work/raw/`) →
  duyệt theo checklist → `place --slot <slot> --src <file> [--key green|magenta|black]` (cắt, tách nền, resize, ghi
  `.webp` + `@2x` khi nguồn đủ lớn). `uv run tools/artgen/artgen.py slots` liệt kê slot đã biết; `selftest` kiểm offline.
- Style chung + negative prompt: `docs/design/art-prompts.md` §0 (artgen tự chèn). Mọi prompt dưới đây là phần
  `[MÔ TẢ]` ghép sau STYLE. Luật chung: mực tàu & ngọc bích (teal/jade + vàng cổ), không chữ / ký tự / logo, không biểu
  tượng tôn giáo (không đài sen, vầng hào quang thánh, ấn tay, chữ vạn, bánh xe pháp), trang phục nữ kín đáo (cổ kín, che
  kín) trừ Vân Khê đã chốt, phong cách tự sáng tác — không bắt chước tác phẩm/nghệ sĩ cụ thể.
- **Trước khi chạy, artgen cần thêm pattern** (`SLOTS` trong `artgen.py`; ngoài phạm vi tài liệu này):
  - `cosmetics/[a-z0-9_]+` — chưa có pattern nào, `place` sẽ từ chối. Khung chân dung 320² alpha; trang trí 256² alpha.
  - `fx/bt-[a-z0-9-]+`, `fx/nascent` — hiện chỉ có `fx/aura-[1-6]`.
  - `scenes/dong-phu` — pattern `scenes/[a-z0-9-]+` đang ép **16:9 / 1920×1080**, nhưng README và
    `scene/defs/dong_phu.ts` dùng khung **4:3 / 1600×1200**. Cần một pattern riêng đứng trước pattern chung.
- **Quy tắc cập nhật README**: mỗi lần thả ảnh mới, sửa bảng "Painted art" trong
  `apps/web/src/lib/assets/art/README.md` (xoá dòng "Missing: …" khi đủ, thêm bộ mới) và thêm dòng vào bảng slot nếu slot
  mới (vd. `fx/bt-*`, `meditate-robe`). Đổi tên/kích thước slot thì sửa cùng lúc `art-prompts.md` và `artgen.py SLOTS`.
  Ảnh nằm trong Git LFS; `apps/web/tools/check-lfs-art.mjs` (prebuild) chặn build khi checkout chỉ có pointer.
- Slot nằm dưới `characters/<body_id>/…` và `fx/…` được nạp lười qua `charArtIndex.ts`; `cosmetics/`, `items/`,
  `factions/`, `world/`, `landmarks/`, `scenes/` nằm trong glob eager của `art.ts` (chỉ URL, ảnh tải khi dùng).

---

## 1. Nhân vật (`characters/<body_id>[/<outfit_id>]/…`)

Đã kiểm bằng script so với `appearances.yaml` (12 body, 36 outfit, tư thế theo path: tiên `kiet_gia|ban_gia|kiem_ngang`,
Vu `trung_binh_tan|toa_thach`; outfit mặc định = thư mục body). Mỗi thư mục có đủ `avatar(+@2x)`, `meditate`,
`meditate-<tư thế khác default>`, `listener(+@2x)`, `body-b1…b6` — **trừ** dòng dưới.

| Slot | Kích thước | Alpha | Ưu tiên | Ghi chú prompt / lý do |
|---|---|---|---|---|
| `characters/tien_nu_y_su/y_su_hoang_tho/body-b6` | 1024×1536 (không @2x) | có (green screen → key) | **P0** | Ô duy nhất còn trống của bộ nhân vật; hiện lùi về b5. Retry đã soạn sẵn: ledger 2026-11-09 17:11 và 17:44 cùng slot, refs `art-work/refs/tien_nu_y_su-y_su_hoang_tho-sheet.png` + raw `characters__tien_nu_y_su__body-b6-20261109-144636-0.png`. Dùng lại đúng prompt trong ledger: Y Sư, áo vải hoàng thổ cổ kín, khăn vấn nâu, túi vải chéo nhiều ngăn, trâm gỗ; khí thế bậc 6 (vòng sao, ánh sao mờ) như b6 của các outfit khác. |

### Mặt nạ màu áo (robe masks)

`avatar-robe` chỉ có ở 4 body (`tien_nu_tieu_thu`, `tien_nam_cong_tu`, `vu_nu_tho_san`, `vu_nam_chien_si`). README ghi
8 body còn lại bị bỏ qua vì không tách được áo khỏi nền teal / da bằng màu. `meditate-robe` **không có ở body nào**.

Giá trị thực tế thấp: mặt nạ chỉ tô lên ảnh **cấp body** (outfit mặc định) khi `Portrait` rơi về đó; vì cả 24 outfit phụ
đều đã có avatar riêng nên mặt nạ hầu như không hiện. `meditate-robe` thậm chí **chưa có code đọc** (`maskSlot(…,
'meditate')` được khai báo trong `looks.ts` nhưng không ai gọi). Vì vậy cả nhóm là P2.

| Slot | Kích thước | Alpha | Ưu tiên | Ghi chú |
|---|---|---|---|---|
| `characters/tien_nam_thu_sinh/avatar-robe` | 256² | có (xám) | P2 | Mặt nạ xám vùng áo, khớp pixel với `avatar` hiện có. Không phải ảnh model vẽ: tách thủ công/bán tự động từ avatar (vẽ vùng áo bằng tay hoặc segment), hoặc vẽ lại avatar trên nền key phẳng rồi tách. |
| `characters/tien_nam_kiem_khach/avatar-robe` | 256² | có | P2 | như trên |
| `characters/tien_nam_an_si/avatar-robe` | 256² | có | P2 | như trên |
| `characters/tien_nu_kiem_khach/avatar-robe` | 256² | có | P2 | như trên |
| `characters/tien_nu_y_su/avatar-robe` | 256² | có | P2 | như trên |
| `characters/tien_nu_an_tu/avatar-robe` | 256² | có | P2 | như trên (áo trắng sát nền sáng — khó nhất) |
| `characters/vu_nam_hanh_gia/avatar-robe` | 256² | có | P2 | như trên |
| `characters/vu_nu_luc_si/avatar-robe` | 256² | có | P2 | như trên |
| `characters/<body_id>/meditate-robe` × 12 (mọi body) | 1024² | có | P2 | Chặn bởi code: chưa có component nào đọc. Chỉ làm khi Động Phủ/Đột Phá bắt đầu tô màu áo cho ảnh thiền. |

### Không đưa vào hàng đợi

- `characters/acc/<hair_id>` (9, README "reserved"): không code nào đọc; phụ kiện tóc giờ đã vẽ sẵn trong từng outfit.
- Bộ dùng chung `characters/{avatar,meditate,listener,body-luyen_khi}`: còn dùng cho dữ liệu chưa có ngoại hình, không cần thay.

---

## 2. Lớp ăn mừng đột phá (BreakthroughMode, bậc 3–6)

Hiện vẽ bằng CSS/SVG trong `BreakthroughMode.svelte`: `.auspicious` (5 đốm mây gradient ngũ sắc, dải đáy cao 34%),
`svg.gate` (viewBox 240×200, 2 cánh `door.l` / `door.r` co `scaleX` quanh mép ngoài, rộng tối đa 420 px), `.galaxy`
(conic-gradient vuông, tối đa 150vh), `.avatar` (pháp tướng = `CultivatorFigure` của chính người chơi), `.nascent`
(bust). Chưa có slot raster nào được code tra — **mỗi slot dưới đây cần thêm một dòng `charArt('fx/…')` + fallback CSS
hiện có**, và một dòng README. Thư mục `fx/` đã nằm trong index lười nên không tăng bundle ban đầu. Bậc 5 = Nhân Tiên…Kim
Tiên, bậc 6 = Thái Ất trở lên (`realms.yaml visual.fx_tier`), tức là cuối game — nên không có P0.

| Slot | Kích thước | Alpha | Ưu tiên | Ghi chú prompt / lý do |
|---|---|---|---|---|
| `fx/bt-clouds-5color` | 1920×640 (3:1, không @2x) | có (nền đen, `--key black`, alpha theo độ sáng) | P1 | Thay `.auspicious` (bậc 5, cũng hiện ở bậc 6). `wide horizontal band of auspicious swirling clouds in five soft colours (jade, gold, azure, violet, crimson), ruyi-scroll cloud curls, glowing edges, rising from the bottom, top edge dissolving into darkness, on pure black`. Hai đầu trái/phải mềm để lặp/tràn. |
| `fx/bt-gate` | 480×400 (+@2x 960×800) | có | P1 | Khung cổng trời (vòm + hai cột + mây chân cột), **giữa rỗng** để hai cánh nằm trong. `towering celestial gate arch of white jade and antique gold floating on clouds, front view, symmetrical, empty doorway, light pouring from behind, no plaque text, on plain flat green background`. Khớp tỉ lệ viewBox 240×200 của SVG hiện tại. |
| `fx/bt-gate-door` | 240×400 (+@2x 480×800) | có | P1 | **Một** cánh cửa trái; cánh phải = cùng ảnh lật `scaleX(-1)` trong CSS (khớp animation `door-open` co quanh mép ngoài). `single tall heavenly gate door leaf, gold-studded dark jade panel with cloud relief, hinge on the left edge, front view, no characters or symbols, on plain flat green background`. |
| `fx/bt-galaxy` | 1024² (không @2x; nội dung mềm, phóng to được) | có (`--key black`) | P2 | Thay `.galaxy` (bậc 6). `top-down spiral galaxy disk of violet and teal starlight with a bright pale-gold core, thin dust lanes, fading smoothly to transparent at the edge, on pure black`. Phải xoay được: đối xứng xoay, không có chi tiết lệch tâm. |
| `fx/bt-phap-tuong` | 1024² | có (`--key black`) | P2 | Bóng pháp tướng **chung** đặt sau/quanh hình người chơi (không thay `CultivatorFigure`). Theo luật `aura-6`: `colossal translucent seated cultivator silhouette made of starlight and mist, faceless, no hood, no halo, no lotus, no hand gesture, robes dissolving into cloud, front view, on pure black`. Không tượng thần, không biểu tượng tôn giáo. |
| `fx/nascent` | 512² (+@2x 1024²) | có (`--key black`) | P2 | Có trong proposal §6 ("fx/nascent, fx/gate, fx/phap-tuong", P2). Thay ánh sáng quanh `.nascent` bậc 3: `small glowing infant-sized spirit silhouette of pale jade light, faceless, curled in a seated pose inside a soft light orb, on pure black`. Hình người lấy vẫn là bust của người chơi; ảnh này chỉ là quầng. |

Đề xuất thêm (chưa đếm, chưa có trong code/proposal): `fx/bt-storm-clouds` (dải mây đen lôi kiếp thay `.cloud c1–c3`
ở bậc 4–6) và `fx/bt-strike` (tia sét). Bậc 4 bắt đầu ở Luyện Hư nên sớm hơn bậc 5; cân nhắc khi xếp đợt sau.

---

## 3. Mỹ phẩm (`cosmetics/<slug>`)

Slug = phần sau dấu `:` trong id (`slugOf`, `cosmetics.svelte.ts`), duy nhất trên cả `cosmetics.yaml` và
`faction_works.yaml`. Thư mục `cosmetics/` **chưa tồn tại** → mọi slot đều thiếu. **Danh hiệu** (`title:*`, 18) và
**danh quang** (`chat_aura:*`, 6) là kiểu chữ CSS (`CosmeticArt` không bao giờ vẽ raster cho hai loại này) → không cần ảnh.

### 3.1 Khung chân dung (`avatar_frame`, 8)

Thấy ở mọi `Portrait` (HUD, chat, Thiên Bảng) khi trang bị; vẽ ở 128% đường kính chân dung. Fallback: vòng CSS
`.frame-*`. Tất cả **320×320 (+@2x 640×640), alpha, chỉ có vành, tâm trong suốt** (đường kính lỗ ≈ 78% cạnh ảnh để vừa
khít vòng tròn chân dung), chính diện, đối xứng, không chữ. Đều **P1** (bán bằng linh thạch / thưởng Tiên Duyên, CSS tạm ổn).

| Slot | Nguồn | Ghi chú prompt |
|---|---|---|
| `cosmetics/truc_moc` | shop, linh | `circular ring frame of bent purple bamboo stems bound with twine, rustic, transparent centre` |
| `cosmetics/ngoc_bich` | shop, huyền | `circular ring frame carved from translucent green jade with water-like veins, transparent centre` |
| `cosmetics/thanh_dong` | shop, huyền | `circular ring frame of ancient verdigris bronze engraved with angular thunder-cloud bands, worn edges` |
| `cosmetics/bach_van` | shop, địa | `circular silver ring frame wrapped in drifting white clouds, soft glow` |
| `cosmetics/xich_ha` | shop, thiên | `circular ring frame of crimson-gold sunset clouds and warm light rays, glowing rim` |
| `cosmetics/ngoc` | Tiên Duyên | `circular jade ring frame with small gold clasps at four points, refined` |
| `cosmetics/kim` | Tiên Duyên | `circular antique-gold ring frame with fine cloud filigree, warm glow` |
| `cosmetics/thien` | Tiên Duyên | `circular celestial ring frame of starlight and pale gold with tiny orbiting stars, faint violet glow` |

### 3.2 Trang trí động phủ (`cave_decoration`, 22)

Thấy trong cảnh Động Phủ (4 ô: 2 tường, 2 sàn; size 88) và tab Trang phục. Fallback: `DecorationGlyph` SVG. Tất cả
**256×256 (+@2x 512×512), alpha, chỉ vật thể, chính diện, bóng đổ mềm dưới chân**. Mẫu prompt:
`single decorative object for a cultivator's cave, front view, centered, soft ground shadow, isolated on plain flat green background, [MÔ TẢ]`.
P1 cho bộ thường trực, mùa hiện tại (Đông, tháng 11–1) và quà Tiên Duyên; P2 cho các mùa/sự kiện khác.

| Slot | Bộ | Ưu tiên | [MÔ TẢ] / lý do |
|---|---|---|---|
| `cosmetics/den_da` | Tùng Hạc (thường trực) | P1 | `mossy stone lantern with a small steady flame` |
| `cosmetics/chau_lan` | Tùng Hạc | P1 | `orchid growing from a rock in a shallow stone pot, long brush-stroke leaves` |
| `cosmetics/tung_hac` | Tùng Hạc | P1 | `folding screen painted with an old pine and a white crane, ink-wash, no inscription` |
| `cosmetics/canh_mai` | Mai Tuyết (Đông — mùa hiện tại) | P1 | `vase with a blooming plum branch dusted with snow` |
| `cosmetics/lo_suoi` | Mai Tuyết | P1 | `small bronze brazier with glowing embers` |
| `cosmetics/binh_phong_tuyet` | Mai Tuyết | P1 | `folding screen of white silk painted with layered snowy mountains, no inscription` |
| `cosmetics/van_hac` | Tiên Duyên | P1 | `miniature pavilion floating on a cloud with a pair of cranes on the roof` (quà Tiên Duyên, phải trông xứng tầm) |
| `cosmetics/canh_dao` | Đào Hoa (Xuân) | P2 | `vase with a flowering peach branch` — chờ mùa Xuân (tháng 2) |
| `cosmetics/den_long_xuan` | Đào Hoa | P2 | `round red paper lantern with peach-blossom painting, no characters` |
| `cosmetics/binh_phong_dao` | Đào Hoa | P2 | `folding screen painted with a peach-blossom valley and a stream` |
| `cosmetics/chau_sen` | Liên Trì (Hạ) | P2 | `jade bowl with lotus leaves and one pink lotus` (vật trang trí cây cỏ, không phải đài sen tôn giáo) |
| `cosmetics/quat_truc` | Liên Trì | P2 | `hanging round bamboo fan with a tassel` |
| `cosmetics/tieu_lien_tri` | Liên Trì | P2 | `tiny stone pond with lily pads and a goldfish` |
| `cosmetics/den_trang` | Thu Nguyệt (Thu) | P2 | `round white paper lantern like a full moon, no characters` |
| `cosmetics/cuc_vang` | Thu Nguyệt | P2 | `clump of yellow chrysanthemums in a clay pot` |
| `cosmetics/tranh_nguyet` | Thu Nguyệt | P2 | `hanging scroll painting of a full moon over mountains, no calligraphy` |
| `cosmetics/ngoc_bai_tu_tieu` | Tử Tiêu (sự kiện) | P2 | `violet jade plaque on a stand with cloud carving` — mô tả game nói "khắc hai chữ Tử Tiêu": **vẽ trơn/hoa văn, không chữ** |
| `cosmetics/bo_doan` | Tử Tiêu | P2 | `round woven violet rush meditation cushion` |
| `cosmetics/hoanh_phi_dao` | Tử Tiêu | P2 | `horizontal wooden lacquered board with cloud frame` — mô tả game có 3 chữ: **để trống / vân mây, không chữ** |
| `cosmetics/co_phong_than` | Phong Thần (sự kiện) | P2 | `tall ceremonial banner on a pole, abstract cloud pattern, no writing` |
| `cosmetics/den_luu_ly` | Phong Thần | P2 | `ornate glazed-glass lantern with eight-sided frame, generic jewel ornaments` (tránh "bát bảo" Phật giáo) |
| `cosmetics/dinh_tam_chan` | Phong Thần | P2 | `ancient three-legged bronze ding cauldron with taotie-free cloud bands, faint smoke` |

### 3.3 Trang trí thế lực (`faction_works.yaml`, 12)

Thấy ở Thế Lực → Công trình (size 44 / 24). Fallback `DecorationGlyph`. **256×256 (+@2x 512×512), alpha**, cùng mẫu prompt
như 3.2 nhưng có thể góc 3/4. Tất cả **P2** — chỉ mở khi cả thế lực góp đủ mốc (100 k → 30 M), ảnh hiển thị rất nhỏ.

| Slot | Công trình | [MÔ TẢ] |
|---|---|---|
| `cosmetics/cot_da_son_mon` | Bài Phường Sơn Môn | `weathered stone pillar with carved cloud bands, no inscription` |
| `cosmetics/su_tu_da` | Bài Phường | `pair of stone guardian lions on plinths` |
| `cosmetics/bai_phuong` | Bài Phường | `stone paifang archway with tiled roof, blank lintel` |
| `cosmetics/co_duong_nui` | Cờ Hiệu | `row of small vertical pennants along a mountain path, plain colours` |
| `cosmetics/co_dai_dien` | Cờ Hiệu | `great banner on a tall pole before a hall, abstract emblem, no writing` |
| `cosmetics/co_hieu_van` | Cờ Hiệu | `many banners fluttering on a ridge` |
| `cosmetics/den_hanh_lang` | Đèn Trường Minh | `string of lanterns along a covered walkway` |
| `cosmetics/den_thap` | Đèn Trường Minh | `tall multi-tier stone lantern tower` |
| `cosmetics/den_truong_minh` | Đèn Trường Minh | `cluster of countless glowing lanterns rising into the night` |
| `cosmetics/nen_lau` | Vọng Nguyệt Lâu | `white marble terrace foundation with balustrade` |
| `cosmetics/lau_hai_tang` | Vọng Nguyệt Lâu | `two-storey pavilion tower` |
| `cosmetics/vong_nguyet_lau` | Vọng Nguyệt Lâu | `tall moon-viewing tower under a full moon` |

---

## 4. Ngoài phạm vi yêu cầu nhưng còn thiếu trên đĩa

Các slot có dòng README + code đang tra, file chưa có.

| Slot | Kích thước | Alpha | Ưu tiên | Ghi chú prompt / lý do |
|---|---|---|---|---|
| `scenes/dong-phu` | 1600×1200 (4:3, không @2x) | không | **P0** | Màn hình người chơi nhìn nhiều nhất (Động Phủ, `dong_phu.ts`); hiện chỉ là quầng ngọc. `cultivator's cave interior, carved stone walls with moss and glowing teal veins, small spirit spring, hanging talismans without writing, floor line along the bottom edge, centre left open and darker for the meditating figure, wall space top-left/top-right and floor space bottom-left/bottom-right for decorations`. **Sửa artgen pattern trước** (đang ép 16:9). |
| `items/tu_linh_dan` | 128² (+@2x 256²) | có | **P0** | Đan bán trong shop, ai cũng thấy. `single game item icon, round pill glowing jade-green with swirling qi, in a small celadon dish`. |
| `items/thanh_dong_kinh` | 128² (+@2x) | có | P1 | Pháp bảo rơi từ bí cảnh. `small round verdigris bronze chest mirror on a cord, cloud-thunder engraving` |
| `items/ty_loi_chau` | 128² (+@2x) | có | P1 | `cool translucent blue pearl beaded with water droplets, faint crackling light around it` |
| `items/loi_van_phu` | 128² (+@2x) | có | P1 | `yellow paper talisman with abstract lightning-line pattern, no characters, edges smouldering` |
| `items/huyen_vu_thuan` | 128² (+@2x) | có | P1 | `round black tortoise-shell shield with old cracks glowing warm` |
| `items/cuu_tieu_hoan` | 128² (+@2x) | có | P1 | `nine-layered ring of clouds and pale gold light orbiting a centre` |
| `factions/huyen_khung_mon` (Tam Thanh) | 512² (+@2x 1024²) | có | **P0** | 5 thế lực mở từ `khai_giang`: thấy ở bước chọn thế lực khi tạo nhân vật. Prompt theo `art-prompts.md` §7.3 #3 + mục "Huy hiệu" trong `content/story/factions/<id>.md`. Motif chung, không biểu tượng tôn giáo. |
| `factions/kim_sa_mon` (Tây Phương) | 512² (+@2x) | có | **P0** | Cồn cát vàng 3 lớp (story file). Tránh mọi ký hiệu Phật giáo. |
| `factions/thuong_lan_dinh` (Tiên Đình) | 512² (+@2x) | có | **P0** | Sóng xanh + mái lầu vàng (story file). |
| `factions/tinh_ha_yeu_dinh` (Thiên Đình Yêu Tộc) | 512² (+@2x) | có | **P0** | Cung 7 sao tím + lông vũ (story file). |
| `factions/vu_toc` | 512² (+@2x) | có | **P0** | Khiên tròn, khối núi ba đỉnh son (story file). |
| `factions/thu_nhat_vien` (Nhân Giáo) | 512² (+@2x) | có | P1 | Mở ở `thanh_nhan`; chưa có file story/huy hiệu → cần story-creator viết mục "Huy hiệu" trước. |
| `factions/lap_cuc_vien` (Xiển Giáo) | 512² (+@2x) | có | P1 | như trên |
| `factions/van_luu_vien` (Tiệt Giáo) | 512² (+@2x) | có | P1 | như trên |
| `factions/kim_sa_giao` (Tây Phương Giáo) | 512² (+@2x) | có | P1 | như trên; không biểu tượng Phật giáo |
| `factions/nhan_toc` | 512² (+@2x) | có | P1 | Mở ở `tao_nhan`; như trên |
| `world/sky` | 1920×1080 | không | P1 | **Thay** `world/map` bằng bản parallax (§1 art-prompts). Có `world/sky` là bản đồ tự chuyển sang chế độ lớp → làm đủ cả 4 lớp cùng đợt. |
| `world/far` | 1920×1080 | có | P1 | Lớp 2, núi xa |
| `world/mid` | 1920×1080 | có | P1 | Lớp 3, núi gần + mây + thác; mép ngoài bỏ được (overscan 3%) |
| `world/ground` | 1920×1080 | có | P1 | Lớp 4, chừa đất trống ở vị trí công trình (`nav.ts` toạ độ %) |
| `landmarks/tu-tieu-cung` | 512² (+@2x) | có | P1 | Chỉ dùng ở chế độ lớp. Prompt §2 art-prompts |
| `landmarks/dong-phu` | 512² (+@2x) | có | P1 | "talismans hanging" → không chữ |
| `landmarks/the-luc` | 512² (+@2x) | có | P1 | banner không chữ |
| `landmarks/dao-hoa-dinh` | 512² (+@2x) | có | P1 | |
| `landmarks/bi-canh` | 512² (+@2x) | có | P1 | "floating runes" → ký hiệu trừu tượng, không chữ thật |
| `landmarks/chien-truong` | 512² (+@2x) | có | P1 | game tự làm xám khi khoá |

---

## 5. Điểm bất thường phát hiện khi đối chiếu

1. **`meditate-robe` có trong `looks.ts` (`maskSlot` kind `'meditate'`) và proposal §6 (P1, 12 ảnh) nhưng không có dòng
   README và không component nào gọi** — chỉ `Portrait.svelte` gọi `maskSlot(…, 'avatar')`.
2. **artgen thiếu pattern** cho `cosmetics/*`, `fx/bt-*`, `fx/nascent`; và pattern `scenes/*` ép 16:9 trong khi
   `scenes/dong-phu` là 4:3 (README + `dong_phu.ts`).
3. artgen có pattern `characters/<body>/<outfit>/avatar-robe` nhưng cả README lẫn code chỉ dùng mặt nạ **cấp body**.
4. Lớp ăn mừng `fx/bt-*` và `fx/nascent` chưa có điểm tra slot nào trong `BreakthroughMode.svelte`; thả file vào sẽ
   **không** tự hiện như các slot khác — cần sửa code + README cùng lúc.
5. Ledger còn ghi lần thử `characters/vu_nu_luc_si/luc_si_hoang_tho/body-b6` (17:10) cùng đợt bị chặn, nhưng file đó
   **đã có** trên đĩa; chỉ `y_su_hoang_tho/body-b6` thực sự thiếu, khớp với README.
6. Proposal §6 còn liệt kê `ui/card-frame-<1..6>` (P1) và `fx/gate` — chưa có code/README tương ứng; không đưa vào
   hàng đợi (fx/gate được thay bằng `fx/bt-gate` + `fx/bt-gate-door` ở mục 2).
7. Mô tả game của `ngoc_bai_tu_tieu` và `hoanh_phi_dao` nói vật có khắc chữ — mâu thuẫn với luật "không chữ" của ảnh;
   vẽ trơn, chữ (nếu cần) để UI vẽ chồng.
