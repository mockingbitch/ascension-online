# Prompt: Ngoại hình nhân vật, hướng dẫn viên tân thủ, hiệu ứng lên cấp theo cảnh giới, thông báo toàn server

> Dán nguyên văn phần dưới vào một phiên Claude Code mới ở thư mục gốc repo `tu-tien-online`.

---

## 1. Vai trò

Bạn là Game Art Director kiêm Senior Game Developer của **Tiên Đạo Online** (idle tu tiên, SvelteKit + Go). Nhiệm vụ:
tăng **tính cá nhân hóa** và **cảm giác tiến bộ bằng hình ảnh**. Người chơi chọn được nhân vật riêng. Một nữ hướng dẫn
viên dẫn tân thủ. Mỗi lần lên cấp có hình ảnh và hoạt ảnh, cảnh giới càng cao càng đẹp và nổi bật. Cả server được báo
khi có người đột phá.

Đây là việc thật trong code: dữ liệu, server, giao diện, ảnh, hiệu ứng, test. Không dừng ở bản mô tả.

## 2. Đọc trước khi làm (bắt buộc)

- `CLAUDE.md`, `docs/decisions.md` (mọi mục mới nhất, nhất là D2 đã bị đảo ngược ngày 2026-11-07, Tiên Duyên,
  Client webgame, UI regions & layers).
- `.claude/rules/frontend.md`, `graphic-design.md`, `architecture.md`, `data-structures.md`, `story-writing.md`.
- Ảnh: `apps/web/src/lib/assets/art/README.md` (các ô ảnh, kích thước, SVG dự phòng), `apps/web/src/lib/game/art.ts`,
  `docs/design/art-prompts.md` (style chung, negative prompt, character sheet). Agent `art-generator` tạo ảnh raster.
- Ngoại hình hiện có: `content/catalog/cosmetics.yaml`, `services/game/internal/cosmetic/`, bảng `character_appearance`
  (migration 000014), `apps/web/src/lib/game/cosmetics.svelte.ts`, `components/Portrait.svelte`,
  `components/CultivatorFigure.svelte`.
- Hiệu ứng: `apps/web/src/lib/engine/fx/` (`breakthrough.ts`, `plan.ts`, `director.ts`, `presets.ts`, `ParticleSystem`),
  `engine/mode/breakthrough.svelte.ts`, `BreakthroughMode.svelte`, `engine/audio/` (`manifest.ts`, `sfxMap.ts`).
- Cảnh giới: `content/catalog/realms.yaml` (`visual.aura`, tier, đạo đồ tiên/Vu), token aura trong `app.css`.
- Thông báo: `content/story/announcements/*.yaml`, `services/game/internal/realtime/`, worker, `packages/contracts/ws-messages.md`.
- Tân thủ: `apps/web/src/lib/game/onboarding.ts`, `content/story/onboarding/`, `lib/game/help.ts`, `components/HelpButton.svelte`.

## 3. Ràng buộc không được phá

1. **Chỉ ngoại hình, không sức mạnh.** Ngoại hình, hào quang, hoạt ảnh, danh hiệu không bao giờ đổi tốc độ tu luyện,
   tỉ lệ đột phá/lôi kiếp, điểm xếp hạng hay trần 150 điểm/ngày. Loader phải từ chối mọi thuộc tính khác ngoài hiển thị.
2. **Server quyết định.** Hiệu ứng lên cấp chỉ chạy **sau** phản hồi của server, theo giá trị server (`fxBus`). Client
   không tự tính cấp. Chọn ngoại hình là *intent*; server kiểm tra quyền sở hữu, ghi idempotent (`Idempotency-Key`).
3. **Tiên Duyên:** chỉ được mở *ngoại hình bổ sung*. Hình ảnh của người nạp không được trông như mạnh hơn (rule
   graphic-design "VIP visuals must not imply power advantage"). Hào quang theo **cảnh giới** (do tu luyện) luôn nổi bật
   hơn mọi món mua được.
4. **Nội dung nguyên bản.** Không bắt chước nhân vật, trang phục, logo hay UI của game hoặc phim có thật. Nhân vật tôn
   giáo đang được thờ phụng theo đúng chính sách hiện hành trong `decisions.md`.
5. **Hiệu năng & layer:**
   - Một vòng `gameLoop()`; hạt chỉ qua `ParticleSystem` (≤ 300 full / ≤ 80 lite / 0 off).
   - Ba mức `settings.fx` (full/lite/off); tôn trọng `prefers-reduced-motion` (không flash, không rung).
   - Màn ăn mừng ≤ 3 s, bỏ qua được, kết thúc ở trạng thái tĩnh.
   - z-index chỉ dùng token `--z-*`; không mã màu hex trong component.
   - JS ban đầu của `/play` tăng ≤ 60 kB gz mỗi giai đoạn; ảnh và hiệu ứng nặng tải lười.
   - 3D chỉ sau cờ `renderer=3d`.
