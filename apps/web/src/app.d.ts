// See https://svelte.dev/docs/kit/types#app.d.ts
import type { WindowState } from '$lib/game/windows';

declare global {
	namespace App {
		interface Error {
			code?: string;
		}
		/** Shallow-routing state of the game shell: which window is open over the scene. */
		interface PageState {
			win?: WindowState;
		}
	}
}
export {};
