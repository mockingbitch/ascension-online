# Bí Cảnh thử luyện — luật chơi (rules v1)

Mini game tài nguyên của Phase 3.4 (cot-truyen §5.5, plan §3.8). Một trong bốn mini game được phép theo phạm vi
(architecture.md, decisions 2026-11-06): **phiên do server phát, server phát lại để kiểm chứng, phần thưởng có trần.**
Nguyên bản: không dùng hình, âm thanh, giao diện hay tên của game có thật. Thể loại "sinh tồn từ trên xuống" là thể loại
chung; mọi thứ cụ thể (trận đồ, tà khí, phi kiếm, kiếm trận, linh khí, kiếm ý) là thiết kế riêng.

Catalogue: `content/catalog/secret_realms.yaml` · Go: `internal/secretrealm` (luật, phần thưởng, giao dịch),
`internal/secretrealm/bicanh` (bộ mô phỏng) · TS: `apps/web/src/lib/minigames/bi-canh/sim.ts` (bản sao từng bit) ·
vector vàng chung: `packages/contracts/golden/bi-canh-v1.json` (Go test + vitest, thiếu file là test hỏng).

## 1. Người chơi thấy gì

- Một **kết giới vuông** 640 × 640 đơn vị, nền trận đồ mực nhạt. Tu sĩ đứng giữa.
- **Tà khí** tràn vào từ bốn cạnh và trôi thẳng về phía tu sĩ:
  | id | Tên | HP | Tốc độ | Điểm | Xuất hiện |
  |---|---|---|---|---|---|
  | `am_hon` | Âm Hồn | 1 | 1,75 đv/nhịp | 1 | từ đầu |
  | `hoa_linh` | Hỏa Linh | 1 | 2,9 | 2 | từ giây 20 |
  | `thach_quy` | Thạch Quỷ | 4 | 1,25 | 4 | từ giây 45 |
  | `ta_tuong` | Tà Tướng | 24 | 1,0 | 15 | đúng giây 60, 105, 135 |
- **Phi kiếm tự xuất**: mỗi `fire_every` nhịp bay về tà khí gần nhất trong tầm 230 (trùng khoảng cách: tà khí xuất
  hiện sớm hơn). Trúng tà khí đầu tiên chạm phải, gây 1 sát thương rồi tan.
- Tà khí bị hạ để lại **linh khí** (giá trị = điểm của nó). Linh khí trong 72 đv bị hút về, trong 22 đv được nhặt;
  tan sau 25 giây.
- **Kiếm ý** tăng theo linh khí đã gom (0 / 10 / 30 / 60 / 110 / 180): phi kiếm nhanh dần (14 → 8 nhịp), từ cấp 3 có
  **kiếm trận** 1–3 lưỡi xoay quanh thân (bán kính 46), mỗi lưỡi chém một tà khí tối đa 1 lần / 10 nhịp.
- Chạm tà khí khi không được bảo hộ: mất 1 **hộ thể** (có 6), tà khí đó tan (không điểm), bảo hộ 1,2 giây.
- Hết 150 giây mà còn hộ thể = **vượt bí cảnh** (`survived`); hết hộ thể = **bị đẩy ra** (`fallen`).

Người chơi chỉ điều khiển hướng đi: 8 hướng (WASD / mũi tên, hoặc nhấn giữ rồi kéo như cần điều khiển nổi, có vùng
chết 14 px và độ trễ ±30° quanh hướng đang giữ để ngón tay không rung giữa hai hướng).

**Điểm** = Σ điểm tà khí bị hạ + Σ linh khí đã nhặt.

## 2. Mô phỏng tất định

- Toàn số nguyên. Vị trí theo 1/16 đơn vị (`SUB = 16`), tốc độ theo 1/16 đv mỗi nhịp; nhịp 50 ms (20 nhịp/giây).
- Hướng chéo = `speed × 181 / 256` mỗi trục. Di chuyển về một điểm dùng độ dài octile `max + min/2` và chia cắt cụt
  (Go `/`, TS `Math.trunc`). Góc kiếm trận từ bảng cos 64 phần tử cố định (round(1024·cos)), sin = cos lệch 48.
- PRNG duy nhất xorshift32 (13/17/5) từ seed của server. Thứ tự rút: mỗi đợt `số lượng`, rồi mỗi tà khí `cạnh`,
  `vị trí dọc cạnh`, `loại` (theo trọng số của loại đã mở). Tà khí sinh trong `safe_radius` 120 quanh tu sĩ bị lật đối
  xứng qua tâm.
