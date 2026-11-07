# Lộ trình triển khai theo phase (checklist)

Dựa trên quyết định ngày 06/11/2026 trong [decisions.md](../decisions.md) và plan tổng
[ke-hoach-trien-khai-hong-hoang.md](ke-hoach-trien-khai-hong-hoang.md) (lý do, kiến trúc, rủi ro nằm ở đó, file này
chỉ là danh sách việc).

- **Ước lượng** tính bằng tuần-người (1 dev full-stack quen codebase), là số định hướng.
- **Hạn** tính theo ngày trong game, ngày 0 = ra mắt mở (`world_state.started_at`).
- Đánh dấu `[x]` khi xong. Việc chờ quyết định có nhãn **⚠️ cần chốt**.

## Định nghĩa "xong" chung (áp dụng cho mọi task code)

- [ ] Số liệu game nằm trong `content/catalog/*.yaml`, được loader validate, không hardcode trong Go/TS.
- [ ] Thay đổi trạng thái đi qua `Service` có row lock + `Idempotency-Key` + ledger/audit; side effect sang domain
      khác qua `outbox_events` (skill `game-transaction`).
- [ ] Đổi schema: migration up/down + `make sqlc` (skill `new-migration`).
- [ ] Đổi API: cập nhật `packages/contracts/openapi.yaml` + `apps/web/src/lib/game/types.ts` cùng PR.
- [ ] `make test`, `make test-integration`, `make lint` đều xanh. Web: `npm run check && npm test && npm run build`.
- [ ] Tính năng đụng tiền tệ, vật phẩm, tiến độ: chạy agent `anti-cheat-reviewer` trước khi merge.
- [ ] UI tiếng Việt, đúng token thiết kế, chạy được ở 360px, không lỗi console.

---

## Phase 0: Chốt thiết kế còn mở (1 tuần)

**Mục tiêu:** không còn quyết định nào chặn Phase 1–3. **Phụ thuộc:** không.

### Quyết định
- [x] **D1a, quà ngày:** giữ linh thạch, cấp Tiên Duyên cao nhận nhiều hơn. Đổi tên VIP thành **Tiên Duyên**.
- [x] **D1b, điểm duyên:** từ tu luyện online, tương tác người chơi, tương tác hệ thống. Nạp 1.000đ = 10 điểm.
- [x] **D7, linh thạch:** tiêu vào mua vật phẩm ở cửa hàng.
- [x] **Luật trao đổi giữa người chơi** — chủ dự án chốt 2026-11-07; triển khai Phase 5.2 (`trade.yaml`).
- [ ] ⚠️ **G2, G3** (Đại chiến, đạo đồ Yêu tộc): chốt trước khoảng ngày 30 sau ra mắt.
- [ ] ⚠️ **G5 đến G8:** xác nhận các đề xuất trong plan (G5 đã dùng tạm cho D3).
- [ ] Ghi tất cả vào `docs/decisions.md`.

### Nội dung & pháp lý
- [x] Tên theo `cot-truyen.md` (chủ dự án đảo ngược D2, 2026-11-07): thế giới Hồng Hoang, 5 thế lực ban đầu, ba giáo,
      Tây Phương Giáo, chuỗi cảnh giới Vu (bậc cuối Tổ Vu), Huyết kiếp. Slug thế lực giữ nguyên.
- [ ] Chủ dự án duyệt các tên `cot-truyen.md` không nêu (xem `docs/decisions.md` mục 2026-11-07 đảo ngược D2).
- [ ] Rà bảng đối chiếu với người am hiểu tín ngưỡng.
- [ ] Gửi luật sư: yêu cầu cấp phép game online, định danh tài khoản, giới hạn người dưới 18 (Nghị định 147/2024),
      quy định thanh toán trong game.
- [ ] Khởi động sản xuất video mở đầu: kịch bản 8 cảnh theo canon mới, storyboard, chọn người làm đồ họa/âm thanh,
      nguồn nhạc có giấy phép.

### Thiết kế dữ liệu
- [x] `data-architect`: schema YAML cho `realms` (thêm `paths`, `tier`, `tribulation`), `factions`,
      `world_milestones`, `contribution`, `shop`, điểm duyên (Tiên Duyên).
- [x] `game-balancer`: mô phỏng sơ bộ 330 ngày (100/ngày, trần 150, kiếp mất 25%) trước khi viết số chính thức.
- [x] Chốt 13 đề xuất số liệu + mô hình dữ liệu (`decisions.md`, mục "Phase 0: số liệu…").
- [ ] Số còn chờ: độ dài Chuẩn Thánh, ngưỡng mốc theo người chơi hoạt động, điểm duyên mỗi hoạt động + ngưỡng 12 cấp,
      thưởng tỉ lệ lôi kiếp (đang làm trong `balance-the-gioi-chung.md`).

