---
id: milestone_effects
kind: world
title: Thời kỳ và biến động thế lực theo mốc cốt truyện
tags: [world, milestones, eras, factions, split]
status: draft
refs: [milestone.khai_giang, milestone.giang_dao_2, milestone.giang_dao_3, milestone.dai_chien, milestone.nu_oa,
       milestone.thanh_nhan, milestone.nhan_toc, milestone.phong_than, milestone.thien_dinh, milestone.tay_du,
       faction.huyen_khung_mon, faction.kim_sa_mon, faction.thuong_lan_dinh, faction.tinh_ha_yeu_dinh, faction.vu_toc]
---

# Thời kỳ và biến động thế lực

Trang này là nguồn tên chính thức cho `eras` và các hiệu ứng thế lực trong `world_milestones.yaml` /
`factions.yaml` (data model §5.2, §6.1). Mốc và ngày mục tiêu xem `timeline/hong-hoang-timeline.md`.

## 1. Thời kỳ (`eras`)

Tên theo `docs/design/cot-truyen.md` §6–7 (chủ dự án đảo ngược D2 ngày 2026-11-07).

| Thứ tự | `id` | Tên hiển thị | Bắt đầu khi | Nguồn tên |
|---|---|---|---|---|
| 1 | `khai_giang` | Tử Tiêu khai giảng | mốc `khai_giang` | §4, §6.2: Hồng Quân giảng đạo ở Tử Tiêu Cung |
| 2 | `dai_chien` | Đại chiến Vu Yêu | mốc `dai_chien` | §6.2, §7.1 |
| 3 | `luan_hoi` | Hậu Thổ hóa luân hồi | hết sự kiện `dai_chien` | §7.1: Hậu Thổ kiến lập Lục Đạo Luân Hồi và Địa Phủ |
| 4 | `tao_nhan` | Nữ Oa tạo người | mốc `nu_oa` | §6.2 |
| 5 | `thanh_nhan` | Thánh nhân xuất thế | mốc `thanh_nhan` | §6.2, §7.2 |
| 6 | `nhan_toc` | Tam Hoàng Ngũ Đế | mốc `nhan_toc` | §6.2: phe Nhân tộc, Tam Hoàng Ngũ Đế |
| 7 | `phong_than` | Phong Thần Lượng Kiếp | mốc `phong_than` | §6.2, §7.3 |
| 8 | `thien_dinh` | Thiên Đình tái lập | mốc `thien_dinh` | §6.2: Thiên Đình mới, không phải Thiên Đình Yêu Tộc |
| 9 | `tay_du` | Tây Du | mốc `tay_du` | chỉ là tên thời kỳ, chưa có nội dung (G7) |

Những người thành thánh ở mốc `thanh_nhan` là các NPC Tam Thanh, Chuẩn Đề, Tiếp Dẫn và Nữ Oa (§7.2). Giữ tông tôn
trọng: họ là bậc thầy trong truyện, không phải đối tượng để cầu khấn hay chế giễu.

## 2. Vòng giảng ở Tử Tiêu Cung

Lời giảng đã xoay vòng cả ngày (quyết định 2026-11-07), nên mốc 2 và 3 không còn là "buổi giảng" mà là **mở vòng
giảng mới** chồng lên vòng cũ:

| Mốc | Vòng mở thêm | Trạng thái kịch bản |
|---|---|---|
| `khai_giang` | `tu_tieu_cycle_1` (8 bài hiện có trong `lectures/`) | bản nháp xong |
| `giang_dao_2` | `tu_tieu_cycle_2` | viết sau |
| `giang_dao_3` | `tu_tieu_cycle_3` | viết sau |

Không có "lần giảng cuối". Hồng Quân không rời Tử Tiêu Cung; từ mốc 3 trở đi, đạo pháp vừa nghe trong điện vừa
được các thế lực truyền lại.

## 3. Thế lực ở mốc `thanh_nhan`

### 3.1 Slug giữ nguyên, tên theo cot-truyen

Slug là ID vĩnh viễn, đặt từ bản tên gốc trước khi D2 bị đảo ngược. Không đổi slug, chỉ đổi tên hiển thị.

| Giữ chỗ trong data model | Slug (giữ nguyên) | Tên hiển thị |
|---|---|---|
| `huyen_mon` | `huyen_khung_mon` | Tam Thanh |
| `huyen_mon_a` / `_b` / `_c` | `thu_nhat_vien` / `lap_cuc_vien` / `van_luu_vien` | Nhân Giáo / Xiển Giáo / Tiệt Giáo |
| `tay_phuong_mon` | `kim_sa_mon` | Tây Phương (phe Chuẩn Đề) |
| `tay_phuong_giao` | `kim_sa_giao` | Tây Phương Giáo |
| `dong_tien_dinh` | `thuong_lan_dinh` | Tiên Đình (Đông Vương Công) |
| `yeu_dinh` | `tinh_ha_yeu_dinh` | Thiên Đình Yêu Tộc |
| `vu_toc` | `vu_toc` | Vu Tộc |

