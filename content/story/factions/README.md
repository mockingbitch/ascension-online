# Thế lực

Nguồn để dựng `content/catalog/factions.yaml` (data model §6.1). Mỗi file `<faction_id>.md` trong thư mục này là
trang truyện của thế lực đó; tên file trùng `id`. Biến động theo mốc (chia viện, đổi tên, đóng cửa) xem
`../world/milestone-effects.md`.

**Trạng thái: draft.** Mọi tên dưới đây đã qua rà D2 nhưng chưa được chủ dự án duyệt cuối (quyết định 12:
ID vĩnh viễn phải chốt trước lần deploy đầu tiên). Khi dựng YAML, giữ `name_placeholder: true` cho đến khi duyệt.

## Năm thế lực khởi đầu (`open_from_era: khai_giang`)

| `id` | Tên hiển thị | Đường tu | Giới thiệu (≤ 80 ký tự) | Màu huy hiệu | Đóng từ |
|---|---|---|---|---|---|
| `huyen_khung_mon` | Huyền Khung Môn | `tien` | Tông môn dưới núi Tử Tiêu, ba tòa đạo đài chờ ngày phân nhánh. | `--color-aura-jade` | `thanh_nhan` (chia ba viện) |
| `kim_sa_mon` | Kim Sa Môn | `tien` | Tông môn giữa cát vàng Tây Mạc, tu chậm mà bền, không hỏi xuất thân. | `--color-aura-golden` | `thanh_nhan` (thành Kim Sa Giáo) |
| `thuong_lan_dinh` | Thương Lan Tiên Đình | `tien` | Triều đình tu sĩ trên đảo nổi biển Đông, giữ Tiên Tịch và lễ nghi. | `--color-spirit-cyan` | `luan_hoi` (chia vào các thế lực tiên còn mở) |
| `tinh_ha_yeu_dinh` | Tinh Hà Yêu Đình | `tien` ⚠ | Triều đình Yêu tộc giữa ngân hà, muôn loài tu thành hình đều có sao. | `--color-spirit-violet` | — |
| `vu_toc` | Vu Tộc | `vu` | Tộc luyện thể mang máu Bàn Cổ, rèn thân như núi, giữ lời như đá. | `--color-aura-blood` | — |

Màu là token trong `apps/web/src/app.css`, dùng trong `content/catalog/factions.yaml` (`color`). Huy hiệu SVG mô tả trong
từng file thế lực (mục "Huy hiệu").

## Thế lực mở sau (theo mốc)

| `id` | Tên hiển thị | Đường tu | Mở từ | Nguồn | Màu huy hiệu |
|---|---|---|---|---|---|
| `thu_nhat_vien` | Thủ Nhất Viện | `tien` | `thanh_nhan` | chia từ `huyen_khung_mon` | `--color-jade-300` |
| `lap_cuc_vien` | Lập Cực Viện | `tien` | `thanh_nhan` | chia từ `huyen_khung_mon` | `--color-jade-500` |
| `van_luu_vien` | Vạn Lưu Viện | `tien` | `thanh_nhan` | chia từ `huyen_khung_mon` | `--color-jade-700` |
| `kim_sa_giao` | Kim Sa Giáo | `tien` | `thanh_nhan` | đổi tên từ `kim_sa_mon` | `--color-aura-golden` |
| `nhan_toc` | Nhân Tộc | `tien` | `tao_nhan` (mốc `nu_oa`) | quyết định 2026-11-07 | `--color-aura-earth` ⚠ |

## Đã chốt (2026-11-07)

- **Thương Lan Tiên Đình** đóng từ `luan_hoi` (Đại chiến kết thúc). Thành viên được chia vào các thế lực tiên còn mở
  lúc đó (tính từ `factions.yaml`: hiện là Huyền Khung Môn, Kim Sa Môn, Tinh Hà Yêu Đình), cùng luật quyết định 9:
  báo trước 3 ngày, 7 ngày tự chọn, ai không chọn vào nhánh ít người nhất.

## Còn nháp (⚠)

- **Tinh Hà Yêu Đình** tạm dùng đường tu `tien`; đường tu Yêu (G3) chưa quyết. Không hứa cơ chế hóa hình.
- **Nhân Tộc** chưa có trang truyện. Đã chốt (2026-11-07): mở từ `tao_nhan` (mốc `nu_oa`); thẻ dòng thời gian số 7
  chỉ kể Nhân tộc hưng thịnh. Màu `aura-earth` là đề xuất.
- Ba viện và Kim Sa Giáo chưa có trang riêng; bản sắc một dòng nằm trong `../world/milestone-effects.md`.
- Không thế lực nào cộng chỉ số. Mọi "nét riêng" chỉ là văn mô tả.
