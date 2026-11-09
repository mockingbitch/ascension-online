// Breakthrough / lôi kiếp choreography (spec §4.6) on the six effect tiers (decisions 2026-11-09 "Ngoại hình…" D):
// pure — the server's answer + the effect tier of the realm it reached (realms.yaml visual.fx_tier, looked up from the
// server's `to` stage) + effects quality → a timeline of beats that BreakthroughMode.svelte plays (≤ 3 s from the
// answer, skippable, ending on a static result). The branch is chosen ONLY from the server values in the event
// (result, tribulation); nothing here decides an outcome.
//
//   tier 1  (Luyện Khí)      formation spin → pillar → name                       (no realm is entered at tier 1)
//   tier 2  (Trúc Cơ, Kim Đan) + a jade ring spreads, clouds part
//   tier 3  (Nguyên Anh, Hóa Thần) + a second counter-rotating ring, the nascent soul behind
//   tier 4  lôi kiếp: darken → 3 strikes (flash + light shake) → black clouds clear → formation ring → pillar → name
//   tier 5  lôi kiếp: 2 strikes → auspicious five-colour clouds, the heaven gate opens, a large golden name
//   tier 6  lôi kiếp: 1 white strike → galaxy turns, the giant pháp tướng (the player's own figure) rises, a light
//           full-screen shake (full only, never under reduced motion — decision #11), auspicious clouds, the name
//   failure     frosted-glass crack + 200 ms shake → the server's text (tribulation failures keep their tier's prelude)
// Each tier is at least as strong as the previous one: particles, duration and visual layers never decrease (test).
// Lite = shorter (×0.6) with fewer particles and no shake; off = the static result at once; reduced motion = no
// flash/shake whatever the level. Stage-ups inside a realm are not a mode: planStageUp (a light, non-blocking toast).
import type { BreakthroughKind } from '$lib/game/types';
import type { Quality } from './particles';

/** Server kinds: an attempt is always `major`; `auto` stage-ups never reach the mode (planStageUp). */
export type BtOutcome = 'major' | 'tribulation' | 'tribulation_fail' | 'fail';
export type BtBeat =
	| 'spin'
	| 'ring'
	| 'ring2'
	| 'pillar'
	| 'nascent'
	| 'title'
	| 'clouds'
	| 'darken'
	| 'strike'
	| 'flash'
	| 'shake'
	| 'clear'
	| 'auspicious'
	| 'gate'
	| 'galaxy'
	| 'avatar'
	| 'crack'
	| 'result';

export interface BtStep {
	beat: BtBeat;
	/** start, ms after the answer arrived */
	at: number;
	ms: number;
}

