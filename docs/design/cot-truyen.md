# **Kế hoạch dự án: Tu Tiên Hồng Hoang Online** 

Bản kế hoạch ý tưởng · Cập nhật 06/11/2026 

## **1. Tổng quan** 

**Tu Tiên Hồng Hoang Online** là một ứng dụng/web tu luyện trực tuyến, lấy cảm hứng từ trào lưu "đi chùa online" nhưng đặt trong bối cảnh thần thoại Hồng Hoang. Người chơi nghe Hồng Quân giảng đạo, gia nhập một thế lực, tu luyện trong động phủ riêng và cùng cộng đồng đẩy cốt truyện tiến triển qua các thời kỳ lớn của Hồng Hoang, từ Tử Tiêu Cung đến Tây Du. 

## **2. Bối cảnh và cảm hứng** 

Trào lưu "đi chùa online" nổi lên gần đây tại Việt Nam. Theo báo chí, nó khởi nguồn từ một website ở Hàn Quốc (Sansa). Các app và web bản Việt cho phép người dùng thực hiện thao tác nghi lễ ảo như thắp hương, tụng kinh, gõ mõ, cầu bình an, có app còn có gieo quẻ. Một số app đã vượt 10.000 lượt tải trên Google Play, có thời điểm gần 3.000 người cùng truy cập web vào lúc 23h, và có người dành hơn 2 tiếng mỗi ngày. 

Trào lưu cũng bộc lộ các vấn đề mà dự án này cần tránh ngay từ thiết kế: 

- Tên tài khoản dung tục, phản cảm, thiếu tôn trọng. 

- Biến tướng trục lợi, nhất là các livestream hứa hẹn nhận lộc, tặng quà ảo. Công an đã có cảnh báo, dù bản thân việc đi chùa online hay livestream tâm linh không vi phạm pháp luật. 

- Nguy cơ lộ dữ liệu cá nhân khi dùng các app này. 

## **3. Ý tưởng cốt lõi và điểm khác biệt** 

- Chủ đề tu tiên Hồng Hoang là thể loại truyện/thần thoại hư cấu, cho phép game hóa thoải mái hơn so với mô hình "chùa". 

- Có hệ thống cảnh giới, thế lực và cốt truyện chung của cả server, thay cho điểm công đức đơn thuần. 

- Có yếu tố treo máy: tu vi tích lũy theo thời gian, giúp người chơi có lý do quay lại mỗi ngày. 

- Có phòng chung thời gian thực, thể hiện cộng đồng cùng tu luyện. 

- Có mini game: bí cảnh thử luyện để kiếm tài nguyên và các thử thách đột phá (tâm ma, hóa phàm). 

- Có đạo lữ song tu: cả hai cùng online thì tăng hiệu quả tu luyện, kết trọn đời. 

- Không thu bất kỳ phí nào; vật phẩm tự kiếm được có thể chia sẻ và trao đổi giữa người chơi. 

## **4. Mở đầu và các thế lực** 

**Mở đầu:** mọi người chơi bắt đầu ở Tử Tiêu Cung, nơi Hồng Quân giảng đạo. Trước đó, người chơi xem video mở đầu kể lịch sử Hồng Hoang (xem mục 10). Sau màn mở đầu, người chơi chọn một thế lực để gia nhập. 

#### **Các thế lực ban đầu:** 

- **Tam Thanh:** Thái Thanh Lão Tử, Nguyên Thủy Thiên Tôn, Thông Thiên Giáo Chủ (theo hệ Hồng Hoang và Phong Thần). 

- **Chuẩn Đề** (phe Tây phương). 

- **Tiên Đình** của Đông Vương Công. 

- **Thiên Đình** của Yêu tộc. 

- **Vu tộc.** 

Sau Đại chiến Vu Yêu sẽ xuất hiện thêm phe **Nhân tộc** do Tam Hoàng Ngũ Đế cầm đầu và phe **Địa Phủ** (xem mục 7). 

Trong thế lực, người chơi nghe giảng đạo và tu luyện trong động phủ riêng của mình. 

## **5. Hệ thống cảnh giới** 

### **5.1. Phe tiên** 

Luyện khí, trúc cơ, kim đan, nguyên anh, hóa thần, luyện hư, hợp thể, đại thừa, độ kiếp, nhân tiên, địa tiên, kim tiên, thái ất kim tiên, đại la kim tiên, chuẩn thánh, thánh nhân. 

