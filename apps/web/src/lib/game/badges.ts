// Attention badges of the action bar / "Thêm" menu, derived from server state in the store. Every badge carries text
// (count or "!") and an aria label — never colour alone.
import { hudFx } from '$lib/engine/fx/hud.svelte';
import { game } from './store.svelte';

export interface Badge {
	text: string;
	label: string;
}

export function badgeOf(id: string): Badge | null {
	switch (id) {
		case 'tasks':
			return game.claimable > 0 ? { text: String(Math.min(9, game.claimable)), label: `${game.claimable} nhiệm vụ chờ nhận thưởng` } : null;
		case 'breakthrough':
			return game.preview?.ready ? { text: '!', label: 'Đủ điều kiện đột phá' } : null;
		case 'inventory':
			return hudFx.unseen > 0 ? { text: String(Math.min(9, hudFx.unseen)), label: `${hudFx.unseen} vật phẩm mới trong Túi Đồ` } : null;
		case 'vip':
			return game.vip && game.vip.benefits.dailyGift > 0 && !game.vip.giftClaimedToday ? { text: '!', label: 'Có quà Tiên Duyên chưa nhận' } : null;
		default:
			return null;
	}
}
