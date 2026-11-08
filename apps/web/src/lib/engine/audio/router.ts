// Audio router (lazy chunk, started by the game shell): fxBus events and button presses → sfxFor → AudioManager.
// It only listens: never calls the API, never derives a value. While sound is off (muted, or not yet unlocked) it does
// no work at all — no timers, no AudioContext calls. Bursts are coalesced twice: sfxFor returns one cue per event
// (a reward = its rarest item), and the manager drops repeats of an id inside COALESCE_MS / beyond MAX_VOICES.
// A press that opens a window plays the window sound only (the click waits PRESS_DELAY_MS and is cancelled).
import { game } from '$lib/game/store.svelte';
import { fxBus as defaultBus, type FxBus } from '../fx/bus';
import type { FxEvent } from '../fx/events';
import { settings as defaultSettings } from '../settings.svelte';
import type { AudioManager } from './AudioManager';
import { sfxFor, STAGE_UP_QUIET_MS, type SfxContext, type SfxCue, type SfxInput } from './sfxMap';

export const PRESS_DELAY_MS = 40;
/** elements whose press clicks (inside an element marked data-sfx="off", nothing) */
const PRESSABLE = 'button, [role="button"], [role="menuitem"], [role="tab"], [role="radio"], a.btn, summary';

export interface RouterDeps {
	audio: AudioManager;
	bus?: FxBus;
	context?: () => SfxContext;
	/** DOM target for the press listener (null = none) */
	target?: Pick<EventTarget, 'addEventListener' | 'removeEventListener'> | null;
	now?: () => number;
	setTimeout?: (fn: () => void, ms: number) => unknown;
	clearTimeout?: (t: unknown) => void;
}

const browserContext = (): SfxContext => ({
	quality: defaultSettings.fx,
	reducedMotion: defaultSettings.reducedMotion,
	rarityOf: (id) => game.catalogue?.items.get(id)?.rarity,
	selfId: game.character?.id ?? null
});

export function startAudioRouter(d: RouterDeps): () => void {
	const bus = d.bus ?? defaultBus;
	const ctx = d.context ?? browserContext;
	const now = d.now ?? (() => performance.now());
	const later = d.setTimeout ?? ((fn: () => void, ms: number) => setTimeout(fn, ms));
	const cancel = d.clearTimeout ?? ((t: unknown) => clearTimeout(t as ReturnType<typeof setTimeout>));
	const a = d.audio;
	const timers = new Set<unknown>();
	let pendingPress: unknown = null;
	let quietStageUpUntil = -Infinity;

	const fire = (c: SfxCue) => void a.play(c.id, c.ch);
	const schedule = (cues: SfxCue[]) => {
		for (const c of cues) {
			if (c.at <= 0) fire(c);
			else {
				const t = later(() => {
					timers.delete(t);
					fire(c);
				}, c.at);
				timers.add(t);
			}
		}
	};

	function handle(e: SfxInput) {
		if (!a.running) return; // sound off / locked / tab hidden: zero work
		if (e.type === 'stageUp' && now() < quietStageUpUntil) return;
		if (e.type === 'breakthrough') quietStageUpUntil = now() + STAGE_UP_QUIET_MS;
		if (e.type === 'breakthroughCancelled') clearAll();
		if (e.type === 'window' && pendingPress !== null) {
			cancel(pendingPress);
			pendingPress = null;
		}
		schedule(sfxFor(e, ctx()));
	}

	function clearAll() {
		for (const t of timers) cancel(t);
		timers.clear();
	}

	const offBus = bus.onAny((e: FxEvent) => handle(e));

	const onPress = (ev: Event) => {
		if (!a.running || pendingPress !== null) return;
		const el = (ev.target as Element | null)?.closest?.(PRESSABLE);
		if (!el || el.closest('[data-sfx="off"]') || (el as HTMLButtonElement).disabled || el.getAttribute('aria-disabled') === 'true') return;
		pendingPress = later(() => {
			pendingPress = null;
			handle({ type: 'press' });
		}, PRESS_DELAY_MS);
	};
	d.target?.addEventListener('click', onPress, true);

	return () => {
		offBus();
		d.target?.removeEventListener('click', onPress, true);
		clearAll();
		if (pendingPress !== null) cancel(pendingPress);
		pendingPress = null;
	};
}
