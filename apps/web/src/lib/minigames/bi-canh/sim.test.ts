import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import type { BiCanhParams } from '$lib/game/types';
import { BiCanhGame, dirVec, Recorder, replay, stepLen, XorShift32, type Run } from './sim';

interface GoldenCase {
	name: string;
	seed: number;
	endTick: number;
	runs: Run[];
	reason: string;
	won: boolean;
	kills: number;
	score: number;
	qi: number;
	level: number;
	hpLeft: number;
	spawned: number;
	stateHash: number;
}

// Shared with the Go tests (internal/secretrealm/bicanh): both simulators must reproduce every case bit for bit.
// Resolved from this file (not the cwd) and REQUIRED: a missing vector fails the suite instead of skipping it.
const goldenPath = fileURLToPath(new URL('../../../../../../packages/contracts/golden/bi-canh-v1.json', import.meta.url));
const golden = JSON.parse(readFileSync(goldenPath, 'utf8')) as { params: BiCanhParams; cases: GoldenCase[] };
const p = golden.params;

describe('bí cảnh simulator (rules v1)', () => {
	it('xorshift32 matches the reference sequence (same as Go)', () => {
		const x = new XorShift32(1);
		expect([x.next(), x.next(), x.next(), x.next(), x.next()]).toEqual([270369, 67634689, 2647435461, 307599695, 2398689233]);
	});

	it('reproduces the Go golden vectors', () => {
		expect(p.rulesVersion).toBe(1);
		expect(golden.cases.length).toBeGreaterThanOrEqual(5);
		for (const c of golden.cases) {
			const r = replay(p, c.seed, c.runs, c.endTick);
			const pick = (o: GoldenCase | typeof r) => ({ won: o.won, reason: o.reason, kills: o.kills, score: o.score, qi: o.qi, level: o.level,
				hpLeft: o.hpLeft, spawned: o.spawned, stateHash: o.stateHash });
			expect({ name: c.name, ...pick(r) }).toEqual({ name: c.name, ...pick(c) });
		}
	});

	it('is deterministic and tamper-evident', () => {
		const c = golden.cases.find((x) => x.won)!;
		const a = replay(p, c.seed, c.runs, c.endTick);
		expect(replay(p, c.seed, c.runs.map((r) => [...r] as Run), c.endTick)).toEqual(a);
		expect(replay(p, c.seed ^ 1, c.runs, c.endTick).stateHash).not.toBe(a.stateHash);
		expect(() => replay(p, 1, [[1, 5], [1, 5]], 10)).toThrow(/invalid/);
		expect(() => replay(p, 1, [[9, 10]], 10)).toThrow(/invalid/);
		expect(() => replay(p, 1, [[1, 5]], 10)).toThrow(/invalid/);
		expect(() => replay(p, 1, [[1, 0], [2, 10]], 10)).toThrow(/invalid/);
	});

	it('rejects inhuman jitter; the recorder never produces it', () => {
		const jitter: Run[] = [];
		for (let i = 0; i < 40; i++) jitter.push([1 + (i % 2), 1]);
		expect(() => replay(p, 1, jitter, 40)).toThrow(/implausible/);
		const rec = new Recorder(p);
		for (let t = 0; t < 600; t++) rec.sample(t % 9);
		expect(() => replay(p, 3, rec.runs, 600)).not.toThrow();
	});

	it('moves in eight directions at the rule speed and stays inside the realm', () => {
		expect(dirVec(3, 80)).toEqual([80, 0]);
		expect(dirVec(4, 80)).toEqual([56, 56]);
		expect(stepLen(-10, 3)).toBe(11);
		const g = new BiCanhGame(p, 9);
		for (let i = 0; i < 400 && !g.over; i++) g.step(8);
		expect(g.px).toBeGreaterThanOrEqual(p.player.radius * 16);
		expect(g.py).toBeGreaterThanOrEqual(p.player.radius * 16);
	});
});
