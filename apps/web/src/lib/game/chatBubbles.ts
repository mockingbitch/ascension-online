// Floating chat bubbles (spec §3.4): the newest messages show for a while over the scene, then fade. Pure state
// transitions so the timing rules are unit-tested; the component drives them with timers (no rAF).

export const BUBBLE_MAX = 3;
export const BUBBLE_TTL_MS = 8000;

export interface BubbleMsg {
	/** unique across channels, e.g. "world:123" */
	key: string;
	name: string;
	text: string;
	stageId: string;
	mine: boolean;
	nameFx?: string;
}

export interface Bubble extends BubbleMsg {
	until: number;
}

/** Add newly arrived messages (oldest first) at `now`; keeps the newest BUBBLE_MAX, ignores keys already shown. */
export function addBubbles(shown: readonly Bubble[], incoming: readonly BubbleMsg[], now: number): Bubble[] {
	const known = new Set(shown.map((b) => b.key));
	const fresh = incoming.filter((m) => !known.has(m.key)).map((m) => ({ ...m, until: now + BUBBLE_TTL_MS }));
	return [...shown, ...fresh].slice(-BUBBLE_MAX);
}

/** Drop bubbles whose time is up. */
export function expireBubbles(shown: readonly Bubble[], now: number): Bubble[] {
	return shown.filter((b) => b.until > now);
}

/** Delay until the next bubble expires (null when none). */
export function nextExpiry(shown: readonly Bubble[], now: number): number | null {
	return shown.length ? Math.max(0, Math.min(...shown.map((b) => b.until)) - now) : null;
}