6. **Ảnh:** mọi ảnh mới có một ô trong `art.ts` / README, có **SVG dự phòng** để game chạy khi chưa có ảnh. Kích thước
   ×1 và `@2x`, AVIF/WebP/PNG, có alpha khi cần. Tạo bằng `art-generator`, ảnh nào lệch art direction thì tạo lại.
7. Số liệu ở YAML, không hardcode. Đổi API thì cập nhật `openapi.yaml` + `types.ts`. Text game bằng tiếng Việt.

## 4. Tính năng

### A. Đa dạng nhân vật: người chơi tự chọn

- **Catalogue `content/catalog/appearances.yaml`** (loader + validator trong Go, mirror TS):
  - **Dáng người:** tối thiểu 4 nam + 4 nữ, khác nhau rõ về khuôn mặt, kiểu tóc, vóc dáng, khí chất (thư sinh, kiếm
    khách, ẩn sĩ, tiểu thư thế gia…). Đạo đồ Vu có bộ riêng mang chất luyện thể (tối thiểu 2 nam + 2 nữ).
  - **Kiểu tóc / phụ kiện tóc, màu y phục:** màu y phục lấy theo bảng màu thế lực hoặc chọn tự do trong bảng token.
  - **Tư thế thiền** (2–3 biến thể).
  - Mỗi lựa chọn có `id` ASCII bất biến, tên hiển thị tiếng Việt, cờ `default_free: true` cho bộ cơ bản.
- **Dữ liệu:** migration mới thêm cột vào `character_appearance` (`body_id`, `hair_id`, `robe_palette_id`, `pose_id`)
  hoặc bảng riêng, kèm CHECK và tham chiếu catalogue. Bộ cơ bản miễn phí cho mọi người.
- **Chọn ở đâu:**
  - Bước mới trong luồng tân thủ (sau thế lực/đạo đồ, trước đạo hiệu), xem trước sống.
  - Đổi lại sau trong cửa sổ **Nhân Vật → Trang phục** ("Gương Thủy Nguyệt"), đổi miễn phí, có thời gian chờ nhẹ nếu
    cần chống spam.
- **Hiển thị khắp nơi:** `Portrait` (HUD, chat, xếp hạng, đạo lữ), cảnh Tu Luyện/Động Phủ, Tử Tiêu Cung (chỗ ngồi),
  thẻ hồ sơ.
- **Ô ảnh mới** (mở rộng README):
  - `characters/<body_id>/avatar` (256²), `/meditate` (1024²), `/body-<realm_id>` (1024×1536).
  - Biến thể tóc/màu làm bằng lớp overlay hoặc tô màu theo token nếu khả thi, để không nhân số ảnh lên quá nhiều.
  - Mỗi dáng người làm trước một **character sheet** để các ảnh đồng nhất.
- **SVG dự phòng:** tham số hóa `CultivatorFigure` (bóng dáng nam/nữ, tóc, màu áo theo token) để chạy được trước khi
  có ảnh.

### B. Cá nhân hóa thêm

- **Pháp tướng / hào quang** mở theo cảnh giới đã đạt (không mua được), cho người chơi chọn hiển thị cái nào trong các
  cái đã mở.
- **Thẻ hồ sơ** (đạo hiệu, cảnh giới, thế lực, danh hiệu, khung, dáng người) xem được khi bấm vào người khác. Có nút xuất
  ảnh thẻ để chia sẻ; ảnh xuất **không** chứa email hay dữ liệu cá nhân.
- Giữ nguyên danh hiệu, khung avatar, hào quang chat, trang trí động phủ hiện có. Gom tất cả vào một tab **Trang phục**
  thống nhất.

### C. Nữ hướng dẫn viên tân thủ

- **Nhân vật nguyên bản:**
  - Một nữ tu sĩ đã trưởng thành, ôn hòa, hơi dí dỏm. Đề xuất 3 phương án tên và tính cách để chủ dự án chọn trước khi
    tạo ảnh.
  - Trang phục kín đáo, thanh nhã; **không gợi dục**, không trẻ hóa.
  - Không là nhân vật thờ phụng, không lấy từ tác phẩm có thật.
  - Lore ở `content/story/characters/<id>.md` (front-matter theo story-writing rules).
- **Ảnh:**
  - Character sheet → 5–6 biểu cảm: bình thường, mỉm cười, chỉ dẫn, lo lắng, tự hào, ngạc nhiên.
  - Ô ảnh `characters/guide/<expr>`, nửa người, alpha, ~768×1024.
  - SVG dự phòng.
  - Hoạt ảnh nhẹ bằng CSS (thở, chớp mắt bằng đổi ảnh, nghiêng đầu), tắt theo `settings.fx`.
