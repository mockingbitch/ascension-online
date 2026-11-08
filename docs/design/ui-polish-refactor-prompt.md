# Prompt: Refactor UI trong game — bố cục vùng, lớp hiển thị, tương tác mượt

> Dán nguyên văn cho agent/dev thực hiện. Làm **sau** refactor webgame GĐ0–GĐ7 (`docs/design/webgame-refactor-prompt.md`,
> commit `cbc045c`, `05561f3`). Không thêm tính năng mới — chỉ sửa bố cục, lớp, tương tác cho đúng và mượt.

Bạn là senior frontend + UI/UX cho webgame tu tiên **Tiên Đạo Online** (SvelteKit + Svelte 5 runes + Tailwind v4,
`apps/web`). Game đã là webgame một trang: cảnh luôn chạy phía sau (`SceneLayer`), UI nổi phía trên (HUD, thanh hành
động, chat nổi, cửa sổ `GameWindow`, chế độ toàn màn, FxLayer). Nhưng các phần UI đang **chồng lên nhau, che nút, che
nội dung**, và tương tác chưa mượt. Nhiệm vụ: đưa UI về một **hệ thống vùng + lớp** rõ ràng, không còn gì bị che, và
mọi tương tác phản hồi tức thì.

Đọc trước: `CLAUDE.md`, `.claude/rules/{frontend.md,graphic-design.md,architecture.md}`, mục "Client webgame" và
"GĐ7" trong `docs/decisions.md`, `apps/web/src/routes/(game)/play/+layout.svelte`, `lib/components/{GameHud,ActionBar,
FloatingChat,GameWindow,Toaster,InstallHint,BootScreen}.svelte`, `lib/engine/scene/{SceneLayer,SceneSlot}.svelte`,
`lib/engine/fx/FxLayer.svelte`, các trang cảnh `routes/(game)/play/{+page,tu-luyen/+page,tu-tieu-cung/+page}.svelte`.

---

## 0. Hiện trạng đã kiểm chứng (audit 2026-11-08, container :2000, fx=off)

Ảnh: `scratchpad/shots/audit/{world,dongphu,hall}-{375,1440}.png` (đường dẫn đầy đủ trong báo cáo kèm prompt).
Probe: với mỗi nút/link đang hiển thị, `document.elementFromPoint(tâm nút)` phải là chính nút đó.

| # | Màn | Lỗi | Mức |
|---|---|---|---|
| A1 | Động Phủ 375×812 | Nút **Đột phá** và **Gieo quẻ** nằm dưới thanh hành động (`.primary-wrap`) / ngoài khung nhìn — người chơi mobile **không bấm được đột phá**. Thẻ "Tu vi · ước tính" bị cắt đáy, không cuộn được. | Chặn |
| A2 | Động Phủ 375 | Chat nổi ("Feng: hello" + nút chat) đè lên thẻ thống kê (Tốc độ/Bế quan). | Cao |
| A3 | Tử Tiêu Cung 375 | 3 tin chat nổi bị **băng phụ đề** (`.subtitles`) che; nút chat đè mép phụ đề. Nửa dưới cảnh trống trong khi chữ bị dồn. | Cao |
| A4 | Tử Tiêu Cung 1440 | Nhãn "TỬ TIÊU CUNG · HỒNG QUÂN GIẢNG ĐẠO" đè lên tranh; tin chat nổi đè lên cảnh trái; băng trạng thái chiếm 1 dải ngang riêng. | Trung bình |
| A5 | Thế Giới 375 | Khoảng trống lớn giữa thẻ trạng thái và chat; bố cục không dùng chiều cao. | Thấp |
| A6 | Mọi màn | `z-index` rải rác, không có thang: 1, 2, 5, 20, 30, 40, 45, 60, 70, 75, 100, 110, 120, 200 (HallView 110/120, tu-tieu-cung 200 > Toaster 75 > ModeLayer 70) trộn với top layer (`<dialog>`, popover FxLayer). Không ai biết cái gì nằm trên cái gì. | Gốc rễ |
| A7 | Mọi màn | Không có vùng "đặt chỗ" cho thanh hành động / chat / HUD: trang tự bù `margin-bottom: 3.25rem`, `--chat-lift`, … nên mỗi màn tự vá một kiểu và vẫn đè. | Gốc rễ |
| A8 | HUD | Menu bánh răng là `<details>` với `z-40` tự đặt, không đóng khi bấm ra ngoài / Esc nhất quán với cửa sổ; mục menu vẫn nằm trong cây hit-test khi đóng (probe báo "Cài đặt/Đăng xuất bị che"). | Trung bình |
| A9 | HUD 1440 | Thanh tu vi HUD và thanh trong Động Phủ có lúc hiện hai giá trị khác nhau (nội suy hiển thị lệch snapshot) — gây cảm giác "giật". | Trung bình |

