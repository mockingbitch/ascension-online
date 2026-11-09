# Đề xuất: ngoại hình nhân vật, nữ hướng dẫn viên, hiệu ứng lên cấp 6 bậc, thông báo toàn server

> Bước 1 ("Khảo sát và đề xuất") của [visual-progression-prompt.md](visual-progression-prompt.md). Chưa viết code,
> catalogue, migration hay ảnh. **Dừng chờ chủ dự án duyệt** các mục ở §8 trước khi làm GĐ-A.
> Ngày khảo sát: 2026-11-09 (sau "UI regions & layers", commit `fa042ef`).

**Một điểm lệch trong brief cần sửa trước:** brief nhắc "Thiên Nguyên Giới". Canon hiện hành là **Hồng Hoang**
(D2 đã đảo ngược ngày 2026-11-07, `docs/decisions.md`; `.claude/rules/story-writing.md`). Mọi đề xuất dưới đây dùng
Hồng Hoang và tên theo `docs/design/cot-truyen.md`.

---

## 1. Hiện trạng có thể tái dùng

### 1.1 Ô ảnh và cơ chế dự phòng
- `apps/web/src/lib/game/art.ts`: `art(path)` thử `.avif → .webp → .png`, có `@2x` thì sinh `srcset`. `artFirst(...)`
  cho chuỗi dự phòng. Thiếu file thì trả `undefined` và component tự vẽ SVG. Không bao giờ gọi 404.
- **Cần lưu ý:** glob đang **eager** (`import.meta.glob(..., { eager: true, query: '?url' })`). Mỗi file ảnh thêm
  một chuỗi URL vào bundle ban đầu (khoảng 70 B). Thêm khoảng 260 file (130 ảnh × `@2x`) thì tăng khoảng 18 kB thô,
  tức 4–6 kB gz. Vẫn trong ngân sách, nhưng nên tách glob `characters/**` thành **lazy** (xem §7.5).
- README `apps/web/src/lib/assets/art/README.md` đã có các ô `characters/avatar` (256²), `characters/meditate`
  (1024²), `characters/body-<realm_id>` (1024×1536, lùi về cảnh giới thấp hơn rồi về chân dung),
  `characters/listener` (lưng đệ tử ở Tử Tiêu Cung), `characters/hong-quan`, `cosmetics/<slug>`.
- Ảnh đang có: `characters/avatar`, `meditate`, `body-luyen_khi`, `hong-quan`, `listener`. Đây là **một bộ dùng
  chung cho mọi người chơi**. `docs/design/art-prompts.md` §7.3 mục 1 ghi rõ chỗ thiếu này: "chưa có lựa chọn
  ngoại hình". Prompt character sheet nam/nữ ở `art-prompts.md` §4 dùng lại được làm khuôn.
- Chuỗi dự phòng của ảnh toàn thân nằm ở `apps/web/src/lib/windows/CharacterWindow.svelte` (dòng ~28–35). Nó lọc
  `cat.realms` cùng path, tier ≤ hiện tại, rồi chọn `characters/body-<id>`.

### 1.2 Trang phục (cosmetics)
- Catalogue `content/catalog/cosmetics.yaml` có 4 `kind`: `title | avatar_frame | chat_aura | cave_decoration`.
  `source: shop` (mua bằng linh thạch, giá tăng dần) hoặc `tien_duyen` (cấp theo tier). Có cửa sổ mùa và thời đại.
- Go `services/game/internal/cosmetic/`: `cosmetic.go` có loader. `rejectEffects` từ chối mọi trường `*_bp`,
  `*_milli`, `*_hours`, `*_seconds` và mọi khóa hiệu ứng của items. Hàm này **tái dùng nguyên** cho
  `appearances.yaml`. `service.go` có `View`, `Buy`, `Equip`. `Equip` khóa dòng (`GetAppearanceForUpdate`), dùng
  Idempotency-Key qua bảng `cosmetic_equips`: cùng key thì phát lại, loadout khác thì 409 `ErrKeyReuse`.
- Bảng (migration `000014_cosmetics_faction_works.up.sql`):
  - `character_appearance(character_id PK, title_id, avatar_frame_id, chat_aura_id, cave_decoration_ids[], version, updated_at)`.
  - `cosmetic_equips` là log idempotent và audit.
  - `character_cosmetics` lưu quyền sở hữu.
  - `chat_messages.name_fx` chụp hiệu ứng tên tại thời điểm gửi.
- API: `GET /cosmetics/catalog` (công khai), `GET /cosmetics`, `POST /cosmetics/buy`, `POST /cosmetics/equip`
  (`services/game/internal/httpapi/cosmetic_handlers.go`).
- Web:
  - `lib/game/cosmetics.svelte.ts` là store catalogue và các gợi ý hiển thị (`frameStyle`, `slugOf`).
  - `components/CosmeticsPanel.svelte` dùng bản nháp xem trước, rồi lưu bằng một lần equip.
  - `components/CosmeticArt.svelte` và `NameFx.svelte` vẽ trang phục và hiệu ứng tên.
  - Tab **"Trang phục" hiện nằm trong cửa sổ Túi Đồ** (`windows/registry.ts`, `inventoryTabs`), không nằm trong
    Nhân Vật như brief viết (xem §8, Q9).
- Ảnh chụp ngoại hình (snapshot) đã đi kèm hàng của người khác: `LeaderboardEntry`, `FactionMemberEntry` (có
  `avatarFrameId`, `titleId`), `ChatMessage.nameFx`. `Listener` (Tử Tiêu Cung) **chưa** có ngoại hình.
- **Lỗ hổng có sẵn:** `vip.yaml` tier 12 hứa perk `breakthrough_fx:chi_ton`, và whitelist ở `internal/vip/vip.go`
  cho phép khóa `breakthrough_fx`. Nhưng chưa có gì hiển thị perk này (`cosmetic.go:359` bỏ qua nó). Perk này đụng
  trực tiếp ràng buộc 3: hào quang cảnh giới phải luôn nổi bật hơn mọi thứ mua được. Cần chốt, xem Q6.

### 1.3 Chân dung và hình nhân vật
- `components/Portrait.svelte` có props `{ aura, size, frameId }`. Nó dùng `characters/avatar` nếu có, nếu không thì
  dùng `CultivatorFigure crop="bust"`. Khung lấy raster `cosmetics/<slug>` (128%), nếu không thì vòng CSS
  `.frame-*`. **Chưa có tham số dáng người.** Nơi dùng: GameHud, ProfileWindow, RankingWindow, CharacterWindow,
  CompanionWindow, CosmeticsPanel, CultivationScene, DongPhuView, trang `(site)/leaderboard`.
- `components/CultivatorFigure.svelte` là một SVG nhân vật ngồi thiền (một bóng dáng, búi tóc và trâm, áo màu
  `ink-*`) tô màu theo `--aura`. Có hai kiểu cắt, `full` và `bust`. Muốn tham số hóa thì tách thành các nhóm
  `<g>`: thân/áo (màu `--robe`), tóc sau, đầu và kiểu tóc, phụ kiện, tư thế tay.
- `ProfileWindow.svelte` đã là "thẻ hồ sơ": mở từ chat, Thiên Bảng hoặc HUD. Của mình thì lấy dữ liệu sống, của
  người khác thì lấy snapshot. **Chưa có endpoint hồ sơ** và chưa có nút xuất ảnh.
- Cảnh: `DongPhuView.svelte` vẽ `characters/meditate` hoặc `CultivatorFigure`, kèm vòng `FormationCircle`, aura
  CSS và hạt `scene.fx.addEmitter` màu aura. Đây chính là chỗ đặt "hào quang thường trực". `HallView.svelte` vẽ
  `characters/listener` dùng chung cho mọi chỗ ngồi.

### 1.4 Hiệu ứng đột phá (`apps/web/src/lib/engine/fx/`)
- `breakthrough.ts` là hàm thuần. Nó nhận `BtInput { result, kind, tribulation }` và quality, trả ra timeline.
  - Các outcome: `minor | major | tribulation | tribulation_fail | fail`.
  - Các beat: `gather, flash, aura, spin, pillar, title, clouds, darken, strike, shake, crack, result`.
  - Tổng thời gian: major 2,6 s, lôi kiếp 2,7 s, `BT_MAX_MS = 3000`. Lite nhân ×0,6, không rung. Off cho thẻ tĩnh.
    Reduced motion thì không flash, không rung.
  - Số hạt khi lộ kết quả: 48/72 (lite 20).
- **Không có tier hay cảnh giới nào đi vào planner.** Màu lấy từ `realm.visual.aura` trong
  `BreakthroughMode.svelte` (`--aura`, burst `bt_${aura}`), nhưng mọi cảnh giới chạy cùng một timeline. Muốn có
  thang 6 bậc thì phải thêm `fxTier` vào `BtInput`, lấy từ catalogue theo `to`/`from` mà server trả về.
