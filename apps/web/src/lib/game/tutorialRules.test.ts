import fs from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';
import { help } from './help';
import { helpLines, layoutGuide, onboardingStep, stepFor } from './tutorialRules';
import type { TutorialScript } from './types';

const script: TutorialScript = {
	guide: 'van_khe',
	name: 'Vân Khê',
	steps: [
		{ id: 'ob_the_luc', flow: 'onboarding', trigger: 'onboarding.the_luc', title: 'T', lines: [{ expr: 'smile', text: 'a' }] },
		{ id: 'first_settle', flow: 'play', trigger: 'fx.settled', title: 'T', lines: [{ expr: 'smile', text: 'b' }] },
		{ id: 'first_shop', flow: 'play', trigger: 'window.inventory.shop', title: 'T', lines: [{ expr: 'smile', text: 'c' }] }
	]
};
const fresh = { done: [], skipped: [], muted: false };

describe('tutorial steps fire once, from known triggers', () => {
	it('shows a play step for its trigger until it is done or skipped (any device)', () => {
		expect(stepFor(script, 'fx.settled', fresh)?.id).toBe('first_settle');
		expect(stepFor(script, 'fx.settled', { ...fresh, done: ['first_settle'] })).toBeUndefined();
		expect(stepFor(script, 'window.inventory.shop', { ...fresh, skipped: ['first_shop'] })).toBeUndefined();
		expect(stepFor(script, 'fx.settled', fresh, new Set(['first_settle']))).toBeUndefined();
	});
	it('stays silent when muted, unknown, onboarding-only, or before state/script load', () => {
		expect(stepFor(script, 'fx.settled', { ...fresh, muted: true })).toBeUndefined();
		expect(stepFor(script, 'fx.reward', fresh)).toBeUndefined();
		expect(stepFor(script, 'onboarding.the_luc', fresh)).toBeUndefined();
		expect(stepFor(null, 'fx.settled', fresh)).toBeUndefined();
		expect(stepFor(script, 'fx.settled', null)).toBeUndefined();
	});
	it('maps a /create wizard step to its onboarding step', () => {
		expect(onboardingStep(script, 'the-luc')?.id).toBe('ob_the_luc');
		expect(onboardingStep(script, 'dao-hieu')).toBeUndefined();
	});
	it('reads the help points of a topic as the guide\'s lines', () => {
		const lines = helpLines(help.breakthrough.points);
		expect(lines.map((l) => l.text)).toEqual(help.breakthrough.points);
		expect(lines[0]!.expr).toBe('smile');
	});
});