**Tương tác chưa mượt** — chưa có số đo; agent phải đo trước (mục 6) rồi mới sửa. Nghi vấn cần kiểm:
nhấn nút không phản hồi ngay (chờ server mới đổi trạng thái), mở cửa sổ/tab có long task (lazy import + render),
`pointer-events: none` ở `.shell` khiến vùng trống "nuốt" hoặc "xuyên" click không đoán được, cuộn trong bottom sheet
kéo theo cảnh/đóng sheet, hover/focus nhảy layout, hoạt ảnh chạy trên thuộc tính layout.

---

## 1. Ràng buộc (không được phá)

- Server authoritative: client không tính thưởng/tỉ lệ; chỉ đổi trình bày. Không đổi Go/API/DB/migration/catalogue.
- Giữ kiến trúc GĐ0–GĐ7: một rAF loop (`gameLoop()`), cảnh không unmount dưới cửa sổ, cửa sổ là shallow routing
  (`page.state.wins`, tối đa 2), chế độ ở ModeLayer, hiệu ứng chỉ nghe `fxBus`, 3 mức fx Đầy đủ/Nhẹ/Tắt,
  reduced-motion = Tắt, renderer 2D/3D sau `SceneRenderer` + anchors.
- Màu/kích thước/thời lượng từ token `app.css`; không hex trong component. Không thêm dependency runtime.
- Không chạy prettier lên file cũ (tab, nháy đơn, dòng ~130). Không đụng `.env*`, `deployments/env/*`.
- JS ban đầu `/play` không tăng quá +10 kB gz. Không phá `routes/(site)`.

---

## 2. Hệ thống vùng (layout regions) — sửa gốc A1–A5, A7

Thay "mỗi màn tự bù margin" bằng **một lưới vùng do shell sở hữu**, công bố kích thước qua CSS custom properties để
mọi thứ khác tính theo:

```
┌──────────────────────── HUD (--hud-h) ─────────────────────────┐
│                                                     │ RAIL     │  desktop ≥ 1024: rail phải (--rail-w)
│   STAGE  = vùng an toàn cho UI của cảnh             │ (action) │
│   (inset: --hud-h, --rail-w, --bar-h, --chat-w*)    │          │
│                                                     │          │
│ CHAT DOCK (góc dưới trái, kích thước cố định)       │          │
└──────────── BAR (--bar-h, mobile < 640: hàng dưới) ─┴──────────┘
```

- Shell đặt và cập nhật (ResizeObserver, một nơi) các biến: `--hud-h`, `--bar-h`, `--rail-w`, `--safe-b`
  (= `env(safe-area-inset-bottom)`), `--chat-reserve` (chiều cao vùng chat thu gọn), `--stage-inset-*`.
- **Stage** = khu vực UI của trang cảnh được phép vẽ: `inset: var(--hud-h) var(--rail-w) calc(var(--bar-h) +
  var(--safe-b)) 0`. Mọi panel/nút của trang (thẻ tu vi, nút Đột phá/Gieo quẻ, băng trạng thái, phụ đề) **nằm trong
  stage**; nếu không đủ cao thì stage **cuộn** (một vùng cuộn duy nhất, `overscroll-behavior: contain`), không bao giờ
  tràn xuống dưới thanh hành động.
- Cảnh (`SceneSlot`) có thể tràn full-bleed phía sau HUD/bar, nhưng **anchor UI** (bảng địa danh, bong bóng, ghế) phải
  nằm trong stage — dùng `projectPoint` + clamp vào stage, không hard-code px.
- **Chat dock** là một vùng đặt chỗ, không phải phần tử nổi tuỳ ý: thu gọn = nút + tối đa 3 dòng trong một hộp có
  chiều cao cố định; stage chừa `--chat-reserve` ở góc dưới trái (mobile: chừa trên hàng bar; desktop: chat nằm trong
  khoảng trống trái-dưới, phụ đề/thẻ tránh ra bằng `padding-inline-start`). Bỏ `--chat-lift` và mọi `margin-bottom`
  bù tay.
- Mobile (< 640): thứ tự ưu tiên chiều cao cho Động Phủ = tiêu đề cảnh giới (gọn 1 dòng) → nhân vật (co giãn, min
  ~38vh) → thẻ tu vi + **hàng nút Đột phá/Gieo quẻ luôn thấy được** (sticky ở đáy stage, trên bar). Nếu bình cảnh,
  "Đột phá" phải nhìn thấy không cần cuộn.
- Tử Tiêu Cung mobile: cảnh 16:9 → phụ đề ngay dưới cảnh → chat dock; phụ đề không đè chat, chat không đè phụ đề.
  Desktop: phụ đề là băng đáy **trong** stage, chừa `--chat-reserve` bên trái; nhãn eyebrow chuyển vào băng trạng thái
  (không đè tranh).
