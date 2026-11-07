---
id: huyen_khung_mon
kind: faction
title: Tam Thanh
tags: [faction, starting, tien, split_at_thanh_nhan]
status: draft
refs: [path.tien, milestone.khai_giang, milestone.thanh_nhan]
replaces_placeholder: huyen_mon
---

# Tam Thanh

> **Tâm ngôn:** "Một khí hóa ba thanh, ba nhánh đạo cùng về một nguồn."

ID `huyen_khung_mon` là slug cũ, giữ nguyên vì ID vĩnh viễn. Tên hiển thị theo `docs/design/cot-truyen.md` §4.

## Giới thiệu

Bàn Cổ ngã xuống, nguyên thần hóa thành ba luồng thanh khí. Ba luồng ấy tu thành ba vị đại năng: Thái Thanh Lão
Tử, Nguyên Thủy Thiên Tôn và Thông Thiên Giáo Chủ, gọi chung là Tam Thanh. Ba vị dựng đạo trường trên núi Côn
Lôn, rồi cùng đến Tử Tiêu Cung nghe Hồng Quân giảng đạo. Môn hạ Tam Thanh tin rằng đạo như vòm trời: nhìn từ đâu
cũng thấy, nhưng không ai ôm trọn. Vì thế ba vị tuy chung một gốc, mỗi vị lại hiểu đạo một cách riêng. Thái Thanh
giữ sự thanh tĩnh, Nguyên Thủy giữ quy củ, Thông Thiên mở cửa cho mọi người. Ba vị cùng ngồi chung một điện, cùng
nghe một bài giảng, nhưng ngày càng ít nói chung một lời. Người mới nhập môn chưa cần chọn theo vị nào. Cứ nghe
giảng, cứ tu luyện, rồi đến lúc trời đất đổi thay, đạo tâm sẽ tự biết mình thuộc về đâu.

## Lối tu luyện

**Tiên đạo.** Tu luyện bằng cách nghe giảng đạo, ngồi thiền và bế quan trong động phủ, đi trọn chuỗi cảnh giới
từ Luyện Khí lên Chuẩn Thánh.

## Nét riêng (chỉ để tạo không khí, không cộng chỉ số)

1. **Chép đạo ngữ.** Đệ tử ghi lại từng câu Hồng Quân giảng lên thẻ ngọc, rồi tranh luận suốt đêm về một chữ.
2. **Ba nhánh một nguồn.** Ba vị Tam Thanh cùng chung một đạo trường ở Côn Lôn. Ai tinh ý sẽ thấy vết rạn đầu tiên.
3. **Thẻ ngọc trống.** Lễ nhập môn trao một thẻ ngọc chưa khắc chữ. Thẻ ấy chỉ được khắc khi đạo tâm đã định.

## Huy hiệu (gửi graphic-designer)

Khung tròn viền `gold-500` mảnh. Bên trong có ba cung tròn đồng tâm màu `jade-300`, `jade-500` và `jade-700`,
xếp như vòm trời úp xuống. Ba cung có độ dày bằng nhau, giữa các cung chừa khe hở. Dưới vòm, ngay tâm, là một
chấm `gold-300` tượng trưng cho "một nguồn". Không dùng biểu tượng âm dương, bát quái hay hình tượng thờ cúng của
Đạo giáo. viewBox 64×64, các nét dùng `currentColor` để đổi tông được.

## Người đứng đầu

**Tam Thanh**: Thái Thanh Lão Tử, Nguyên Thủy Thiên Tôn, Thông Thiên Giáo Chủ (NPC, theo hệ Hồng Hoang và Phong
Thần, `cot-truyen.md` §4).

## Ghi chú thiết kế

- Tam Thanh và Đạo giáo là tín ngưỡng thật (`cot-truyen.md` §8). Giữ tông tôn trọng: ba vị là bậc thầy trong
  truyện, không bị chế giễu, không làm đối thủ để đánh, không viết lời khấn vái hay chép nguyên văn kinh điển.
- Ở mốc `thanh_nhan` (hiệu ứng `split_faction`), Tam Thanh tách thành ba giáo (`cot-truyen.md` §7.2):
  `thu_nhat_vien` = **Nhân Giáo** (Thái Thanh Lão Tử), `lap_cuc_vien` = **Xiển Giáo** (Nguyên Thủy Thiên Tôn),
  `van_luu_vien` = **Tiệt Giáo** (Thông Thiên Giáo Chủ). Người chơi có 3 ngày báo trước và 7 ngày để chọn, giữ
  nguyên tu vi. Mỗi giáo sẽ có trang riêng sau.
- Thế lực **chưa** cộng chỉ số gì. Mọi "nét riêng" ở trên chỉ là văn mô tả.
