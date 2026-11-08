import { afterEach, describe, expect, it, vi } from 'vitest';

// a manual loop so the field's subscription is observable
const subs = new Set<(dt: number, now: number) => void>();
vi.mock('../loop', () => ({
	gameLoop: () => ({
		subscribe: (fn: (dt: number, now: number) => void) => {
			subs.add(fn);
			return () => subs.delete(fn);
		}
	})
}));
const frame = (dt: number) => [...subs].forEach((f) => f(dt, 0));

const { ParticleField } = await import('./layer');
const { ParticleSystem } = await import('./particles');
const { qiBurst } = await import('./presets');

afterEach(() => subs.clear());

describe('ParticleField (loop only while alive)', () => {
	it('subscribes on a burst and leaves the loop when the last particle dies', () => {
		const f = new ParticleField(new ParticleSystem('full'));
		expect(f.running).toBe(false);
		f.burst(qiBurst, 0, 0);
		expect(f.running).toBe(true);
		for (let i = 0; i < 12; i++) frame(100);
		expect(f.particles.alive).toBe(0);
		expect(f.running).toBe(false);
	});
	it('effects off: no particles, no loop, emitters idle', () => {
		const f = new ParticleField(new ParticleSystem('off'));
		expect(f.burst(qiBurst, 0, 0)).toBe(0);
		const spawn = vi.fn();
		f.addEmitter(spawn);
		expect(f.running).toBe(false);
		f.setQuality('full');
		expect(f.running).toBe(true);
		frame(16);
		expect(spawn).toHaveBeenCalledOnce();
		f.setQuality('off');
		expect(f.running).toBe(false);
		expect(f.particles.alive).toBe(0);
	});
	it('paused (scene covered): cleared and silent until resumed; reset drops emitters', () => {
		const f = new ParticleField(new ParticleSystem('full'));
		const off = f.addEmitter((_, ps) => ps.spawn(qiBurst, 0, 0, 1));
		frame(16);
		expect(f.particles.alive).toBe(1);
		f.setPaused(true);
		expect(f.particles.alive).toBe(0);
		expect(f.running).toBe(false);
		expect(f.burst(qiBurst, 0, 0)).toBe(0);
		f.setPaused(false);
		expect(f.running).toBe(true);
		off();
		f.reset();
		expect(f.emitters).toBe(0);
		expect(f.running).toBe(false);
	});
});
