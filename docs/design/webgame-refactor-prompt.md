# Prompt: Refactor Tiên Đạo Online thành webgame (2D bây giờ, sẵn sàng 3D sau)

> Dán toàn bộ phần dưới cho agent/dev thực hiện. Làm theo từng giai đoạn; mỗi giai đoạn có tiêu chí "xong khi" và phải
> gửi ảnh chụp/quay màn hình 375 px + 1440 px trước khi sang giai đoạn sau.

---

## 0. Bối cảnh

Bạn đang làm việc trong repo `ascension-online` (Tiên Đạo Online): idle cultivation MMORPG, web SvelteKit + Svelte 5
(`apps/web`), máy chủ Go (`services/game`). Đọc trước và tuân thủ: `CLAUDE.md`, `.claude/rules/*.md` (đặc biệt
`frontend.md`, `graphic-design.md`, `architecture.md`), `docs/decisions.md` (mới nhất trước), `apps/web/src/app.css`
(design tokens), `apps/web/src/lib/game/*` (store, nav, art), `apps/web/src/routes/(game)/play/**`.

**Hiện trạng:** shell một trang (`routes/(game)/play/+layout.svelte`: HUD, chat cột phải, dock dưới) và mỗi tính năng là
một route con hiển thị trong khung `ScreenFrame` (thanh tiêu đề + nội dung chữ/bảng). Kết quả: cảm giác **website nhiều
trang**, không phải game. Cụ thể:
- Mỗi tính năng là một "trang" chữ/bảng; luật chơi viết thành đoạn dài; mini game nằm trong khung trang.
- Bản đồ thế giới biến mất khi đổi màn; địa điểm (Tử Tiêu Cung, Động Phủ) và menu (Túi Đồ, Nhân Vật) bị đối xử như nhau.
- Chat là cột cố định ~25% chiều ngang (desktop); dock là thanh tab có chữ.
- Không có loading/boot, âm thanh, chuyển cảnh, hoạt ảnh phần thưởng, toàn màn hình.

**Mục tiêu:** biến client thành **webgame** — một cảnh luôn sống phía sau, menu là cửa sổ đè lên cảnh, mini game chiếm
toàn màn hình, nhiều hoạt ảnh/phản hồi — và kiến trúc sao cho **sau này thay renderer 2D bằng 3D (WebGL)** mà không viết
lại logic game hay UI.

## 1. Ràng buộc không được phá (non-negotiable)

1. **Server authoritative** giữ nguyên: client chỉ gửi ý định; mọi số liệu (tu vi, phần thưởng, tỉ lệ) lấy từ phản hồi
   server và **thay thế** state. Hoạt ảnh chỉ *trình diễn* kết quả server đã trả — không bao giờ tự tính phần thưởng.
   Nội suy hiển thị (thanh tu vi chạy giữa hai snapshot) được phép nếu bị phản hồi kế tiếp sửa lại.
2. **Không đổi API/DB** trong refactor này (trừ khi một mục dưới đây ghi rõ). Không đổi luật game, số liệu catalogue.
3. **Design tokens** trong `app.css` là nguồn duy nhất cho màu/font/motion; không hex/rgb thô trong component.
4. **Accessibility**: `prefers-reduced-motion` tắt mọi hoạt ảnh không thiết yếu (đã có rule toàn cục — không override);
   thêm tùy chọn "Hiệu ứng: Đầy đủ / Nhẹ / Tắt" trong Cài đặt. Tương phản ≥ 4.5:1, tap target ≥ 40 px, điều khiển bàn
   phím cho mọi cửa sổ (Esc đóng, focus trap), không truyền trạng thái chỉ bằng màu.
