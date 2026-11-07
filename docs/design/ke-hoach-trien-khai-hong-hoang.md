# Kế hoạch triển khai chi tiết: cốt truyện Hồng Hoang

Nguồn: [cot-truyen.md](cot-truyen.md) (bản ý tưởng 06/11/2026). Đối chiếu với
[GDD](tien-dao-online-gdd-architecture.md), [decisions.md](../decisions.md) và code hiện có
(`services/game`, `apps/web`, `content/`).

Checklist theo phase: [lo-trinh-checklist.md](lo-trinh-checklist.md).

Trạng thái: **đã chốt D1, D3–D6, G1, G4 (06/11/2026, xem `decisions.md`)**. Đã chốt thêm D2, D1a/D1b, D7 (phần chỗ tiêu). Còn mở: luật trao đổi, G2, G3 (trước ngày 30), G5–G8. Ước lượng công sức tính bằng **tuần-người**
(1 dev full-stack quen codebase), chỉ là số định hướng, cần ước lượng lại khi chia task.

---

## 0. Tóm tắt

Cốt truyện mới biến game từ "idle tu luyện + Thiên Bảng" thành **game thế giới chung có cốt truyện chạy theo
tổng tu vi của cả server**, kéo dài khoảng 330+ ngày. Có thế lực, lượng kiếp làm đổi phe, nhiều mini game, đạo lữ
và **không thu phí**.

Nền móng hiện có vẫn dùng được gần hết:

| Hiện có | Dùng lại thế nào |
|---|---|
| Settle offline theo timestamp, idempotent | Treo máy động phủ (50 điểm/8h), cộng dồn vào đóng góp server |
| Breakthrough + pity + cooldown + audit roll | Mở rộng thành **lôi kiếp** (mất 25%, bảo hiểm 3 lần) |
| Catalogue YAML + validator | Thêm `factions`, `world_milestones`, chuỗi cảnh giới Vu tộc |
| Daily tasks, game day UTC+7 | Dâng hương, ngộ đạo, gieo quẻ, nhiệm vụ ngày |
| Outbox → worker → Redis, WebSocket thông báo | Mốc cốt truyện toàn server, giảng đạo trực tiếp, presence |
| Thiên Bảng (Redis ZSET) | Bảng tu vi, bảng thế lực, chốt bảng trước Phong Thần |

Phần phải làm mới: **trạng thái thế giới**, **thế lực**, **mini game có kiểm chứng phía server**,
**PvP cờ caro**, **đạo lữ**, **trao đổi vật phẩm**, và một khối nội dung lớn (lore, sự kiện, video mở đầu).

Ràng buộc lịch quan trọng nhất: các tính năng phải lên **trước ngày sớm nhất người chơi có thể chạm tới
chúng**, không phải ngày trung bình. Xem bảng hạn chót ở mục 4.

---

## 1. Xung đột với thiết kế đã chốt: cần quyết trước khi code

Cốt truyện mới mâu thuẫn trực tiếp với một số quyết định trong `decisions.md`. Mỗi dòng có đề xuất. Cần chủ dự
án chốt rồi ghi vào `decisions.md`.

| # | Vấn đề | Hiện tại | Cốt truyện mới | Đề xuất |
|---|---|---|---|---|
| D1 | **Thu phí / VIP** | VIP 0–12 theo điểm nạp | "Không thu bất kỳ phí nào" | ✅ **Đã chốt:** giữ kênh nạp, VIP đổi tên **Tiên Duyên**, điểm duyên kiếm từ hoạt động + nạp (**1.000đ = 10 điểm**). Quà ngày linh thạch giữ nguyên, cấp cao nhận nhiều hơn. Chặn: trần 150/ngày, vật phẩm tăng tỉ lệ kiếp không bán. |
| D2 | **Tên & thế giới** | "Tiên Đạo Online", thế giới tự đặt (bản cũ) | "Tu Tiên Hồng Hoang Online", thần thoại Hồng Hoang | ✅ **Đã chốt (sửa 2026-11-07):** tên sản phẩm **Tiên Đạo Online**; thế giới **Hồng Hoang**, dùng đúng tên nhân vật/thế lực/nơi chốn của `cot-truyen.md` (chủ dự án đảo ngược luật "kết hợp" ban đầu, xem `docs/decisions.md`). Slug thế lực giữ nguyên. Vẫn cần rà pháp lý/tín ngưỡng trước khi mở công khai. |
| D3 | **Thất bại đột phá** | Giữ nguyên tu vi | Từ Hóa Thần: mất 25%, chờ 24h, thua 3 lần thì lần sau chắc chắn qua | ✅ **Đã chốt** theo cốt truyện. Không tụt cảnh giới/tầng, hiện số tu vi sẽ mất trước khi bấm. |
| D4 | **Thang cảnh giới** | 11 cảnh giới | 15 cảnh giới + chuỗi Vu tộc | ✅ **Đã chốt**, giữ tầng trong mỗi cảnh giới. |
| D5 | **Thang điểm & nhịp** | Mốc 180 ngày | 100 điểm/ngày, trần 150, 330 ngày | ✅ **Đã chốt.** |
| D6 | **Mini game & PvP** | Ngoài phạm vi MVP | 4 mini game + caro PvP | ✅ **Đã chốt: bổ sung mini game.** Kiểm chứng phía server, canvas 2D thuần trong `src/lib/minigames/`, không engine. |
| D7 | **Nguồn tài nguyên** | Linh thạch chưa có chỗ tiêu | Linh thạch tăng tốc, pháp bảo tăng tỉ lệ kiếp, trao đổi được | ✅ **Đã chốt:** linh thạch tiêu vào **mua vật phẩm** ở cửa hàng trong game (không bán vật phẩm tăng tỉ lệ kiếp). ⚠️ Luật trao đổi giữa người chơi còn mở (cần trước Phase 5). |

