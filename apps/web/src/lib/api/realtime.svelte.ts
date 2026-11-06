// Receive-only WebSocket: world announcements + chat. Reconnects with capped exponential backoff + jitter.
// Delivery is at-most-once, so every (re)connect backfills chat from REST (merged by id).
import { loadChat, onChatHidden, onChatMessage } from '$lib/game/chat.svelte';
import type { Announcement, ChatEvent } from '$lib/game/types';

export const feed = $state<{ items: Announcement[]; connected: boolean }>({ items: [], connected: false });

const MAX_ITEMS = 5;
let started = false;
let chatEnabled = false;

/** Start the socket. `withChat` (game shell, signed in) also backfills chat history on every (re)connect. */
export function startRealtime(withChat = false) {
	if (withChat && !chatEnabled) {
		chatEnabled = true;
		if (feed.connected) void loadChat();
	}
	if (started || typeof window === 'undefined') return;
	started = true;
	let attempt = 0;
	const connect = () => {
		const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
		const ws = new WebSocket(`${proto}//${location.host}/ws`);
		ws.onopen = () => {
			attempt = 0;
			feed.connected = true;
			if (chatEnabled) void loadChat();
		};
		ws.onmessage = (e) => {
			let msg: Announcement | ChatEvent;
			try {
				msg = JSON.parse(e.data);
			} catch {
				return; // ignore malformed frames
			}
			switch (msg.type) {
				case 'announcement.major_breakthrough':
					if (!feed.items.some((m) => m.id === msg.id)) feed.items = [msg, ...feed.items].slice(0, MAX_ITEMS);
					break;
				case 'chat.message':
					if (chatEnabled) onChatMessage(msg.data);
					break;
				case 'chat.hidden':
					onChatHidden(msg.data.id);
					break;
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
