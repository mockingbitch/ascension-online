// Pure fps watchdog of the 3D renderer: trips when the measured frame rate stays under `minFps` for `windowMs` of
// rendering (after a warm-up grace for shader compilation). Fed with the loop's frame timestamps; long gaps (a pause,
// a hidden tab, a stall ≥ `gapMs`) restart the measurement instead of counting as slow time.

export interface WatchdogOptions {
	minFps: number;
	windowMs: number;
	graceMs: number;
	/** a frame gap at least this long is a pause, not slowness */
	gapMs: number;
}

export const WATCHDOG_DEFAULTS: WatchdogOptions = { minFps: 40, windowMs: 5000, graceMs: 2000, gapMs: 1000 };

/** Evaluated in buckets this long, so one janky frame never decides. */
const BUCKET_MS = 500;

export class FpsWatchdog {
	readonly opts: WatchdogOptions;
	#last: number | null = null;
	#warm = 0;
	#bucketMs = 0;
	#bucketFrames = 0;
	#slowMs = 0;
	#tripped = false;

	constructor(opts: Partial<WatchdogOptions> = {}) {
		this.opts = { ...WATCHDOG_DEFAULTS, ...opts };
	}

	get tripped() {
		return this.#tripped;
	}

	/** Slow time accumulated so far (ms). */
	get slowMs() {
		return this.#slowMs;
	}

	/** Restart measuring (mount, resume, quality change). Keeps a trip. */
	reset() {
		this.#last = null;
		this.#warm = 0;
		this.#bucketMs = 0;
		this.#bucketFrames = 0;
		this.#slowMs = 0;
	}

	/** Feed one rendered frame at `now` (ms). Returns true once tripped. */
	frame(now: number): boolean {
		if (this.#tripped) return true;
		const last = this.#last;
		this.#last = now;
		if (last === null) return false;
		const dt = now - last;
		if (!(dt > 0)) return false;
		if (dt >= this.opts.gapMs) {
			// a pause / hidden tab: start over (with a fresh grace)
			this.reset();
			this.#last = now;
			return false;
		}
		if (this.#warm < this.opts.graceMs) {
			this.#warm += dt;
			return false;
		}
		this.#bucketMs += dt;
		this.#bucketFrames++;
		if (this.#bucketMs >= BUCKET_MS) {
			const fps = (this.#bucketFrames * 1000) / this.#bucketMs;
			this.#slowMs = fps < this.opts.minFps ? this.#slowMs + this.#bucketMs : 0;
			this.#bucketMs = 0;
			this.#bucketFrames = 0;
			if (this.#slowMs >= this.opts.windowMs) this.#tripped = true;
		}
		return this.#tripped;
	}
}