**Quy tắc đỉnh cao:** Chuẩn Thánh là đích thực tế mà người chơi có thể đạt tới, trừ người đã lên Phong Thần Bảng chỉ tu luyện được đến Thái Ất Kim Tiên (xem mục 7.4). Thánh nhân là vị trí giới hạn, mở theo sự kiện hoặc dành cho nhân vật NPC, để giữ cảm giác hiếm có như trong nguyên tác. 

### **5.2. Lôi kiếp** 

Từ Hóa Thần trở đi, mỗi lần đột phá sang cảnh giới tiếp theo đều phải vượt lôi kiếp. Vượt qua thì bước sang cảnh giới mới, thất bại thì bị tụt tu vi. 

- **Thất bại:** mất khoảng 25% tu vi tích lũy trong cảnh giới hiện tại (không tụt cảnh giới), nghỉ 24 giờ rồi được thử lại. 

- **Tỷ lệ vượt kiếp:** khởi đầu 80%, giảm dần 3% mỗi cảnh giới, sàn 50%. 

- **Tăng tỷ lệ:** dư điểm tu vi so với ngưỡng, pháp bảo kiếm từ bí cảnh (mục 5.5), đan dược hộ thân, đồng môn trong thế lực hộ pháp. 

- **Bảo hiểm:** thất bại liên tiếp 3 lần thì lần sau chắc chắn vượt kiếp. 

- **Không bán tăng tỷ lệ vượt kiếp bằng tiền thật** , theo nguyên tắc ở mục 8. 

### **5.3. Vu tộc** 

Vu tộc đi đường luyện thể nên dùng chuỗi cảnh giới riêng, ngang bậc với phe tiên (theo nguyên tác, Đại Vu ngang Đại La Kim Tiên và Tổ Vu ngang Chuẩn Thánh): 

- Khai Huyết (ngang luyện khí) 

- Luyện Cốt (ngang trúc cơ) 

- Tẩy Tủy (ngang kim đan) 

- Huyết Hải (ngang nguyên anh) 

- Vu Hồn (ngang hóa thần) 

- Tiểu Vu (ngang luyện hư) 

- Trung Vu (ngang hợp thể) 

- Thượng Vu (ngang đại thừa) 

- Chân Vu (ngang độ kiếp) 

- Vu Tướng (ngang nhân tiên) 

- Vu Vương (ngang địa tiên) 

- Vu Tôn (ngang kim tiên) 

- Vu Hoàng (ngang thái ất kim tiên) 

- Đại Vu (ngang đại la kim tiên) 

- Tổ Vu (ngang chuẩn thánh) 

Vu tộc cũng phải qua kiếp từ Vu Hồn trở đi, cơ chế giống lôi kiếp ở mục 5.2. Kiếp của Vu tộc có thể đặt tên riêng, ví dụ "Huyết kiếp". Tên các bậc có thể đặt lại tùy ý. 

**Cách chơi:** thay vì nghe giảng đạo và ngồi thiền, Vu tộc luyện thể, tế Bàn Cổ và chiến đấu để lên cấp. Cách tu luyện hằng ngày là mini game đấm vào đá. Tốc độ tu vi dùng cùng hệ số với phe tiên để công bằng. Vu mạnh về chiến đấu trong Đại chiến, phe tiên mạnh về giảng đạo và pháp bảo. Có thể thêm hiệu ứng tập thể khi đủ người, như Đại trận Tổ Vu bên Vu tộc và Chu Thiên Tinh Đấu bên Yêu tộc. 

### **5.4. Mini game đột phá** 

Bước lên Nguyên Anh và Hóa Thần không chỉ cần đủ điểm tu vi, người chơi còn phải vượt một thử thách bằng mini game: 

- **Bước lên Nguyên Anh, vượt tâm ma:** mini game kiểu Tetris. 

- **Bước lên Hóa Thần, hóa phàm:** mini game kiểu chém hoa quả. 

Thua mini game không mất tu vi, chỉ là chưa đột phá được. Người chơi nghỉ một khoảng ngắn rồi thử lại. 

### **5.5. Bí cảnh thử luyện và tài nguyên** 

Bí cảnh thử luyện là mini game kiểu Vampire Survivors, người chơi vào đó để kiếm tài nguyên: 

