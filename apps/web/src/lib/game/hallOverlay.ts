// Tử Tiêu Cung overlay planning (owner review 2026-11-10): name labels over every seated listener and chat bubbles
// that never overlap or get clipped. Pure (positions in the scene box's px) — HallView feeds it projected seat points,
// measured bubble sizes and the boxes to keep clear (header overlay, subtitles, chat dock, the "Bạn" tag).
import { intersects, relaxLabels, type Rect } from '$lib/engine/layout/regions';

/** How long after a chat line its speaker's name stays visible on phones. */
export const SPOKE_MS = 10_000;
/** At most this many bubbles at once (the newest); older ones fade out. Phones / short screens: PHONE_BUBBLES. */
export const MAX_BUBBLES = 5;
export const PHONE_BUBBLES = 2;
/** Names longer than this are shortened on the label (the hover/tap card has the full name). */
export const LABEL_MAX_CHARS = 14;

export const shortName = (name: string, max = LABEL_MAX_CHARS) => {
	const chars = [...name];
	return chars.length <= max ? name : chars.slice(0, max - 1).join('').trimEnd() + '…';
};

/** Label width estimate (Be Vietnam Pro, semibold): ~0.6 em per character + padding. Deterministic, no DOM read. */
export const labelWidth = (text: string, fontPx: number) => Math.ceil([...text].length * fontPx * 0.6 + fontPx * 0.9);

export interface SeatBox {
	id: string;
	/** anchor (feet on the cushion) */
	x: number;
	y: number;
	/** drawn sitter size (square) */
	size: number;
}

/** The sitter's box: the seat element is `size` square, translated (-50%, -88%) from its anchor. */
export const figureRect = (s: SeatBox): Rect => ({ left: s.x - s.size / 2, right: s.x + s.size / 2, top: s.y - 0.88 * s.size, bottom: s.y + 0.12 * s.size });
/** Where a speech bubble / label points to: just over the sitter's head. */
export const headPoint = (s: SeatBox) => ({ x: s.x, y: s.y - 0.8 * s.size });

export interface LabelInput extends SeatBox {
	name: string;
	mine: boolean;
	partner: boolean;
	sameFaction: boolean;
	fxTier: number;
	/** ms timestamp of the last chat line (0 = none) */
	spokeAt: number;
}

export type LabelState = 'shown' | 'faded' | 'hidden';

export interface LabelPlan {
	id: string;
	text: string;
	x: number;
	y: number;
	w: number;
	h: number;
	state: LabelState;
}

/** "Tên: Quan trọng" draws at most this many names (the rest show on hover / tap). */
export const IMPORTANT_CAP = 12;

export interface LabelOpts {
	/** Tên: Quan trọng (important names, capped) · Tất cả (all, collisions hidden) · Ẩn (none) — settings.hallNames */
	mode: 'important' | 'all' | 'off';
	now: number;
	/** seats in focus: the one under the pointer / tapped and its nearest neighbours (their names show) */
	focus?: ReadonlySet<string>;
	fontPx: number;
	/** boxes no label may cover (the "Bạn" tag, header overlay, subtitles, chat dock): a label there is hidden */
	obstacles: readonly Rect[];
	bounds: Rect;
}

/** "Quan trọng" names: me, my đạo lữ, my faction, fx_tier ≥ 4, anyone who spoke lately, the seats in focus. */
export function isImportant(l: LabelInput, now: number, focus?: ReadonlySet<string>): boolean {
	return l.mine || l.partner || l.sameFaction || l.fxTier >= 4 || now - l.spokeAt < SPOKE_MS || !!focus?.has(l.id);
}

const rank = (l: LabelInput, now: number, focus?: ReadonlySet<string>) =>
	(now - l.spokeAt < SPOKE_MS ? 1e7 : 0) +
	(focus?.has(l.id) ? 1e6 : 0) +
	(l.partner ? 1e5 : 0) +
	(l.fxTier >= 4 ? l.fxTier * 1e4 : 0) +
	(l.sameFaction ? 1e3 : 0) +
	l.y; // nearer = lower on screen = first

