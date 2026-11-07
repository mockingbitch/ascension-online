# Lời giảng đạo ở Tử Tiêu Cung

Hồng Quân giảng đạo suốt ngày trong Tử Tiêu Cung. Người chơi đang online ngồi trong điện và nghe. Mỗi file ở đây
là một bài giảng. Các bài nối nhau theo `order` thành một vòng, hết bài cuối thì quay lại bài 1
(quyết định sản phẩm 2026-11-07).

- Lời của Hồng Quân hiện **từng câu một** thành popup, mỗi câu hiện trong `seconds` giây.
- Cứ 10 phút có một khoảnh khắc **ngộ đạo**. Hồng Quân hỏi `question` của bài đang giảng. Người chơi chạm "ngộ"
  để nhận tu vi, và `takeaway` của bài được ghi vào sổ **Đạo ngộ** của họ.

## Định dạng (loader Go đọc strict, từ chối key lạ)

File: `NN_<slug>.yaml`, trong đó `NN` là `order` có 2 chữ số và `<slug>` trùng `id`.

```yaml
id: dao_tich_tieu_thanh_dai      # slug ASCII snake_case, vĩnh viễn, duy nhất
order: 1                         # vị trí trong vòng, 1..N, duy nhất, liên tục
title: Tích tiểu thành đại       # tên hiển thị
theme: Kiên trì tích lũy         # 2–4 chữ (tiếng)
lines:                           # Hồng Quân nói lần lượt, mỗi câu một popup
  - text: "…"                    # 1–160 ký tự, một hoặc hai câu
    seconds: 12                  # 8–20; xấp xỉ ký tự/12 + 3, tối thiểu 8
question: "…"                    # câu hỏi lúc ngộ đạo, ≤ 160 ký tự
takeaway: "…"                    # bài học một câu, lưu vào sổ Đạo ngộ, ≤ 160 ký tự
```

Ràng buộc:

| Trường | Quy tắc |
|---|---|
| `lines` | 18–30 câu |
| `text`, `question`, `takeaway` | Đếm theo ký tự Unicode (rune, chuẩn NFC), không theo byte |
| `seconds` | 8–20 mỗi câu. **Tổng một bài: 240–360** (giảng 4–6 phút) |
| Comment `#` | Được phép, loader bỏ qua |

## Quy tắc nội dung

1. **Có ý nghĩa thật.** Mỗi bài dạy một điều đúng cho cả đường tu lẫn đời thường. Ít nhất một câu nói rõ chuyện
   "ở đời cũng vậy". Bài học gắn với vòng chơi thật: bế quan tích lũy, đột phá, bình cảnh, lôi kiếp, Thiên Bảng,
   đạo hữu.
2. **Giọng.** Một bậc thầy cổ xưa, trang nghiêm mà ấm áp. Hồng Quân xưng "ta", gọi người nghe là "ngươi" hoặc
   "các ngươi". Câu ngắn, đọc vừa một popup. Tiếng Việt có dấu đầy đủ, dùng thuật ngữ trong glossary của
   `.claude/rules/story-writing.md`.
3. **Tên và tín ngưỡng.** Tên nhân vật, thế lực, nơi chốn theo `docs/design/cot-truyen.md` (chủ dự án đảo ngược D2
   ngày 2026-11-07). Hồng Quân là người thầy chứ không phải đối tượng thờ phụng: không lạy, không cầu, không khấn,
   không xin ban phước; Hồng Quân nói rằng đạo không thuộc về mình. Được nhắc tên Tam Thanh, Chuẩn Đề, Tiếp Dẫn, Nữ
   Oa… như học trò ngồi nghe, nhưng không trích hay chép nguyên văn kinh điển (Đạo Đức Kinh, kinh Phật…), không
   biến họ thành trò đùa, không đặt lời cầu khấn. Văn phải tự viết, có thể gợi ý tưởng cổ nhưng không mượn câu chữ.
4. **Không hứa cơ chế chưa có.** Chỉ nhắc cơ chế đã chốt trong `docs/decisions.md`, và không đưa con số. Lôi kiếp
   làm hao tu vi ở cảnh giới cao. Thất bại không làm rớt cảnh giới. Sau vài lần thất bại thì lần sau chắc thành.
   Bài giảng lặp lại nên bỏ lỡ không sao.
