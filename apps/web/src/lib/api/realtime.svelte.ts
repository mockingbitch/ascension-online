// Read-only WebSocket for world announcements. Reconnects with capped exponential backoff + jitter.
import type { Announcement } from '$lib/game/types';

export const feed = $state<{ items: Announcement[]; connected: boolean }>({ items: [], connected: false });

const MAX_ITEMS = 5;
let started = false;

export function startRealtime() {
	if (started || typeof window === 'undefined') return;
	started = true;
	let attempt = 0;
	const connect = () => {
		const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
		const ws = new WebSocket(`${proto}//${location.host}/ws`);
		ws.onopen = () => {
			attempt = 0;
			feed.connected = true;
		};
		ws.onmessage = (e) => {
			try {
				const msg = JSON.parse(e.data) as Announcement;
				if (msg.type !== 'announcement.major_breakthrough' || feed.items.some((m) => m.id === msg.id)) return;
				feed.items = [msg, ...feed.items].slice(0, MAX_ITEMS);
			} catch {
				/* ignore malformed frames */
			}
		};
		ws.onclose = () => {
			feed.connected = false;
			const delay = Math.min(30_000, 1000 * 2 ** attempt++) * (0.5 + Math.random() / 2);
			setTimeout(connect, delay);
		};
	};
	connect();
}
