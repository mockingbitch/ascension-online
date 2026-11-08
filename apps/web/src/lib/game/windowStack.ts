// Window stack of the game shell (spec §3.3, §4.7) — pure, no SvelteKit imports, so it is unit-tested directly.
// Windows are shallow routing: `page.state.wins` is the stack (≤ MAX_WINDOWS), one history entry per stacked window,
// so the browser Back button (and Esc, which calls history.back()) closes the top window first, then leaves the scene.
// Old screen URLs (/play/nhan-vat, /play/tui-do?tab=…) stay valid deep links: they map to a window over the default
// scene (windowFromPath), and `?win=<kind>` links are parsed by windowFromSearch.

export const MAX_WINDOWS = 2;

export type InventoryTab = 'bag' | 'shop' | 'cosmetics' | 'trade';
export type RankingTab = 'personal' | 'faction';
export type HelpTopic = 'dong_phu' | 'hall' | 'breakthrough' | 'timeline' | 'inventory' | 'world';

/** A player picked from a card (trade composer / proposal). Display data only; the server checks every gate. */
export interface PlayerRef {
	characterId: string;
	name: string;
	stageId: string;
}

export type WindowState =
	| { kind: 'tasks' }
	| { kind: 'vip' }
	| { kind: 'notifications' }
	| { kind: 'settings' } // Cài đặt: effects, sound, renderer (client only)
	| { kind: 'character' }
	| { kind: 'inventory'; tab?: InventoryTab; to?: PlayerRef }
	| { kind: 'ranking'; tab?: RankingTab }
	| { kind: 'timeline' }
	| { kind: 'breakthrough' }
	| { kind: 'faction' }
	| { kind: 'companion'; propose?: PlayerRef }
	| { kind: 'fortune' }
	| { kind: 'menu' } // mobile "Thêm": every window/scene the bottom row has no room for
	| { kind: 'help'; topic: HelpTopic }
	| { kind: 'sealed'; screen: string } // a place that is not open yet (Đại Chiến…)
	| {
			kind: 'profile';
			characterId: string;
			name: string;
			stageId: string;
			messageId?: number;
			/** equipped cosmetics carried by the row/message (display only) */
			avatarFrameId?: string;
			titleId?: string;
			nameFx?: string;
	  };

export type WindowKind = WindowState['kind'];

export interface StackChange {
	stack: WindowState[];
	/** push = one more history entry (Back closes it); replace = same depth (Back still returns along the stack) */
	mode: 'push' | 'replace';
}

/**
 * Open `w` over the current stack. A window of the same kind is replaced in place of the old one and moved to the
 * top; when the stack is full the top window is replaced, so the stack never exceeds MAX_WINDOWS and the history
 * depth always equals the stack length.
 */
export function pushWindow(stack: readonly WindowState[], w: WindowState): StackChange {
	const rest = stack.filter((x) => x.kind !== w.kind);
	if (rest.length < stack.length) return { stack: [...rest, w], mode: 'replace' };
	if (stack.length >= MAX_WINDOWS) return { stack: [...stack.slice(0, MAX_WINDOWS - 1), w], mode: 'replace' };
	return { stack: [...stack, w], mode: 'push' };
}

/** Replace the top window's state (e.g. its tab) without changing depth. */
export function updateTop(stack: readonly WindowState[], w: WindowState): WindowState[] {
	return stack.length ? [...stack.slice(0, -1), w] : [w];
}

// ── deep links ──

/** Old screen routes → window kind (the URL keeps working; it opens the window over the default scene). */
export const windowSlugs: Partial<Record<WindowKind, string>> = {
	character: 'nhan-vat',
	inventory: 'tui-do',
	ranking: 'xep-hang',
	timeline: 'thien-dao',
	breakthrough: 'dot-pha',
	faction: 'the-luc',
	companion: 'dao-lu'
};
const kindOfSlug = new Map(Object.entries(windowSlugs).map(([k, s]) => [s, k as WindowKind]));

