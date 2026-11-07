---
id: mo_dau_tu_tieu_cung
kind: onboarding
title: Mở đầu — Từ hỗn độn đến Tử Tiêu Cung
tags: [onboarding, opening, tu_tieu_cung, hong_quan]
status: draft
refs: [huyen_khung_mon, kim_sa_mon, thuong_lan_dinh, tinh_ha_yeu_dinh, vu_toc]
---

# Mở đầu: từ hỗn độn đến Tử Tiêu Cung

Luồng người mới (data model §9.6): **video mở đầu → Tử Tiêu Cung → chọn thế lực → (xác nhận đạo đồ) → đặt đạo hiệu**.
Video thật chưa làm. Web tạm chiếu 8 cảnh dưới đây thành ảnh tĩnh có phụ đề, bỏ qua được (`docs/design/cot-truyen.md`
§10). Mỗi phụ đề ≤ 140 ký tự. Bản sao trong web: `apps/web/src/lib/game/onboarding.ts`, sửa hai nơi cùng lúc
(test web so khớp từng câu với file này).

## Tám cảnh mở đầu

| # | Tên cảnh | Phụ đề |
|---|---|---|
| 1 | Hỗn Độn | Thuở chưa có trời đất, chỉ có hỗn độn mịt mùng. Giữa lòng hỗn độn, Bàn Cổ ngủ một giấc dài không ai đếm được. |
| 2 | Bàn Cổ khai thiên | Bàn Cổ tỉnh dậy, vung rìu bổ hỗn độn. Khí trong nhẹ bay lên làm trời, khí đục nặng chìm xuống làm đất. |
| 3 | Bàn Cổ vẫn lạc | Bàn Cổ ngã xuống. Thân hóa núi sông, mắt hóa nhật nguyệt, nguyên thần hóa Tam Thanh, tinh huyết hóa Tổ Vu. |
| 4 | Thiên địa sơ khai | Linh khí tràn khắp Hồng Hoang, chảy thành sông ngầm dưới núi. Cỏ cây mọc lên, muông thú sinh sôi. |
| 5 | Long Hán Sơ Kiếp | Long, Phượng, Kỳ Lân tranh nhau làm chủ đất trời. Chiến hỏa kéo dài, rồi cả ba cùng suy tàn. |
| 6 | Các đại năng xuất hiện | Tam Thanh, Nữ Oa, Chuẩn Đề, Tiếp Dẫn, Đông Vương Công, Đế Tuấn, Thái Nhất, Hậu Thổ lần lượt hiện thế giữa trời đất. |
| 7 | Hồng Quân hiện thế | Hồng Quân Đạo Tổ bước ra từ nơi sâu nhất của hỗn độn, ban lệnh mời các đại năng khắp Hồng Hoang: ai muốn nghe đạo, hãy đến. |
| 8 | Tử Tiêu Cung mở | Cửa Tử Tiêu Cung mở. Hồng Quân ngồi lên đạo đài. Giữa những người nghe đầu tiên, có ngươi. |

## Lời mở đầu của Hồng Quân

Hồng Quân nói từng câu, người chơi chạm để nghe câu tiếp (bỏ qua được).

1. Các ngươi đến từ khắp Hồng Hoang. Đường xa rồi, hãy ngồi xuống, thở chậm lại.
2. Ta là Hồng Quân. Ta không ban đạo cho ai, vì đạo chưa từng thuộc về ta.
3. Ta chỉ giảng những gì ta đã thấy. Nghe hay không, đi hay ở, là việc của các ngươi.
4. Lời giảng ở đây không dứt. Hôm nay lỡ một câu, ngày mai câu ấy sẽ lại vòng về.
5. Ngoài điện, các thế lực đã mở cửa. Mỗi nhà một lối đi, không lối nào cao hơn lối nào.
6. Vu Tộc rèn thân bằng máu Bàn Cổ, các nhà khác tu linh khí. Hãy chọn nơi lòng ngươi thấy yên.
7. Rồi để lại một đạo hiệu. Thiên Bảng sẽ khắc nó, nhưng chính ngươi mới làm nó có nghĩa.
8. Chọn xong thì quay lại đây ngồi. Bài giảng đầu tiên đang chờ ngươi.

## Ghi chú thiết kế

- **Theo cot-truyen §10 (D2 đảo ngược 2026-11-07).** Tên tám cảnh và các đại năng lấy đúng bảng video mở đầu:
  Bàn Cổ vẫn lạc (nguyên thần hóa Tam Thanh, tinh huyết hóa Tổ Vu), Long Hán Sơ Kiếp, các đại năng Tam Thanh, Nữ
  Oa, Chuẩn Đề, Tiếp Dẫn, Đông Vương Công, Đế Tuấn, Thái Nhất, Hậu Thổ. Thế giới là Hồng Hoang. Video dừng trước
  khi các thế lực kiến lập, đúng §10. Các đại năng chỉ được nêu tên, không có lời cầu khấn hay hình tượng thờ cúng.
- Hồng Quân là người thầy, không phải đối tượng thờ phụng: không lạy, không cầu, nói rõ đạo không thuộc về mình.
- Không hứa cơ chế chưa có. Câu 4 ứng với lời giảng xoay vòng cả ngày (quyết định 2026-11-07). Câu 5 ứng với
  "thế lực không cộng sức mạnh". Câu 6 ứng với đạo đồ đi theo thế lực (Vu Tộc → đạo đồ Vu, Phase 2.2).
- Không nhắc Tiên Duyên hay nạp tiền.
