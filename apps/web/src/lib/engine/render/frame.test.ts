import { describe, expect, it } from 'vitest';
import type { SceneFrame } from '../scene/types';
import { anchorInBox, anchorToScreen, frameRect } from './frame';

const hall: SceneFrame = { aspect: 16 / 9, fit: 'cover', focus: { x: 0.5, y: 0 } };
const map: SceneFrame = { aspect: 16 / 9, fit: 'contain', focus: { x: 0.5, y: 0.5 } };

describe('frameRect cover, centre-top (Tử Tiêu Cung: width = max(100cqw, 177.78cqh))', () => {
	it.each([
		['16:9 exact', 1600, 900],
		['wide 21:9', 2100, 900],
		['laptop 1440×700', 1440, 700],
		['phone portrait 375×520', 375, 520],
		['tall 360×800', 360, 800]
	])('%s', (_, w, h) => {
		const r = frameRect(w, h, hall);
		expect(r.width).toBeCloseTo(Math.max(w, (h * 16) / 9), 6);
		expect(r.height).toBeCloseTo(r.width / (16 / 9), 6);
		expect(r.width).toBeGreaterThanOrEqual(w - 1e-9);
		expect(r.height).toBeGreaterThanOrEqual(h - 1e-9);
		expect(r.y).toBe(0); // anchored top
		expect(r.x).toBeCloseTo((w - r.width) / 2, 6); // left 50% + translateX(-50%)
	});

	it('phone portrait: the dais stays centred, outer seats are cropped off-screen', () => {
		const r = frameRect(375, 520, hall); // art 924.4 × 520
		const dais = anchorInBox(r, { x: 0.5, y: 0.24 });
		expect(dais.x).toBeCloseTo(187.5, 6);
		expect(dais.y).toBeCloseTo(0.24 * 520, 6);
		const edge = anchorInBox(r, { x: 0.29, y: 0.72 });
		expect(edge.x).toBeLessThan(0);
	});

	it('wide: the bottom of the painting is cropped, the top kept', () => {
		const r = frameRect(2100, 900, hall);
		expect(r.height).toBeCloseTo(1181.25, 6);
		expect(anchorInBox(r, { x: 0.5, y: 0 }).y).toBe(0);
		expect(anchorInBox(r, { x: 0.5, y: 0.9 }).y).toBeGreaterThan(900);
	});
});

describe('frameRect contain (world map)', () => {
	it('letterboxes and centres on a tall box', () => {
		const r = frameRect(375, 600, map);
		expect(r).toMatchObject({ x: 0, width: 375 });
		expect(r.height).toBeCloseTo(210.9375, 6);
		expect(r.y).toBeCloseTo((600 - 210.9375) / 2, 6);
	});
	it('pillarboxes on a wide box', () => {
		const r = frameRect(2000, 900, map);
		expect(r.width).toBeCloseTo(1600, 6);
		expect(r.x).toBeCloseTo(200, 6);
	});
});

describe('anchorToScreen', () => {
	it('adds the box offset', () => {
		const p = anchorToScreen({ x: 10, y: 56, width: 1600, height: 900 }, hall, { x: 0.25, y: 0.5 });
		expect(p).toEqual({ x: 410, y: 506 });
	});
	it('applies a camera: the camera point moves to the frame centre', () => {
		const r = frameRect(1600, 900, map);
		const p = anchorInBox(r, { x: 0.22, y: 0.74 }, { x: 0.22, y: 0.74, zoom: 1.6 });
		expect(p.x).toBeCloseTo(800, 6);
		expect(p.y).toBeCloseTo(450, 6);
		expect(anchorInBox(r, { x: 0.3, y: 0.3 }, { x: 0.5, y: 0.5, zoom: 1 })).toEqual(anchorInBox(r, { x: 0.3, y: 0.3 }));
	});
});
