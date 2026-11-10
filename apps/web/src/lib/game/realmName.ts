// Names by realm tier in Tử Tiêu Cung (decisions 2026-11-10): how a name is drawn from the realm's effect tier
// (realms.yaml visual.fx_tier 1–6) and path, and how a purchasable chat aura (cosmetics.yaml chat_aura:*) combines
// with it. Pure — RealmName.svelte renders it; realmName.test.ts checks the escalation and the fairness rule.
//
// Fairness rule (decision: "realm-tier styling always outranks purchasable Tiên Duyên chat auras"): the REALM owns
// the name — colour, glow, motion and glints come only from the tier. A chat aura never fills, animates or lights
// the name; it is shown as a thin static underline in its own colours (`accent`), which is weaker than the
// faintest tier effect. So a tier-1 name with the richest aura still looks like a tier-1 name.
import type { PathId } from './types';

export type NameEffect = 'plain' | 'glow' | 'shimmer' | 'flow' | 'prismatic';

export interface RealmNameStyle {
	tier: 1 | 2 | 3 | 4 | 5 | 6;
	path: PathId;
	effect: NameEffect;
	/** glow radius in px (0 = none) */
	glow: number;
	/** the text runs an animation (at effects full/lite; static at off / reduced motion) */
	animated: boolean;
	/** star glints around the name (tier 6) */
	glints: boolean;
	weight: 500 | 600 | 700;
	/** colour tokens (app.css --color-rname-<path>-<tier><stop>) the text uses, first = base */
	stops: string[];
}

const EFFECT: Record<RealmNameStyle['tier'], { effect: NameEffect; glow: number; weight: RealmNameStyle['weight']; stops: string }> = {
	1: { effect: 'plain', glow: 0, weight: 500, stops: 'a' },
	2: { effect: 'glow', glow: 3, weight: 600, stops: 'a' },
	3: { effect: 'glow', glow: 6, weight: 600, stops: 'a' },
	4: { effect: 'shimmer', glow: 6, weight: 600, stops: 'ab' },
	5: { effect: 'flow', glow: 8, weight: 700, stops: 'abc' },
	6: { effect: 'prismatic', glow: 10, weight: 700, stops: 'abc' }
};

export const clampTier = (t: number | undefined): RealmNameStyle['tier'] =>
	Math.max(1, Math.min(6, Math.round(Number.isFinite(t) ? (t as number) : 1))) as RealmNameStyle['tier'];

/** The style of a name at effect tier `fxTier` on `path`. */
export function realmNameStyle(fxTier: number | undefined, path: PathId = 'tien'): RealmNameStyle {
	const tier = clampTier(fxTier);
	const e = EFFECT[tier];
	return {
		tier,
		path,
		effect: e.effect,
		glow: e.glow,
		animated: tier >= 4,
		glints: tier === 6,
		weight: e.weight,
		stops: [...e.stops].map((s) => `--color-rname-${path}-${tier}${s}`)
	};
}

/** Chat aura colours for the underline accent (tokens only; mirrors NameFx.svelte's palettes). */
const ACCENT: Record<string, string[]> = {
	'fx-mac-huong': ['--color-mist-500', '--color-mist-100'],
	'fx-bich-lam': ['--color-jade-300', '--color-spirit-cyan'],
	'fx-kim-sa': ['--color-gold-500', '--color-gold-300'],
	'fx-tinh-ha': ['--color-spirit-violet', '--color-spirit-cyan'],
	'fx-linh-quang': ['--color-jade-300', '--color-aura-celestial'],
	'fx-tu-quang': ['--color-spirit-violet', '--color-aura-primal']
};

export interface AuraAccent {
	/** colour tokens of the static underline */
	stops: string[];
	/** never: the aura does not animate, glow or fill the text */
	animated: false;
	glow: 0;
}

/** How a chat aura (its NameFx class, `fx-<slug>`) shows on a realm-styled name: a static underline, or nothing. */
export function auraAccent(fxClass: string | undefined): AuraAccent | null {
	if (!fxClass) return null;
	return { stops: ACCENT[fxClass] ?? ['--color-gold-500', '--color-gold-300'], animated: false, glow: 0 };
}

/** A comparable "visual weight" of a tier style (for the escalation test). */
export const styleWeight = (s: RealmNameStyle) =>
	s.glow + s.stops.length * 4 + (s.animated ? 6 : 0) + (s.glints ? 6 : 0) + (s.weight - 500) / 50;
