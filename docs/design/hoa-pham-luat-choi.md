# Hóa Phàm — luật chơi (rules v1)

Thử luyện chắn bước đột phá **Nguyên Anh → Hóa Thần** (Vu đạo: bậc 4 → 5) — cot-truyen §5.4, decisions D6, lộ trình
Phase 3.3. Số liệu nằm ở `content/catalog/trials.yaml` (`hoa_pham`); tài liệu này mô tả luật, không lặp số. Số trong
ngoặc là giá trị hiện tại, **chỉ để tham khảo, sẽ hiệu chỉnh ở closed beta**.

## Ý tưởng

Muốn hóa thần, trước hết phải **hóa phàm**: trở về sự giản phác của người thường, cắt bỏ trần duyên vướng víu. Từ
mặt hồ thức hải, những **nút trần duyên** — sợi chỉ đỏ rối, sợi tơ sen, tấm lưới nghiệp — trôi lên rồi chìm xuống.
Người chơi vung bút như vung kiếm, **chém đứt** chúng. Lẫn trong đó là những **đèn chân tâm** thả bay lên trời: đó là
tâm thật của mình, chém vào là tự dập tắt.

Thể loại "vuốt để chém vật bay lên" là thể loại chung; Hóa Phàm có vật thể, luật chém, hình ảnh và cơ chế riêng —
không có trái cây, không có bom, không có vệt kiếm phát sáng, không dùng tên, âm thanh hay giao diện của game có thật.
Nét chém là **một nét bút lông thuôn dần rồi khô**, không phải lưỡi kiếm.

## Sân và vật thể

- Sân **120 × 160 đơn vị lưới** (`width`, `height`, dọc 3:4). Con trỏ được lượng tử về **đơn vị nguyên**; vật thể
  di chuyển theo đơn vị con = 1/32 đơn vị lưới (`Sub`), để chuyển động chậm vẫn là số nguyên.
- Bốn loại (`objects`):

| id | Tên | Vai | Bán kính | Điểm | Nét cần | Trọng số |
|---|---|---|---|---|---|---|
| `ket` | Kết Trần Duyên | nút (chỉ đỏ rối) | 9 | 1 | 1 | 6 |
| `to` | Tơ Vương | nút nhỏ (tơ sen) | 6 | 2 | 1 | 3 |
| `luoi` | Lưới Nghiệp | lưới lớn | 13 | 3 | **2** | 1 |
| `den` | Đăng Chân Tâm | **đèn — đừng chém** | 9 | — | — | 2 |

- **Nút** được tung lên từ dưới mép sân với vận tốc dọc ngẫu nhiên trong `knot_motion.vy_sub`, trôi ngang tối đa
  `vx_max_sub` (ở 1/3 sân bên trái luôn trôi sang phải, 1/3 bên phải sang trái, ở giữa ngẫu nhiên), chịu `gravity_sub`
  kéo xuống: lên tới đỉnh trong sân rồi chìm lại. Rơi khỏi đáy (hoặc trôi khỏi mép) mà chưa đứt → **sót**.
- **Đèn** bay lên đều (`lantern_motion`), không chịu trọng lực, ra khỏi mép trên thì biến mất.
- **Đợt**: cứ `max(min_ticks, start_ticks − tick / step_every_ticks)` tick một đợt (1,8 s lúc đầu → 0,9 s), mỗi đợt
  `1 + rand(min(wave_max, 1 + tick / wave_grow_every_ticks))` vật (tối đa 1 → 4 vật theo thời gian). Đợt đầu tới sau
  ~12 tick.

## Chém

- Khi nhấn giữ (ngón tay hoặc chuột), mỗi tick (`tick_ms` = 40 ms) lấy **một điểm** con trỏ. Hai điểm liên tiếp của
  cùng một lần nhấn tạo một **đoạn**; mỗi tick đoạn đó được xét va chạm với mọi vật **ở vị trí đầu tick**.
- Đoạn chỉ chém được khi dài ít nhất `min_cut_step` (3 đơn vị/tick, độ dài bát giác
  `max(|dx|,|dy|) + min(|dx|,|dy|)/2`): **rê chậm không đứt**, nét phải dứt khoát.
- Va chạm: khoảng cách từ tâm vật tới đoạn ≤ bán kính (phép thử nguyên: tích vô hướng + tích có hướng bình phương).
- Mỗi **nét** (một lần nhấn) chạm một vật nhiều nhất một lần. Lưới Nghiệp cần **hai nét khác nhau**: nét đầu làm rách.
- **Liên đoạn**: nút thứ k bị đứt trong cùng một nét được `điểm + min(max_bonus, (k − 1) × step_bonus)` (k = 1, 2, 3, 4+
  → +0, +1, +2, +3).