/** The seats near `id` (by distance between anchors): the hovered/tapped seat plus its `n` nearest neighbours. */
export function focusAround(id: string | null | undefined, seats: readonly { id: string; x: number; y: number }[], n = 4): Set<string> {
	const c = id ? seats.find((s) => s.id === id) : undefined;
	if (!c) return new Set();
	const near = seats
		.filter((s) => s.id !== c.id)
		.map((s) => ({ id: s.id, d: Math.hypot(s.x - c.x, s.y - c.y) }))
		.sort((a, b) => a.d - b.d)
		.slice(0, n);
	return new Set([c.id, ...near.map((s) => s.id)]);
}

/**
 * Name labels: one above each sitter (my own name is the "Bạn" tag, not a label) — in "important" mode only the
 * isImportant() ones, at most IMPORTANT_CAP; "off" none. Placed in priority order (spoke lately, in focus, đạo lữ,
 * high realm, faction, then nearer seats first): the first free spot of [above the head, one line
 * higher]; a label over a nearer sitter's figure is `faded`; one that still overlaps a placed label, an obstacle or
 * the bounds is `hidden` (never half-covered).
 */
export function planNameLabels(items: readonly LabelInput[], o: LabelOpts): LabelPlan[] {
	const h = Math.ceil(o.fontPx * 1.5);
	const placed: Rect[] = [];
	const figures = items.map((s) => ({ y: s.y, r: figureRect(s) }));
	const inBounds = (r: Rect) => r.left >= o.bounds.left && r.right <= o.bounds.right && r.top >= o.bounds.top && r.bottom <= o.bounds.bottom;
	const out: LabelPlan[] = [];
	if (o.mode === 'off') return [];
	let order = [...items].filter((l) => !l.mine && (o.mode === 'all' || isImportant(l, o.now, o.focus))).sort((a, b) => rank(b, o.now, o.focus) - rank(a, o.now, o.focus));
	if (o.mode === 'important') order = order.slice(0, IMPORTANT_CAP);
	for (const l of order) {
		const text = shortName(l.name);
		const w = labelWidth(text, o.fontPx);
		const head = headPoint(l);
		const at = (dy: number) => ({ x: head.x, y: head.y - h / 2 - 2 - dy });
		const rect = (c: { x: number; y: number }): Rect => ({ left: c.x - w / 2, right: c.x + w / 2, top: c.y - h / 2, bottom: c.y + h / 2 });
		const free = (r: Rect) => !placed.some((p) => intersects(p, r));
		let c = at(0);
		if (!free(rect(c)) && free(rect(at(h + 1)))) c = at(h + 1);
		const r = rect(c);
		let state: LabelState = 'shown';
		// never drawn half under something: a label still colliding with a placed one (or an obstacle) is hidden; only
		// a label over a nearer sitter's figure is faded (the sitter shows through it)
		if (o.obstacles.some((b) => intersects(b, r)) || !inBounds(r) || !free(r)) state = 'hidden';
		else if (figures.some((f) => f.y > l.y + 0.5 && intersects(f.r, r))) state = 'faded';
		if (state !== 'hidden') placed.push(r);
		out.push({ id: l.id, text, x: c.x, y: c.y, w, h, state });
	}
	return out;
}

export interface BubbleInput {
	id: number;
	/** speaker's head point */
	ax: number;
	ay: number;
	/** measured size (0 before the first measure) */
	w: number;
	h: number;
	/** ms timestamp of the line */
	at: number;
}

export interface BubblePlan {
	id: number;
	x: number;
	y: number;
	/** bubble anchored to its speaker: tail from the bubble's edge to (ax, ay) */
	ax: number;
	ay: number;
	visible: boolean;
}

/**
 * Speech bubbles: the newest MAX_BUBBLES are laid out (above their speaker, then nudged apart and off the obstacles
 * with relaxLabels, inside `bounds`); older ones are not visible (they fade out), nor is one that still can't find
 * free room (a short screen shows fewer). Unmeasured bubbles wait hidden.
 */
export interface BubbleOpts {
	/** at most this many at once (desktop MAX_BUBBLES; phones / short screens 2) */
	max?: number;
	/**
	 * Soft obstacles, strongest preference first (e.g. [name labels + the seat block, name labels]): a bubble first
	 * looks for room clear of all of them, then drops one tier at a time; only the hard `obstacles` are never covered.
	 * Whatever soft box a placed bubble still covers is reported in `covered` (the caller hides those labels).
	 */
	soft?: readonly (readonly Rect[])[];
	/** extra starting points to try besides "just above the speaker" (free space above the seats, the sides) */
	spots?: readonly { x: number; y: number }[];
	gap?: number;
}

