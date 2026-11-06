# Cân bằng sơ bộ: tiến trình thế giới chung (Giai đoạn 0)

Cập nhật 06/11/2026 · agent `game-balancer` · **chỉ phân tích + đề xuất, chưa sửa catalogue/Go.**

> Trạng thái: chủ dự án đã chấp nhận mọi đề xuất §0–§10 (`decisions.md`, mục "Phase 0: số liệu & mô hình dữ liệu
> thế giới chung"). §11 là vòng đề xuất tiếp theo, chưa chốt.

Đầu vào ràng buộc: [decisions.md](../decisions.md) (hai mục 06/11/2026 "Tiên Duyên…" và "Cốt truyện thế giới chung"),
[cot-truyen.md](cot-truyen.md) §5.2, §6, §9.1, [ke-hoach-trien-khai-hong-hoang.md](ke-hoach-trien-khai-hong-hoang.md) §4.1, §5.
Mọi giá trị ghi **"đề xuất"** là số để chủ dự án chốt rồi mới đưa vào YAML.

---

## 0. Tóm tắt

| Câu hỏi | Kết quả chính |
|---|---|
| Lôi kiếp làm chậm bao nhiêu? | 10 lần kiếp (Hóa Thần → Chuẩn Thánh), kỳ vọng **5,0 lần trượt**, chỉ **+4 ngày** với người 100/ngày (Chuẩn Thánh ngày 335 thay vì 330), +7 đến +13 ngày với người đăng nhập 1 lần/ngày. Mất trung bình ~340 điểm tổng cộng. |
| Ngưỡng điều chỉnh | Đề xuất bảng mới (Chuẩn Thánh **32.450** điểm) để người 100/ngày đạt Chuẩn Thánh **ngày 329** (P10–P90: 326–333), thời gian mỗi cảnh giới **tăng dần nghiêm ngặt** (bảng §6.4 cũ có ba cảnh giới liền nhau cùng 20 ngày). |
| Tốc độ treo máy | Đề xuất **50 điểm / 24h thực** (2,083/h), cửa sổ 8h. Treo 3 lần/ngày mới được 50. Treo 1 lần/ngày chỉ được 16,7 (25 nếu dùng Tụ Linh Phù). Người chỉ treo máy tới Chuẩn Thánh mất **655 ngày** (3 lần/ngày) đến **1.317 ngày** (1 lần/ngày). |
| Mốc cốt truyện | Với tổ hợp người chơi thực tế (TB **66,5 điểm/người/ngày**, không phải 100), **cơ chế kẹp 70%/150% quyết định mọi mốc** ở 1.000, 5.000, 10.000 và 30.000 người. Công thức chỉ thực sự có tác dụng khi server có khoảng 10.000–21.500 người hoạt động. Đề xuất đổi sang **ngưỡng theo đầu người**. |
| Tiên Duyên | Nếu chỉ có trần 150, linh thạch từ quà ngày có thể cho TD12 hơn người miễn phí tới **2,6 lần** và **phá luật "TD12 đăng nhập 1 lần ≤ miễn phí đăng nhập 3 lần"**. Cần **giới hạn tiêu thụ đan tăng tốc mỗi ngày** (đề xuất +5 điểm/viên, 125 linh thạch, tối đa 2 viên/ngày): luật được giữ ở mọi kiểu chơi, chênh lệch do linh thạch ≤ 6 điểm/ngày. |
| Người vào muộn ×2 | Người chơi chuẩn vào ngày 60/120/200 cần **46/94/154 ngày** để bắt kịp trung vị server, vì trần 150 khóa tốc độ bắt kịp. Người 1 lần/ngày gần như không bắt kịp. |
| Tầng | Luyện Khí 9 tầng, các cảnh giới sau dùng **4 kỳ × 1–4 trọng** (4/8/12/16 tầng). Người 100/ngày lên tầng mỗi **1,6–2,9 ngày**. |

---

## 1. Mô hình và giả định

Script mô phỏng (Python, Monte Carlo, ngoài repo):
`/tmp/claude-1000/-home-feng-Desktop-tu-tien-online/d2aa8022-3a2d-43ac-957a-98d5d26710e4/scratchpad/balance/`
(`model.py`, `q1.py`, `calibrate.py`, `q24.py`, `q35.py`, `q5.py`, `q6.py`). Mỗi kiểu người chơi chạy 300–1.000 lượt.

**Thang điểm.** Mọi nguồn tu vi đều quy về "điểm". Ngưỡng là điểm tích lũy để vào cảnh giới (§6.4). Tốc độ
**không** đổi theo cảnh giới (khác `realms.yaml` hiện tại, nơi `base_rate_milli` tăng theo cảnh giới).

**Kiểu người chơi (Tiên Duyên 0, có Tụ Linh Phù từ điểm danh):**

| Mã | Mô tả | Treo | Giảng đạo | Đan (shop đề xuất) | Cộng đồng | Tổng/ngày |
|---|---|---|---|---|---|---|
| I1 / I2 / I3 | chỉ treo máy, 1/2/3 lần mỗi ngày | 25 / 41,7 / 50 | 0 | không mua | 0 | 25 / 41,7 / 50 |
| A1 / A2 / A3 | treo + giảng đạo + nhiệm vụ | như trên | 10 / 20 / 20 | 10 (A1: 10) | 0 | 45 / 71,7 / 80 |
| S1 / S2 / S3 | A + cộng đồng | như trên | như trên | 10 | 10 / 20 / 20 | 55 / 91,7 / **100** |
| G | cày trần | | | | | **150** |

- 30 điểm "giảng đạo + nhiệm vụ" = 2 buổi giảng × 10 điểm + 10 điểm từ đan tăng tốc mua bằng linh thạch nhiệm vụ.
  Người 1 lần/ngày chỉ kịp 1 buổi giảng.
- Linh thạch/ngày: điểm danh 100, vận công 150 (cần ≥ 3 lần thu), nhiệm vụ giảng đạo 200 (thay `trung_kich`,
  xem §5.3), cộng quà Tiên Duyên theo `vip.yaml`.
- Đột phá tầng nhỏ: thành công 100% (đề xuất). Đột phá lớn trước lôi kiếp (Luyện Khí → … → Hóa Thần): giữ tỉ lệ,
  pity và cooldown của `realms.yaml` hiện tại. Mini game tâm ma và hóa phàm coi như qua ngay lần đầu.
- Lôi kiếp (từ lần đột phá rời Hóa Thần): 80%, −3% mỗi cảnh giới (lên Chuẩn Thánh còn 53%, chưa chạm sàn 50%).
  Trượt thì mất **25% tu vi đang tích trong tầng hiện tại** (tầng cuối của cảnh giới), cooldown 24h.
  Trượt 3 lần liên tiếp thì lần thứ 4 chắc chắn qua.
- Bình cảnh 120%: điểm vượt 120% yêu cầu của tầng bị mất (giống `overflow_cap_bp` hiện tại).
- Đột phá chỉ xảy ra khi đang online (lúc đăng nhập). Các lần đăng nhập cách đều nhau.

**Giới hạn của mô hình:**
- Điểm/ngày là kỳ vọng cố định, chưa có biến động hằng ngày, nghỉ chơi hay rời game.
- **Chưa mô hình hóa các yếu tố tăng tỉ lệ kiếp** (dư điểm, pháp bảo, đan hộ thân, hộ pháp). Chi phí lôi kiếp ở đây
  là **cận trên**.
- Chưa có song tu, bí cảnh, sự kiện. Kiểu G gộp tất cả thành 150.
- Tổ hợp người chơi ở §6, §7 là **giả định**, cần đo lại ở closed beta.
- Không có rủi ro tràn số: tối đa 32.450.000 milli-điểm.

---

## 2. Cách hiểu tốc độ treo máy (câu 2)

"50 điểm, tích lũy tối đa 8 giờ, phải quay lại thu" có ba cách hiểu:

| Phương án | Cấp TD (cửa sổ) | 1 lần/ngày | 1 lần + phù | 2 lần | 3 lần |
|---|---|---|---|---|---|
| **A: 50/24h (2,083/h)** | 0 (8h) | 16,7 | 25,0 | 41,7 | 50 |
| | 4 (16h) | 33,3 | 41,7 | 50 | 50 |
| | 8–12 (24–32h) | 50 | 50 | 50 | 50 |
| B: 50/16h (3,125/h), trần treo 50/ngày | 0 (8h) | 25,0 | 37,5 | 50 | 50 |
| | ≥ 4 (≥ 16h) | 50 | 50 | 50 | 50 |
| C: 50/8h (6,25/h), không trần riêng | 0 (8h) | 50 | 75 | 125 | 150 |
| | ≥ 8 | 150 | 150 | 150 | 150 |

- **C bị loại.** Chỉ treo máy đã chạm trần 150 và phá mức trung bình 100.
- **A (đề xuất)** khớp trần 24h/ngày hiện có (`daily_cultivation_ceiling_hours: 24`). Mức tối đa từ treo là 50 với mọi người.
  TD12 đăng nhập 1 lần **bằng đúng** người miễn phí đăng nhập 3 lần. Đây là trường hợp sát nhất của luật công bằng.
  Đúng: **cần 3 lần đăng nhập** (cách nhau ≤ 8h) mới đủ 50.
- B nhẹ tay hơn với người chơi ít đăng nhập (2 lần/ngày là đủ, TD4 trở lên chỉ cần 1 lần), nhưng giá trị của cửa sổ
  Tiên Duyên giảm gần hết từ cấp 4. Phương án này cần thêm một trần treo riêng.

**Ảnh hưởng của cửa sổ Tiên Duyên dài hơn (phương án A, ngưỡng đề xuất).** Ô ghi: điểm/ngày · ngày tới Chuẩn Thánh.

| Kiểu | TD0 | TD4 | TD8 | TD12 |
|---|---|---|---|---|
| I1 | 25 · 1.317 | 41,7 · 791 | 50 · 663 | 50 · 663 |
| I2 | 41,7 · 788 | 50 · 658 | 50 · 658 | 50 · 658 |
| I3 | 50 · 655 | 50 · 655 | 50 · 655 | 50 · 655 |
| A1 | 45 · 733 | 61,7 · 547 | 70 · 487 | 70 · 487 |
| A2 | 71,7 · 459 | 80 · 412 | 80 · 412 | 80 · 412 |
| S1 | 55 · 605 | 71,7 · 475 | 80 · 431 | 80 · 431 |
| S3 | 100 · 329 | 100 · 329 | 100 · 329 | 100 · 329 |

Cửa sổ dài chỉ có lợi cho người đăng nhập 1–2 lần/ngày. Từ cấp 8 không còn lợi thêm, trừ khi bỏ cả ngày.
Ở 1 lần/ngày, chênh lệch là 25 → 50 điểm treo (×2,0). Tính cả hoạt động khác thì A1 45 → 70 (×1,56).
Với người đăng nhập 3 lần/ngày, Tiên Duyên không đổi gì.

> Đơn vị lưu đề xuất: `cultivation_milli` (1 điểm = 1.000) và `idle_rate_milli_per_hour: 2083`
> (49,99 điểm/ngày), giữ phần dư như `RemainderMilli` hiện tại.

---

## 3. Ngày đạt từng cảnh giới, gồm cả lôi kiếp (câu 1)

### 3.1. Với bảng §6.4 hiện tại (trung vị, đã gồm lần trượt kiếp)

| Kiểu | /ngày | TC | KĐ | NA | HT | LH | HTh | ĐT | ĐK | NhT | ĐTi | KT | TA | ĐL | **CT** | Kiếp thêm | Điểm mất |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| I1 | 25 | 18 | 51 | 99 | 163 | 243 | 324 | 405 | 506 | 607 | 729 | 871 | 1013 | 1174 | **1336** | +13,0 | 332 |
| I2 | 41,7 | 10 | 30 | 59 | 98 | 146 | 194 | 242 | 303 | 364 | 437 | 522 | 607 | 704 | **801** | +8,0 | 331 |
| I3 | 50 | 9 | 25 | 49 | 81 | 121 | 162 | 202 | 252 | 303 | 364 | 435 | 506 | 587 | **668** | +6,7 | 327 |
| A1 | 45 | 9 | 28 | 55 | 91 | 136 | 181 | 226 | 282 | 338 | 406 | 485 | 563 | 653 | **743** | +8,0 | 344 |
| A2 | 71,7 | 8 | 20 | 37 | 60 | 88 | 116 | 144 | 179 | 214 | 257 | 306 | 356 | 412 | **468** | +4,5 | 342 |
| A3 | 80 | 6 | 16 | 31 | 51 | 76 | 102 | 127 | 159 | 190 | 228 | 273 | 317 | 368 | **418** | +4,3 | 339 |
| S1 | 55 | 9 | 25 | 48 | 78 | 114 | 151 | 188 | 236 | 284 | 339 | 404 | 468 | 542 | **616** | +7,0 | 348 |
| S2 | 91,7 | 4 | 14 | 28 | 45 | 67 | 89 | 112 | 139 | 166 | 200 | 239 | 278 | 322 | **366** | +4,5 | 353 |
| **S3** | **100** | 5 | 13 | 25 | 41 | 61 | 81 | 102 | 127 | 152 | 183 | 218 | 254 | 294 | **335** | **+4,0** | 343 |
| G | 150 | 3 | 8 | 17 | 27 | 41 | 54 | 68 | 85 | 102 | 123 | 147 | 170 | 198 | **225** | +4,0 | 350 |

"Kiếp thêm" là số ngày chênh so với cùng mô phỏng tắt lôi kiếp.

**Vì sao lôi kiếp rẻ:**
- Kỳ vọng số lần trượt mỗi kiếp là q + q² + q³, với q = 1 − p.
  Con số này đi từ 0,25 (kiếp đầu, 80%) đến 0,80 (lên Chuẩn Thánh, 53%). Tổng là **5,0 lần trượt**, P90 là 8 lần, tối đa 14 lần.
- Với người 100/ngày, mỗi lần trượt tốn khoảng 1 ngày (cooldown 24h). Phần 25% bị mất chỉ khoảng 60 điểm, nhỏ hơn
  lượng điểm kiếm được trong lúc chờ cooldown, và bình cảnh 120% giữ phần dư lại.
- Người chơi chậm mất nhiều ngày hơn (I1: +13 ngày) vì 25% một tầng tương ứng nhiều ngày treo máy hơn.
- **Nếu hiểu "25% tu vi tích lũy trong cảnh giới"** (cả cảnh giới, không phải tầng): người 100/ngày mất thêm khoảng
  **+39 ngày**. Cách hiểu này cũng mâu thuẫn với "không tụt tầng", vì mỗi tầng lưu điểm riêng. Đề xuất chốt:
  **25% điểm đang tích trong tầng hiện tại**.

### 3.2. Ngưỡng đề xuất (Chuẩn Thánh ≈ ngày 330 cho người 100/ngày)

Ngày mục tiêu cho S3 (thời gian mỗi cảnh giới tăng dần: 4, 7, 11, 15, 19, 21, 23, 25, 27, 29, 32, 35, 39, 43 ngày).
Hiệu chỉnh lặp trên mô phỏng có lôi kiếp:

| Cảnh giới | §6.4 cũ | **Đề xuất** | Ngày S3 (cũ → mới) | Ghép mốc cốt truyện |
|---|---|---|---|---|
| Luyện Khí | 0 | **0** | 0 | Tử Tiêu Cung |
| Trúc Cơ | 400 | **300** | 5 → 3 | |
| Kim Đan | 1.200 | **950** | 13 → 10 | |
| Nguyên Anh | 2.400 | **2.050** | 25 → 21 | Giảng đạo 2 (30) |
| Hóa Thần | 4.000 | **3.500** | 41 → 36 | |
| Luyện Hư | 6.000 | **5.400** | 61 → 55 | Giảng đạo 3 (60) |
| Hợp Thể | 8.000 | **7.500** | 81 → 76 | |
| Đại Thừa | 10.000 | **9.750** | 102 → 99 | Đại chiến Vu Yêu (100) |
| Độ Kiếp | 12.500 | **12.250** | 127 → 124 | Nữ Oa (130) |
| Nhân Tiên | 15.000 | **14.900** | 152 → 151 | Thánh nhân / Tam Hoàng |
| Địa Tiên | 18.000 | **17.750** | 183 → 180 | |
| Kim Tiên | 21.500 | **20.900** | 218 → 212 | |
| Thái Ất Kim Tiên | 25.000 | **24.350** | 254 → 247 | Phong Thần (240) |
| Đại La Kim Tiên | 29.000 | **28.200** | 294 → 286 | |
| Chuẩn Thánh | 33.000 | **32.450** | 335 → **329** | Tây Du (330) |

Với ngưỡng đề xuất, ngày đạt Chuẩn Thánh của các kiểu khác: I1 1.318 · I2 789 · I3 656 · A1 733 · A2 459 · A3 411 ·
S1 605 · S2 361 · **G 223** (P10–P90: 220–226).

**Hạn chót §4.1 cập nhật** (người cày trần G, trung vị): tâm ma (lên Nguyên Anh) **ngày 15** (cũ 16) · hóa phàm
(lên Hóa Thần) **ngày 25** (cũ 27) · lôi kiếp đầu tiên (lên Luyện Hư) **ngày 38** (cũ 40) · Độ Kiếp ngày 85 ·
Chuẩn Thánh ngày 223. Hạn chót sớm hơn 1–2 ngày.

> Ghi chú: một nửa người chơi (I, A1, S1) **không** đạt Chuẩn Thánh trong ~1 năm. Kế hoạch §5 đã nêu điều này
> (khoảng 660 ngày cho người chỉ treo máy). Thực tế người treo 1 lần/ngày còn chậm hơn, khoảng **1.300 ngày**.
> Chủ dự án cần xác nhận đây là chủ đích.

---

## 4. Số tầng mỗi cảnh giới (câu 6)

Quy tắc đề xuất: Luyện Khí giữ 9 tầng. Các cảnh giới sau dùng **4 kỳ** (Sơ, Trung, Hậu, Viên Mãn) × **k trọng**,
với k = ⌈số ngày của cảnh giới / 12⌉. Như vậy mỗi tầng ≤ 3 ngày ở mức 100/ngày. Điểm chia đều giữa các tầng.

| Cảnh giới | Điểm cả cảnh giới | Tầng | Điểm/tầng | Ngày/tầng @100 | @72 | @45 | @25 | @150 | Mất khi trượt kiếp |
|---|---|---|---|---|---|---|---|---|---|
| Luyện Khí | 300 | 9 | 33 | 0,33 | 0,5 | 0,7 | 1,3 | 0,22 | — |
| Trúc Cơ | 650 | 4 | 162 | 1,62 | 2,3 | 3,6 | 6,5 | 1,08 | — |
| Kim Đan | 1.100 | 4 | 275 | 2,75 | 3,8 | 6,1 | 11,0 | 1,83 | — |
| Nguyên Anh | 1.450 | 8 | 181 | 1,81 | 2,5 | 4,0 | 7,2 | 1,21 | — |
| Hóa Thần | 1.900 | 8 | 238 | 2,38 | 3,3 | 5,3 | 9,5 | 1,58 | 59 (0,6 ngày) |
| Luyện Hư | 2.100 | 8 | 262 | 2,62 | 3,7 | 5,8 | 10,5 | 1,75 | 66 |
| Hợp Thể | 2.250 | 8 | 281 | 2,81 | 3,9 | 6,2 | 11,2 | 1,88 | 70 |
| Đại Thừa | 2.500 | 12 | 208 | 2,08 | 2,9 | 4,6 | 8,3 | 1,39 | 52 |
| Độ Kiếp | 2.650 | 12 | 221 | 2,21 | 3,1 | 4,9 | 8,8 | 1,47 | 55 |
| Nhân Tiên | 2.850 | 12 | 238 | 2,38 | 3,3 | 5,3 | 9,5 | 1,58 | 59 |
| Địa Tiên | 3.150 | 12 | 262 | 2,62 | 3,7 | 5,8 | 10,5 | 1,75 | 66 |
| Kim Tiên | 3.450 | 12 | 288 | 2,88 | 4,0 | 6,4 | 11,5 | 1,92 | 72 |
| Thái Ất Kim Tiên | 3.850 | 16 | 241 | 2,41 | 3,4 | 5,3 | 9,6 | 1,60 | 60 |
| Đại La Kim Tiên | 4.250 | 16 | 266 | 2,66 | 3,7 | 5,9 | 10,6 | 1,77 | 66 |

Tên gợi ý: "Đại Thừa Trung Kỳ Nhị Trọng". Độ Kiếp có thể dùng "Đệ N Trọng Thiên Kiếp".
Tổng cộng 149 tầng. Người 1 lần/ngày (A1) lên tầng mỗi 4–6 ngày. Người chỉ treo 1 lần/ngày (I1) mất 7–11 ngày mỗi tầng.

**Lưu ý về bình cảnh:** tầng Luyện Khí chỉ có 33 điểm. Người cày nhận khoảng 50 điểm mỗi lần đăng nhập, nên phần
vượt 120% bị mất. **Đề xuất:** bình cảnh 120% chỉ áp ở tầng cuối của cảnh giới (trước đột phá lớn hoặc lôi kiếp).
Tầng nhỏ được lên liên tiếp trong một lần thu.

---

## 5. Ngân sách công bằng Tiên Duyên (câu 4)

### 5.1. Nếu chỉ dựa vào trần 150 (shop không giới hạn số lần mỗi ngày)

Ô là điểm/ngày (phương án A, quà theo `vip.yaml`). Luật cần kiểm: TD12 đăng nhập 1 lần ≤ miễn phí đăng nhập 3 lần.

| Giá | Hoạt động | free@1 | TD12@1 | free@3 | TD12@3 | Luật | TD12/free @1 |
|---|---|---|---|---|---|---|---|
| 10 LT/điểm | Treo + shop | 35 | **90** | 75 | 105 | **✗** | 2,57× |
| | Tích cực | 65 | **120** | 115 | 145 | **✗** | 1,85× |
| | Chuẩn | 75 | 130 | 135 | 150 | ✓ | 1,73× |
| 25 LT/điểm | Treo + shop | 29 | **66** | 60 | 72 | **✗** | 2,28× |
| | Tích cực | 47 | 84 | 88 | 100 | ✓ | 1,79× |

Kết luận: **trần 150 không phải là hàng rào công bằng.** Mức tăng tối đa nhờ linh thạch của TD12 so với TD0 bằng
`min(quà / giá, 150 − điểm nền)`. Với 300 linh thạch/ngày, con số này là 12–30 điểm/ngày, cộng thêm phần chênh do
cửa sổ treo.

### 5.2. Ràng buộc đề xuất

Gọi B là số điểm tăng tốc tối đa được **dùng** mỗi ngày cho mỗi nhân vật, C là giá linh thạch cho mỗi điểm.

1. **B × C ≤ thu nhập linh thạch của người miễn phí chỉ treo 3 lần/ngày (250).** Khi đó người đăng nhập 3 lần
   luôn mua đủ trần, và quà Tiên Duyên không thêm tốc độ cho họ.
2. **B ≤ 10% của 100** (≤ 10 điểm/ngày). Đan tăng tốc là một phần trong "30 điểm giảng đạo + nhiệm vụ", không cộng thêm
   ngoài mức 100.
3. Giới hạn tính theo **lượt dùng mỗi nhân vật mỗi ngày**, không theo lượt mua, để không lách được qua trao đổi hay tặng đồ.
4. Linh thạch quà vượt mức B × C phải có chỗ tiêu **không tăng tiến độ** (ngoại hình, động phủ, tiện ích).

**Đề xuất:** Tụ Linh Đan +5 điểm, 125 linh thạch, tối đa 2 viên/ngày (B = 10, B × C = 250).

| Hoạt động | free@1 | TD12@1 | free@2 | TD12@2 | free@3 | TD12@3 | Luật | TD12/free @1 |
|---|---|---|---|---|---|---|---|---|
| Treo + shop | 29 | 60 | 45,7 | 60 | 60 | 60 | ✓ (=) | 2,07× |
| Tích cực | 45 | 70 | 71,7 | 80 | 80 | 80 | ✓ | 1,56× |
| Chuẩn | 55 | 80 | 91,7 | 100 | 100 | 100 | ✓ | 1,45× |

- Phần chênh do linh thạch ở 1 lần/ngày là 4 → 10 điểm (**≤ 6 điểm/ngày**). Phần còn lại là do cửa sổ treo, đúng như thiết kế.
- **Mục tiêu đề xuất ghi vào rule:** "TD12@1 ≤ free@3 với cùng kiểu hoạt động" **và** "phần tăng tốc từ quà Tiên Duyên
  ≤ 6 điểm/ngày".
- Bình luận ngân sách quà trong `vip.yaml` ("quà TD12 ≤ ~15% thu nhập linh thạch của người miễn phí tích cực")
  **đang bị vi phạm** (300/450 = 67%). Có hai hướng: viết lại bình luận theo ràng buộc B × C ở trên, hoặc giảm quà.
  Giữ nguyên số quà vẫn ổn khi có giới hạn 2 viên/ngày.

### 5.3. Lỗi phát hiện kèm theo
- Nhiệm vụ ngày `trung_kich` ("thử đột phá 1 lần") không tương thích với tầng 2–3 ngày. Người chơi không có lượt thử
  mỗi ngày, nên phần thưởng này hoặc không lấy được, hoặc khuyến khích thử thất bại. Đề xuất thay bằng "nghe giảng đạo 1 buổi".
- Luồng linh thạch vẫn **chưa có chỗ tiêu** cho tới khi shop lên. Shop phải ra cùng lúc hoặc trước nguồn bí cảnh.

---

## 6. Mốc cốt truyện (câu 3)

Tổ hợp giả định (DAU = người có đóng góp trong ngày): I1 15%, I2 10%, I3 5%, A1 15%, A2 15%, A3 10%, S1 5%,
S2 10%, S3 10%, G 5%, và 10% số người có Tiên Duyên ≥ 8. Kết quả: **trung bình 66,5 điểm/người/ngày**.
Ngưỡng theo §6.1 = ngày mục tiêu × 10.000 × 100.

**DAU không đổi** (ô ghi: ngày theo tổng điểm → ngày thực sau kẹp; * = bị kẹp):

| Mốc (mục tiêu) | 1.000 | 5.000 | 10.000 | 30.000 |
|---|---|---|---|---|
| Giảng đạo 2 (30) | 452 → **45*** | 91 → **45*** | 46 → **45*** | 16 → **21*** |
| Giảng đạo 3 (60) | >600 → **90*** | 181 → **90*** | 91 → **90*** | 31 → **42*** |
| Đại chiến Vu Yêu (100) | → **150*** | 301 → **150*** | 151 → **150*** | 51 → **70*** |
| Nữ Oa (130) | → **195*** | 391 → **195*** | 196 → **195*** | 66 → **91*** |
| Thánh nhân (140) | → **210*** | 422 → **210*** | 211 → **210*** | 71 → **98*** |
| Tam Hoàng (150) | → **225*** | 452 → **225*** | 226 → **225*** | 76 → **105*** |
| Phong Thần (240) | → **360*** | → **360*** | 361 → **360*** | 121 → **168*** |
| Thiên Đình (310) | → **465*** | → **465*** | 467 → **465*** | 156 → **217*** |
| Tây Du (330) | → **495*** | → **495*** | 497 → **495*** | 166 → **231*** |

Với DAU thực tế hơn (tăng 40% → 100% trong 14 ngày đầu, rồi giảm một nửa sau mỗi 240 ngày), kết quả giống hệt sau kẹp.
Riêng server 30.000 người có raw gần biên dưới hơn (Tây Du raw 221 → 231).

**Kết luận:**
- **Kẹp 70%/150% quyết định mọi mốc ở cả bốn quy mô.** Công thức chỉ có tác dụng khi DAU nằm trong khoảng
  1.000.000/(1,5 × 66,5) ≈ **10.000** đến 1.000.000/(0,7 × 66,5) ≈ **21.500**.
- Ngay cả server tham chiếu 10.000 người cũng chạy ở mức 150%, vì người chơi thực tế trung bình 66,5 điểm chứ không phải 100.
  Khi đó Tây Du rơi vào **ngày 495**, trong khi người 100/ngày đạt Chuẩn Thánh ở ngày 329.
- Server đông hơn ~21.500 người thì mốc dính sàn 70%, nhanh hơn tiến độ tu luyện của người chơi.
  Ví dụ Phong Thần ở ngày 168, khi trung vị mới ở Hóa Thần hoặc Luyện Hư.
- Hệ quả của hai điều trên: tổng điểm hầu như **không còn vai trò "cộng đồng đẩy cốt truyện"**.

**Đề xuất: ngưỡng theo đầu người.** Mỗi game day, worker cộng `tiến_độ += tổng đóng góp ngày / DAU ngày`.
Như vậy tiến độ thế giới bằng điểm tích lũy của một "người hoạt động trung bình".
Mốc đạt khi `tiến_độ ≥ T × a_ref` (a_ref = 65, hiệu chỉnh ở closed beta). Kẹp 70%/150% giữ lại làm lưới an toàn.

| Mốc | TB thực 45 | 55 | 65 | 80 | 100 |
|---|---|---|---|---|---|
| Giảng đạo 2 (30) | 44 | 36 | 30 | 25 | 21* |
| Đại chiến Vu Yêu (100) | 145 | 119 | 100 | 82 | 70* |
| Phong Thần (240) | 347 | 284 | 240 | 195 | 168* |
| Tây Du (330) | 477 | 390 | 330 | 269 | 231* |

- Không phụ thuộc quy mô server, không bị ảnh hưởng bởi tăng hay giảm DAU.
- Cốt truyện bám theo **vị trí cảnh giới của người chơi điển hình**, đúng ý đồ bảng §6.4.
- Chống bot tốt hơn: mỗi bot đóng góp tối đa 150, nên bot chỉ đẩy được trung bình lên, và sàn 70% chặn lại.
- Vẫn giữ cảm giác cộng đồng: thanh "Thiên Đạo vận chuyển" hiện % tới mốc và tổng đóng góp.
  Cộng đồng tích cực hơn thì mốc đến sớm hơn, cộng đồng ít hoạt động thì mốc đến muộn hơn.
- Ngưỡng tuyệt đối theo N cố định chỉ đúng nếu biết trước DAU, mà điều này không thể biết trước. **Không khuyến nghị** giữ.

---

## 7. Người vào muộn ×2 (câu 5)

Trung vị server (tổ hợp §6, ngưỡng đề xuất): ngày 60 khoảng 3.570 điểm (Hóa Thần), ngày 120 khoảng 7.300 điểm (Luyện Hư),
ngày 200 khoảng 12.300 điểm (Độ Kiếp). Trung vị tăng khoảng **62 điểm/ngày**. Hệ số ×2 áp trước trần 150 và tắt khi
đạt trung vị.

| Người vào muộn | /ngày → khi ×2 | Vào ngày 60 | Vào ngày 120 | Vào ngày 200 |
|---|---|---|---|---|
| S3 | 100 → 150 | **46** | **94** | **154** |
| S2 | 92 → 150 | 50 | 101 | 170 |
| A2 | 72 → 143 | 54 | 105 | 175 |
| A1 | 45 → 90 | 177 | 351 | 387 |
| I3 | 50 → 100 | 95 | 199 | 301 |
| I1 | 25 → 50 | không bao giờ | không bao giờ | không bao giờ |

So với trung bình (mean) thay vì trung vị, kết quả gần giống (S3: 50 / 101 / 150).

- Thời gian bắt kịp ≈ J × 62 / (150 − 62) ≈ **0,7 × J**, vì trần 150 khóa tốc độ. Hệ số ×3 cũng không giúp người
  chơi tích cực. Người vào ngày 200 chỉ bắt kịp vào khoảng ngày 354, sau cả Phong Thần.
- Người ít hoạt động (A1, I3) không thực sự được bù. ×2 chỉ giúp người vốn đã chơi nhiều.
- **Đề xuất (cần chốt):** cộng thêm **điểm khởi đầu một lần** khi tạo nhân vật sau ngày 14, bằng 50% trung vị server.
  Khoản này không tính vào đóng góp thế giới, không vào bảng xếp hạng ngày, và không vi phạm trần/ngày vì không phải
  thu nhập ngày. Ước tính với S3: bắt kịp trong khoảng 20 / 41 / 70 ngày khi vào ngày 60 / 120 / 200.
  Chống lạm dụng: mỗi tài khoản Google chỉ một nhân vật nhận khoản này.

---

## 8. Giá trị YAML đề xuất (đề xuất, chưa áp dụng)

```yaml
# realms.yaml (schema mới) — đề xuất
settings:
  idle_rate_milli_per_hour: 2083        # 50 điểm / 24h thực; cửa sổ = Tiên Duyên offline_cap_hours
  daily_points_ceiling: 150
  overflow_cap_bp: 12000                # đề xuất: chỉ áp ở tầng cuối mỗi cảnh giới
  minor_breakthrough_success_bp: 10000  # đề xuất: tầng nhỏ không trượt
tribulation:                            # từ lần đột phá rời Hóa Thần
  from_realm: hoa_than
  base_success_bp: 8000
  step_down_bp: 300
  floor_bp: 5000
  failure_loss_bp: 2500                 # của tu vi đang tích trong TẦNG hiện tại
  cooldown_seconds: 86400
  guaranteed_after_failures: 3
realms:   # cumulative_points = điểm tích lũy để VÀO cảnh giới; stages = 4 kỳ × trọng
  - { id: luyen_khi,       cumulative_points: 0,     stages: 9 }
  - { id: truc_co,         cumulative_points: 300,   stages: 4 }
  - { id: kim_dan,         cumulative_points: 950,   stages: 4 }
  - { id: nguyen_anh,      cumulative_points: 2050,  stages: 8 }
  - { id: hoa_than,        cumulative_points: 3500,  stages: 8 }
  - { id: luyen_hu,        cumulative_points: 5400,  stages: 8 }
  - { id: hop_the,         cumulative_points: 7500,  stages: 8 }
  - { id: dai_thua,        cumulative_points: 9750,  stages: 12 }
  - { id: do_kiep,         cumulative_points: 12250, stages: 12 }
  - { id: nhan_tien,       cumulative_points: 14900, stages: 12 }
  - { id: dia_tien,        cumulative_points: 17750, stages: 12 }
  - { id: kim_tien,        cumulative_points: 20900, stages: 12 }
  - { id: thai_at_kim_tien, cumulative_points: 24350, stages: 16 }
  - { id: dai_la_kim_tien, cumulative_points: 28200, stages: 16 }
  - { id: chuan_thanh,     cumulative_points: 32450, stages: 1 }

# items.yaml + shop — đề xuất
- id: tu_linh_dan
  category: pill
  cultivation_points: 5
  shop_price_spirit_stone: 125
  max_use_per_game_day: 2        # theo nhân vật, kể cả đan được tặng hay trao đổi

# daily_tasks.yaml — đề xuất: thay trung_kich bằng
- { id: nghe_giang, kind: lecture_attend, target: 1, rewards: [{ currency: spirit_stone, amount: 200 }] }

# world_milestones.yaml — đề xuất
progress_mode: per_active_capita   # tiến_độ += đóng góp ngày / DAU ngày
reference_points_per_day: 65       # a_ref, hiệu chỉnh ở closed beta
clamp: { min_bp: 7000, max_bp: 15000 }
late_joiner: { multiplier_bp: 20000, until: server_median, start_grant_bp_of_median: 5000, grant_after_day: 14 }
```

---

## 9. Quyết định cần chủ dự án chốt

1. **Tốc độ treo:** phương án A (50/24h, cần 3 lần đăng nhập) hay B (2 lần đăng nhập là đủ, giá trị cửa sổ Tiên Duyên giảm).
   Đề xuất: **A**.
2. **Phạm vi mất 25% khi trượt kiếp:** tầng hiện tại (+4 ngày) hay cả cảnh giới (+39 ngày). Đề xuất: **tầng**.
3. **Ngưỡng cảnh giới đề xuất** (§3.2) và quy tắc tầng 4 kỳ × trọng (§4). Thêm: bình cảnh chỉ áp ở tầng cuối.
4. **Mốc cốt truyện theo đầu người** thay cho ngưỡng tuyệt đối 10.000 × 100.
   Nếu giữ ngưỡng tuyệt đối, server tham chiếu 10.000 người sẽ tới Tây Du ở **ngày ~495**.
5. **Giới hạn đan tăng tốc mỗi ngày** (B = 10 điểm, B × C ≤ 250 linh thạch) và mục tiêu "quà Tiên Duyên ≤ 6 điểm/ngày".
   Kèm theo: sửa bình luận ngân sách quà trong `vip.yaml`.
6. **Bù cho người vào muộn:** chỉ ×2 (bắt kịp mất ≈ 0,7 × ngày vào) hay thêm điểm khởi đầu bằng 50% trung vị.
7. Xác nhận người chơi ít hoạt động **không** tới Chuẩn Thánh trong năm đầu (I1 khoảng 1.300 ngày, A1 khoảng 730 ngày).

## 10. Việc tiếp theo (Giai đoạn 1)
- Đưa mô hình này vào `services/game/cmd/balancesim`: đọc ngưỡng, lôi kiếp, shop, tổ hợp người chơi từ catalogue,
  không hardcode. Thêm chế độ `-world` cho mốc cốt truyện và `-late J` cho người vào muộn.
- Bổ sung vào mô phỏng: các yếu tố tăng tỉ lệ kiếp (dư điểm, pháp bảo, hộ pháp) có trần, song tu, rời game.
- Closed beta: đo điểm/ngày thực và tổ hợp kiểu người chơi để đặt `reference_points_per_day`.

---

## 11. Vòng 2: số liệu chi tiết sau khi chốt Phase 0 (đề xuất)

Mô hình đã cập nhật theo các quyết định đã chốt:
- bình cảnh 120% chỉ áp ở tầng cuối mỗi cảnh giới, tầng nhỏ được lên liên tiếp;
- mất 25% tu vi của tầng hiện tại khi trượt kiếp;
- người vào sau ngày 14 nhận ×2 cộng quà khởi đầu 50% trung vị.

Với các luật này, ngưỡng §3.2 vẫn cho người 100/ngày tới Chuẩn Thánh ở **ngày 329**, người cày trần ở ngày 221.
Script nằm trong cùng thư mục scratchpad: `p1_stages.py`, `p2_world.py`, `p2b.py`, `p2c.py`, `p3_td.py`, `p4_trib.py`.

### 11.1. Chuẩn Thánh và số tầng chính xác

**Cần kéo dài Chuẩn Thánh bao xa?** Mục tiêu: người cày trần (150/ngày) chưa đầy tầng cuối Chuẩn Thánh trước khi Tây Du mở.
Nếu đầy rồi, Thiên Bảng chỉ còn xếp theo thời điểm chạm đỉnh. Số điểm cần thêm = (ngày Tây Du − 221) × 150.

| Ngày Tây Du thực tế (theo luật mốc §11.2) | Điểm Chuẩn Thánh cần có |
|---|---|
| 326 (ra mắt thực tế, mọi quy mô) | 15.750 |
| 341–349 (DAU cố định) | 18.000–19.200 |
| 371 (cộng đồng ít tương tác) | 22.500 |

**Đề xuất: Chuẩn Thánh = 19.200 điểm** (32.450 → 51.650), chia **4 kỳ × 16 trọng = 64 tầng × 300 điểm**.
- Người cày trần đầy Chuẩn Thánh ở ngày **346 / 349 / 353** (P10/P50/P90), sau Tây Du ở mọi kịch bản chuẩn.
- Người 100/ngày vào ngày 329, đầy ở ngày ~521.
- Thời gian ở Chuẩn Thánh (192 ngày ở mức 100/ngày) dài hơn Đại La (43 ngày), nên vẫn giữ quy tắc "cảnh giới sau dài hơn".
- Nếu Tây Du trễ hơn (cộng đồng ít tương tác), **nối thêm trọng** vào cuối Chuẩn Thánh. ID tầng chỉ thêm, không đổi,
  nên không cần migration. Làm việc này cùng G7 (Tây Du).
- Chuẩn Thánh không có đột phá tiếp. Khi đầy tầng cuối, bình cảnh 120% giữ điểm lại, và Thiên Bảng phân xếp
  theo `stage_reached_at`.

**Bảng tầng đề xuất.** Quy tắc: k = ⌈số ngày của cảnh giới ở mức 100/ngày ÷ 12⌉. Điểm chia đều, phần dư dồn vào các tầng cuối.

| Cảnh giới | Vào ở | Điểm | Cấu trúc | Tầng | Yêu cầu mỗi tầng (điểm × số tầng, theo thứ tự) | Max ngày/tầng @100 | @150 |
|---|---|---|---|---|---|---|---|
| Luyện Khí | 0 | 300 | 9 tầng | 9 | 33×6, 34×3 | 0,34 | 0,23 |
| Trúc Cơ | 300 | 650 | 4 kỳ × 1 | 4 | 162×2, 163×2 | 1,63 | 1,09 |
| Kim Đan | 950 | 1.100 | 4 kỳ × 1 | 4 | 275×4 | 2,75 | 1,83 |
| Nguyên Anh | 2.050 | 1.450 | 4 kỳ × 2 | 8 | 181×6, 182×2 | 1,82 | 1,21 |
| Hóa Thần | 3.500 | 1.900 | 4 kỳ × 2 | 8 | 237×4, 238×4 | 2,38 | 1,59 |
| Luyện Hư | 5.400 | 2.100 | 4 kỳ × 2 | 8 | 262×4, 263×4 | 2,63 | 1,75 |
| Hợp Thể | 7.500 | 2.250 | 4 kỳ × 2 | 8 | 281×6, 282×2 | 2,82 | 1,88 |
| Đại Thừa | 9.750 | 2.500 | 4 kỳ × 3 | 12 | 208×8, 209×4 | 2,09 | 1,39 |
| Độ Kiếp | 12.250 | 2.650 | 4 kỳ × 3 | 12 | 220×2, 221×10 | 2,21 | 1,47 |
| Nhân Tiên | 14.900 | 2.850 | 4 kỳ × 3 | 12 | 237×6, 238×6 | 2,38 | 1,59 |
| Địa Tiên | 17.750 | 3.150 | 4 kỳ × 3 | 12 | 262×6, 263×6 | 2,63 | 1,75 |
| Kim Tiên | 20.900 | 3.450 | 4 kỳ × 3 | 12 | 287×6, 288×6 | 2,88 | 1,92 |
| Thái Ất Kim Tiên | 24.350 | 3.850 | 4 kỳ × 4 | 16 | 240×6, 241×10 | 2,41 | 1,61 |
| Đại La Kim Tiên | 28.200 | 4.250 | 4 kỳ × 4 | 16 | 265×6, 266×10 | 2,66 | 1,77 |
| **Chuẩn Thánh** | 32.450 | **19.200** | 4 kỳ × 16 | **64** | 300×64 | 3,00 | 2,00 |

Tổng cộng **205 tầng**. Đỉnh thang 51.650 điểm (51.650.000 milli), không có rủi ro tràn int64.
Thứ tự trọng trong mỗi kỳ: Sơ Kỳ 1…k → Trung Kỳ 1…k → Hậu Kỳ 1…k → Viên Mãn 1…k. Lôi kiếp nằm ở **Viên Mãn k**.

### 11.2. Ngưỡng mốc cốt truyện theo đầu người

**Một điều chỉnh cần chủ dự án duyệt** (bổ sung cho quyết định Phase 0 #4).
Nếu đếm toàn bộ điểm credited, đợt ra mắt thực tế làm trung bình mỗi người tăng lên **88 điểm/ngày**.
Lý do: người ít hoạt động rời game, và điểm ×2 của người vào muộn được đếm. Khi đó Tây Du rơi vào **ngày 258**,
còn nếu thêm 10% bot thì **ngày 235** (`p2b.py`).

Đề xuất đếm tiến độ thế giới như sau:

```
tiến_độ += Σ_nhân vật min(100, credited_không_tính_phần_×2) / DAU      (mỗi game day)
DAU = số nhân vật có credited > 0 trong ngày đó
```

Hai điểm mới so với quyết định đã chốt:
- **Không đếm phần thưởng ×2 của người vào muộn**. Quà khởi đầu vốn đã không được đếm.
- **Mỗi nhân vật đóng góp tối đa 100 điểm/ngày vào tiến độ thế giới.** Trần 150 cho tu vi cá nhân giữ nguyên.

Tác động lên hệ thống:
- Bot hoặc người cày không thể kéo cốt truyện nhanh hơn mức "người chơi chuẩn".
- Bảng `daily_contribution` vẫn lưu đủ dữ liệu để worker tính lại. Cần thêm cột hoặc trường lưu phần ×2.

**Ngưỡng đề xuất** = ngày mục tiêu × **a_ref = 67** (`threshold_points_per_active` trong `world_milestones.yaml`).
Ngày mục tiêu giữ nguyên. Các mốc có khoảng ngày (Đại chiến 100–115, Thánh nhân 140–170, Nhân tộc 150–210,
Phong Thần 240–300) kích hoạt ở ngày đầu khoảng và chạy bằng `duration_days`.

Ngày kích hoạt mô phỏng (3 seed, min–max). Kịch bản "cố định" = mọi người vào ngày 0, không ai rời.
Kịch bản "thực tế" = 40% vào ngày 0, 60% trong 14 ngày đầu, thêm 0,4% người mới mỗi ngày, rời game theo kiểu chơi.

| Mốc | Mục tiêu | **Ngưỡng/người** | 1k cố định | 5k | 10k | 30k | 1k thực tế | 5k | 10k | 30k |
|---|---|---|---|---|---|---|---|---|---|---|
| Giảng đạo lần 2 | 30 | **2.010** | 31–32 | 32 | 32 | 32 | 31–32 | 32 | 31–32 | 32 |
| Giảng đạo lần 3 | 60 | **4.020** | 62–64 | 63–64 | 63–64 | 63–64 | 61–63 | 62 | 62 | 62 |
| Đại chiến Vu Yêu | 100 | **6.700** | 104–106 | 105–106 | 105–106 | 105–106 | 101–104 | 102–103 | 102–103 | 103 |
| Nữ Oa tạo người | 130 | **8.710** | 135–138 | 136–137 | 136–137 | 136–137 | 130–135 | 132–133 | 132–133 | 132–133 |
| Thánh nhân xuất thế | 140 | **9.380** | 145–148 | 146–148 | 146–148 | 147–148 | 140–145 | 142–143 | 142 | 142 |
| Nhân tộc · Tam Hoàng | 150 | **10.050** | 155–159 | 157–158 | 157–158 | 157–158 | 150–155 | 151–153 | 152 | 152 |
| Phong Thần | 240 | **16.080** | 248–254 | 250–253 | 251–253 | 252–253 | 238–245 | 239–241 | 239–240 | 240 |
| Thiên Đình tái lập | 310 | **20.770** | 320–328 | 323–326 | 323–327 | 325–326 | 305–314 | 306–309 | 306 | 307 |
| Tây Du | 330 | **22.110** | 341–349 | 344–347 | 344–348 | 346–347 | 325–333 | 326–328 | 325 | 326 |

Không mốc nào chạm kẹp 70% hoặc 150%. Điểm đếm trung bình mỗi người: 64 (cố định) và 68–69 (thực tế).

**Độ nhạy** (10.000 người, kịch bản thực tế): ngày Tây Du và điểm đếm trung bình mỗi người.

| Kịch bản | Ngày Tây Du | Điểm đếm TB/người |
|---|---|---|
| Tổ hợp chuẩn | 325 | 69 |
| Thêm 10% bot ở trần 150 | 309 | 73 |
| Cộng đồng ít tương tác (I1, I2, A1 ×1,6) | 371 | 61 |
| Cộng đồng tương tác nhiều | 296 | 76 |

Như vậy luật mới theo sát mức độ hoạt động thực của cộng đồng, và **không còn phụ thuộc quy mô server**: từ 1.000
đến 30.000 người chỉ lệch nhau vài ngày do nhiễu mẫu. a_ref = 67 cần đo lại ở closed beta.

```yaml
# world_milestones.yaml — đề xuất
settings:
  progress_mode: per_active_capita
  world_credit_cap_points: 100      # mỗi nhân vật đóng góp tối đa/ngày vào tiến độ thế giới
  exclude_catch_up_bonus: true      # phần ×2 và quà khởi đầu không đếm
  reference_points_per_active_day: 67
  min_pct_bp: 7000
  max_pct_bp: 15000
milestones:   # threshold_points_per_active = target_day × 67
  - { id: giang_dao_2, target_day: 30,  threshold_points_per_active: 2010 }
  - { id: giang_dao_3, target_day: 60,  threshold_points_per_active: 4020 }
  - { id: dai_chien,   target_day: 100, threshold_points_per_active: 6700 }    # duration_days: 15
  - { id: nu_oa,       target_day: 130, threshold_points_per_active: 8710 }
  - { id: thanh_nhan,  target_day: 140, threshold_points_per_active: 9380 }    # đợt kéo dài tới ngày 170
  - { id: nhan_toc,    target_day: 150, threshold_points_per_active: 10050 }   # tới ngày 210
  - { id: phong_than,  target_day: 240, threshold_points_per_active: 16080 }   # duration_days: 60
  - { id: thien_dinh,  target_day: 310, threshold_points_per_active: 20770 }
  - { id: tay_du,      target_day: 330, threshold_points_per_active: 22110 }
```

### 11.3. Tiên Duyên: điểm duyên, ngưỡng cấp, quà ngày

**Điểm duyên từ hoạt động** (đề xuất). Trần **20 điểm/ngày** cho tổng các nguồn không phải nạp. Nạp: 1.000đ = 10 điểm.

| Hoạt động | `id` | Nguồn | Điểm | Tối đa lượt/ngày | Max/ngày |
|---|---|---|---|---|---|
| Tu luyện online (các lần thu cách nhau ≥ 1h) | `online_cultivation` | cultivation | 1 | 3 | 3 |
| Nghe giảng đạo | `lecture_attendance` | lecture | 3 | 2 | 6 |
| Nhận thưởng nhiệm vụ ngày | `daily_task_claim` | task | 1 | 4 | 4 |
| Hộ pháp đồng môn | `protector_assist` | interaction | 2 | 2 | 4 |
| Song tu | `dual_cultivation` | interaction | 2 | 1 | 2 |
| Góp công thế lực | `faction_merit` | interaction | 1 | 3 | 3 |
| Qua bí cảnh hoặc mini game | `trial_pass` | trial | 2 | 2 | 4 |
| Tham gia sự kiện | `event_participation` | event | 3 | 1 | 3 |

Tổng lý thuyết 29 điểm/ngày, bị trần 20 chặn lại. Mỗi kiểu người chơi kiếm (giả định 1 lượt bí cảnh/ngày và sự kiện 1/3 số ngày):
I1 2 · I2 3 · I3 5 · A1 9 · **A2 13 · A3 15** · S1 12 · S2 18 · S3 20 điểm duyên/ngày.

**Ngưỡng cấp đề xuất.** Cột "Giá nạp" là số tiền tương đương để mua thẳng cấp đó bằng tiền nạp.

| Cấp | Cũ (1.000đ = 1 điểm) | **Đề xuất (1.000đ = 10 điểm)** | Giá nạp | Cửa sổ | Quà LT/ngày | Treo 1 lần + phù | Lợi ích tiến độ tối đa của quà |
|---|---|---|---|---|---|---|---|
| 0 | 0 | **0** | 0 | 8h | 0 | 25,0 | 0 |
| 1 | 10 | **30** | 3.000đ | 10h | 0 | 29,2 | 0 |
| 2 | 50 | **150** | 15.000đ | 12h | 50 | 33,3 | 2 điểm/ngày |
| 3 | 100 | **500** | 50.000đ | 14h | 50 | 37,5 | 2 |
| 4 | 200 | **1.200** | 120.000đ | 16h | 50 | 41,7 | 2 |
| 5 | 500 | **2.500** | 250.000đ | 18h | 100 | 45,8 | 4 |
| 6 | 1.000 | **4.500** | 450.000đ | 20h | 100 | 50,0 | 4 |
| 7 | 2.000 | **10.000** | 1.000.000đ | 22h | 100 | 50,0 | 4 |
| 8 | 5.000 | **25.000** | 2.500.000đ | 24h | 200 | 50,0 | 6 |
| 9 | 10.000 | **50.000** | 5.000.000đ | 26h | 200 | 50,0 | 6 |
| 10 | 20.000 | **100.000** | 10.000.000đ | 28h | 200 | 50,0 | 6 |
| 11 | 35.000 | **250.000** | 25.000.000đ | 30h | 300 | 50,0 | 6 |
| 12 | 50.000 | **500.000** | 50.000.000đ | 32h | 300 | 50,0 | 6 |

Cấp đạt được khi chỉ chơi miễn phí:

| Kiểu | Ngày 30 | Ngày 90 | Ngày 180 | Ngày 365 | Ngày lên cấp 4 | cấp 6 | cấp 7 |
|---|---|---|---|---|---|---|---|
| I1 / I3 (chỉ treo) | 1 / 2 | 2 / 2 | 2 / 3 | 3 / 4 | 600 / 240 | — | — |
| A1 | 2 | 3 | 4 | 5 | 134 | 500 | — |
| **A2 / A3 (tích cực)** | 2 | **3 / 4** | 4 / 5 | **6** | 93 / 80 | 347 / 300 | 770 / 667 |
| S3 (tích cực + cộng đồng) | 3 | 4 | 5 | 6 | 60 | 225 | 500 |

Mục tiêu được đề ra (người tích cực miễn phí đạt cấp 3–4 ở ngày 90, cấp 6 sau một năm) **đạt đúng**.
Góp ý về mục tiêu:
- Giữ nguyên các mục tiêu này. Người miễn phí chăm nhất đạt cấp 6 ở ngày ~225. Ở cấp 6, cửa sổ 20h cộng Tụ Linh Phù
  đã đủ 24h, nên treo 1 lần/ngày cũng nhận đủ 50 điểm, bằng TD12.
- Cấp 8 trở lên gần như chỉ đạt được bằng nạp (S3 cần khoảng 1.250 ngày). Cấp 8 trở lên chỉ còn giá trị ở quà và ngoại hình.
- Giá các cấp 4–8 rẻ hơn bảng cũ từ 2 đến 2,2 lần. Cấp 12 giữ nguyên 50 triệu đ. Doanh thu là quyết định kinh doanh:
  muốn giữ giá cũ ở các cấp giữa thì phải nâng ngưỡng, và người miễn phí sẽ chậm hơn mục tiêu.

**Kiểm tra quà ngày với trần đan tăng tốc (quyết định Phase 0 #5).**
- Linh thạch chỉ đổi thành tu vi qua đan tăng tốc: tối đa 10 điểm/ngày, chi phí 250 linh thạch.
- Người làm nhiệm vụ đã tự mua đủ trần (≥ 300 LT/ngày), nên **quà không thêm tiến độ cho họ**.
- Người chỉ treo 1 lần/ngày tự mua được 4 điểm/ngày. Quà nâng con số này lên tối đa 10, tức **chênh ≤ 6 điểm/ngày**.
- Luật "cấp L đăng nhập 1 lần ≤ miễn phí đăng nhập 3 lần" **giữ ở cả 13 cấp và 3 kiểu hoạt động**, với quà như hiện tại (`p3_td.py`).
- **Kết luận: giữ nguyên số quà.**
- Lưu ý kinh tế: người tích cực (A3) dư **200 LT/ngày** sau khi mua đan (300 nếu ở cấp 6).
  Cửa hàng phải có chỗ tiêu không tăng tiến độ (ngoại hình, động phủ, vật phẩm điều kiện đột phá theo quyết định #11)
  trước khi thêm nguồn linh thạch từ bí cảnh.

**Bình luận ngân sách quà đề xuất cho `vip.yaml`** (thay dòng "Budget: VIP12 gift ≤ ~15%…"):

```yaml
# daily_gift: spirit stones claimable per day at this tier (absolute, not cumulative).
# BUDGET (decisions 2026-11-06 "Phase 0" #5): stones turn into cultivation ONLY through speed items, capped per
#   character per game day (Tụ Linh Đan 2 × 5 = 10 points, cost 250 ≤ stone income of a free 3-login idle player).
#   ⇒ a gift's progress value ≤ 10 − 4 (what a free 1-login player self-funds) = 6 points/day, and 0 for anyone
#   doing daily tasks. Invariant checked by balancesim: tier L at 1 login/day ≤ free at 3 logins/day, every tier,
#   every activity profile. Gift stones beyond the speed-item budget need non-progress sinks (cosmetics, cave,
#   breakthrough-requirement items); track stone creation vs destruction per day.
```

### 11.4. Tỉ lệ cộng của lôi kiếp

So sánh các cấu hình (`p4_trib.py`, 1.500 lượt chạy). Hai chiến lược thử kiếp:
"thử ngay" = thử ngay khi đủ 100% yêu cầu; "đợi 120%" = đợi đầy bình cảnh rồi mới thử.

| Cấu hình | Chiến lược | Số lần trượt/10 kiếp | P(dùng bảo hiểm ≥ 1 lần) | Ngày CT: S3 · A1 · G |
|---|---|---|---|---|
| Không cộng | thử ngay | 5,02 | 38% | 329 · 731 · 221 |
| Nháp data-model (dốc 0,5, dư ≤ 10%, tổng ≤ 20%, không trần), chưa có đồ | thử ngay | 4,07 | 23% | 328 · 730 · 220 |
| | đợi 120% | 3,19 | 17% | 330 · 736 · 222 |
| Nháp, đồ + hộ pháp tối đa | thử ngay | **1,68** | **5%** | 326 · 726 · 218 |
| **Đề xuất**, chưa có đồ | thử ngay | 4,47 | 28% | 329 · 730 · 220 |
| | đợi 120% | 4,04 | 26% | 331 · 737 · 223 |
| **Đề xuất**, đồ + hộ pháp tối đa | thử ngay | **2,98** | **15%** | 327 · 728 · 219 |

Tỉ lệ từng kiếp theo đề xuất:

| Kiếp (rời cảnh giới) | Cơ bản | + dư 20% | + tối đa (trần 90%) | P(trượt 3 lần liền): cơ bản → tối đa | Nháp: tối đa |
|---|---|---|---|---|---|
| Hóa Thần | 80% | 85% | 90% | 0,8% → 0,1% | **100%** |
| Hợp Thể | 74% | 79% | 86% | 1,8% → 0,3% | 94% |
| Độ Kiếp | 68% | 73% | 80% | 3,3% → 0,8% | 88% |
| Kim Tiên | 59% | 64% | 71% | 6,9% → 2,4% | 79% |
| Đại La Kim Tiên | 53% | 58% | 65% | 10,4% → 4,3% | 73% |

**Nhận xét:**
- Nháp trong data-model có hai vấn đề:
  - **Kiếp đầu thành 100%** khi đủ đồ.
  - Số lần trượt giảm từ 5 xuống 1,7, nên bảo hiểm 3 lần gần như không bao giờ được dùng (5%).
- **Đợi đầy 120% không bao giờ có lợi về thời gian** ở cả hai cấu hình: chậm hơn 2–7 ngày. Không xuất hiện chiến lược
  "tích trữ" bắt buộc. Phần cộng từ dư điểm chủ yếu giúp **lần thử sau khi trượt**, vì trong 24h cooldown tu vi tự
  hồi về gần 120%. Đây là cơ chế an ủi tự nhiên.
- Lôi kiếp chỉ tốn 2–4 ngày tổng cộng ở mọi cấu hình. Tác dụng chính là cảm giác căng thẳng và quyết định trước mỗi
  lần thử, không phải nhịp tiến độ.
- Sàn 50% áp lên **tỉ lệ cơ bản**. Trên thang 15 cảnh giới, sàn không bao giờ chạm, vì thấp nhất là 53%.
  Sàn có tác dụng với cảnh giới thêm sau này hoặc đạo đồ khác. Trần 90% giữ khoảng cách giữa kiếp đầu (90%) và kiếp
  cuối (65%), và giữ xác suất trượt 3 lần liền ở các kiếp cuối từ 2–4% trở lên.

**Đề xuất `tribulation` (thay khối `surplus_bonus` / `max_total_bonus_bp` trong data-model):**

```yaml
tribulation:
  # … base_bp 8000, step_bp 300, floor_bp 5000, loss_bp 2500, cooldown 86400, guarantee_after 3 (giữ nguyên)
  surplus_bonus:
    bp_per_surplus_bp: 2500       # mỗi 1% dư → +0,25%; dư tối đa 20% (bình cảnh) → +5%
    max_bp: 500
  item_bonus_max_bp: 700          # pháp bảo (≤ 500) + đan hộ thân (≤ 300), chỉ rơi từ bí cảnh, không bán
  protector_bonus_max_bp: 300     # hộ pháp: +100 mỗi đồng môn online, tối đa 3 người
  max_total_bonus_bp: 1200        # dư + đồ + hộ pháp
  max_success_bp: 9000            # trần tỉ lệ cuối (trừ lần bảo hiểm = 10000)
# success = guarantee ? 10000 : min(max_success_bp, max(floor_bp, base) + min(max_total_bonus_bp, surplus + items + protector))
```

Validator: `surplus.max_bp ≤ max_total_bonus_bp`, `max_total_bonus_bp + base_bp ≥ max_success_bp`
(nghĩa là trần 90% có thể chạm được ở kiếp đầu), `max_success_bp < 10000`.

### 11.5. Việc chủ dự án cần duyệt thêm
1. Chuẩn Thánh 19.200 điểm, 64 tầng; nối thêm trọng nếu Tây Du trễ.
2. **Bổ sung cho quyết định #4:** tiến độ thế giới không đếm phần ×2, và mỗi nhân vật đóng góp tối đa 100/ngày; a_ref = 67.
3. Ngưỡng Tiên Duyên mới (giá các cấp 4–8 rẻ hơn 2–2,2 lần so với bảng cũ) và bảng điểm duyên theo hoạt động.
   Giữ nguyên quà ngày.
4. Thông số cộng lôi kiếp: dư ≤ +5%, đồ ≤ +7%, hộ pháp ≤ +3%, tổng ≤ +12%, trần 90%. Nháp data-model (+10/+20%,
   không trần) cần sửa theo.