> Các quyết định khác đã chốt **vẫn giữ**: server authoritative, settle idempotent, chỉ đăng nhập Google,
> Postgres là nguồn sự thật, không ticker theo từng người chơi.

### 1.1. Chỗ còn mâu thuẫn hoặc thiếu trong chính `cot-truyen.md`

| # | Chỗ | Vấn đề | Đề xuất |
|---|---|---|---|
| G1 ✅ | Phong Thần (6.2 và 7.3) | "Không tham gia từ đầu: không sao" nhưng "không chơi đủ 15 ván: luân hồi". Người không tham gia cũng chơi chưa đủ 15 ván. | **Ứng Kiếp là đăng ký tự nguyện.** Chỉ người đã đăng ký mới bị luật 15 ván. |
| G2 ⏸ bổ sung sau | Đại chiến Vu Yêu | Chưa có cách chơi: người chơi làm gì trong 15 ngày chiến? | Cần thiết kế riêng (đề xuất: chiến dịch góp công theo thế lực, xem 3.9). |
| G3 ⏸ bổ sung sau | Yêu tộc, Tiên Đình, Nhân tộc | Chưa nói Yêu tộc dùng chuỗi cảnh giới nào; Tiên Đình (Đông Vương Công) ra sao sau chiến; Nhân tộc tu theo đạo đồ nào | Yêu tộc và Tiên Đình dùng đạo đồ tiên. Tiên Đình sáp nhập/giải tán ở mốc Vu Yêu. Nhân tộc dùng đạo đồ tiên. Cần chốt. |
| G4 ✅ | Luân hồi | "Giữ một phần tu vi" chưa có số | **Đã chốt: giữ 50% tu vi.** |
| G5 | Lôi kiếp | Mục 5.2 nói "từ Hóa Thần", bảng 6.4 ghi Hóa Thần "Không" | Hiểu là: **đột phá *rời* Hóa Thần (lên Luyện Hư) trở đi** mới có kiếp. Ghi rõ trong catalogue. |
| G6 | "Hoạt động cộng đồng 20 điểm" | Chưa định nghĩa | Góp công thế lực, hộ pháp đồng môn, có mặt ở phòng chung. |
| G7 | Tây Du (ngày 330+) | Chưa có nội dung | Thiết kế sau, khi đã có số liệu 6 tháng đầu. |
| G8 | Thần vị | 365 thần vị, "hiệu ứng nhỏ" chưa định | Chỉ danh hiệu + ngoại hình + một tiện ích nhỏ, không cộng tu vi. |

---

## 2. Kiến trúc mở rộng

Giữ **modular monolith**. Mỗi hệ thống là một package domain mới trong `services/game/internal/`, theo đúng mẫu
hiện có: hàm luật thuần + `Service` sở hữu transaction, side effect sang domain khác đi qua `outbox_events`.

### 2.1. Module mới