**Xong khi:** `decisions.md` không còn mục ⚠️ chặn Phase 1–3, bảng nhân vật đã duyệt, schema đã chốt.

---

## Phase 1: Nền móng (khoảng 5–6 tuần-người)

**Mục tiêu:** game hiện tại chạy trên thang cảnh giới, thang điểm và luật lôi kiếp mới.
**Phụ thuộc:** Phase 0. **Hạn:** trước closed beta.

### 1.1. Thang cảnh giới & đạo đồ (D4)
- [x] Viết lại `content/catalog/realms.yaml` theo ngưỡng đã chốt: 15 cảnh giới đạo đồ tiên (Luyện Khí 9 tầng, sau đó
      4 kỳ × trọng), Thánh Nhân đánh dấu `reserved`. Bình cảnh 120% chỉ ở tầng cuối mỗi cảnh giới.
- [x] Thêm đạo đồ `vu`: 15 bậc tương đương, `tier` khớp từng bậc.
- [x] `internal/realm`: hỗ trợ `paths`, tra cứu theo `(path, tier)`, validate hai đạo đồ cùng số bậc.
- [x] Migration `000005`: reset dữ liệu nhân vật + điểm Tiên Duyên (giữ tài khoản). Đạo đồ suy ra từ `stage_id`,
      không thêm cột `path`.
- [x] Hàm quy đổi đạo đồ (giữ nguyên điểm, đổi `stage_id` theo `tier`), có test.
- [x] Web: `catalogue.ts`, `RealmBadge`, aura cho các cảnh giới mới (`graphic-designer` thêm token aura).

### 1.2. Thang điểm & trần ngày (D5)
- [x] `contribution.yaml`: trần 150/ngày, cơ cấu 50/30/20, hệ số bù người vào muộn.
- [x] Đổi tốc độ treo máy sang 50 điểm/24h, tích tối đa 8h (`rate_milli_per_hour` + `rate_carry`).
- [x] Trần theo nhóm: treo máy 75 · giảng đạo/nhiệm vụ 45 · cộng đồng 30, tổng 150/ngày.
- [x] Bảng `daily_contribution(character_id, game_day, points_milli)`. Mọi nguồn tu vi cộng vào đây **trong cùng
      transaction**, chặn ở trần.
- [x] Hệ số bù người vào muộn ×2 áp trước trần. Tặng một lần 50% trung vị server cho nhân vật tạo sau ngày 14
      (không tính vào tiến độ thế giới và bảng xếp hạng ngày).
- [x] `cmd/balancesim`: mục tiêu mới. Trung vị tới Chuẩn Thánh khoảng 330 ngày ở mức 100/ngày.
- [x] Web: hiển thị "hôm nay đã tích X/150".

### 1.3. Lôi kiếp (D3)
- [x] `realms.yaml` → `tribulation`: `base_bp 8000`, `step_bp 300`, `floor_bp 5000`, `loss_bp 2500`,
      `cooldown_seconds 86400`, `guarantee_after 3`. Chỉ áp từ lần đột phá rời Hóa Thần. Mất 25% tu vi **tầng hiện tại**.
- [x] Xem trước lôi kiếp tính cả phần thu tu vi ngay trước lần thử (số mất hiển thị = số mất thật).
- [x] `internal/breakthrough`: nhánh lôi kiếp. Thất bại thì trừ 25% tu vi đang tích (không tụt tầng), đặt
      cooldown 24h, tăng `fail_streak`. Thua 3 lần thì lần 4 = 100%.
- [x] Cộng tỉ lệ có trần: dư tu vi so với ngưỡng (pháp bảo, đan, hộ pháp để ở Phase 2–3, giờ chỉ để chỗ).
- [x] Ghi `loss_milli` vào `breakthrough_attempts` để audit.
- [x] `GET /breakthrough/preview` trả: tỉ lệ từng thành phần, số tu vi sẽ mất, có phải lôi kiếp không.
- [x] Web: `BreakthroughPanel` hiện "Thất bại sẽ mất X tu vi" + đếm bảo hiểm (x/3). Celebration riêng cho lôi kiếp (hiện chỉ đổi tiêu đề "Vượt qua Lôi Kiếp"; hiệu ứng riêng để `graphic-designer`).
- [x] Test: bảng tỉ lệ theo cảnh giới, bảo hiểm, idempotency khi gửi lại cùng key.

### 1.4. VIP thành Tiên Duyên (D1, D1a, D1b)
- [x] Đổi mọi chữ hiển thị "VIP" → **Tiên Duyên**, điểm → **điểm duyên** (UI, thông báo, `vip.yaml` `name`).
      Giữ package `vip` và bảng hiện có để khỏi migrate rủi ro. Loader từ chối tên cấp chứa "VIP".
