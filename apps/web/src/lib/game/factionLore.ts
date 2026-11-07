// Display-only faction lore that is narrative, not catalogue config (like lib/game/timeline.ts). Copied from
// content/story/factions/*.md and content/story/world/milestone-effects.md (story-creator, status: draft).
// Names, paths, colours, mottos and pitches come from the server (GET /factions, content/catalog/factions.yaml).

export interface FactionLore {
	leaderTitle?: string;
	traits: string[];
	description: string;
}

// Keys are the permanent legacy slugs; display names follow docs/design/cot-truyen.md (see content/catalog/factions.yaml
// header for the slug → canon name table).
export const factionLore: Record<string, FactionLore> = {
	huyen_khung_mon: {
		leaderTitle: 'Thái Thanh Lão Tử · Nguyên Thủy Thiên Tôn · Thông Thiên Giáo Chủ',
		traits: ['Chép đạo ngữ', 'Ba nhánh một nguồn', 'Thẻ ngọc trống'],
		description: 'Môn hạ Tam Thanh nơi Côn Lôn: một khí hóa ba thanh, chung một gốc, chờ ngày đạo tâm tự phân giáo.'
	},
	kim_sa_mon: {
		leaderTitle: 'Chuẩn Đề · Tiếp Dẫn',
		traits: ['Túi cát nhập môn', 'Đi xa thu người', 'Lời nguyện trước bế quan'],
		description: 'Phe Chuẩn Đề nơi Tây phương nghèo linh khí, tu chậm mà bền và không hỏi xuất thân ai.'
	},
	thuong_lan_dinh: {
		leaderTitle: 'Đông Vương Công',
		traits: ['Ghi danh sổ tiên', 'Triều trên đảo tiên', 'Phẩm phục theo màu biển'],
		description: 'Tiên Đình của Đông Vương Công trên đảo tiên biển Đông, giữ sổ tiên và trọng lễ nghi, phẩm trật.'
	},
	tinh_ha_yeu_dinh: {
		leaderTitle: 'Đế Tuấn · Thái Nhất',
		traits: ['Sao bản mệnh', 'Bản tướng', 'Chu Thiên Tinh Đấu'],
		description: 'Thiên Đình của Yêu tộc trên chín tầng trời, nơi mọi loài tu thành hình đều được một vì sao.'
	},
	vu_toc: {
		leaderTitle: 'Tổ Vu',
		traits: ['Tế Bàn Cổ lúc bình minh', 'Thân là binh khí', 'Huyết kiếp'],
		description: 'Tộc luyện thể do tinh huyết Bàn Cổ hóa thành, rèn thân như núi và coi bầu trời là đất lạ.'
	},
	thu_nhat_vien: {
		leaderTitle: 'Thái Thanh Lão Tử',
		traits: ['Thanh tĩnh vô vi', 'Ít lời', 'Bế quan dài ngày'],
		description: 'Một trong ba giáo tách từ Tam Thanh: giáo của Thái Thanh Lão Tử, giữ một điều cho trọn.'
	},
	lap_cuc_vien: {
		leaderTitle: 'Nguyên Thủy Thiên Tôn',
		traits: ['Giới luật rõ', 'Truyền thụ có thứ bậc'],
		description: 'Một trong ba giáo tách từ Tam Thanh: giáo của Nguyên Thủy Thiên Tôn, đạo cần cột mốc.'
	},
	van_luu_vien: {
		leaderTitle: 'Thông Thiên Giáo Chủ',
		traits: ['Hữu giáo vô loại', 'Mọi xuất thân'],
		description: 'Một trong ba giáo tách từ Tam Thanh: giáo của Thông Thiên Giáo Chủ, trăm dòng cùng về biển.'
	},
	kim_sa_giao: {
		leaderTitle: 'Chuẩn Đề · Tiếp Dẫn',
		traits: ['Túi cát nhập môn', 'Đi xa thu người', 'Lời nguyện trước bế quan'],
		description: 'Phe Chuẩn Đề sau khi Chuẩn Đề và Tiếp Dẫn thành thánh: cùng đệ tử, cùng tâm ngôn, cùng huy hiệu, nay thành giáo.'
	},
	nhan_toc: {
		leaderTitle: 'Tam Hoàng Ngũ Đế',
		traits: ['Nữ Oa nặn từ đất', 'Cày cấy, làm thuốc, định lịch'],
		description: 'Nhân tộc do Nữ Oa tạo ra, được Tam Hoàng Ngũ Đế dẫn dắt.'
	}
};

export const loreOf = (id: string): FactionLore => factionLore[id] ?? { traits: [], description: '' };
