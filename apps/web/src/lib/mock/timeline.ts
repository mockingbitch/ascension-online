// SAMPLE DATA — the server story clock exists (world_state) but milestones are Phase 2 (world_milestones.yaml
// + GET /world). Slugs/titles/blurbs from content/story/timeline/hong-hoang-timeline.md (draft, D2-compliant).
// `serverDay` and community totals below are illustrative; the screen shows the "Bản xem trước" ribbon.

export interface Milestone {
	slug: string;
	title: string;
	/** target server day (cot-truyen pacing) */
	targetDay: number;
	/** last day of a multi-day era, when it has one */
	untilDay?: number;
	blurb: string;
}

export const milestones: Milestone[] = [
	{ slug: 'khai_giang', title: 'Tử Tiêu Cung khai giảng', targetDay: 0, blurb: 'Hồng Quân mở Tử Tiêu Cung, giảng đạo lần đầu cho chúng sinh Thiên Nguyên Giới. Năm thế lực mở cửa thu nhận đệ tử.' },
	{ slug: 'giang_dao_2', title: 'Hồng Quân giảng đạo lần hai', targetDay: 30, blurb: 'Tử Tiêu Cung mở cửa lần thứ hai. Ai đã vững căn cơ sẽ nghe được tầng đạo lý sâu hơn, vốn dành cho người kiên trì.' },
	{ slug: 'giang_dao_3', title: 'Hồng Quân giảng đạo lần ba', targetDay: 60, blurb: 'Lần giảng cuối của Hồng Quân tại Tử Tiêu Cung. Từ đây, đạo pháp được truyền lại qua các thế lực.' },
	{ slug: 'dai_chien', title: 'Đại chiến Huyết Tinh', targetDay: 100, untilDay: 115, blurb: 'Vu Tộc và Tinh Hà Yêu Đình quyết chiến suốt mười lăm ngày. Trời đất nứt vỡ, và từ đổ nát, Địa Phủ cùng vòng luân hồi ra đời.' },
	{ slug: 'nu_oa', title: 'Nữ Oa tạo người', targetDay: 130, blurb: 'Nữ Oa nặn đất bên bờ sông, thổi linh khí thành người. Nhân tộc ra đời, yếu ớt nhưng mang trong mình mọi khả năng.' },
	{ slug: 'thanh_nhan', title: 'Thánh nhân xuất thế', targetDay: 140, untilDay: 170, blurb: 'Thánh nhân lần lượt xuất thế. Huyền Khung Môn chia ba viện, Kim Sa Môn đổi thành Kim Sa Giáo, Oa Hoàng Cung mở cửa.' },
	{ slug: 'nhan_toc', title: 'Tam Hoàng Ngũ Đế', targetDay: 150, untilDay: 210, blurb: 'Tam Hoàng Ngũ Đế dẫn dắt Nhân tộc: dạy cày cấy, nếm cỏ làm thuốc, định lịch. Nhân tộc thành một thế lực của Thiên Nguyên Giới.' },
	{ slug: 'phong_than', title: 'Phong Thần Lượng Kiếp', targetDay: 240, untilDay: 300, blurb: 'Phong Thần Bảng mở ra. Ai muốn Ứng Kiếp thì đấu cờ với đạo hữu. Người có tên trên bảng sẽ thành thần của Thiên Đình mới.' },
	{ slug: 'thien_dinh', title: 'Thiên Đình tái lập', targetDay: 310, blurb: 'Thiên Đình được dựng lại. Người trên Phong Thần Bảng nhận thần vị, và người tu vi cao nhất đứng đầu chúng thần.' },
	{ slug: 'tay_du', title: 'Tây Du', targetDay: 330, blurb: 'Con đường về phía tây mở ra. Yêu quái ven đường tu luyện hóa hình, lữ khách vượt núi băng sông. Một thời đại mới bắt đầu.' }
];

/** Illustrative world state for the preview (server day, community tu vi toward the next milestone). */
export const sampleWorld = {
	serverDay: 18,
	communityPoints: 1_212_000,
	nextThresholdPoints: 2_010_000,
	activePlayers: 5_213
};