| Package | Trách nhiệm | Dữ liệu chính |
|---|---|---|
| `world` | Tổng tu vi server, mốc cốt truyện, kỷ nguyên hiện tại, chặn 70%/150% | `world_state`, `world_milestones_reached`, `daily_contribution` |
| `faction` | Thế lực, gia nhập/đổi phe, khóa phe (Phong Thần), chuyển phe theo lượng kiếp | `character_faction`, `faction_history` |
| `tribulation` | Luật lôi kiếp (mở rộng `breakthrough`): tỉ lệ, mất 25%, bảo hiểm, hộ pháp | dùng lại `breakthrough_attempts` + cột mới |
| `trial` | Khung mini game: phát phiên có seed, nhận kết quả, kiểm chứng, cấp cửa đột phá | `trial_runs` |
| `secretrealm` | Bí cảnh: phiên chơi, phần thưởng có trần, chống gian lận | `secret_realm_runs` + ledger kho đồ |
| `lecture` | Giảng đạo trực tiếp 2 buổi/ngày, điểm danh, phòng chung | `lecture_sessions`, `lecture_attendance`; presence ở Redis |
| `ritual` | Dâng hương (chuỗi ngày), ngộ đạo/gieo quẻ (buff nhỏ) | mở rộng `dailytask` |
| `bond` | Đạo lữ: cầu thân, xác nhận đôi, song tu, chấm dứt theo luật | `dao_lu_bonds` (unique một bond đang hiệu lực mỗi người) |
| `trade` | Tặng/đổi vật phẩm, giới hạn ngày, nhật ký | `trades` + ledger hiện có |
| `caro` | Ghép trận, phòng WS, đồng hồ 10s phía server, bot thay thế | `caro_matches` |
| `fengshen` | Đăng ký Ứng Kiếp, đếm ván, chốt bảng, xếp 365 thần vị | `fengshen_entries`, `divine_ranks` |
| `moderation` | Lọc đạo hiệu, báo cáo, xử lý quấy rối, giải bond | `reports`, `moderation_actions` |

### 2.2. Catalogue mới (`content/catalog/`)

- `realms.yaml`: thêm `paths:` (`tien`, `vu`), mỗi cảnh giới có `tier` để quy đổi ngang bậc, `tribulation:`
  (`base_bp`, `step_bp`, `floor_bp`, `loss_bp: 2500`, `cooldown_seconds: 86400`, `guarantee_after: 3`).
- `factions.yaml`: id, tên, đạo đồ cho phép, kỷ nguyên xuất hiện/biến mất, luật chuyển phe khi qua mốc.
- `world_milestones.yaml`: id, `target_day`, `threshold_points`, `min_pct: 7000`, `max_pct: 15000`, hiệu ứng
  (mở phe, tách phe, bắt đầu sự kiện), mẫu thông báo.
- `trials.yaml`: tâm ma / hóa phàm, cảnh giới áp dụng, thời gian chờ khi thua, độ khó.
- `secret_realms.yaml`: bảng rơi, trần phần thưởng/ngày, giới hạn số lượt.
- `lectures.yaml`: lịch giảng (giờ VN), thời lượng, điểm danh.
- `contribution.yaml`: trần 150 điểm/ngày, cơ cấu 50/30/20, hệ số bù cho người vào muộn.

Mọi số đều ở YAML, Go không hardcode (đúng rule hiện tại).

### 2.3. Cơ chế cốt truyện toàn server (`world`)

```
mỗi lần cộng tu vi (settle, giảng đạo, nhiệm vụ…)
  └─ trong cùng transaction: cộng daily_contribution(character, game_day), chặn ở trần 150
       └─ outbox: world.contribution
worker (theo chu kỳ, KHÔNG theo người chơi)
  ├─ cộng dồn tổng server từ daily_contribution (nguồn sự thật = Postgres)
  ├─ mốc kế tiếp đạt khi: tổng ≥ threshold VÀ ngày ≥ 70% target
  │                      HOẶC ngày ≥ 150% target (chạy theo thời gian khi server vắng)
  ├─ ghi world_milestones_reached (idempotent theo milestone_id)
  └─ outbox: world.milestone_reached → thông báo WS + hiệu ứng (mở phe, sự kiện…)
```

- Thời điểm bắt đầu tính ngày = `world_state.started_at` (ngày mở server), không dùng giờ client.
- Hệ số bù người vào muộn (x2) áp **trước** trần 150, nên không ai vượt trần.
- Không có đếm tu vi realtime trên client. Client chỉ đọc `GET /world` (kỷ nguyên, % tới mốc kế tiếp).

### 2.4. Mini game & chống gian lận (áp dụng D6)

Mini game chạy trong trình duyệt nên **không thể tin kết quả client**. Nguyên tắc:

1. Server phát phiên: `POST /trials/{kind}/start` → `{runId, seed, expiresAt}`. Mỗi phiên dùng một lần.
2. Client chơi với seed đó, gửi `{runId, inputs log, score}`.
3. Server kiểm tra: thời lượng hợp lý (theo timestamp server), log input khớp seed (mini game theo lượt hoặc tất
   định thì phát lại được), điểm trong khoảng khả thi.
