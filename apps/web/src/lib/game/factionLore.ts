// Display-only faction lore that is narrative, not catalogue config (like lib/game/timeline.ts). Copied from
// content/story/factions/*.md and content/story/world/milestone-effects.md (story-creator, status: draft).
// Names, paths, colours, mottos and pitches come from the server (GET /factions, content/catalog/factions.yaml).

export interface FactionLore {
	leaderTitle?: string;
	traits: string[];
	description: string;
}

export const factionLore: Record<string, FactionLore> = {
	huyen_khung_mon: {
		leaderTitle: 'Tam Tọa Khung Chủ',
		traits: ['Chép đạo ngữ', 'Ba tòa đạo đài', 'Thẻ ngọc trống'],
		description: 'Tông môn dưới chân núi Tử Tiêu, ba tòa đạo đài chung một sơn môn, chờ ngày đạo tâm tự phân nhánh.'
	},
	kim_sa_mon: {
		leaderTitle: 'Kim Sa Song Chủ',
		traits: ['Túi cát nhập môn', 'Đi xa thu người', 'Lời nguyện trước bế quan'],
		description: 'Tông môn nơi Tây Mạc nghèo linh khí, tu chậm mà bền và không hỏi xuất thân ai.'
	},
	thuong_lan_dinh: {
		leaderTitle: 'Thương Lan Tiên Chủ',
		traits: ['Ghi danh Tiên Tịch', 'Triều trên đảo nổi', 'Phẩm phục theo màu biển'],
		description: 'Triều đình tu sĩ đầu tiên trên đảo nổi biển Đông, giữ Tiên Tịch và trọng lễ nghi, phẩm trật.'
	},
	tinh_ha_yeu_dinh: {
		leaderTitle: 'Tinh Hà Yêu Hoàng',
		traits: ['Sao bản mệnh', 'Bản tướng', 'Hội dưới trăng'],
		description: 'Triều đình Yêu tộc giữa ngân hà, nơi mọi loài tu thành hình đều được một vì sao.'
	},
	vu_toc: {
		leaderTitle: 'Thủ Tế Vu Chủ',
		traits: ['Tế Bàn Cổ lúc bình minh', 'Thân là binh khí', 'Huyết kiếp'],
		description: 'Tộc luyện thể mang máu Bàn Cổ, rèn thân như núi và coi bầu trời là đất lạ.'
	},
	thu_nhat_vien: {
		traits: ['Tĩnh tu', 'Ít lời', 'Bế quan dài ngày'],
		description: 'Một trong ba viện tách từ Huyền Khung Môn: giữ một điều cho trọn.'
	},
	lap_cuc_vien: {
		traits: ['Giới luật rõ', 'Truyền thụ có thứ bậc'],
		description: 'Một trong ba viện tách từ Huyền Khung Môn: đạo cần cột mốc.'
	},
	van_luu_vien: {
		traits: ['Mở cửa cho mọi người', 'Mọi xuất thân'],
		description: 'Một trong ba viện tách từ Huyền Khung Môn: trăm dòng cùng về biển.'
	},
	kim_sa_giao: {
		leaderTitle: 'Kim Sa Song Chủ',
		traits: ['Túi cát nhập môn', 'Đi xa thu người', 'Lời nguyện trước bế quan'],
		description: 'Kim Sa Môn sau khi hai vị Song Chủ thành thánh: cùng đệ tử, cùng tâm ngôn, cùng huy hiệu, chỉ đổi tên.'
	}
};

export const loreOf = (id: string): FactionLore => factionLore[id] ?? { traits: [], description: '' };
