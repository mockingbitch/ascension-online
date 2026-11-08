// HUD presentation state shared by the effects director and the HUD (display only).
//  - holds: while orbs/items are in flight the matching HUD number keeps its old value, then counts up to the
//    SERVER value when they land (release) — or after a safety timeout, so a number is never stuck.
//  - shake / bump counters: bump → the HUD element replays its animation ({#key}).
//  - unseen: reward lines received since Túi Đồ was last opened (bag badge). A UI marker, not a game value.
import { fxBus } from './bus';

export type HoldKey = 'qi' | 'stones';

/** Longest a HUD number may wait for a landing (ms). */
export const HOLD_MAX_MS = 1600;

class HudFx {
	holds = $state<Record<HoldKey, number>>({ qi: 0, stones: 0 });
	/** bumps when the daily cap was hit (tu vi bar shakes) */
	capShake = $state(0);
	/** bumps when something lands in the bag button (bounce) */
	bagBounce = $state(0);
	unseen = $state(0);

	/** Hold `key` until release() or HOLD_MAX_MS; returns the release (idempotent). */
	hold(key: HoldKey, maxMs = HOLD_MAX_MS): () => void {
		this.holds[key]++;
		let done = false;
		const release = () => {
			if (done) return;
			done = true;
			clearTimeout(t);
			this.holds[key] = Math.max(0, this.holds[key] - 1);
		};
		const t = setTimeout(release, maxMs);
		return release;
	}

	held(key: HoldKey) {
		return this.holds[key] > 0;
	}

	clearUnseen() {
		this.unseen = 0;
	}
}

export const hudFx = new HudFx();

// Always on (not part of the lazy director): the badge must show even with effects off.
if (typeof window !== 'undefined') {
	fxBus.on('reward', (e) => {
		const n = e.items.length;
		if (n > 0) hudFx.unseen = Math.min(99, hudFx.unseen + n);
	});
	fxBus.on('settled', (e) => {
		if (e.hitDailyCap) hudFx.capShake++;
	});
}
