import { describe, expect, it } from 'vitest';
import {
	auraAmbient,
	beatsAt,
	BT_MAX_MS,
	btOutcome,
	planBreakthrough,
	planGather,
	planLayers,
	planStageUp,
	playedTier,
	SHAKE_MS,
	TIER_PARTICLES,
	type BtInput
} from './breakthrough';
import type { FxEvent } from './events';
import { planFx } from './plan';

const major = (fxTier = 2): BtInput => ({ result: 'success', kind: 'major', tribulation: false, fxTier });
const trib = (fxTier = 4): BtInput => ({ result: 'success', kind: 'major', tribulation: true, fxTier });
const tribFail = (fxTier = 4): BtInput => ({ result: 'failure', kind: 'major', tribulation: true, fxTier });
const fail = (fxTier = 2): BtInput => ({ result: 'failure', kind: 'major', tribulation: false, fxTier });
const beats = (i: BtInput, q: 'full' | 'lite' | 'off' = 'full', reduced = false) => planBreakthrough(i, q, reduced).steps.map((s) => s.beat);
const QUALITIES = ['full', 'lite', 'off'] as const;
/** every success celebration a player can see, in ladder order (tier 1 only exists as a fallback) */
const ladder = [major(1), major(2), major(3), trib(4), trib(5), trib(6)];
const everyInput = [...ladder, tribFail(4), tribFail(5), tribFail(6), fail(2), fail(3)];

describe('breakthrough planning: outcome branches come from the server answer only', () => {
	it('maps result/tribulation to four branches (an attempt is always major; auto stage-ups never reach the mode)', () => {
		expect([major(), trib(), tribFail(), fail()].map(btOutcome)).toEqual(['major', 'tribulation', 'tribulation_fail', 'fail']);
		expect(btOutcome({ result: 'success', tribulation: false })).toBe('major');
	});
	it('plays tiers 1–3 for a plain major and 4–6 for a tribulation (clamped, from the realm\'s fx_tier)', () => {
		expect([0, 1, 2, 3, 4, 9].map((t) => playedTier('major', t))).toEqual([1, 1, 2, 3, 3, 3]);
		expect([1, 4, 5, 6, 7].map((t) => playedTier('tribulation', t))).toEqual([4, 4, 5, 6, 6]);
		expect(planBreakthrough({ result: 'success', kind: 'major', tribulation: false }, 'full').tier).toBe(2);
	});
	it('tier 2 major: spin → jade ring → pillar → name → clouds part', () => {
		expect(beats(major(2))).toEqual(['spin', 'ring', 'pillar', 'title', 'clouds', 'result']);
	});
	it('tier 3 adds the counter-rotating second ring and the nascent soul', () => {
		expect(beats(major(3))).toEqual(expect.arrayContaining(['ring2', 'nascent']));
		expect(beats(major(2))).not.toContain('nascent');
	});
	it('tier 4 lôi kiếp: darkens, exactly 3 strikes each with flash + shake, clouds clear, then reveal or crack', () => {
		for (const i of [trib(4), tribFail(4)]) {
			const b = beats(i);
			expect(b[0]).toBe('darken');
			expect(b.filter((x) => x === 'strike')).toHaveLength(3);
			expect(b.filter((x) => x === 'flash')).toHaveLength(3);
		}
		expect(beats(trib(4))).toEqual(expect.arrayContaining(['clear', 'pillar']));
		expect(beats(trib(4))).not.toContain('crack');
		expect(beats(tribFail(4))).toContain('crack');
	});
	it('tier 5: auspicious clouds + the heaven gate; tier 6: galaxy + pháp tướng + one light shake (full only)', () => {
		expect(beats(trib(5))).toEqual(expect.arrayContaining(['auspicious', 'gate']));
		const six = planBreakthrough(trib(6), 'full');
		expect(six.steps.map((s) => s.beat)).toEqual(expect.arrayContaining(['galaxy', 'avatar', 'auspicious']));
		// the strike tremor + the tier-6 tremor, each 200 ms (decision #11)
		expect(six.steps.filter((s) => s.beat === 'shake').every((s) => s.ms === SHAKE_MS)).toBe(true);
		expect(beats(trib(6), 'lite')).not.toContain('shake');
		expect(beats(trib(6), 'full', true)).not.toContain('shake');
	});
	it('failure: crack + a 200 ms shake, then the static result; tribulation failures keep their tier prelude', () => {
		const p = planBreakthrough(fail(), 'full');
		expect(p.steps.find((s) => s.beat === 'shake')).toMatchObject({ at: 0, ms: SHAKE_MS });
		expect(beats(fail())).toEqual(['crack', 'shake', 'result']);
		expect(p.particles).toBe(0);
		expect(beats(tribFail(5))).toContain('gate');
		expect(beats(tribFail(6))).toContain('galaxy');
		for (const t of [4, 5, 6]) expect(planBreakthrough(tribFail(t), 'full').particles).toBe(0);
	});
	it('every branch × tier × quality settles to the static result within 3 s, inside the particle budget', () => {
		for (const i of everyInput)
			for (const q of QUALITIES) {
				const p = planBreakthrough(i, q);
				expect(p.totalMs).toBeLessThanOrEqual(BT_MAX_MS);
				for (const s of p.steps) expect(s.at + s.ms).toBeLessThanOrEqual(BT_MAX_MS);
				expect(p.steps.at(-1)).toMatchObject({ beat: 'result', at: p.totalMs });
				expect(p.particles).toBeLessThanOrEqual(q === 'full' ? 300 : q === 'lite' ? 80 : 0);
			}
	});
});

