import { describe, expect, it } from 'vitest';
import { mix, parseCssColor, PALETTE_TOKENS, readPalette } from './palette';

describe('token palette for 3D materials', () => {
	it('parses the computed colour forms browsers return', () => {
		expect(parseCssColor('rgb(232, 209, 154)')).toEqual([232 / 255, 209 / 255, 154 / 255]);
		expect(parseCssColor('rgba(10, 15, 16, 0.5)')).toEqual([10 / 255, 15 / 255, 16 / 255]);
		expect(parseCssColor('rgb(10 15 16 / 50%)')).toEqual([10 / 255, 15 / 255, 16 / 255]);
		expect(parseCssColor('color(srgb 0.5 0.25 1)')).toEqual([0.5, 0.25, 1]);
		expect(parseCssColor('#e8d19a')).toEqual([232 / 255, 209 / 255, 154 / 255]);
		expect(parseCssColor('oklch(0.7 0.1 80)')).toBeNull();
	});

	it('mix interpolates in sRGB', () => {
		expect(mix([0, 0, 0], [1, 0.5, 0.25], 0.5)).toEqual([0.5, 0.25, 0.125]);
	});

	it('reads every hall token through the resolver; an unresolved token throws', () => {
		const p = readPalette(() => 'rgb(255, 0, 0)');
		expect(Object.keys(p)).toEqual([...PALETTE_TOKENS]);
		expect(() => readPalette((v) => (v.includes('gold-300') ? '' : 'rgb(1, 2, 3)'))).toThrow(/gold-300/);
	});
});
