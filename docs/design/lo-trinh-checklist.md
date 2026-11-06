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
- [ ] ⚠️ **Luật trao đổi giữa người chơi** (cần trước Phase 5).
- [ ] ⚠️ **G2, G3** (Đại chiến, đạo đồ Yêu tộc): chốt trước khoảng ngày 30 sau ra mắt.
- [ ] ⚠️ **G5 đến G8:** xác nhận các đề xuất trong plan (G5 đã dùng tạm cho D3).
- [ ] Ghi tất cả vào `docs/decisions.md`.

### Nội dung & pháp lý
- [ ] `story-creator`: canon Thiên Nguyên Giới mới trong `content/story/lore/`. Bảng đối chiếu **từng nhân vật/thế
      lực** của `cot-truyen.md`: giữ tên, đổi tên nguyên bản, hoặc thay bằng nhân vật nguyên bản (luật D2).
- [ ] Đặt tên nguyên bản cho 5 thế lực ban đầu, chuỗi cảnh giới Vu tộc và các kiếp.
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
- [ ] Đổi mọi chữ hiển thị "VIP" → **Tiên Duyên**, điểm → **điểm duyên** (UI, thông báo, `vip.yaml` `name`).
      Giữ package `vip` và bảng hiện có để khỏi migrate rủi ro.
- [ ] Bảng `vip_point_ledger` (nguồn: `topup`, `cultivation`, `lecture`, `interaction`, `task`, `trial`, `event`,
      `admin`), cấp = tổng điểm. Ghi trong cùng transaction với hoạt động sinh ra điểm.
- [ ] `vip.yaml`: `points_per_1000_vnd: 10`, điểm duyên cho từng hoạt động, **tính lại ngưỡng 12 cấp** để người
      chơi miễn phí lên cấp được qua hoạt động (`game-balancer`).
- [ ] Giữ quà ngày linh thạch theo cấp (D1a). Sửa chú thích ngân sách quà ngày trong `vip.yaml` (đang sai: 67% chứ
      không phải 15%).
- [ ] Hoàn tiền: trừ đúng điểm duyên đã cộng, cấp có thể tụt.
- [ ] Loader vẫn từ chối mọi đặc quyền cộng tu vi, tỉ lệ đột phá/lôi kiếp, điểm xếp hạng.
- [ ] Balancesim: đo khoảng cách tiến độ Tiên Duyên 12 so với cấp 0 (qua linh thạch). Không ai vượt 150/ngày.
- [ ] Web: trang `/vip` thành trang **Tiên Duyên** (đổi route `/tien-duyen`, giữ redirect từ `/vip`), nút ở HUD và
      dock mobile đổi tên.

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
- [ ] `content/catalog/world_milestones.yaml`: tất cả mốc cốt truyện (ngày mục tiêu, ngưỡng điểm, `min_pct 7000`,
      `max_pct 15000`, hiệu ứng, mẫu thông báo).
- [ ] Bảng `world_state` (một dòng: kỷ nguyên, mốc hiện tại, `started_at`) + `world_milestones_reached`.
- [ ] `internal/world`: luật thuần "đạt mốc" theo **tiến độ trung bình mỗi người chơi hoạt động** (Σ điểm đã cộng ÷
      DAU mỗi ngày, cộng dồn), 70%/150% làm lưới an toàn. Có test bảng.
- [ ] Worker: **tính lại** tổng ngày + DAU từ `daily_contribution`, đạt mốc thì ghi idempotent + outbox
      `world.milestone_reached`.
- [ ] Thông báo mốc qua WS (`content/story/announcements/`).
- [ ] `GET /world`: kỷ nguyên, % tới mốc kế tiếp, ngày dự kiến.
- [ ] Admin: xem tổng, dời mốc thủ công (có audit).
- [ ] Web: thanh tiến trình thế giới trên HUD, thay thẻ "Thiên hạ truyền âm".

### 2.2. Thế lực
- [ ] `content/catalog/factions.yaml`: 5 thế lực ban đầu (tên nguyên bản theo Phase 0), đạo đồ cho phép, kỷ nguyên
      mở/đóng.
- [ ] Bảng `character_faction` + `faction_history`. Đổi phe có thời gian chờ (YAML, đề xuất 7 ngày).
- [ ] Khóa đổi phe (dùng cho Phong Thần sau này).
- [ ] Bảng xếp hạng thế lực (Redis ZSET mới, dựng lại được từ Postgres).
- [ ] Web: trang Thế lực (danh sách thành viên, xếp hạng, đổi phe), mở khóa ô "Tông Môn" trên dock.

