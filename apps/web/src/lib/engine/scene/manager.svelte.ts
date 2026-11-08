// SceneManager (spec §3.3): exactly one current scene, a window stack of at most two over it, and a Back order that
// closes windows before leaving the scene. Pure state (runes so the shell can render from it); URL sync with
// shallow routing is the shell's job (GĐ2), this class only decides what Back means.
import type { SceneId } from './types';

export const MAX_WINDOWS = 2;

export interface SceneWindow {
	kind: string;
	[key: string]: unknown;
}

export type BackResult = 'closed_window' | 'changed_scene' | 'none';

export class SceneManager {
	current = $state<SceneId>('world');
	windows = $state<SceneWindow[]>([]);
	/** scenes visited before the current one (Back returns along it) */
	history = $state<SceneId[]>([]);

	constructor(initial: SceneId = 'world') {
		this.current = initial;
	}

	get topWindow(): SceneWindow | undefined {
		return this.windows.at(-1);
	}

	/** Change scene. Open windows close (they belong to the scene they were opened over). */
	go(scene: SceneId) {
		if (scene === this.current) {
			this.windows = [];
			return;
		}
		this.history = [...this.history, this.current];
		this.current = scene;
		this.windows = [];
	}

	/**
	 * Open a window over the scene. A window of the same kind is brought to the top (replaced); when the stack is full
	 * the top window is replaced, so the stack never exceeds MAX_WINDOWS.
	 */
	open(w: SceneWindow) {
		const rest = this.windows.filter((x) => x.kind !== w.kind);
		this.windows = rest.length >= MAX_WINDOWS ? [...rest.slice(0, MAX_WINDOWS - 1), w] : [...rest, w];
	}

	/** Mirror an externally owned stack (shallow routing); keeps at most MAX_WINDOWS, the top ones. */
	setStack(ws: readonly SceneWindow[]) {
		this.windows = ws.slice(-MAX_WINDOWS);
	}

	/** Close the top window (or the one of `kind`). Returns whether a window closed. */
	close(kind?: string): boolean {
		const i = kind ? this.windows.findLastIndex((x) => x.kind === kind) : this.windows.length - 1;
		if (i < 0) return false;
		this.windows = this.windows.filter((_, j) => j !== i);
		return true;
	}

	/** Back / Esc: close the top window first; else return to the previous scene; else nothing. */
	back(): BackResult {
		if (this.close()) return 'closed_window';
		const prev = this.history.at(-1);
		if (prev === undefined) return 'none';
		this.history = this.history.slice(0, -1);
		this.current = prev;
		return 'changed_scene';
	}
}
