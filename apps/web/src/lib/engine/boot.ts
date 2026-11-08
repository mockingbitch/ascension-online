// Boot screen timing (spec §4.1), pure so it is unit-tested. The screen shows real progress (fonts, the first scene
// mounted with its art decoded, the initial game data) and leaves as soon as everything is ready — after a short
// minimum so it does not flash — or at the hard limit, so a slow request never locks the player out.
import type { Quality } from './fx/particles';

/** Never show the boot screen longer than this (ms); cached loads finish well under 2 s. */
export const BOOT_MAX_MS = 6000;

/** Shortest display when everything is already cached (ms): long enough to read the logo, never a flash. */
export function bootMinMs(q: Quality): number {
	return q === 'full' ? 700 : q === 'lite' ? 350 : 0;
}

/** Leave now? */
export function bootDone(ready: number, total: number, elapsedMs: number, q: Quality): boolean {
	if (elapsedMs >= BOOT_MAX_MS) return true;
	return ready >= total && elapsedMs >= bootMinMs(q);
}

/** 0..1 bar value. */
export const bootProgress = (ready: number, total: number) => (total > 0 ? Math.min(1, Math.max(0, ready / total)) : 1);

/**
 * Ask for "Chạm để vào"? Only when the first scene has a real music/ambience file to start (a loop the player would
 * otherwise miss), sound is on and audio is still locked. One-shot SFX never gate the boot: the first real tap
 * unlocks them. A muted player is never blocked.
 */
export function needsTapToEnter(s: { unlocked: boolean; muted: boolean; loopFile: boolean }): boolean {
	return s.loopFile && !s.muted && !s.unlocked;
}
