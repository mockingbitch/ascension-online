// Receive-only WebSocket: world announcements (breakthroughs, story milestones), chat and the Tử Tiêu Cung hall. Reconnects with capped exponential backoff + jitter.
// Delivery is at-most-once, so every (re)connect backfills chat from REST (merged by id).
import { loadChat, onChatHidden, onChatMessage } from '$lib/game/chat.svelte';
import { onHallChat, onHallEvent } from '$lib/game/lecture.svelte';
import { loadFactionMe, loadFactions } from '$lib/game/faction.svelte';
import { loadWorld } from '$lib/game/world.svelte';
import { fxBus } from '$lib/engine/fx/bus';
import type { Announcement, ChatEvent, LectureEvent } from '$lib/game/types';

export const feed = $state<{ items: Announcement[]; connected: boolean }>({ items: [], connected: false });

const MAX_ITEMS = 5;

const KIND = {
	'announcement.major_breakthrough': 'breakthrough',
	'world.milestone_reached': 'milestone',
	'world.event_ended': 'event',
	'faction.announcement': 'faction'
} as const;

function pushFeed(msg: Announcement) {
	if (feed.items.some((m) => m.id === msg.id && m.type === msg.type)) return;
	feed.items = [msg, ...feed.items].slice(0, MAX_ITEMS);
	// the server's text, as sent: effects unroll it as a scroll (the HUD ticker shows it either way)
	fxBus.emit('announcement', { id: `${msg.type}:${msg.id}`, text: msg.data.text, kind: KIND[msg.type as keyof typeof KIND] ?? 'event' });
	if (msg.type === 'world.milestone_reached') fxBus.emit('milestone', { id: String(msg.id) });
}
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
			let msg: Announcement | ChatEvent | LectureEvent;
			try {
				msg = JSON.parse(e.data);
			} catch {
				return; // ignore malformed frames
			}
			switch (msg.type) {
				case 'announcement.major_breakthrough':
					pushFeed(msg);
					break;
				case 'world.milestone_reached':
				case 'world.event_ended':
					// The text goes to the feed; the state itself comes from GET /world (the client never derives it).
					pushFeed(msg);
					void loadWorld(true);
					break;
				case 'faction.announcement':
					// Split/migrate notices: text to the feed, then refetch membership (pending choice) and the roster.
					pushFeed(msg);
					if (chatEnabled) void Promise.all([loadFactionMe(), loadFactions()]);
					break;
				case 'chat.message':
					if (chatEnabled) onChatMessage(msg.data);
					onHallChat(msg.data);
					break;
				case 'chat.hidden':
					onChatHidden(msg.data.id);
					onHallChat(null, msg.data.id);
					break;
				case 'lecture.enter':
				case 'lecture.leave':
					onHallEvent(msg);
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
