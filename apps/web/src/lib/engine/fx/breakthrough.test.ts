import { describe, expect, it } from 'vitest';
import { beatsAt, BT_MAX_MS, btOutcome, planBreakthrough, planGather, SHAKE_MS, type BtInput } from './breakthrough';
import type { FxEvent } from './events';
import { planFx } from './plan';

const minor: BtInput = { result: 'success', kind: 'auto', tribulation: false };
const major: BtInput = { result: 'success', kind: 'major', tribulation: false };
const trib: BtInput = { result: 'success', kind: 'major', tribulation: true };
const tribFail: BtInput = { result: 'failure', kind: 'major', tribulation: true };
const fail: BtInput = { result: 'failure', kind: 'auto', tribulation: false };
const beats = (i: BtInput, q: 'full' | 'lite' | 'off' = 'full', reduced = false) => planBreakthrough(i, q, reduced).steps.map((s) => s.beat);

describe('breakthrough planning: outcome branches come from the server answer only', () => {
	it('maps result/kind/tribulation to the five branches', () => {
		expect([minor, major, trib, tribFail, fail].map(btOutcome)).toEqual(['minor', 'major', 'tribulation', 'tribulation_fail', 'fail']);
	});
	it('minor: aura gathers 0.8 s → flash → new realm aura + stage text', () => {
		const p = planBreakthrough(minor, 'full');
		expect(p.steps[0]).toEqual({ beat: 'gather', at: 0, ms: 800 });
		expect(beats(minor)).toEqual(['gather', 'flash', 'aura', 'title', 'result']);
		expect(p.steps.find((s) => s.beat === 'flash')!.at).toBe(800);
	});
	it('major: formation spins 1.2 s → pillar → calligraphy title → clouds part', () => {
		const p = planBreakthrough(major, 'full');
		expect(p.steps[0]).toEqual({ beat: 'spin', at: 0, ms: 1200 });
		expect(beats(major)).toEqual(['spin', 'pillar', 'title', 'clouds', 'result']);
	});
	it('lôi kiếp: darkens, exactly 3 strikes each with flash + shake, then reveal or crack', () => {
		for (const i of [trib, tribFail]) {
			const b = beats(i);
			expect(b[0]).toBe('darken');
			expect(b.filter((x) => x === 'strike')).toHaveLength(3);
			expect(b.filter((x) => x === 'flash')).toHaveLength(3);
		}
		expect(beats(trib)).toContain('pillar');
		expect(beats(trib)).not.toContain('crack');
		expect(beats(tribFail)).toContain('crack');
	});
	it('failure: crack + a 200 ms shake, then the static result', () => {
		const p = planBreakthrough(fail, 'full');
		expect(p.steps.find((s) => s.beat === 'shake')).toMatchObject({ at: 0, ms: SHAKE_MS });
		expect(beats(fail)).toEqual(['crack', 'shake', 'result']);
		expect(p.particles).toBe(0);
	});
	it('every branch settles to the static result within 3 s', () => {
		for (const i of [minor, major, trib, tribFail, fail])
			for (const q of ['full', 'lite'] as const) {
				const p = planBreakthrough(i, q);
				expect(p.totalMs).toBeLessThanOrEqual(BT_MAX_MS);
				for (const s of p.steps) expect(s.at + s.ms).toBeLessThanOrEqual(BT_MAX_MS);
				expect(p.steps.at(-1)).toMatchObject({ beat: 'result', at: p.totalMs });
			}
	});
});

describe('breakthrough planning: skip and effects gating', () => {
	it('skip = jumping to the end: only the static result remains', () => {
		const p = planBreakthrough(trib, 'full');
		expect(beatsAt(p, 520).has('strike')).toBe(true);
		expect([...beatsAt(p, p.totalMs)]).toEqual(['result']);
		expect([...beatsAt(p, Infinity)]).toEqual(['result']);
	});
	it('lite: shorter, fewer particles, flash kept, no shake (the failure crack stays)', () => {
		const full = planBreakthrough(major, 'full');
		const lite = planBreakthrough(major, 'lite');
		expect(lite.totalMs).toBeLessThan(full.totalMs);
		expect(lite.particles).toBeLessThan(full.particles);
		expect(beats(tribFail, 'lite')).not.toContain('shake');
		expect(beats(tribFail, 'lite')).toContain('crack');
		expect(beats(trib, 'lite').filter((b) => b === 'strike')).toHaveLength(3);
	});
	it('off: an instant static card — no beats, no particles, no flash or shake', () => {
		for (const i of [minor, major, trib, tribFail, fail]) {
			const p = planBreakthrough(i, 'off');
			expect(p).toMatchObject({ steps: [{ beat: 'result', at: 0, ms: 0 }], totalMs: 0, particles: 0, flash: false, shake: false });
		}
	});
	it('reduced motion: no flash and no shake even at full', () => {
		const p = planBreakthrough(tribFail, 'full', true);
		expect(p.flash || p.shake).toBe(false);
		expect(p.steps.some((s) => s.beat === 'flash' || s.beat === 'shake')).toBe(false);
	});
	it('the wait loop depends on the preview only (major spins faster; off = still)', () => {
		expect(planGather({ kind: 'major', tribulation: false }, 'full').turnMs).toBeLessThan(planGather({ kind: 'auto', tribulation: false }, 'full').turnMs);
		expect(planGather({ kind: 'auto', tribulation: false }, 'lite').motesPerSec).toBeLessThan(planGather({ kind: 'auto', tribulation: false }, 'full').motesPerSec);
		expect(planGather({ kind: 'major', tribulation: true }, 'off')).toEqual({ motesPerSec: 0, turnMs: 0, dark: true, motion: false });
	});
});

describe('fxBus → breakthrough mode commands (wait → outcome)', () => {
	const answer: FxEvent = { type: 'breakthrough', result: 'failure', kind: 'major', tribulation: true, lossMilli: 1500, from: 'a', to: 'a', failStreak: 2 };
	it('start → gather; the outcome only on the server answer, carrying its values', () => {
		const [g] = planFx({ type: 'breakthroughStart', kind: 'major', tribulation: true }, 'full');
		expect(g).toMatchObject({ kind: 'mode', step: 'gather', tribulation: true });
		expect(g).not.toHaveProperty('plan');
		const [o] = planFx(answer, 'full');
		expect(o).toMatchObject({ kind: 'mode', step: 'outcome', plan: { outcome: 'tribulation_fail' }, result: { lossMilli: 1500, failStreak: 2 } });
	});
	it('the mode opens even with effects off (a static card), and reduced motion reaches the plan', () => {
		expect(planFx(answer, 'off')[0]).toMatchObject({ step: 'outcome', plan: { totalMs: 0 } });
		expect(planFx(answer, 'full', undefined, true)[0]).toMatchObject({ plan: { flash: false, shake: false } });
	});
	it('stalled → soft error; refused → the mode closes', () => {
		expect(planFx({ type: 'breakthroughStalled', message: 'x' }, 'full')).toEqual([{ kind: 'mode', mode: 'breakthrough', step: 'stall', message: 'x' }]);
		expect(planFx({ type: 'breakthroughCancelled' }, 'off')).toEqual([{ kind: 'mode', mode: 'breakthrough', step: 'cancel' }]);
	});
});
