// Player settings for the game client (Cài đặt): effects quality, audio, renderer flag. A Svelte 5 runes store,
// persisted to localStorage (wrapped in try/catch: private mode or blocked storage just means defaults). Client
// presentation only — nothing here is sent to the server or affects game results.
import type { Quality } from './fx/particles';
import type { RendererKind } from './render/types';
import { AUDIO_CHANNELS, defaultAudioSettings, type AudioChannel, type AudioSettings } from './audio/AudioManager';

export const SETTINGS_KEY = 'tiendao.settings.v1';
const QUALITIES: Quality[] = ['full', 'lite', 'off'];

export interface StoredSettings {
	/** null = follow prefers-reduced-motion (off when reduced, else full) */
	fx: Quality | null;
	audio: AudioSettings;
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
		return { fx, audio: { muted: v.audio?.muted === true, volumes: vol } };
	} catch {
		return d;
	}
}

export class Settings {
	#env: SettingsEnv;
	/** explicit choice; null = follow reduced motion */
	fxChoice = $state<Quality | null>(null);
	audio = $state<AudioSettings>(defaultAudioSettings());
	/** what `?renderer=` asked for (shown read-only); only 2D exists before GĐ7 */
	rendererRequested = $state<string | null>(null);
	reducedMotion = $state(false);

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
		this.rendererRequested = new URLSearchParams(env.search ?? '').get('renderer');
	}

	/** Effective effects quality. */
	get fx(): Quality {
		return this.fxChoice ?? (this.reducedMotion ? 'off' : 'full');
	}

	/** Effective renderer: 2D until a 3D renderer exists (GĐ7). */
	get renderer(): RendererKind {
		return '2d';
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
			this.#env.storage?.setItem(SETTINGS_KEY, JSON.stringify({ fx: this.fxChoice, audio: this.audio } satisfies StoredSettings));
		} catch {
			/* storage full / blocked: the setting still applies for this session */
		}
	}
}

/** The app-wide settings store. */
export const settings = new Settings();
