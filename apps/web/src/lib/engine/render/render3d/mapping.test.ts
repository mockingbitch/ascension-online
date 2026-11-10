import { describe, expect, it } from 'vitest';
import { HALL_ROWS, hallSeats, tuTieuCung } from '../../scene/defs/tu_tieu_cung';
import { anchorToScreen, frameRect } from '../frame';
import { DAIS_TOP, HALL, HALL_REF_POSE } from './layout';
import { anchorToWorld, drift, FAR, frameRay, lerpPreset, projectToFrame, viewRect, withOffset, worldToBox } from './mapping';

const pose = HALL_REF_POSE;
const anchors = Object.entries(tuTieuCung.anchors);

describe('anchor → 3D (reference camera)', () => {
	it('floor anchors land on the floor, raised anchors at their height', () => {
		for (const [k, a] of anchors) {
			const w = anchorToWorld(pose, a);
			expect(w.y, k).toBeCloseTo(a.z ?? 0, 6);
		}
		expect(anchorToWorld(pose, tuTieuCung.anchors.hong_quan!).y).toBeCloseTo(DAIS_TOP, 6);
	});

	it('round trip: projecting the 3D point gives the anchor back (parity with 2D at rest)', () => {
		for (const [k, a] of anchors) {
			const f = projectToFrame(pose, anchorToWorld(pose, a))!;
			expect(f.x, k).toBeCloseTo(a.x, 9);
			expect(f.y, k).toBeCloseTo(a.y, 9);
		}
	});

	it.each([
		[375, 667],
		[375, 520],
		[1440, 900],
		[1440, 700],
		[2100, 900]
	])('screen parity with the 2D renderer at %i×%i (cover, centre-top)', (w, h) => {
		const box = { x: 10, y: 50, width: w, height: h };
		const rect = frameRect(w, h, tuTieuCung.frame);
		for (const [k, a] of anchors) {
			const two = anchorToScreen(box, tuTieuCung.frame, a, tuTieuCung.camera![0]);
			const p = worldToBox(pose, viewRect(rect, tuTieuCung.camera![0]), anchorToWorld(pose, a))!;
			expect(Math.abs(box.x + p.x - two.x), k).toBeLessThan(1e-6);
			expect(Math.abs(box.y + p.y - two.y), k).toBeLessThan(1e-6);
		}
	});

	it('the hall is laid out sensibly: rows recede toward the dais, seats inside the pillars, sage on the dais', () => {
		const rows = HALL_ROWS.map((r) => anchorToWorld(pose, { x: 0.5, y: r.y }).z);
		for (let i = 1; i < rows.length; i++) expect(rows[i]!).toBeGreaterThan(rows[i - 1]! + 2 * HALL.cushion.radius); // cushions never touch
		const sage = anchorToWorld(pose, tuTieuCung.anchors.hong_quan!);
		expect(sage.z).toBeLessThan(rows[0]! - 3);
		expect(sage.z).toBeGreaterThan(HALL.facade.z);
		const dais = anchorToWorld(pose, tuTieuCung.anchors.dais!);
		expect(Math.abs(dais.z - sage.z)).toBeLessThan(0.1); // the qi source sits on the sage, not behind him
		for (const s of hallSeats()) {
			const w = anchorToWorld(pose, s);
			expect(Math.abs(w.x)).toBeLessThan(HALL.pillars.x - HALL.pillars.radius - HALL.cushion.radius);
			expect(w.z).toBeLessThan(HALL.floor.zTo);
		}
	});

	it('a điện of 60: neighbouring 3D cushions in a row never touch', () => {
		const seats = hallSeats(60);
		for (let r = 0; r < HALL_ROWS.length; r++) {
			const xs = seats.filter((s) => s.row === r).map((s) => anchorToWorld(pose, s).x).sort((a, b) => a - b);
			for (let i = 1; i < xs.length; i++) expect(xs[i]! - xs[i - 1]!, `row ${r}`).toBeGreaterThan(2 * HALL.cushion.radius);
		}
	});

	it('a ray that never meets its plane is placed FAR along the ray', () => {
		const a = { x: 0.5, y: 0.5, z: 50 }; // plane above the camera
		const w = anchorToWorld(pose, a);
		const d = frameRay(pose, 0.5, 0.5);
		const p = pose.position;
		expect(Math.hypot(w.x - p.x - d.x * FAR, w.y - p.y - d.y * FAR, w.z - p.z - d.z * FAR)).toBeLessThan(1e-9);
	});

	it('points behind the camera do not project', () => {
		expect(projectToFrame(pose, { x: 0, y: 9, z: 40 })).toBeNull();
	});
});

describe('camera presets and drift', () => {
	it('viewRect is the identity for the centred preset and zooms about the preset point like the 2D transform', () => {
		const r = { x: -20, y: 0, width: 1600, height: 900 };
		expect(viewRect(r, { x: 0.5, y: 0.5, zoom: 1 })).toEqual(r);
		expect(viewRect(r, null)).toEqual(r);
		const z = viewRect(r, { x: 0.25, y: 0.5, zoom: 2 });
		// the preset point lands in the middle of the original frame
		expect(z.x + 0.25 * z.width).toBeCloseTo(r.x + r.width / 2, 9);
		expect(z.width).toBe(3200);
	});

	it('lerpPreset goes from a to b (geometric zoom)', () => {
		const a = { id: 'a', x: 0.5, y: 0.5, zoom: 1 };
		const b = { id: 'b', x: 0.3, y: 0.4, zoom: 4 };
		expect(lerpPreset(a, b, 0)).toMatchObject({ x: 0.5, y: 0.5, zoom: 1 });
		expect(lerpPreset(a, b, 1)).toMatchObject({ x: 0.3, y: 0.4, zoom: 4 });
		expect(lerpPreset(a, b, 0.5).zoom).toBeCloseTo(2, 9);
	});

	it('drift stays within its amplitude and moves anchors only a little', () => {
		const rect = frameRect(1440, 900, tuTieuCung.frame);
		const dais = anchorToWorld(pose, tuTieuCung.anchors.dais!);
		const at0 = worldToBox(pose, rect, dais)!;
		let max = 0;
		for (let t = 0; t < 120_000; t += 250) {
			const d = drift(t, HALL.drift.full);
			expect(Math.abs(d.x)).toBeLessThanOrEqual(HALL.drift.full + 1e-9);
			const p = worldToBox(withOffset(pose, d), rect, dais)!;
			max = Math.max(max, Math.hypot(p.x - at0.x, p.y - at0.y));
		}
		expect(max).toBeGreaterThan(1); // it does move
		expect(max).toBeLessThan(25); // but stays a drift (px at 1440)
		expect(drift(0, 0)).toEqual({ x: 0, y: 0, z: 0 });
	});
});