- **Linh thạch:** tăng tốc độ tu luyện. 

- **Pháp bảo:** tăng tỷ lệ vượt qua lôi kiếp. 

Linh thạch và pháp bảo do người chơi tự kiếm, có thể chia sẻ và trao đổi cho nhau. Không có kênh mua bằng tiền thật. 

Tốc độ tăng nhờ linh thạch vẫn nằm trong trần 150 điểm/ngày ở mục 9.1, để không phá nhịp cốt truyện. Giao dịch giữa người chơi có giới hạn số lần mỗi ngày và có nhật ký giao dịch, để tránh bị lợi dụng mua bán bằng tiền thật bên ngoài game. 

### **5.6. Đạo lữ song tu** 

Người chơi có thể kết đạo lữ với một người chơi khác để song tu. 

- **Hiệu quả:** khi cả hai cùng online, hiệu quả tu luyện của cả hai tăng trong 2 giờ. 

- **Kết trọn đời:** đã kết đạo lữ thì không thể bỏ nhau. 

- **Ngoại lệ duy nhất:** một bên bỏ mạng trong lượng kiếp, gồm luân hồi sau Đại chiến Vu Yêu, lên Phong Thần Bảng hoặc vào luân hồi ở Phong Thần Lượng Kiếp (xem mục 7). Khi ràng buộc đã chấm dứt, người chơi có thể kết đạo lữ mới. 

Đề xuất để bảo vệ người chơi, vì ràng buộc không thể hủy giữa hai người thật có thể gây rắc rối: 

- Cả hai cùng xác nhận, kèm cảnh báo rõ rằng ràng buộc là vĩnh viễn trước khi kết. 

- Nếu một bên có hành vi quấy rối, quản trị viên có quyền chấm dứt ràng buộc sau khi xử lý báo cáo. 

- Nếu một bên không online quá lâu (ví dụ 90 ngày), coi như tọa hóa theo cốt truyện, bên còn lại được giải ràng buộc. 

- Mức tăng hiệu quả nằm trong trần 150 điểm/ngày ở mục 9.1. 

## **6. Cốt truyện và cột mốc** 

Cốt truyện tiến triển theo **tổng tu vi của toàn bộ người chơi** . Mốc tham chiếu lý tưởng là **10.000 người chơi** . 

### **6.1. Công thức** 

<mark>Mốc (điểm) = số ngày mục tiêu × 10.000 × tu vi trung bình mỗi người mỗi ngày</mark> 

Ví dụ minh họa: nếu một người chơi tích cực kiếm khoảng 100 điểm mỗi ngày thì tổng tu vi của server là 1 triệu điểm mỗi ngày. Con số 100 điểm/ngày chỉ là giả định để tính, cần hiệu chỉnh khi thử nghiệm. 

### **6.2. Lịch mốc dự kiến (tính từ lúc mở game)** 

- Mở đầu Tử Tiêu Cung và chọn thế lực: ngày 0 

- Hồng Quân giảng đạo lần 2: ngày 30 (khoảng 30 triệu điểm) 

- Hồng Quân giảng đạo lần 3: ngày 60 (khoảng 60 triệu điểm) 

- Đại chiến Vu Yêu: ngày 100 đến 115 (mốc khoảng 100 triệu điểm) 

- Nữ Oa tạo người: ngày 130 

- Thánh nhân xuất thế lần lượt: ngày 140 đến 170, sau đó các phe phái của Thánh nhân định hình hoàn chỉnh 

- Phe Nhân tộc, Tam Hoàng Ngũ Đế: ngày 150 đến 210 

- Phong Thần Lượng Kiếp: ngày 240 đến 300, người chơi Ứng Kiếp bằng cờ caro, ai thua 10 ván lên Phong Thần Bảng gia nhập Thiên Đình, ai chơi không đủ 15 ván vào luân hồi 

- Thiên Đình tái lập: ngày 310, ban thần vị cho người có tu vi cao nhất trong số những người đã lên Phong Thần Bảng 

- Tây Du: ngày 330 trở đi 

### **6.3. Cơ chế chặn hai đầu** 

- Mỗi mốc không đến sớm hơn **70%** và không muộn hơn **150%** thời gian mục tiêu. 

- Server đông hơn 10.000 người thì cốt truyện không chạy quá nhanh. Server vắng thì vẫn tiến, không bị kẹt. 