5. **Hiệu năng** (đo trên Android tầm trung + laptop):
   - 60 fps khi chỉ có cảnh idle; không dưới 45 fps khi đang chạy hiệu ứng lớn.
   - **Một** vòng `requestAnimationFrame` duy nhất cho toàn bộ hiệu ứng canvas; tạm dừng khi tab ẩn
     (`visibilitychange`) và khi cửa sổ che kín cảnh.
   - Chỉ animate `transform`/`opacity` cho DOM; không animate layout/box-shadow trên nhiều node.
   - Giới hạn `devicePixelRatio` của canvas ở 2; pool đối tượng hạt, không cấp phát mỗi frame; ≤ 300 hạt cùng lúc
     (≤ 80 ở chế độ "Nhẹ").
   - Bundle: tải lười (dynamic import) từng cảnh, mini game, âm thanh; JS ban đầu của `/play` không tăng quá +60 kB gz.
6. **UI tiếng Việt**, code/comment tiếng Anh. Không copy giao diện, asset, âm thanh của game có thật.
7. **Engine/WebGL:** `frontend.md` hiện cấm thêm engine canvas khi chưa có quyết định. Giai đoạn này **không thêm thư
   viện 3D/engine** (không PixiJS/Phaser/Three). Chỉ Canvas 2D thuần cho hiệu ứng hạt. Việc mở đường 3D làm bằng
   *kiến trúc* (mục 3) + ghi quyết định (mục 9); spike 3D là giai đoạn riêng, cần chủ dự án duyệt.

## 2. Bước 0 — sửa lỗi hiện có (làm trước)

- Mobile `/play/tu-luyen`: bảng chỉ số đè lên nút "Thu nhận tu vi"/"Đột phá".
- Mobile `/play/nhan-vat`: hiện "Chưa gia nhập thế lực" trong khi HUD/desktop hiện thế lực đúng (kiểm tra thứ tự tải
  dữ liệu thế lực / race điều kiện).
- Viết test Playwright smoke cho cả hai.

## 3. Kiến trúc mục tiêu

### 3.1 Ba lớp, tách bạch

```
GameRoot (routes/(game)/play/+layout.svelte)
├── SceneLayer     ← luôn mounted; vẽ cảnh hiện tại qua một SceneRenderer (2D bây giờ, 3D sau)
├── FxLayer        ← canvas hiệu ứng toàn màn (hạt, tia, chớp, chữ bay) + overlay DOM (banner, số nhảy)
├── UiLayer        ← HUD, thanh hành động, chat nổi, cửa sổ (window stack), toast, modal xác nhận
└── ModeLayer      ← chế độ toàn màn (mini game, đột phá/lôi kiếp, cảnh mở đầu) — ẩn HUD/dock/chat
```

- Đặt mã engine phía client trong `apps/web/src/lib/engine/` (mới):
  - `scene/` — `SceneManager`, định nghĩa cảnh, chuyển cảnh.
  - `render/` — interface `SceneRenderer` + `render2d/` (DOM/CSS/SVG + Canvas2D). Chừa `render3d/` (chỉ README, chưa code).
  - `fx/` — `FxBus` (sự kiện), `ParticleSystem` (Canvas2D, pool), các preset hiệu ứng.
  - `audio/` — `AudioManager` (Web Audio API, không thư viện), kênh BGM/SFX/UI, âm lượng, mute, tải lười.
  - `input/` — phím tắt, cử chỉ, haptic (`navigator.vibrate` khi có).
  - `loop.ts` — vòng rAF duy nhất, đăng ký/huỷ subscriber, đo fps (overlay dev `?fps=1`).
- `lib/game/store.svelte.ts` và các store domain **giữ vai trò nguồn dữ liệu**; thêm phát sự kiện ngữ nghĩa vào
  `FxBus` sau khi phản hồi server về: `settled{earnedMilli}`, `reward{items,currencies,source}`,
  `breakthrough{result,kind,tribulation}`, `stageUp{from,to}`, `insight{earnedMilli,takeaway}`, `milestone{id}`,
  `chat{message}`, `trialResult{verdict}`, `purchase`, `equip`… FX/âm thanh chỉ nghe bus, **không** gọi API.

