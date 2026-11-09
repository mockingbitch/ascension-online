// GĐ2: the (game) layout load must not depend on the URL (SvelteKit re-runs a load when a URL part it read changes),
// otherwise every scene/screen change re-fetched realms, items, the character and the side data. The onboarding
// guard moved to child loads that only read parent data.
import { isRedirect } from '@sveltejs/kit';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const { api, ensureSession } = vi.hoisted(() => ({
	api: { realms: vi.fn(), items: vi.fn(), character: vi.fn(), characterOrNull: vi.fn(), factions: vi.fn(), cosmeticCatalog: vi.fn() },
	ensureSession: vi.fn()
}));
vi.mock('$lib/api/client', () => {
	class ApiError extends Error {
		constructor(
			public status: number,
			public code: string
		) {
			super(code);
		}
	}
	return { api, ApiError };
});
vi.mock('$lib/api/session.svelte', () => ({ ensureSession }));
vi.mock('$lib/game/catalogue', () => ({ CatalogueIndex: class {} }));

import { _CHARACTER_DEP, load as gameLoad } from '../../routes/(game)/+layout';
import { load as playLoad } from '../../routes/(game)/play/+layout';
import { load as createLoad } from '../../routes/(game)/create/+page';
import { load as factionLoad } from '../../routes/(game)/chon-the-luc/+page';

/** A load event whose URL records every read made outside `untrack` (what SvelteKit would track). */
function event(pathname: string) {
	const tracked: string[] = [];
	let untracked = 0;
	const url = new Proxy(new URL(`http://x${pathname}`), {
		get(t, k) {
			if (!untracked) tracked.push(String(k));
			const v = Reflect.get(t, k);
			return typeof v === 'function' ? v.bind(t) : v;
		}
	});
	const deps: string[] = [];
	const ev = {
		fetch,
		url,
		depends: (...d: string[]) => deps.push(...d),
		untrack: <T>(fn: () => T) => {
			untracked++;
			try {
				return fn();
			} finally {
				untracked--;
			}
		}
	};
	return { ev, tracked, deps };
}

const run = (fn: unknown, ev: unknown) => (fn as (e: unknown) => Promise<Record<string, unknown> | void>)(ev);
async function redirectOf(p: Promise<unknown>) {
	try {
		await p;
		return null;
	} catch (e) {
		if (isRedirect(e)) return e.location;
		throw e;
	}
}

beforeEach(() => {
	vi.clearAllMocks();
	ensureSession.mockResolvedValue(true);
	api.realms.mockResolvedValue({ realms: [] });
	api.items.mockResolvedValue({ items: [] });
	api.factions.mockResolvedValue({ factions: [] });
	api.cosmeticCatalog.mockResolvedValue({ appearances: undefined });
});

describe('(game) layout load', () => {
	it('reads nothing from the URL, so scene/screen/window changes never re-run it', async () => {
		api.characterOrNull.mockResolvedValue({ id: 'c', factionId: 'vu_toc' });
		for (const path of ['/play', '/play/tu-luyen', '/play/tu-tieu-cung', '/play/nhan-vat']) {
			const { ev, tracked, deps } = event(path);
			const data = await run(gameLoad, ev);
			expect(tracked).toEqual([]);
			expect(deps).toEqual([_CHARACTER_DEP]);
			expect(data).toMatchObject({ character: { id: 'c' } });
		}
	});
	it('signed out: redirects to login with the return path (read untracked)', async () => {
		ensureSession.mockResolvedValue(false);
		const { ev, tracked } = event('/play/tu-luyen');
		expect(await redirectOf(run(gameLoad, ev))).toBe('/login?return=%2Fplay%2Ftu-luyen');
		expect(tracked).toEqual([]);
	});
});

describe('onboarding guard (child loads, parent data only)', () => {
	const parentWith = (character: unknown) => ({ parent: async () => ({ character }) });
	it('/play: no character → /create, no faction → /chon-the-luc, member → stays', async () => {
		expect(await redirectOf(run(playLoad, parentWith(null)))).toBe('/create');
		expect(await redirectOf(run(playLoad, parentWith({ factionId: null })))).toBe('/chon-the-luc');
		expect(await redirectOf(run(playLoad, parentWith({ factionId: 'vu_toc' })))).toBeNull();
	});
	it('/create and /chon-the-luc keep their guards', async () => {
		const withFetch = (c: unknown) => ({ fetch, ...parentWith(c) });
		expect(await redirectOf(run(createLoad, withFetch({ factionId: 'vu_toc' })))).toBe('/play');
		expect(await redirectOf(run(createLoad, withFetch({ factionId: null })))).toBe('/chon-the-luc');
		expect(await redirectOf(run(createLoad, withFetch(null)))).toBeNull();
		expect(await redirectOf(run(factionLoad, withFetch(null)))).toBe('/create');
		expect(await redirectOf(run(factionLoad, withFetch({ factionId: 'vu_toc' })))).toBe('/play/the-luc');
		expect(await redirectOf(run(factionLoad, withFetch({ factionId: null })))).toBeNull();
	});
});
