// GĐ1 open question 2: the cosmetics catalogue (public, per-IP rate limited) gets the same retry-with-backoff as the
// faction roster when the shell loads it.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const { api } = vi.hoisted(() => ({ api: { cosmeticCatalog: vi.fn() } }));
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
	return { api, ApiError };
});

import { ApiError } from '$lib/api/client';
import { cosmetics } from './cosmetics.svelte';
import { RETRY_MS, withRetry } from './retry';

const catalog = { cosmetics: [{ id: 'title:dao_tam', name: 'Đạo Tâm' }], seasons: [] } as never;

beforeEach(() => vi.useFakeTimers());
afterEach(() => {
	vi.useRealTimers();
	vi.clearAllMocks();
	cosmetics.catalog = null;
});

describe('withRetry', () => {
	it('backs off on 429/5xx/network and stops after the last delay', async () => {
		const fn = vi.fn().mockRejectedValue(new ApiError(503, 'down', 'down'));
		const p = withRetry(fn, true);
		for (const ms of RETRY_MS) await vi.advanceTimersByTimeAsync(ms);
		expect(await p).toBe(false);
		expect(fn).toHaveBeenCalledTimes(RETRY_MS.length + 1);
	});
	it('gives up at once on another 4xx, or when retry is off', async () => {
		const notFound = vi.fn().mockRejectedValue(new ApiError(404, 'nope', 'nope'));
		expect(await withRetry(notFound, true)).toBe(false);
		const limited = vi.fn().mockRejectedValue(new ApiError(429, 'rate_limited', 'rate_limited'));
		expect(await withRetry(limited, false)).toBe(false);
		expect(notFound).toHaveBeenCalledTimes(1);
		expect(limited).toHaveBeenCalledTimes(1);
	});
});

describe('cosmetics.load', () => {
	it('retries a 429 with backoff when the shell asks', async () => {
		api.cosmeticCatalog.mockRejectedValueOnce(new ApiError(429, 'rate_limited', 'rate_limited')).mockResolvedValueOnce(catalog);
		const p = cosmetics.load(true);
		await vi.advanceTimersByTimeAsync(RETRY_MS[0]!);
		await p;
		expect(api.cosmeticCatalog).toHaveBeenCalledTimes(2);
		expect(cosmetics.name('title:dao_tam')).toBe('Đạo Tâm');
		// loaded once per page life: later calls reuse it
		await cosmetics.load(true);
		expect(api.cosmeticCatalog).toHaveBeenCalledTimes(2);
	});
});