describe('each tier is at least as strong as the previous one (decisions 2026-11-09)', () => {
	it('particles, duration and visual layers never decrease up the ladder, and every step grows', () => {
		for (const q of ['full', 'lite'] as const) {
			const plans = ladder.map((i) => planBreakthrough(i, q));
			for (let k = 1; k < plans.length; k++) {
				const [a, b] = [plans[k - 1]!, plans[k]!];
				expect(b.particles, `tier ${b.tier} particles (${q})`).toBeGreaterThanOrEqual(a.particles);
				expect(b.totalMs, `tier ${b.tier} duration (${q})`).toBeGreaterThanOrEqual(a.totalMs);
				expect(planLayers(b), `tier ${b.tier} layers (${q})`).toBeGreaterThanOrEqual(planLayers(a));
				expect(b.particles + b.totalMs + planLayers(b)).toBeGreaterThan(a.particles + a.totalMs + planLayers(a));
			}
		}
		expect(Object.values(TIER_PARTICLES).map((p) => p.full)).toEqual([48, 90, 140, 180, 240, 300]);
	});
	it('the persistent aura grows with the tier and stays within ≤ 24 live motes (full) / ≤ 8 (lite)', () => {
		const live = (t: number, q: 'full' | 'lite') => {
			const a = auraAmbient(t, q);
			return a.lifeMs / a.everyMs;
		};
		for (let t = 1; t <= 6; t++) {
			expect(live(t, 'full')).toBeLessThanOrEqual(24);
			expect(live(t, 'lite')).toBeLessThanOrEqual(8);
			if (t > 1) expect(live(t, 'full')).toBeGreaterThan(live(t - 1, 'full'));
		}
		expect(auraAmbient(6, 'off')).toEqual({ everyMs: 0, lifeMs: 0 });
	});
	it('a stage-up is light and non-blocking: a small burst + bar sweep, growing with the tier; off = nothing moves', () => {
		for (let t = 1; t <= 6; t++) {
			const f = planStageUp(t, 'full');
			expect(f.particles).toBeLessThanOrEqual(20);
			expect(planStageUp(t, 'lite').particles).toBeLessThan(f.particles);
			if (t > 1) expect(f.particles).toBeGreaterThan(planStageUp(t - 1, 'full').particles);
		}
		expect(planStageUp(4, 'off')).toEqual({ particles: 0, sweepMs: 0 });
		expect(planStageUp(4, 'full', true).sweepMs).toBe(0);
	});
});

