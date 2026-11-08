import { describe, expect, it } from 'vitest';
import { countUpAt, countUpMs, isRare, motesForEarned, orbFlightMs, rewardFlights } from './presets';

describe('earned → orbs (Thu nhận)', () => {
	it('count is capped 8..24 at full whatever the server value, monotone in it', () => {
		let prev = 0;
		for (const v of [1, 10, 100, 1e3, 1e4, 1e5, 1e6, 1e9, 1e15, Number.MAX_SAFE_INTEGER]) {
			const n = motesForEarned(v, 'full');
			expect(n).toBeGreaterThanOrEqual(8);
			expect(n).toBeLessThanOrEqual(24);
			expect(n).toBeGreaterThanOrEqual(prev);
			prev = n;
		}
		expect(motesForEarned(Number.NaN, 'full')).toBe(0);
		expect(motesForEarned(Infinity, 'lite')).toBe(8);
	});
	it('flight lasts 600–900 ms (lite 600, off 0)', () => {
		expect(orbFlightMs(1, 'full')).toBe(600);
		expect(orbFlightMs(1e12, 'full')).toBe(900);
		expect(orbFlightMs(5000, 'lite')).toBe(600);
		expect(orbFlightMs(5000, 'off')).toBe(0);
		expect(orbFlightMs(0, 'full')).toBe(0);
	});
});

describe('count-up', () => {
	it('starts at from, ends exactly at to, eases out, integers only', () => {
		expect(countUpAt(100, 600, 0, 500)).toBe(100);
		expect(countUpAt(100, 600, 500, 500)).toBe(600);
		expect(countUpAt(100, 600, 9999, 500)).toBe(600);
		const mid = countUpAt(100, 600, 250, 500);
		expect(mid).toBeGreaterThan(350); // ease-out: past the linear midpoint
		expect(Number.isInteger(mid)).toBe(true);
		expect(countUpAt(600, 100, 250, 500)).toBeLessThan(350); // counts down too
		expect(countUpAt(5, 9, 10, 0)).toBe(9);
	});
	it('duration: off = jump, lite short, full log-scaled 420–900', () => {
		expect(countUpMs(1000, 'off')).toBe(0);
		expect(countUpMs(0, 'full')).toBe(0);
		expect(countUpMs(1000, 'lite')).toBe(300);
		expect(countUpMs(1, 'full')).toBe(420);
		expect(countUpMs(1e12, 'full')).toBe(900);
		expect(countUpMs(-1e4, 'full')).toBe(countUpMs(1e4, 'full'));
	});
});

describe('rewards', () => {
	it('rarity huyền and above is rare', () => {
		expect(['pham', 'linh', undefined, 'x'].map(isRare)).toEqual([false, false, false, false]);
		expect(['huyen', 'dia', 'thien', 'tien'].map(isRare)).toEqual([true, true, true, true]);
	});
	it('flights are capped per quality', () => {
		expect(rewardFlights(10, 'full')).toBe(4);
		expect(rewardFlights(10, 'lite')).toBe(2);
		expect(rewardFlights(10, 'off')).toBe(0);
		expect(rewardFlights(1, 'full')).toBe(1);
	});
});
