// Retry with backoff for public, per-IP rate-limited GETs the shell loads at startup (faction roster, cosmetics
// catalogue). A 429/5xx/network miss is retried; any other 4xx is final. Screen loaders (+page.ts) don't retry: they
// must not hold a navigation for the backoff.
import { ApiError } from '$lib/api/client';

export const RETRY_MS = [2_000, 6_000, 15_000];

export const retryable = (e: unknown) => !(e instanceof ApiError) || e.status === 429 || e.status >= 500;

/** Run `fn`; on a retryable failure wait RETRY_MS[attempt] and try again (only when `retry`). Never throws. */
export async function withRetry(fn: () => Promise<void>, retry: boolean, delays: number[] = RETRY_MS): Promise<boolean> {
	for (let attempt = 0; ; attempt++) {
		try {
			await fn();
			return true;
		} catch (e) {
			const wait = retry ? delays[attempt] : undefined;
			if (!retryable(e) || wait === undefined) return false; // caller keeps its last known state
			await new Promise((r) => setTimeout(r, wait));
		}
	}
}
