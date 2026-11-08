import { describe, expect, it } from 'vitest';
import { MAX_WINDOWS, pushWindow, updateTop, windowFromPath, windowFromSearch, windowHref, type WindowState } from './windowStack';

/**
 * Browser-history model of the shell: each entry is { path, wins }. openWindow pushes or replaces like
 * lib/game/windows.ts; Back (and Esc / swipe-down, which call history.back()) pops one entry.
 */
class History {
	entries: { path: string; wins: WindowState[] }[];
	constructor(path: string) {
		this.entries = [{ path, wins: [] }];
	}
	get top() {
		return this.entries.at(-1)!;
	}
	open(w: WindowState) {
		const { stack, mode } = pushWindow(this.top.wins, w);
		if (mode === 'push') this.entries.push({ path: this.top.path, wins: stack });
		else this.entries[this.entries.length - 1] = { path: this.top.path, wins: stack };
	}
	navigate(path: string) {
		this.entries.push({ path, wins: [] });
	}
	back() {
		if (this.entries.length > 1) this.entries.pop();
	}
}

const kinds = (ws: WindowState[]) => ws.map((w) => w.kind);

describe('window stack', () => {
	it('stacks at most two windows; a third replaces the top', () => {
		let s: WindowState[] = [];
		const a = pushWindow(s, { kind: 'character' });
		expect(a.mode).toBe('push');
		s = a.stack;
		const b = pushWindow(s, { kind: 'vip' });
		expect(b.mode).toBe('push');
		s = b.stack;
		const c = pushWindow(s, { kind: 'tasks' });
		expect(c.mode).toBe('replace');
		expect(kinds(c.stack)).toEqual(['character', 'tasks']);
		expect(c.stack.length).toBe(MAX_WINDOWS);
	});

	it('the same kind is replaced (moved to the top), never duplicated', () => {
		const s: WindowState[] = [{ kind: 'inventory', tab: 'bag' }, { kind: 'profile', characterId: 'x', name: 'A', stageId: 's' }];
		const r = pushWindow(s, { kind: 'inventory', tab: 'trade' });
		expect(r.mode).toBe('replace');
		expect(kinds(r.stack)).toEqual(['profile', 'inventory']);
		expect(r.stack[1]).toEqual({ kind: 'inventory', tab: 'trade' });
	});

	it('updateTop changes only the top window', () => {
		expect(updateTop([{ kind: 'character' }, { kind: 'ranking' }], { kind: 'ranking', tab: 'faction' })).toEqual([
			{ kind: 'character' },
			{ kind: 'ranking', tab: 'faction' }
		]);
		expect(updateTop([], { kind: 'menu' })).toEqual([{ kind: 'menu' }]);
	});

	it('Back / Esc close the top window first, then the one under it, then leave the scene', () => {
		const h = new History('/play');
		h.navigate('/play/tu-luyen');
		h.open({ kind: 'profile', characterId: 'x', name: 'A', stageId: 's' });
		h.open({ kind: 'inventory', tab: 'trade' });
		h.open({ kind: 'companion' }); // full: replaces the top, same depth
		expect(kinds(h.top.wins)).toEqual(['profile', 'companion']);
		h.back();
		expect(kinds(h.top.wins)).toEqual(['profile']);
		expect(h.top.path).toBe('/play/tu-luyen');
		h.back();
		expect(h.top.wins).toEqual([]);
		expect(h.top.path).toBe('/play/tu-luyen');
		h.back();
		expect(h.top.path).toBe('/play');
	});
});

describe('window deep links', () => {
	it('old screen routes map to windows; scenes and mini games do not', () => {
		expect(windowFromPath('/play/nhan-vat')).toEqual({ kind: 'character' });
		expect(windowFromPath('/play/xep-hang/')).toEqual({ kind: 'ranking' });
		expect(windowFromPath('/play/tui-do', '?tab=trang-phuc')).toEqual({ kind: 'inventory', tab: 'cosmetics' });
		expect(windowFromPath('/play/tui-do', '?tab=trao-doi&den=c1&ten=Lan&canh-gioi=luyen_khi.tang_3')).toEqual({
			kind: 'inventory',
			tab: 'trade',
			to: { characterId: 'c1', name: 'Lan', stageId: 'luyen_khi.tang_3' }
		});
		expect(windowFromPath('/play/dao-lu', '?cau-than=c2&ten=Mai')).toEqual({
			kind: 'companion',
			propose: { characterId: 'c2', name: 'Mai', stageId: '' }
		});
		for (const p of ['/play', '/play/tu-luyen', '/play/tu-tieu-cung', '/play/tam-ma', '/play/bi-canh', '/play/x/y', '/login']) {
			expect(windowFromPath(p)).toBeNull();
		}
	});

	it('?win= links: argument-free windows, windows with parameters; profile and unknown kinds are refused', () => {
		expect(windowFromSearch('?win=settings')).toEqual({ kind: 'settings' });
		expect(windowFromSearch('?win=ranking&tab=the-luc')).toEqual({ kind: 'ranking', tab: 'faction' });
		expect(windowFromSearch('?win=help&chu-de=hall')).toEqual({ kind: 'help', topic: 'hall' });
		expect(windowFromSearch('?win=help&chu-de=nope')).toBeNull();
		expect(windowFromSearch('?win=sealed&noi=dai-chien')).toEqual({ kind: 'sealed', screen: 'dai-chien' });
		expect(windowFromSearch('?win=profile')).toBeNull();
		expect(windowFromSearch('?win=bogus')).toBeNull();
		expect(windowFromSearch('')).toBeNull();
	});

	it('windowHref round-trips through windowFromSearch', () => {
		const ws: WindowState[] = [
			{ kind: 'tasks' },
			{ kind: 'inventory', tab: 'shop' },
			{ kind: 'inventory', tab: 'trade', to: { characterId: 'c1', name: 'Lan Anh', stageId: 'kim_dan.tang_1' } },
			{ kind: 'ranking', tab: 'faction' },
			{ kind: 'companion', propose: { characterId: 'c2', name: 'Mai', stageId: 'x.y' } },
			{ kind: 'help', topic: 'dong_phu' },
			{ kind: 'sealed', screen: 'dai-chien' }
		];
		for (const w of ws) {
			const href = windowHref(w);
			expect(href.startsWith('/play?')).toBe(true);
			expect(windowFromSearch(href.slice('/play'.length))).toEqual(w);
		}
	});
});
