// Regression (GĐ0): Nhân Vật showed "Chưa gia nhập thế lực" for a member when the public roster (GET /factions,
// per-IP rate limited) failed — membership must come from /factions/me, and the shell retries the roster.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const { api } = vi.hoisted(() => ({ api: { factions: vi.fn(), factionMe: vi.fn() } }));
vi.mock('$lib/api/client', () => {
	class ApiError extends Error {
		constructor(
			public status: number,
			public code: string,
			message = code
		) {
			super(message);
		}
	}
	return { api, ApiError, newIdempotencyKey: () => 'k' };
});

import { ApiError } from '$lib/api/client';
import { factions, loadFactions, myFaction } from './faction.svelte';

const roster = { factions: [{ id: 'vu_toc', name: 'Vu Tộc', color: 'cinnabar-500' }], switchCooldownSeconds: 0, eraId: 'e', meritSince: null, rankingAvailable: true };
const member = { factionId: 'vu_toc' } as never;

beforeEach(() => {
	factions.list = null;
	factions.me = null;
	vi.useFakeTimers();
});
afterEach(() => {
	vi.useRealTimers();
	vi.clearAllMocks();
});

describe('myFaction', () => {
	it('is loading (never "none") until membership is known', () => {
		expect(myFaction()).toEqual({ state: 'loading' });
	});
	it('a member without the roster is still a member', () => {
		factions.me = member;
		expect(myFaction()).toEqual({ state: 'member', id: 'vu_toc', view: undefined });
		factions.list = roster as never;
		expect(myFaction()).toMatchObject({ state: 'member', view: { name: 'Vu Tộc' } });
	});
	it('none only when the server says so', () => {
		factions.me = { factionId: null } as never;
		expect(myFaction()).toEqual({ state: 'none' });
	});
});

describe('loadFactions', () => {
	it('retries a 429 with backoff when asked to', async () => {
		api.factions.mockRejectedValueOnce(new ApiError(429, 'rate_limited', 'slow down')).mockResolvedValueOnce(roster);
		const p = loadFactions(fetch, true);
		await vi.advanceTimersByTimeAsync(2_000);
		await p;
		expect(api.factions).toHaveBeenCalledTimes(2);
		expect(factions.list).toEqual(roster);
	});
	it('does not retry by default (screen loaders) or on a 4xx other than 429', async () => {
		api.factions.mockRejectedValue(new ApiError(429, 'rate_limited', 'slow down'));
		await loadFactions(fetch);
		api.factions.mockRejectedValue(new ApiError(404, 'nope', 'nope'));
		await loadFactions(fetch, true);
		expect(api.factions).toHaveBeenCalledTimes(2);
		expect(factions.list).toBeNull();
	});
});