4. **Phần thưởng nhỏ và có trần**: tâm ma/hóa phàm chỉ mở cửa đột phá; bí cảnh có trần tài nguyên mỗi ngày,
   nên gian lận cũng không phá được kinh tế.
5. Ghi toàn bộ phiên để soát lại sau.

Mini game kiểu Tetris và chém hoa quả nên viết **tất định theo seed** để server phát lại được. Bí cảnh kiểu
Vampire Survivors khó phát lại hơn, nên dựa vào trần + kiểm tra hợp lý thay vì phát lại toàn bộ.

Công nghệ: DOM/SVG đủ cho Tetris-like và caro. Chém hoa quả và bí cảnh có nhiều vật thể chuyển động, nên cần
**`<canvas>` 2D thuần** cho riêng hai mini game này. Điều này chạm vào luật "MVP không canvas/PixiJS" và cần ghi
quyết định. Đề xuất: cho phép canvas 2D thuần trong `src/lib/minigames/`, vẫn không dùng engine.

---

## 3. Chi tiết từng hệ thống

Mỗi hệ thống: phạm vi · phụ thuộc · rủi ro · ước lượng.

### 3.1. Nền móng chuyển đổi (bắt buộc làm đầu tiên)
- Viết lại `realms.yaml` (15 cảnh giới đạo đồ tiên + 15 đạo đồ Vu, có tầng), loader validate bậc tương đương.
- Đổi thang điểm sang 100/ngày (`*_milli`), cập nhật `cultivation` settle, `cmd/balancesim` mục tiêu 330 ngày.
- Đổi VIP thành Tiên Duyên theo D1: giữ đường nạp (1.000đ = 10 điểm duyên), thêm nguồn điểm duyên trong game, kiểm tra lại công bằng (VIP cao nhất đăng nhập 1 lần/ngày không hơn người miễn phí đăng nhập 3 lần/ngày, không vượt trần 150).
- Cập nhật `story-writing.md` theo canon mới (thế giới Hồng Hoang, tên theo `cot-truyen.md` từ khi D2 bị đảo ngược 2026-11-07).
- Reset DB dev (chưa ra mắt).
- **Ước lượng: 2–3 tuần-người.** Rủi ro: chạm nhiều file, cần test kỹ settle/breakthrough.

### 3.2. Lôi kiếp
- Tỉ lệ: 80% ở kiếp đầu, −3%/cảnh giới, sàn 50%. Cộng thêm: dư tu vi so với ngưỡng, pháp bảo, đan hộ thân,
  hộ pháp đồng môn. Tất cả cộng có trần và hiển thị từng dòng trước khi bấm.
- Thất bại: trừ 25% tu vi tầng hiện tại, cooldown 24h, tăng bộ đếm. Thua 3 lần liên tiếp thì lần 4 = 100%.
- Hộ pháp: đồng môn cùng thế lực, đang online (presence), mỗi người hộ pháp tối đa N lần/ngày.
- UI: dùng lại `BreakthroughPanel`/celebration, thêm hiệu ứng thất bại mất tu vi (đã có backlash).
- **Ước lượng: 1,5 tuần-người.** Phụ thuộc: 3.1, presence (3.5).

### 3.3. Thế giới chung & mốc cốt truyện
- Module `world` (2.3), `GET /world`, thanh "Thiên Đạo vận chuyển" trên HUD, thông báo mốc qua WS.
- Admin: xem tổng, xem dự báo ngày đạt mốc, tạm dừng/đẩy mốc thủ công (có audit).
- **Ước lượng: 2 tuần-người.**

### 3.4. Thế lực & mở đầu Tử Tiêu Cung
- Luồng người mới: video mở đầu (bỏ qua được) → Tử Tiêu Cung, Hồng Quân giảng đạo → chọn thế lực → chọn đạo đồ
  (Vu tộc thì đạo đồ Vu) → đặt đạo hiệu (đã có lọc tên ở 3.10).
- 5 thế lực ban đầu, đổi phe tự do có thời gian chờ (đề xuất 7 ngày), bảng xếp hạng thế lực (ZSET mới).
- Đổi giữa Vu và tiên: quy đổi theo `tier`, giữ nguyên điểm.
- **Ước lượng: 2,5 tuần-người** (chưa gồm video).

