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

| Thứ tự | `id` | Tên hiển thị | Bắt đầu khi | Ghi chú D2 |
|---|---|---|---|---|
| 1 | `khai_giang` | Tử Tiêu khai giảng | mốc `khai_giang` | Hồng Quân, Tử Tiêu Cung: thần thoại, giữ tên |
| 2 | `dai_chien` | Đại chiến Huyết Tinh | mốc `dai_chien` | thay khung "Vu Yêu đại chiến" |
| 3 | `luan_hoi` | Luân hồi sơ chuyển | hết sự kiện `dai_chien` (đề xuất) | không gắn người lập luân hồi |
| 4 | `tao_nhan` | Nữ Oa tạo người | mốc `nu_oa` | Nữ Oa: thần thoại, giữ tên |
| 5 | `thanh_nhan` | Thánh vị định tọa | mốc `thanh_nhan` | thay "Tam Thanh xuất thế" |
| 6 | `nhan_toc` | Nhân tộc hưng khởi | mốc `nhan_toc` (đề xuất) | Tam Hoàng Ngũ Đế: cổ sử, giữ tên |
| 7 | `phong_than` | Phong Thần lượng kiếp | mốc `phong_than` | Phong Thần Diễn Nghĩa: hết bản quyền |
| 8 | `thien_dinh` | Thiên Đình tái lập | mốc `thien_dinh` (đề xuất) | Thiên Đình mới, không phải Tinh Hà Yêu Đình |
| 9 | `tay_du` | Tây Du | mốc `tay_du` (đề xuất) | chỉ là tên thời kỳ, chưa có nội dung (G7) |

"Thánh vị định tọa" nghĩa là các bậc Thánh Nhân ngồi vào ghế của mình. Trong truyện, những người thành thánh là NPC
gốc (Tam Tọa Khung Chủ, Kim Sa Song Chủ) cùng Nữ Oa, không ai là nhân vật được thờ phụng.

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

### 3.1 Đổi ID giữ chỗ sang ID thật

| Giữ chỗ trong data model | ID thật | Tên hiển thị |
|---|---|---|
| `huyen_mon` | `huyen_khung_mon` | Huyền Khung Môn |
| `huyen_mon_a` / `_b` / `_c` | `thu_nhat_vien` / `lap_cuc_vien` / `van_luu_vien` | xem 3.2 |
| `tay_phuong_mon` | `kim_sa_mon` | Kim Sa Môn |
| `tay_phuong_giao` | `kim_sa_giao` | Kim Sa Giáo |
| `dong_tien_dinh` | `thuong_lan_dinh` | Thương Lan Tiên Đình |
| `yeu_dinh` | `tinh_ha_yeu_dinh` | Tinh Hà Yêu Đình |
| `vu_toc` | `vu_toc` | Vu Tộc |

### 3.2 Huyền Khung Môn chia ba viện (`split_faction`)

Ba tòa đạo đài vốn chung một sơn môn. Khi các Khung Chủ thành thánh, vòm trời rạn thành ba cung, mỗi tòa thành
một viện độc lập. Không viện nào "đúng" hơn viện nào; mỗi viện giữ một cách hiểu về cùng một nguồn đạo.

| `id` | Tên | Một dòng bản sắc | Màu huy hiệu |
|---|---|---|---|
| `thu_nhat_vien` | Thủ Nhất Viện | Giữ một điều cho trọn: tĩnh tu, ít lời, bế quan dài ngày. | `jade-300` |
| `lap_cuc_vien` | Lập Cực Viện | Đạo cần cột mốc: giới luật rõ, truyền thụ có thứ bậc. | `jade-500` |
| `van_luu_vien` | Vạn Lưu Viện | Trăm dòng cùng về biển: mở cửa cho mọi người, mọi xuất thân. | `jade-700` |

Huy hiệu mỗi viện giữ khung tròn `gold-500` của Huyền Khung Môn nhưng chỉ còn một cung tròn tương ứng.

### 3.3 Kim Sa Môn thành Kim Sa Giáo (`migrate_faction`)

Hai vị Song Chủ thành thánh, lời nguyện nơi Tây Mạc được ghi thành giáo điều riêng. Môn đổi tên thành
**Kim Sa Giáo** (`kim_sa_giao`): cùng đệ tử, cùng tâm ngôn, cùng huy hiệu, chỉ đổi tên. Không dùng tên, biểu tượng
hay giáo lý của tôn giáo có thật nào.

### 3.4 Các thế lực khác

- **Tinh Hà Yêu Đình**, **Vu Tộc**: không đổi ở mốc này. Số phận sau Đại chiến chờ G2/G4.
- **Thương Lan Tiên Đình**: đóng từ `luan_hoi` (cầu sương đứt sau Đại chiến). Thành viên chia vào các thế lực tiên
  còn mở lúc đó, cùng luật báo trước/chọn (quyết định 2026-11-07).

## Ghi chú thiết kế

- `status: draft`. Mọi hiệu ứng ghi "(đề xuất)" cần `data-architect` thêm vào `world_milestones.yaml`:
  `set_era` cho `nhan_toc`, `thien_dinh`, `tay_du`; và một cách đặt `luan_hoi` (hiện không mốc nào đặt thời kỳ này,
  mà `thuong_lan_dinh` lại đóng từ `luan_hoi`).
- `split_faction`: `from: huyen_khung_mon`, `into: [thu_nhat_vien, lap_cuc_vien, van_luu_vien]` (thứ tự này là thứ
  tự hòa khi chia mặc định), `notice_days: 3`, `choose_days: 7` (quyết định 9). Tu vi giữ nguyên.
- `migrate_faction`: `from: kim_sa_mon`, `to: kim_sa_giao`, `notice_days: 3`.
- Ba viện và `kim_sa_giao`: `allowed_paths: [tien]`, `open_from_era: thanh_nhan`. `huyen_khung_mon`, `kim_sa_mon`:
  `close_from_era: thanh_nhan`.
- Vòng giảng 2/3: chưa có effect kind cho việc mở vòng giảng. Đề xuất `lectures.yaml` khai báo
  `cycles: [{ id, unlock_milestone }]` để không phải thêm effect kind. Sau khi mở, các vòng nối nhau thành một
  vòng dài; trong ngày sự kiện (`duration_days: 1`) có thể phát riêng vòng mới. Thưởng ngộ đạo không đổi.
- Thương Lan (đã chốt 2026-11-07): `factions.yaml` `on_close: split` — chia khi thời kỳ chạm `luan_hoi`, nhánh tính
  từ catalogue (mở ở `luan_hoi`, cho đường `tien`, không đóng ở đó), `notice_days: 3`, `choose_days: 7`.