### 2.3. Mở đầu Tử Tiêu Cung
- [ ] Luồng người mới: video mở đầu (để chỗ, bỏ qua được) → Tử Tiêu Cung → chọn thế lực → chọn đạo đồ → đặt đạo hiệu.
- [ ] Nội dung lời giảng mở đầu (`story-creator`).
- [ ] Web: màn Tử Tiêu Cung (cảnh SVG nguyên bản, `graphic-designer`).

### 2.4. Giảng đạo trực tiếp & presence
- [ ] `content/catalog/lectures.yaml`: 2 buổi/ngày (giờ VN), thời lượng, điểm thưởng (thuộc phần 30/ngày).
- [ ] Presence: heartbeat WS → khóa Redis có TTL, không ghi DB mỗi nhịp.
- [ ] Phòng giảng WS: broadcast nội dung + số người đang nghe.
- [ ] Điểm danh: phải có mặt đủ thời lượng + 1 tương tác "ngộ đạo" giữa buổi (chống bot).
- [ ] Ngân hàng lời giảng: mẫu + biến thể theo kỷ nguyên (`story-creator`).
- [ ] Load test k6: 3.000 kết nối cùng buổi.

### 2.5. Hoạt động hằng ngày & chỗ tiêu linh thạch
- [ ] Dâng hương buổi sáng có chuỗi ngày (mở rộng `dailytask`), không "nhận lộc".
- [ ] Thay nhiệm vụ `trung_kich` bằng "Nghe giảng đạo".
- [ ] Ngộ đạo/gieo quẻ: buff nhỏ, công bố tỉ lệ, trong trần 150.
- [ ] **Cửa hàng** (D7): `content/catalog/shop.yaml` (vật phẩm, giá linh thạch, giới hạn mua/ngày), `POST /shop/buy`
      có idempotency + ledger tiêu/nhận trong một transaction.
- [ ] Loader từ chối đưa vào cửa hàng vật phẩm có thuộc tính tăng tỉ lệ đột phá/lôi kiếp (được bán vật phẩm *điều kiện*
      đột phá như Trúc Cơ Đan).
- [ ] Vật phẩm tăng tốc: tối đa khoảng 10 điểm/ngày mỗi nhân vật (ví dụ Tụ Linh Đan +5, tối đa 2 viên/ngày).
- [ ] Vật phẩm tăng tốc tu luyện: hiệu lực vẫn bị trần 150/ngày.
- [ ] Cập nhật `decisions.md`: xóa cảnh báo "spirit stones have no sink" khi cửa hàng lên.

**Xong khi:** một server thử với dữ liệu giả lập đi qua ít nhất 2 mốc (một mốc theo ngưỡng, một mốc theo 150%).
Buổi giảng chịu được load test. Linh thạch có ít nhất một chỗ tiêu.

---

## Phase 3: Mini game đột phá & bí cảnh (khoảng 8–9 tuần-người)

**Mục tiêu:** đủ mini game cần cho tháng đầu (xem bảng hạn chót trong plan, mục 4.1).
**Phụ thuộc:** Phase 1 (lôi kiếp), Phase 2.4 (presence cho hộ pháp).
**Hạn:** tâm ma **trước ra mắt mở**. Hóa phàm trước ngày 20. Bí cảnh + pháp bảo trước ngày 33.

### 3.1. Khung mini game
- [ ] Bảng `trial_runs` (seed, issued_at, expires_at, submitted_at, kết quả, phán quyết).
- [ ] `POST /trials/{kind}/start` → `{runId, seed, expiresAt}`. `POST /trials/{runId}/submit` → kiểm chứng.
- [ ] Kiểm chứng: dùng một lần, thời lượng theo giờ server, điểm trong khoảng khả thi, phát lại log input (game
      tất định).
- [ ] `content/catalog/trials.yaml`: cảnh giới áp dụng, thời gian chờ khi thua (đề xuất 30 phút), độ khó.
- [ ] Web: `src/lib/minigames/` (canvas 2D thuần), khung chung: tải, đếm giờ, gửi kết quả, chế độ giảm tốc.

### 3.2. Tâm ma (lên Nguyên Anh, kiểu Tetris)
- [ ] Luật nguyên bản, tất định theo seed, có phát lại phía server (Go).
- [ ] Đột phá lên Nguyên Anh yêu cầu phiên tâm ma "qua" còn hiệu lực.
- [ ] Đồ họa nguyên bản (khối "tâm ma"), không dùng hình/âm thanh của game có thật.
- [ ] Test gian lận: gửi lại run cũ, sửa điểm, rút ngắn thời gian.

