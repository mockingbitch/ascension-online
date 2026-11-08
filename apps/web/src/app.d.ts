// See https://svelte.dev/docs/kit/types#app.d.ts
import type { WindowState } from '$lib/game/windowStack';

declare global {
	namespace App {
		interface Error {
			code?: string;
		}
		/** Shallow-routing state of the game shell: the window stack over the scene (bottom → top, ≤ 2). */
		interface PageState {
			wins?: WindowState[];
			/** a local window (confirmation, item detail) pushed its own entry so Back closes it first */
			sub?: string;
			/** a full-screen mode that is not a route (the breakthrough sequence) holds this entry so Back reaches it first */
			mode?: 'breakthrough';
		}
	}
}
export {};