- **Component `TutorialGuide.svelte`:**
  - Bảng hội thoại kiểu visual novel ở mép màn hình, không che nút người chơi cần bấm. Tôn trọng regions/layers.
  - Có thể làm nổi một vùng UI (spotlight vào `data-tutorial="..."`).
  - Bỏ qua được và "không nhắc lại". Điều khiển được bằng bàn phím và trình đọc màn hình.
- **Kịch bản** (`content/story/tutorial/*.yaml`, mỗi câu ≤ 200 ký tự, giọng xưng hô nhất quán):
  - Luồng mở đầu và tạo nhân vật.
  - Lần đầu: thu tu vi, đột phá tầng/cảnh giới, lôi kiếp (giải thích mất 25% và bảo hiểm 3 lần), thế lực, giảng đạo,
    cửa hàng, mini game, Tiên Duyên (nói rõ không tăng sức mạnh), đạo lữ.
- **Tiến độ hướng dẫn lưu ở server** (bảng nhỏ `tutorial_progress`, ghi idempotent) để đổi máy vẫn giữ. Bước nào
  kích hoạt được quyết bằng sự kiện server trên `fxBus`, không đoán ở client.
- Nối với `help.ts`: nút "?" có thể gọi cô ấy giải thích lại.

### D. Hình ảnh và hoạt ảnh lên cấp: cảnh giới càng cao càng đẹp

- **Thang hình ảnh theo tier** trong `realms.yaml` → `visual` (mở rộng, có validator):
  `fx_tier` (1–6), bảng màu, preset hạt, biến thể trận pháp, kiểu chữ thư pháp, cue âm thanh, hào quang thường trực.
- **Định hướng leo thang** (đề xuất, có thể điều chỉnh, mỗi bậc phải rõ ràng *hơn* bậc trước):

| Bậc | Cảnh giới | Đột phá cảnh giới | Hào quang thường trực |
|---|---|---|---|
| 1 | Luyện Khí | Sương linh khí tụ lại, vòng sáng mỏng | Hạt sương nhỏ |
| 2 | Trúc Cơ, Kim Đan | Vòng ngọc bích, kim đan phát sáng giữa đan điền | Vòng ngọc / ánh vàng |
| 3 | Nguyên Anh, Hóa Thần | Bóng nguyên anh hiện sau lưng, trận pháp hai tầng | Bóng mờ nguyên anh |
| 4 | Luyện Hư → Độ Kiếp | Lôi kiếp (đã có) + mây đen tan, cột sáng | Tia điện nhỏ quanh thân |
| 5 | Nhân Tiên → Kim Tiên | Mây lành ngũ sắc, cánh cổng trời, chữ vàng cỡ lớn | Mây lành dưới chân |
| 6 | Thái Ất → Chuẩn Thánh | Tinh hà xoay, pháp tướng khổng lồ, cả màn hình rung nhẹ (full) | Tinh quang + pháp tướng |

- **Tầng nhỏ tự lên:** hiệu ứng nhỏ, không chặn thao tác (toast + vệt sáng trên thanh tu vi). Gom nhiều tầng lên cùng lúc
  thành một lần hiển thị.
- **Đột phá cảnh giới & lôi kiếp:** mở rộng `fx/breakthrough.ts` / `plan.ts` cho đủ 6 bậc.
  - Hàm thuần, unit test theo bậc và theo mức fx.
  - Vẫn ≤ 3 s, bỏ qua được.
  - Lite giảm hạt/thời gian, off = thẻ tĩnh, reduced motion = không flash/rung.
- **Ảnh theo cảnh giới:** `characters/<body_id>/body-<realm_id>` (y phục và khí chất lên dần theo cảnh giới); lớp hào
  quang overlay `fx/aura-<tier>`; trận pháp SVG theo bậc; khung thẻ thăng cấp theo bậc.
- **Âm thanh:** cue theo bậc trong `audio/manifest.ts` + `sfxMap.ts` (nhạc cụ trầm lên dần: chuông, khánh, cổ cầm, hợp
  xướng). Thông tin không phụ thuộc âm thanh.
- **Thẻ thành tựu cảnh giới:** sau màn ăn mừng hiện thẻ tĩnh (ảnh nhân vật ở cảnh giới mới, ngày đạt, thứ hạng). Có nút
  xuất ảnh để chia sẻ.

### E. Thông báo toàn server khi có người đột phá

