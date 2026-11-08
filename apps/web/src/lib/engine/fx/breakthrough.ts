// Breakthrough / lôi kiếp choreography (spec §4.6). Pure: the server's answer + effects quality → a timeline of beats
// that BreakthroughMode.svelte plays (≤ 3 s from the answer, skippable, ending on a static result). The branch is
// chosen ONLY from the server values in the event (result, kind, tribulation); nothing here decides an outcome.
//   minor     aura gathers (0.8 s) → flash → new realm aura + new stage text
//   major     formation spins faster (1.2 s) → pillar of light → realm name in calligraphy → clouds part
//   lôi kiếp  sky darkens, black clouds roll, 3 strikes (flash + light shake) → reveal (success) or crack (failure)
//   failure   frosted-glass crack + 200 ms shake → the server's text
// Lite = shorter (×0.6) with fewer particles and no shake; off = the static result at once; reduced motion = no
// flash/shake whatever the level.
import type { BreakthroughKind } from '$lib/game/types';
import type { Quality } from './particles';

export type BtOutcome = 'minor' | 'major' | 'tribulation' | 'tribulation_fail' | 'fail';
export type BtBeat = 'gather' | 'flash' | 'aura' | 'spin' | 'pillar' | 'title' | 'clouds' | 'darken' | 'strike' | 'shake' | 'crack' | 'result';

export interface BtStep {
	beat: BtBeat;
	/** start, ms after the answer arrived */
	at: number;
	ms: number;
}

export interface BtPlan {
	outcome: BtOutcome;
	steps: BtStep[];
	/** when the static result is reached (≤ BT_MAX_MS) */
	totalMs: number;
	/** burst size at the reveal (0 when off) */
	particles: number;
	flash: boolean;
	shake: boolean;
}

/** While waiting for the answer: the looping gather. */
export interface GatherPlan {
	/** motes spawned per second converging on the cultivator (0 = none) */
	motesPerSec: number;
	/** formation turn period (ms); major spins faster */
	turnMs: number;
	/** lôi kiếp: the sky is already dark while waiting */
	dark: boolean;
	motion: boolean;
}

export const BT_MAX_MS = 3000;
export const SHAKE_MS = 200;

export interface BtInput {
	result: 'success' | 'failure';
	kind: BreakthroughKind;
	tribulation: boolean;
}

export function btOutcome(e: BtInput): BtOutcome {
	if (e.tribulation) return e.result === 'success' ? 'tribulation' : 'tribulation_fail';
	if (e.result === 'failure') return 'fail';
	return e.kind === 'major' ? 'major' : 'minor';
}

type Raw = [BtBeat, number, number][];

/** Full-quality timelines (ms). */
const FULL: Record<BtOutcome, Raw> = {
	minor: [
		['gather', 0, 800],
		['flash', 800, 160],
		['aura', 880, 700],
		['title', 950, 600],
		['result', 1600, 0]
	],
	major: [
		['spin', 0, 1200],
		['pillar', 1200, 700],
		['title', 1450, 800],
		['clouds', 1900, 700],
		['result', 2600, 0]
	],
	tribulation: [
		['darken', 0, 450],
		['strike', 500, 240],
		['flash', 500, 160],
		['shake', 500, SHAKE_MS],
		['strike', 1050, 240],
		['flash', 1050, 160],
		['shake', 1050, SHAKE_MS],
		['strike', 1600, 240],
		['flash', 1600, 160],
		['shake', 1600, SHAKE_MS],
		['pillar', 1950, 500],
		['title', 2100, 600],
		['result', 2700, 0]
	],
	tribulation_fail: [
		['darken', 0, 450],
		['strike', 500, 240],
		['flash', 500, 160],
		['shake', 500, SHAKE_MS],
		['strike', 1050, 240],
		['flash', 1050, 160],
		['shake', 1050, SHAKE_MS],
		['strike', 1600, 240],
		['flash', 1600, 160],
		['shake', 1600, SHAKE_MS],
		['crack', 1950, 350],
		['shake', 1950, SHAKE_MS],
		['result', 2400, 0]
	],
	fail: [
		['crack', 0, 350],
		['shake', 0, SHAKE_MS],
		['result', 450, 0]
	]
};

const LITE_SCALE = 0.6;

export function planBreakthrough(e: BtInput, q: Quality, reduced = false): BtPlan {
	const outcome = btOutcome(e);
	if (q === 'off') return { outcome, steps: [{ beat: 'result', at: 0, ms: 0 }], totalMs: 0, particles: 0, flash: false, shake: false };
	const lite = q === 'lite';
	const flash = !reduced;
	const shake = !reduced && !lite;
	const steps: BtStep[] = FULL[outcome]
		.filter(([beat]) => (beat !== 'flash' || flash) && (beat !== 'shake' || shake))
		.map(([beat, at, ms]) => ({
			beat,
			at: lite ? Math.round(at * LITE_SCALE) : at,
			// the shake stays 200 ms (it carries "it failed"), the rest shortens
			ms: lite && beat !== 'shake' ? Math.round(ms * LITE_SCALE) : ms
		}));
	const totalMs = Math.min(BT_MAX_MS, steps.find((s) => s.beat === 'result')?.at ?? 0);
	const success = outcome === 'minor' || outcome === 'major' || outcome === 'tribulation';
	const particles = success ? (lite ? 20 : outcome === 'minor' ? 48 : 72) : 0;
	return { outcome, steps, totalMs, particles, flash, shake };
}

/** The wait loop, from the preview the player confirmed (kind, tribulation): never an outcome. */
export function planGather(e: { kind: BreakthroughKind; tribulation: boolean }, q: Quality): GatherPlan {
	const major = e.kind === 'major' || e.tribulation;
	if (q === 'off') return { motesPerSec: 0, turnMs: 0, dark: e.tribulation, motion: false };
	return { motesPerSec: q === 'lite' ? 6 : 18, turnMs: major ? 2400 : 6000, dark: e.tribulation, motion: true };
}

/** Beats active `t` ms after the answer (for the player and tests). A skip = t ≥ totalMs. */
export function beatsAt(plan: BtPlan, t: number): Set<BtBeat> {
	const out = new Set<BtBeat>();
	if (t >= plan.totalMs) {
		out.add('result');
		return out;
	}
	for (const s of plan.steps) if (t >= s.at && t < s.at + Math.max(s.ms, 1)) out.add(s.beat);
	return out;
}
