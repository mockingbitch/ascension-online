// Shared story-clock state (GET /world) for the HUD indicator and the Thiên Đạo Sử screen.
// Refetched on world.* WebSocket events and at most every REFRESH_MS otherwise; the server owns every number.
import { api } from '$lib/api/client';
import type { WorldView } from './types';

export const world = $state<{ view: WorldView | null; error: boolean }>({ view: null, error: false });

const REFRESH_MS = 60_000;
let loadedAt = 0;
let inflight: Promise<void> | null = null;

/** Load the world view; `force` bypasses the freshness window (WS event, explicit refresh). */
export function loadWorld(force = false, f: typeof fetch = fetch): Promise<void> {
	if (inflight) return inflight;
	if (!force && world.view && Date.now() - loadedAt < REFRESH_MS) return Promise.resolve();
	inflight = api
		.world(f)
		.then((v) => {
			world.view = v; // replace, never merge
			world.error = false;
			loadedAt = Date.now();
		})
		.catch(() => {
			world.error = true;
		})
		.finally(() => {
			inflight = null;
		});
	return inflight;
}