- Thế Giới mobile: bản đồ + lưới địa danh + thẻ trạng thái chia chiều cao hợp lý (không còn khoảng trống chết).
- Landscape phone (812×375) và tablet (768, 1024) là kích thước hạng nhất — kiểm như 375/1440.

## 3. Thang lớp (z-index) — sửa gốc A6, A8

Định nghĩa **một** thang trong `app.css`, mọi component chỉ dùng token, không số trần:

| Token | Giá trị | Ai |
|---|---|---|
| `--z-scene` | 0 | SceneLayer, scene particle field |
| `--z-scene-ui` | 10 | UI gắn cảnh: bảng địa danh, bong bóng, tên người chơi, ghế (HallView/WorldView) |
| `--z-stage` | 20 | UI trang cảnh trong stage (thẻ, phụ đề, băng trạng thái) |
| `--z-chat` | 30 | Chat dock |
| `--z-bar` | 40 | Thanh hành động / rail |
| `--z-hud` | 50 | HUD + menu HUD |
| `--z-wipe` | 60 | SceneWipe |
| `--z-mode` | 70 | ModeLayer |
| `--z-toast` | 80 | Toaster, InstallHint, ticker |
| `--z-boot` | 90 | BootScreen |
| top layer | — | `GameWindow` (`<dialog>`), FxLayer (popover), FpsOverlay — chỉ những thứ này dùng top layer |

- Mỗi vùng là một **stacking context** riêng (`isolation: isolate`) để z nội bộ (vd. HallView 110/120,
  tu-tieu-cung 200) chỉ có nghĩa bên trong vùng đó; đổi chúng về 1/2/3 cục bộ.
- Quy tắc top layer: thứ tự = thứ tự mở; FxLayer được raise sau mỗi cửa sổ (đã có) — viết test cho việc phần thưởng bay
  qua cửa sổ đang mở và toast nằm trên cửa sổ.
- Menu HUD: thay `<details>` bằng popover (`popover="auto"` hoặc GameWindow `local` dạng menu) — đóng khi bấm ngoài /
  Esc / mở cửa sổ, focus trả về nút bánh răng, nội dung không nằm trong hit-test khi đóng.
- Thêm lint nhỏ (script vitest hoặc grep trong `make lint`): fail nếu có `z-index:` với số trần hoặc class `z-[0-9]`
  trong `apps/web/src/lib` + `routes/(game)` (ngoại trừ file token).

## 4. Con trỏ & hit-test

- Bỏ mô hình "`.shell` pointer-events: none + bật lại từng con" khó đoán. Thay bằng: vùng cảnh nhận pointer ở chỗ
  **không có UI**; các vùng UI (HUD, bar, chat, stage panels) là khối `pointer-events: auto` rõ ràng; khoảng trống
  trong stage xuyên xuống cảnh **chỉ** khi stage được đánh dấu `data-passthrough`.
- Probe tự động (mục 6): mọi phần tử tương tác hiển thị phải hit-test chính nó ở tâm và ở 4 điểm cách mép 4 px
  (trừ phần tử bị cửa sổ/chế độ phủ có chủ đích — loại qua `inert`).
- Tap target ≥ 44×44 px trên mobile, khoảng cách giữa hai target ≥ 8 px (hàng bar hiện chật ở 360).

## 5. Tương tác mượt (A9 + "chưa mượt")

Đo trước, sửa sau. Mục tiêu:

- **Phản hồi < 100 ms** cho mọi nhấn: trạng thái nhấn (`:active` scale 0.96, đã có token) + trạng thái *đang gửi*
  ngay lập tức (spinner nhỏ / nút mờ + `aria-busy`), không chờ server mới đổi gì. Kết quả vẫn chỉ hiện sau server.
- **INP p75 ≤ 200 ms**, không long task > 50 ms khi: mở/đóng cửa sổ, đổi tab, mở chat, đổi cảnh, bấm Thu nhận/Ngộ đạo.
  Cách: prefetch chunk cửa sổ khi hover/focus/`pointerdown` nút mở; render nội dung nặng sau khung đầu (skeleton có kích
  thước cố định để không nhảy layout); danh sách dài cắt/ảo hoá.
- **CLS ≈ 0** trong `/play`: ảnh cảnh/chân dung có kích thước đặt sẵn; số liệu `tabular-nums`; badge không đẩy layout;
  hover không đổi kích thước (dùng transform/outline).
- **Một nguồn số cho tu vi** (A9): HUD và thẻ Động Phủ đọc cùng một giá trị nội suy hiển thị (một store/derived), cùng
  CountUp; không hai bộ nội suy riêng.