- Outcome `minor` **không bao giờ xảy ra**. Server chỉ có `BreakthroughKind = 'auto' | 'major'`, lượt thử luôn là
  major, còn tầng nhỏ tự lên trong luồng credit (`breakthrough.go:3`). Có thể tái dùng outcome này cho "tầng nhỏ".
- `plan.ts` (`planFx`): `stageUp` **không có hình**, rơi vào nhánh `default: []`. Chỉ có âm `sfx.stage_up` và
  `CountUp` trên HUD. `settled.stagesAdvanced` đã mang danh sách tầng, đủ để gộp hiển thị.
- `events.ts`: bus đã có `settled, breakthroughStart, breakthrough, stageUp, insight, trialResult, reward,
  announcement, equip…`. Đây là nguồn kích hoạt cho tutorial và hiệu ứng, phát sau phản hồi server.
- `presets.ts` có `qiBurst, qiOrb, landSpark, firework` (màu là token). `particles.ts` có ngân sách ≤ 300 / ≤ 80.
- `engine/mode/breakthrough.svelte.ts` và `BreakthroughMode.svelte` có history entry riêng, bỏ qua được.
  `FormationCircle` là trận pháp SVG duy nhất.
- `FxOverlay.svelte` vẽ cuộn thông báo `scroll`. **Hiện mọi announcement, kể cả của người khác, đều mở một cuộn ở
  giữa màn hình** (`position: fixed; inset: 0`, hàng đợi `SCROLL_QUEUE_MAX = 3`) khi fx full/lite. Cách này dễ che
  thao tác. GĐ-D sẽ thay.

### 1.5 Âm thanh (`engine/audio/`)
- `manifest.ts` `AUDIO_SLOTS` có `sfx.stage_up`, `sfx.breakthrough`, `sfx.thunder`, `sfx.fail`,
  `sfx.announcement` cùng các bộ ui/item/bgm. Mỗi slot có placeholder synth (`synth.ts`). File thật nằm ở
  `lib/assets/audio/<id>.ogg|mp3`, glob **lazy**.
- `sfxMap.ts` là hàm thuần. Tiếng sấm khớp từng `strike`, tiếng lộ kết quả khớp `flash/pillar/aura`.
  `STAGE_UP_QUIET_MS` tránh kêu hai lần. **Không theo tier.**

### 1.6 Thông báo
- Mẫu câu ở `content/story/announcements/major-breakthrough.yaml`: `major_breakthrough` (3 biến thể),
  `first_to_realm`, `rank_top10`.
- Producer: `breakthrough/service.go` `recordSuccess` ghi `outbox_events` topic `announcement.major_breakthrough`,
  payload `{characterId, name, realmId, stageId, tribulation}`, trong cùng transaction.
- Worker (`cmd/worker/main.go` ~394, `announce` ~460) render rồi `realtime.Publish` lên Redis
  `realtime:broadcast`, các instance API đẩy tiếp qua `/ws`. Cơ chế at-most-once.
- **Lỗ hổng:**
  - `first_to_realm` và `rank_top10` **có mẫu nhưng chưa bao giờ được phát**.
  - Cờ `tribulation` trong payload bị worker bỏ qua.
  - Chưa có giới hạn tần suất hay gộp.
  - Envelope chưa có `path` hay `first`.
- Phía client:
  - `api/realtime.svelte.ts` giữ `feed.items` (5 mục, chỉ trong bộ nhớ, khử trùng theo id), sau đó gọi
    `fxBus.emit('announcement')`.
  - `AnnouncementTicker.svelte` là ticker trên HUD, chỉ hiện tin mới nhất.
  - `NotificationList.svelte` liệt kê `feed.items`. Tải lại trang là mất danh sách.
  - `FxOverlay` hiện cuộn như đã nói ở §1.4.
- Chat chưa có loại "tin hệ thống".
- `packages/contracts/ws-messages.md` đã mô tả envelope `{v, type, id, ts, data}`.
- Hỗ trợ cho "người đầu tiên": `progression_events` có index `progression_events_stage_first_idx (stage_id,
  occurred_at) WHERE kind='realm_reached'`. Index này chưa chống được tranh chấp hai người cùng lúc, nên cần bảng có
  PK (§7.2).

### 1.7 Tân thủ và trợ giúp
- `lib/game/onboarding.ts`: các bước `mo-dau → tu-tieu-cung → the-luc → (dao-do nếu thế lực có nhiều path) →
  dao-hieu`. Bản nháp giữ trong sessionStorage. Kết thúc bằng **một** intent `POST /character {name, factionId,
  path}` (Go `character.Onboarding{IdempotencyKey, FactionID, Path}`). Text là bản sao của
  `content/story/onboarding/mo-dau.md`, có `onboarding.test.ts` so khớp từng câu.
- **Path do thế lực quyết định.** `vu_toc` chỉ cho `vu`, các thế lực khác chỉ cho `tien` (`factions.yaml`). Đổi
  thế lực hoặc bị chia tách thế lực có thể đổi path, kéo theo bộ dáng người tiên/Vu (Q4).
- `lib/game/help.ts` có 6 topic (`world, dong_phu, hall, breakthrough, timeline, inventory`), mở qua
  `HelpButton`, cửa sổ `help`. Chưa có topic cho thế lực, cửa hàng, mini game, Tiên Duyên, đạo lữ.
- `content/story/characters/` **trống**, nên hướng dẫn viên sẽ là nhân vật đầu tiên ở đây.
- Chưa có tiến độ tutorial nào ở server.

### 1.8 Cảnh giới (`content/catalog/realms.yaml`)
- 15 tier chơi được và tier 16 dành riêng (Thánh Nhân). Hai path chia sẻ số liệu theo tier.
- `visual: { badge, aura }`. Aura tiên đi theo thang `mist → jade → golden → azure → violet → void → crimson →
  radiant → thunder → celestial → earth → aureate → primal → chaos → saint` (token `--color-aura-*` trong
  `app.css`).
- **Cả 15 cảnh giới Vu đều dùng `aura: blood`.** Nhìn vào thì không có leo thang nào, nên phải bổ sung (§4).
- Lôi kiếp/Huyết kiếp áp cho đột phá **rời tier 5** trở đi (`tribulation.from_tier: 5`). Vì vậy mọi lần đột phá
  vào tier 6–15 đều là lôi kiếp, còn đột phá vào tier 2–5 là major thường.
- **Không ai đột phá *vào* Luyện Khí.** Bậc 1 chỉ có tầng nhỏ.

---

## 2. A. Dáng người

### 2.1 Danh sách đề xuất

ID dạng `<path>_<giới>_<khí chất>`, không đổi sau khi phát hành. "Màu gợi ý" là token y phục mặc định (§2.2).
Mọi dáng người đều là người trưởng thành, y phục kín đáo.

**Đạo đồ tiên: nam (4 bắt buộc + 1 tùy chọn)**

| id | Tên hiển thị | Khí chất | Đặc điểm nhận diện | Màu gợi ý |
|---|---|---|---|---|
| `tien_nam_thu_sinh` | Thư Sinh | Nho nhã, ôn hòa | Mặt trái xoan, mắt dài hiền. Dáng mảnh. Tóc búi nửa đầu buộc dải lụa, tóc dưới xõa. Ống bút trúc bên hông | `ngoc` |
| `tien_nam_kiem_khach` | Kiếm Khách | Cương nghị, ít lời | Hàm vuông, sẹo nhỏ đuôi mày. Vai rộng. Tóc buộc đuôi ngựa cao. Tay áo bó, đai da, bao kiếm sau lưng (không cầm vũ khí) | `mac` |
| `tien_nam_an_si` | Ẩn Sĩ | Điềm đạm, từng trải | Trung niên, râu ngắn tỉa gọn. Cao gầy. Búi tóc cài trâm gỗ. Áo vải gai, hồ lô bên hông | `tho` |
| `tien_nam_cong_tu` | Công Tử Thế Gia | Phong lưu, tự tin | Mặt tròn, mày rậm. Dáng cân đối. Kim quan nhỏ trên búi. Áo nhiều lớp thêu vân mây, ngọc bội | `kim` |
| `tien_nam_lang_khach` *(tùy chọn)* | Lãng Khách | Phóng khoáng | Tóc dài buộc thấp lòa xòa trán. Khăn choàng vai, áo choàng du hành. Sáo trúc | `lam` |

**Đạo đồ tiên: nữ (4 bắt buộc + 1 tùy chọn)**