### 3.5. Giảng đạo trực tiếp & phòng chung
- 2 buổi cố định (sáng/tối, giờ VN) trong `lectures.yaml`. Phòng WS chung, hiện số người đang nghe.
- Điểm danh nhận tu vi (thuộc phần 30 điểm/ngày), có tương tác nhẹ giữa buổi (ngộ đạo) để chống treo bot.
- Presence: heartbeat vào Redis (khóa có TTL), không ghi DB mỗi nhịp. Dùng chung cho hộ pháp, song tu.
- Kiểm tra tải: một buổi giảng có thể có hàng nghìn kết nối cùng lúc (đã có `WS_MAX_CLIENTS`).
- **Ước lượng: 2 tuần-người.**

### 3.6. Hoạt động hằng ngày
- Dâng hương buổi sáng có chuỗi ngày, ngộ đạo/gieo quẻ (buff nhỏ, công bố rõ tỉ lệ), việc vặt động phủ
  (hái linh dược, luyện đan đơn giản → tiêu linh thạch, là chỗ tiêu còn thiếu).
- Dùng lại `dailytask` và game day UTC+7.
- Lưu ý tín ngưỡng: dâng hương **không gắn "nhận lộc"** và không dâng cho nhân vật tôn giáo có thật (xem mục 6).
- **Ước lượng: 1,5 tuần-người.**

### 3.7. Mini game đột phá: tâm ma (Nguyên Anh) và hóa phàm (Hóa Thần)
- Khung `trial` (2.4) + 2 mini game nguyên bản. Chỉ mượn thể loại, không dùng tên, hình, âm thanh của game có thật.
- Thua: không mất tu vi, chờ ngắn (đề xuất 30 phút) rồi thử lại.
- Có chế độ hỗ trợ tiếp cận: giảm tốc độ, hoặc phương án thay thế cho người dùng bàn phím/màn đọc.
- **Ước lượng: 4 tuần-người** (khung 1, mỗi game 1,5).

### 3.8. Bí cảnh thử luyện + pháp bảo
- Mini game kiểu survivor (canvas 2D), phiên tối đa vài phút, số lượt/ngày có trần.
- Rơi: linh thạch, pháp bảo (độ hiếm, tăng tỉ lệ vượt kiếp có trần). Định nghĩa vật phẩm trong `items.yaml`.
- Kinh tế: theo dõi tạo/hủy linh thạch, mô phỏng lại trong balancesim trước khi mở.
- **Ước lượng: 4–5 tuần-người.** Đây là món nặng và rủi ro gian lận cao nhất.

### 3.9. Đại chiến Vu Yêu (cần thiết kế, G2)
- Đề xuất: chiến dịch 15 ngày theo thế lực. Mỗi người góp "chiến công" từ hoạt động sẵn có (bí cảnh, tu luyện,
  hộ pháp), Vu tộc có hệ số chiến đấu, có hiệu ứng tập thể (Đại trận Tổ Vu / Chu Thiên Tinh Đấu) khi đủ người
  online. Kết cục theo cốt truyện cố định (hai tộc suy tàn), người chơi quyết định *danh hiệu và phần thưởng*.
- Sau chiến: mở Địa Phủ và Lục Đạo Luân Hồi. Người chơi Vu/Yêu chọn Địa Phủ, luân hồi Nhân tộc, hoặc luân hồi
  loài vật (giữ 50% tu vi, G4).
- **Ước lượng: 3 tuần-người** sau khi chốt thiết kế.

### 3.10. An toàn, kiểm duyệt, dữ liệu
- Lọc đạo hiệu: danh sách từ cấm tiếng Việt (có/không dấu, biến thể), chặn mạo danh tên nhân vật cốt truyện,
  báo cáo tên.
- Báo cáo quấy rối, công cụ admin xử lý (có audit). Ghi rõ dữ liệu thu (chỉ email Google + đạo hiệu).
- **Ước lượng: 1,5 tuần-người.** Nên làm trước khi mở bất kỳ tính năng xã hội nào.

### 3.11. Đạo lữ song tu
- Cầu thân → xác nhận đôi, kèm cảnh báo "vĩnh viễn". Unique một bond đang hiệu lực mỗi người (partial index).
- Song tu: cả hai cùng online (presence) thì tăng hiệu quả trong 2h/ngày, vẫn trong trần 150.
- Chấm dứt: lượng kiếp (luân hồi, Phong Thần), quản trị viên (sau báo cáo), hoặc một bên vắng 90 ngày.
- Chống lạm dụng: một người dùng nhiều tài khoản để tự song tu. Giới hạn: tài khoản phải đủ tuổi và đạt
  cảnh giới tối thiểu, theo dõi cặp đăng nhập cùng thiết bị/IP.
- **Ước lượng: 2 tuần-người.**