- [x] Bảng `vip_point_ledger` (nguồn: `topup`, `cultivation`, `lecture`, `interaction`, `task`, `trial`, `event`,
      `admin`), cấp = tổng điểm. Ghi trong cùng transaction với hoạt động sinh ra điểm. Migration `000009`
      (`source`, `activity_id`, `game_day`, `reverses_ledger_id`); một hàm `vip.EarnLocked` áp trần theo hoạt động +
      trần 20/ngày dưới khóa `vip_accounts`. Đã nối: vận công (settle cách ≥ 1 giờ), nghe giảng (mốc 10/20), nhận
      thưởng nhiệm vụ ngày. Hộ pháp, song tu, thế lực, thí luyện, sự kiện để `live: false` cho tới khi tính năng có.
- [x] `vip.yaml` v3: `points_per_1000_vnd: 10`, điểm duyên cho từng hoạt động (quyết định D), ngưỡng 12 cấp theo
      quyết định E.
- [x] Giữ quà ngày linh thạch theo cấp (D1a). Sửa chú thích ngân sách quà ngày trong `vip.yaml` (67%, ràng buộc
      B × C của đan tăng tốc).
- [x] Hoàn tiền: trừ đúng điểm duyên đã cộng, cấp có thể tụt. `POST /admin/vip/refund` (audited, idempotent, mỗi giao
      dịch nạp hoàn tối đa 1 lần). Webhook nạp (khung + sandbox, Phase 4) gọi `TopupLocked`; hoàn đơn nạp qua `POST /admin/payments/orders/{id}/refund`.
- [x] Loader vẫn từ chối mọi đặc quyền cộng tu vi, tỉ lệ đột phá/lôi kiếp, điểm xếp hạng.
- [x] Balancesim: đo khoảng cách tiến độ Tiên Duyên 12 so với cấp 0 (qua linh thạch). Không ai vượt 150/ngày.
      Thêm bảng cấp Tiên Duyên người miễn phí đạt được từ nguồn đang mở.
- [x] Web: trang `/vip` thành trang **Tiên Duyên** (đổi route `/tien-duyen`, giữ redirect từ `/vip`), nút ở HUD đổi
      tên; trang/cửa sổ hiện nguồn điểm duyên và tiến độ hôm nay. Dock mobile chưa có nút Tiên Duyên (mở từ HUD).

### 1.5. Kiểm duyệt cơ bản
- [x] Bộ lọc đạo hiệu: `content/catalog/moderation.yaml` + `internal/moderation` (có/không dấu, vị trí dấu thanh,
      chữ fullwidth/biến thể Latin, lặp ký tự, tách chữ). Chặn trùng nhân vật cốt truyện, nhân vật tín ngưỡng, giả danh
      ban quản trị. Áp dụng khi tạo nhân vật và khi admin đổi tên.
      ⚠️ Danh sách nhân vật cốt truyện cập nhật lại khi Phase 0 chốt đổi tên (D2). Nhóm `religion`/`political` chờ
      pháp lý và người am hiểu tín ngưỡng rà.
- [x] Bảng `reports` (migration `000006`) + `POST /reports` (tên, quấy rối; idempotent, 1 báo cáo mở/đối tượng/lý do,
      10/ngày) + `GET /admin/reports`, `POST /admin/reports/{id}/resolve` (bỏ qua / xử lý / đổi tên, ghi
      `admin_audit_logs`). Web: nút báo cáo trên Thiên Bảng, trang `/admin/reports`.
- [x] Trang `/chinh-sach-du-lieu`. Quyết định 06/11: giữ tên + ảnh Google và user agent, trang ghi đủ (không chỉ
      email + đạo hiệu). ⚠️ Cần địa chỉ kênh hỗ trợ cho yêu cầu xóa dữ liệu; chưa có tính năng tự xóa tài khoản.

**Xong khi:** người chơi mới tạo nhân vật, tu luyện, đột phá và vượt lôi kiếp được trên thang mới. `make balance`
cho trung vị Chuẩn Thánh trong khoảng 300–360 ngày. `anti-cheat-reviewer` không còn lỗi mức cao.

---

## Phase 2: Thế giới chung & thế lực (khoảng 8 tuần-người)

**Mục tiêu:** cốt truyện chạy theo tổng tu vi server. Người chơi vào Tử Tiêu Cung, chọn thế lực, nghe giảng đạo.
**Phụ thuộc:** Phase 1. **Hạn:** trước closed beta.

### 2.1. Trạng thái thế giới
- [x] `content/catalog/world_milestones.yaml`: tất cả mốc cốt truyện (ngày mục tiêu, ngưỡng điểm, `min_pct 7000`,
      `max_pct 15000`, hiệu ứng, mẫu thông báo).
