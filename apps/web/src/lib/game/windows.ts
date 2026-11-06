// Game-shell windows: features open over the cultivation scene instead of navigating away. Uses SvelteKit
// shallow routing, so the browser Back button (and Esc) closes the window and the scene never unmounts.
import { pushState, replaceState } from '$app/navigation';
import { page } from '$app/state';

export type WindowState =
	| { kind: 'tasks' }
	| { kind: 'leaderboard' }
	| { kind: 'vip' }
	| { kind: 'breakthrough' }
	| { kind: 'chat' } // mobile: the chat panel is docked on desktop
	| { kind: 'profile'; characterId: string; name: string; stageId: string; messageId?: number };

export type WindowKind = WindowState['kind'];

/** Open a window; switching from one window to another replaces the entry so Back returns to the scene. */
export function openWindow(w: WindowState) {
	if (page.state.win) replaceState('', { win: w });
	else pushState('', { win: w });
}

export function closeWindow() {
	if (page.state.win) history.back();
}
