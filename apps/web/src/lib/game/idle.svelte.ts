// Display-only idle clock between server snapshots. GET /character returns cultivation already projected to "now"
// and lastCultivatedAt = the last settlement; from those we animate:
//   shown         — stage cultivation, advancing at the idle rate, bounded by the server-reported rooms
//                   (bình cảnh, idle bucket, daily ceiling) AND by the offline window, never past them;
//   elapsed       — time banked since the last settlement (capped at the offline window).
// The next server response replaces everything; nothing here is sent back or trusted.
// UI polish U4 (A9): ONE clock for the whole shell — every reader (HUD, Động Phủ card, the world card, the action bar
// ring) sees the same `now` and the same snapshot time, so two places can never show two different tu vi values. The
// 1 s tick runs only while something reads it (createSubscriber).
import { createSubscriber } from 'svelte/reactivity';
import { game } from './store.svelte';
import { projectQi } from './qi';
import type { CharacterView as Character } from './types';

export { projectQi };

const pad = (n: number) => String(n).padStart(2, '0');
export const hms = (secs: number) => {
	const s = Math.max(0, Math.floor(secs));
	return `${pad(Math.floor(s / 3600))}:${pad(Math.floor((s % 3600) / 60))}:${pad(s % 60)}`;
};

// ── the shared clock ──
const tick = createSubscriber((update) => {
	const id = setInterval(update, 1000);
	return () => clearInterval(id);
});
let snap: Character | null = null;
let fetchedAt = Date.now();
/** the snapshot's receive time: reset whenever the store holds a new character object (a server response) */
function snapshotTime(ch: Character) {
	if (ch !== snap) {
		snap = ch;
		fetchedAt = Date.now();
	}
	return fetchedAt;
}

class IdleClock {
	get #ch() {
		return game.character;
	}
	get #now() {
		tick();
		return Date.now();
	}
	/** displayed stage cultivation (estimate; the server value replaces it on the next response) */
	get shown() {
		const ch = this.#ch;
		if (!ch) return 0;
		const at = snapshotTime(ch);
		return projectQi(ch, at, Math.max(at, this.#now));
	}
	get elapsedSeconds() {
		const ch = this.#ch;
		return ch ? Math.max(0, Math.min(ch.offlineCapSeconds, (this.#now - Date.parse(ch.lastCultivatedAt)) / 1000)) : 0;
	}
	get elapsedLabel() {
		return hms(this.elapsedSeconds);
	}
	get capSeconds() {
		return this.#ch?.offlineCapSeconds ?? 0;
	}
	get untilFullLabel() {
		return hms(this.capSeconds - this.elapsedSeconds);
	}
	get isFull() {
		return !!this.#ch && this.elapsedSeconds >= this.capSeconds;
	}
	/** bình cảnh reached: only a breakthrough moves on */
	get atBottleneck() {
		const ch = this.#ch;
		return !!ch && ch.capMilli > 0 && this.shown >= ch.requiredMilli;
	}
}

/** The one display clock (HUD, Động Phủ, world card, action bar). */
export const idleClock = new IdleClock();

/** Kept for existing callers: the shared clock. */
export const useIdleClock = () => idleClock;