- **Chém đèn chân tâm**: mất một **tâm đăng** (có `lanterns` = 3), chuỗi liên đoạn của nét về 0, thanh tâm tĩnh về 0.
  Hết tâm đăng → **thua** (`lanterns_out`).

## Tâm tĩnh

Mỗi nút đứt nạp `charge_per_cut` vào thanh tâm tĩnh; đầy (`charge_max` = 12) thì **tự vào tâm tĩnh** trong
`duration_ticks` (5 s): vạn vật chỉ chuyển động ở tick chẵn — **chậm một nửa**, kể cả nhịp sinh đợt. Một nút **sót**
hoặc một đèn bị chém (khi chưa vào tâm tĩnh) làm thanh về 0 — phải đoạn liền, không để sót.

## Nhịp (thứ tự mỗi tick)

1. Nếu đang nhấn: tick đầu của nét → mở nét mới (không có đoạn); các tick sau → đoạn từ điểm trước tới điểm này chém
   vật (theo thứ tự danh sách).
2. Nếu không tâm tĩnh, hoặc tâm tĩnh và bộ đếm chẵn: vật di chuyển (`x += vx; y += vy; nút: vy += gravity`), vật ra
   khỏi sân bị bỏ (nút → sót), bộ đếm đợt tăng, đủ thì sinh đợt. Rồi bộ đếm tâm tĩnh giảm.
3. Tăng tick. Đủ `pass_score` **và** tick ≥ `min_pass_ticks` → **qua** (`cleared`). Hết `time_limit_ticks` (90 s)
   → **thua** (`time_up`).

## Điều kiện qua

Đạt **`pass_score` (75) điểm trần duyên** và **giữ tâm tới `min_pass_ticks` (60 s)** mà đèn chân tâm chưa tắt hết.
Đạt đủ sớm thì phải tiếp tục (giao diện hiện "Giữ tâm thêm Ns"); một lần qua vì thế luôn tốn thời gian thật. Thua
(`lanterns_out`, `time_up`, `gave_up`) không mất tu vi; chỉ chờ `lose_cooldown_seconds` (30 phút).

Độ khó đo bằng người chơi mẫu (`internal/trial/hoapham/hoaphamtest`, chỉ dùng trong test): chỉ nhắm nút đã thấy ít
nhất `React` tick, dự đoán vị trí theo vận tốc, thử 8 hướng vuốt qua nút và chọn nét đứt nhiều nhất mà không chạm đèn
(nhìn trước như một người chú ý), **rồi cộng sai số tay** ±`AimErr` đơn vị; vuốt `Speed` đơn vị/tick, nghỉ `Think`
tick giữa hai nét; mọi nét đi qua cùng bộ ghi như client thật. 40 seed mỗi mức:

| Mức | Phản xạ | Tốc vuốt | Nghỉ | Sai số | Qua |
|---|---|---|---|---|---|
| nhanh | 320 ms | 14 đv/tick | 240 ms | ±4 | 40/40 (100%) |
| vừa | 480 ms | 10 đv/tick | 400 ms | ±5 | 36/40 (90%) |
| chậm | 720 ms | 7 đv/tick | 640 ms | ±7 | 33/40 (82,5%) |

Mọi lần trượt đều là `lanterns_out` (sai số tay khi đèn bay sát nút), không lần nào thiếu điểm. Người thật đọc sân
kém hơn bot — mục tiêu beta: ~80–90% người "khá" qua trong 3 lần. ⚠ hiệu chỉnh ở beta.

## Nhịp người (chống tool hỗ trợ)

Không chặn được bot hoàn toàn (thế cục tất định từ seed, client có toàn bộ luật). Các luật sau vừa là **luật chơi**
(client áp dụng khi ghi, nên người thật không bao giờ vi phạm) vừa là **kiểm tra phía máy chủ**:
- **Bút theo tay có giới hạn**: mỗi tick bút đi tối đa `max_step` (48 đơn vị ≈ cả chiều ngang sân trong 100 ms).
  Tay nhanh hơn thì nét "trễ" theo. Log có bước dài hơn → `rejected / implausible_input` (**dịch chuyển tức thời**).
- **Cửa sổ trượt** `input_window_ticks` (1 s): tổng quãng bút ≤ `max_window_travel` (720 đơn vị = 6 lần ngang sân)
  và ≤ `max_strokes_per_window` (8) lần bắt đầu nét. Vượt → `implausible_input` (**tốc độ phi nhân**).
