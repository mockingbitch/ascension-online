---
id: kim_sa_mon
kind: faction
title: Tây Phương
tags: [faction, starting, tien, renamed_at_thanh_nhan]
status: draft
refs: [path.tien, milestone.khai_giang, milestone.thanh_nhan]
replaces_placeholder: tay_phuong_mon
---

# Tây Phương

> **Tâm ngôn:** "Đất tây tuy cằn, nguyện lực rất dài."

ID `kim_sa_mon` là slug cũ, giữ nguyên vì ID vĩnh viễn. Tên hiển thị theo `docs/design/cot-truyen.md` §4 ("Chuẩn
Đề, phe Tây phương").

## Giới thiệu

Sau trận tranh bá của Long, Phượng, Kỳ Lân, đất Tây phương cằn cỗi, linh khí mỏng như sương sớm. Chuẩn Đề và
Tiếp Dẫn từng nghe giảng ở Tử Tiêu Cung, rồi về phương tây với một lời nguyện: không để nơi nghèo linh khí mãi
nghèo đạo pháp. Đệ tử Tây Phương tu chậm nhưng bền. Họ gom từng sợi linh khí như gom từng hạt cát, và tin rằng
kiên nhẫn chính là một loại thiên phú. Phe Chuẩn Đề không hỏi xuất thân. Kẻ hàn môn, kẻ lạc đường, kẻ từng thất
bại đều có chỗ ngồi bên đống lửa nơi đất tây. Người ngoài chê họ đi chậm. Người Tây Phương chỉ mỉm cười, vì sa
mạc nào cũng được đắp nên từ những hạt cát rất nhỏ.

## Lối tu luyện

**Tiên đạo.** Đi cùng chuỗi cảnh giới tiên, từ Luyện Khí lên Chuẩn Thánh. Văn phong nhấn vào nhẫn nại và bế quan
dài ngày.

## Nét riêng (chỉ để tạo không khí, không cộng chỉ số)

1. **Túi cát nhập môn.** Mỗi đệ tử mang theo một túi cát vàng, lấy từ nơi mình bước vào con đường tu hành.
2. **Đi xa thu người.** Sứ giả Tây Phương đi khắp Hồng Hoang, mời những người bị nơi khác từ chối.
3. **Lời nguyện trước bế quan.** Trước mỗi lần bế quan, đệ tử nói to một điều mình muốn giữ trọn.

## Huy hiệu (gửi graphic-designer)

Nền tròn. Nửa dưới là ba đường cồn cát lượn sóng xếp chồng, màu `gold-700`, `gold-500` và `gold-300` từ dưới
lên. Một vầng mặt trời `gold-300` lặn nửa sau cồn cát. Phía trên mặt trời có một ngôi sao nhỏ bốn cánh màu
`spirit-cyan`, tượng trưng cho lời nguyện. **Không** dùng hoa sen, bánh xe pháp luân, chuông, chữ vạn hay bất kỳ
biểu tượng thờ cúng nào. viewBox 64×64.

## Người đứng đầu

**Chuẩn Đề** và **Tiếp Dẫn** (NPC, `cot-truyen.md` §4, §7.2): một vị lo thu nhận người mới, một vị lo truyền pháp.

## Ghi chú thiết kế

- Chuẩn Đề và Tiếp Dẫn gắn với Phật giáo trong nhiều bản truyện. Giữ tông tôn trọng: không chế giễu, không làm đối
  thủ để đánh, không lời cầu khấn, không chép kinh văn.
- Ở mốc `thanh_nhan` (hiệu ứng `migrate_faction`), phe đổi thành **Tây Phương Giáo** (ID `kim_sa_giao`).
  Thành viên được chuyển tự động, báo trước 3 ngày.
- Thế lực chưa cộng chỉ số gì.
