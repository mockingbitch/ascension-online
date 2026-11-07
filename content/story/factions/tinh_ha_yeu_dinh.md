---
id: tinh_ha_yeu_dinh
kind: faction
title: Thiên Đình Yêu Tộc
tags: [faction, starting, tien, yeu]
status: draft
refs: [path.tien, milestone.khai_giang, milestone.dai_chien]
replaces_placeholder: yeu_dinh
---

# Thiên Đình Yêu Tộc

> **Tâm ngôn:** "Sao trời không hỏi xuất thân."

ID `tinh_ha_yeu_dinh` là slug cũ, giữ nguyên vì ID vĩnh viễn. Tên hiển thị theo `docs/design/cot-truyen.md` §4
("Thiên Đình của Yêu tộc").

## Giới thiệu

Trước khi có người, trời đất thuộc về muôn loài. Chim bay, thú chạy, cá lặn. Những con vật hấp thụ linh khí đủ
lâu thì hóa hình, mở linh trí và gọi mình là Yêu tộc. Thiên Đình Yêu Tộc là triều đình của họ, do Đế Tuấn và Thái
Nhất cầm đầu. Cung điện treo trên chín tầng trời, giữa dải sao, và mỗi vì sao là một chỗ ngồi dành cho một thành
viên. Thiên Đình tự hào vì không xét gốc gác. Hổ, hạc, rắn hay cá chép, ai tu thành đều được một ngôi sao. Yêu tộc
cho rằng bầu trời là của mình, còn mặt đất là của Vu Tộc. Hai bên đã nhìn nhau qua đường chân trời quá lâu, và ngày
nào đó cái nhìn ấy sẽ hóa thành Đại chiến Vu Yêu.

## Lối tu luyện

**Tiên đạo** (tạm thời). Thành viên đi chuỗi cảnh giới tiên, từ Luyện Khí lên Chuẩn Thánh. Văn phong nhấn vào
việc hấp thụ tinh quang và hóa hình.

## Nét riêng (chỉ để tạo không khí, không cộng chỉ số)

1. **Sao bản mệnh.** Mỗi thành viên nhận một vì sao trên trời. Tên sao được đọc lên khi họ nhập Thiên Đình.
2. **Bản tướng.** Ai cũng giữ ký ức về hình dạng đầu tiên của mình và tự hào về nó.
3. **Chu Thiên Tinh Đấu.** Khi đủ người, Yêu tộc xếp thành đại trận theo sao trời (`cot-truyen.md` §5.3, chỉ là văn).

## Huy hiệu (gửi graphic-designer)

Nền tròn tối. Một cung gồm bảy ngôi sao nhỏ màu `spirit-violet` bao quanh nửa trên. Ở giữa là một chiếc lông vũ
cách điệu màu `gold-500`, cắm chéo, có sống lông mảnh. Có thể đặt một con ngươi dọc màu `spirit-cyan` mờ ở gốc
lông để gợi chất "yêu". viewBox 64×64.

## Người đứng đầu

**Đế Tuấn** và **Thái Nhất** (NPC, `cot-truyen.md` §10 cảnh 6). Vai trò "đứng đầu Thiên Đình Yêu tộc" lấy theo
mạch Hồng Hoang phổ biến; `cot-truyen.md` chỉ liệt kê hai vị trong các đại năng hiện thế.

## Ghi chú thiết kế

- Đây không phải Thiên Đình được tái lập ở mốc `thien_dinh` (Thiên Đình của người lên Phong Thần Bảng).
- Đường tu Yêu riêng (G3) **chưa** được quyết định. Hiện thành viên dùng `allowed_paths: [tien]`, nên văn bản không
  hứa hẹn cơ chế hóa hình hay hiệu ứng tập thể nào.
- Số phận sau Đại chiến (vào Địa Phủ hay luân hồi, `cot-truyen.md` §7.1) phụ thuộc G2/G4. Chưa viết ở đây.
- Thế lực chưa cộng chỉ số gì.