- Giai đoạn đầu ít người, thực tế cốt truyện sẽ chạy theo thời gian. 

- Giới hạn mức đóng góp tu vi tối đa của mỗi người mỗi ngày để chống bot và người cày quá mức. 

### **6.4. Bảng điểm tu vi mỗi người** 

Bảng tính theo mức trung bình 100 điểm/ngày. Phe Vu tộc dùng cùng mốc điểm cho bậc tương ứng. 

|Cảnh giới|Điểm tích lũy|Ngày<br>(trung<br>bình)|Lôi kiếp|Mốc cốt truyện|
|---|---|---|---|---|
|Luyện khí|0|0|Không|Mở đầu Tử Tiêu Cung|
|Trúc cơ|400|4|Không||
|Kim đan|1.200|12|Không||
|Nguyên anh|2.400|24|Không|Giảng đạo lần 2 (ngày 30)|
|Hóa thần|4.000|40|Không||
|Luyện hư|6.000|60|Có|Giảng đạo lần 3|
|Hợp thể|8.000|80|Có||
|Đại thừa|10.000|100|Có|Đại chiến Vu Yêu|
|Độ kiếp|12.500|125|Có|Nữ Oa tạo người (ngày 130)|
|Nhân tiên|15.000|150|Có|Thánh nhân xuất thế (ngày 140<br>đến 170)|
|Địa tiên|18.000|180|Có|Tam Hoàng Ngũ Đế|
|Kim tiên|21.500|215|Có||
|Thái Ất Kim Tiên|25.000|250|Có|Phong Thần Lượng Kiếp|
|Đại La Kim Tiên|29.000|290|Có||
|Chuẩn Thánh|33.000|330|Có|Tây Du|



## **7. Chuyển thế lực qua các lượng kiếp** 

### **7.1. Sau Đại chiến Vu Yêu** 

Theo nguyên tác, sau Đại chiến Vu Yêu, hai tộc gần như diệt vong. Để người chơi Vu và Yêu không mất tiến độ: 

- Trong Đại chiến, Hậu Thổ kiến lập **Lục Đạo Luân Hồi** và **Địa Phủ** . 

- Người chơi thuộc Vu tộc hoặc Yêu tộc được chọn: gia nhập phe **Địa Phủ** , hoặc **luân hồi** sang **Nhân tộc** hay sang **loài vật khác** . 

- Đề xuất: người luân hồi giữ lại một phần tu vi hoặc quy đổi thành "nghiệp lực". Người luân hồi thành thú có đường hóa hình về sau, nối vào giai đoạn Tây Du, nơi yêu quái tu luyện hóa hình. 

### **7.2. Định hình phe phái sau khi Thánh nhân xuất thế** 

Sau khi Nữ Oa tạo người, các Thánh nhân lần lượt xuất thế, và các phe phái của Thánh nhân định hình hoàn chỉnh. Có Nhân tộc thì các Thánh nhân mới có cớ lập giáo phái để thành Thánh nhân: 

- **Oa Hoàng Cung:** phe của Nữ Oa. 

- **Tây Phương Giáo:** phe của Chuẩn Đề và Tiếp Dẫn, đổi tên từ phe Chuẩn Đề ban đầu. 

- **Nhân Giáo, Xiển Giáo, Tiệt Giáo:** Tam Thanh tách thành ba giáo, lần lượt của Thái Thanh Lão Tử, Nguyên Thủy Thiên Tôn và Thông Thiên Giáo Chủ. 

Đề xuất: 

- Người chơi thuộc phe Tam Thanh được chọn một trong ba giáo ngay lúc tách, giữ nguyên tu vi. 

- Người chơi phe Chuẩn Đề tự động chuyển sang Tây Phương Giáo. 

- Người chơi luân hồi sang Nhân tộc (mục 7.1) có thể gia nhập Oa Hoàng Cung. 

### **7.3. Phong Thần Lượng Kiếp** 

Trong Phong Thần Lượng Kiếp, mỗi người chơi có cơ hội **Ứng Kiếp** bằng mini game cờ caro (X O) đấu với người chơi khác trực tuyến, mỗi nước đi giới hạn 10 giây, và có thể chọn từ bỏ. 

- **Thua 10 ván:** lên Phong Thần Bảng, gia nhập Thiên Đình. 