const inventoryTabs: Record<string, InventoryTab> = { 'hanh-trang': 'bag', 'cua-hang': 'shop', 'trang-phuc': 'cosmetics', 'trao-doi': 'trade' };
const inventorySlugs = Object.fromEntries(Object.entries(inventoryTabs).map(([s, t]) => [t, s])) as Record<InventoryTab, string>;
const helpTopics: HelpTopic[] = ['dong_phu', 'hall', 'breakthrough', 'timeline', 'inventory', 'world'];

/** Query parameters that belong to a window link (stripped from the URL once the window is open). */
export const WINDOW_PARAMS = ['win', 'tab', 'den', 'ten', 'canh-gioi', 'cau-than', 'chu-de', 'noi'] as const;

const player = (q: URLSearchParams, idKey: string): PlayerRef | undefined => {
	const id = q.get(idKey);
	return id ? { characterId: id, name: q.get('ten') || 'đạo hữu', stageId: q.get('canh-gioi') ?? '' } : undefined;
};

/** Build the window `kind` from link parameters (null when the kind cannot be linked or a parameter is missing). */
function fromParams(kind: string, q: URLSearchParams): WindowState | null {
	switch (kind) {
		case 'tasks':
		case 'vip':
		case 'notifications':
		case 'settings':
		case 'character':
		case 'timeline':
		case 'breakthrough':
		case 'faction':
		case 'fortune':
		case 'menu':
			return { kind };
		case 'inventory': {
			const tab = inventoryTabs[q.get('tab') ?? ''];
			const to = player(q, 'den');
			return { kind, ...(tab ? { tab } : {}), ...(to ? { to, tab: 'trade' as const } : {}) };
		}
		case 'ranking':
			return q.get('tab') === 'the-luc' ? { kind, tab: 'faction' } : { kind };
		case 'companion': {
			const propose = player(q, 'cau-than');
			return propose ? { kind, propose } : { kind };
		}
		case 'help': {
			const topic = q.get('chu-de') as HelpTopic | null;
			return topic && helpTopics.includes(topic) ? { kind, topic } : null;
		}
		case 'sealed': {
			const screen = q.get('noi');
			return screen ? { kind, screen } : null;
		}
		default:
			return null; // profile (needs a server row), unknown kinds
	}
}

/** `?win=<kind>&…` → window (or null). */
export function windowFromSearch(search: string): WindowState | null {
	const q = new URLSearchParams(search);
	const kind = q.get('win');
	return kind ? fromParams(kind, q) : null;
}

/** `/play/<slug>` of a former screen route → window (or null for scenes, mini games and unknown paths). */
export function windowFromPath(path: string, search = ''): WindowState | null {
	const m = /^\/play\/([^/]+)\/?$/.exec(path);
	const kind = m ? kindOfSlug.get(m[1]!) : undefined;
	return kind ? fromParams(kind, new URLSearchParams(search)) : null;
}

/** Shareable link that opens `w` over the default scene (`/play?win=…`). */
export function windowHref(w: WindowState): string {
	const q = new URLSearchParams({ win: w.kind });
	const ref = (r: PlayerRef, idKey: string) => {
		q.set(idKey, r.characterId);
		q.set('ten', r.name);
		if (r.stageId) q.set('canh-gioi', r.stageId);
	};
	if (w.kind === 'inventory') {
		if (w.tab) q.set('tab', inventorySlugs[w.tab]);
		if (w.to) ref(w.to, 'den');
	} else if (w.kind === 'ranking' && w.tab === 'faction') q.set('tab', 'the-luc');
	else if (w.kind === 'companion' && w.propose) ref(w.propose, 'cau-than');
	else if (w.kind === 'help') q.set('chu-de', w.topic);
	else if (w.kind === 'sealed') q.set('noi', w.screen);
	return `/play?${q}`;
}
