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
| 3 | Thân hóa vạn vật | Bàn Cổ ngã xuống. Thân hóa núi sông, mắt hóa nhật nguyệt, tinh huyết thấm vào đất. Thiên Nguyên Giới thành hình. |
| 4 | Thiên địa sơ khai | Linh khí tràn khắp đại lục, chảy thành sông ngầm dưới núi. Cỏ cây mọc lên, muông thú sinh sôi. |
| 5 | Ba tộc tranh bá | Long, Phượng, Kỳ Lân tranh nhau làm chủ đất trời. Chiến hỏa kéo dài, rồi cả ba cùng suy tàn. |
| 6 | Đại năng hiện thế | Từ đổ nát, các đại năng lần lượt hiện thân: dưới núi Tử Tiêu, giữa cát vàng Tây Mạc, trên biển Đông, giữa ngân hà, trong máu Bàn Cổ. |
| 7 | Hồng Quân hiện thế | Hồng Quân bước ra từ nơi sâu nhất của hỗn độn còn sót lại, gửi lời mời khắp Thiên Nguyên Giới: ai muốn nghe đạo, hãy đến. |
| 8 | Tử Tiêu Cung mở cửa | Cửa Tử Tiêu Cung mở. Hồng Quân ngồi lên đạo đài. Giữa những người nghe đầu tiên, có ngươi. |

## Lời mở đầu của Hồng Quân

Hồng Quân nói từng câu, người chơi chạm để nghe câu tiếp (bỏ qua được).

1. Các ngươi đến từ khắp Thiên Nguyên Giới. Đường xa rồi, hãy ngồi xuống, thở chậm lại.
2. Ta là Hồng Quân. Ta không ban đạo cho ai, vì đạo chưa từng thuộc về ta.
3. Ta chỉ giảng những gì ta đã thấy. Nghe hay không, đi hay ở, là việc của các ngươi.
4. Lời giảng ở đây không dứt. Hôm nay lỡ một câu, ngày mai câu ấy sẽ lại vòng về.
5. Ngoài điện, các thế lực đã mở cửa. Mỗi nhà một lối đi, không lối nào cao hơn lối nào.
6. Vu Tộc rèn thân bằng máu Bàn Cổ, các nhà khác tu linh khí. Hãy chọn nơi lòng ngươi thấy yên.
7. Rồi để lại một đạo hiệu. Thiên Bảng sẽ khắc nó, nhưng chính ngươi mới làm nó có nghĩa.
8. Chọn xong thì quay lại đây ngồi. Bài giảng đầu tiên đang chờ ngươi.

## Ghi chú thiết kế

- **D2.** Cảnh 3 và 6 của bản nháp gốc nhắc Tam Thanh, Chuẩn Đề, Tiếp Dẫn và các danh xưng của tiểu thuyết mạng
  (Tổ Vu, Long Hán sơ kiếp). Ở đây chỉ còn Bàn Cổ, Long, Phượng, Kỳ Lân, Hồng Quân (thần thoại). Các đại năng được
  gợi qua nơi chốn của năm thế lực khởi đầu (`content/story/factions/`), không nêu tên. "Tinh huyết thấm vào đất"
  khớp với Vu Tộc "mang máu Bàn Cổ". Video dừng trước khi các thế lực kiến lập, đúng §10.
- Hồng Quân là người thầy, không phải đối tượng thờ phụng: không lạy, không cầu, nói rõ đạo không thuộc về mình.
- Không hứa cơ chế chưa có. Câu 4 ứng với lời giảng xoay vòng cả ngày (quyết định 2026-11-07). Câu 5 ứng với
  "thế lực không cộng sức mạnh". Câu 6 ứng với đạo đồ đi theo thế lực (Vu Tộc → đạo đồ Vu, Phase 2.2).
- Không nhắc Tiên Duyên hay nạp tiền.
