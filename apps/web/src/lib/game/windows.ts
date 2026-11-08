// Game-shell windows: features open over the current scene instead of navigating away. SvelteKit shallow routing:
// `page.state.wins` is the window stack (≤ 2, lib/game/windowStack.ts), one history entry per stacked window, so the
// browser Back button (and Esc / swipe-down, which call history.back()) closes the top window first and the scene
// never unmounts.
import { goto, pushState, replaceState } from '$app/navigation';
import { page } from '$app/state';
import { pushWindow, updateTop, type WindowState } from './windowStack';

export type { WindowKind, WindowState } from './windowStack';

/** The open windows, bottom → top. */
export const openWindows = (): WindowState[] => page.state.wins ?? [];

/** Open a window over the scene (stacked over an open window; at most two — the top one is replaced when full). */
export function openWindow(w: WindowState) {
	const { stack, mode } = pushWindow(openWindows(), w);
	if (mode === 'push') pushState('', { wins: stack });
	else replaceState('', { wins: stack });
}

/** Change the top window's state in place (its tab) — same history entry. */
export function updateWindow(w: WindowState) {
	if (openWindows().length) replaceState('', { wins: updateTop(openWindows(), w) });
}

/** Close the top window (history.back(), so the URL/back stack stay consistent). */
export function closeWindow() {
	if (openWindows().length || page.state.sub) history.back();
}

// A mode route (the mini games) closes back to where the player came from inside the game, or to
// the world when the game was opened on it directly. The shell records whether an in-app navigation happened.
let cameInApp = false;
export const markInAppNavigation = (inApp: boolean) => (cameInApp = inApp);
export function leaveRoute() {
	if (cameInApp) history.back();
	else void goto('/play');
}