- **Chơi đủ 15 ván và thua dưới 10 ván:** không sao. 

- **Không tham gia từ đầu:** không sao. 

- **Không chơi đủ 15 ván khi sự kiện kết thúc:** vào luân hồi. 

- **Thần vị cao nhất:** các thần vị đứng đầu Thiên Đình được ban cho những người có tu vi cao nhất trong số những người đã lên Phong Thần Bảng. 

- **Đạo lữ:** người lên Phong Thần Bảng hoặc vào luân hồi thì ràng buộc đạo lữ chấm dứt, và có thể kết đạo lữ mới sau đó. 

Đề xuất: 

- Chốt bảng xếp hạng tu vi tại một thời điểm cố định ngay trước khi lượng kiếp bắt đầu, để người chơi không tranh nhau tăng điểm phút chót. 

- Theo nguyên tác, Phong Thần Bảng có 365 chính thần, có thể dùng con số này làm số thần vị, xếp hạng theo tu vi. 

- Thần vị chỉ mang danh hiệu và hiệu ứng nhỏ, không tạo lợi thế quá lớn, để giữ công bằng cho người chơi vào sau. 

- Người chơi từ bỏ giữa chừng được tính như chưa chơi đủ 15 ván: nếu hết sự kiện vẫn chưa đủ thì vào luân hồi, giữ một phần tu vi như ở mục 7.1. 

- Sự kiện kéo dài đủ lâu (ví dụ 60 ngày) và 15 ván là số nhỏ, để người online ít vẫn kịp chơi đủ. 

- Hết 10 giây mà chưa đi nước nào thì thua ván. Ghép đối thủ ngẫu nhiên, và dùng bot thay thế khi không tìm được đối thủ trong thời gian ngắn, để không ai bị kẹt vì thiếu người chơi online cùng lúc. 

- Chống cố tình thua để lên Phong Thần Bảng nhằm lấy thần vị: không ghép người cùng thế lực hoặc đạo lữ với nhau, ẩn danh đối thủ trước ván, theo dõi các cặp đối thủ lặp lại và các ván thua bất thường. 

- Người mới vào sau Phong Thần gia nhập Thiên Đình ở chức vị thấp nhất. 

### **7.4. Đổi phe và giới hạn của người trên Phong Thần Bảng** 

- **Đổi phe:** trừ người đã lên Phong Thần Bảng, mọi người chơi có thể đổi phe thoải mái. 

- **Người trên Phong Thần Bảng:** không đổi phe được và chỉ tu luyện được đến Thái Ất Kim Tiên. 

Đề xuất: 

- Khi đổi giữa Vu tộc và các phe tiên, quy đổi cảnh giới theo bảng ngang bậc ở mục 5.3 và giữ nguyên điểm tu vi. 

- Có thể đặt thời gian chờ ngắn giữa hai lần đổi phe để tránh nhảy phe liên tục chỉ để chạy theo bảng xếp hạng. 

- Người trên Phong Thần Bảng đã chạm Thái Ất Kim Tiên thì điểm tu vi dư quy đổi thành công đức hoặc danh vọng, để họ vẫn có việc để làm trong giai đoạn Tây Du. 

## **8. An toàn, đạo đức và rủi ro** 

- **Kiểm duyệt tên hiển thị:** lọc tên đạo hiệu ngay từ lúc đăng ký để tránh tình trạng tên dung tục như ở trào lưu gốc. 

- **Không gắn tiền thật hay "nhận lộc" vào nghi lễ:** tránh các biến tướng trục lợi mà cơ quan chức năng đã cảnh báo. 

- **Thu ít dữ liệu cá nhân nhất có thể,** và xem kỹ các quyền truy cập mà app yêu cầu. 

- **Tôn trọng tín ngưỡng:** Tam Thanh và Đạo giáo là tín ngưỡng thật, nên cần giữ tông tôn trọng, nhất là phần nội dung kinh và nghi lễ. 

- **Chống bot và gian lận** bằng giới hạn đóng góp tu vi mỗi ngày. 

- **Nhất quán thiết lập:** dự án dùng hệ Tam Thanh theo Hồng Hoang và Phong Thần, cần giữ nhất quán trong toàn bộ nội dung. 

## **9. Thông số đề xuất và nguyên tắc vận hành** 