### 3.2 SceneRenderer — hợp đồng để thay 2D → 3D

```ts
interface SceneDef {
  id: 'world' | 'dong_phu' | 'tu_tieu_cung' | 'son_mon' | 'dao_hoa_dinh' | 'bi_canh_gate';
  layers: LayerDef[];                 // 2D: ảnh/SVG theo lớp parallax; 3D sau: model/skybox
  anchors: Record<string, Anchor>;    // điểm tương tác/neo hiệu ứng: toạ độ chuẩn hoá {x,y} (0..1) + z tuỳ chọn
  hotspots: Hotspot[];                // vùng bấm → hành động (mở cửa sổ, đổi cảnh)
  camera?: CameraPreset[];            // 2D: pan/zoom; 3D: vị trí/góc
  ambient?: AmbientDef[];             // mây trôi, đèn lồng, chim, linh khí… (dữ liệu, không code riêng)
  music?: string; ambience?: string;  // id âm thanh
}
interface SceneRenderer {
  mount(el: HTMLElement, def: SceneDef, opts: { quality: 'full'|'lite'|'off' }): Promise<void>;
  unmount(): void;
  anchorToScreen(id: string): { x: number; y: number } | null;  // FX và UI dùng để neo hiệu ứng/bong bóng
  setCamera(preset: string, ms?: number): Promise<void>;
  pause(): void; resume(): void;
}
```

- Mọi chỗ cần vị trí trong cảnh (bảng địa danh, bong bóng chat trên đầu nhân vật, hạt linh khí bay từ Hồng Quân, điểm
  rơi phần thưởng) **chỉ** đi qua `anchors` + `anchorToScreen` — không hard-code pixel/% trong component. Đây là điều
  kiện để renderer 3D thay thế được.
- Chuyển dữ liệu toạ độ hiện có (`nav.ts` `mapLocations`, `ROWS` ghế Tử Tiêu Cung, `DAIS`) thành `SceneDef` dữ liệu
  trong `lib/engine/scene/defs/*.ts`.

### 3.3 Cảnh / Cửa sổ / Chế độ

| Loại | Thành phần | Hành vi |
|---|---|---|
| **Cảnh** | Thế Giới (bản đồ), Động Phủ (tu luyện), Tử Tiêu Cung, Sơn môn thế lực, Đào Hoa Đình, Cổng bí cảnh | Toàn màn, luôn có một cảnh. Đổi cảnh = chuyển cảnh có hiệu ứng (mục 4.2). Hotspot trong cảnh mở cửa sổ liên quan. |
| **Cửa sổ** | Nhân Vật, Túi Đồ (Hành trang/Cửa hàng/Trang phục/Trao đổi), Xếp Hạng, Thiên Đạo Sử, Đột Phá, Nhiệm vụ, Tiên Duyên, Thông báo, Đạo Lữ (khi ở cảnh khác), Thư, Cài đặt | Đè lên cảnh, cảnh phía sau mờ + tối nhẹ (không unmount). Desktop: bảng giữa/phải có thể kéo; mobile: bottom sheet kéo lên/xuống. Stack tối đa 2; Esc/Back/vuốt xuống đóng. |
| **Chế độ** | Tâm Ma, Hóa Phàm, Bí cảnh, chuỗi đột phá/lôi kiếp, cảnh mở đầu | Chiếm toàn màn, ẩn HUD/dock/chat, có nút tạm dừng/thoát; đếm ngược 3-2-1; màn kết quả có hoạt ảnh phần thưởng; xoay ngang gợi ý trên mobile nếu cần. |

- **URL:** giữ deep link nhưng không cảm giác "chuyển trang": cảnh là route (`/play`, `/play/dong-phu`,
  `/play/tu-tieu-cung`…), cửa sổ là **shallow routing** (`pushState` + `page.state.win`, như `lib/game/windows.ts` đang
  làm cho một số cửa sổ). Back đóng cửa sổ trước rồi mới đổi cảnh. Mở link trực tiếp tới cửa sổ → dựng cảnh mặc định +
  mở cửa sổ đó.
