// Tử Tiêu Cung hall state. The server owns presence, listening time and every reward; this store only
//  - replays Hồng Quân's script on the SERVER clock (offset measured from serverNow), display only,
//  - heartbeats while the hall screen is open (one interval per open tab — the server measures time itself),
//  - mirrors seats from GET /lecture/state + WS `lecture.enter/leave`, and hall chat as speech bubbles.
import { api, ApiError, newIdempotencyKey } from '$lib/api/client';
import type {
	ChatChannel,
	ChatMessage,
	InsightResult,
	LectureEvent,
	LectureScript,
	LectureScripts,
	LectureState,
	Listener,
	NoteEntry
} from './types';

const BUBBLE_MS = 7000;
const KEEP_MESSAGES = 40;
const REFRESH_EVERY_BEATS = 4; // full state refresh (seats/counts) every N heartbeats
const DEFAULT_CHANNEL = 'tu_tieu_cung';

export interface Bubble {
	id: number;
	text: string;
	until: number;
}

/**
 * The one state the ngộ đạo control is in (derived from the server clock + server-reported claims):
 * - listening: the next ngộ đạo moment has not opened yet — `progress` runs 0→1 until it opens;
 * - ready: a moment is open, this player has not taken it and has listening time to be paid;
 * - cooldown: this player already took the latest moment — `msToNext` counts down to the next one.
 */
export type Phase = 'listening' | 'ready' | 'cooldown';

export interface InsightPhase {
	phase: Phase;
	/** latest moment that has opened (slot number) */
	slot: number;
	/** 0..1 through the current wait for the next moment (1 while ready) */
	progress: number;
	/** ms until the next moment opens */
	msToNext: number;
	/** ready only: ms until the open moment closes */
	msToClose: number;
}

class Hall {
	scripts = $state<LectureScripts | null>(null);
	state = $state<LectureState | null>(null);
	seats = $state<Record<number, Listener>>({});
	listeners = $state(0);
	messages = $state<ChatMessage[]>([]);
	bubbles = $state<Record<string, Bubble>>({});
	notebook = $state<NoteEntry[] | null>(null);
	lastInsight = $state<InsightResult | null>(null);
	busy = $state(false);
	sending = $state(false);
	error = $state<string | null>(null);
	chatError = $state<string | null>(null);
	/** local ms clock, ticks once per second while the hall is open */
	now = $state(Date.now());
	/** ms to add to the local clock to get server time */
	#offset = 0;
	/** local time of the last authoritative accrual reading */
	#accruedAt = Date.now();
	/** moments (slot numbers) this player has taken — from the server state and from successful ngộ đạo */
	claimed = $state<number[]>([]);
	/** the hall screen is open (seat taken, heartbeats running) */
	active = $state(false);
	/** hall chat composer + channel info (for the "Luận Đạo" chat tab) */
	draft = $state('');
	chatLoaded = $state(false);
	chatChannel = $state<ChatChannel | null>(null);
	maxChars = $state(200);
	unread = $state(0);
	chatVisible = false;
	#insightKey: string | null = null;
	#beatTimer: ReturnType<typeof setInterval> | null = null;
	#tickTimer: ReturnType<typeof setInterval> | null = null;
	#beats = 0;

	get channel() {
		return this.state?.chatChannel ?? DEFAULT_CHANNEL;
	}
	get serverNow() {
		return this.now + this.#offset;
	}

	/** Script and line Hồng Quân is speaking at the current server time (display only). */
	get speaking(): {
		script: LectureScript;
		line: number;
		lineLeftMs: number;
	} | null {
		const sc = this.scripts;
		if (!sc || !sc.lectures.length || sc.cycleSeconds <= 0) return null;
		const cycle = sc.cycleSeconds * 1000;
		let off = (this.serverNow - Date.parse(sc.cycleEpoch)) % cycle;
		if (off < 0) off += cycle;
		for (const script of sc.lectures) {
			const len = script.seconds * 1000;
			if (off < len) {
				for (let i = 0; i < script.lines.length; i++) {
					const l = script.lines[i]!.seconds * 1000;
					if (off < l) return { script, line: i, lineLeftMs: l - off };
					off -= l;
				}
			}
			off -= len;
		}
		return null;
	}