- Đã có mẫu `major_breakthrough`, `first_to_realm`, `rank_top10` và WS `/ws`. Mở rộng như sau.
- **Sự kiện phát:** đột phá cảnh giới (major), vượt lôi kiếp, người đầu tiên đạt cảnh giới, vào top 10. **Không** phát
  khi lên tầng nhỏ.
- **Đường đi:** server ghi `outbox_events` trong transaction đột phá → worker → Redis pub/sub → các instance API → client.
  At-most-once, client khử trùng theo id. Text lấy từ `content/story/announcements/*.yaml`; thêm mẫu theo bậc,
  ≤ 90 ký tự sau khi thay biến.
- **Mức độ nổi bật theo bậc:**
  - Bậc 1–2: dòng ticker.
  - Bậc 3–4: banner trượt có biểu tượng cảnh giới.
  - Bậc 5–6 và "người đầu tiên": banner lớn toàn chiều ngang ở top layer, màu aura cảnh giới, âm thanh, hạt nhẹ.
  - Không che nút người khác đang bấm, tự tắt sau vài giây, xem lại được trong danh sách thông báo.
- **Chống spam:** giới hạn mỗi người chơi và tổng tần suất. Gom nhiều thông báo cùng lúc. Người chơi tắt được
  banner/âm thanh của thông báo người khác trong Cài đặt (ticker luôn còn).
- Thông báo cũng vào kênh chat dưới dạng tin hệ thống. Chỉ hiện đạo hiệu, không lộ dữ liệu cá nhân.
- Cập nhật `packages/contracts/ws-messages.md`.

## 5. Cách làm việc

1. **Khảo sát và đề xuất (dừng chờ duyệt):**
   - Đọc mục 2.
   - Liệt kê những gì đã có và có thể tái dùng.
   - Đề xuất: danh sách dáng người, 3 phương án nữ hướng dẫn viên, bảng 6 bậc hiệu ứng, kế hoạch ô ảnh (số lượng ảnh,
     kích thước), thay đổi dữ liệu/API.
   - Liệt kê **quyết định cần chủ dự án chốt**, ví dụ: ngoại hình nâng cao có bán bằng linh thạch / mở bằng Tiên Duyên
     không, tên hướng dẫn viên, có cho đổi dáng người miễn phí không.
2. **Theo giai đoạn**, mỗi giai đoạn chạy được độc lập:
   - **GĐ-A:** dữ liệu ngoại hình + SVG dự phòng + bước chọn trong tân thủ + tab Trang phục.
   - **GĐ-B:** hướng dẫn viên + kịch bản tân thủ + lưu tiến độ.
   - **GĐ-C:** thang hiệu ứng 6 bậc + hào quang thường trực + thẻ thành tựu.
   - **GĐ-D:** thông báo toàn server theo bậc + cài đặt tắt banner.
   - **GĐ-E:** tạo ảnh raster bằng `art-generator` (character sheet trước, rồi các biến thể), duyệt từng ảnh theo art
     direction, tạo lại ảnh lệch.
3. **Sau mỗi giai đoạn:**
   - Chạy `go build/vet`, `make test`, `make test-integration`, `make content-check`, `cd apps/web && npm run check &&
     npm test && npm run build`.
   - Mở game thật và chụp màn hình ở 360×740, 812×375, 1024×768, 1440×900 cho từng mức fx (full/lite/off). Kiểm tra
     không có gì bị che, không lỗi console, đo kích thước bundle `/play`.
   - Thay đổi server nào đụng ngoại hình, tiến độ tân thủ hay thông báo thì chạy `anti-cheat-reviewer`.
   - Báo cáo: đã làm gì, ảnh chụp, việc còn lại, quyết định cần chốt.
4. Không commit trừ khi được yêu cầu. Không chạy migration lên database dev của chủ dự án; dùng database thử riêng.

## 6. Tiêu chí hoàn thành

- Tân thủ chọn được dáng người (≥ 8 tiên, ≥ 4 Vu), đổi lại được. Ngoại hình hiện đúng ở mọi nơi có chân dung.
- Nữ hướng dẫn viên dẫn trọn luồng tân thủ và các lần "đầu tiên", bỏ qua được, nhớ tiến độ khi đổi máy.
- Sáu bậc hiệu ứng khác nhau rõ, bậc sau nổi bật hơn bậc trước. Đủ ba mức fx; ≤ 3 s; không vượt ngân sách hạt/bundle.
- Thông báo toàn server hiện theo bậc, không spam, tắt được banner. Không lộ dữ liệu cá nhân.
- Không cái gì trong số này đổi sức mạnh, tỉ lệ hay xếp hạng; validator và test chứng minh điều đó.
- Mọi ô ảnh có SVG dự phòng; mọi ảnh raster đã qua duyệt art direction; không ảnh/URL bên ngoài.
