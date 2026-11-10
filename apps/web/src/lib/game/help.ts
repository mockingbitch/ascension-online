// Rules and explanations that used to sit as paragraphs on the main screens (spec §3.4 "bớt chữ"): they live behind a
// "?" button now (help window). Text only — no game numbers here (those come from the server/catalogue on screen).
import type { IconName } from '$lib/components/Icon.svelte';
import type { HelpTopic } from './windowStack';

export interface HelpEntry {
	title: string;
	icon: IconName;
	points: string[];
}

export const help: Record<HelpTopic, HelpEntry> = {
	world: {
		title: 'Thế Giới',
		icon: 'map',
		points: [
			'Bấm một địa danh trên bản đồ để tới đó: Động Phủ để bế quan, Tử Tiêu Cung để nghe Hồng Quân giảng đạo, Bí Cảnh để thử luyện.',
			'Tu vi vẫn tích khi bạn rời game, tới giới hạn bế quan. Bấm "Thu nhận" để máy chủ kết toán phần đã tích.',
			'Địa danh có khoá sẽ khai mở theo Thiên Đạo Sử.'
		]
	},
	dong_phu: {
		title: 'Động Phủ · Tu luyện',
		icon: 'lotus',
		points: [
			'Thanh tu vi chạy theo tốc độ treo máy chỉ là ước tính hiển thị — máy chủ kết toán khi bạn thu nhận.',
			'Thời gian bế quan tích tới giới hạn bế quan (tăng theo Tiên Duyên); quá giới hạn thì không tích thêm, hãy thu nhận trước khi đầy.',
			'Mỗi ngày có hạn mức tu vi chung cho mọi nguồn; chạm hạn mức thì phần dư không được cộng.',
			'Bình cảnh: tầng cuối của một cảnh giới — tu vi dừng lại cho tới khi đột phá thành công.',
			'Tụ Linh Phù cộng thêm giờ cho lần bế quan kế tiếp, tối đa một lần mỗi ngày.',
			'Người đến sau được Thiên đạo bù đắp: hệ số nhân hiện ngay trên màn khi đang áp dụng.'
		]
	},
	hall: {
		title: 'Tử Tiêu Cung · Ngộ đạo',
		icon: 'scroll',
		points: [
			'Ngồi trong điện là đang nghe giảng: máy chủ tự đo thời gian bạn có mặt.',
			'Khi Hồng Quân đặt câu hỏi, thời khắc ngộ đạo mở trong ít phút — bấm "Ngộ đạo" để nhận tu vi theo thời gian đã nghe.',
			'Sau mỗi lần ngộ đạo cần chờ tới thời khắc kế tiếp. Tu vi từ giảng đạo có hạn mức riêng mỗi ngày; khi đã đủ, lời dạy vẫn được ghi vào Sổ Đạo Ngộ.',
			'Tử Tiêu Cung chia thành nhiều điện, mỗi điện đủ bồ đoàn cho mọi người ngồi. Bạn được xếp gần đạo lữ, rồi đồng môn; có thể đổi sang điện còn chỗ. Lời giảng và ngộ đạo giống nhau ở mọi điện.',
			'Tin nhắn Luận Đạo chỉ đạo hữu cùng điện thấy, hiện thành bong bóng trên đầu họ.',
			'Tên đạo hữu sáng dần theo cảnh giới; hiệu ứng tên mua bằng Tiên Duyên chỉ là nét gạch chân, không lấn cảnh giới.'
		]
	},
	breakthrough: {
		title: 'Đột phá',
		icon: 'bolt',
		points: [
			'Tỉ lệ thành công được công bố trước khi bạn thử; kết quả do máy chủ quyết định.',
			'Thất bại không tụt tầng: tâm cảnh vững thêm và lần sau tỉ lệ cao hơn (có trần).',
			'Từ lôi kiếp trở đi, tu vi dư, pháp bảo hộ thân (Túi Đồ) và đồng môn hộ pháp cộng thêm tỉ lệ — mỗi phần có trần riêng, tổng cộng thêm và tỉ lệ cuối cũng có trần.',
			'Lôi kiếp thất bại mất một phần tu vi tầng hiện tại, không tụt tầng.',
			'Một số cảnh giới có thử luyện (Tâm Ma, Hóa Phàm) phải vượt qua trước khi được đột phá. Pháp bảo chỉ rơi từ bí cảnh.'
		]
	},
	timeline: {
		title: 'Thiên Đạo Sử',
		icon: 'clock',
		points: [
			'Mỗi ngày bạn tu luyện, tu vi của bạn góp vào tiến độ chung của cả cõi (có giới hạn mỗi người mỗi ngày).',
			'Tiến độ chỉ tính những ngày đã chốt (sau nửa đêm).',
			'Mốc không đến trước ngày sớm nhất và chắc chắn đến trước ngày muộn nhất.'
		]
	},
	inventory: {
		title: 'Túi Đồ',
		icon: 'bag',
		points: [
			'Đan tăng tốc cộng tu vi ngay, có giới hạn số viên mỗi ngày và nằm trong hạn mức hằng ngày.',
			'Pháp bảo chỉ rơi từ bí cảnh, không bán, không trao đổi. Chỉ một pháp bảo hộ thân được tính khi độ lôi kiếp.',
			'Vật phẩm có khoá không thể tặng hay trao đổi. Trang phục chỉ đổi ngoại hình, không tăng sức mạnh.'
		]
	}
};
