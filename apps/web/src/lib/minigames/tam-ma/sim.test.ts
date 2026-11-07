import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import type { TamMaParams } from '$lib/game/types';
import { Act, maxWindowInputs, replay, TamMaGame, windowRoom, XorShift32, type Input } from './sim';

interface GoldenCase {
	name: string;
	seed: number;
	endTick: number;
	inputs: Input[];
	reason: string;
	won: boolean;
	lines: number;
	score: number;
	pieces: number;
	stateHash: number;
	minLockGapTicks: number;
}

// Shared with the Go tests (internal/trial/tamma): both simulators must reproduce every case bit for bit.
// Resolved from this file (not the cwd) and REQUIRED: a missing vector fails the suite instead of skipping it.
const goldenPath = fileURLToPath(new URL('../../../../../../packages/contracts/golden/tam-ma-v2.json', import.meta.url));
const golden = JSON.parse(readFileSync(goldenPath, 'utf8')) as { params: TamMaParams; cases: GoldenCase[] };
const p = golden.params;

describe('tâm ma simulator (rules v2)', () => {
	it('xorshift32 matches the reference sequence (same as Go)', () => {
		const x = new XorShift32(1);
		expect([x.next(), x.next(), x.next(), x.next(), x.next()]).toEqual([270369, 67634689, 2647435461, 307599695, 2398689233]);
		expect(new XorShift32(0).s).not.toBe(0);
	});

	it('reproduces the Go golden vectors', () => {
		expect(p.rulesVersion).toBe(2);
		expect(golden.cases.length).toBeGreaterThanOrEqual(5);
		for (const c of golden.cases) {
			const r = replay(p, c.seed, c.inputs, c.endTick);
			expect({ name: c.name, won: r.won, reason: r.reason, lines: r.lines, score: r.score, pieces: r.pieces, stateHash: r.stateHash, gap: r.minLockGapTicks }).toEqual({
				name: c.name,
				won: c.won,
				reason: c.reason,
				lines: c.lines,
				score: c.score,
				pieces: c.pieces,
				stateHash: c.stateHash,
				gap: c.minLockGapTicks
			});
		}
	});

	it('is deterministic and tamper-evident', () => {
		const c = golden.cases.find((x) => x.won)!;
		const a = replay(p, c.seed, c.inputs, c.endTick);
		expect(replay(p, c.seed, [...c.inputs], c.endTick)).toEqual(a);
		expect(replay(p, c.seed ^ 1, c.inputs, c.endTick).stateHash).not.toBe(a.stateHash);
		expect(replay(p, c.seed, c.inputs.slice(c.inputs.length >> 1), c.endTick).stateHash).not.toBe(a.stateHash);
		expect(() => replay(p, c.seed, [[5, Act.Left], [4, Act.Left]], 100)).toThrow();
		expect(() => replay(p, c.seed, [[1, 9]], 100)).toThrow();
	});

	it('rejects a superhuman input burst and the client throttle never produces one', () => {
		const burst: Input[] = Array.from({ length: p.maxInputsPerWindow + 1 }, (_, i) => [i, Act.Left]);
		expect(maxWindowInputs(burst, p.inputWindowTicks)).toBe(p.maxInputsPerWindow + 1);
		expect(() => replay(p, 1, burst, 100)).toThrow(/implausible/);
		// Throttled logging (as TamMaBoard does): spam 4 presses every tick for 200 ticks.
		const log: Input[] = [];
		for (let t = 0; t < 200; t++) {
			const room = windowRoom(p, log, t);
			for (let k = 0; k < Math.min(4, room); k++) log.push([t, Act.Left]);
		}
		expect(maxWindowInputs(log, p.inputWindowTicks)).toBeLessThanOrEqual(p.maxInputsPerWindow);
		expect(() => replay(p, 1, log, 200)).not.toThrow();
	});

	it('ignores a drop inside the lock gap and passes only after minPassTicks', () => {
		const g = new TamMaGame(p, 5);
		g.step([Act.HardDrop]);
		expect(g.locked).toBe(1);
		g.step([Act.HardDrop]);
		expect(g.locked).toBe(1);
		const h = new TamMaGame(p, 9);
		h.lines = p.passLines;
		while (h.tick < p.minPassTicks - 1) {
			h.calmLeft = 1;
			h.step([]);
		}
		expect(h.over).toBe(false);
		h.calmLeft = 1;
		h.step([]);
		expect(h.won && h.reason === 'cleared' && h.tick === p.minPassTicks).toBe(true);
	});

	it('tĩnh tâm freezes gravity only with a full charge', () => {
		const g = new TamMaGame(p, 3);
		g.step([Act.Calm]);
		expect(g.calmLeft).toBe(0);
		g.charge = p.calm.chargeMax;
		g.step([Act.Calm]);
		const y = g.py;
		for (let i = 0; i < 50; i++) g.step([]);
		expect(g.py).toBe(y);
		expect(g.charge).toBe(0);
	});
});
