# Vượt Tâm Ma — luật chơi (rules v2)

Thử luyện tâm cảnh chắn bước đột phá **Kim Đan → Nguyên Anh** (Vu đạo: bậc 3 → 4) — cot-truyen §5.4, decisions D6,
lộ trình Phase 3.2. Số liệu nằm ở `content/catalog/trials.yaml` (`tam_ma`); tài liệu này mô tả luật, không lặp số.
Số trong ngoặc là giá trị hiện tại, **chỉ để tham khảo, sẽ hiệu chỉnh ở closed beta**.

## Ý tưởng

Tạp niệm hóa thành những vệt mực rơi xuống thức hải. Người chơi xoay, dời và hạ chúng xuống cho kín hàng; mỗi hàng
kín là một tạp niệm được tiêu trừ. Định kỳ, tâm ma đẩy một hàng "khói" tạp niệm từ đáy lên. Khi đã tiêu trừ đủ, tâm
có thể **tĩnh**: mọi thứ ngừng lại một lúc.

Thể loại "xếp khối rơi" là thể loại chung; Vượt Tâm Ma có bộ khối, bàn, nhịp, cơ chế và hình ảnh riêng — không dùng
tên, nhạc, bộ 7 khối 4 ô, màu hay giao diện của bất kỳ game có thật nào.

## Bàn và khối

- Bàn **9 cột × 16 hàng** (`width`, `height`).
- Sáu "tạp niệm", chỉ có khối **3 ô hoặc 5 ô** (loader từ chối khối 4 ô):

| id | Tên | Ô | Hình | Trọng số |
|---|---|---|---|---|
| `vong` | Vọng Niệm | 3 | thẳng | 3 |
| `chap` | Chấp Niệm | 3 | gấp | 3 |
| `tham` | Tham | 5 | móc lớn (chữ V 3×3) | 1 |
| `san` | Sân | 5 | khối lửa (2×2 + 1) | 1 |
| `si` | Si | 5 | chữ đinh dài | 1 |
| `man` | Mạn | 5 | thanh ngạo 5 ô | 1 |

- Thứ tự khối: **túi có trọng số** — mỗi túi chứa mỗi khối đúng `weight` lần, xáo Fisher–Yates bằng PRNG; hết túi thì
  xáo túi mới. Luôn thấy trước 1 khối ("Kế tiếp").
- Khối mới xuất hiện ở hàng trên cùng, căn giữa (`(width − rộng) / 2`, chia nguyên). Nếu chỗ đó đã có mực → **thua**
  (`topped_out`).

## Điều khiển (mã trong log input)

| Mã | Hành động | Phím | Cảm ứng |
|---|---|---|---|
| 1 / 2 | sang trái / phải | ← → (A D) | ◀ ▶ (giữ để lặp) |
| 3 / 4 | xoay thuận / ngược | ↑ X W / Z | ↻ |
| 5 | rơi nhanh 1 hàng (chạm đáy thì khóa*) | ↓ S | ▼ (giữ để lặp) |
| 6 | hạ khối (rơi hết rồi khóa*) | Space | ⤓ |
| 7 | tĩnh tâm | C / Shift | nút "Tĩnh tâm" |

\* Nhịp người (v2): hạ khối, hay rơi nhanh mà sẽ khóa, **không có tác dụng** nếu chưa qua `min_lock_gap_ticks`
(6 tick = 300 ms) kể từ lần khóa trước — đây là luật chơi, giống hệt ở client và máy chủ.

Xoay: hình xoay 90° quanh khung bao, giữ tâm khung (lệch `(rộng cũ − rộng mới)/2`, chia nguyên về 0), rồi thử lần
lượt các độ dời (0,0) (−1,0) (+1,0) (0,−1) (−2,0) (+2,0); không vừa chỗ nào thì không xoay.
Tạm dừng: P / Esc, tự dừng khi rời tab. Tạm dừng **không** dừng giờ của phiên phía máy chủ.

## Nhịp (mọi thứ tính bằng tick, `tick_ms` = 50 ms)

Mỗi tick, theo đúng thứ tự:
1. Áp các input của tick đó theo thứ tự gửi.
2. Nếu đang tĩnh tâm: giảm bộ đếm tĩnh tâm. Nếu không:
   - **Trọng lực**: cứ `max(min_ticks, start_ticks − tick / step_every_ticks)` tick khối rơi 1 hàng (0,7 s/hàng lúc
     đầu, nhanh thêm 1 tick mỗi 20 s, tối đa 0,25 s/hàng). Không rơi được thì khóa.
   - **Tạp niệm dâng**: cứ `rise_every_ticks` tick (20 s) cả bàn bị đẩy lên 1 hàng, đáy thêm một hàng khói có đúng
     một lỗ (vị trí lỗ từ dòng PRNG thứ hai). Có mực ở hàng trên cùng lúc đẩy → thua. Khối đang rơi bị đè thì được
     đẩy lên 1 hàng; vẫn vướng → thua.
3. Tăng tick. Nếu đã trừ đủ `pass_lines` **và** tick ≥ `min_pass_ticks` → qua (`cleared`). Hết `time_limit_ticks`
   (3 phút) mà chưa qua → thua (`time_up`).

Khi khóa: in khối vào bàn, xóa mọi hàng kín (cả hàng khói), cộng `line_scores[số hàng]` vào điểm và
`charge_per_line × số hàng` vào thanh tĩnh tâm (tối đa `charge_max`).

## Tĩnh tâm

Thanh đầy (`charge_max` = 4 hàng) thì có thể tĩnh tâm: trong `duration_ticks` (8 s) khối **không tự rơi** và tạp niệm
**không dâng**; người chơi vẫn dời/xoay/hạ khối bình thường. Dùng xong thanh về 0.

