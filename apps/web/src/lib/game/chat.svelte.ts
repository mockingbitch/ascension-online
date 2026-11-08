// World chat state. History comes from REST; live messages from the WebSocket (`chat.message` / `chat.hidden`).
// Delivery over WS is at-most-once, so after a reconnect we backfill from REST and merge by id.
import { api, ApiError, newIdempotencyKey } from '$lib/api/client';
import { fxBus } from '$lib/engine/fx/bus';
import type { ChatChannel, ChatMessage } from './types';

const KEEP = 200; // messages kept in memory per channel

export const chat = $state<{
	channel: ChatChannel | null;
	messages: ChatMessage[];
	maxChars: number;
	loaded: boolean;
	loadingOlder: boolean;
	hasOlder: boolean;
	sending: boolean;
	/** last send error shown under the input */
	error: string | null;
	/** messages arrived while the panel was not visible (mobile badge) */
	unread: number;
	visible: boolean;
	/** composer text (shared so the player card can insert a mention) */
	draft: string;
}>({
	channel: null,
	messages: [],
	maxChars: 200,
	loaded: false,
	loadingOlder: false,
	hasOlder: true,
	sending: false,
	error: null,
	unread: 0,
	visible: false,
	draft: ''
});

/** Floating chat of the shell: collapsed (latest bubbles) or expanded (tabbed panel). */
export const chatUi = $state({ open: false });

const CHANNEL = 'world';
let pendingKey: string | null = null;
let pendingText = '';

function merge(incoming: ChatMessage[]) {
	const byId = new Map(chat.messages.map((m) => [m.id, m]));
	for (const m of incoming) byId.set(m.id, m);
	chat.messages = [...byId.values()].sort((a, b) => a.id - b.id).slice(-KEEP);
}

/** Load (or backfill after a reconnect) the newest page. */
export async function loadChat() {
	try {
		const h = await api.chatHistory(CHANNEL);
		chat.channel = h.channel;
		chat.maxChars = h.maxChars;
		merge(h.messages);
		if (!chat.loaded) chat.hasOlder = h.messages.length > 0;
		chat.loaded = true;
	} catch {
		/* keep what we have; the next reconnect retries */
	}
}

export async function loadOlder() {
	const first = chat.messages[0];
	if (!first || chat.loadingOlder || !chat.hasOlder) return;
	chat.loadingOlder = true;
	try {
		const h = await api.chatHistory(CHANNEL, first.id);
		chat.hasOlder = h.messages.length > 0;
		const byId = new Map([...h.messages, ...chat.messages].map((m) => [m.id, m]));
		chat.messages = [...byId.values()].sort((a, b) => a.id - b.id); // older pages may exceed KEEP while scrolling
	} finally {
		chat.loadingOlder = false;
	}
}

/** Send; returns true when stored. The same key is reused if the user retries the same text after a network error. */
export async function sendChat(text: string): Promise<boolean> {
	if (chat.sending) return false;
	if (pendingKey === null || pendingText !== text) {
		pendingKey = newIdempotencyKey();
		pendingText = text;
	}
	chat.sending = true;
	chat.error = null;
	try {
		const m = await api.chatSend(CHANNEL, text, pendingKey);
		pendingKey = null;
		merge([m]); // the WS echo is de-duplicated by id
		return true;
	} catch (e) {
		if (e instanceof ApiError && e.status < 500) pendingKey = null;
		chat.error = e instanceof ApiError ? e.message : 'Mất kết nối, thử gửi lại.';
		return false;
	} finally {
		chat.sending = false;
	}
}

// ── realtime hooks (called by realtime.svelte.ts) ──
export function onChatMessage(m: ChatMessage) {
	if (m.channel !== CHANNEL) return;
	const known = chat.messages.some((x) => x.id === m.id);
	merge([m]);
	if (!known && !chat.visible) chat.unread = Math.min(99, chat.unread + 1);
	if (!known) fxBus.emit('chat', { message: m }); // sound only (the router skips my own lines)
}

export function onChatHidden(id: number) {
	chat.messages = chat.messages.filter((m) => m.id !== id);
}

/** Insert "@name " at the start of the composer. */
export function mention(name: string) {
	const tag = `@${name} `;
	if (!chat.draft.startsWith(tag)) chat.draft = (tag + chat.draft).slice(0, chat.maxChars);
}

export function markChatVisible(v: boolean) {
	chat.visible = v;
	if (v) chat.unread = 0;
}
