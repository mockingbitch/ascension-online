---
id: thuong_lan_dinh
kind: faction
title: Tiên Đình
tags: [faction, starting, tien, closes_at_luan_hoi]
status: draft
refs: [path.tien, milestone.khai_giang, milestone.dai_chien]
replaces_placeholder: dong_tien_dinh
---

# Tiên Đình

> **Tâm ngôn:** "Bốn biển một sổ tiên, không ai bị bỏ sót."

ID `thuong_lan_dinh` là slug cũ, giữ nguyên vì ID vĩnh viễn. Tên hiển thị theo `docs/design/cot-truyen.md` §4
("Tiên Đình của Đông Vương Công").

## Giới thiệu

Ngoài khơi biển Đông có những hòn đảo tiên, nối với nhau bằng những cây cầu sương. Đó là nơi Đông Vương Công đặt
Tiên Đình. Đây là triều đình đầu tiên của tu sĩ, ra đời khi các thế lực khác còn đang dựng đạo trường. Tiên Đình
giữ sổ tiên, cuốn sổ ghi tên mọi người bước lên con đường thành tiên, bất kể họ thuộc phe nào. Người Tiên Đình coi
trọng lễ nghi, phẩm trật và lời hứa. Họ tin rằng con đường tu hành không có trật tự thì sẽ thành loạn. Thế nhưng
trật tự của biển chỉ yên khi trời yên. Khi Vu Tộc và Thiên Đình Yêu Tộc bắt đầu tranh nhau trời đất, những cây cầu
sương cũng bắt đầu lung lay.

## Lối tu luyện

**Tiên đạo.** Đi cùng chuỗi cảnh giới tiên, từ Luyện Khí lên Chuẩn Thánh. Văn phong nhấn vào lễ nghi, phẩm trật
và việc ghi danh.

## Nét riêng (chỉ để tạo không khí, không cộng chỉ số)

1. **Ghi danh sổ tiên.** Mỗi lần thành viên đột phá đại cảnh giới, một dòng mới được viết thêm vào sổ.
2. **Triều trên đảo tiên.** Đảo trôi theo thủy triều, nên mỗi ngày đường đến điện chính lại khác đi đôi chút.
3. **Phẩm phục theo màu biển.** Áo của tu sĩ đổi dần từ xanh nhạt sang xanh thẳm khi cảnh giới tăng lên.

## Huy hiệu (gửi graphic-designer)

Ấn tròn. Nửa dưới là ba ngọn sóng cuộn màu `spirit-cyan`. Nổi trên sóng là mái lầu ba tầng màu `gold-500`, vẽ
bằng nét đơn giản, góc mái hơi cong. Viền ấn màu `gold-700`. Có thể thêm một vệt sương mảnh `jade-300` nối hai
bên mái, tượng trưng cho cầu sương. viewBox 64×64.

## Người đứng đầu

**Đông Vương Công** (NPC, `cot-truyen.md` §4, §10): người đã viết dòng đầu tiên vào sổ tiên.

## Ghi chú thiết kế

- Thế lực đóng từ kỷ nguyên `luan_hoi`, tức sau Đại chiến Vu Yêu (`on_close: split`, quyết định 2026-11-07):
  thành viên chia vào các thế lực tiên còn mở lúc đó, báo trước 3 ngày, 7 ngày tự chọn.
- `cot-truyen.md` không kể số phận Tiên Đình sau Đại chiến. "Cầu sương đứt" là chi tiết văn của dự án.
- Sổ tiên chỉ là chi tiết văn học. Nó **không** phải bảng xếp hạng riêng và không thay Thiên Bảng.
- Thế lực chưa cộng chỉ số gì.