- Bỏ `ScreenFrame` cho các màn đã thành cửa sổ/cảnh; tạo `GameWindow` mới (header nhỏ có icon + tên kiểu bảng gỗ/ngọc,
  tab bên trong, nút đóng) dùng chung.

### 3.4 UI kiểu game

- **HUD** giữ (đã có tu vi giữa); thêm hoạt ảnh số (mục 4.4). Chip tiền tệ có icon + số, bấm mở cửa sổ liên quan.
- **Thanh hành động:** icon tròn/ô vuông có badge; desktop đặt **dọc bên phải** (nhãn hiện khi hover), mobile hàng dưới
  chỉ icon + nhãn ngắn. Nút chính theo ngữ cảnh cảnh (Động Phủ: "Thu nhận"; Tử Tiêu Cung: "Ngộ đạo") nổi bật ở giữa.
- **Chat nổi** góc dưới trái: 3 tin mới nhất mờ dần sau 8 s; bấm để mở rộng thành khung có tab (Luận Đạo/Thế Giới — tái
  dùng `ChatPanel`). Bỏ cột chat cố định. Bong bóng chat vẫn hiện trên đầu nhân vật trong cảnh có nhân vật.
- **Bớt chữ:** luật chơi/giải thích chuyển vào nút "?" (popover/cửa sổ trợ giúp). Màn chính dùng icon, số lớn, thanh
  tiến độ, minh hoạ. Mọi số liệu vẫn từ server.
- **Empty state** có minh hoạ + gợi ý hành động (ví dụ Túi Đồ trống → "Vào bí cảnh để kiếm pháp bảo").

## 4. Danh mục hoạt ảnh & phản hồi (bắt buộc)

Thêm **motion tokens** vào `app.css` (`--dur-instant 90ms`, `--dur-fast 160ms`, `--dur-base 240ms`, `--dur-slow 420ms`,
`--dur-scene 650ms`, các easing `--ease-qi` (có sẵn), `--ease-out-back`, `--ease-in-out-sine`). Dùng Svelte transitions
và Web Animations API; hiệu ứng hạt dùng `ParticleSystem` (Canvas2D). Mỗi hiệu ứng có biến thể "Nhẹ" và "Tắt".

### 4.1 Khởi động
- **Boot/loading** khi vào `/play`: logo + vòng trận pháp quay, thanh tiến độ thật (font, catalogue, ảnh cảnh đầu, âm
  thanh UI), câu đạo ngữ ngẫu nhiên; tối đa 2 s nếu đã cache. "Chạm để vào" mở khoá âm thanh (autoplay policy).

### 4.2 Chuyển cảnh
- **Mây/mực phủ** (ink-wash wipe) 600–650 ms: lớp mây SVG/canvas trôi vào che, đổi cảnh, trôi ra; camera nhích nhẹ.
- Từ bản đồ vào địa điểm: zoom vào hotspot (scale + blur nền) rồi mây phủ.

### 4.3 Cảnh sống (ambient, chỉ chạy khi cảnh hiển thị)
- Bản đồ: mây trôi parallax 3 lớp, trăng hắt sáng, đèn lồng nhấp nháy, cánh đào rơi ở Đào Hoa Đình, tia sáng Tử Tiêu
  Cung "thở", chim bay ngang thi thoảng, hotspot sáng khi hover/được gợi ý.
- Động Phủ: nhân vật thở (scale 1→1.015), hào quang theo `visual.aura` của cảnh giới, linh khí hội tụ chậm về đan điền,
  khói nhang (nếu đã dâng hương hôm nay).
- Tử Tiêu Cung: hào quang Hồng Quân, hạt linh khí từ bục giảng về chỗ người chơi (đã có — chuyển sang `FxLayer`), phụ đề
  xuất hiện dần, bồ đoàn người mới vào "hiện" bằng khói.

