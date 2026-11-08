import { describe, expect, it } from 'vitest';
import { FpsWatchdog } from './watchdog';

/** Feed frames at `fps` for `ms`, starting at `t0`; returns the end time and whether it tripped. */
function run(w: FpsWatchdog, fps: number, ms: number, t0 = 0) {
	const step = 1000 / fps;
	let t = t0;
	let tripped = false;
	for (; t < t0 + ms; t += step) tripped = w.frame(t) || tripped;
	return { t, tripped };
}

describe('FpsWatchdog (fps < 40 for 5 s → fall back)', () => {
	it('never trips at 60 fps or at 45 fps', () => {
		expect(run(new FpsWatchdog(), 60, 30_000).tripped).toBe(false);
		expect(run(new FpsWatchdog(), 45, 30_000).tripped).toBe(false);
	});

	it('trips after the grace + 5 s of sustained 30 fps, not before', () => {
		const w = new FpsWatchdog();
		expect(run(w, 30, 6_400).tripped).toBe(false); // 2 s grace + 4.4 s slow
		expect(run(w, 30, 1_200, 6_400).tripped).toBe(true);
		expect(w.tripped).toBe(true);
	});

	it('a fast stretch resets the slow streak', () => {
		const w = new FpsWatchdog();
		let r = run(w, 30, 6_000); // grace + 4 s slow
		r = run(w, 60, 1_000, r.t);
		r = run(w, 30, 4_000, r.t);
		expect(r.tripped).toBe(false);
	});

	it('a long gap (paused / hidden) is not slowness and restarts the grace', () => {
		const w = new FpsWatchdog();
		let r = run(w, 30, 6_000);
		r = run(w, 30, 4_000, r.t + 10_000); // 10 s pause, then 4 s slow (2 s of it grace)
		expect(r.tripped).toBe(false);
		expect(w.slowMs).toBeLessThan(5_000);
	});

	it('reset() keeps a trip', () => {
		const w = new FpsWatchdog({ graceMs: 0, windowMs: 1000 });
		run(w, 10, 2_000);
		expect(w.tripped).toBe(true);
		w.reset();
		expect(w.frame(99_999)).toBe(true);
	});
});
