import { describe, expect, it, vi } from 'vitest';
import { Loop, MAX_DT_MS } from './loop';

/** Manual rAF + fake document visibility. */
function harness(hidden = false) {
	let next = 1;
	const queue = new Map<number, (t: number) => void>();
	const listeners = new Set<() => void>();
	const doc = {
		hidden,
		addEventListener: (_: string, fn: () => void) => void listeners.add(fn),
		removeEventListener: (_: string, fn: () => void) => void listeners.delete(fn)
	};
	const loop = new Loop({
		raf: (cb) => (queue.set(next, cb), next++),
		caf: (id) => void queue.delete(id),
		doc: doc as unknown as Document
	});
	const frame = (t: number) => {
		const cbs = [...queue.values()];
		queue.clear();
		cbs.forEach((cb) => cb(t));
	};
	const setHidden = (h: boolean) => {
		doc.hidden = h;
		listeners.forEach((fn) => fn());
	};
	return { loop, frame, setHidden, pending: () => queue.size };
}

describe('Loop', () => {
	it('runs only while it has subscribers', () => {
		const h = harness();
		expect(h.loop.running).toBe(false);
		const fn = vi.fn();
		const off = h.loop.subscribe(fn);
		expect(h.loop.running).toBe(true);
		h.frame(1000);
		h.frame(1016);
		expect(fn).toHaveBeenCalledTimes(2);
		expect(fn.mock.calls[1]![0]).toBe(16);
		off();
		expect(h.loop.running).toBe(false);
		expect(h.pending()).toBe(0);
	});

	it('pauses when the tab is hidden and resumes when visible', () => {
		const h = harness();
		const fn = vi.fn();
		h.loop.subscribe(fn);
		h.frame(0);
		h.setHidden(true);
		expect(h.loop.paused).toBe(true);
		expect(h.loop.running).toBe(false);
		expect(h.pending()).toBe(0);
		h.frame(16);
		expect(fn).toHaveBeenCalledTimes(1);
		h.setHidden(false);
		expect(h.loop.running).toBe(true);
	});

	it('starts paused when created in a hidden tab', () => {
		const h = harness(true);
		h.loop.subscribe(() => {});
		expect(h.loop.running).toBe(false);
	});

	it('stacks explicit pause reasons', () => {
		const h = harness();
		h.loop.subscribe(() => {});
		h.loop.pause('window');
		h.loop.pause('user');
		h.loop.resume('window');
		expect(h.loop.running).toBe(false);
		h.loop.resume('user');
		expect(h.loop.running).toBe(true);
	});

	it('clamps dt after a stall and measures fps', () => {
		const h = harness();
		const dts: number[] = [];
		h.loop.subscribe((dt) => dts.push(dt));
		for (let t = 0; t <= 1100; t += 1000 / 60) h.frame(t);
		expect(h.loop.fps).toBeGreaterThanOrEqual(59);
		expect(h.loop.fps).toBeLessThanOrEqual(61);
		h.frame(5000);
		expect(dts.at(-1)).toBe(MAX_DT_MS);
	});
});