### 3.12. Trao đổi vật phẩm
- Tặng/đổi trực tiếp, chuyển quyền sở hữu nguyên tử, giới hạn số lần/ngày, nhật ký đầy đủ.
- Chống mua bán tiền thật ngoài game: giới hạn giá trị nhận mỗi ngày, tài khoản mới không nhận được đồ hiếm,
  cảnh báo luồng chuyển một chiều bất thường.
- **Ước lượng: 2 tuần-người.**

### 3.13. Nữ Oa, Thánh nhân, tách phe
- Mốc Nữ Oa: mở phe Nhân tộc (người luân hồi vào được) và Oa Hoàng Cung.
- Thánh nhân xuất thế (ngày 140–170): Tam Thanh tách 3 giáo (người chơi chọn, giữ tu vi), Chuẩn Đề tự chuyển
  Tây Phương Giáo. Thực hiện bằng job **idempotent** theo mốc, có thông báo trước 3 ngày.
- **Ước lượng: 2 tuần-người** (phần lớn là luật chuyển phe + UI chọn).

### 3.14. Phong Thần Lượng Kiếp + cờ caro
- Caro PvP qua WS: server giữ bàn cờ, kiểm tra nước đi, đồng hồ 10s phía server, hết giờ là thua.
- Ghép trận ngẫu nhiên, ẩn danh trước ván, không ghép cùng thế lực hoặc đạo lữ. Có bot khi chờ quá lâu.
- Đăng ký Ứng Kiếp (G1), đếm ván. Thua 10 ván thì lên Phong Thần Bảng, khóa phe, trần Thái Ất. Đã đăng ký mà chưa
  đủ 15 ván khi hết sự kiện thì luân hồi.
- Chốt bảng tu vi ngay trước sự kiện. 365 thần vị xếp theo tu vi trong số người lên bảng.
- Chống cố tình thua: theo dõi cặp lặp lại, tỉ lệ thua bất thường, thời gian đi nước bất thường.
- **Ước lượng: 5 tuần-người.** Đây là tính năng realtime PvP đầu tiên, cần load test.

### 3.15. Thiên Đình tái lập, Tây Du
- Thiên Đình (ngày ~310): trao thần vị, chức vị thấp nhất cho người vào sau.
- Người trên Phong Thần Bảng chạm Thái Ất: tu vi dư quy đổi thành công đức.
- Tây Du: thiết kế sau (G7).
- **Ước lượng: 2 tuần-người** (chưa gồm Tây Du).

### 3.16. Video mở đầu (~3 phút 30 giây)
- Sản xuất bên ngoài code: kịch bản 8 cảnh (đã có), storyboard, minh họa 2D tối giản theo phong cách thủy mặc
  hiện tại, giọng kể, nhạc **có giấy phép rõ ràng**.
- Trong web: file video tự host (WebM/MP4), phụ đề, bỏ qua được, xem lại trong menu. Không bắt tải trước khi chơi.
- **Ước lượng phía dev: 0,5 tuần-người.** Phần sản xuất cần người làm đồ họa/âm thanh, ước lượng riêng.

---

## 4. Lộ trình

### 4.1. Hạn chót theo ngày trong game

Người chơi nhanh nhất bị trần 150 điểm/ngày. Mốc cốt truyện sớm nhất là 70% ngày mục tiêu. Tính năng phải **lên
trước** ngày sớm nhất, đề xuất trừ ít nhất 7 ngày dự phòng.

| Tính năng cần có | Ai chạm sớm nhất | Ngày sớm nhất | Phải lên trước ngày |
|---|---|---|---|
| Mini game tâm ma (lên Nguyên Anh, 2.400 điểm) | Người cày trần | 16 | **ra mắt** |
| Mini game hóa phàm (lên Hóa Thần, 4.000) | Người cày trần | 27 | 20 |
| Lôi kiếp + pháp bảo + bí cảnh (lên Luyện Hư, 6.000) | Người cày trần | 40 | 33 |
| Giảng đạo lần 2 (mốc 30) | Server | 21 | 14 |
| Giảng đạo lần 3 (mốc 60) | Server | 42 | 35 |
| Đại chiến Vu Yêu + Địa Phủ/luân hồi (mốc 100) | Server | 70 | 60 |
| Nữ Oa + phe Nhân tộc (mốc 130) | Server | 91 | 80 |
| Thánh nhân, tách giáo (mốc 140–170) | Server | 98 | 85 |
| Tam Hoàng Ngũ Đế (mốc 150–210) | Server | 105 | 95 |
| Phong Thần + caro (mốc 240) | Server | 168 | 150 |
| Thiên Đình tái lập (mốc 310) | Server | 217 | 200 |
| Tây Du (mốc 330) | Server | 231 | 210 |

