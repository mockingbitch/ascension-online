# Thế lực

Nguồn để dựng `content/catalog/factions.yaml` (data model §6.1). Mỗi file `<faction_id>.md` trong thư mục này là
trang truyện của thế lực đó; tên file trùng `id`. Biến động theo mốc (chia viện, đổi tên, đóng cửa) xem
`../world/milestone-effects.md`.

**Trạng thái: draft.** Tên theo `docs/design/cot-truyen.md` (chủ dự án đảo ngược D2 ngày 2026-11-07, xem
`docs/decisions.md`). ID là slug cũ của bản tên gốc trước đây và **không đổi** (ID vĩnh viễn, quyết định 12), nên slug
không còn trùng tên hiển thị. Bảng đối chiếu slug → tên nằm ở đầu `content/catalog/factions.yaml`.

## Năm thế lực khởi đầu (`open_from_era: khai_giang`)

| `id` | Tên hiển thị | Đường tu | Giới thiệu (≤ 80 ký tự) | Màu huy hiệu | Đóng từ |
|---|---|---|---|---|---|
| `huyen_khung_mon` | Tam Thanh | `tien` | Môn hạ Tam Thanh nơi Côn Lôn, nghe đạo Tử Tiêu, chờ ngày phân giáo. | `--color-aura-jade` | `thanh_nhan` (tách ba giáo) |
| `kim_sa_mon` | Tây Phương | `tien` | Phe Chuẩn Đề nơi Tây phương cằn cỗi, tu chậm mà bền, không hỏi xuất thân. | `--color-aura-golden` | `thanh_nhan` (thành Tây Phương Giáo) |
| `thuong_lan_dinh` | Tiên Đình | `tien` | Tiên Đình của Đông Vương Công trên biển Đông, giữ sổ tiên và trọng lễ nghi. | `--color-spirit-cyan` | `luan_hoi` (chia vào các thế lực tiên còn mở) |
| `tinh_ha_yeu_dinh` | Thiên Đình Yêu Tộc | `tien` ⚠ | Thiên Đình của Yêu tộc trên chín tầng trời, Đế Tuấn và Thái Nhất cầm đầu. | `--color-spirit-violet` | — |
| `vu_toc` | Vu Tộc | `vu` | Tộc luyện thể do tinh huyết Bàn Cổ hóa thành, Tổ Vu cầm đầu, rèn thân như núi. | `--color-aura-blood` | — |

Màu là token trong `apps/web/src/app.css`, dùng trong `content/catalog/factions.yaml` (`color`). Huy hiệu SVG mô tả trong
từng file thế lực (mục "Huy hiệu").

## Thế lực mở sau (theo mốc)

| `id` | Tên hiển thị | Đường tu | Mở từ | Nguồn | Màu huy hiệu |
|---|---|---|---|---|---|
| `thu_nhat_vien` | Nhân Giáo (Thái Thanh Lão Tử) | `tien` | `thanh_nhan` | tách từ Tam Thanh | `--color-jade-300` |
| `lap_cuc_vien` | Xiển Giáo (Nguyên Thủy Thiên Tôn) | `tien` | `thanh_nhan` | tách từ Tam Thanh | `--color-jade-500` |
| `van_luu_vien` | Tiệt Giáo (Thông Thiên Giáo Chủ) | `tien` | `thanh_nhan` | tách từ Tam Thanh | `--color-jade-700` |
| `kim_sa_giao` | Tây Phương Giáo | `tien` | `thanh_nhan` | đổi tên từ Tây Phương | `--color-aura-golden` |
| `nhan_toc` | Nhân Tộc | `tien` | `tao_nhan` (mốc `nu_oa`) | quyết định 2026-11-07 | `--color-aura-earth` ⚠ |

## Đã chốt (2026-11-07)

- **Tiên Đình** đóng từ `luan_hoi` (Đại chiến Vu Yêu kết thúc). Thành viên được chia vào các thế lực tiên còn mở
  lúc đó (tính từ `factions.yaml`: hiện là Tam Thanh, Tây Phương, Thiên Đình Yêu Tộc), cùng luật quyết định 9:
  báo trước 3 ngày, 7 ngày tự chọn, ai không chọn vào nhánh ít người nhất.

## Còn nháp (⚠)

- **Thiên Đình Yêu Tộc** tạm dùng đường tu `tien`; đường tu Yêu (G3) chưa quyết. Không hứa cơ chế hóa hình.
- **Nhân Tộc** chưa có trang truyện. Đã chốt (2026-11-07): mở từ `tao_nhan` (mốc `nu_oa`); thẻ dòng thời gian số 7
  chỉ kể Nhân tộc hưng thịnh. Màu `aura-earth` là đề xuất.
- Ba giáo (Nhân, Xiển, Tiệt) và Tây Phương Giáo chưa có trang riêng; bản sắc một dòng nằm trong `../world/milestone-effects.md`.
- Không thế lực nào cộng chỉ số. Mọi "nét riêng" chỉ là văn mô tả.
- **Oa Hoàng Cung** (phe của Nữ Oa, `cot-truyen.md` §7.2) chưa có thế lực trong catalogue; thẻ dòng thời gian chỉ
  nhắc tên.
- Tam Thanh, Chuẩn Đề, Tiếp Dẫn là nhân vật được thờ phụng thật: giữ tông tôn trọng (`cot-truyen.md` §8). Nên rà
  pháp lý/tín ngưỡng trước khi mở công khai.
