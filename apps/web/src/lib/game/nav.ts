// Game screens and world-map locations: one table drives the action bar, the world map and the "coming soon" states.
// status: live = backed by the real API · preview = UI on labelled sample data · soon = not built.
// Since GĐ3 menus are windows over the scene (`window`); their /play/<slug> URL stays as a deep link.
import type { IconName } from '$lib/components/Icon.svelte';
import type { WindowKind } from './windowStack';

export type ScreenStatus = 'live' | 'preview' | 'soon';

export interface Screen {
	id: string;
	label: string;
	icon: IconName;
	href: string;
	status: ScreenStatus;
	/** opens as this window over the current scene (in-app); undefined = a scene or mini-game route */
	window?: WindowKind;
}

export const screens = {
	world: { id: 'world', label: 'Thế Giới', icon: 'map', href: '/play', status: 'live' },
	cultivate: { id: 'cultivate', label: 'Tu Luyện', icon: 'lotus', href: '/play/tu-luyen', status: 'live' },
	character: { id: 'character', label: 'Nhân Vật', icon: 'person', href: '/play/nhan-vat', status: 'live', window: 'character' },
	inventory: { id: 'inventory', label: 'Túi Đồ', icon: 'bag', href: '/play/tui-do', status: 'live', window: 'inventory' },
	faction: { id: 'faction', label: 'Thế Lực', icon: 'sect', href: '/play/the-luc', status: 'live', window: 'faction' },
	ranking: { id: 'ranking', label: 'Xếp Hạng', icon: 'rank', href: '/play/xep-hang', status: 'live', window: 'ranking' },
	breakthrough: { id: 'breakthrough', label: 'Đột Phá', icon: 'bolt', href: '/play/dot-pha', status: 'live', window: 'breakthrough' },
	hall: { id: 'hall', label: 'Tử Tiêu Cung', icon: 'scroll', href: '/play/tu-tieu-cung', status: 'live' },
	timeline: { id: 'timeline', label: 'Thiên Đạo Sử', icon: 'scroll', href: '/play/thien-dao', status: 'live', window: 'timeline' },
	companion: { id: 'companion', label: 'Đạo Lữ', icon: 'heart', href: '/play/dao-lu', status: 'live', window: 'companion' },
	trial: { id: 'trial', label: 'Bí Cảnh', icon: 'portal', href: '/play/bi-canh', status: 'live' },
	war: { id: 'war', label: 'Đại Chiến', icon: 'bolt', href: '/play/dai-chien', status: 'soon' }
} satisfies Record<string, Screen>;

export type ScreenId = keyof typeof screens;

/** Action bar entry: a scene (route) or a window. `short` is the mobile label. */
export interface BarItem {
	id: string;
	label: string;
	short: string;
	icon: IconName;
	href?: string;
	window?: WindowKind;
}

/** Scenes, in the order of the rail (desktop) / first slots of the bottom row (mobile). */
export const sceneItems: BarItem[] = [
	{ id: 'world', label: 'Thế Giới', short: 'Bản đồ', icon: 'map', href: screens.world.href },
	{ id: 'cultivate', label: 'Động Phủ', short: 'Tu luyện', icon: 'lotus', href: screens.cultivate.href },
	{ id: 'hall', label: 'Tử Tiêu Cung', short: 'Giảng đạo', icon: 'scroll', href: screens.hall.href }
];

/** Windows of the rail. */
export const windowItems: BarItem[] = [
	{ id: 'character', label: 'Nhân Vật', short: 'Nhân vật', icon: 'person', window: 'character' },
	{ id: 'inventory', label: 'Túi Đồ', short: 'Túi đồ', icon: 'bag', window: 'inventory' },
	{ id: 'tasks', label: 'Nhiệm vụ', short: 'Nhiệm vụ', icon: 'check', window: 'tasks' },
	{ id: 'breakthrough', label: 'Đột Phá', short: 'Đột phá', icon: 'bolt', window: 'breakthrough' },
	{ id: 'faction', label: 'Thế Lực', short: 'Thế lực', icon: 'sect', window: 'faction' },
	{ id: 'ranking', label: 'Xếp Hạng', short: 'Xếp hạng', icon: 'rank', window: 'ranking' },
	{ id: 'companion', label: 'Đạo Lữ', short: 'Đạo lữ', icon: 'heart', window: 'companion' },
	{ id: 'timeline', label: 'Thiên Đạo Sử', short: 'Thiên đạo', icon: 'clock', window: 'timeline' }
];

/** Only in the "Thêm" menu (the HUD has its own buttons for these on every width). */
export const extraItems: BarItem[] = [
	{ id: 'vip', label: 'Tiên Duyên', short: 'Tiên Duyên', icon: 'seal', window: 'vip' },
	{ id: 'fortune', label: 'Gieo quẻ', short: 'Gieo quẻ', icon: 'dice', window: 'fortune' },
	{ id: 'notifications', label: 'Thông báo', short: 'Thông báo', icon: 'bell', window: 'notifications' },
	{ id: 'settings', label: 'Cài đặt', short: 'Cài đặt', icon: 'gear', window: 'settings' }
];

/** Phones: what the bottom row shows around the primary button (the rest is in "Thêm"). */
export const mobileRow = { left: ['world', 'cultivate', 'character'], right: ['inventory', 'tasks'] } as const;

/** Clickable places on the world map (percent coordinates inside the 16:9 map art). */
export interface MapLocation {
	screen: ScreenId;
	/** raster landmark slot: src/lib/assets/art/landmarks/<landmark>.webp (layered map only) */
	landmark: string;
	title: string;
	subtitle: string;
	x: number;
	y: number;
}

export const mapLocations: MapLocation[] = [
	{ screen: 'hall', landmark: 'tu-tieu-cung', title: 'Tử Tiêu Cung', subtitle: 'Hồng Quân giảng đạo', x: 50, y: 47 },
	{ screen: 'cultivate', landmark: 'dong-phu', title: 'Động Phủ', subtitle: 'Bế quan tu luyện', x: 22, y: 74 },
	{ screen: 'faction', landmark: 'the-luc', title: 'Thế Lực', subtitle: 'Sơn môn của bạn', x: 76, y: 59 },
	{ screen: 'companion', landmark: 'dao-hoa-dinh', title: 'Đào Hoa Đình', subtitle: 'Đạo lữ song tu', x: 40, y: 66 },
	{ screen: 'trial', landmark: 'bi-canh', title: 'Bí Cảnh', subtitle: 'Thử luyện kiếm tài nguyên', x: 87, y: 69 },
	{ screen: 'war', landmark: 'chien-truong', title: 'Chiến Trường', subtitle: 'Đại chiến Vu Yêu — sắp khai mở', x: 62, y: 76 }
];

/** Screen of each trial kind (trials.yaml id → route). Kinds without a screen yet are absent. */
export const trialRoutes: Record<string, string> = { tam_ma: '/play/tam-ma', hoa_pham: '/play/hoa-pham' };