- **Bottom sheet:** kéo theo ngón tay 1:1 (transform, không layout), có quán tính + ngưỡng vận tốc để đóng; nội dung
  sheet cuộn được mà không kéo sheet khi chưa ở đỉnh; khoá cuộn nền (`overscroll-behavior`, không `body{overflow:hidden}`
  gây nhảy). Desktop: cửa sổ không nhảy vị trí khi đổi tab (chiều cao min theo tab lớn nhất hoặc animate height bằng
  `interpolate-size`/FLIP).
- **Chuyển tab / mở cửa sổ:** giữ token `--dur-window`, `--dur-fast`; không animate `box-shadow`/`height` trên nhiều nút.
- **Focus & bàn phím:** thứ tự Tab hợp lý trong từng vùng; Esc đóng thứ trên cùng (menu → cửa sổ → chế độ hỏi xác nhận);
  focus ring vàng luôn thấy được (không bị vùng khác che — kiểm bằng probe).
- **Cuộn:** đúng một vùng cuộn ở mỗi màn (stage hoặc nội dung cửa sổ); không cuộn lồng nhau ngoài ý muốn.

## 6. Kiểm thử & đo (bắt buộc)

Script Playwright trong `scratchpad/shots/` (tái dùng `gd2/common.mjs`, `gd4/mocks.mjs`); tham khảo `audit/shoot.mjs`.

- **Ma trận kích thước:** 360×740, 375×812, 390×844, 812×375 (ngang), 768×1024, 1024×768, 1440×900, 1920×1080 ×
  fx full/off × cảnh world/dongphu/hall + mỗi loại cửa sổ + chat mở + menu HUD mở + một chế độ.
- **Occlusion probe:** 0 phần tử tương tác bị che (tâm + 4 góc trong). Ghi bảng trước/sau.
- **Vùng:** assert thẻ/nút trang cảnh nằm trong stage (`getBoundingClientRect` không giao bar/HUD/chat dock); nút
  **Đột phá** thấy được không cần cuộn khi bình cảnh ở 360×740.
- **Hiệu năng tương tác:** dùng `PerformanceObserver` (`event`, `longtask`, `layout-shift`) — báo INP xấp xỉ, số long
  task, CLS cho từng thao tác ở mục 5; CPU throttle 4× khi đo mobile.
- Chạy lại toàn bộ bộ cũ tuần tự: `gd0/smoke`, `gd2`, `gd3`, `gd4 --no-soak`, `gd5`, `gd6`, `gd7` (sửa selector chỉ
  khi DOM đổi hợp lệ, ghi rõ). Cắm sạc khi đo fps (máy chạy pin yếu khiến Chrome giới hạn 30 fps).
- Vitest: logic tính vùng (thuần), thang z (lint), store tu vi hiển thị dùng chung.
- `cd apps/web && npm run check && npm test && npm run build`, `make lint` xanh.

## 7. Giai đoạn

| GĐ | Nội dung | Xong khi |
|---|---|---|
| U0 | Đo nền: occlusion probe + INP/long task/CLS + ảnh ma trận **trước** khi sửa | Có bảng số liệu & ảnh "trước" |
| U1 | Thang z + stacking context + menu HUD popover + lint z | 0 số z trần; probe không còn lỗi do lớp |
| U2 | Hệ thống vùng (biến shell, stage, chat dock, bar/rail); sửa A1–A5 | Probe 0 bị che ở mọi kích thước; Đột phá thấy ngay ở 360 |
| U3 | Hit-test rõ ràng (bỏ pointer-events vá), tap target | Không click xuyên/nuốt ngoài ý muốn (test bấm khoảng trống → hotspot cảnh) |
| U4 | Tương tác mượt: phản hồi tức thì, prefetch, skeleton, sheet, một nguồn tu vi | INP ≤ 200 ms, 0 long task > 50 ms ở thao tác chính, CLS ≈ 0 |
| U5 | Rà soát cuối ma trận + a11y bàn phím + tài liệu | Ảnh "sau", bảng trước/sau, rules cập nhật |

## 8. Tài liệu phải cập nhật

- `.claude/rules/frontend.md`: vùng shell + biến CSS, stage là nơi duy nhất cho UI trang cảnh, một vùng cuộn, quy tắc
  top layer, cấm z trần.
- `.claude/rules/graphic-design.md`: thang `--z-*`, tap target 44 px, trạng thái đang gửi.
- `docs/decisions.md`: mục "UI regions & layers" — lý do, thang z, ngân sách INP/CLS.

## 9. Báo cáo cuối mỗi giai đoạn

File đổi; bảng occlusion trước/sau theo kích thước; INP/long task/CLS trước/sau theo thao tác; bundle trước/sau;
dòng tổng kết test; đường dẫn ảnh (tuyệt đối) trước/sau cùng khung; quyết định tự chọn; việc cần chủ dự án quyết.
