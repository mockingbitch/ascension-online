// AudioManager (spec §4.10): Web Audio API, no library. Channels BGM / Ambience / SFX / UI, each with its own gain
// under a master gain; mute and volumes come from the settings store (which persists them).
//   · Autoplay: nothing plays before the first user interaction. The AudioContext is only created by unlock(), called
//     from a gesture, and only while sound is on — "Tắt tiếng" means no AudioContext at all (zero audio work); a
//     context created earlier is suspended while muted or while the tab is hidden, and resumed afterwards.
//   · Loading: lazy. A file id is fetched + decoded on first use (OGG, MP3 fallback — manifest.ts picks the URL);
//     a `synth:<id>` id is rendered once by the lazy synth module (original placeholder sounds, no binary assets).
//     Unknown / missing ids are silently ignored.
//   · Loops: BGM and ambience each hold one looping track and crossfade (1 s) when the scene asks for another one.
//   · Bursts: one id plays at most once per COALESCE_MS, and at most MAX_VOICES one-shots sound together — 24 orbs
//     never become 24 sounds.

export type AudioChannel = 'bgm' | 'ambience' | 'sfx' | 'ui';
export const AUDIO_CHANNELS: AudioChannel[] = ['bgm', 'ambience', 'sfx', 'ui'];
export type LoopChannel = 'bgm' | 'ambience';
export type ShotChannel = 'sfx' | 'ui';

export interface AudioSettings {
	muted: boolean;
	/** 0..1 per channel */
	volumes: Record<AudioChannel, number>;
}

export const defaultAudioSettings = (): AudioSettings => ({ muted: false, volumes: { bgm: 0.6, ambience: 0.5, sfx: 0.8, ui: 0.7 } });

/** The part of AudioContext the manager uses (lets tests pass a mock). */
export interface AudioCtxLike {
	readonly state: string;
	readonly currentTime: number;
	readonly sampleRate: number;
	readonly destination: AudioNode;
	createGain(): GainNode;
	createBufferSource(): AudioBufferSourceNode;
	createBuffer(channels: number, length: number, sampleRate: number): AudioBuffer;
	decodeAudioData(data: ArrayBuffer): Promise<AudioBuffer>;
	resume(): Promise<void>;
	suspend(): Promise<void>;
}

export interface AudioDeps {
	createContext: () => AudioCtxLike;
	fetchBytes: (url: string) => Promise<ArrayBuffer>;
	/** render a `synth:` placeholder to mono PCM (lazy module); null = no such recipe */
	renderSynth?: (id: string, sampleRate: number) => Promise<Float32Array | null>;
	/** ms clock for burst coalescing (performance.now) */
	now?: () => number;
}

export const SYNTH_PREFIX = 'synth:';
export const CROSSFADE_S = 1;
/** the same id again within this window is dropped (a burst of rewards = one sound) */
export const COALESCE_MS = 120;
/** one-shots sounding at once; more are dropped */
export const MAX_VOICES = 6;

const clamp01 = (v: number) => (Number.isFinite(v) ? Math.min(1, Math.max(0, v)) : 0);

interface Loop {
	id: string;
	src: AudioBufferSourceNode;
	gain: GainNode;
}

export class AudioManager {
	#deps: AudioDeps;
	#ctx: AudioCtxLike | null = null;
	#master: GainNode | null = null;
	#gains = new Map<AudioChannel, GainNode>();
	#settings: AudioSettings = defaultAudioSettings();
	#urls = new Map<string, string>();
	#buffers = new Map<string, Promise<AudioBuffer | null>>();
	#loops: Record<LoopChannel, Loop | null> = { bgm: null, ambience: null };
	#want: Record<LoopChannel, string | null> = { bgm: null, ambience: null };
	#hidden = false;
	#lastAt = new Map<string, number>();
	#voices = 0;

	constructor(deps: AudioDeps) {
		this.#deps = deps;
	}

	get unlocked() {
		return this.#ctx !== null;
	}
	/** the context exists and is producing sound (not muted, tab visible) */
	get running() {
		return this.#ctx?.state === 'running';
	}
	get settings(): Readonly<AudioSettings> {
		return this.#settings;
	}
	/** one-shots currently sounding */
	get voices() {
		return this.#voices;
	}

	/** Register sounds by id (manifest): a file URL, or `synth:<id>` for a generated placeholder. */
	register(map: Record<string, string>) {
		for (const [id, url] of Object.entries(map)) this.#urls.set(id, url);
	}