### 4.4 HUD & phản hồi số
- **Số nhảy** (count-up) cho tu vi, linh thạch, điểm duyên khi server trả giá trị mới; màu xanh ngọc khi tăng.
- Thanh tu vi: dải sáng chạy dọc khi đang tích, nhấp sáng khi đầy (bình cảnh), rung nhẹ khi chạm trần ngày.
- Badge mới: nảy (scale 0.6→1.1→1) + chấm sáng.

### 4.5 Hành động chính
- **Thu nhận tu vi:** cầu linh khí bay từ nhân vật lên thanh tu vi trên HUD (đường cong Bézier, 600–900 ms, 8–24 hạt theo
  lượng `earnedMilli`), thanh tăng khi hạt chạm, chữ "+X tu vi" nổi lên. Lượng hạt ∝ log(giá trị) — chỉ để trình diễn.
- **Nhận thưởng** (nhiệm vụ, bí cảnh, mua hàng, quà Tiên Duyên): icon vật phẩm bật ra từ nguồn rồi **bay vào nút Túi Đồ**,
  nút nảy + badge; vật phẩm hiếm có viền hào quang theo `--color-rarity-*` và tia sáng xoay.
- **Ngộ đạo:** viền màn hình nhịp vàng khi thời khắc mở; bấm → vòng sáng loang từ nút, lời dạy hiện trên cuộn giấy mở ra,
  tu vi bay lên HUD.
- **Gieo quẻ:** ống thẻ lắc → một thẻ bật ra, lật mặt hiện quẻ; tỉ lệ vẫn hiển thị.
- **Dâng hương:** ba nén hương sáng dần, khói bốc; chuỗi ngày hiện dạng ngọn lửa đếm.

### 4.6 Đột phá & lôi kiếp (chế độ toàn màn, ≤ 3 s, bỏ qua được)
- **Đột phá nhỏ:** hào quang tụ (0.8 s) → chớp sáng → hào quang cảnh giới mới, chữ tầng mới.
- **Đột phá lớn:** trận pháp dưới chân xoay nhanh dần (1.2 s) → cột sáng → cảnh giới mới ghi bằng thư pháp; mây tan.
- **Lôi kiếp:** trời tối, mây đen cuộn, 3 tia sét (chớp toàn màn + rung camera nhẹ, tôn trọng reduced-motion), kết quả.
- **Thất bại:** nứt kính mờ + rung 200 ms + "tu vi còn nguyên"/"mất X tu vi" theo đúng phản hồi server.
- Kết quả **chỉ** hiện sau khi server trả về; trong lúc chờ: hoạt ảnh tụ khí lặp, timeout thì hiện lỗi mềm.

### 4.7 Cửa sổ & điều hướng
- Mở: scale 0.96→1 + fade 220 ms, nền mờ dần; bottom sheet trượt lên có quán tính, kéo xuống để đóng.
- Đổi tab trong cửa sổ: trượt ngang 160 ms. Danh sách: phần tử xuất hiện so le (stagger 30 ms, tối đa 10).
- Nút: nhấn lún (scale 0.96) + highlight; nút chính có ánh quét nhẹ khi sẵn sàng; disabled không hoạt ảnh.
- Hover desktop: nâng 2 px + viền vàng mờ.

### 4.8 Xã hội & thế giới
- Chat: tin mới trượt lên + fade; tin của mình loé ngọc; bong bóng trên đầu nhân vật bật nhẹ.
- **Thông báo thế giới/mốc cốt truyện:** cuộn thư pháp mở ngang giữa màn (2.5 s, bỏ qua được), âm chuông; ticker trên HUD.
- Đạo lữ: sợi tơ hồng nối hai chân dung khi song tu; nhịp tim nhẹ khi cả hai online.
- Thế lực: cờ phấp phới ở huy hiệu; công trình lên bậc → pháo hoa nhỏ.