- **Nhấc tay**: giữa hai nét ít nhất `min_lift_ticks` (80 ms) không nhấn.
- **Thời gian qua tối thiểu**: `min_pass_ticks` (60 s), cộng kiểm tra "quá nhanh" theo giờ máy chủ (≥ 90% thời gian
  mô phỏng) → một lần qua tốn ít nhất ~54 s thật.
- **Biết trước seed**: seed được phát lúc bắt đầu, phiên mở lại giữ seed, nên có thể tập trước thế cục trong thời hạn
  phiên. Thời hạn ngắn (`expire_after_seconds` 300 s → thấy seed tối đa ~3 lần, chấp nhận). Gửi muộn hơn thời gian
  ván + 60 s được **đánh dấu** (`slowSubmit`, `elapsedMillis`) để soát, không bị từ chối.
- Số liệu nhịp (số điểm lấy mẫu, số nét, bước dài nhất, quãng/nét dày nhất trong cửa sổ, đèn chém, nút sót, liên đoạn
  cao nhất) lưu trong `trial_runs.result` để soát sau; client chỉ nhận chi tiết khi ván là qua/thua thật.

## Tất định & kiểm chứng (server authoritative)

- PRNG **xorshift32** (13/17/5), seed 32 bit do máy chủ phát. Một dòng duy nhất; mỗi vật rút theo đúng thứ tự: loại,
  x, vy, vx, hướng. `intN(n) = next() mod n`.
- Chỉ số nguyên; Go (`internal/trial/hoapham`) và TS (`lib/minigames/hoa-pham/sim.ts`) giống hệt nhau. Mọi tích trong
  phép thử va chạm < 2^53 nhờ giới hạn của loader (sân ≤ 200, bán kính ≤ 20, `max_step` ≤ 64), nên JS tính chính xác.
- Băm trạng thái: FNV-1a 32 bit (mỗi số 4 byte little-endian) trên tick, over, won, score, cuts, escaped, lanternsCut,
  lanternsLeft, charge, calmLeft, spawnCounter, nextId, spawned, pressed, px, py, strokeId, strokeCuts, bestCombo,
  trạng thái PRNG, số vật, rồi mỗi vật: id, loại, x, y, vx, vy, hp, nét chạm gần nhất.
- Vector vàng chung: `packages/contracts/golden/hoa-pham-v1.json` (params + 5 ván: bot nhanh, bot chậm, bỏ giữa
  chừng, đứng yên tới hết giờ, quét zig-zag tới khi tắt hết đèn) — Go test và vitest đều phải ra đúng; thiếu file là
  test lỗi. Đổi luật = tăng `rules_version` + sinh lại vector.
- **Log nét** (`strokes`): mỗi lần nhấn là một mảng `[tick bắt đầu, x0, y0, dx1, dy1, dx2, dy2, …]` — một điểm mỗi
  tick, mã hóa **delta**, tick không nhấn không tốn byte. Nét theo thứ tự, không chồng/không liền nhau, mọi điểm trong
  sân, nét kết thúc trước `endTick`, tối đa `max_strokes` nét; giới hạn thân request `trial.MaxClaimBytes` (96 KiB),
  loader kiểm `time_limit_ticks × 9 B + max_strokes × 24 B` vừa.
- Client gửi `{endTick, lines (= số nút đứt), score, stateHash, inputs: [], strokes}`. Máy chủ phán: hết hạn → log
  hỏng (`invalid_log`) → số nút/điểm bất khả → **quá nhanh** → **nhịp phi nhân** (`implausible_input`) → **không
  khớp** → kết quả thật của ván.

## Ghi chú thiết kế

- Khung chung với Tâm Ma: ấn chứng 7 ngày, giữ qua đột phá thất bại, dùng khi thành công; chỉ mở ở tầng cuối Nguyên
  Anh (`opens_at: last_stage`); phiên mở lại giữ seed; phiên bỏ dở tính thua; mọi phán quyết không qua → chờ 30 phút.
- Phần thưởng duy nhất ngoài cửa đột phá: điểm duyên `trial_pass` (2 điểm, trong trần của vip.yaml).
- Luyện tập: seed ngẫu nhiên phía client, không gửi máy chủ, ghi rõ "không tính".
- Tiếp cận: chế độ `prefers-reduced-motion` bỏ mảnh vỡ bay, vật không xoay, không nội suy chuyển động giữa hai
  tick, vệt bút ngắn hơn, chớp đỏ nhẹ hơn. Chưa có phương án cho
  người không vuốt/kéo được (câu hỏi mở — xem decisions).
- Âm thanh: chưa có. Khi thêm phải là âm thanh nguyên bản.