5. **Không pay-to-win, không FOMO.** Không gắn Tiên Duyên/nạp với sức mạnh hay số mệnh. Không thúc ép thức khuya.
   Bài 3 khuyên rõ là nên đi ngủ. Nói về thất bại thì phải khích lệ, có nhắc chuyện thử lại.
6. **Tên riêng** (cảnh giới, linh căn, vật phẩm) viết đúng như trong `content/catalog/`.

## Thêm bài giảng

1. **ID chỉ được thêm, không đổi, không xóa.** `id` có thể nằm trong sổ Đạo ngộ của người chơi. Muốn bỏ một bài
   khỏi vòng thì đề xuất cờ `retired` cho loader, không xóa file.
2. Bài mới lấy `order` tiếp theo (09, 10…). Muốn chèn vào giữa vòng thì đổi `order` và tên file của các bài sau,
   còn `id` giữ nguyên.
3. Tính lại tổng `seconds` (240–360) trước khi commit. Thêm bài thì độ dài cả vòng thay đổi, nên báo cho
   backend nếu lịch ngộ đạo phụ thuộc vào nó.

## Các vòng giảng

| Vòng | Mở ở mốc | Trạng thái |
|---|---|---|
| `tu_tieu_cycle_1` | `khai_giang` | 8 bài dưới đây (bản nháp) |
| `tu_tieu_cycle_2` | `giang_dao_2` | chưa viết |
| `tu_tieu_cycle_3` | `giang_dao_3` | chưa viết |

Vòng mới chồng lên vòng cũ, không thay thế. Định dạng file và cách loader gán bài vào vòng chờ `data-architect`
(đề xuất trong `../world/milestone-effects.md` §Ghi chú thiết kế).

## Vòng 1 (bản nháp)

| # | id | Tên | Chủ đề | Giây |
|---|---|---|---|---|
| 1 | `dao_tich_tieu_thanh_dai` | Tích tiểu thành đại | Kiên trì tích lũy | 288 |
| 2 | `tam_tinh_nhu_mat_ho` | Tâm tĩnh như mặt hồ | Giữ tâm bình lặng | 280 |
| 3 | `ngay_dem_cung_nuoi_dao` | Ngày và đêm cùng nuôi đạo | Cân bằng cần nghỉ | 265 |
| 4 | `hat_giong_khong_noi_doi` | Hạt giống không biết nói dối | Nhân quả | 270 |
| 5 | `thien_bang_khac_ten` | Thiên Bảng khắc tên, không khắc đạo | Khiêm cung tự tại | 267 |
| 6 | `binh_canh_la_cua` | Bình cảnh là cửa, không phải tường | Chấp nhận thất bại | 276 |
| 7 | `gieng_sau_khong_so_nguoi_muc` | Giếng sâu không sợ người múc | Chia sẻ, tử tế | 277 |
| 8 | `buoc_qua_nguong_cua` | Bước qua ngưỡng cửa | Can đảm đột phá | 275 |

Cả vòng dài 2.198 giây (khoảng 36 phút 38 giây), nên mỗi vòng có khoảng 3–4 lần ngộ đạo. Bài 8 kết bằng câu dẫn
về "giọt linh khí đầu tiên" của bài 1 để vòng giảng liền mạch.

## Ghi chú thiết kế

- Đề xuất cách đồng bộ: vị trí trong vòng = (giờ server − mốc epoch) mod tổng giây cả vòng. Như vậy mọi người
  trong điện thấy cùng một câu, không cần ticker riêng cho từng người chơi (non-negotiable #6).
- Ngộ đạo dùng `question` và `takeaway` của bài đang giảng ở thời điểm đó. Phần thưởng tu vi tính vào nhóm
  `activity` (trần 45/ngày), và server quyết định số tu vi, client không gửi.
- Bài 4 và bài 6 nhắc "trời chưa thuận" một cách trung thực, vì tỉ lệ đột phá có yếu tố may rủi. Đừng sửa thành
  "thất bại là do ngươi".