Hệ quả: **tâm ma phải có từ ngày ra mắt**, hóa phàm và bí cảnh trong tháng đầu. Đây là lý do mini game nằm
trước ra mắt trong lộ trình dưới đây.

### 4.2. Các giai đoạn

**Giai đoạn 0: Chốt thiết kế** (1 tuần, chủ yếu thiết kế)
- Chốt D1–D7, G1–G8, ghi vào `decisions.md`.
- Viết lại canon Hồng Hoang cho `content/story/lore/` (agent `story-creator`), chọn một mạch truyện nhất quán.
- Rà pháp lý và tín ngưỡng (mục 6).

**Giai đoạn 1: Nền móng** (khoảng 5 tuần-người)
- 3.1 Chuyển đổi nền · 3.2 Lôi kiếp · 3.10 Kiểm duyệt.

**Giai đoạn 2: Thế giới & thế lực** (khoảng 8 tuần-người)
- 3.3 Thế giới chung · 3.4 Thế lực + Tử Tiêu Cung · 3.5 Giảng đạo + presence · 3.6 Hoạt động ngày.

**Giai đoạn 3: Mini game trước ra mắt** (khoảng 8–9 tuần-người)
- 3.7 Tâm ma + hóa phàm · 3.8 Bí cảnh + pháp bảo. Có thể tách: tâm ma bắt buộc lúc ra mắt, hóa phàm và bí cảnh
  có thể lên trong tuần 2–4 sau ra mắt nếu kịp hạn ở 4.1.

**Ra mắt kín (closed beta, 2–4 tuần)**: 200–500 người chơi, đo điểm/ngày thực tế để chỉnh mốc
(`threshold_points` có thể đổi trong YAML), soát kinh tế, load test buổi giảng.

**Ra mắt mở** = **ngày 0** của cốt truyện (`world_state.started_at`).

**Giai đoạn 4: Xã hội** (khoảng 4 tuần-người, xong trước ngày ~30)
- 3.11 Đạo lữ · 3.12 Trao đổi.

**Giai đoạn 5: Lượng kiếp thứ nhất** (khoảng 5 tuần-người, xong trước ngày 60)
- 3.9 Đại chiến Vu Yêu + Địa Phủ + luân hồi.

**Giai đoạn 6: Thánh nhân & Nhân tộc** (khoảng 2–3 tuần-người, xong trước ngày 80–95)
- 3.13.

**Giai đoạn 7: Phong Thần** (khoảng 5–6 tuần-người, xong trước ngày 150)
- 3.14 + load test PvP.

**Giai đoạn 8: Thiên Đình & Tây Du** (xong trước ngày 200–210)
- 3.15 + thiết kế và làm Tây Du.

Tổng trước ra mắt: khoảng **21–23 tuần-người**. Sau ra mắt còn khoảng **16–18 tuần-người** (chưa gồm Tây Du),
phải kịp theo lịch cốt truyện. Với 2 dev, trước ra mắt khoảng 3 tháng lịch.

### 4.3. Song song với dev
- **Nội dung** (`story-creator`): lore từng kỷ nguyên, lời giảng đạo cho mỗi buổi (2 buổi × 330 ngày là rất
  nhiều, cần mẫu + biến thể), thông báo mốc, văn bản sự kiện.
- **Cân bằng** (`game-balancer`, `make balance`): mốc 330 ngày, tỉ lệ kiếp, trần 150, hệ số bù x2, nguồn/chỗ
  tiêu linh thạch.
- **Đồ họa** (`graphic-designer`): huy hiệu 5 thế lực, chuỗi cảnh giới Vu (aura riêng), Tử Tiêu Cung, động phủ.
- **Video mở đầu**: sản xuất ngoài, bắt đầu từ giai đoạn 0 vì cần nhiều thời gian nhất.

---

## 5. Cân bằng: số liệu cần kiểm chứng

- Nhịp chuẩn 100 điểm/ngày = 50 (treo máy, 8h) + 30 (giảng đạo + nhiệm vụ) + 20 (cộng đồng).
  Người chỉ treo máy đạt khoảng 50/ngày, tức **tới Chuẩn Thánh mất khoảng 660 ngày**. Cần quyết đây có phải
  chủ đích không.
- Trần 150/ngày: người cày nhất tới Chuẩn Thánh ở ngày 220, trước Tây Du 110 ngày. Họ sẽ làm gì trong thời gian
  đó? Đề xuất: công đức, hộ pháp, nội dung thế lực.