- Thứ tự một nhịp: di chuyển (kẹp trong kết giới) → phi kiếm (nạp, chọn mục tiêu, phóng) → kiếm bay và trúng →
  kiếm trận → dọn tà khí đã hạ → tà khí tiến lên và chạm → linh khí (nhặt / hút / phai) → kiếm ý → đợt sinh (Tà Tướng
  trước, rồi đợt thường) → giảm bảo hộ → `tick++` → kiểm tra hết giờ.
- Băm trạng thái FNV-1a 32 bit trên mọi biến và mọi thực thể (cùng thứ tự Go/TS).
- Mọi tích số < 2^53 nhờ giới hạn trong `Params.Validate` (an toàn với số JS).

## 3. Nhật ký gửi lên và kiểm chứng

Client gửi `{endTick, kills, score, stateHash, moves}`, `moves` = RLE `[hướng 0..8, số nhịp]`: 0 đứng yên, 1..8 =
B, ĐB, Đ, ĐN, N, TN, T, TB. Σ số nhịp = `endTick`, mỗi đoạn ≥ 1 nhịp, hai đoạn liền nhau khác hướng, ≤ `max_runs` 1.500
đoạn (giới hạn thân request 32 KiB, loader kiểm tra 1.500 × 12 B vừa).

Server (`secretrealm.Judge`, thuần) theo thứ tự: **trễ** (sau `expires_at`) → **log hỏng** → **điểm bất khả**
(kills vượt số tà khí có thể sinh, điểm > 2 × điểm cao nhất × kills) → **quá nhanh** (giờ server từ lúc phát <
thời gian ván × `min_elapsed_bp` 90%) → **phát lại**: nhịp người (dưới) → **lệch** (endTick/kills/score/stateHash khác
bản phát lại) → kết quả của ván. Giá trị client gửi không bao giờ được dùng làm giá trị — chỉ để so khớp.

### Luật nhịp người (cũng là luật của game)
- Trong mọi cửa sổ 20 nhịp (1 giây): **≤ 8 lần đổi hướng**. Vượt → `rejected / implausible_input`.
- Bộ ghi phía client (`Recorder`) áp đúng luật này: một lần đổi hướng vượt trần bị **hoãn** (giữ hướng cũ), nên log
  trung thực không bao giờ vi phạm. Bot tham chiếu dùng cùng bộ ghi.
- Như Tâm Ma / Hóa Phàm: đây là kiểm tra kỹ năng mềm, không chống bot tuyệt đối. Seed được gửi lúc bắt đầu nên có thể
  thử lại cục bộ cho tới khi hết hạn; `expire_after_seconds` **240** (150 s ván + 90 s) giới hạn điều đó ~1,6 lần; nộp
  muộn hơn thời gian ván + 60 s được **đánh dấu** `slowSubmit` (không từ chối). Phần thưởng nhỏ và có trần nên gian lận
  không phá được kinh tế.

### Ngưỡng tối thiểu (anti-cheat M4)
- Ván ngắn hơn `min_reward_ticks` 600 (30 giây) **không có thưởng** dù điểm bao nhiêu.
- Mỗi phẩm cần **chơi thật**: đủ điểm **và** đủ linh khí đã nhặt (phải đi tới chỗ nó) **và** đủ tà khí đã hạ.
  Đo trên 100 seed: kịch bản mù (đứng yên, vòng 8 hướng cố định 10–100 nhịp, qua lại, vuông, chéo) đạt linh khí ≤ 195 và
  tà khí ≤ 178 ở P99 (điểm ≤ 446) → **0/100** seed có thưởng; bot nhịp người cả ba tốc độ đạt ≥ Hạ Phẩm 99–100/100.
- **Cờ soát xét** trong `result.reviewFlags` (chỉ để soát, không từ chối): `mostly_idle` (di chuyển < 20% số nhịp),
  `low_change` (< 6 lần đổi hướng/phút), `periodic` (≥ 80% đoạn cùng độ dài ±1 **và** chuỗi hướng lặp chu kỳ ≤ 8).
  Mọi kịch bản mù ở trên bị gắn cờ 100/100; bot nhịp người 0/100.

## 4. Lượt và phần thưởng

- **3 lượt / ngày cho mỗi nhân vật, tính chung mọi bí cảnh** (ngày game UTC+7 lúc **bắt đầu**). Bắt đầu là tính lượt — bỏ dở không hoàn lượt (không đổi seed lấy
  ván dễ). Phiên đang mở được **tiếp tục** với cùng seed (ván chơi lại từ đầu) cho tới khi hết hạn.
