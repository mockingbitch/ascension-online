import { describe, expect, it } from 'vitest';
import { mapLocations } from '$lib/game/nav';
import { loadSceneDef } from './defs';
import { HALL_ROWS, hallSeats, rowCounts } from './defs/tu_tieu_cung';
import type { SceneId } from './types';

describe('scene defs', () => {
	it.each(['world', 'tu_tieu_cung', 'dong_phu'] as SceneId[])('%s: anchors are normalised and hotspots resolve', async (id) => {
		const def = (await loadSceneDef(id))!;
		expect(def.id).toBe(id);
		for (const [k, a] of Object.entries(def.anchors)) {
			expect(a.x, k).toBeGreaterThanOrEqual(0);
			expect(a.x, k).toBeLessThanOrEqual(1);
			expect(a.y, k).toBeGreaterThanOrEqual(0);
			expect(a.y, k).toBeLessThanOrEqual(1);
		}
		for (const h of def.hotspots) expect(def.anchors[h.anchor], h.id).toBeDefined();
	});

	it('world anchors come from the map locations', async () => {
		const def = (await loadSceneDef('world'))!;
		for (const l of mapLocations) expect(def.anchors[`loc.${l.screen}`]).toEqual({ x: l.x / 100, y: l.y / 100 });
	});

	it('a điện of 60: rows fill from the centre outwards, nearest the dais first', () => {
		const seats = hallSeats(60);
		expect(seats).toHaveLength(60);
		expect(rowCounts(60)).toEqual(HALL_ROWS.map((r) => r.weight));
		expect(seats[0]).toMatchObject({ y: HALL_ROWS[0].y, row: 0 });
		expect(Math.abs(seats[0]!.x - 0.5)).toBeLessThan(0.02); // the middle cushion first
		expect(seats[59]).toMatchObject({ y: HALL_ROWS[5].y, row: 5 });
		expect(new Set(seats.map((s) => s.seat))).toEqual(new Set([...Array(60).keys()]));
	});

	it('depth scaling: rows further from the dais are lower on screen, wider and drawn bigger', () => {
		for (let i = 1; i < HALL_ROWS.length; i++) {
			const [a, b] = [HALL_ROWS[i - 1]!, HALL_ROWS[i]!];
			expect(b.y).toBeGreaterThan(a.y);
			expect(b.scale).toBeGreaterThan(a.scale);
			expect(b.to - b.from).toBeGreaterThan(a.to - a.from);
			if (i > 1) expect(b.y - a.y).toBeGreaterThanOrEqual(a.y - HALL_ROWS[i - 2]!.y - 1e-9); // perspective
		}
	});

	it('cushions of 60 never overlap: neighbours in a row are at least one sitter apart (scaled)', () => {
		const seats = hallSeats(60);
		for (let r = 0; r < HALL_ROWS.length; r++) {
			const xs = seats.filter((s) => s.row === r).map((s) => s.x).sort((a, b) => a - b);
			for (let i = 1; i < xs.length; i++) expect(xs[i]! - xs[i - 1]!).toBeGreaterThan(0.025);
		}
		for (const s of seats) expect(s.x > 0.28 && s.x < 0.72, `seat ${s.seat}`).toBe(true); // inside the pillars
	});

	it('short screens: the seat block fits the free area (rows above the subtitles, columns clear of the dock)', () => {
		expect(hallSeats(60, null)).toEqual(hallSeats(60));
		expect(hallSeats(60, { bottom: 0.9 })).toEqual(hallSeats(60)); // room enough: unchanged
		const fit = { bottom: 0.6, left: 0.36 };
		const seats = hallSeats(60, fit);
		const plain = hallSeats(60);
		for (const s of seats) {
			expect(s.y).toBeLessThanOrEqual(0.6 + 1e-9);
			expect(s.y).toBeGreaterThanOrEqual(0.34 - 1e-9); // clear of the dais
			expect(s.x).toBeGreaterThanOrEqual(0.36 - 1e-9);
			expect(s.scale).toBeLessThanOrEqual(plain[s.seat]!.scale);
			expect(s.scale).toBeGreaterThanOrEqual(plain[s.seat]!.scale * 0.55 - 1e-9);
		}
		// a header over the top: the first row starts below it
		const below = hallSeats(60, { top: 0.45, bottom: 0.7 });
		for (const s of below) expect(s.y).toBeGreaterThanOrEqual(0.45 - 1e-9);
		expect(Math.max(...below.map((s) => s.y))).toBeLessThanOrEqual(0.7 + 1e-9);
		// rows keep their order and depth scaling
		for (let i = 1; i < HALL_ROWS.length; i++) {
			const a = seats.find((s) => s.row === i - 1)!;
			const b = seats.find((s) => s.row === i)!;
			expect(b.y).toBeGreaterThan(a.y);
			expect(b.scale).toBeGreaterThan(a.scale);
		}
	});

	it('other capacities keep every cushion (server capacity wins over the default)', () => {
		for (const n of [1, 7, 36, 59, 61, 120]) {
			expect(rowCounts(n).reduce((a, b) => a + b, 0)).toBe(n);
			expect(hallSeats(n)).toHaveLength(n);
		}
		expect(hallSeats(0)).toEqual([]);
	});
});