	/** The ngộ đạo state at the current server time (moments repeat every `everySeconds` from the cycle epoch). */
	get insight(): InsightPhase | null {
		const st = this.state;
		if (!st) return null;
		const every = st.insight.everySeconds * 1000;
		const win = Date.parse(st.insight.current.closesAt) - Date.parse(st.insight.current.opensAt);
		const off = this.serverNow - Date.parse(st.cycleEpoch);
		if (off < 0 || every <= 0) return null;
		const slot = Math.floor(off / every);
		const into = off - slot * every;
		const msToNext = every - into;
		const open = into < win;
		if (this.claimed.includes(slot)) return { phase: 'cooldown', slot, progress: into / every, msToNext, msToClose: 0 };
		if (open && st.me.inHall && this.accruedSeconds > 0) return { phase: 'ready', slot, progress: 1, msToNext, msToClose: win - into };
		return { phase: 'listening', slot, progress: into / every, msToNext, msToClose: 0 };
	}

	/** Listening time has hit the per-ngộ-đạo cap (a moment was missed): more sitting is not counted until one is taken. */
	get full() {
		return !!this.state && this.accruedSeconds >= this.state.maxSecondsPerInsight;
	}

	/** Unpaid listening shown to the player: last server reading + time since (display only, capped like the server). */
	get accruedSeconds() {
		const st = this.state;
		if (!st?.me.inHall) return 0;
		const live = st.me.accruedSeconds + Math.max(0, Math.floor((this.now - this.#accruedAt) / 1000));
		return Math.min(live, st.maxSecondsPerInsight);
	}

	#apply(st: LectureState) {
		this.#offset = Date.parse(st.serverNow) - Date.now();
		this.#accruedAt = Date.now();
		this.state = st;
		this.seats = Object.fromEntries(st.seated.map((l) => [l.seat, l]));
		this.listeners = st.listeners;
		// `current` is the latest moment that opened (even if already closed): claimed → cooldown until the next one.
		if (st.insight.claimed && !this.claimed.includes(st.insight.current.no)) this.claimed = [...this.claimed, st.insight.current.no];
	}

	/** Open the hall screen: take a seat, start heartbeats and the display clock. */
	async open() {
		if (this.active) return;
		this.active = true;
		this.error = null;
		try {
			const [sc, st] = await Promise.all([
				this.scripts ? Promise.resolve(this.scripts) : api.lectureScripts(),
				api.lectureEnter()
			]);
			if (!this.active) return void api.lectureLeave().catch(() => {});
			this.scripts = sc;
			this.#apply(st);
			void this.loadChat();
		} catch (e) {
			this.error = e instanceof ApiError ? e.message : 'Không vào được Tử Tiêu Cung. Thử lại sau.';
			this.active = false;
			return;
		}
		this.#tickTimer = setInterval(() => (this.now = Date.now()), 1000);
		this.#beatTimer = setInterval(() => void this.#beat(), (this.state?.heartbeatSeconds ?? 30) * 1000);
		addEventListener('pagehide', this.#onPageHide);
	}

	/** Leave the hall (screen closed). */
	close() {
		if (!this.active) return;
		this.active = false;
		if (this.#beatTimer) clearInterval(this.#beatTimer);
		if (this.#tickTimer) clearInterval(this.#tickTimer);
		this.#beatTimer = this.#tickTimer = null;
		removeEventListener('pagehide', this.#onPageHide);
		void api.lectureLeave().catch(() => {});
		if (this.state)
			this.state = {
				...this.state,
				me: { inHall: false, seat: -1, accruedSeconds: 0 }
			};
	}

	#onPageHide = () => {
		void api.lectureLeave(fetch, true).catch(() => {});
	};

	async #beat() {
		try {
			if (++this.#beats % REFRESH_EVERY_BEATS === 0) {
				this.#apply(await api.lectureState());
				if (!this.state?.me.inHall) this.#apply(await api.lectureEnter());
				return;
			}
			const me = await api.lectureHeartbeat();
			if (this.state) this.state = { ...this.state, me };
			this.#accruedAt = Date.now();
		} catch (e) {
			// Swept (tab slept, Redis restarted…): sit down again.
			if (e instanceof ApiError && e.code === 'not_in_hall') this.#apply(await api.lectureEnter().catch(() => this.state!));
		}
	}

	/** Ngộ đạo: the server pays the listening it measured; the result replaces local numbers. */
	async ngoDao(): Promise<InsightResult | null> {
		if (this.busy) return null;
		this.busy = true;
		this.#insightKey ??= newIdempotencyKey();
		try {
			const r = await api.lectureInsight(this.#insightKey);
			this.#insightKey = null;
			const slot = this.insight?.slot;
			if (slot !== undefined && !this.claimed.includes(slot)) this.claimed = [...this.claimed, slot];
			this.lastInsight = r;
			if (this.state)
				this.state = {
					...this.state,
					today: r.today,
					me: { ...this.state.me, accruedSeconds: 0 }
				};
			this.#accruedAt = Date.now();
			this.notebook = null; // reload on next open
			return r;
		} catch (e) {
			if (e instanceof ApiError && e.status < 500) this.#insightKey = null;
			throw e;
		} finally {
			this.busy = false;
		}
	}

	async loadNotebook() {
		this.notebook = (await api.lectureNotebook()).entries;
	}

	async loadChat() {
		try {
			const h = await api.chatHistory(this.channel);
			this.messages = h.messages.slice(-KEEP_MESSAGES);
			this.chatChannel = h.channel;
			this.maxChars = h.maxChars;
			this.chatLoaded = true;
		} catch {
			/* the WS still delivers new messages */
		}
	}

	async sendChat(text: string): Promise<boolean> {
		if (this.sending) return false;
		this.sending = true;
		this.chatError = null;
		try {
			const m = await api.chatSend(this.channel, text, newIdempotencyKey());
			this.#addMessage(m);
			return true;
		} catch (e) {
			this.chatError = e instanceof ApiError ? e.message : 'Mất kết nối, thử gửi lại.';
			return false;
		} finally {
			this.sending = false;
		}
	}

	#addMessage(m: ChatMessage) {
		if (this.messages.some((x) => x.id === m.id)) return;
		this.messages = [...this.messages, m].slice(-KEEP_MESSAGES);
		if (!this.chatVisible) this.unread = Math.min(99, this.unread + 1);
		const until = Date.now() + BUBBLE_MS;
		this.bubbles = {
			...this.bubbles,
			[m.characterId]: { id: m.id, text: m.text, until }
		};
		setTimeout(() => {
			if (this.bubbles[m.characterId]?.id === m.id) {
				const { [m.characterId]: _, ...rest } = this.bubbles;
				this.bubbles = rest;
			}
		}, BUBBLE_MS);
	}

	// ── realtime hooks ──
	onChat(m: ChatMessage | null, hiddenId?: number) {
		if (m) {
			if (this.active && m.channel === this.channel) this.#addMessage(m);
		} else if (hiddenId !== undefined) {
			this.messages = this.messages.filter((x) => x.id !== hiddenId);
			for (const [k, b] of Object.entries(this.bubbles)) {
				if (b.id === hiddenId) {
					const { [k]: _, ...rest } = this.bubbles;
					this.bubbles = rest;
				}
			}
		}
	}

	onEvent(e: LectureEvent) {
		if (!this.active) return;
		const l = e.data;
		const known = Object.values(this.seats).some((s) => s.characterId === l.characterId);
		if (e.type === 'lecture.enter') {
			if (l.seat >= 0) this.seats = { ...this.seats, [l.seat]: l };
			if (!known) this.listeners += 1;
		} else {
			const seats = { ...this.seats };
			for (const [k, s] of Object.entries(seats)) if (s.characterId === l.characterId) delete seats[Number(k)];
			this.seats = seats;
			this.listeners = Math.max(Object.keys(seats).length, this.listeners - 1);
		}
	}
}

export const hall = new Hall();

/** The "Luận Đạo" chat tab is on screen (clears its unread count). */
export function markHallChatVisible(v: boolean) {
	hall.chatVisible = v;
	if (v) hall.unread = 0;
}

export const onHallChat = (m: ChatMessage | null, hiddenId?: number) => hall.onChat(m, hiddenId);
export const onHallEvent = (e: LectureEvent) => hall.onEvent(e);
