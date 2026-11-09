// World news store (GĐ-D): the recent history (GET /announcements/recent, merged with the live WebSocket feed by id),
// the banner queue and the banner on screen. Display only — the server rendered every text. Other players' banners
// and their sound follow Cài đặt (settings.others); the ticker and the notification list always keep the news.
import { api } from '$lib/api/client';
import { fxBus } from '$lib/engine/fx/bus';
import { settings } from '$lib/engine/settings.svelte';
import { BANNER_MS, enqueue, nextBanner, wantsBanner, type Queued } from './newsRules';
import type { WorldAnnouncement } from './types';

const KEEP = 50;

class NewsStore {
	/** newest first */
	items = $state<WorldAnnouncement[]>([]);
	/** the banner on screen */
	banner = $state<Queued | null>(null);
	#queue: Queued[] = [];
	#last = { banner: -Infinity, major: -Infinity };
	#timer: ReturnType<typeof setTimeout> | null = null;
	#selfId: () => string | null = () => null;

	/** who "me" is (my own news never becomes a banner: my celebration already played) */
	setSelf(fn: () => string | null) {
		this.#selfId = fn;
	}

	#merge(list: WorldAnnouncement[]) {
		const byId = new Map(this.items.map((x) => [x.id, x]));
		for (const x of list) byId.set(x.id, x);
		this.items = [...byId.values()].sort((a, b) => b.id - a.id).slice(0, KEEP);
	}

	/** History for the list and the chat lines (on start and after every reconnect). */
	async load() {
		try {
			this.#merge((await api.recentAnnouncements()).items);
		} catch {
			/* keep what we have */
		}
	}

	/** A live announcement from the WebSocket. */
	push(a: WorldAnnouncement) {
		if (this.items.some((x) => x.id === a.id)) return;
		this.#merge([a]);
		const p = wantsBanner(a, this.#selfId(), settings.others.banner);
		if (!p) return;
		this.#queue = enqueue(this.#queue, { item: a, prominence: p, at: Date.now() });
		this.#pump();
	}

	#pump() {
		if (this.banner || this.#timer) return;
		const { show, rest, waitMs } = nextBanner(this.#queue, Date.now(), this.#last);
		this.#queue = rest;
		if (!show) {
			if (waitMs > 0) this.#timer = setTimeout(() => ((this.#timer = null), this.#pump()), waitMs);
			return;
		}
		this.banner = show;
		this.#last[show.prominence] = Date.now();
		// sound only for the large banner, and only when the player wants other players' sounds
		if (show.prominence === 'major' && settings.others.sound)
			fxBus.emit('announcement', { id: `news:${show.item.id}`, text: show.item.text, kind: 'breakthrough', sound: true });
		this.#timer = setTimeout(() => ((this.#timer = null), this.dismiss()), BANNER_MS[show.prominence]);
	}

	/** Close the banner (auto after a few seconds, or the player). */
	dismiss() {
		this.banner = null;
		if (this.#timer) clearTimeout(this.#timer);
		this.#timer = null;
		this.#pump();
	}
}

export const news = new NewsStore();
