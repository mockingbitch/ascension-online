// Every window of the game shell: its frame (title, icon, size, tabs, help topic) and its content, loaded lazily so
// the initial /play bundle only carries the frame. Content components take `{ win, tab }` (WindowProps).
import type { Component } from 'svelte';
import type { IconName } from '$lib/components/Icon.svelte';
import type { WindowTab } from '$lib/components/GameWindow.svelte';
import { help } from '$lib/game/help';
import { screens } from '$lib/game/nav';
import type { HelpTopic, WindowKind, WindowState } from '$lib/game/windowStack';

export interface WindowProps {
	win: WindowState;
	/** current tab id of a tabbed window ('' otherwise) */
	tab: string;
}

export interface WindowMeta {
	title: string;
	icon: IconName;
	size: 'md' | 'lg';
	tabs?: WindowTab[];
	/** initial tab (from the window state, e.g. a deep link) */
	tab?: string;
	help?: HelpTopic;
}

const inventoryTabs: WindowTab[] = [
	{ id: 'bag', label: 'Hành trang', icon: 'bag' },
	{ id: 'shop', label: 'Cửa hàng', icon: 'stone' },
	{ id: 'cosmetics', label: 'Trang phục', icon: 'spark' },
	{ id: 'trade', label: 'Trao đổi', icon: 'gift' }
];
const rankingTabs: WindowTab[] = [
	{ id: 'personal', label: 'Tu sĩ', icon: 'person' },
	{ id: 'faction', label: 'Thế lực', icon: 'sect' }
];

export function windowMeta(w: WindowState): WindowMeta {
	switch (w.kind) {
		case 'tasks':
			return { title: 'Nhiệm vụ ngày', icon: 'scroll', size: 'md' };
		case 'vip':
			return { title: 'Tiên Duyên', icon: 'seal', size: 'lg' };
		case 'notifications':
			return { title: 'Thông báo', icon: 'bell', size: 'md' };
		case 'settings':
			return { title: 'Cài đặt', icon: 'gear', size: 'md' };
		case 'character':
			return { title: 'Nhân Vật', icon: 'person', size: 'lg' };
		case 'inventory':
			return { title: 'Túi Đồ', icon: 'bag', size: 'lg', tabs: inventoryTabs, tab: w.tab ?? 'bag', help: 'inventory' };
		case 'ranking':
			return { title: 'Thiên Bảng', icon: 'rank', size: 'lg', tabs: rankingTabs, tab: w.tab ?? 'personal' };
		case 'timeline':
			return { title: 'Thiên Đạo Sử', icon: 'clock', size: 'lg', help: 'timeline' };
		case 'breakthrough':
			return { title: 'Đột Phá', icon: 'bolt', size: 'lg', help: 'breakthrough' };
		case 'faction':
			return { title: 'Thế Lực', icon: 'sect', size: 'lg' };
		case 'companion':
			return { title: 'Đào Hoa Đình', icon: 'heart', size: 'lg' };
		case 'fortune':
			return { title: 'Gieo quẻ', icon: 'dice', size: 'md' };
		case 'menu':
			return { title: 'Hành trang đạo đồ', icon: 'more', size: 'md' };
		case 'help':
			return { title: help[w.topic].title, icon: 'help', size: 'md' };
		case 'sealed': {
			const s = Object.values(screens).find((x) => x.href === `/play/${w.screen}`);
			return { title: s?.label ?? 'Chốn chưa khai mở', icon: 'lock', size: 'md' };
		}
		case 'profile':
			return { title: w.name, icon: 'person', size: 'md' };
	}
}

type Loader = () => Promise<{ default: Component<WindowProps> }>;
// Every content component accepts WindowProps; those that need nothing simply ignore them.
const lazy = (f: () => Promise<{ default: unknown }>) => f as unknown as Loader;

export const windowContent: Record<WindowKind, Loader> = {
	tasks: lazy(() => import('./TasksWindow.svelte')),
	vip: lazy(() => import('$lib/components/VipWindow.svelte')),
	notifications: lazy(() => import('$lib/components/NotificationList.svelte')),
	settings: lazy(() => import('$lib/components/SettingsPanel.svelte')),
	character: lazy(() => import('./CharacterWindow.svelte')),
	inventory: lazy(() => import('./InventoryWindow.svelte')),
	ranking: lazy(() => import('./RankingWindow.svelte')),
	timeline: lazy(() => import('./TimelineWindow.svelte')),
	breakthrough: lazy(() => import('./BreakthroughWindow.svelte')),
	faction: lazy(() => import('./FactionWindow.svelte')),
	companion: lazy(() => import('./CompanionWindow.svelte')),
	fortune: lazy(() => import('./FortuneWindow.svelte')),
	menu: lazy(() => import('./MenuWindow.svelte')),
	help: lazy(() => import('./HelpWindow.svelte')),
	sealed: lazy(() => import('./SealedWindow.svelte')),
	profile: lazy(() => import('$lib/components/ProfileWindow.svelte'))
};