| id | Tên hiển thị | Khí chất | Đặc điểm nhận diện | Màu gợi ý |
|---|---|---|---|---|
| `tien_nu_tieu_thu` | Tiểu Thư Thế Gia | Đoan trang, kiêu hãnh nhẹ | Mặt trái xoan. Tóc vấn cao nhiều trâm ngọc. Áo giao lĩnh nhiều lớp, quạt tròn lụa | `kim` |
| `tien_nu_kiem_khach` | Nữ Kiếm Khách | Lanh lợi, quyết đoán | Mắt sắc, dáng thon khỏe. Tóc buộc cao dây đỏ. Áo bó tay, bao kiếm sau lưng | `lam` |
| `tien_nu_y_su` | Y Sư | Dịu dàng, chu đáo | Mặt tròn, mắt cười. Tóc tết lệch vai, khăn vấn. Giỏ dược thảo, túi kim | `ngoc` |
| `tien_nu_an_tu` | Ẩn Tu | Tĩnh lặng, xa cách | Gò má cao. Tóc dài xõa buộc thấp, mạng che tóc. Áo trắng nhiều lớp kín cổ. Đàn cổ cầm sau lưng | `bach` |
| `tien_nu_thu_hoa` *(tùy chọn)* | Thư Họa | Tinh nghịch, mơ mộng | Tóc mái ngang, hai búi nhỏ cài hoa mai. Ống tranh cuộn, vệt mực trên tay áo | `tu` |

**Đạo đồ Vu (luyện thể): 2 nam + 2 nữ**

| id | Tên hiển thị | Khí chất | Đặc điểm nhận diện | Màu gợi ý |
|---|---|---|---|---|
| `vu_nam_chien_si` | Vu Chiến Sĩ | Hào sảng, dũng mãnh | To lớn, da rám. Tóc tết nhiều bím cài vòng đồng. Hoa văn mây sấm dạng hình học trên cánh tay (không phải biểu tượng tôn giáo). Giáp vai da | `huyet` |
| `vu_nam_hanh_gia` | Vu Hành Giả | Trầm, kiên nhẫn | Gầy rắn, tóc cạo hai bên buộc chỏm. Chuỗi xương thú. Gậy gỗ mộc, áo vải thô | `tho` |
| `vu_nu_tho_san` | Nữ Thợ Săn | Nhanh nhẹn, thẳng thắn | Dáng cao. Tóc tết đuôi dài. Áo da kín, khăn choàng lông. Cung sau lưng | `tho` |
| `vu_nu_luc_si` | Nữ Lực Sĩ | Vững chãi, ấm áp | Vạm vỡ, vai rộng. Tóc ngắn buộc khăn. Vòng đồng ở cổ tay, búa nhỏ đeo hông. Áo giáp da kín | `dong` |

Ghi chú art direction: nam Vu được để lộ cánh tay, nữ Vu luôn có áo kín thân. Không vẽ biểu tượng thờ cúng (thái cực,
bát quái, sen, chữ vạn), theo `art-prompts.md` §7.3.

### 2.2 Các tùy chọn đi kèm
- **Phụ kiện tóc** (`hair_id`). Đề xuất đây là *phụ kiện*, còn kiểu tóc chính gắn liền với dáng người (lý do ở
  §2.3).
  - Tiên: `tram_ngoc` (trâm ngọc), `kim_quan` (kim quan nhỏ), `day_lua` (dải lụa), `hoa_mai` (hoa mai),
    `tram_go` (trâm gỗ), `khong` (không đeo).
  - Vu: `vong_dong` (vòng đồng), `long_vu` (lông vũ), `day_da` (dây da), `khong`.
  - Mỗi dáng người khai báo phụ kiện mặc định của nó.
- **Màu y phục** (`robe_palette_id`). Mỗi lựa chọn trỏ tới token có sẵn qua token mới `--color-robe-<id>`:
  - `mac` (ink-700), `ngoc` (jade-500), `kim` (gold-500), `lam` (spirit-cyan), `tu` (spirit-violet), `bach`
    (mist-100), `tho` (aura-earth), `huyet` (aura-blood), `dong` (bronze-500).
  - `the_luc` (theo `factions.yaml` `color`, đổi khi đổi thế lực).
  - Không dùng `cinnabar` vì màu này dành cho thất bại.
  - Vu dùng `tho, huyet, dong, mac, the_luc`.
- **Tư thế thiền** (`pose_id`):
  - Tiên: `kiet_gia` (kiết già, tay nâng linh châu, như hiện tại), `ban_gia` (bán già, tay đặt gối),
    `kiem_ngang` (kiếm đặt ngang đùi).
  - Vu: `trung_binh_tan` (đứng trung bình tấn), `toa_thach` (ngồi trên tảng đá, hai tay chống gối).
- Mỗi lựa chọn có `id` ASCII, `name` tiếng Việt, `default_free: true` cho bộ cơ bản và `path`. Loader từ chối mọi
  trường hiệu ứng (`rejectEffects`).

### 2.3 Cách tạo biến thể và số lượng ảnh

- **Tóc trên ảnh vẽ tay không thay bằng lớp phủ được.** Phủ lớp tóc lên ảnh painterly gần như luôn lộ đường nối.
  Vì vậy kiểu tóc chính là một phần của dáng người, và mỗi dáng người có character sheet riêng. Phụ kiện tóc là
  lớp phủ nhỏ (`characters/acc/<hair_id>`, 128², alpha), đặt theo điểm neo đầu khai báo cho từng ảnh trong
  `appearances.yaml` (`anchors: { avatar: [x,y,scale], meditate: [...] }`). Trên SVG thì đổi nhóm `<g>`.
- **Màu y phục dùng mặt nạ thay vì ảnh riêng.** Mỗi ảnh có thêm một mặt nạ xám `…-robe` (cùng kích thước, alpha).
  Web phủ `<div style="mask-image:url(robe-mask); background: var(--color-robe-x); mix-blend-mode: color">`. Trên
  DOM/CSS cách này chạy được, không cần canvas. Thiếu mặt nạ thì ảnh giữ màu gốc (dự phòng mềm).
- **Tư thế:** mỗi tư thế là một ảnh `meditate` riêng, vì tư thế đổi cả bóng dáng.
- **Ảnh toàn thân theo cảnh giới:** đề xuất **theo bậc fx (6 nhóm)** thay vì theo 15 cảnh giới. Ô
  `characters/<body_id>/body-<realm_id>` vẫn được ưu tiên nếu ai đó vẽ riêng.

| Phương án | Ảnh mỗi dáng người | × 12 dáng người (8 tiên + 4 Vu) | Kèm mặt nạ màu áo | File (×1 + @2x) |
|---|---|---|---|---|
| Ngây thơ: avatar + 3 tư thế + 15 body | 19 | 228 | 456 | ~910 |
| **Đề xuất P0+P1:** sheet + avatar + 1 meditate + 6 body-bậc | 9 | 108 | +36 (chỉ avatar & meditate) = 144 | ~270 |
| Thêm P2: 2 tư thế phụ + lưng ngồi nghe giảng | +3 | +36 | +24 | ~120 thêm |

Với 2 dáng người tùy chọn thì cộng thêm 18 ảnh. Character sheet không vào game (chỉ làm tham chiếu, lưu ở
`tools/artgen/refs/`).

### 2.4 SVG dự phòng
`CultivatorFigure` nhận `{ aura, body: BodyLook, crop }`, trong đó `BodyLook = { silhouette: 'nam'|'nu'|'vu_nam'|'vu_nu',
hair: HairStyle, acc, robe, pose }`. Kiểu tóc SVG lấy từ trường `svg_hair` của dáng người: `bui_cao`, `duoi_ngua`,
`xoa_buoc_thap`, `tet_lech`, `bim_nhieu`, `chom`. Màu áo đọc `--robe`. Bốn bóng dáng, sáu kiểu tóc, năm tư thế đều
là các nhóm path tham số, không thêm ảnh. Cách này đủ để GĐ-A chạy trọn trước khi có raster.

---

## 3. C. Ba phương án nữ hướng dẫn viên

Cả ba đều là nhân vật **nguyên bản**: không phải nhân vật thờ phụng, không thay thế nhân vật canon nào, không lấy
từ tác phẩm có thật. Cả ba đều là phụ nữ trưởng thành (dáng vẻ khoảng 28–35 tuổi), y phục kín đáo, không gợi dục,
không trẻ hóa. Tên chọn xong sẽ thêm vào `moderation.yaml` (category `story`) để người chơi không mạo danh.
Lore đặt ở `content/story/characters/<id>.md`.

### Phương án 1 (khuyến nghị): **Vân Khê**, người dẫn chỗ ở Tử Tiêu Cung
- **Xuất thân:** một tán tu tìm tới Tử Tiêu Cung từ trước khi cửa mở, ngồi chờ ba ngày ngoài bậc đá. Hồng Quân
  không thu đệ tử, nên nàng tự nhận việc dẫn người mới vào chỗ ngồi và ghi những câu giảng họ lỡ vào Sổ Đạo Ngộ.
  Nàng không thuộc thế lực nào, nên người chơi chọn phe nào nàng cũng dẫn được. Truyện hợp với mở đầu hiện tại
  (cảnh 8 "Tử Tiêu Cung mở").