- [x] Bảng `world_state` (một dòng: kỷ nguyên, mốc hiện tại, `started_at`) + `world_milestones_reached`
      (migration `000010`: + `world_effects`, `world_events`, `world_milestone_overrides`).
- [x] `internal/world`: luật thuần "đạt mốc" theo **tiến độ trung bình mỗi người chơi hoạt động** (Σ điểm đã cộng ÷
      DAU mỗi ngày, cộng dồn), 70%/150% làm lưới an toàn. Có test bảng.
- [x] Worker: **tính lại** tổng ngày + DAU từ `daily_contribution`, đạt mốc thì ghi idempotent + outbox
      `world.milestone_reached`.
- [x] Thông báo mốc qua WS (`content/story/announcements/`).
- [x] `GET /world`: kỷ nguyên, % tới mốc kế tiếp, ngày dự kiến.
- [x] Admin: xem tổng, dời mốc thủ công (có audit).
- [x] Web: thanh tiến trình thế giới trên HUD (`WorldProgress`), `/play/thien-dao` chạy dữ liệu thật. (Thẻ "Thiên hạ
      truyền âm" không còn trong shell hiện tại; tin mốc vào feed thông báo.)

Ghi chú: hiệu ứng `split_faction`/`migrate_faction` được lưu chờ trong `world_effects` — Phase 2.2 thực thi (đã xong).

### 2.2. Thế lực
- [x] `content/catalog/factions.yaml`: 5 thế lực ban đầu (tên nguyên bản theo Phase 0), đạo đồ cho phép, kỷ nguyên
      mở/đóng. + ba giáo Nhân/Xiển/Tiệt, Tây Phương Giáo, Nhân Tộc (mở theo mốc). Loader + cross-check
      (đạo đồ, kỷ nguyên, hiệu ứng chia/đổi tên trong `world_milestones.yaml`).
- [x] Bảng `character_faction` + `faction_history` (migration `000011`). Đổi phe có thời gian chờ (YAML, 7 ngày).
- [x] Khóa đổi phe (dùng cho Phong Thần sau này): `POST /admin/factions/lock|unlock`, có audit.
- [x] Bảng xếp hạng thế lực (Redis ZSET mới, dựng lại được từ Postgres): công thế lực trong kỷ nguyên hiện tại +
      ZSET thành viên mỗi thế lực + số thành viên.
- [x] Worker thực thi `split_faction`/`migrate_faction`: báo trước → chọn nhánh → hết hạn xếp vào nhánh ít người nhất.
- [x] Web: trang Thế lực (danh sách thành viên, xếp hạng, đổi phe, chọn nhánh), mở khóa ô thế lực trên dock,
      `chon-the-luc` chạy thật.
- [ ] Điểm duyên "Công thế lực" (`faction_merit`, 1 × 3): chưa có định nghĩa hành động kích hoạt → giữ `live: false`.

### 2.3. Mở đầu Tử Tiêu Cung
- [x] Luồng người mới: video mở đầu (để chỗ, bỏ qua được) → Tử Tiêu Cung → chọn thế lực → chọn đạo đồ → đặt đạo hiệu.
      `/create` là wizard phía client, bước cuối gửi một intent `POST /character {name, factionId, path}` (§9.6, một
      transaction). Có nhân vật mà chưa có thế lực → `/chon-the-luc`; người chơi cũ bỏ qua cả hai.
- [x] Nội dung lời giảng mở đầu (`content/story/onboarding/mo-dau.md`: 8 cảnh + 8 câu của Hồng Quân, bản nháp D2).
- [x] Web: màn Tử Tiêu Cung (ảnh tĩnh + CSS từ art đã có; hội trường `/play/tu-tieu-cung` là nơi hạ cánh).
- [ ] Video mở đầu thật (đồ họa, giọng kể, nhạc, phụ đề) thay chuỗi ảnh tĩnh; xem lại từ menu.

### 2.4. Giảng đạo trực tiếp & presence
- [ ] `content/catalog/lectures.yaml`: 2 buổi/ngày (giờ VN), thời lượng, điểm thưởng (thuộc phần 30/ngày).
- [ ] Presence: heartbeat WS → khóa Redis có TTL, không ghi DB mỗi nhịp.
- [ ] Phòng giảng WS: broadcast nội dung + số người đang nghe.
- [ ] Điểm danh: phải có mặt đủ thời lượng + 1 tương tác "ngộ đạo" giữa buổi (chống bot).
- [ ] Ngân hàng lời giảng: mẫu + biến thể theo kỷ nguyên (`story-creator`).
- [ ] Load test k6: 3.000 kết nối cùng buổi.

### 2.5. Hoạt động hằng ngày & chỗ tiêu linh thạch
- [x] Dâng hương buổi sáng có chuỗi ngày (mở rộng `dailytask`), không "nhận lộc".
- [x] Thay nhiệm vụ `trung_kich` bằng "Nghe giảng đạo".
- [x] Ngộ đạo/gieo quẻ: buff nhỏ, công bố tỉ lệ, trong trần 150. (Tên trong game: "Gieo quẻ", để khỏi trùng khoảnh khắc ngộ đạo ở Tử Tiêu Cung.)
- [x] **Cửa hàng** (D7): `content/catalog/shop.yaml` (vật phẩm, giá linh thạch, giới hạn mua/ngày), `POST /shop/buy`
      có idempotency + ledger tiêu/nhận trong một transaction.
- [x] Loader từ chối đưa vào cửa hàng vật phẩm có thuộc tính tăng tỉ lệ đột phá/lôi kiếp (được bán vật phẩm *điều kiện*
      đột phá như Trúc Cơ Đan).
- [x] Vật phẩm tăng tốc: tối đa khoảng 10 điểm/ngày mỗi nhân vật (ví dụ Tụ Linh Đan +5, tối đa 2 viên/ngày).
- [x] Vật phẩm tăng tốc tu luyện: hiệu lực vẫn bị trần 150/ngày.
- [x] Cập nhật `decisions.md`: xóa cảnh báo "spirit stones have no sink" khi cửa hàng lên.
- [x] **Chỗ tiêu linh thạch định kỳ, không tăng tiến độ** (chủ dự án chốt cả 3, decisions 2026-11-07 "Linh thạch
      sinks", migration `000014`): `cosmetics.yaml` (danh hiệu, khung chân dung, danh quang chat, trang trí động phủ theo
      mùa/thời thế, giá leo thang; cùng id với perk Tiên Duyên), `POST /cosmetics/buy|equip`; `faction_works.yaml` +
      `POST /factions/donate` (trần/ngày, công đức bảng, chỉ mở trang trí sơn môn). Người khác thấy: `GET /character`,
      tin chat (`nameFx`), Thiên Bảng/thành viên thế lực. ⚠️ Giá và mốc công trình là placeholder — chờ chủ dự án duyệt.

**Xong khi:** một server thử với dữ liệu giả lập đi qua ít nhất 2 mốc (một mốc theo ngưỡng, một mốc theo 150%).
Buổi giảng chịu được load test. Linh thạch có ít nhất một chỗ tiêu.

---

## Phase 3: Mini game đột phá & bí cảnh (khoảng 8–9 tuần-người)

**Mục tiêu:** đủ mini game cần cho tháng đầu (xem bảng hạn chót trong plan, mục 4.1).
**Phụ thuộc:** Phase 1 (lôi kiếp), Phase 2.4 (presence cho hộ pháp).
**Hạn:** tâm ma **trước ra mắt mở**. Hóa phàm trước ngày 20. Bí cảnh + pháp bảo trước ngày 33.

### 3.1. Khung mini game
- [x] Bảng `trial_runs` (seed, issued_at, expires_at, submitted_at, kết quả, phán quyết) — migration `000013`.
- [x] `POST /trials/{kind}/start` → `{runId, seed, expiresAt, params}`. `POST /trials/{runId}/submit` → kiểm chứng.
      `GET /trials`: trạng thái từng loại (có thể thử, chờ tới, ấn chứng còn hạn).
- [x] Kiểm chứng: dùng một lần, thời lượng theo giờ server, điểm trong khoảng khả thi, phát lại log input (game
      tất định). Idempotency-Key, khóa hàng, thời gian chờ khi thua, phiên bỏ dở cũng tính thua.
- [x] `content/catalog/trials.yaml`: cảnh giới áp dụng, thời gian chờ khi thua (30 phút), độ khó.
- [x] Web: `src/lib/minigames/` (canvas 2D thuần), khung chung: tải, đếm giờ, gửi kết quả, chế độ giảm tốc.

### 3.2. Tâm ma (lên Nguyên Anh, kiểu Tetris)
- [x] Luật nguyên bản, tất định theo seed, có phát lại phía server (Go) — `docs/design/tam-ma-luat-choi.md`,
      vector vàng chung Go/TS `packages/contracts/golden/tam-ma-v2.json`.
- [x] Đột phá lên Nguyên Anh yêu cầu phiên tâm ma "qua" còn hiệu lực (giữ qua lần thất bại, dùng khi thành công).
- [x] Đồ họa nguyên bản (khối "tâm ma"), không dùng hình/âm thanh của game có thật.
- [x] Test gian lận: gửi lại run cũ, sửa điểm, rút ngắn thời gian, phiên hết hạn, run của nhân vật khác, gửi khi
      đang chờ, gửi đồng thời.
- [ ] Beta: hiệu chỉnh độ khó (`trials.yaml` params) theo tỉ lệ qua thật; `anti-cheat-reviewer` duyệt khung trial.

### 3.3. Hóa phàm (lên Hóa Thần, kiểu chém hoa quả)
- [x] Luật nguyên bản, sinh vật thể theo seed, kiểm tra hợp lý phía server — `docs/design/hoa-pham-luat-choi.md`,
      phát lại log nét (delta, một điểm/tick) bằng Go, vector vàng chung `packages/contracts/golden/hoa-pham-v1.json`,
      luật nhịp người (bước bút tối đa, cửa sổ trượt quãng/nét, nhấc tay, thời gian qua tối thiểu).
- [x] Gắn vào điều kiện đột phá lên Hóa Thần (`trials.yaml` `hoa_pham` gate tier 4; mọi đường có đột phá lớn đó).
- [x] Test gian lận: như tâm ma + dịch chuyển tức thời, tốc độ phi nhân, log quá cỡ.
- [ ] Beta: hiệu chỉnh độ khó (`pass_score`, đợt, đèn) theo tỉ lệ qua thật; phương án cho người không vuốt được.

### 3.4. Bí cảnh thử luyện + pháp bảo
- [x] `content/catalog/secret_realms.yaml`: thời lượng tối đa (150 s), lượt/ngày (3), bảng rơi theo phẩm điểm, trần
      phần thưởng/ngày (120 linh thạch · 30 nguyên liệu · 1 pháp bảo). Tách riêng khỏi `trials.yaml` (trial = cửa đột phá,
      bí cảnh = lượt tài nguyên hằng ngày), cùng khung kiểm chứng. Luật: `docs/design/bi-canh-luat-choi.md`.
- [x] `items.yaml`: 5 pháp bảo (linh → tiên, lôi kiếp +1% … +5%), loader: không trao đổi, chỉ `tribulation_bonus_bp`,
      chỉ artifact được tăng tỉ lệ; `content.LoadAll`: ≤ `item_bonus_max_bp`, chỉ rơi từ bí cảnh, không trong shop/nhiệm vụ.
      Một pháp bảo "hộ thân" được trang bị (`character_artifacts`, `POST /artifacts/equip`).
- [x] Bảng `secret_realm_runs` + `secret_realm_days` (migration `000015`), phần thưởng qua `currency_ledger` /
      `inventory_transactions` trong giao dịch nộp, khóa `sr:<run id>`.
- [x] Mini game survivor nguyên bản (canvas 2D): tất định số nguyên, log hướng 8 phía RLE, server phát lại, luật nhịp
      người (≤ 8 lần đổi hướng/giây), sàn 30 s, telemetry, hết hạn 240 s; vector vàng `packages/contracts/golden/bi-canh-v1.json`.
- [x] Pháp bảo/hộ pháp cộng vào tỉ lệ lôi kiếp (hoàn thiện 1.3), có trần tổng (quyết định F), bảng phân tích hiện mọi phần
      và trần đã cắt (`bonus_cap`, `max_cap`), snapshot trong `breakthrough_attempts.odds`. ⚠️ Đan hộ thân (≤ +2% còn lại
      của trần đồ 7%) chưa làm.
- [x] Hộ pháp: đồng môn online (presence Redis TTL qua `POST /presence/ping`), mời → nhận lời trong 30 phút → lần độ
      kiếp kế tiếp đếm người còn online + cùng thế lực (≤ 3), 2 lần/ngày/người hộ pháp; điểm duyên `protector_assist`
      `live: true` (worker cộng qua outbox). ⚠️ Chưa cộng tu vi nhóm cộng đồng cho người hộ pháp.
- [x] `game-balancer`: `make balance` có bảng bí cảnh 60 ngày (nguồn linh thạch/nguyên liệu, pháp bảo, ngân sách linh
      thạch trước/sau, hiệu ứng lôi kiếp, công bằng Tiên Duyên). Decisions 2026-11-07 "Bí cảnh thử luyện + pháp bảo".
- [ ] Beta: hiệu chỉnh độ khó/bảng rơi theo số liệu thật; `anti-cheat-reviewer` duyệt bí cảnh + hộ pháp.

**Xong khi:** người chơi đi được từ Luyện Khí đến Luyện Hư qua đủ mini game. `anti-cheat-reviewer` đã duyệt khung
trial. Kinh tế linh thạch ổn định trong mô phỏng 60 ngày.

---

## Phase 4: Closed beta & ra mắt mở (2–4 tuần)

**Mục tiêu:** hiệu chỉnh số liệu thật trước khi đồng hồ cốt truyện bắt đầu chạy.
**Phụ thuộc:** Phase 1–3 (hóa phàm và bí cảnh có thể lên sau theo hạn).

### Trước closed beta
- [ ] Pháp lý: đã có ý kiến luật sư, đủ điều kiện vận hành theo yêu cầu được xác nhận.
- [x] **Kênh nạp — khung xong, cổng thật còn mở:** đơn nạp, webhook có chữ ký + idempotent → `vip.TopupLocked` (1.000đ = 10
      điểm), đối soát + hết hạn ở worker, hoàn tiền nối `vip.Refund`, cổng giả lập (sandbox) cho dev/staging
      (`payments.yaml`, migration 000016, decisions 2026-11-07 "Khung nạp Tiên Duyên").
  - [ ] ⚠️ Chọn cổng thật (PayOS/VNPay/MoMo) sau hợp đồng + ý kiến pháp lý, viết adapter `payment.Provider`, bật trong
        `payments.yaml`. Chính sách hoàn tiền chính thức (web đang hiện bản nháp).
- [x] Backup + khôi phục thử Postgres (`make backup` / `make restore FILE=…` / `make backup-test`). Dashboard metrics,
      cảnh báo: `/metrics` api (:9090) + worker (:9091), profile compose `monitoring` (Prometheus + Grafana + luật cảnh
      báo). Xem [deployments/README.md](../../deployments/README.md).
- [ ] Cảnh báo tới người trực (Alertmanager + kênh nhận) khi có lịch trực; lịch cron backup + chép ra ngoài máy chủ.
- [x] Load test: settle, đột phá, buổi giảng, WS — kịch bản k6 `loadtest/` (`make loadtest`, smoke 50 VU đạt ngưỡng).
- [ ] Chạy `make loadtest PROFILE=full` (3.000 người nghe giảng + 3.000 WS) trên máy staging cấu hình như prod.

### Closed beta (200–500 người)
- [ ] Đo điểm/ngày thực tế theo nhóm (1, 2, 3 lần đăng nhập/ngày).
- [ ] Hiệu chỉnh `threshold_points` của mốc và tốc độ treo máy.
- [ ] Soát log gian lận mini game, tỉ lệ thua tâm ma (độ khó).
- [ ] Thu phản hồi UX: luồng Tử Tiêu Cung, lôi kiếp (mất 25% có gây bỏ game không).

### Ra mắt mở
- [ ] Reset dữ liệu beta (thông báo trước), ghi `world_state.started_at` = **ngày 0**. Công cụ đã có: `opsctl reset-beta`
      + `opsctl start-world` (xác nhận gõ tay, chặn prod, ghi audit) — xem deployments/README.md.
- [ ] Video mở đầu đã có (nếu chưa xong: ảnh tĩnh + chữ, thay sau).
- [ ] Trực vận hành 2 tuần đầu.

**Xong khi:** server mở, mốc cốt truyện đầu tiên được dự báo đúng khoảng.

---

## Phase 5: Xã hội (khoảng 4 tuần-người) · hạn ngày 30

**Phụ thuộc:** presence (2.4), kiểm duyệt (1.5).

### 5.1. Đạo lữ song tu
- [x] Bảng `dao_lu_bonds` + partial unique index (một bond đang hiệu lực mỗi người) — index trên `dao_lu_members`
      (cả hai phía), migration 000017.
- [x] Cầu thân → xác nhận đôi, cảnh báo "vĩnh viễn", điều kiện tối thiểu (tuổi tài khoản, cảnh giới) — `dao_lu.yaml`.
- [x] Song tu: cả hai online thì tăng hiệu quả 2h/ngày, trong trần 150 — phiên Redis + nhận `dual_cultivation`
      (community bucket); điểm duyên `dual_cultivation` đã `live`.
- [x] Chấm dứt: lượng kiếp (hook `bond.EndForCharacterLocked` cho Phase 6/8), admin sau báo cáo, vắng 90 ngày (worker).
- [x] Chống tự song tu bằng nhiều tài khoản: băm IP có muối (`internal/netsig`, 30 ngày), cảnh báo admin
      (`/admin/dao-lu/flags`, `/admin/netsig/clusters`). Chưa theo dõi thiết bị (chỉ mạng).

### 5.2. Trao đổi vật phẩm
- [x] Bảng `trade_offers`/`trade_lines`/`trade_days`/`trade_events` (migration 000018), tặng + đổi hai chiều, chuyển
      quyền sở hữu nguyên tử qua ledger trong một giao dịch (khóa theo thứ tự character_id), giới hạn lần/ngày (`trade.yaml`).
- [x] Tài khoản (< 7 ngày) và nhân vật (< 3 ngày, dưới Trúc Cơ) mới không nhận được. Trần giá trị nhận mỗi ngày theo quan
      hệ (người lạ 300 · đồng môn 600 · đạo lữ 1.000), phí 5% linh thạch bị hủy. Chỉ linh thạch + nguyên liệu.
- [x] Cảnh báo luồng chuyển một chiều bất thường cho admin (`GET /admin/trades/flows`, có audit), cờ trùng mạng (netsig),
      báo cáo giao dịch (`trade_abuse`).
- [x] Tab "Trao đổi" trong Túi Đồ (hộp thư, xác nhận "không hoàn tác", soạn từ thẻ đạo hữu, lịch sử); vật phẩm không
      trao đổi được có dấu khóa.

**Xong khi:** `anti-cheat-reviewer` duyệt trade (nhân bản đồ, race condition) và bond.

---

## Phase 6: Lượng kiếp thứ nhất · hạn ngày 60

**Phụ thuộc:** ⚠️ **G2** (cách chơi Đại chiến) và ⚠️ **G3** (đạo đồ Yêu tộc) phải chốt trước ngày ~30.

- [ ] Thiết kế Đại chiến (đề xuất trong plan mục 3.9: chiến dịch góp công theo thế lực, 15 ngày).
- [ ] Hiệu ứng tập thể khi đủ người online (tên nguyên bản theo D2).
- [ ] Mốc sau chiến: mở Địa Phủ + luân hồi. Người chơi Vu/Yêu chọn: Địa Phủ, luân hồi Nhân tộc, luân hồi loài vật.
- [ ] Luân hồi giữ **50%** tu vi (G4), quy đổi cảnh giới tương ứng, ghi ledger.
- [ ] Chấm dứt bond đạo lữ khi luân hồi.
- [ ] Nội dung: lore, thông báo, lời giảng kỷ nguyên mới.
- [ ] Diễn tập toàn bộ trên server staging với đồng hồ tua nhanh.

---

## Phase 7: Nữ Oa, Thánh nhân, Nhân tộc · hạn ngày 80

- [ ] Mốc tạo người: mở phe Nhân tộc + phe của Nữ Oa.
- [ ] Tách phe: báo trước 3 ngày, 7 ngày để chọn, không chọn thì vào nhánh ít người nhất.
- [ ] Mốc Thánh nhân: tách phe theo bảng D2 (thế lực thay cho Tam Thanh tách 3 nhánh, người chơi chọn, giữ tu vi;
      thế lực phương Tây đổi tên tự động). Job idempotent, báo trước 3 ngày.
- [ ] Mốc Tam Hoàng Ngũ Đế (hạn ngày 95).
- [ ] UI chọn nhánh khi tách phe.
- [ ] Diễn tập trên staging.

---

## Phase 8: Phong Thần Lượng Kiếp + cờ caro (khoảng 5–6 tuần-người) · hạn ngày 150

- [ ] Caro PvP qua WS: server giữ bàn, kiểm tra nước, đồng hồ 10s phía server, hết giờ là thua, cho phép bỏ cuộc.
- [ ] Ghép trận: ngẫu nhiên, ẩn danh, không ghép cùng thế lực hoặc đạo lữ. Bot khi chờ quá lâu.
- [ ] Bảng `caro_matches`, `fengshen_entries` (đăng ký Ứng Kiếp tự nguyện, G1).
- [ ] Chốt bảng tu vi ngay trước sự kiện (snapshot).
- [ ] Kết thúc sự kiện: thua 10 ván thì lên Phong Thần Bảng (khóa phe, trần Thái Ất). Đã đăng ký mà chưa đủ 15 ván
      thì luân hồi (giữ 50%). Chấm dứt bond tương ứng.
- [ ] Chống cố tình thua: cặp lặp lại, tỉ lệ thua bất thường, thời gian đi nước bất thường.
- [ ] Load test PvP. `anti-cheat-reviewer` duyệt.

---

## Phase 9: Thiên Đình tái lập & Tây Du · hạn ngày 200–210

- [ ] Xếp 365 thần vị theo tu vi trong số người đã lên Phong Thần Bảng (G8: danh hiệu + ngoại hình).
- [ ] Người vào sau Phong Thần vào Thiên Đình ở chức thấp nhất.
- [ ] Tu vi dư của người chạm Thái Ất quy đổi thành công đức.
- [ ] ⚠️ Thiết kế Tây Du (G7) theo luật D2: không dùng Phật/Quan Âm làm nhân vật chơi được, người giao nhiệm vụ
      hoặc đối thủ.

---

## Việc chạy song song suốt dự án

- [ ] **Nội dung:** lời giảng mỗi kỷ nguyên, thông báo mốc, lore. Luôn đi trước dev ít nhất một phase.
- [ ] **Cân bằng:** chạy lại `make balance` mỗi khi đổi số. Báo cáo hằng tuần sau ra mắt.
- [ ] **Đồ họa:** huy hiệu thế lực, aura cảnh giới mới, cảnh Tử Tiêu Cung, động phủ, mini game.
- [ ] **Vận hành:** theo dõi tốc độ tổng tu vi so với dự báo mốc, chuẩn bị dời mốc nếu lệch.