export interface BtPlan {
	outcome: BtOutcome;
	/** effect tier played (1–6) */
	tier: number;
	steps: BtStep[];
	/** when the static result is reached (≤ BT_MAX_MS) */
	totalMs: number;
	/** burst size at the reveal (0 when off or on failure) */
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
export const FX_TIERS = 6;

export interface BtInput {
	result: 'success' | 'failure';
	kind: BreakthroughKind;
	tribulation: boolean;
	/** effect tier of the realm the server's `to` stage belongs to (realms.yaml visual.fx_tier); default 2 */
	fxTier?: number;
}

export function btOutcome(e: Pick<BtInput, 'result' | 'tribulation'>): BtOutcome {
	if (e.tribulation) return e.result === 'success' ? 'tribulation' : 'tribulation_fail';
	return e.result === 'failure' ? 'fail' : 'major';
}

/** The tier a branch plays: a tribulation is always tier 4–6, a plain major 1–3 (the ladder guarantees it). */
export function playedTier(outcome: BtOutcome, fxTier = 2): number {
	const t = Math.round(Math.min(FX_TIERS, Math.max(1, fxTier)));
	return outcome === 'tribulation' || outcome === 'tribulation_fail' ? Math.max(4, t) : Math.min(3, t);
}

type Raw = [BtBeat, number, number][];
const strikes = (...at: number[]): Raw => at.flatMap((t) => [['strike', t, 240], ['flash', t, 160], ['shake', t, SHAKE_MS]] as Raw);

/** Full-quality success timelines per tier (ms). */
const SUCCESS: Record<number, Raw> = {
	1: [['spin', 0, 900], ['pillar', 900, 600], ['title', 1000, 700], ['result', 1800, 0]],
	2: [['spin', 0, 1000], ['ring', 700, 800], ['pillar', 1100, 700], ['title', 1300, 800], ['clouds', 1600, 600], ['result', 2200, 0]],
	3: [
		['spin', 0, 1100],
		['ring2', 0, 1100],
		['ring', 900, 700],
		['pillar', 1100, 700],
		['nascent', 1300, 900],
		['title', 1500, 800],
		['clouds', 1900, 600],
		['result', 2500, 0]
	],
	4: [
		['darken', 0, 450],
		['spin', 0, 1900],
		...strikes(500, 1050, 1600),
		['clear', 1900, 500],
		['ring', 1900, 600],
		['pillar', 1950, 550],
		['title', 2100, 700],
		['result', 2800, 0]
	],
	5: [
		['darken', 0, 400],
		['spin', 0, 1300],
		...strikes(450, 950),
		['clear', 1250, 500],
		['auspicious', 1300, 1200],
		['gate', 1500, 1000],
		['pillar', 1700, 700],
		['title', 1800, 900],
		['result', 2900, 0]
	],
	6: [
		['darken', 0, 350],
		['spin', 0, 750],
		...strikes(400),
		['clear', 750, 450],
		['galaxy', 750, 2000],
		['auspicious', 900, 1200],
		['avatar', 1100, 1700],
		['shake', 1500, SHAKE_MS],
		['pillar', 1800, 700],
		['title', 2000, 900],
		['result', 3000, 0]
	]
};

/** Tribulation failures keep their tier's prelude, then the crack. */
const TRIB_FAIL: Record<number, Raw> = {
	4: [['darken', 0, 450], ...strikes(500, 1050, 1600), ['crack', 1950, 350], ['shake', 1950, SHAKE_MS], ['result', 2400, 0]],
	5: [['darken', 0, 400], ...strikes(450, 950), ['gate', 1250, 500], ['crack', 1750, 350], ['shake', 1750, SHAKE_MS], ['result', 2200, 0]],
	6: [['darken', 0, 350], ...strikes(400), ['galaxy', 700, 800], ['crack', 1500, 350], ['shake', 1500, SHAKE_MS], ['result', 2000, 0]]
};

const FAIL: Raw = [['crack', 0, 350], ['shake', 0, SHAKE_MS], ['result', 450, 0]];

/** Reveal burst per tier (full / lite) — within the ParticleSystem budget (≤ 300 / ≤ 80). */
export const TIER_PARTICLES: Record<number, { full: number; lite: number }> = {
	1: { full: 48, lite: 20 },
	2: { full: 90, lite: 30 },
	3: { full: 140, lite: 45 },
	4: { full: 180, lite: 55 },
	5: { full: 240, lite: 65 },
	6: { full: 300, lite: 80 }
};

const LITE_SCALE = 0.6;

function raw(outcome: BtOutcome, tier: number): Raw {
	if (outcome === 'fail') return FAIL;
	if (outcome === 'tribulation_fail') return TRIB_FAIL[tier]!;
	return SUCCESS[tier]!;
}

export function planBreakthrough(e: BtInput, q: Quality, reduced = false): BtPlan {
	const outcome = btOutcome(e);
	const tier = playedTier(outcome, e.fxTier);
	if (q === 'off') return { outcome, tier, steps: [{ beat: 'result', at: 0, ms: 0 }], totalMs: 0, particles: 0, flash: false, shake: false };
	const lite = q === 'lite';
	const flash = !reduced;
	const shake = !reduced && !lite;
	const steps: BtStep[] = raw(outcome, tier)
		.filter(([beat]) => (beat !== 'flash' || flash) && (beat !== 'shake' || shake))
		.map(([beat, at, ms]) => ({
			beat,
			at: lite ? Math.round(at * LITE_SCALE) : at,
			// the shake stays 200 ms (it carries "it failed" / the tier-6 tremor), the rest shortens
			ms: lite && beat !== 'shake' ? Math.round(ms * LITE_SCALE) : ms
		}));
	const totalMs = Math.min(BT_MAX_MS, steps.find((s) => s.beat === 'result')?.at ?? 0);
	const success = outcome === 'major' || outcome === 'tribulation';
	const p = TIER_PARTICLES[tier]!;
	const particles = success ? (lite ? p.lite : p.full) : 0;
	return { outcome, tier, steps, totalMs, particles, flash, shake };
}

/** Visual layers a plan shows (distinct beats except the bookkeeping ones) — the "bigger" measure of the tier test. */
export const planLayers = (p: BtPlan) => new Set(p.steps.map((s) => s.beat).filter((b) => b !== 'result' && b !== 'shake' && b !== 'flash')).size;

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

// ───── stage-ups inside a realm (auto, never a mode) ─────

export interface StageUpPlan {
	/** small burst on the tu vi bar (0 = none) */
	particles: number;
	/** light sweep along the tu vi bar (0 = none) */
	sweepMs: number;
}

/** One settle may pass several stages: ONE light, non-blocking effect (the toast carries the information). */
export function planStageUp(fxTier: number, q: Quality, reduced = false): StageUpPlan {
	if (q === 'off') return { particles: 0, sweepMs: 0 };
	const t = Math.min(FX_TIERS, Math.max(1, Math.round(fxTier)));
	const full = [6, 10, 12, 14, 16, 20][t - 1]!;
	return { particles: q === 'lite' ? Math.max(2, Math.round(full / 3)) : full, sweepMs: reduced ? 0 : q === 'lite' ? 420 : 700 };
}

// ───── persistent realm aura (scene field around the cultivator) ─────

export interface AuraAmbient {
	/** ms between ambient motes (0 = none) */
	everyMs: number;
	/** mote life (ms): live motes ≈ lifeMs / everyMs */
	lifeMs: number;
}

/** Ambient motes of the persistent aura: ≤ 24 live at full, ≤ 8 at lite, none off; more with each tier. */
export function auraAmbient(fxTier: number, q: Quality): AuraAmbient {
	if (q === 'off') return { everyMs: 0, lifeMs: 0 };
	const t = Math.min(FX_TIERS, Math.max(1, Math.round(fxTier)));
	const life = 3000;
	const full = [600, 450, 330, 250, 190, 130][t - 1]!;
	const lite = [1500, 1100, 850, 650, 500, 400][t - 1]!;
	return { everyMs: q === 'lite' ? lite : full, lifeMs: life };
}