### 4.9 Mini game (Tâm Ma, Hóa Phàm, Bí cảnh)
- Vào: chế độ toàn màn, đếm ngược 3-2-1 có âm; HUD riêng tối giản.
- Kết thúc: màn kết quả — huy hiệu qua/không qua, thống kê, phần thưởng bay vào túi (4.5), nút "Chơi lại/Về".
- Giữ nguyên logic mô phỏng và định dạng input log (đã kiểm chứng phía server) — chỉ đổi khung trình bày.

### 4.10 Âm thanh
- `AudioManager`: kênh BGM (theo cảnh, crossfade 1 s), Ambience, SFX, UI; âm lượng từng kênh + tắt hẳn trong Cài đặt,
  lưu `localStorage` (bọc try/catch). Tải lười; định dạng OGG/MP3; không phát trước tương tác đầu tiên.
- Danh sách SFX tối thiểu: bấm nút, mở/đóng cửa sổ, nhận tu vi, nhận vật phẩm (theo độ hiếm), lên tầng, đột phá, sét, thất
  bại, ngộ đạo, tin chat, thông báo thế giới, đếm ngược. **Âm thanh phải nguyên bản/có giấy phép** — để file giữ chỗ +
  `apps/web/src/lib/assets/audio/README.md` liệt kê id, mô tả, thời lượng để sản xuất sau.

### 4.11 Toàn màn hình & PWA
- Nút toàn màn hình (Fullscreen API) trong Cài đặt/HUD; mobile: gợi ý "Thêm vào màn hình chính".
- `manifest.webmanifest` (`display: fullscreen`/`standalone`, icon, theme color theo token), service worker chỉ cache
  asset tĩnh (không cache API). Chặn kéo-để-tải-lại và zoom chạm đôi trong `/play`.

## 5. Sẵn sàng 3D (không code 3D ở giai đoạn này)

- Mọi thứ phụ thuộc vị trí đi qua `SceneDef.anchors` + `anchorToScreen` (3.2). Component UI không biết cảnh là 2D hay 3D.
- `FxLayer` hoạt động trong không gian màn hình, nhận toạ độ từ renderer → dùng lại nguyên vẹn khi có 3D.
- Quy ước asset: mỗi cảnh có thư mục `assets/scenes/<id>/` với `layers/*.webp` (2D) và chừa chỗ `model.glb`, `env.hdr`
  (3D); nhân vật: chừa `characters/<id>.glb` + bảng ánh xạ hoạt ảnh (`idle`, `meditate`, `cast`, `breakthrough`).
- Viết `lib/engine/render/render3d/README.md`: phương án đề xuất (Three.js qua Threlte cho Svelte, glTF + KTX2/Draco,
  ngân sách tam giác/texture cho mobile, fallback 2D khi không có WebGL2), các bước spike, rủi ro (bundle, pin, thiết bị
  yếu). **Không** thêm dependency 3D.
- Feature flag `renderer` (`2d` mặc định) đọc từ Cài đặt/URL để sau này bật thử 3D theo từng cảnh.

## 6. Giai đoạn & tiêu chí "xong khi"

