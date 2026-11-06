import { describe, expect, it } from 'vitest';
import { formatBp, formatPoints, formatQi, progressRatio, ratePerDay } from './format';

describe('format', () => {
	it('formats qi with vạn/ức', () => {
		expect(formatQi(15000)).toBe('1,5 vạn');
		expect(formatQi(300000000)).toBe('3,0 ức');
	});
	it('formats bp', () => {
		expect(formatBp(7500)).toBe('75%');
	});
	it('formats milli-points', () => {
		expect(formatPoints(33_400)).toBe('33,4');
		expect(formatPoints(150_000)).toBe('150');
		expect(ratePerDay(2083)).toBe('50 điểm/ngày');
	});
	it('clamps progress', () => {
		expect(progressRatio(150, 100)).toBe(1);
		expect(progressRatio(-1, 100)).toBe(0);
	});
});