- **Tính cách:** ôn hòa, kiên nhẫn, hơi dí dỏm, hay tự trêu mình từng ngủ gật giữa bài giảng. Không bao giờ chê
  người chơi.
- **Xưng hô:** "ta" và "đạo hữu". Khi người chơi đạt cảnh giới cao hơn nàng (từ Hóa Thần), nàng đổi sang "tiền
  bối", có chút ngượng.
- **Ngoại hình:** tóc vấn thấp cài trâm gỗ đào, vài sợi bạc sớm. Áo giao lĩnh màu tím khói (`spirit-violet` nhạt),
  khoác áo choàng màu mực, cổ áo kín. Bên hông đeo bút lông và cuốn sổ bìa trúc. Nụ cười một bên khóe miệng.
- **Thoại mẫu:**
  1. "Chỗ này còn trống, đạo hữu ngồi đi. Lời giảng không chạy mất đâu, nó xoay vòng cả ngày."
  2. "Tu vi tích cả lúc đạo hữu đi vắng. Về thì bấm Thu nhận, để Thiên đạo kết sổ cho rõ ràng."
  3. "Lôi kiếp hỏng thì mất một phần tu vi tầng này, không tụt tầng. Hỏng ba lần liền, lần sau chắc chắn qua. Ta
     từng hỏng rồi, vẫn ngồi đây."

### Phương án 2: **Hạc Thư**, hạc linh giữ bia Thiên Bảng
- **Xuất thân:** một con hạc trắng thành tinh từ cuối Long Hán Sơ Kiếp, sống bên tấm bia Thiên Bảng, quen đọc tên
  khắc trên bia. Nàng hóa hình người để giảng cho kẻ mới đến cách tên được khắc lên. Nàng thuộc Yêu tộc nhưng không
  theo Thiên Đình Yêu Tộc. *(Ghi chú cho chủ dự án: chi tiết "bia có linh giữ" là sáng tác của dự án, cot-truyen
  không nhắc. Lore hiện chỉ có "Thiên Bảng là tấm bia đá".)*
- **Tính cách:** tinh nghịch, thích trêu nhẹ, tự hào về trí nhớ, ấm áp khi người chơi thất bại.
- **Xưng hô:** "ta" và "tiểu hữu", vì nàng sống đã nghìn năm. Có thể bị cảm thấy là xem nhẹ người chơi lâu năm.
- **Ngoại hình:** tóc trắng bạc, đuôi tóc nhuộm đen như cánh hạc, cài một chiếc lông vũ. Áo trắng viền đen kín cổ,
  khăn choàng đỏ son nhỏ ở vai (gợi đỉnh đầu hạc). Dáng cao, cổ thanh.
- **Thoại mẫu:**
  1. "Tiểu hữu mới tới à? Bia này ta đọc thuộc từng nét. Sớm muộn gì tên ngươi cũng nằm trên đó."
  2. "Lên tầng nhỏ là tự nhiên như nước chảy. Tới bình cảnh mới cần ngươi tự tay đột phá."
  3. "Tiên Duyên chỉ cho thêm giờ bế quan và vài món đẹp mắt. Bia không khắc tên ai nhờ nó cả."

### Phương án 3: **Thạch Lam**, con gái Vu Tộc rời Bất Chu Sơn
- **Xuất thân:** một nữ Vu luyện thể dưới chân Bất Chu Sơn, rời bộ tộc tới nghe giảng vì tò mò về "linh khí" mà
  người Vu không tu. Nàng đứng giữa hai đạo đồ nên giải thích được cả tiên lẫn Vu. *(Rủi ro: hướng dẫn viên mang
  màu Vu Tộc có thể bị hiểu là game thiên vị một thế lực. Bất Chu Sơn là chi tiết thần thoại cổ điển, đã có trong
  lore.)*
- **Tính cách:** thẳng thắn, hào sảng, hơi vụng về với chữ nghĩa. Hài hước theo kiểu bộc trực.
- **Xưng hô:** "ta" và "đạo hữu". Khi người chơi làm tốt thì gọi thân mật "bạn đồng hành".
- **Ngoại hình:** cao, khỏe, da rám nắng. Tóc tết một bím dài quấn vòng đồng. Áo vải thô màu đất kín thân, khoác
  áo da thuộc. Vòng tay đồng khắc hoa văn mây. Ánh mắt sáng.
- **Thoại mẫu:**
  1. "Bên ta luyện thân bằng máu, bên này luyện khí bằng hơi thở. Đường nào cũng mỏi chân, chọn đường nào thấy vững thì đi."
  2. "Huyết kiếp với Lôi kiếp nghe đáng sợ thôi. Trượt thì mất một phần tu vi tầng đang tu, nghỉ một ngày rồi thử lại."
  3. "Tổ ta dặn: thân có mạnh tới đâu cũng phải biết ngồi yên. Ngồi đi, ta kể tiếp."

**Ảnh hướng dẫn viên (cho phương án được chọn):** character sheet trước, rồi 6 biểu cảm `bình thường, mỉm cười,
chỉ dẫn, lo lắng, tự hào, ngạc nhiên`, nửa người, alpha, 768×1024. Thêm 1 khung "nhắm mắt" để chớp mắt bằng cách
đổi ảnh. Tổng cộng 8 ảnh (§6).

---

## 4. D. Bảng 6 bậc hiệu ứng

### 4.1 Gán bậc vào `realms.yaml`
Bậc gắn theo **tier**, nên hai path luôn trùng bậc (đúng nguyên tắc "numbers once per tier"). Đề xuất đặt
`fx_tier` ở cấp **tier** (khóa `tiers[].visual.fx_tier`). Từng realm vẫn giữ `aura`/`badge` riêng.

| Bậc | Tier | Tiên (realm id) | Vu (realm id) | Aura tiên hiện có | Aura Vu đề xuất (token mới) |
|---|---|---|---|---|---|
| 1 | 1 | `luyen_khi` | `vu_1` | mist | `vu-ember` (đỏ than nhạt) |
| 2 | 2–3 | `truc_co`, `kim_dan` | `vu_2`, `vu_3` | jade, golden | `vu-blood` (= blood hiện tại), `vu-bronze` |
| 3 | 4–5 | `nguyen_anh`, `hoa_than` | `vu_4`, `vu_5` | azure, violet | `vu-crimson-deep`, `vu-soul` (tím đỏ) |
| 4 | 6–9 | `luyen_hu` → `do_kiep` | `vu_6` → `vu_9` | void, crimson, radiant, thunder | `vu-storm` (đỏ sấm), `vu-obsidian`, … |
| 5 | 10–12 | `nhan_tien`, `dia_tien`, `kim_tien` | `vu_10` → `vu_12` | celestial, earth, aureate | `vu-totem` (vàng đồng rực) … |
| 6 | 13–15 | `thai_at_kim_tien`, `dai_la_kim_tien`, `chuan_thanh` | `vu_13` → `vu_15` | primal, chaos, saint | `vu-ancestral` (đỏ–vàng sáng) |

- **Vu hiện dùng `blood` cho cả 15 cảnh giới**, nên không thể leo thang về màu. Đề xuất mỗi tier Vu có một token
  aura riêng trong `app.css`, độ sáng và độ bão hòa tăng dần. Đây chỉ là thay đổi hiển thị, cần chốt (Q10).
- Validator mới trong `realm.Load`: `fx_tier` ∈ 1..6, không giảm theo tier, có ở mọi tier chơi được (tier 16 dành
  riêng thì không bắt buộc). TS mirror `RealmVisual.fxTier` đi qua `GET /catalogue` (§7).

### 4.2 Bảng hiệu ứng

Bậc của hiệu ứng **đột phá** = bậc của **cảnh giới đích** (`to`). Bậc 1 chỉ có tầng nhỏ, vì không ai đột phá
*vào* Luyện Khí. Lôi kiếp là đột phá vào tier ≥ 6, nên chỉ xuất hiện ở bậc 4–6.