- Lôi kiếp: tỉ lệ trung bình khoảng 65% và bảo hiểm sau 3 lần, kèm mất 25% và chờ 24h. Mô phỏng xem một kiếp tốn
  trung bình bao nhiêu ngày, cộng vào bảng 6.4.
- Mốc server giả định 10.000 người × 100 điểm. Server vắng (ví dụ 1.000 người) thì mốc sẽ chạy theo **150% thời
  gian**: giảng đạo lần 2 ở ngày 45, Đại chiến ở ngày 150. Cần quyết đây có chấp nhận được không, hay chỉnh
  `threshold_points` theo số người hoạt động thực tế.

---

## 6. Rủi ro

| Rủi ro | Mức | Giảm thiểu |
|---|---|---|
| **Bản quyền nội dung**: canon "Hồng Hoang" (Hồng Quân giảng đạo 3 lần, Vu Yêu đại chiến, Tổ Vu, Hậu Thổ hóa luân hồi, cảnh giới Chuẩn Thánh…) phần lớn được định hình bởi tiểu thuyết mạng Trung Quốc hiện đại, không chỉ thần thoại hay *Phong Thần Diễn Nghĩa* / *Tây Du Ký* (đã thuộc phạm vi công cộng). | Cao | Viết toàn bộ văn bản mới, không sao chép lời văn hoặc nhân vật do tiểu thuyết mạng tự tạo. Ưu tiên nguồn thần thoại và tác phẩm cổ điển. Nhờ luật sư rà. Cập nhật rule "original content only" trong `story-writing.md` cho phù hợp. |
| **Tín ngưỡng**: Tam Thanh, Lão Tử (Đạo giáo), Chuẩn Đề, Tiếp Dẫn, Tây Phương Giáo, Phật trong Tây Du (Phật giáo) là đối tượng tín ngưỡng có thật. Việt Nam có cộng đồng Phật giáo lớn. | Cao | Không để người chơi đánh, hạ thấp hay giễu nhân vật thờ phụng. Nghi lễ chỉ mang tính tượng trưng, không "nhận lộc". Rà nội dung với người am hiểu. Cân nhắc kỹ việc đưa Phật vào giai đoạn Tây Du. |
| **Pháp lý vận hành game**: game online nhiều người chơi tại Việt Nam có quy định về cấp phép, phê duyệt nội dung, định danh tài khoản, giới hạn thời gian cho người dưới 18 (Nghị định 147/2024/NĐ-CP). | Cao | Nhờ luật sư xác nhận yêu cầu cụ thể **trước ra mắt mở**. Không thu phí không có nghĩa là được miễn. |
| **Gian lận mini game** | Trung bình | Kiểm chứng phía server (2.4), phần thưởng có trần, soát phiên bất thường. |
| **Mua bán tiền thật ngoài game** | Trung bình | Giới hạn trao đổi, nhật ký, cảnh báo luồng bất thường (3.12). |
| **Ràng buộc đạo lữ vĩnh viễn giữa người thật** | Trung bình | Xác nhận đôi, cảnh báo, quyền giải của admin, 90 ngày vắng (3.11). |
| **Lịch nội dung cứng 330 ngày** với đội nhỏ | Cao | Mốc có trần 150% nên có đệm tự nhiên. Admin có thể dời mốc. Ưu tiên làm xong lượng kiếp kế tiếp trước khi mở lượng kiếp sau. |
| **Nạp tiền làm lệch công bằng** | Trung bình | Đã chấp nhận (D1a): quà linh thạch giúp gián tiếp. Còn lại hai chốt chặn: trần 150/ngày và vật phẩm tăng tỉ lệ kiếp không bán. Đo bằng `make balance` mỗi lần đổi giá cửa hàng hoặc quà ngày. |
| **Buổi giảng đông người cùng lúc** | Trung bình | Load test WS (k6), một thông điệp broadcast thay vì trạng thái riêng từng người. |

---

## 7. Bước tiếp theo đề xuất

1. Chủ dự án chốt **D1–D7, G1–G8** (mục 1). Có thể chốt nhanh bằng cách trả lời từng dòng "đồng ý / sửa".
2. Ghi các quyết định vào `docs/decisions.md`, cập nhật `CLAUDE.md` (tên, phạm vi) và `story-writing.md` (canon).
3. Chạy `data-architect` để chốt schema catalogue mới (`realms` có `paths`, `factions`, `world_milestones`).
4. Chạy `game-balancer` với mốc 330 ngày, kiếp và trần 150.
5. Bắt đầu Giai đoạn 1 (nền móng), song song với kịch bản và sản xuất video mở đầu.
