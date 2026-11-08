import { describe, expect, it } from 'vitest';
import { chatBox, clampToRect, intersects, regionVars, relaxLabels, stageRect, type RegionSizes } from './regions';

const phone: RegionSizes = { vw: 360, vh: 740, hud: 56, bar: 64, rail: 0, chatW: 360, chatH: 48, safeB: 0, wide: false };
const desk: RegionSizes = { vw: 1440, vh: 900, hud: 56, bar: 0, rail: 84, chatW: 352, chatH: 136, safeB: 0, wide: true };

describe('regionVars', () => {
	it('phone: bar and chat row below the stage, no rail', () => {
		const v = regionVars({ ...phone, safeB: 20 });
		expect(v['--hud-h']).toBe('56px');
		expect(v['--bar-h']).toBe('64px');
		expect(v['--rail-w']).toBe('0px');
		expect(v['--chat-reserve']).toBe('48px');
		expect(v['--stage-inset-bottom']).toBe('132px'); // bar + safe area + chat row
	});
	it('wide: rail on the right, no bottom bar, chat inside the stage', () => {
		const v = regionVars({ ...desk, bar: 70 });
		expect(v['--bar-h']).toBe('0px');
		expect(v['--rail-w']).toBe('84px');
		expect(v['--stage-inset-right']).toBe('84px');
		expect(v['--stage-inset-bottom']).toBe('0px');
		expect(v['--chat-w']).toBe('352px');
	});
	it('never publishes negative or fractional sizes', () => {
		expect(regionVars({ ...phone, hud: 55.6, bar: -3 })['--hud-h']).toBe('56px');
		expect(regionVars({ ...phone, bar: -3 })['--bar-h']).toBe('0px');
	});
});

describe('stageRect / chatBox', () => {
	it('phone stage stops above the chat row and the bar', () => {
		expect(stageRect(phone)).toEqual({ top: 56, left: 0, right: 360, bottom: 740 - 64 - 48 });
		expect(chatBox(phone)).toBeNull();
	});
	it('wide chat box sits in the stage bottom-left', () => {
		const s = stageRect(desk);
		expect(s).toEqual({ top: 56, left: 0, right: 1356, bottom: 900 });
		expect(chatBox(desk)).toEqual({ left: 0, right: 352, bottom: 900, top: 764 });
	});
});

describe('clampToRect', () => {
	const r = { top: 56, left: 0, right: 360, bottom: 628 };
	it('keeps an inside box where it is', () => expect(clampToRect(180, 300, 100, 40, r)).toEqual({ x: 180, y: 300 }));
	it('pulls a box back inside every edge (with padding)', () => {
		expect(clampToRect(10, 40, 100, 40, r, 4)).toEqual({ x: 54, y: 80 });
		expect(clampToRect(400, 700, 100, 40, r, 4)).toEqual({ x: 306, y: 604 });
	});
	it('centres a box larger than the rect', () => expect(clampToRect(0, 0, 500, 40, r).x).toBe(180));
});

describe('relaxLabels', () => {
	const bounds = { top: 0, left: 0, right: 800, bottom: 600 };
	const noOverlap = (bs: { x: number; y: number; w: number; h: number }[]) => {
		const rr = bs.map((b) => ({ left: b.x - b.w / 2, right: b.x + b.w / 2, top: b.y - b.h / 2, bottom: b.y + b.h / 2 }));
		for (let i = 0; i < rr.length; i++) for (let j = i + 1; j < rr.length; j++) if (intersects(rr[i]!, rr[j]!)) return false;
		return true;
	};
	it('leaves separate labels untouched', () => {
		const bs = [
			{ x: 100, y: 100, w: 120, h: 40 },
			{ x: 400, y: 300, w: 120, h: 40 }
		];
		expect(relaxLabels(bs, bounds)).toEqual(bs);
	});
	it('separates overlapping labels vertically and keeps them in bounds', () => {
		const bs = [
			{ x: 300, y: 300, w: 160, h: 44 },
			{ x: 330, y: 310, w: 160, h: 44 },
			{ x: 360, y: 320, w: 160, h: 44 }
		];
		const out = relaxLabels(bs, bounds);
		expect(noOverlap(out)).toBe(true);
		for (const b of out) expect(b.y - b.h / 2).toBeGreaterThanOrEqual(0);
	});
	it('moves labels out of an avoided box (the chat dock)', () => {
		const dock = { left: 0, right: 350, top: 470, bottom: 600 };
		const [b] = relaxLabels([{ x: 100, y: 560, w: 120, h: 40 }], bounds, { avoid: [dock] });
		expect(intersects({ left: b!.x - 60, right: b!.x + 60, top: b!.y - 20, bottom: b!.y + 20 }, dock)).toBe(false);
		expect(b!.y + 20).toBeLessThanOrEqual(600);
	});
	it('a small landscape map: clears the status card and separates the cluster under it', () => {
		const card = { left: 12, top: 12, right: 332, bottom: 100 };
		const bs = [
			{ x: 356, y: 128, w: 154, h: 44 },
			{ x: 209, y: 128, w: 125, h: 44 },
			{ x: 492, y: 174, w: 110, h: 44 },
			{ x: 304, y: 100, w: 157, h: 44 },
			{ x: 550, y: 222, w: 111, h: 44 },
			{ x: 419, y: 271, w: 153, h: 44 }
		];
		const out = relaxLabels(bs, { left: 0, top: 0, right: 712, bottom: 295 }, { avoid: [card] });
		expect(noOverlap([...out, { x: 172, y: 56, w: 320, h: 88 }])).toBe(true);
		for (const b of out) expect(b.y + b.h / 2).toBeLessThanOrEqual(295);
	});
});
