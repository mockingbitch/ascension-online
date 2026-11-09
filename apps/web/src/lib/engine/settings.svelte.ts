// Player settings for the game client (Cài đặt): effects quality, audio, renderer (2D / experimental 3D). A Svelte 5
// runes store, persisted to localStorage (wrapped in try/catch: private mode or blocked storage just means defaults).
// Client presentation only — nothing here is sent to the server or affects game results.
import type { Quality } from './fx/particles';
import { FALLBACK_REASONS, rendererFor, wantedRenderer, type Fallback3d, type Fallback3dReason } from './render/select';
import type { RendererKind } from './render/types';
import type { SceneId } from './scene/types';
import { AUDIO_CHANNELS, defaultAudioSettings, type AudioChannel, type AudioSettings } from './audio/AudioManager';

export const SETTINGS_KEY = 'tiendao.settings.v1';
const QUALITIES: Quality[] = ['full', 'lite', 'off'];

export interface StoredSettings {
	/** null = follow prefers-reduced-motion (off when reduced, else full) */
	fx: Quality | null;
	audio: AudioSettings;
	/** Cài đặt renderer choice; absent = '2d' (GĐ7: '3d' = experimental Tử Tiêu Cung) */
	renderer?: RendererKind;
	/** remembered 3D → 2D fallback (reason + when); absent = none */
	fallback3d?: Fallback3d;
	/** other players' news (decisions 2026-11-09 #8): banners / their sound; absent = both on. The ticker always stays. */
	others?: OthersNews;
}

export interface OthersNews {
	banner: boolean;
	sound: boolean;
}

export interface SettingsEnv {
	storage?: Pick<Storage, 'getItem' | 'setItem'> | null;
	reducedMotion?: () => boolean;
	/** current URL query (`?renderer=`) */
	search?: string;
}

function browserEnv(): SettingsEnv {
	if (typeof window === 'undefined') return {};
	let storage: Storage | null = null;
	try {
		storage = window.localStorage;
	} catch {
		storage = null;
	}
	return {
		storage,
		reducedMotion: () => window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false,
		search: window.location.search
	};
}

function parse(raw: string | null): StoredSettings {
	const d: StoredSettings = { fx: null, audio: defaultAudioSettings() };
	if (!raw) return d;
	try {
		const v = JSON.parse(raw) as Partial<StoredSettings>;
		const fx = QUALITIES.includes(v.fx as Quality) ? (v.fx as Quality) : null;
		const vol = { ...d.audio.volumes };
		for (const ch of AUDIO_CHANNELS) {
			const x = v.audio?.volumes?.[ch];
			if (typeof x === 'number' && Number.isFinite(x)) vol[ch] = Math.min(1, Math.max(0, x));
		}
		const out: StoredSettings = { fx, audio: { muted: v.audio?.muted === true, volumes: vol } };
		if (v.renderer === '3d') out.renderer = '3d';
		const fb = v.fallback3d;
		if (fb && FALLBACK_REASONS.includes(fb.reason) && typeof fb.at === 'number') out.fallback3d = { reason: fb.reason, at: fb.at };
		if (v.others) out.others = { banner: v.others.banner !== false, sound: v.others.sound !== false };
		return out;
	} catch {
		return d;
	}
}

export class Settings {
	#env: SettingsEnv;
	/** explicit choice; null = follow reduced motion */
	fxChoice = $state<Quality | null>(null);
	audio = $state<AudioSettings>(defaultAudioSettings());
	/** what `?renderer=` asked for (dev/test override of the choice) */
	rendererRequested = $state<string | null>(null);
	/** Cài đặt › Hiển thị: 2D (default) or the experimental 3D Tử Tiêu Cung */
	rendererChoice = $state<RendererKind>('2d');
	/** the 3D renderer fell back to 2D (kept until the player picks 3D again) */
	fallback3d = $state<Fallback3d | null>(null);
	/** `?fpsguard=0` disables the 3D fps watchdog (headless tests on software GL only) */
	fpsGuard = true;
	reducedMotion = $state(false);
	/** Cài đặt › Thông báo: other players' banners and their sound (default on; the ticker always stays) */
	others = $state<OthersNews>({ banner: true, sound: true });

	constructor(env: SettingsEnv = browserEnv()) {
		this.#env = env;
		let raw: string | null = null;
		try {
			raw = env.storage?.getItem(SETTINGS_KEY) ?? null;
		} catch {
			raw = null;
		}
		const s = parse(raw);
		this.fxChoice = s.fx;
		this.audio = s.audio;
		this.reducedMotion = env.reducedMotion?.() ?? false;
		this.rendererChoice = s.renderer ?? '2d';
		this.fallback3d = s.fallback3d ?? null;
		this.others = s.others ?? { banner: true, sound: true };
		const q = new URLSearchParams(env.search ?? '');
		this.rendererRequested = q.get('renderer');
		this.fpsGuard = q.get('fpsguard') !== '0';
	}

	/** Effective effects quality. */
	get fx(): Quality {
		return this.fxChoice ?? (this.reducedMotion ? 'off' : 'full');
	}

	/** Renderer the player wants (URL over the setting), ignoring per-scene support; 2D after a fallback. */
	get renderer(): RendererKind {
		return this.fallback3d ? '2d' : wantedRenderer({ requested: this.rendererRequested, choice: this.rendererChoice });
	}

	/** Renderer for one scene (only Tử Tiêu Cung has a 3D version). */
	rendererFor(scene: SceneId | null): RendererKind {
		return rendererFor(scene, { requested: this.rendererRequested, choice: this.rendererChoice, fallback: this.fallback3d });
	}

	/** Pick 2D / 3D. Picking 3D again clears a remembered fallback (the player retries). */
	setRenderer(kind: RendererKind) {
		this.rendererChoice = kind;
		if (kind === '3d') this.fallback3d = null;
		this.#save();
	}

	/** The 3D renderer gave up: remember why, so the scene stays 2D on the next visits. */
	recordFallback3d(reason: Fallback3dReason, at = Date.now()) {
		this.fallback3d = { reason, at };
		this.#save();
	}

	setOthers(o: Partial<OthersNews>) {
		this.others = { ...this.others, ...o };
		this.#save();
	}

	setFx(q: Quality | null) {
		this.fxChoice = q;
		this.#save();
	}

	setMuted(muted: boolean) {
		this.audio = { ...this.audio, muted };
		this.#save();
	}

	setVolume(ch: AudioChannel, v: number) {
		this.audio = { ...this.audio, volumes: { ...this.audio.volumes, [ch]: Math.min(1, Math.max(0, Number.isFinite(v) ? v : 0)) } };
		this.#save();
	}

	#save() {
		try {
			const v: StoredSettings = { fx: this.fxChoice, audio: this.audio };
			// renderer fields only when not default: a 2D player's stored settings stay as before GĐ7
			if (this.rendererChoice === '3d') v.renderer = '3d';
			if (this.fallback3d) v.fallback3d = this.fallback3d;
			if (!this.others.banner || !this.others.sound) v.others = this.others;
			this.#env.storage?.setItem(SETTINGS_KEY, JSON.stringify(v));
		} catch {
			/* storage full / blocked: the setting still applies for this session */
		}
	}
}

/** The app-wide settings store. */
export const settings = new Settings();