	/** A sound is available for `id` (a file or a synth placeholder). */
	has(id: string | null | undefined): boolean {
		return !!id && this.#urls.has(id);
	}
	/** A real audio file (not a synth placeholder) is registered for `id`. */
	hasFile(id: string | null | undefined): boolean {
		const u = id ? this.#urls.get(id) : undefined;
		return !!u && !u.startsWith(SYNTH_PREFIX);
	}

	/** Apply settings (from the settings store). Safe before unlock; muting suspends the context. */
	configure(s: AudioSettings) {
		this.#settings = { muted: s.muted, volumes: { ...s.volumes } };
		this.#applyGains();
		this.#syncRunning();
	}

	setMuted(muted: boolean) {
		this.configure({ ...this.#settings, muted });
	}

	setVolume(ch: AudioChannel, v: number) {
		this.configure({ ...this.#settings, volumes: { ...this.#settings.volumes, [ch]: clamp01(v) } });
	}

	/** Effective gain of a channel (0 when muted). */
	gainOf(ch: AudioChannel): number {
		return this.#settings.muted ? 0 : clamp01(this.#settings.volumes[ch]);
	}

	/** Tab hidden → suspend; visible again → resume (unless muted). */
	setHidden(hidden: boolean) {
		this.#hidden = hidden;
		this.#syncRunning();
	}

	/**
	 * Create/resume the context. Call from a user gesture only (installUnlock does this). Muted = nothing is created:
	 * the next gesture after unmuting (or the unmute click itself) creates it.
	 */
	async unlock(): Promise<void> {
		if (this.#settings.muted) return;
		if (!this.#ctx) {
			const ctx = this.#deps.createContext();
			this.#ctx = ctx;
			this.#master = ctx.createGain();
			this.#master.connect(ctx.destination);
			for (const ch of AUDIO_CHANNELS) {
				const g = ctx.createGain();
				g.connect(this.#master);
				this.#gains.set(ch, g);
			}
			this.#applyGains();
		}
		if (this.#ctx.state !== 'running' && !this.#hidden) await this.#ctx.resume().catch(() => undefined);
		for (const ch of ['bgm', 'ambience'] as const) {
			const want = this.#want[ch];
			if (want && !this.#loops[ch]) void this.playLoop(ch, want);
		}
	}

	/**
	 * Unlock on pointer/key interactions with `target` until a context exists (a muted player keeps the listener, so
	 * unmuting later still unlocks on the next tap). Returns the remover.
	 */
	installUnlock(target: Pick<EventTarget, 'addEventListener' | 'removeEventListener'>): () => void {
		const events = ['pointerdown', 'keydown', 'touchend'] as const;
		const off = () => events.forEach((e) => target.removeEventListener(e, on, true));
		const on = () => {
			if (this.#settings.muted) return;
			off();
			void this.unlock();
		};
		events.forEach((e) => target.addEventListener(e, on, true));
		return off;
	}

	/** Follow `document.visibilityState` (pause when hidden). Returns the remover. */
	installVisibility(doc: Pick<Document, 'addEventListener' | 'removeEventListener' | 'visibilityState'>): () => void {
		const on = () => this.setHidden(doc.visibilityState === 'hidden');
		on();
		doc.addEventListener('visibilitychange', on);
		return () => doc.removeEventListener('visibilitychange', on);
	}

	/**
	 * One-shot sound on `ch`. Returns false when it does not play: locked, muted, hidden, unknown id, a repeat of the
	 * same id inside COALESCE_MS, or MAX_VOICES already sounding. Bookkeeping is synchronous, so a burst of calls in
	 * one tick plays once.
	 */
	async play(id: string, ch: ShotChannel = 'sfx'): Promise<boolean> {
		const ctx = this.#ctx;
		if (!ctx || ctx.state !== 'running' || this.gainOf(ch) === 0 || !this.#urls.has(id)) return false;
		const now = this.#now();
		const last = this.#lastAt.get(id);
		if (last !== undefined && now - last < COALESCE_MS) return false;
		if (this.#voices >= MAX_VOICES) return false;
		this.#lastAt.set(id, now);
		this.#voices++;
		let started = false;
		try {
			const buf = await this.#load(id);
			const out = this.#gains.get(ch);
			if (!buf || !out || this.#ctx !== ctx || ctx.state !== 'running') return false;
			const src = ctx.createBufferSource();
			src.buffer = buf;
			src.connect(out);
			src.onended = () => {
				this.#voices = Math.max(0, this.#voices - 1);
				src.disconnect();
			};
			src.start();
			started = true;
			return true;
		} finally {
			if (!started) this.#voices = Math.max(0, this.#voices - 1);
		}
	}

	/** Background music of the scene (1 s crossfade). Remembered while locked and started on unlock. */
	playBgm(id: string | null | undefined): Promise<void> {
		return this.playLoop('bgm', id ?? null);
	}

	/** A looping track on `ch` with a 1 s crossfade from the current one; null fades out. Missing ids = silence. */
	async playLoop(ch: LoopChannel, id: string | null): Promise<void> {
		this.#want[ch] = id;
		const ctx = this.#ctx;
		if (!ctx || this.#loops[ch]?.id === id) return;
		const t = ctx.currentTime;
		const old = this.#loops[ch];
		this.#loops[ch] = null;
		if (old) {
			old.gain.gain.cancelScheduledValues?.(t);
			old.gain.gain.setValueAtTime(old.gain.gain.value, t);
			old.gain.gain.linearRampToValueAtTime(0, t + CROSSFADE_S);
			old.src.stop(t + CROSSFADE_S);
		}
		if (!id || !this.#urls.has(id)) return;
		const buf = await this.#load(id);
		const out = this.#gains.get(ch);
		if (!buf || !out || this.#want[ch] !== id || this.#ctx !== ctx || this.#loops[ch]) return;
		const src = ctx.createBufferSource();
		src.buffer = buf;
		src.loop = true;
		const g = ctx.createGain();
		const t0 = ctx.currentTime;
		g.gain.setValueAtTime(0, t0);
		g.gain.linearRampToValueAtTime(1, t0 + CROSSFADE_S);
		src.connect(g);
		g.connect(out);
		src.start();
		this.#loops[ch] = { id, src, gain: g };
	}

	/** id of the loop playing on `ch` (null when none) */
	loopOf(ch: LoopChannel): string | null {
		return this.#loops[ch]?.id ?? null;
	}

	#now() {
		return this.#deps.now?.() ?? (typeof performance !== 'undefined' ? performance.now() : Date.now());
	}

	#syncRunning() {
		const ctx = this.#ctx;
		if (!ctx) return;
		const off = this.#settings.muted || this.#hidden;
		if (off && ctx.state === 'running') void ctx.suspend().catch(() => undefined);
		else if (!off && ctx.state === 'suspended') void ctx.resume().catch(() => undefined);
	}

	#applyGains() {
		if (this.#master) this.#master.gain.value = this.#settings.muted ? 0 : 1;
		for (const [ch, g] of this.#gains) g.gain.value = clamp01(this.#settings.volumes[ch]);
	}

	#load(id: string): Promise<AudioBuffer | null> {
		const url = this.#urls.get(id);
		const ctx = this.#ctx;
		if (!url || !ctx) return Promise.resolve(null);
		let p = this.#buffers.get(id);
		if (!p) {
			p = (url.startsWith(SYNTH_PREFIX) ? this.#synth(ctx, url.slice(SYNTH_PREFIX.length)) : this.#decode(ctx, url)).catch(() => null);
			p.then((b) => b ?? this.#buffers.delete(id)); // retry a failed load on a later play
			this.#buffers.set(id, p);
		}
		return p;
	}

	async #decode(ctx: AudioCtxLike, url: string): Promise<AudioBuffer | null> {
		return ctx.decodeAudioData(await this.#deps.fetchBytes(url));
	}

	async #synth(ctx: AudioCtxLike, recipe: string): Promise<AudioBuffer | null> {
		const pcm = await this.#deps.renderSynth?.(recipe, ctx.sampleRate);
		if (!pcm || pcm.length === 0) return null;
		const buf = ctx.createBuffer(1, pcm.length, ctx.sampleRate);
		buf.getChannelData(0).set(pcm);
		return buf;
	}
}

type WebkitWindow = Window & { webkitAudioContext?: typeof AudioContext };

let shared: AudioManager | null = null;

/** The app-wide audio manager (browser only). Creating it costs nothing: no context until a gesture unlocks it. */
export function audio(): AudioManager {
	shared ??= new AudioManager({
		createContext: () => {
			const C = window.AudioContext ?? (window as WebkitWindow).webkitAudioContext;
			return new C({ latencyHint: 'interactive' });
		},
		fetchBytes: (url) => fetch(url).then((r) => (r.ok ? r.arrayBuffer() : Promise.reject(new Error(`${r.status} ${url}`)))),
		renderSynth: (id, rate) => import('./synth').then((m) => m.renderSynth(id, rate))
	});
	return shared;
}