- Mở từ Luyện Khí (`min_tier: 1`). **Tiên Duyên không ảnh hưởng** lượt, phần thưởng hay trần (không có khóa nào đọc
  Tiên Duyên; loader từ chối khóa lạ; balancesim và test tích hợp kiểm tra).
- Kết quả `pass` (vượt) hoặc `fail` (bị đẩy ra / tự rời) đều nhận thưởng theo **phẩm** của điểm phát lại:

| Phẩm | Điểm ≥ | Linh khí ≥ | Tà khí ≥ | Linh thạch | Linh Thảo | Cơ hội pháp bảo |
|---|---|---|---|---|---|---|
| Hạ | 450 | 200 | 180 | 10–20 | 1–3 | 0% |
| Trung | 650 | 250 | 220 | 25–35 | 2–4 | 0,5% |
| Thượng | 900 | 300 | 250 | 35–50 | 3–5 | 1,5% |
| + vượt bí cảnh | — | — | — | +10 | — | +1,5% |

- Bảng pháp bảo (trọng số): Thanh Đồng Hộ Tâm Kính 50 · Tị Lôi Châu 28 · Lôi Văn Bảo Phù 14 · Huyền Vũ Thuẫn 6 ·
  Cửu Tiêu Hoàn 2. Pháp bảo đã có → hóa **linh thạch trùng** (30 / 50 / 80 / 120 / 200).
- **Trần mỗi ngày cho mỗi nhân vật, chung mọi bí cảnh** (ngày game lúc **nộp**): 120 linh thạch, 30 nguyên liệu, 1 pháp bảo. Phần vượt trần bị cắt và báo
  rõ trên thẻ thưởng (`clipped`, `artifactCapped`).
- Vượt bí cảnh còn được hoạt động điểm duyên `trial_pass` (vip.yaml: 2 điểm, ≤ 2 lần/ngày, trong trần 20/ngày).
- Thưởng được tung xúc xắc **phía server** (RNG tiêm vào, crypto seed ở production) lúc nộp, ghi vào
  `secret_realm_runs.loot` (gồm cả số trước trần), cộng qua `currency_ledger` / `inventory_transactions` cùng giao dịch,
  khóa idempotency `sr:<run id>`. Nộp lại cùng khóa → trả kết quả cũ, không cộng lần hai (kể cả điểm duyên, kể cả sang
  ngày khác — trả về đúng dòng sổ điểm duyên đã ghi); khóa khác → 409. Phát lại chạy **ngoài** giao dịch rồi khóa và kiểm
  tra lại; điểm duyên được ghi trước linh thạch/vật phẩm (thứ tự khóa).

## 5. Độ khó (⚠ hiệu chỉnh ở closed beta)

Bot tham chiếu (`internal/secretrealm/bicanh/bicanhtest`, chỉ dùng trong test): mỗi `think` nhịp chọn 1 trong 9 hướng
theo khoảng cách dự đoán tới tà khí, nhặt linh khí khi an toàn, tránh góc; có tỉ lệ lỡ tay. 40 seed:

| Hồ sơ | think / nhìn trước / lỡ tay | Vượt | Điểm P10 · P50 · P90 |
|---|---|---|---|
| nhanh | 3 / 8 / 2% | 40/40 | 930 · 991 · 1083 |
| vừa | 5 / 7 / 5% | 39/40 | 875 · 1020 · 1065 |
| chậm | 8 / 6 / 9% | 29/40 | 640 · 947 · 1055 |

Bot nhìn thấy toàn bộ sân nên mạnh hơn người thật; mục tiêu beta: người chơi khá vượt ~70–85%, phần lớn đạt phẩm
Trung–Thượng. Lần thử đầu (sinh dày hơn, tà khí nhanh hơn) chỉ 0–22% vượt nên đã nới.

## 6. Giao diện

`/play/bi-canh`: giới thiệu + lượt còn lại + đã nhận hôm nay + bảng thưởng công khai; canvas 2D thuần, một vòng rAF,
nhịp cố định, đếm ngược 3 s, tạm dừng (P / Esc, tự dừng khi ẩn tab — giờ phiên vẫn trôi theo server), luyện tập cục bộ
"không tính". Thẻ kết quả chỉ hiện phán định và phần thưởng của server. `prefers-reduced-motion`: tắt vệt tan, nhấp nháy
bảo hộ và quầng linh khí.
