// Vân Khê's tutorial (decisions 2026-11-09 #1, GĐ-B) — pure rules, unit-tested: which step a trigger shows, how the
// visual-novel overlay is laid out (she is big and on top of everything, but never over the control she points at),
// and the "?" summon lines. Display only.
import type { TutorialLine, TutorialScript, TutorialState, TutorialStep } from './types';

/** The stored (play) step a trigger shows now, or undefined (muted, already seen here or on another device, unknown). */
export function stepFor(
	script: TutorialScript | null,
	trigger: string,
	state: TutorialState | null,
	seen: ReadonlySet<string> = new Set()
): TutorialStep | undefined {
	if (!script || !state || state.muted) return undefined;
	const s = script.steps.find((x) => x.flow === 'play' && x.trigger === trigger);
	if (!s || seen.has(s.id) || state.done.includes(s.id) || state.skipped.includes(s.id)) return undefined;
	return s;
}

/** The onboarding step of a /create wizard step (client-only progress), e.g. 'the-luc' → trigger onboarding.the_luc. */
export function onboardingStep(script: TutorialScript | null, wizardStep: string): TutorialStep | undefined {
	const trigger = `onboarding.${wizardStep.replaceAll('-', '_')}`;
	return script?.steps.find((x) => x.flow === 'onboarding' && x.trigger === trigger);
}

/** "?" summon: the help points of a topic read by the guide (one point per line). */
export function helpLines(points: string[]): TutorialLine[] {
	return points.map((text, i) => ({ expr: i === 0 ? 'smile' : i % 2 ? 'point' : 'neutral', text }));
}

export interface Rect {
	x: number;
	y: number;
	w: number;
	h: number;
}

/**
 * The visual-novel overlay: Vân Khê's portrait (768×1024 art, 3:4), the dialogue box, and the spotlight hole cut in
 * the backdrop around the control a line points at.
 * - `side`: she stands at the left/right edge, the box over her lower half toward the middle (desktop / tablet); on a
 *   short landscape phone she and the box sit side by side.
 * - `stack` (portrait phone): she fills about the upper half, the box below — or flipped when the target is up there.
 */
export interface GuideLayout {
	mode: 'side' | 'stack';
	/** side: the edge she stands at; stack: top/bottom half */
	at: 'left' | 'right' | 'top' | 'bottom';
	portrait: Rect;
	box: Rect;
	/** the spotlight target grown by HOLE_PAD — the backdrop is cut here so it stays visible and clickable */
	hole?: Rect;
	/** nothing avoided the target: the box (worse) or her visible figure still covers part of the hole */
	covers: { box: boolean; portrait: boolean };
}

const GAP = 8;
const M = 12; // screen margin
export const HOLE_PAD = 6;
const ASPECT = 0.75; // portrait art w/h

function overlap(a: Rect, b: Rect) {
	const w = Math.min(a.x + a.w, b.x + b.w) - Math.max(a.x, b.x);
	const h = Math.min(a.y + a.h, b.y + b.h) - Math.max(a.y, b.y);
	return w > 0 && h > 0 ? w * h : 0;
}
/** the opaque part of the portrait (the art has transparent margins round her) */
function figure(p: Rect): Rect {
	return { x: p.x + p.w * 0.16, y: p.y + p.h * 0.04, w: p.w * 0.68, h: p.h * 0.96 };
}
const clamp = (v: number, lo: number, hi: number) => Math.max(lo, Math.min(hi, v));

/**
 * Lay the overlay out for a viewport, the dialogue box's height (measured; an estimate before the first paint) and an
 * optional spotlight target. Candidates are tried in preference order — she stands on the side away from the target,
 * the box at the bottom, then just above / below the target, then at the top — and the first that leaves the hole free wins;
 * otherwise the one covering least, the box weighing far more than her (the target wins over the box; she stays big).
 * Pure.
 */
export function layoutGuide(view: { w: number; h: number }, boxH: number, target?: Rect): GuideLayout {
	const { w: vw, h: vh } = view;
	const hole = target
		? { x: target.x - HOLE_PAD, y: target.y - HOLE_PAD, w: target.w + 2 * HOLE_PAD, h: target.h + 2 * HOLE_PAD }
		: undefined;
	const bh = Math.min(boxH, vh - 2 * M);
	const ys = (): number[] => {
		const bottom = vh - bh - M;
		// next to the target before the far edge: a box at the top of a phone would sit on her face
		const list = hole ? [bottom, hole.y - GAP - bh, hole.y + hole.h + GAP, M] : [bottom];
		return list.map((y) => clamp(Math.round(y), M, Math.max(M, bottom)));
	};
	const stack = vw < vh && vw < 700;
	type Cand = Omit<GuideLayout, 'hole' | 'covers'>;
	const cands: Cand[] = [];

	if (stack) {
		let ph = Math.round(vh * 0.58);
		let pw = Math.round(ph * ASPECT);
		if (pw > vw) [pw, ph] = [vw, Math.round(vw / ASPECT)];
		const x = Math.round((vw - pw) / 2);
		const halves: ('top' | 'bottom')[] = hole && hole.y + hole.h / 2 < vh / 2 ? ['bottom', 'top'] : ['top', 'bottom'];
		for (const at of halves) {
			const portrait = { x, y: at === 'top' ? Math.round(vh * 0.02) : vh - ph, w: pw, h: ph };
			for (const y of ys()) cands.push({ mode: 'stack', at, portrait, box: { x: M, y, w: vw - 2 * M, h: bh } });
		}
	} else {
		const short = vh < 520;
		let ph = Math.round(vh * (short ? 0.98 : 0.92));
		let pw = Math.round(ph * ASPECT);
		const maxW = Math.round(vw * (short ? 0.36 : 0.5));
		if (pw > maxW) [pw, ph] = [maxW, Math.round(maxW / ASPECT)];
		// side by side on a short screen; otherwise the box overlaps her lower half (the classic VN framing)
		const inner = Math.round(pw * (short ? 0.86 : 0.55));
		const bw = Math.min(short ? vw : 760, vw - inner - M);
		const sides: ('left' | 'right')[] = hole && hole.x + hole.w / 2 < vw / 2 ? ['right', 'left'] : ['left', 'right'];
		for (const at of sides) {
			const portrait = { x: at === 'left' ? 0 : vw - pw, y: vh - ph, w: pw, h: ph };
			const bx = at === 'left' ? inner : vw - inner - bw;
			for (const y of ys()) cands.push({ mode: 'side', at, portrait, box: { x: bx, y, w: bw, h: bh } });
		}
	}

	let best = cands[0]!;
	let bestBox = 0;
	let bestFig = 0;
	if (hole) {
		let bestScore = Infinity;
		for (const c of cands) {
			const onBox = overlap(c.box, hole);
			const onFig = overlap(figure(c.portrait), hole);
			const score = onBox * 1000 + onFig;
			if (score < bestScore) [best, bestScore, bestBox, bestFig] = [c, score, onBox, onFig];
			if (score === 0) break;
		}
	}
	return { ...best, hole, covers: { box: bestBox > 0, portrait: bestFig > 0 } };
}