### 3.3. Hóa phàm (lên Hóa Thần, kiểu chém hoa quả)
- [ ] Luật nguyên bản, sinh vật thể theo seed, kiểm tra hợp lý phía server.
- [ ] Gắn vào điều kiện đột phá lên Hóa Thần.

### 3.4. Bí cảnh thử luyện + pháp bảo
- [ ] `content/catalog/secret_realms.yaml`: thời lượng tối đa, lượt/ngày, bảng rơi, trần phần thưởng/ngày.
- [ ] `items.yaml`: pháp bảo (độ hiếm, % tăng tỉ lệ vượt kiếp có trần), linh thạch.
- [ ] Bảng `secret_realm_runs`, phần thưởng qua ledger kho đồ.
- [ ] Mini game survivor nguyên bản (canvas 2D).
- [ ] Pháp bảo/đan/hộ pháp cộng vào tỉ lệ lôi kiếp (hoàn thiện 1.3), có trần tổng.
- [ ] Hộ pháp: đồng môn online, giới hạn lần/ngày.
- [ ] `game-balancer`: mô phỏng nguồn và chỗ tiêu linh thạch + pháp bảo trước khi mở.

**Xong khi:** người chơi đi được từ Luyện Khí đến Luyện Hư qua đủ mini game. `anti-cheat-reviewer` đã duyệt khung
trial. Kinh tế linh thạch ổn định trong mô phỏng 60 ngày.

---

## Phase 4: Closed beta & ra mắt mở (2–4 tuần)

**Mục tiêu:** hiệu chỉnh số liệu thật trước khi đồng hồ cốt truyện bắt đầu chạy.
**Phụ thuộc:** Phase 1–3 (hóa phàm và bí cảnh có thể lên sau theo hạn).

### Trước closed beta
- [ ] Pháp lý: đã có ý kiến luật sư, đủ điều kiện vận hành theo yêu cầu được xác nhận.
- [ ] ⚠️ **Kênh nạp:** chọn cổng thanh toán, tích hợp webhook idempotent → `vip_point_ledger` (1.000đ = 10 điểm), đối soát. Hiện
      chỉ có admin grant. Có thể để sau beta nếu beta không thu tiền.
- [ ] Backup + khôi phục thử Postgres. Dashboard metrics, cảnh báo.
- [ ] Load test: settle, đột phá, buổi giảng, WS.

### Closed beta (200–500 người)
- [ ] Đo điểm/ngày thực tế theo nhóm (1, 2, 3 lần đăng nhập/ngày).
- [ ] Hiệu chỉnh `threshold_points` của mốc và tốc độ treo máy.
- [ ] Soát log gian lận mini game, tỉ lệ thua tâm ma (độ khó).
- [ ] Thu phản hồi UX: luồng Tử Tiêu Cung, lôi kiếp (mất 25% có gây bỏ game không).

### Ra mắt mở
- [ ] Reset dữ liệu beta (thông báo trước), ghi `world_state.started_at` = **ngày 0**.
- [ ] Video mở đầu đã có (nếu chưa xong: ảnh tĩnh + chữ, thay sau).
- [ ] Trực vận hành 2 tuần đầu.

**Xong khi:** server mở, mốc cốt truyện đầu tiên được dự báo đúng khoảng.

---

## Phase 5: Xã hội (khoảng 4 tuần-người) · hạn ngày 30

**Phụ thuộc:** presence (2.4), kiểm duyệt (1.5).

### 5.1. Đạo lữ song tu
- [ ] Bảng `dao_lu_bonds` + partial unique index (một bond đang hiệu lực mỗi người).
- [ ] Cầu thân → xác nhận đôi, cảnh báo "vĩnh viễn", điều kiện tối thiểu (tuổi tài khoản, cảnh giới).
- [ ] Song tu: cả hai online thì tăng hiệu quả 2h/ngày, trong trần 150.
- [ ] Chấm dứt: lượng kiếp (hook cho Phase 6/8), admin sau báo cáo, vắng 90 ngày (worker).
- [ ] Chống tự song tu bằng nhiều tài khoản: theo dõi cặp cùng thiết bị/IP, cảnh báo admin.

### 5.2. Trao đổi vật phẩm
- [ ] Bảng `trades`, chuyển quyền sở hữu nguyên tử qua ledger, giới hạn lần/ngày (YAML).
- [ ] Tài khoản mới không nhận được đồ hiếm. Giới hạn giá trị nhận mỗi ngày.
- [ ] Cảnh báo luồng chuyển một chiều bất thường cho admin.
- [ ] Mở khóa ô "Túi Đồ" trên dock (kèm tab Cửa hàng nếu chưa có ở Phase 2).

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