| Bậc | Tầng nhỏ tự lên (không chặn) | Đột phá cảnh giới (mode) | Biến thể lôi kiếp | Hào quang thường trực | Âm thanh (cue) | Hạt full / lite | Thời gian full (lite ×0,6) |
|---|---|---|---|---|---|---|---|
| 1 | Toast "Luyện Khí Tầng N" + vệt sáng chạy dọc thanh tu vi (`data-fx-target="qi"`), 6 hạt sương | — (không có) | — | Vài hạt sương trôi chậm (≤ 6 hạt/scene) | `sfx.stage_up` (chuông gió nhỏ) | 6 / 2 | toast 1,6 s, không chặn |
| 2 | Như bậc 1, vệt sáng màu aura, 10 hạt | Sương tụ → vòng ngọc bích lan ra → kim đan sáng ở đan điền (Kim Đan) → tên cảnh giới thư pháp | — | Vòng ngọc mỏng dưới chân / ánh vàng đan điền (CSS) + ≤ 10 hạt | `sfx.bt_tier2`: khánh ngọc một tiếng | 90 / 30 | 2,2 s |
| 3 | 12 hạt + nhịp sáng nhẹ ở chân dung HUD | Trận pháp **hai tầng** quay ngược chiều → cột sáng → **bóng nguyên anh** (SVG mờ, scale 0.6 → 1) sau lưng → thư pháp → mây tách | — | Bóng nguyên anh mờ sau lưng (opacity 0,18, thở chậm) + ≤ 14 hạt | `sfx.bt_tier3`: khánh + chuông đồng | 140 / 45 | 2,5 s |
| 4 | 14 hạt + tia điện nhỏ 1 lần | (luôn là lôi kiếp) | Trời tối → mây đen cuộn → **3 tia sét** (flash + rung nhẹ) → **mây đen tan** → cột sáng → thư pháp. Thất bại: nứt kính + text server | Tia điện nhỏ quanh thân, 1 tia mỗi 3–5 s (full), tĩnh khi lite | `sfx.bt_tier4`: sấm (`sfx.thunder` ×3) + chuông lớn | 180 / 55 | 2,8 s |
| 5 | 16 hạt + chữ tầng mạ vàng | Lôi kiếp rút gọn (2 tia) → **mây lành ngũ sắc** trôi vào → **cổng trời** (khung SVG mở) → **chữ vàng cỡ lớn** | Như cột trái. Thất bại: cổng khép + nứt | Mây lành dưới chân (2 lớp SVG trôi) + ≤ 18 hạt | `sfx.bt_tier5`: cổ cầm rải + chuông | 240 / 65 | 2,9 s |
| 6 | 20 hạt + vòng tinh quang quanh chân dung | Lôi kiếp 1 tia trắng → **tinh hà xoay** (canvas, hạt theo quỹ đạo) → **pháp tướng khổng lồ** (bóng dáng người chơi phóng to, mờ, sau cảnh) → rung nhẹ cả màn hình (chỉ full, không có khi reduced motion) → chữ vàng + ấn triện | Như cột trái. Thất bại: tinh hà tắt + nứt | Tinh quang quỹ đạo + pháp tướng mờ (opacity 0,12) + ≤ 24 hạt | `sfx.bt_tier6`: hợp xướng ngân + chuông lớn + cổ cầm | 300 / 80 | 3,0 s |

**Thỏa ràng buộc:**
- **≤ 3 s:** mọi timeline ≤ `BT_MAX_MS = 3000` (bậc 6 chạm đúng trần). Lite nhân ×0,6, nên ≤ 1,8 s. Off là thẻ
  tĩnh. Bỏ qua bằng chạm, Esc hoặc Back như hiện nay. Test cũ `breakthrough.test.ts` vẫn giữ.
- **Leo thang đơn điệu, kiểm chứng được bằng test:** với mọi bậc `k < k+1`, các đại lượng sau đều không giảm và ít
  nhất một đại lượng tăng ngặt:
  - số hạt (full và lite);
  - số lớp hình (`layers`: vòng, trận 2 tầng, bóng, cổng, tinh hà);
  - thời gian;
  - độ sáng aura (độ sáng OKLab của token, kiểm bằng test web đọc `app.css`);
  - mức âm (số nhạc cụ trong cue).
  Bậc 4–6 đều có lôi kiếp nhưng mỗi bậc có điểm kết riêng (mây tan → cổng trời → tinh hà/pháp tướng).
- **Ngân sách hạt:** reveal ≤ 300/80. Hào quang thường trực ≤ 24/8 hạt mỗi scene và tạm dừng khi mode chạy. Tầng
  nhỏ ≤ 20/6.
- **Reduced motion:** không flash, không rung, không tinh hà xoay (vẽ tĩnh). Thông tin nằm ở thẻ cuối.
- **Tầng nhỏ gộp:** một lần `settled` có `stagesAdvanced = [a,b,c]` → **một** toast "Tầng a → c" và một vệt sáng.
  Hàm thuần `planStageUp(stages, fxTier, q)` có test. Sau đột phá cảnh giới, `stageUp` phát kèm bị bỏ qua phần hình
  (như `STAGE_UP_QUIET_MS` ở âm thanh).

### 4.3 Cách mở rộng code (chưa làm, chỉ phác)
- `BtInput` thêm `fxTier: 1..6`. `planBreakthrough` chọn timeline theo `(outcome, fxTier)`. Các bảng beat theo bậc
  để trong module lazy `fx/tiers.ts` (không vào bundle ban đầu).
- Các beat mới: `ring2, nascent, gate, galaxy, avatar, seal`. Thứ tự phụ thuộc beat (strike → reveal) giữ nguyên để
  `sfxMap` vẫn khớp.
- Preset mới trong `presets.ts`: `mist_motes, jade_ring, thunder_spark, auspicious_cloud, star_orbit`. Màu chỉ dùng
  token.
- `fxTier` lấy từ catalogue theo `to` mà server trả về. Client không tự tính cấp.

### 4.4 Thẻ thành tựu cảnh giới
- Sau màn ăn mừng, `ModeResult` hiện thêm một thẻ tĩnh gồm:
  - ảnh `characters/<body>/body-b<k>` (dự phòng là SVG `CultivatorFigure` cỡ lớn);
  - tên cảnh giới, ngày đạt (`realm_reached` từ server);
  - thứ hạng (từ server, có thể "—");
  - khung theo bậc `ui/card-frame-<k>` (SVG trước).
- Nút "Lưu ảnh" vẽ lại thẻ bằng Canvas2D trong module lazy, không dùng thư viện html-to-image. Chỉ lấy đạo hiệu,
  cảnh giới, thế lực, danh hiệu, ngày, hạng, **không** lấy email, id hay userId.

---

## 5. E. Thông báo toàn server

### 5.1 Sự kiện
| Sự kiện | Nguồn (trong transaction đột phá trừ khi ghi khác) | Type WS |
|---|---|---|
| Đột phá cảnh giới (vào tier 2–5) | `recordSuccess`, đã có | `announcement.major_breakthrough` |
| Vượt lôi kiếp/huyết kiếp (vào tier ≥ 6) | cùng chỗ, `tribulation: true` (hiện bị bỏ qua) | `announcement.major_breakthrough` + `tribulation` |
| Người đầu tiên đạt cảnh giới | `INSERT INTO realm_firsts … ON CONFLICT DO NOTHING` trong cùng transaction; insert được thì thêm outbox `announcement.first_to_realm` | `announcement.first_to_realm` (thay thế major cho người đó) |
| Vào top 10 Thiên Bảng | **worker** sau `lb.Refresh`, so hạng trước/sau trong ZSET (dữ liệu dẫn xuất, at-most-once chấp nhận được). Chỉ phát khi từ > 10 vào ≤ 10 hoặc lên hạng 1 | `announcement.rank_top10` |
| Gộp | worker khi vượt ngưỡng (§5.3) | `announcement.digest` |
| Tầng nhỏ | **không phát** | — |

Mẫu câu theo bậc thêm vào `major-breakthrough.yaml`:
- khóa `major_breakthrough.t2..t5`, `tribulation.tien.t4..t6`, `tribulation.vu.t4..t6` (dùng chữ "Huyết Kiếp"),
  `first_to_realm`, `rank_top10`, `digest`;
- mỗi khóa 2–3 biến thể.