describe('guide overlay layout (Vân Khê big, on top, never over her target)', () => {
	type R = { x: number; y: number; w: number; h: number };
	const hit = (a: R, b: R) => a.x < b.x + b.w && a.x + a.w > b.x && a.y < b.y + b.h && a.y + a.h > b.y;
	const onScreen = (r: R, v: { w: number; h: number }) => r.x >= 0 && r.y >= 0 && r.x + r.w <= v.w && r.y + r.h <= v.h;

	it('desktop: she stands at the left edge about half the width / ~90% height, the box over her lower half', () => {
		for (const v of [
			{ w: 1440, h: 900 },
			{ w: 1024, h: 768 }
		]) {
			const l = layoutGuide(v, 220);
			expect(l.mode).toBe('side');
			expect(l.at).toBe('left');
			expect(l.portrait.x).toBe(0);
			expect(l.portrait.y + l.portrait.h).toBe(v.h);
			expect(l.portrait.h).toBeGreaterThanOrEqual(v.h * 0.85);
			expect(l.portrait.w).toBeGreaterThanOrEqual(v.w * 0.4);
			expect(l.portrait.w).toBeLessThanOrEqual(v.w * 0.5);
			expect(l.box.x).toBeLessThan(l.portrait.x + l.portrait.w); // overlaps her
			expect(l.box.y + l.box.h).toBe(v.h - 12);
			expect(l.box.y).toBeGreaterThan(v.h / 2); // over her lower half
			expect(onScreen(l.box, v)).toBe(true);
			expect(l.hole).toBeUndefined();
		}
	});
	it('portrait phone: she fills the upper half, the box below', () => {
		const v = { w: 360, h: 740 };
		const l = layoutGuide(v, 240);
		expect(l.mode).toBe('stack');
		expect(l.at).toBe('top');
		expect(l.portrait.h).toBeGreaterThanOrEqual(v.h * 0.5);
		expect(l.portrait.y + l.portrait.h / 2).toBeLessThan(v.h / 2);
		expect(l.box).toEqual({ x: 12, y: 740 - 240 - 12, w: 336, h: 240 });
	});
	it('landscape phone: side by side, scaled to fit', () => {
		const v = { w: 812, h: 375 };
		const l = layoutGuide(v, 200);
		expect(l.mode).toBe('side');
		expect(l.portrait.h).toBeGreaterThanOrEqual(v.h * 0.9);
		expect(l.portrait.h).toBeLessThanOrEqual(v.h);
		expect(l.box.x).toBeGreaterThanOrEqual(l.portrait.x + l.portrait.w * 0.8); // beside her
		expect(onScreen(l.box, v)).toBe(true);
	});
	it('never lets a box taller than the screen run off it', () => {
		const l = layoutGuide({ w: 812, h: 375 }, 900);
		expect(l.box.h).toBe(375 - 24);
		expect(l.box.y).toBe(12);
	});
	it('stands on the side away from a spotlight target and keeps box and figure off the hole', () => {
		const v = { w: 1024, h: 768 };
		const left = layoutGuide(v, 220, { x: 80, y: 600, w: 240, h: 48 });
		expect(left.at).toBe('right');
		const right = layoutGuide(v, 220, { x: 700, y: 300, w: 240, h: 48 });
		expect(right.at).toBe('left');
		for (const l of [left, right]) {
			expect(l.covers).toEqual({ box: false, portrait: false });
			expect(hit(l.box, l.hole!)).toBe(false);
		}
		expect(left.hole).toEqual({ x: 74, y: 594, w: 252, h: 60 });
	});
	it('a centred target: the box moves (beside the target / top) rather than covering it; she stays big', () => {
		const v = { w: 1024, h: 768 };
		const t = { x: 360, y: 640, w: 300, h: 56 }; // e.g. the Đột phá button at the bottom of a window
		const l = layoutGuide(v, 220, t);
		expect(l.covers.box).toBe(false);
		expect(hit(l.box, l.hole!)).toBe(false);
		expect(l.portrait.h).toBeGreaterThanOrEqual(v.h * 0.85);
	});
	it('portrait phone: a target in the upper half flips her to the bottom, the box clear of it', () => {
		const v = { w: 360, h: 740 };
		const l = layoutGuide(v, 220, { x: 20, y: 120, w: 320, h: 60 });
		expect(l.at).toBe('bottom');
		expect(l.covers).toEqual({ box: false, portrait: false });
		const low = layoutGuide(v, 220, { x: 20, y: 600, w: 320, h: 60 });
		expect(low.at).toBe('top');
		expect(low.covers.box).toBe(false);
		// the box goes just above a low target (over her lower half), not at the top over her face
		const odds = layoutGuide(v, 250, { x: 270, y: 566, w: 80, h: 64 });
		expect(odds.covers).toEqual({ box: false, portrait: false });
		expect(odds.box.y).toBeGreaterThan(odds.portrait.y + odds.portrait.h * 0.5);
	});
	it('reports when nothing avoids a huge target (the box loses least)', () => {
		const l = layoutGuide({ w: 360, h: 740 }, 220, { x: 0, y: 0, w: 360, h: 740 });
		expect(l.covers.box).toBe(true);
		expect(l.portrait.h).toBeGreaterThanOrEqual(740 * 0.5);
	});
});

describe('tutorial copy (content/story/tutorial) ↔ web', () => {
	const dir = path.resolve(__dirname, '../../../../../content/story/tutorial');
	const yaml = fs.readdirSync(dir).filter((f) => f.endsWith('.yaml')).map((f) => fs.readFileSync(path.join(dir, f), 'utf8')).join('\n');
	const src = (p: string) => fs.readFileSync(path.resolve(__dirname, '..', p), 'utf8');
	it('every spotlight target exists in the UI (data-tutorial)', () => {
		const ids = new Set([...yaml.matchAll(/spotlight:\s*([a-z][a-z0-9-]*)/g)].map((m) => m[1]!));
		expect(ids.size).toBeGreaterThan(0);
		const ui = [
			src('components/GameHud.svelte'),
			src('components/ActionBar.svelte'),
			src('components/BreakthroughPanel.svelte'),
			fs.readFileSync(path.resolve(__dirname, '../../routes/(game)/play/tu-luyen/+page.svelte'), 'utf8')
		].join('\n');
		for (const id of ids) expect(ui, id).toContain(`data-tutorial="${id}"`);
	});
	it('every line keeps the guide\'s voice: ≤ 200 characters, never "ngươi" (ta / đạo hữu)', () => {
		const texts = [...yaml.matchAll(/text:\s*"([^"]*)"/g)].map((m) => m[1]!);
		expect(texts.length).toBeGreaterThanOrEqual(30);
		for (const t of texts) {
			expect(t.length, t).toBeLessThanOrEqual(200);
			expect(t, t).not.toMatch(/ngươi/i);
		}
		expect(texts.some((t) => t.includes('đạo hữu'))).toBe(true);
	});
	it('every trigger the scripts use is wired in the web', () => {
		const triggers = [...yaml.matchAll(/trigger:\s*([a-z_.]+)/g)].map((m) => m[1]!).filter((t) => !t.startsWith('onboarding.'));
		const host = src('components/TutorialHost.svelte') + src('windows/BreakthroughWindow.svelte');
		for (const t of triggers) expect(host, t).toContain(`'${t}'`);
	});
});