describe('breakthrough planning: skip and effects gating', () => {
	it('skip = jumping to the end: only the static result remains', () => {
		const p = planBreakthrough(trib(), 'full');
		expect(beatsAt(p, 520).has('strike')).toBe(true);
		expect([...beatsAt(p, p.totalMs)]).toEqual(['result']);
		expect([...beatsAt(p, Infinity)]).toEqual(['result']);
	});
	it('lite: shorter, fewer particles, flash kept, no shake (the failure crack stays)', () => {
		for (const i of ladder) {
			const full = planBreakthrough(i, 'full');
			const lite = planBreakthrough(i, 'lite');
			expect(lite.totalMs).toBeLessThan(full.totalMs);
			expect(lite.particles).toBeLessThan(full.particles);
		}
		expect(beats(tribFail(), 'lite')).not.toContain('shake');
		expect(beats(tribFail(), 'lite')).toContain('crack');
		expect(beats(trib(), 'lite').filter((b) => b === 'strike')).toHaveLength(3);
	});
	it('off: an instant static card — no beats, no particles, no flash or shake', () => {
		for (const i of everyInput) {
			const p = planBreakthrough(i, 'off');
			expect(p).toMatchObject({ steps: [{ beat: 'result', at: 0, ms: 0 }], totalMs: 0, particles: 0, flash: false, shake: false });
		}
	});
	it('reduced motion: no flash and no shake even at full, on every tier', () => {
		for (const i of everyInput) {
			const p = planBreakthrough(i, 'full', true);
			expect(p.flash || p.shake).toBe(false);
			expect(p.steps.some((s) => s.beat === 'flash' || s.beat === 'shake')).toBe(false);
		}
	});
	it('the wait loop depends on the preview only (major spins faster; off = still)', () => {
		expect(planGather({ kind: 'major', tribulation: false }, 'full').turnMs).toBeLessThan(planGather({ kind: 'auto', tribulation: false }, 'full').turnMs);
		expect(planGather({ kind: 'auto', tribulation: false }, 'lite').motesPerSec).toBeLessThan(planGather({ kind: 'auto', tribulation: false }, 'full').motesPerSec);
		expect(planGather({ kind: 'major', tribulation: true }, 'off')).toEqual({ motesPerSec: 0, turnMs: 0, dark: true, motion: false });
	});
});

describe('fxBus → breakthrough mode commands (wait → outcome)', () => {
	const answer: FxEvent = { type: 'breakthrough', result: 'failure', kind: 'major', tribulation: true, lossMilli: 1500, from: 'a', to: 'b', failStreak: 2 };
	const ctx = { tierOf: (stage: string) => (stage === 'b' ? 6 : 1) };
	it('start → gather; the outcome only on the server answer, carrying its values and the tier of its `to` stage', () => {
		const [g] = planFx({ type: 'breakthroughStart', kind: 'major', tribulation: true }, 'full');
		expect(g).toMatchObject({ kind: 'mode', step: 'gather', tribulation: true });
		expect(g).not.toHaveProperty('plan');
		const [o] = planFx(answer, 'full', undefined, false, ctx);
		expect(o).toMatchObject({ kind: 'mode', step: 'outcome', plan: { outcome: 'tribulation_fail', tier: 6 }, result: { lossMilli: 1500, failStreak: 2 } });
	});
	it('the mode opens even with effects off (a static card), and reduced motion reaches the plan', () => {
		expect(planFx(answer, 'off')[0]).toMatchObject({ step: 'outcome', plan: { totalMs: 0 } });
		expect(planFx(answer, 'full', undefined, true)[0]).toMatchObject({ plan: { flash: false, shake: false } });
	});
	it('stalled → soft error; refused → the mode closes', () => {
		expect(planFx({ type: 'breakthroughStalled', message: 'x' }, 'full')).toEqual([{ kind: 'mode', mode: 'breakthrough', step: 'stall', message: 'x' }]);
		expect(planFx({ type: 'breakthroughCancelled' }, 'off')).toEqual([{ kind: 'mode', mode: 'breakthrough', step: 'cancel' }]);
	});
	it('several stage-ups from one settle = ONE stage-up command with one toast; off keeps the toast only', () => {
		const names = { tierOf: () => 1, stageName: (s: string) => (s === 'x.3' ? 'Tầng 3' : 'Tầng 5') };
		const cmds = planFx({ type: 'stageUp', from: 'x.3', to: 'x.5' }, 'full', undefined, false, names);
		expect(cmds).toHaveLength(1);
		expect(cmds[0]).toMatchObject({ kind: 'stageUp', text: 'Lên tầng: Tầng 3 → Tầng 5', particles: 6 });
		expect(planFx({ type: 'stageUp', from: 'x.3', to: 'x.5' }, 'off', undefined, false, names)[0]).toMatchObject({ kind: 'stageUp', particles: 0, sweepMs: 0 });
	});
});
