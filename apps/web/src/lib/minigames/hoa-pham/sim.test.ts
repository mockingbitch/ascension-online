import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import type { HoaPhamParams } from '$lib/game/types';
import { HoaPhamGame, maxWindowSum, Recorder, replay, stepLen, SUB, toward, XorShift32, type Stroke } from './sim';

interface GoldenCase {
	name: string;
	seed: number;
	endTick: number;
	strokes: Stroke[];
	reason: string;
	won: boolean;
	cuts: number;
	score: number;
	spawned: number;
	lanternsCut: number;
	escaped: number;
	bestCombo: number;
	stateHash: number;
}

// Shared with the Go tests (internal/trial/hoapham): both simulators must reproduce every case bit for bit.
// Resolved from this file (not the cwd) and REQUIRED: a missing vector fails the suite instead of skipping it.
const goldenPath = fileURLToPath(new URL('../../../../../../packages/contracts/golden/hoa-pham-v1.json', import.meta.url));
const golden = JSON.parse(readFileSync(goldenPath, 'utf8')) as { params: HoaPhamParams; cases: GoldenCase[] };
const p = golden.params;

describe('hóa phàm simulator (rules v1)', () => {
	it('xorshift32 matches the reference sequence (same as Go)', () => {
		const x = new XorShift32(1);
		expect([x.next(), x.next(), x.next(), x.next(), x.next()]).toEqual([270369, 67634689, 2647435461, 307599695, 2398689233]);
	});

	it('reproduces the Go golden vectors', () => {
		expect(p.rulesVersion).toBe(1);
		expect(golden.cases.length).toBeGreaterThanOrEqual(5);
		for (const c of golden.cases) {
			const r = replay(p, c.seed, c.strokes, c.endTick);
			const pick = (o: GoldenCase | typeof r) => ({ won: o.won, reason: o.reason, cuts: o.cuts, score: o.score, spawned: o.spawned,
				lanternsCut: o.lanternsCut, escaped: o.escaped, bestCombo: o.bestCombo, stateHash: o.stateHash });
			expect({ name: c.name, ...pick(r) }).toEqual({ name: c.name, ...pick(c) });
		}
	});

	it('is deterministic and tamper-evident', () => {
		const c = golden.cases.find((x) => x.won)!;
		const a = replay(p, c.seed, c.strokes, c.endTick);
		expect(replay(p, c.seed, c.strokes.map((s) => [...s]), c.endTick)).toEqual(a);
		expect(replay(p, c.seed ^ 1, c.strokes, c.endTick).stateHash).not.toBe(a.stateHash);
		expect(replay(p, c.seed, c.strokes.slice(c.strokes.length >> 1), c.endTick).stateHash).not.toBe(a.stateHash);
		expect(() => replay(p, c.seed, [[5, 1, 1], [4, 1, 1]], 100)).toThrow(/invalid/);
		expect(() => replay(p, c.seed, [[5, p.width, 1]], 100)).toThrow(/invalid/);
		expect(() => replay(p, c.seed, [[10, 5, 5, 1, 1], [9223372036854775807, 5, 5, 0, 0], [0, 5, 5, 1, 1]], 100)).toThrow(/invalid/);
	});

	it('octile length and toward never overshoot', () => {
		expect(stepLen(4, 4)).toBe(6);
		expect(stepLen(-10, 3)).toBe(11);
		for (let dx = -90; dx <= 90; dx += 7) {
			for (let dy = -90; dy <= 90; dy += 11) {
				const [x, y] = toward(50, 50, 50 + dx, 50 + dy, 48);
				expect(stepLen(x - 50, y - 50)).toBeLessThanOrEqual(48);
			}
		}
	});

	it('rejects teleports and superhuman speed; the recorder never produces either', () => {
		expect(() => replay(p, 1, [[10, 5, 80, 100, 0]], 100)).toThrow(/implausible/);
		const fast: Stroke = [10, 10, 80];
		for (let i = 0; i < 24; i++) fast.push(i % 2 ? -p.maxStep : p.maxStep, 0);
		expect(() => replay(p, 1, [fast], 100)).toThrow(/implausible/);
		// A frantic "player": flings the pointer corner to corner every tick and re-presses as fast as possible.
		const rec = new Recorder(p);
		for (let t = 0; t < 600; t++) {
			const down = t % 9 !== 0;
			rec.sample(t, down ? { x: t % 2 ? 0 : p.width + 30, y: t % 4 < 2 ? -10 : p.height } : null);
		}
		rec.sample(600, null);
		expect(() => replay(p, 7, rec.strokes, 601)).not.toThrow();
		const travel: [number, number][] = [];
		for (const s of rec.strokes) for (let j = 3; j < s.length; j += 2) travel.push([s[0]! + (j - 1) / 2, stepLen(s[j]!, s[j + 1]!)]);
		expect(maxWindowSum(travel, p.inputWindowTicks)).toBeLessThanOrEqual(p.maxWindowTravel);
	});

	it('a fast stroke cuts, a slow drag does not, and chân tâm lanterns cost a life', () => {
		const g = new HoaPhamGame(p, 5);
		g.spawnCounter = -(2 ** 40);
		g.objs = [{ id: 1, type: 0, x: 60 * SUB, y: 80 * SUB, vx: 0, vy: 0, hp: 1, lastStroke: 0 }];
		g.step(true, 58, 80);
		g.step(true, 60, 80);
		g.step(true, 62, 80);
		expect(g.cuts).toBe(0);
		g.step(true, 90, 80);
		expect(g.cuts).toBe(1);
		g.step(false);
		const lanternType = p.objects.findIndex((o) => o.role === 'lantern');
		g.objs = [{ id: 2, type: lanternType, x: 60 * SUB, y: 40 * SUB, vx: 0, vy: 0, hp: 1, lastStroke: 0 }];
		g.step(true, 40, 40);
		g.step(true, 80, 40);
		expect(g.lanternsLeft).toBe(p.lanterns - 1);
	});
});
