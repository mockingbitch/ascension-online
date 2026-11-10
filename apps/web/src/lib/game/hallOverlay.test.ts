import { describe, expect, it } from 'vitest';
import { intersects, type Rect } from '$lib/engine/layout/regions';
import { IMPORTANT_CAP, MAX_BUBBLES, PHONE_BUBBLES, SPOKE_MS, coveredLabels, focusAround, isImportant, labelRects, planBubbles, planNameLabels, shortName, tailPoints, type LabelInput } from './hallOverlay';

const bounds: Rect = { left: 0, top: 0, right: 1000, bottom: 600 };
const seat = (id: string, x: number, y: number, extra: Partial<LabelInput> = {}): LabelInput => ({
	id, x, y, size: 40, name: `Đạo hữu ${id}`, mine: false, partner: false, sameFaction: false, fxTier: 1, spokeAt: 0, ...extra
});
const rect = (p: { x: number; y: number; w: number; h: number }): Rect => ({ left: p.x - p.w / 2, right: p.x + p.w / 2, top: p.y - p.h / 2, bottom: p.y + p.h / 2 });

describe('hall name labels', () => {
	it('shortens long names (the card has the full one)', () => {
		expect(shortName('Lâm Thanh Vân')).toBe('Lâm Thanh Vân');
		expect(shortName('Mộ Dung Tuyết Nhược Băng Tâm')).toMatch(/…$/);
		expect([...shortName('Mộ Dung Tuyết Nhược Băng Tâm')].length).toBeLessThanOrEqual(14);
	});

	it('desktop: every listener gets a label; shown labels never overlap each other', () => {
		const items = Array.from({ length: 60 }, (_, i) => seat(String(i), 300 + (i % 12) * 34, 200 + Math.floor(i / 12) * 50));
		const out = planNameLabels(items, { mode: 'all', now: 0, fontPx: 11, obstacles: [], bounds });
		expect(out).toHaveLength(60);
		const shown = out.filter((l) => l.state !== 'hidden'); // shown + faded: everything drawn
		expect(shown.length).toBeGreaterThan(10);
		for (let i = 0; i < shown.length; i++) for (let j = i + 1; j < shown.length; j++) expect(intersects(rect(shown[i]!), rect(shown[j]!))).toBe(false);
	});

	it('my own name is the "Bạn" tag, never a label; nothing covers that tag', () => {
		const me = seat('me', 500, 300, { mine: true });
		const tag: Rect = { left: 440, right: 560, top: 300, bottom: 322 };
		const below = seat('b', 500, 352); // its label would sit right on my tag
		const out = planNameLabels([me, below], { mode: 'all', now: 0, fontPx: 11, obstacles: [tag], bounds });
		expect(out.map((l) => l.id)).toEqual(['b']);
		expect(out[0]!.state === 'hidden' || !intersects(rect(out[0]!), tag)).toBe(true);
	});

	it('a label over a nearer sitter fades', () => {
		const far = seat('far', 500, 300);
		const near = seat('near', 500, 330, { size: 100 }); // its figure covers the far label
		const out = planNameLabels([far, near], { mode: 'all', now: 0, fontPx: 11, obstacles: [], bounds });
		expect(out.find((l) => l.id === 'far')!.state).toBe('faded');
	});

	it('"Quan trọng": only me / đạo lữ / faction / fx_tier ≥ 4 / recent speakers / the seats in focus', () => {
		const now = 100_000;
		expect(isImportant(seat('a', 0, 0), now)).toBe(false);
		expect(isImportant(seat('a', 0, 0, { partner: true }), now)).toBe(true);
		expect(isImportant(seat('a', 0, 0, { sameFaction: true }), now)).toBe(true);
		expect(isImportant(seat('a', 0, 0, { fxTier: 4 }), now)).toBe(true);
		expect(isImportant(seat('a', 0, 0, { fxTier: 3 }), now)).toBe(false);
		expect(isImportant(seat('a', 0, 0, { spokeAt: now - SPOKE_MS + 1 }), now)).toBe(true);
		expect(isImportant(seat('a', 0, 0, { spokeAt: now - SPOKE_MS - 1 }), now)).toBe(false);
		expect(isImportant(seat('a', 0, 0), now, new Set(['a']))).toBe(true);
		const out = planNameLabels([seat('a', 100, 300), seat('b', 300, 300, { fxTier: 5 })], { mode: 'important', now, fontPx: 11, obstacles: [], bounds });
		expect(out.map((l) => l.id)).toEqual(['b']);
	});

	it('"Quan trọng" draws at most IMPORTANT_CAP names, the focus first; "Ẩn" draws none', () => {
		const items = Array.from({ length: 40 }, (_, i) => seat(String(i), 100 + (i % 10) * 80, 150 + Math.floor(i / 10) * 90, { sameFaction: true }));
		const focus = focusAround('25', items, 4);
		expect(focus.size).toBe(5);
		expect(focus.has('24') && focus.has('26') && focus.has('15') && focus.has('35')).toBe(true);
		const out = planNameLabels(items, { mode: 'important', now: 0, focus, fontPx: 11, obstacles: [], bounds });
		expect(out.length).toBeLessThanOrEqual(IMPORTANT_CAP);
		for (const id of focus) expect(out.find((l) => l.id === id)?.state).not.toBe('hidden');
		expect(planNameLabels(items, { mode: 'off', now: 0, fontPx: 11, obstacles: [], bounds })).toEqual([]);
		expect(planNameLabels(items, { mode: 'all', now: 0, fontPx: 11, obstacles: [], bounds })).toHaveLength(40);
	});
});