### 3.2 Tam Thanh tách ba giáo (`split_faction`)

Có Nhân tộc, các Thánh nhân mới có cớ lập giáo (§7.2). Tam Thanh tách thành ba giáo; người chơi chọn một trong ba,
giữ nguyên tu vi. Không giáo nào "đúng" hơn giáo nào. Cách gán slug theo bản sắc cũ của từng nhánh:

| Slug | Tên | Vị đứng đầu | Một dòng bản sắc | Màu huy hiệu |
|---|---|---|---|---|
| `thu_nhat_vien` | Nhân Giáo | Thái Thanh Lão Tử | Thanh tĩnh vô vi: ít lời, bế quan dài ngày. | `jade-300` |
| `lap_cuc_vien` | Xiển Giáo | Nguyên Thủy Thiên Tôn | Đạo cần cột mốc: giới luật rõ, truyền thụ có thứ bậc. | `jade-500` |
| `van_luu_vien` | Tiệt Giáo | Thông Thiên Giáo Chủ | Hữu giáo vô loại: mở cửa cho mọi người, mọi xuất thân. | `jade-700` |

Huy hiệu mỗi giáo giữ khung tròn `gold-500` của Tam Thanh nhưng chỉ còn một cung tròn tương ứng.

### 3.3 Tây Phương thành Tây Phương Giáo (`migrate_faction`)

Chuẩn Đề và Tiếp Dẫn thành thánh, lời nguyện nơi đất tây được lập thành giáo. Phe đổi tên thành **Tây Phương
Giáo** (`kim_sa_giao`): cùng đệ tử, cùng tâm ngôn, cùng huy hiệu, chỉ đổi tên (§7.2). Không chép kinh văn, không
dùng biểu tượng thờ cúng.

### 3.4 Các thế lực khác

- **Thiên Đình Yêu Tộc**, **Vu Tộc**: không đổi ở mốc này. Số phận sau Đại chiến Vu Yêu (Địa Phủ hoặc luân hồi,
  §7.1) chờ G2/G4.
- **Tiên Đình**: đóng từ `luan_hoi` (cầu sương đứt sau Đại chiến). Thành viên chia vào các thế lực tiên còn mở lúc
  đó, cùng luật báo trước/chọn (quyết định 2026-11-07).
- **Oa Hoàng Cung** (phe Nữ Oa, §7.2) và **Địa Phủ** (§4, §7.1) chưa có thế lực trong catalogue.

## Ghi chú thiết kế

- `status: draft`. Mọi hiệu ứng ghi "(đề xuất)" cần `data-architect` thêm vào `world_milestones.yaml`:
  `set_era` cho `nhan_toc`, `thien_dinh`, `tay_du`; và một cách đặt `luan_hoi` (hiện không mốc nào đặt thời kỳ này,
  mà Tiên Đình `thuong_lan_dinh` lại đóng từ `luan_hoi`).
- `split_faction`: `from: huyen_khung_mon`, `into: [thu_nhat_vien, lap_cuc_vien, van_luu_vien]` (thứ tự này là thứ
  tự hòa khi chia mặc định), `notice_days: 3`, `choose_days: 7` (quyết định 9). Tu vi giữ nguyên.
- `migrate_faction`: `from: kim_sa_mon`, `to: kim_sa_giao`, `notice_days: 3`.
- Ba viện và `kim_sa_giao`: `allowed_paths: [tien]`, `open_from_era: thanh_nhan`. `huyen_khung_mon`, `kim_sa_mon`:
  `close_from_era: thanh_nhan`.
- Vòng giảng 2/3: chưa có effect kind cho việc mở vòng giảng. Đề xuất `lectures.yaml` khai báo
  `cycles: [{ id, unlock_milestone }]` để không phải thêm effect kind. Sau khi mở, các vòng nối nhau thành một
  vòng dài; trong ngày sự kiện (`duration_days: 1`) có thể phát riêng vòng mới. Thưởng ngộ đạo không đổi.
- Tiên Đình `thuong_lan_dinh` (đã chốt 2026-11-07): `factions.yaml` `on_close: split` — chia khi thời kỳ chạm `luan_hoi`, nhánh tính
  từ catalogue (mở ở `luan_hoi`, cho đường `tien`, không đóng ở đó), `notice_days: 3`, `choose_days: 7`.