Các thông số dưới đây là đề xuất ban đầu, cần hiệu chỉnh khi thử nghiệm. 

### **9.1. Tốc độ tu vi: 100 điểm/ngày** 

- Treo máy trong động phủ: khoảng 50 điểm, tích lũy tối đa 8 giờ, phải quay lại thu. 

- Nghe giảng đạo và nhiệm vụ ngày: khoảng 30 điểm. 

- Hoạt động cộng đồng và thế lực: khoảng 20 điểm. 

- Trần đóng góp 150 điểm/ngày mỗi người để chống bot. 

- Người vào muộn được hệ số bù (ví dụ x2) cho đến khi ngang mức trung bình của server. 

### **9.2. Hoạt động hằng ngày giai đoạn đầu** 

- Dâng hương buổi sáng, có chuỗi ngày liên tiếp để thưởng. 

- Giảng đạo 2 buổi cố định (sáng và tối), cả server vào cùng lúc. 

- Ngộ đạo mỗi ngày một lần, gieo quẻ nhận buff nhỏ. 

- Việc nhỏ ở động phủ: hái linh dược, luyện đan đơn giản để tăng tốc tu vi. 

- Vào bí cảnh thử luyện (mini game) để kiếm linh thạch và pháp bảo. 

- Góp công đức cho thế lực, có bảng xếp hạng giữa các thế lực. 

### **9.3. Nguyên tắc không thu phí** 

- Game không thu bất kỳ khoản phí nào và không có vật phẩm mua bằng tiền thật. 

- Vật phẩm do người chơi tự kiếm (linh thạch, pháp bảo) có thể chia sẻ và trao đổi cho nhau. 

- Không gắn "nhận lộc" hay thưởng tiền thật vào nghi lễ, lôi kiếp hay mini game, theo nguyên tắc ở mục 8. 

## **10. Kế hoạch video mở đầu** 

Trước khi vào game, người chơi xem một video mở đầu kể lịch sử Hồng Hoang, từ lúc Bàn Cổ khai thiên đến lúc Hồng Quân mở Tử Tiêu Cung, tức điểm bắt đầu của màn chơi đầu tiên. Dưới đây là bản nháp đề xuất, tổng thời lượng khoảng 3 phút 30 giây. 

|Cảnh|Nội dung|Thời lượng|
|---|---|---|
|1. Hỗn Độn|Vũ trụ chưa có trời đất, Bàn Cổ ngủ trong hỗn độn|20 giây|
|2. Bàn Cổ khai thiên|Rìu bổ hỗn độn, thanh khí bay lên làm trời, trọc khí<br>chìm xuống làm đất|30 giây|
|3. Bàn Cổ vẫn lạc|Thân hóa vạn vật (núi sông, mặt trời, mặt trăng),<br>nguyên thần hóa Tam Thanh, tinh huyết hóa Tổ Vu|40 giây|
|4. Thiên địa sơ khai|Linh khí tràn ngập, vạn vật sinh sôi|20 giây|
|5. Long Hán Sơ Kiếp|Long, Phượng, Kỳ Lân tranh bá và cùng suy tàn|30 giây|
|6. Các đại năng xuất hiện|Tam Thanh, Nữ Oa, Chuẩn Đề, Tiếp Dẫn, Đông<br>Vương Công, Đế Tuấn, Thái Nhất, Hậu Thổ lần lượt<br>hiện thế|30 giây|
|7. Hồng Quân hiện thế|Hồng Quân Đạo Tổ ban lệnh mời các đại năng đến<br>nghe giảng đạo|20 giây|
|8. Tử Tiêu Cung mở|Hồng Quân bắt đầu giảng đạo, cắt sang màn chơi<br>đầu tiên|20 giây|



Đề xuất triển khai: 

- Cho phép bỏ qua video, có phụ đề, và xem lại trong menu. 

- Làm theo phong cách đồ họa 2D chuyển động tối giản hoặc ảnh tĩnh có hiệu ứng, kèm giọng kể và nhạc, phù hợp với dự án không thu phí. 

- Video dừng ngay trước khi các thế lực kiến lập, để việc người chơi chọn thế lực ở mục 4 vẫn là bước mở đầu của cốt truyện. 

- Truyện Hồng Hoang có nhiều phiên bản khác nhau về chi tiết, nên cần chọn một mạch nhất quán cho cả video và nội dung game. 