describe('hall chat bubbles', () => {
	const bubble = (id: number, ax: number, ay: number, at: number) => ({ id, ax, ay, w: 180, h: 40, at });

	it('caps visible bubbles to the newest five and never overlaps them', () => {
		const items = Array.from({ length: 8 }, (_, i) => bubble(i + 1, 480 + (i % 3) * 20, 320, 1000 + i));
		const out = planBubbles(items, bounds, []);
		const vis = out.filter((b) => b.visible);
		expect(vis).toHaveLength(MAX_BUBBLES);
		expect(vis.map((b) => b.id).sort()).toEqual([4, 5, 6, 7, 8]);
		const r = vis.map((b) => rect({ x: b.x, y: b.y, w: 180, h: 40 }));
		for (let i = 0; i < r.length; i++) {
			for (let j = i + 1; j < r.length; j++) expect(intersects(r[i]!, r[j]!)).toBe(false);
			expect(r[i]!.top).toBeGreaterThanOrEqual(0);
			expect(r[i]!.bottom).toBeLessThanOrEqual(600);
		}
	});

	it('keeps clear of the header overlay / subtitles / my tag and stays anchored (tail to the speaker)', () => {
		const header: Rect = { left: 0, right: 1000, top: 0, bottom: 120 };
		const subs: Rect = { left: 0, right: 1000, top: 520, bottom: 600 };
		const out = planBubbles([bubble(1, 500, 150, 1), bubble(2, 520, 160, 2)], bounds, [header, subs]);
		for (const b of out) {
			const r = rect({ x: b.x, y: b.y, w: 180, h: 40 });
			expect(intersects(r, header) || intersects(r, subs)).toBe(false);
			const pts = tailPoints({ ...b, w: 180, h: 40 }).split(' ');
			expect(pts[2]).toBe(`${b.ax},${b.ay}`);
		}
	});

	it('a cramped stage shows fewer bubbles, never overlapping or clipped ones', () => {
		const tiny: Rect = { left: 0, top: 0, right: 400, bottom: 120 };
		const out = planBubbles(Array.from({ length: 6 }, (_, i) => bubble(i + 1, 200, 80, i)), tiny, [{ left: 0, right: 400, top: 0, bottom: 30 }]);
		const vis = out.filter((b) => b.visible);
		expect(vis.length).toBeGreaterThanOrEqual(1);
		expect(vis.length).toBeLessThan(6);
		expect(vis[0]!.id).toBe(6); // the newest wins the room
		const r = vis.map((b) => rect({ x: b.x, y: b.y, w: 180, h: 40 }));
		for (let i = 0; i < r.length; i++) {
			expect(r[i]!.top).toBeGreaterThanOrEqual(30);
			for (let j = i + 1; j < r.length; j++) expect(intersects(r[i]!, r[j]!)).toBe(false);
		}
	});

	it('bubbles go to free room clear of the name labels when there is some; labels a bubble still covers are hidden', () => {
		const items = Array.from({ length: 24 }, (_, i) => seat(String(i), 380 + (i % 8) * 34, 300 + Math.floor(i / 8) * 46));
		const labels = planNameLabels(items, { mode: 'all', now: 0, fontPx: 11, obstacles: [], bounds });
		const out = planBubbles([bubble(1, 450, 270, 2), bubble(2, 520, 316, 1)], bounds, [], { soft: [labelRects(labels)], spots: [{ x: 500, y: 120 }] });
		const vis = out.filter((b) => b.visible).map((b) => ({ ...b, w: 180, h: 40 }));
		expect(vis).toHaveLength(2);
		expect(coveredLabels(labels, vis).size).toBe(0);
		// no free room at all (bounds = the label field): the bubble still shows (hard rules only) and reports covered labels
		const tight: Rect = { left: 360, top: 225, right: 660, bottom: 365 };
		const forced = planBubbles([bubble(3, 450, 300, 3)], tight, [], { soft: [labelRects(labels)] }).filter((b) => b.visible);
		expect(forced).toHaveLength(1);
		expect(coveredLabels(labels, forced.map((b) => ({ ...b, w: 180, h: 40 }))).size).toBeGreaterThan(0);
	});

	it('phones show at most two bubbles (the newest)', () => {
		const out = planBubbles(Array.from({ length: 5 }, (_, i) => bubble(i + 1, 100 + i * 200, 400, i)), bounds, [], { max: PHONE_BUBBLES });
		expect(out.filter((b) => b.visible).map((b) => b.id).sort()).toEqual([4, 5]);
	});

	it('waits for a measure before showing a bubble', () => {
		expect(planBubbles([{ id: 1, ax: 10, ay: 10, w: 0, h: 0, at: 1 }], bounds, [])[0]!.visible).toBe(false);
	});
});