| GĐ | Nội dung | Xong khi |
|---|---|---|
| 0 | Sửa 2 lỗi mobile (mục 2) | Test Playwright xanh, ảnh 375/1440 |
| 1 | `lib/engine` khung: loop, FxBus, SceneManager, SceneRenderer 2D, AudioManager (chưa cần file âm), Cài đặt hiệu ứng/âm thanh | Unit test vitest cho SceneManager/FxBus/loop pause; overlay `?fps=1` |
| 2 | GameRoot ba lớp; Thế Giới + Động Phủ + Tử Tiêu Cung thành **cảnh**; chuyển cảnh mây phủ | Cảnh không unmount khi mở cửa sổ; Back đúng thứ tự; 60 fps idle |
| 3 | Chuyển các màn còn lại thành **cửa sổ** (`GameWindow`, bottom sheet mobile); thanh hành động mới; chat nổi; luật chơi vào "?" | Không còn `ScreenFrame` trong `/play`; không cột chat cố định; mọi cửa sổ Esc/Back/vuốt đóng |
| 4 | Hoạt ảnh 4.1–4.8 + FxLayer + số nhảy + thông báo thế giới | Mỗi hiệu ứng có bản Nhẹ/Tắt; reduced-motion tắt hết; không rò bộ nhớ sau 10 phút (DevTools heap) |
| 5 | Chế độ toàn màn cho mini game + đột phá/lôi kiếp (4.6, 4.9) | Logic/golden test mini game không đổi; kết quả chỉ hiện sau phản hồi server |
| 6 | Âm thanh (4.10), toàn màn hình + PWA (4.11), README audio | Âm chỉ phát sau tương tác; mute lưu được; Lighthouse PWA installable |
| 7 (cần duyệt) | Spike 3D: Tử Tiêu Cung bằng Three.js/Threlte sau flag `renderer=3d` | Chỉ làm khi chủ dự án ghi quyết định (mục 9) |

Mỗi GĐ: `npm run check && npm test && npm run build` xanh, `make lint`; ảnh chụp hoặc video ngắn (Playwright) cho từng
cảnh/cửa sổ ở 375 px và 1440 px, cả chế độ "Đầy đủ" và "Tắt"; ghi đo fps/bundle trước–sau.

## 7. Kiểm thử

- Vitest: SceneManager (đổi cảnh, stack cửa sổ, Back), FxBus (sự kiện → preset), loop (pause khi ẩn tab), AudioManager
  (mute/âm lượng/không phát trước tương tác — mock AudioContext), mapping `earnedMilli → số hạt` (thuần, có trần).
- Playwright: luồng chính (vào game → thu nhận → mở Túi Đồ → đóng → vào Tử Tiêu Cung → ngộ đạo giả lập bằng mock API →
  đột phá mock → mini game mở/thoát) ở 375/1440, có/không reduced-motion; không lỗi console.
- Không thêm test phụ thuộc thời gian thật; dùng fake timers/clock.

## 8. Những gì KHÔNG làm

- Không tự tính phần thưởng/tỉ lệ ở client; không đổi API/DB/catalogue; không thêm PixiJS/Phaser/Three ở GĐ 0–6.
- Không dùng asset/âm thanh/thiết kế của game có thật. Không chạy prettier lên file cũ (repo không có config prettier;
  style: tab, nháy đơn, dòng ~130 ký tự).
- Không phá trang site (`routes/(site)`: landing, đăng nhập, chính sách) — chỉ thống nhất phong cách nếu cần.

## 9. Tài liệu phải cập nhật

- `docs/decisions.md`: mục "Client webgame: cảnh/cửa sổ/chế độ, engine 2D tách renderer, lộ trình 3D" — ghi kiến trúc,
  ngân sách hiệu năng, quyết định *chưa* thêm engine 3D, điều kiện mở GĐ 7.
- `.claude/rules/frontend.md`: cho phép `src/lib/engine/` (Canvas2D thuần, một rAF loop) ngoài `minigames/`; giữ cấm
  engine 3D cho tới khi có quyết định GĐ 7.
- `.claude/rules/graphic-design.md`: motion tokens mới, chế độ hiệu ứng Đầy đủ/Nhẹ/Tắt, quy ước hạt/hào quang.
- `apps/web/src/lib/assets/art/README.md` + `assets/audio/README.md`: slot cảnh/lớp/âm thanh mới.

## 10. Báo cáo cuối mỗi giai đoạn

Danh sách file đổi, ảnh/video (đường dẫn), số đo fps + bundle trước/sau, test đã chạy (dòng tổng kết), quyết định đã
tự chọn, việc còn mở cần chủ dự án quyết.