**Ràng buộc độ dài:** đạo hiệu tối đa 24 ký tự (`character/name.go`), tên cảnh giới dài nhất 16 ("Thái Ất Kim
Tiên"). Phần chữ cố định của mỗi mẫu phải ≤ 50 ký tự. Thêm test Go render mọi mẫu với tên 24 ký tự và cảnh giới dài
nhất, rồi khẳng định ≤ 90.

### 5.2 Mức nổi bật
Bậc lấy từ catalogue theo `realmId` ở client, không cần gửi thêm.

| Bậc / loại | Hiển thị | Vị trí, layer | Âm | Thời gian |
|---|---|---|---|---|
| 2 (Trúc Cơ, Kim Đan) | Dòng ticker (`AnnouncementTicker`) | HUD | không | xoay vòng |
| 3–4 | Banner trượt có huy hiệu cảnh giới (`RealmBadge`) | Mép trên của stage, dưới HUD, `--z-toast`, rộng ≤ 420 px, không nhận pointer trừ nút đóng | `sfx.announcement` (nhẹ) | 4 s |
| 5–6, người đầu tiên, top 10 hạng 1 | Banner lớn toàn chiều ngang, màu `--color-aura-<aura>`, ≤ 30 hạt nhẹ ở FxLayer | Top layer (popover thủ công như Toaster), cao ≤ 72 px, `pointer-events: none` trừ nút "Xem" | `sfx.announce_major` | 5 s |
| Top 10 (hạng 2–10) | Banner trượt (như bậc 3–4) | | | 4 s |

- Banner nào cũng có ticker đi kèm, và mọi mục đều vào **danh sách Thông báo**.
- **Bỏ cuộn giữa màn hình** (`FxOverlay` scroll) cho thông báo của người khác. Cuộn chỉ giữ cho mốc thế giới
  (`world.milestone_reached`), vì đó là thời khắc chung hiếm có.
- Thông báo cũng hiện ở kênh chat **Thế giới** dưới dạng một dòng hệ thống: client chèn từ WS, khử trùng theo id,
  khi kết nối lại thì backfill từ `GET /announcements/recent`. Chỉ hiện đạo hiệu, không hiện id.

### 5.3 Chống spam (số đề xuất, đặt trong YAML `announcements.yaml` → `settings`, không hardcode)
- **Server, mỗi người chơi:**
  - ≤ 1 thông báo đột phá mỗi 10 phút. Cooldown đột phá (≥ 30 phút, lôi kiếp 24 h) vốn đã đảm bảo điều này, ghi
    thêm cho chắc.
  - Top 10: ≤ 1 lần mỗi 6 h mỗi người (Redis `announce:cool:<charId>` có TTL, mất key thì chỉ phát thừa một lần).
- **Server, toàn cục (worker):**
  - Mỗi cửa sổ 10 s: tối đa 3 thông báo bậc ≥ 3 được phát riêng. Phần dư gộp thành 1 `announcement.digest`
    `{count, topRealmId, names[≤3]}` ("Thanh Vân, Mộc Phong và 12 đạo hữu khác vừa đột phá").
  - Bậc 2 luôn chỉ là ticker. Tối đa 1 bậc 2 mỗi 5 s, phần dư gộp.
  - **Lưu ý ngày mở server:** với đường cong dồn đầu, hàng nghìn người lên Trúc Cơ trong ngày 1–2, nên gộp là bắt
    buộc.
- **Client:**
  - Hàng đợi banner ≤ 3, banner lớn cách nhau ≥ 6 s, banner trượt cách nhau ≥ 3 s. Mục cũ hơn 30 s thì bỏ qua
    phần hình, chỉ còn ticker và danh sách.
  - Không chiếm focus, không chặn pointer. Ẩn khi đang ở mode (mini game hoặc đột phá) rồi phát tiếp sau, hoặc bỏ
    nếu đã cũ.

### 5.4 Cài đặt
`engine/settings.svelte.ts` thêm hai lựa chọn, lưu localStorage cùng `fx` và `audio`, hiện trong Cài đặt › Thông
báo:
- `others.banner: 'all' | 'major' | 'off'`, mặc định `all` (Q8);
- `others.sound: boolean`, mặc định `true`.

Ticker và danh sách luôn còn. Thông báo của chính mình không chịu ảnh hưởng.

### 5.5 Luồng dữ liệu
Transaction đột phá ghi:
- `progression_events`;
- `realm_firsts` (insert-if-absent);
- `outbox_events` (`announcement.major_breakthrough` hoặc `.first_to_realm`).

Sau commit, worker drain:
1. render mẫu theo bậc, path, tribulation;
2. áp rate-limit và gộp, dùng Redis làm cửa sổ đếm (mất key thì chỉ phát thừa);
3. ghi `world_announcements` (lịch sử ngắn, xem §7.2);
4. `PUBLISH realtime:broadcast`.

API đẩy `/ws`. Client khử trùng theo `type:id`, map bậc, rồi đưa vào ticker, banner, danh sách và dòng chat.
Top 10: worker tự sinh sau `lb.Refresh`.

---

## 6. Kế hoạch ô ảnh

Mọi ô có SVG dự phòng, có ×1 và `@2x`, định dạng AVIF/WebP/PNG. "GĐ" là giai đoạn cần ô đó **chạy được bằng SVG**.
Ảnh raster làm ở GĐ-E.

| Đường dẫn | Kích thước ×1 | Alpha | Số ảnh | Dự phòng | Ưu tiên |
|---|---|---|---|---|---|
| `characters/<body_id>/avatar` | 256² | không (crop tròn) | 12 (+2) | `CultivatorFigure` bust tham số hóa | **P0** (GĐ-A; raster GĐ-E) |
| `characters/<body_id>/avatar-robe` (mặt nạ) | 256² | có | 12 | không tô màu áo | P1 |
| `characters/<body_id>/meditate` | 1024² | có | 12 | `CultivatorFigure` full + `FormationCircle` | **P0** |
| `characters/<body_id>/meditate-robe` | 1024² | có | 12 | không tô | P1 |
| `characters/<body_id>/body-b<1..6>` | 1024×1536 | có | 72 | `body-<realm_id>` → bậc thấp hơn → avatar → SVG | P1 (b1–b3 trước, b4–b6 sau) |
| `characters/<body_id>/meditate-<pose_id>` | 1024² | có | 24 | `meditate` mặc định | P2 |
| `characters/<body_id>/listener` | 256² | có | 12 | `characters/listener` chung + tô màu áo | P2 |
| `characters/acc/<hair_id>` | 128² | có | 9 | nhóm `<g>` SVG | P1 |
| `characters/guide/<expr>` (normal, smile, point, worry, proud, surprise, blink) | 768×1024 | có | 7 | `GuideFigure.svelte` (SVG bóng dáng + thay biểu cảm bằng mắt/miệng SVG) | **P0** (GĐ-B; raster GĐ-E) |
| `fx/aura-<1..6>` | 512² | có | 6 | CSS radial + canvas preset | P2 (code vẽ đã đủ) |
| `fx/formation-<1..6>` | SVG (code) | — | 6 biến thể SVG | `FormationCircle` hiện có | **P0** (GĐ-C, là SVG) |
| `ui/card-frame-<1..6>` | 1080×1350 | có | 6 | khung SVG/CSS theo bậc | P1 |
| `fx/nascent`, `fx/gate`, `fx/phap-tuong` | 512–1024 | có | 3 | SVG bóng mờ | P2 |
| Character sheet (12 dáng người + 1 hướng dẫn viên) | 1536×1024 | — | 13 | (không vào game, `tools/artgen/refs/`) | **P0** cho GĐ-E |

**Tổng raster đề xuất:**
- P0: 12 avatar + 12 meditate + 7 guide + 13 sheet = **44**.
- P1: 12 + 12 + 72 + 9 + 6 = **111**.
- P2: 24 + 12 + 6 + 3 = **45**.
- Nếu chọn ảnh toàn thân theo từng cảnh giới (Q3), P1 tăng thêm 108.

Prompt bổ sung vào `docs/design/art-prompts.md` §4: một mục cho mỗi dáng người, kèm khối "khí thế" theo 6 bậc
(thay bảng 5 cảnh giới hiện có), và mục hướng dẫn viên.

---

## 7. Thay đổi dữ liệu và API

### 7.1 YAML
- **`content/catalog/appearances.yaml` (mới)**:
  - `version`;
  - `settings { body_change_cooldown_seconds, free_rechoose_on_path_change: true }`;
  - `bodies[] { id, path, sex, name, temperament, svg_hair, default_hair, default_robe, default_pose,
    default_free, pair (body cùng khí chất ở path kia), anchors }`;
  - `hair_accessories[] { id, name, paths, default_free }`;
  - `robe_palettes[] { id, name, token, paths, default_free }`;
  - `poses[] { id, name, path, default_free }`;
  - `auras[] { fx_tier, name }` (pháp tướng/hào quang mở theo bậc, xem B).
  - Loader trong `internal/cosmetic`, file `appearance.go`. Dùng lại `rejectEffects` và `KnownFields`.
  - Cross-check trong `content.LoadAll`: `path` có trong `realms.yaml`, `pair` đối xứng và khác path, mỗi path có
    ≥ 1 body `default_free` cho mỗi `sex`, token màu áo thuộc whitelist.
  - Phía web có test đối chiếu token với `app.css`, giống cách token aura đang dùng.
- **`realms.yaml`:** `tiers[].visual.fx_tier`. Aura Vu mới cho `vu_1..vu_15` (Q10).
- **`content/catalog/fx_tiers.yaml` (mới, tùy chọn)**: tên preset hạt, biến thể trận pháp, cue âm thanh, số hạt
  full/lite và thời gian theo bậc. Đây là số thiết kế, nên đặt ở YAML theo quy tắc "không hardcode". Go chỉ validate
  (thời gian ≤ 3000, hạt ≤ 300/80, đơn điệu) rồi phục vụ qua `GET /catalogue`. Client đọc, không tự đặt số.
- **`content/story/tutorial/*.yaml` (mới):**
  - `steps[] { id, trigger (fxBus event | window | first_login), lines[] { expr, text ≤ 200, spotlight? } }`;
  - các luồng: `mo_dau`, `tao_nhan_vat`, `thu_tu_vi`, `dot_pha_tang`, `dot_pha_canh_gioi`, `loi_kiep`, `the_luc`,
    `giang_dao`, `cua_hang`, `mini_game`, `tien_duyen`, `dao_lu`;
  - nhân vật ở `content/story/characters/<guide_id>.md`.
- **`content/story/announcements/major-breakthrough.yaml`:** thêm các khóa ở §5.1 và `settings` rate-limit (hoặc tách
  thành `content/catalog/announcements.yaml` vì chứa số).

### 7.2 Migration (phác thảo; số tiếp theo là `000019`)
```sql
-- 000019_character_looks
ALTER TABLE character_appearance
  ADD COLUMN body_id          TEXT CHECK (body_id ~ '^[a-z][a-z0-9_]*$'),
  ADD COLUMN hair_id          TEXT CHECK (hair_id ~ '^[a-z][a-z0-9_]*$'),
  ADD COLUMN robe_palette_id  TEXT CHECK (robe_palette_id ~ '^[a-z][a-z0-9_]*$'),
  ADD COLUMN pose_id          TEXT CHECK (pose_id ~ '^[a-z][a-z0-9_]*$'),
  ADD COLUMN aura_tier        SMALLINT CHECK (aura_tier BETWEEN 1 AND 6),   -- NULL = highest unlocked
  ADD COLUMN body_changed_at  TIMESTAMPTZ;                                  -- cooldown (server time)
ALTER TABLE cosmetic_equips ADD COLUMN body_id TEXT, ADD COLUMN hair_id TEXT,
  ADD COLUMN robe_palette_id TEXT, ADD COLUMN pose_id TEXT, ADD COLUMN aura_tier SMALLINT;
ALTER TABLE chat_messages ADD COLUMN body_fx TEXT;    -- body snapshot at send time, like name_fx
-- NULL body_id = the path's default (catalogue) → no backfill needed; existing rows stay valid.

-- 000020_tutorial_progress
CREATE TABLE tutorial_progress (
  character_id    UUID NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
  step_id         TEXT NOT NULL CHECK (step_id ~ '^[a-z][a-z0-9_]*$'),
  status          TEXT NOT NULL CHECK (status IN ('done', 'skipped')),
  idempotency_key TEXT NOT NULL,
  at              TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (character_id, step_id),
  UNIQUE (character_id, idempotency_key)
);
ALTER TABLE characters ADD COLUMN tutorial_muted BOOLEAN NOT NULL DEFAULT false;  -- "không nhắc lại"

-- 000021_announcements
CREATE TABLE realm_firsts (
  realm_id     TEXT PRIMARY KEY,
  character_id UUID NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
  reached_at   TIMESTAMPTZ NOT NULL
);
CREATE TABLE world_announcements (           -- short history for reconnect/reload; worker prunes > 7 days
  id         BIGINT PRIMARY KEY,             -- = outbox event id (dedupe key shared with WS)
  type       TEXT NOT NULL,
  realm_id   TEXT, character_id UUID, name TEXT NOT NULL DEFAULT '',
  text       TEXT NOT NULL CHECK (char_length(text) <= 120),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX world_announcements_time_idx ON world_announcements (created_at DESC);
```
- Kiểm tra quyền (dáng người đúng path, phụ kiện hay màu đã mở, `aura_tier` ≤ bậc đã đạt) làm trong service theo
  catalogue, không làm bằng FK (quy tắc DB).
- `aura_tier` ≤ bậc của cảnh giới **hiện tại** do server suy ra. Lôi kiếp thất bại không tụt cảnh giới, nên không
  bao giờ bị khóa lại.

### 7.3 Endpoint và WS
- `GET /appearances/catalog` (công khai, cache), hoặc gộp vào `GET /cosmetics/catalog` thành mục `appearances`.
  Đề xuất **gộp** để bớt một request lúc khởi động.
- `POST /character`: thêm `appearance?: { bodyId, hairId, robePaletteId, poseId }` vào cùng intent tạo nhân vật. Để
  trống thì lấy mặc định của path.
- `POST /cosmetics/equip`: loadout thêm `bodyId, hairId, robePaletteId, poseId, auraTier`. Lỗi mới:
  - `409 body_cooldown` (kèm `retryAt`);
  - `422 wrong_path`;
  - `403 aura_locked`.
  Phản hồi trả toàn bộ `Appearance`, client thay thế, không merge.
- `GET /profile/{characterId}`: thẻ hồ sơ công khai (đạo hiệu, cảnh giới, thế lực, danh hiệu, khung, dáng người,
  màu áo, aura, hạng, ngày đạt cảnh giới). Không có email, không có user id. Rate-limit theo IP như `/factions`.
- `GET /tutorial` → `{ done: string[], skipped: string[], muted }`.
  `POST /tutorial/steps/{stepId}` `{ status }` + Idempotency-Key.
  `POST /tutorial/mute` `{ muted }` + Idempotency-Key.
  Server chỉ nhận `stepId` có trong catalogue tutorial. Ghi "đã xem" không có giá trị kinh tế nên rủi ro thấp,
  nhưng vẫn cần anti-cheat-reviewer xem nếu sau này gắn quà.
- `GET /announcements/recent?limit=50`.
- Snapshot ngoại hình thêm `bodyId, robePaletteId` vào `LeaderboardEntry`, `FactionMemberEntry`, `ChatMessage`
  (`bodyFx`) và `Listener` (WS `lecture.enter`).
- WS (`packages/contracts/ws-messages.md`):
  - `announcement.major_breakthrough` thêm `tribulation, path`;
  - mới `announcement.first_to_realm` `{characterId, characterName, realmId, stageId, path, text}`;
  - mới `announcement.rank_top10` `{characterId, characterName, rank, text}`;
  - mới `announcement.digest` `{count, realmId, names[], text}`.
- `openapi.yaml` và `types.ts` cập nhật trong cùng PR.

### 7.4 Test và validator cần mở rộng
- **Go:**
  - `cosmetic_test.go`: thêm `TestShippedAppearances`, các ca từ chối hiệu ứng, cooldown, sai path.
  - `service_integration_test.go`: equip idempotent kèm dáng người, đổi path thì được chọn lại miễn phí.
  - `realm` load test cho `fx_tier`.
  - `content.LoadAll` crossCheck.
  - Test độ dài mẫu thông báo.
  - Integration `realm_firsts` khi hai đột phá chạy đồng thời.
  - Test rate-limit và gộp của worker (hàm thuần).
  - `tutorial` integration.
- **Web:**
  - `breakthrough.test.ts` (6 bậc × 3 mức fx × reduced, tổng ≤ 3000, hạt đơn điệu);
  - `plan.test.ts` (`planStageUp` gộp);
  - `sfxMap.test.ts` (cue theo bậc);
  - `presets.test.ts`;
  - `onboarding.test.ts` (bước `ngoai-hinh`, câu text khớp story);
  - `cosmetics.test.ts`;
  - test mới cho đường dẫn ô ảnh (`characters/<body>/…` → chuỗi dự phòng);
  - test hàng đợi và gộp banner;
  - test text tutorial ≤ 200;
  - `zindex.test.ts` (đã có, áp cho component mới).
- **Anti-cheat-reviewer** sau GĐ-A (equip/cooldown), GĐ-B (tutorial) và GĐ-D (realm_firsts, rate-limit).

### 7.5 Ảnh hưởng tới bundle `/play` ban đầu
Hiện ~119 kB gz (112,4 sau GĐ7 + 6,5 sau UI polish).

| GĐ | Vào bundle ban đầu | Lazy | Ước tính tăng ban đầu |
|---|---|---|---|
| A | `CultivatorFigure` tham số (+~3 kB), store appearance, bản đồ đường dẫn art nhân vật | Bước tân thủ (route `/create`, không thuộc `/play`), tab Trang phục (cửa sổ lazy) | **+4–6 kB gz**. Nếu glob `characters/**` vẫn eager thì +4–6 kB nữa, vì vậy chuyển sang lazy glob kèm `artAsync()` |
| B | Trình kích hoạt tutorial (nhỏ, nghe fxBus) | `TutorialGuide.svelte`, kịch bản YAML → JSON chunk | +2–3 kB |
| C | `fxTier` trong catalogue, `planStageUp` | `fx/tiers.ts`, preset mới, thẻ thành tựu, xuất ảnh | +2–4 kB |
| D | Bộ điều phối banner, cài đặt | Component banner lớn | +2–3 kB |
| E | 0 (chỉ ảnh, tải khi cần) | — | 0 |

Mọi giai đoạn đều nằm xa trần +60 kB gz.

---

## 8. Quyết định cần chủ dự án chốt

1. **Hướng dẫn viên: chọn phương án nào?**
   - Phương án 1: Vân Khê (Tử Tiêu Cung, trung lập).
   - Phương án 2: Hạc Thư (hạc linh giữ Thiên Bảng; thêm lore mới).
   - Phương án 3: Thạch Lam (Vu Tộc; rủi ro bị xem là thiên vị phe).
   - Khuyến nghị: **phương án 1**, xưng "ta" và gọi "đạo hữu".
2. **Ngoại hình nâng cao lấy từ đâu?**
   - (a) Chỉ nhận được qua chơi (cảnh giới, thành tựu, sự kiện mùa).
   - (b) Thêm bán bằng linh thạch như các series trang phục hiện có (một nguồn tiêu linh thạch nữa).
   - (c) Thêm mở bằng Tiên Duyên.
   - Khuyến nghị: **toàn bộ 12 dáng người cơ bản miễn phí**. Phụ kiện tóc và màu áo "đặc biệt" theo (a) + (b). Tiên
     Duyên chỉ mở **1–2 màu áo hoặc phụ kiện** trung tính (không vàng chói, không hiệu ứng động), như danh hiệu hiện
     tại. **Không bao giờ** để hào quang hay pháp tướng mua được.
3. **Ảnh toàn thân theo cảnh giới hay theo bậc?**
   - Theo 15 cảnh giới: 180 ảnh.
   - Theo 6 bậc: 72 ảnh.
   - Khuyến nghị: **theo 6 bậc**. Ô `body-<realm_id>` vẫn được ưu tiên nếu sau này vẽ thêm.
4. **Đổi dáng người: miễn phí, có thời gian chờ?**
   - Khuyến nghị: **miễn phí, chờ 24 h** giữa hai lần đổi dáng người (phụ kiện, màu áo, tư thế đổi tự do). Khi đổi
     path do đổi hoặc chia tách thế lực: tự chuyển sang dáng người `pair` và được **một lần chọn lại không chờ**.
5. **Kiểu tóc:**
   - (a) Kiểu tóc gắn với dáng người, chỉ chọn phụ kiện.
   - (b) 2 kiểu tóc riêng cho mỗi dáng người (gấp đôi số ảnh).
   - Khuyến nghị: **(a)**.
6. **Perk `breakthrough_fx:chi_ton` (Tiên Duyên 12) đã hứa trong `vip.yaml`:**
   - (a) Bỏ perk.
   - (b) Đổi thành ấn triện "Chí Tôn" nhỏ trên thẻ thành tựu, tĩnh, nhỏ hơn mọi hiệu ứng bậc 1, không đổi timeline.
   - Khuyến nghị: **(b)**. Validator khẳng định perk không thêm hạt và không thêm thời gian.
7. **"Người đầu tiên" tính theo realm id hay theo tier?** Theo realm id thì cả tiên lẫn Vu đều có người đầu tiên.
   Theo tier thì toàn server chỉ một người.
   - Khuyến nghị: **theo realm id**.
8. **Banner của người khác mặc định bật?**
   - Khuyến nghị: **bật ở mức "all"** nhưng đã gộp và giới hạn (§5.3). Âm thanh thông báo người khác bật nhưng nhỏ.
     Người chơi tắt được.
9. **Vị trí tab Trang phục:** hiện ở Túi Đồ. Brief muốn ở Nhân Vật ("Gương Thủy Nguyệt").
   - Khuyến nghị: **chuyển tab sang cửa sổ Nhân Vật** (thêm `tabs` cho `character`). Túi Đồ giữ một nút tắt mở
     sang. Deep link cũ `?tab=cosmetics` chuyển hướng.
10. **Aura Vu theo từng tier** (hiện cả 15 là `blood`): thêm 15 token Vu leo thang, hay chỉ 6 (một mỗi bậc)?
    - Khuyến nghị: **6 token theo bậc** cho Vu, giữ `blood` cho bậc 2.
11. **Bậc 6 có rung màn hình không?** Brief đề xuất có (chỉ full).
    - Khuyến nghị: **có, 200 ms biên độ nhỏ**, tắt ở lite, off và reduced motion.
12. **Thông báo vào chat:** chỉ hiện dòng hệ thống phía client (không lưu chat), hay lưu thành `chat_messages` hệ
    thống?
    - Khuyến nghị: **client + `world_announcements`**, không làm phình bảng chat.
13. **Tên thư mục và tài liệu:** brief dùng "Thiên Nguyên Giới". Xác nhận dùng **Hồng Hoang** (canon hiện hành).
14. **Dáng người tùy chọn thứ 5** (Lãng Khách, Thư Họa): làm luôn (+18 ảnh) hay để sau?
    - Khuyến nghị: **để sau** GĐ-E đợt 1.

---

## 9. Ước lượng (tuần-người) và rủi ro

| GĐ | Nội dung | Ước lượng |
|---|---|---|
| A | `appearances.yaml` + loader/validator + migration 000019 + equip/cooldown/path pair + `POST /character` + snapshot ở mọi nơi (Thiên Bảng, chat, thế lực, Tử Tiêu Cung) + `CultivatorFigure` tham số + bước tân thủ `ngoai-hinh` + tab Trang phục (Gương Thủy Nguyệt) + `/profile` + test | **3–3,5** |
| B | Lore hướng dẫn viên + `GuideFigure` SVG + `TutorialGuide.svelte` (spotlight `data-tutorial`, bàn phím/SR, regions) + ~12 luồng kịch bản (~120 câu) + `tutorial_progress` + nối help (`HelpButton`) + test | **2,5–3** |
| C | `fx_tier` + `fx_tiers.yaml` + planner 6 bậc + preset/beat mới + gộp tầng nhỏ + hào quang thường trực và chọn hào quang + 5 cue synth + thẻ thành tựu + xuất ảnh Canvas | **2,5–3** |
| D | Worker: mẫu theo bậc, `realm_firsts`, top 10, rate-limit/digest, `world_announcements` + banner 3 mức + cài đặt + dòng chat hệ thống + contract | **1,5–2** |
| E | 13 character sheet + ~140 ảnh P0/P1 (thường tạo lại khoảng ×2), tách nền, mặt nạ màu áo, duyệt art direction, chụp màn hình kiểm | **2,5–4** (phụ thuộc chất lượng giữ nhân vật của công cụ) |
| **Tổng** | | **12–15,5 tuần-người** |

**Rủi ro:**
- **Giữ đúng gương mặt qua 9 ảnh mỗi dáng người.** Meta Muse không có reference ảnh thật, gpt-image có chỉnh sửa
  theo ảnh. Có thể phải chuyển sang provider có reference, hoặc chấp nhận avatar và meditate trước, ảnh toàn thân
  sau.
- **Mặt nạ màu áo trên ảnh painterly** có thể ra màu bệt. Cần thử trên 1 dáng người trước khi làm hàng loạt. Nếu
  không đạt thì chỉ tô màu áo cho SVG và khung.
- **Đổi path giữa chừng** (chia tách thế lực, Vu ↔ tiên) có thể làm ngoại hình "nhảy". Cách giảm: dùng `pair` và
  cho chọn lại miễn phí.
- **Bùng thông báo ngày mở server** (đường cong dồn đầu). Cách giảm: gộp và chỉ dùng ticker cho bậc 2.
- **Banner che thao tác.** Cách giảm: `pointer-events: none`, đặt trong region, chạy probe che khuất ở 8 kích thước
  như UI polish.
- **Tôn giáo và pháp lý:** pháp tướng bậc 6 và cổng trời không được giống tượng thờ. Chỉ dùng bóng dáng của chính
  người chơi, không dùng tư thế thủ ấn tôn giáo. Duyệt pháp lý vẫn được khuyến nghị (cot-truyen §8).
- **Perk VIP `breakthrough_fx`** nếu bị hiểu là "VIP đẹp hơn" sẽ phá ràng buộc 3. Cần chốt Q6 trước GĐ-C.
- **Hiệu năng:** tinh hà bậc 6 với 300 hạt trên Android tầm trung. Cách giảm: preset lite 80 hạt và đo FPS trên
  thiết bị thật (việc của chủ dự án).
- **Ngân sách nội dung:** ~120 câu tutorial và ~25 mẫu thông báo cần review giọng văn và độ dài (story-creator).
