// SAMPLE DATA — factions are not on the server yet (Phase 2: content/catalog/factions.yaml + character_faction).
// Names/IDs from content/story/factions/*.md (story-creator, status: draft, D2-compliant). Member counts and
// totals are illustrative only; screens using this file show the "Bản xem trước" ribbon.
import type { Aura } from '$lib/game/types';

export interface Faction {
	id: string;
	name: string;
	motto: string;
	path: 'tien' | 'vu';
	/** aura token used for the faction colour */
	color: Aura | 'spirit-cyan' | 'spirit-violet';
	leaderTitle: string;
	traits: string[];
	description: string;
	/** illustrative numbers (sample) */
	members: number;
	totalCultivation: number;
}

export const factions: Faction[] = [
	{
		id: 'huyen_khung_mon',
		name: 'Huyền Khung Môn',
		motto: 'Một vòm trời, ba nhánh đạo, cùng về một nguồn.',
		path: 'tien',
		color: 'jade',
		leaderTitle: 'Tam Tọa Khung Chủ',
		traits: ['Chép đạo ngữ', 'Ba tòa đạo đài', 'Thẻ ngọc trống'],
		description: 'Tông môn dưới chân núi Tử Tiêu, ba tòa đạo đài chung một sơn môn, chờ ngày đạo tâm tự phân nhánh.',
		members: 1284,
		totalCultivation: 412_800
	},
	{
		id: 'kim_sa_mon',
		name: 'Kim Sa Môn',
		motto: 'Hạt cát tuy nhỏ, nguyện lực rất dài.',
		path: 'tien',
		color: 'golden',
		leaderTitle: 'Kim Sa Song Chủ',
		traits: ['Túi cát nhập môn', 'Đi xa thu người', 'Lời nguyện trước bế quan'],
		description: 'Tông môn nơi Tây Mạc nghèo linh khí, tu chậm mà bền và không hỏi xuất thân ai.',
		members: 936,
		totalCultivation: 301_450
	},
	{
		id: 'thuong_lan_dinh',
		name: 'Thương Lan Tiên Đình',
		motto: 'Bốn biển một sổ tiên, không ai bị bỏ sót.',
		path: 'tien',
		color: 'spirit-cyan',
		leaderTitle: 'Thương Lan Tiên Chủ',
		traits: ['Ghi danh Tiên Tịch', 'Triều trên đảo nổi', 'Phẩm phục theo màu biển'],
		description: 'Triều đình tu sĩ đầu tiên trên đảo nổi biển Đông, giữ Tiên Tịch và trọng lễ nghi, phẩm trật.',
		members: 1102,
		totalCultivation: 358_900
	},
	{
		id: 'tinh_ha_yeu_dinh',
		name: 'Tinh Hà Yêu Đình',
		motto: 'Sao trời không hỏi xuất thân.',
		path: 'tien',
		color: 'spirit-violet',
		leaderTitle: 'Tinh Hà Yêu Hoàng',
		traits: ['Sao bản mệnh', 'Bản tướng', 'Hội dưới trăng'],
		description: 'Triều đình Yêu tộc giữa ngân hà, nơi mọi loài tu thành hình đều được một vì sao.',
		members: 1017,
		totalCultivation: 336_200
	},
	{
		id: 'vu_toc',
		name: 'Vu Tộc',
		motto: 'Máu Bàn Cổ chưa nguội, thân ta tức là pháp bảo.',
		path: 'vu',
		color: 'blood',
		leaderTitle: 'Thủ Tế Vu Chủ',
		traits: ['Tế Bàn Cổ lúc bình minh', 'Thân là binh khí', 'Huyết kiếp'],
		description: 'Tộc luyện thể mang máu Bàn Cổ, rèn thân như núi và coi bầu trời là đất lạ.',
		members: 874,
		totalCultivation: 297_600
	}
];

export const factionById = (id: string | null | undefined) => factions.find((f) => f.id === id);

/** CSS colour for a faction (all are design tokens). */
export const factionColor = (f: Faction) =>
	f.color === 'spirit-cyan' || f.color === 'spirit-violet' ? `var(--color-${f.color})` : `var(--color-aura-${f.color})`;
