import { describe, expect, it } from 'vitest';
import { MAX_DPR, PARTICLE_CAP, ParticleSystem } from './particles';
import { motesForEarned, qiBurst } from './presets';

const seq = () => {
	let i = 0;
	return () => ((i = (i * 9301 + 49297) % 233280), i / 233280);
};

describe('ParticleSystem', () => {
	it('never exceeds the cap of its quality', () => {
		for (const q of ['full', 'lite', 'off'] as const) {
			const ps = new ParticleSystem(q, seq());
			for (let k = 0; k < 40; k++) ps.spawn(qiBurst, 100, 100);
			expect(ps.alive).toBe(PARTICLE_CAP[q]);
		}
		expect(PARTICLE_CAP).toEqual({ full: 300, lite: 80, off: 0 });
	});

	it('spawns fewer per burst at lite and drops the excess when quality is lowered', () => {
		const full = new ParticleSystem('full', seq());
		const lite = new ParticleSystem('lite', seq());
		expect(full.spawn(qiBurst, 0, 0)).toBe(24);
		expect(lite.spawn(qiBurst, 0, 0)).toBe(9);
		for (let k = 0; k < 10; k++) full.spawn(qiBurst, 0, 0);
		full.setQuality('lite');
		expect(full.alive).toBe(80);
	});

	it('recycles the pool as particles die', () => {
		const ps = new ParticleSystem('full', seq());
		ps.spawn(qiBurst, 0, 0);
		ps.update(500);
		expect(ps.alive).toBe(24); // lifeMs ≥ 600
		ps.update(500);
		expect(ps.alive).toBe(0);
		expect(ps.spawn(qiBurst, 0, 0)).toBe(24);
	});

	it('caps the device pixel ratio', () => {
		const ps = new ParticleSystem();
		ps.resize(100, 100, 3);
		expect(ps.dpr).toBe(MAX_DPR);
		ps.resize(100, 100, 0);
		expect(ps.dpr).toBe(1);
	});
});

describe('motesForEarned', () => {
	it('is log-scaled, bounded 8..24, reduced at lite, zero when off or nothing earned', () => {
		expect(motesForEarned(0, 'full')).toBe(0);
		expect(motesForEarned(-5, 'full')).toBe(0);
		expect(motesForEarned(10, 'full')).toBe(8);
		expect(motesForEarned(1_000_000, 'full')).toBe(24);
		expect(motesForEarned(1e12, 'full')).toBe(24);
		expect(motesForEarned(1_000_000, 'lite')).toBe(8);
		expect(motesForEarned(1_000_000, 'off')).toBe(0);
		expect(motesForEarned(100_000, 'full')).toBeGreaterThan(motesForEarned(1_000, 'full'));
	});
});

describe('ParticleSystem path mode + custom caps', () => {
	it('path particles follow the curve from start to end, staggered, then die', () => {
		const ps = new ParticleSystem('full', seq());
		const n = ps.spawnPath(qiBurst, { from: { x: 0, y: 100 }, to: { x: 200, y: 0 }, ms: 500, stagger: 100 }, 3);
		expect(n).toBe(3);
		ps.update(250); // first halfway; the third still waiting (delay 200 → 50 left)
		expect(ps.alive).toBe(3);
		ps.update(260);
		ps.update(200);
		ps.update(200);
		expect(ps.alive).toBe(0);
	});
	it('a scene-sized pool honours its own caps; off is always zero', () => {
		const small = new ParticleSystem('full', seq(), { full: 90, lite: 30 });
		expect(small.capacity).toBe(90);
		for (let k = 0; k < 10; k++) small.spawn(qiBurst, 0, 0);
		expect(small.alive).toBe(90);
		small.setQuality('lite');
		expect(small.alive).toBe(30);
		small.setQuality('off');
		expect(small.spawn(qiBurst, 0, 0)).toBe(0);
		expect(new ParticleSystem('full', seq(), { full: 9999, lite: 9999 }).caps).toEqual({ full: 300, lite: 80, off: 0 });
	});
});
