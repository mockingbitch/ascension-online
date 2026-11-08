// The one requestAnimationFrame loop of the client (spec §1.5, §3.1). Canvas effects, parallax and the fps overlay
// subscribe here instead of starting their own rAF. The loop only runs while it has subscribers and no pause reason:
// a hidden tab (visibilitychange) and explicit pauses (a window covering the scene, a settings toggle) stop it.
// Display only — nothing here may compute game results.

/** Called once per frame with the time since the previous frame (ms, clamped) and the frame timestamp. */
export type FrameFn = (dtMs: number, now: number) => void;

export interface LoopDeps {
	raf: (cb: (t: number) => void) => number;
	caf: (id: number) => void;
	/** visibility source; omitted in tests that don't need it */
	doc?: Pick<Document, 'hidden' | 'addEventListener' | 'removeEventListener'>;
}

/** Longest step handed to subscribers: after a stall (tab switch, GC) effects jump at most this far. */
export const MAX_DT_MS = 100;

export class Loop {
	#subs = new Set<FrameFn>();
	#pauses = new Set<string>();
	#id: number | null = null;
	#last = 0;
	// fps: frames counted over a rolling ~1 s window
	#frames = 0;
	#windowStart: number | null = null;
	#fps = 0;
	#deps: LoopDeps;
	#onVisibility = () => this.#syncVisibility();

	constructor(deps: LoopDeps) {
		this.#deps = deps;
		deps.doc?.addEventListener('visibilitychange', this.#onVisibility);
		this.#syncVisibility();
	}

	/** Frames per second over the last second (0 while stopped). */
	get fps() {
		return this.#fps;
	}
	get running() {
		return this.#id !== null;
	}
	get paused() {
		return this.#pauses.size > 0;
	}
	get subscribers() {
		return this.#subs.size;
	}

	/** Add a per-frame callback; returns its unsubscribe. */
	subscribe(fn: FrameFn): () => void {
		this.#subs.add(fn);
		this.#update();
		return () => {
			this.#subs.delete(fn);
			this.#update();
		};
	}

	/** Pause for a named reason ('hidden' is managed by the loop itself). Nested reasons stack. */
	pause(reason = 'user') {
		this.#pauses.add(reason);
		this.#update();
	}

	resume(reason = 'user') {
		this.#pauses.delete(reason);
		this.#update();
	}

	destroy() {
		this.#deps.doc?.removeEventListener('visibilitychange', this.#onVisibility);
		this.#subs.clear();
		this.#update();
	}

	#syncVisibility() {
		if (this.#deps.doc?.hidden) this.pause('hidden');
		else this.resume('hidden');
	}

	#update() {
		const want = this.#subs.size > 0 && this.#pauses.size === 0;
		if (want && this.#id === null) {
			this.#last = 0;
			this.#frames = 0;
			this.#windowStart = null;
			this.#id = this.#deps.raf(this.#tick);
		} else if (!want && this.#id !== null) {
			this.#deps.caf(this.#id);
			this.#id = null;
			this.#fps = 0;
		}
	}

	#tick = (now: number) => {
		this.#id = this.#deps.raf(this.#tick);
		const dt = this.#last ? Math.min(MAX_DT_MS, now - this.#last) : 0;
		this.#last = now;
		this.#windowStart ??= now;
		this.#frames++;
		if (now - this.#windowStart >= 1000) {
			this.#fps = Math.round((this.#frames * 1000) / (now - this.#windowStart));
			this.#frames = 0;
			this.#windowStart = now;
		}
		for (const fn of this.#subs) fn(dt, now);
	};
}

let shared: Loop | null = null;

/** The app-wide loop (browser only). */
export function gameLoop(): Loop {
	shared ??= new Loop({
		raf: (cb) => requestAnimationFrame(cb),
		caf: (id) => cancelAnimationFrame(id),
		doc: document
	});
	return shared;
}