## Điều kiện qua

Tiêu trừ **`pass_lines` (12) hàng** và **giữ tâm tới `min_pass_ticks` (75 s)** mà thức hải không bị lấp đầy →
`pass` (`cleared`). Trừ đủ sớm thì phải tiếp tục chống đỡ tới mốc đó (giao diện hiện "Giữ tâm thêm Ns"); một lần qua
vì thế luôn tốn thời gian thật. Thua (`topped_out`, `time_up`, `gave_up`) không mất tu vi; chỉ phải chờ
`lose_cooldown_seconds` (30 phút).

Độ khó đo bằng người chơi mẫu (`internal/trial/tamma/tammatest`, chỉ dùng trong test): đọc khối đang rơi **và khối kế
tiếp** (nhìn trước 2 khối) với hàm đánh giá bàn kinh điển, nghĩ `3 × gap` tick khi khối mới xuất hiện rồi mỗi
`gap + 1` tick một thao tác. 40 seed mỗi nhịp, rules v2:

| Nhịp | Qua | Qua trung bình lúc |
|---|---|---|
| 5 thao tác/s | 31/40 (78%) | 75 s |
| 3,3 thao tác/s | 35/40 (88%) | 75 s |
| 2,2 thao tác/s | 36/40 (90%) | 78 s |

Người thật đọc bàn kém hơn bot — mục tiêu beta: 70–85% người qua trong 3 lần.

## Nhịp người (v2, chống tool hỗ trợ)

Không chặn được bot hoàn toàn (thế cục tất định từ seed, client có toàn bộ luật); các luật sau chỉ buộc bot phải
chơi **với tốc độ người**, nên gian lận không nhanh hơn chơi thật:
- **Trần thao tác**: trong bất kỳ cửa sổ `input_window_ticks` (20 tick = 1 s) nào, tối đa `max_inputs_per_window` (16)
  thao tác. Log vượt trần → `rejected / implausible_input`. Client tự điều tiết (hàng chờ 3 thao tác, thừa thì bỏ),
  nên người chơi thật không bao giờ vượt.
- **Khoảng cách khóa**: `min_lock_gap_ticks` (300 ms) như ở bảng điều khiển.
- **Thời gian qua tối thiểu**: `min_pass_ticks` (75 s), cộng với kiểm tra "quá nhanh" theo giờ máy chủ (≥ 90% thời
  gian mô phỏng) → một lần qua tốn ít nhất ~68 s thật.
- Số liệu nhịp (số thao tác, thao tác/giây, cửa sổ dày nhất, khoảng khóa nhỏ nhất) được lưu trong
  `trial_runs.result` để soát sau.

## Tất định & kiểm chứng (server authoritative)

- PRNG: **xorshift32** (13/17/5), seed = số 32 bit máy chủ phát (≠ 0). Dòng khối: `seed`; dòng tạp niệm dâng:
  `seed XOR 0xA5A5A5A5`. `intN(n) = next() mod n`.
- Chỉ dùng số nguyên; Go (`internal/trial/tamma`) và TS (`lib/minigames/tam-ma/sim.ts`) là hai bản giống hệt nhau.
- Băm trạng thái: FNV-1a 32 bit trên các số (mỗi số 4 byte little-endian): tick, over, won, lines, score, charge,
  calmLeft, gravCounter, riseCounter, cur, rot, px, py, next, bagPos, len(bag), bag…, 2 trạng thái PRNG, số khối đã
  khóa, tick khóa gần nhất, khoảng khóa nhỏ nhất, rồi toàn bộ ô bàn.
- Vector vàng chung: `packages/contracts/golden/tam-ma-v2.json` (params + 5 ván) — Go test và vitest đều phải ra
  đúng từng hash; thiếu file là test lỗi. Đổi luật = tăng `rules_version` + sinh lại vector. Vector v1 đã bỏ: trước
  ra mắt không có phiên v1 nào còn dở (máy chủ chỉ chấp nhận `rules_version: 2`).
- Client gửi `{endTick, lines, score, stateHash, inputs: [[tick, mã], …]}`. Máy chủ phát lại từ seed + bản chụp luật
  của phiên và phán: hết hạn → log hỏng → số hàng/điểm bất khả → **quá nhanh** (giờ máy chủ < `endTick × tick_ms ×
  min_elapsed_bp`) → **nhịp phi nhân** (`implausible_input`) → **không khớp** (kết quả phát lại ≠ điều client khai)
  → kết quả thật của ván. Client chỉ nhận chi tiết phát lại khi ván là qua/thua thật.

## Ghi chú thiết kế

- Ấn chứng "qua" có hạn `pass_valid_seconds` (7 ngày), **giữ qua các lần đột phá thất bại**, dùng hết khi đột phá
  thành công. Lý do và các lựa chọn khác: docs/decisions.md 2026-11-07 "Mini game: khung thử luyện + Tâm Ma".
- Chỉ mở khi đứng ở tầng cuối Kim Đan (`opens_at: last_stage`): tâm ma hiện ra trước bình cảnh. Ai cũng có thể
  **luyện tập** (seed ngẫu nhiên phía client, không gửi máy chủ, không tính).
- Phần thưởng duy nhất ngoài cửa đột phá: điểm duyên `trial_pass` (2 điểm, tối đa 2 lần/ngày, trong trần 20/ngày của
  vip.yaml) — không tu vi, không linh thạch, không ảnh hưởng tỉ lệ đột phá.
- Âm thanh: chưa có. Khi thêm phải là âm thanh nguyên bản.