/**
 * Speech bubbles, newest first (the newest lines win the room; older ones and anything beyond `max` are not visible
 * and fade out). Each bubble tries starting points (above its speaker, then `spots`), nudged with relaxLabels clear
 * of the hard obstacles, the bubbles already placed and the soft tiers — the candidate nearest its speaker wins; when
 * no candidate is clear of a soft tier, that tier is dropped. A bubble that can't clear the hard obstacles / bounds
 * is not shown (a short screen shows fewer, never overlapping or clipped ones). Unmeasured bubbles wait hidden.
 */
export function planBubbles(items: readonly BubbleInput[], bounds: Rect, obstacles: readonly Rect[], o: BubbleOpts = {}): BubblePlan[] {
	const gap = o.gap ?? 6;
	const newest = [...items].sort((a, b) => b.at - a.at || b.id - a.id);
	const live = newest.slice(0, o.max ?? MAX_BUBBLES).filter((b) => b.w > 0 && b.h > 0);
	const at = new Map<number, { x: number; y: number }>();
	const kept: Rect[] = [];
	const inside = (r: Rect) => r.left >= bounds.left && r.right <= bounds.right && r.top >= bounds.top && r.bottom <= bounds.bottom;
	const tiers = [...(o.soft ?? []), []];
	for (const b of live) {
		const box = (p: { x: number; y: number }): Rect => ({ left: p.x - b.w / 2, right: p.x + b.w / 2, top: p.y - b.h / 2, bottom: p.y + b.h / 2 });
		const starts = [{ x: b.ax, y: b.ay - 10 - b.h / 2 }, ...(o.spots ?? [])];
		let best: { x: number; y: number } | null = null;
		for (const soft of tiers) {
			const avoid = [...obstacles, ...kept, ...soft];
			let bestD = Infinity;
			for (const s of starts) {
				const p = relaxLabels([{ ...s, w: b.w, h: b.h }], bounds, { gap, avoid })[0]!;
				const r = box(p);
				if (!inside(r) || avoid.some((x) => intersects(x, r))) continue;
				const d = Math.hypot(p.x - b.ax, p.y - b.ay);
				if (d < bestD) (bestD = d), (best = { x: p.x, y: p.y });
			}
			if (best) break;
		}
		if (!best) continue;
		kept.push(box(best));
		at.set(b.id, best);
	}
	return items.map((b) => {
		const p = at.get(b.id);
		return p ? { id: b.id, x: p.x, y: p.y, ax: b.ax, ay: b.ay, visible: true } : { id: b.id, x: b.ax, y: b.ay - 10 - b.h / 2, ax: b.ax, ay: b.ay, visible: false };
	});
}

/** Ids of the labels a visible bubble covers even partly (the caller hides them completely). */
export function coveredLabels(labels: readonly LabelPlan[], bubbles: readonly { x: number; y: number; w: number; h: number }[]): Set<string> {
	const rect = (b: { x: number; y: number; w: number; h: number }): Rect => ({ left: b.x - b.w / 2, right: b.x + b.w / 2, top: b.y - b.h / 2, bottom: b.y + b.h / 2 });
	const rs = bubbles.map(rect);
	return new Set(labels.filter((l) => l.state !== 'hidden' && rs.some((r) => intersects(r, rect(l)))).map((l) => l.id));
}

/** The label boxes (for bubbles to avoid). */
export const labelRects = (labels: readonly LabelPlan[]): Rect[] =>
	labels.filter((l) => l.state !== 'hidden').map((l) => ({ left: l.x - l.w / 2, right: l.x + l.w / 2, top: l.y - l.h / 2, bottom: l.y + l.h / 2 }));

/** Tail triangle (SVG points) from the bubble box edge nearest the speaker to the speaker's head. */
export function tailPoints(b: { x: number; y: number; w: number; h: number; ax: number; ay: number }, half = 6): string {
	const below = b.ay >= b.y;
	const edgeY = below ? b.y + b.h / 2 - 1 : b.y - b.h / 2 + 1;
	const ex = Math.max(b.x - b.w / 2 + half + 4, Math.min(b.x + b.w / 2 - half - 4, b.ax));
	return `${ex - half},${edgeY} ${ex + half},${edgeY} ${b.ax},${b.ay}`;
}
