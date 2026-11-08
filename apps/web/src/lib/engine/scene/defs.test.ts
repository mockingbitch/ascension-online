import { describe, expect, it } from 'vitest';
import { mapLocations } from '$lib/game/nav';
import { loadSceneDef } from './defs';
import { hallSeats } from './defs/tu_tieu_cung';
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

	it('hall seats fill rows from the centre outwards', () => {
		const seats = hallSeats(36);
		expect(seats).toHaveLength(36);
		expect(seats[0]).toMatchObject({ y: 0.43, scale: 0.62 });
		expect(seats[0]!.x).toBeCloseTo(0.5, 5); // 9 per row → the middle cushion first
		expect(seats[35]).toMatchObject({ y: 0.72 });
	});
});
