// Game screens and world-map locations: one table drives the dock, the world map and the "coming soon" states.
// status: live = backed by the real API · preview = UI on labelled sample data (src/lib/mock) · soon = not built.
import type { IconName } from '$lib/components/Icon.svelte';

export type ScreenStatus = 'live' | 'preview' | 'soon';

export interface Screen {
	id: string;
	label: string;
	icon: IconName;
	href: string;
	status: ScreenStatus;
}

export const screens = {
	world: { id: 'world', label: 'Thế Giới', icon: 'map', href: '/play', status: 'live' },
	cultivate: { id: 'cultivate', label: 'Tu Luyện', icon: 'lotus', href: '/play/tu-luyen', status: 'live' },
	character: { id: 'character', label: 'Nhân Vật', icon: 'person', href: '/play/nhan-vat', status: 'live' },
	inventory: { id: 'inventory', label: 'Túi Đồ', icon: 'bag', href: '/play/tui-do', status: 'live' },
	faction: { id: 'faction', label: 'Thế Lực', icon: 'sect', href: '/play/the-luc', status: 'preview' },
	ranking: { id: 'ranking', label: 'Xếp Hạng', icon: 'rank', href: '/play/xep-hang', status: 'live' },
	breakthrough: { id: 'breakthrough', label: 'Đột Phá', icon: 'bolt', href: '/play/dot-pha', status: 'live' },
	timeline: { id: 'timeline', label: 'Thiên Đạo Sử', icon: 'scroll', href: '/play/thien-dao', status: 'preview' },
	companion: { id: 'companion', label: 'Đạo Lữ', icon: 'heart', href: '/play/dao-lu', status: 'preview' },
	trial: { id: 'trial', label: 'Bí Cảnh', icon: 'portal', href: '/play/bi-canh', status: 'soon' },
	war: { id: 'war', label: 'Đại Chiến', icon: 'bolt', href: '/play/dai-chien', status: 'soon' }
} satisfies Record<string, Screen>;

export type ScreenId = keyof typeof screens;

/** Bottom navigation, in order (GDD + art brief). */
export const dockScreens: Screen[] = [screens.world, screens.cultivate, screens.character, screens.inventory, screens.faction, screens.ranking];

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
	{ screen: 'timeline', landmark: 'tu-tieu-cung', title: 'Tử Tiêu Cung', subtitle: 'Thiên đạo sử · giảng đạo', x: 50, y: 47 },
	{ screen: 'cultivate', landmark: 'dong-phu', title: 'Động Phủ', subtitle: 'Bế quan tu luyện', x: 22, y: 74 },
	{ screen: 'faction', landmark: 'the-luc', title: 'Thế Lực', subtitle: 'Sơn môn của bạn', x: 76, y: 59 },
	{ screen: 'companion', landmark: 'dao-hoa-dinh', title: 'Đào Hoa Đình', subtitle: 'Đạo lữ song tu', x: 40, y: 66 },
	{ screen: 'trial', landmark: 'bi-canh', title: 'Bí Cảnh', subtitle: 'Thử luyện — sắp khai mở', x: 87, y: 69 },
	{ screen: 'war', landmark: 'chien-truong', title: 'Chiến Trường', subtitle: 'Đại chiến — sắp khai mở', x: 62, y: 76 }
];
