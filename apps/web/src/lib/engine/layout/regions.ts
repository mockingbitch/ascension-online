// Layout regions of the /play shell (UI polish U2) — pure, unit-tested. The shell measures its regions once per
// resize (one ResizeObserver) and publishes them as CSS custom properties on <html>; everything else (pages, toasts,
// the install hint, scene anchors) lays out against those variables instead of patching margins by hand.
//
//   ┌──────────── HUD (--hud-h: HUD + ticker) ────────────┐
//   │ STAGE (page UI, one scroller)              │ RAIL   │  wide (≥ 640): rail on the right (--rail-w)
//   │ CHAT DOCK (wide: bottom-left box --chat-w × │        │
//   │            --chat-reserve inside the stage) │        │
//   ├──── CHAT DOCK (phone: own row, --chat-reserve) ──────┤  phone (< 640): rows hud / stage / chat / bar
//   └──────────── BAR (--bar-h, + --safe-b) ──────────────┘

export interface RegionSizes {
	/** viewport */
	vw: number;
	vh: number;
	/** HUD region height (HUD + world ticker) */
	hud: number;
	/** bottom action row height without the safe-area padding (0 on wide layouts) */
	bar: number;
	/** right rail width (0 on phones) */
	rail: number;
	/** collapsed chat dock box */
	chatW: number;
	chatH: number;
	/** env(safe-area-inset-bottom) in px */
	safeB: number;
	wide: boolean;
}

export interface Rect {
	top: number;
	right: number;
	bottom: number;
	left: number;
}

const px = (n: number) => `${Math.max(0, Math.round(n))}px`;

/** The custom properties the shell sets on <html>. */
export function regionVars(s: RegionSizes): Record<string, string> {
	const bottom = s.wide ? 0 : s.bar + s.safeB + s.chatH;
	return {
		'--hud-h': px(s.hud),
		'--bar-h': px(s.wide ? 0 : s.bar),
		'--rail-w': px(s.wide ? s.rail : 0),
		'--chat-w': px(s.chatW),
		'--chat-reserve': px(s.chatH),
		'--stage-inset-top': px(s.hud),
		'--stage-inset-right': px(s.wide ? s.rail : 0),
		'--stage-inset-bottom': px(bottom),
		'--stage-inset-left': '0px'
	};
}

/** The stage rectangle in viewport px (where page UI and scene-anchored UI may draw). */
export function stageRect(s: RegionSizes): Rect {
	return {
		top: s.hud,
		left: 0,
		right: s.vw - (s.wide ? s.rail : 0),
		bottom: s.vh - (s.wide ? 0 : s.bar + s.safeB + s.chatH)
	};
}

/** Wide layouts keep the chat dock inside the stage's bottom-left corner: the box page UI has to stay clear of. */
export function chatBox(s: RegionSizes): Rect | null {
	if (!s.wide) return null;
	const r = stageRect(s);
	return { left: r.left, right: r.left + s.chatW, bottom: r.bottom, top: r.bottom - s.chatH };
}

/**
 * Clamp a box of size w×h centred at (x, y) into `r` (inset by `pad`). Returns the clamped centre.
 * When the box is larger than the rect it is centred on it.
 */
export function clampToRect(x: number, y: number, w: number, h: number, r: Rect, pad = 0): { x: number; y: number } {
	const fit = (c: number, size: number, lo: number, hi: number) => {
		const min = lo + pad + size / 2;
		const max = hi - pad - size / 2;
		return min > max ? (lo + hi) / 2 : Math.min(max, Math.max(min, c));
	};
	return { x: fit(x, w, r.left, r.right), y: fit(y, h, r.top, r.bottom) };
}

export const intersects = (a: Rect, b: Rect) => a.left < b.right && b.left < a.right && a.top < b.bottom && b.top < a.bottom;

/** A label box: centre + size (scene plaques). */
export interface LabelBox {
	x: number;
	y: number;
	w: number;
	h: number;
}

const boxRect = (b: LabelBox, gap: number): Rect => ({ left: b.x - b.w / 2 - gap / 2, right: b.x + b.w / 2 + gap / 2, top: b.y - b.h / 2 - gap / 2, bottom: b.y + b.h / 2 + gap / 2 });

/**
 * Nudge overlapping labels apart (map plaques on small maps), deterministically:
 *   1. clamp every label into `bounds`;
 *   2. sweep down (top to bottom): a label that overlaps an obstacle (an `avoid` box: page card, chat dock) or a label
 *      already placed moves just below it — or just beside it when that is shorter and free;
 *   3. sweep up (bottom to top) for whatever left the bounds: it moves just above the obstacle / label below it.
 * Returns new centres in input order; labels that never overlap anything stay put.
 */
export function relaxLabels(boxes: readonly LabelBox[], bounds: Rect, opts: { gap?: number; avoid?: readonly Rect[] } = {}): LabelBox[] {
	const gap = opts.gap ?? 6;
	const out = boxes.map((b) => {
		const c = clampToRect(b.x, b.y, b.w, b.h, bounds, 2);
		return { ...b, ...c };
	});
	const rect = (b: LabelBox): Rect => ({ left: b.x - b.w / 2, right: b.x + b.w / 2, top: b.y - b.h / 2, bottom: b.y + b.h / 2 });
	const grow = (r: Rect): Rect => ({ left: r.left - gap / 2, right: r.right + gap / 2, top: r.top - gap / 2, bottom: r.bottom + gap / 2 });
	const hits = (b: LabelBox, r: Rect) => intersects(grow(rect(b)), grow(r));
	const avoid = opts.avoid ?? [];
	const order = out.map((_, i) => i).sort((i, j) => out[i]!.y - out[j]!.y || out[i]!.x - out[j]!.x);

	// a move that clears `block`: past it vertically (dir +1 below, -1 above), or beside it when shorter and free
	const inX = (x: number, w: number) => x - w / 2 >= bounds.left + 2 && x + w / 2 <= bounds.right - 2;
	const inY = (y: number, h: number) => y - h / 2 >= bounds.top + 2 && y + h / 2 <= bounds.bottom - 2;
	const step = (b: LabelBox, block: Rect, others: Rect[], dir: 1 | -1) => {
		const vert = { x: b.x, y: dir > 0 ? block.bottom + gap + b.h / 2 : block.top - gap - b.h / 2 };
		const side = [block.right + gap + b.w / 2, block.left - gap - b.w / 2]
			.filter((x) => inX(x, b.w))
			.map((x) => ({ x, y: b.y }))
			.filter((c) => !others.some((r) => hits({ ...b, ...c }, r)));
		const cost = (c: { x: number; y: number }) => Math.abs(c.x - b.x) + Math.abs(c.y - b.y) + (inY(c.y, b.h) ? 0 : 1e4);
		return [vert, ...side].sort((m, n) => cost(m) - cost(n))[0]!;
	};
	const sweep = (idx: number[], dir: 1 | -1) => {
		const done: Rect[] = [];
		for (const i of idx) {
			const b = out[i]!;
			if (dir < 0) b.y = Math.min(b.y, bounds.bottom - 2 - b.h / 2);
			for (let guard = 0; guard < 2 * (out.length + avoid.length) + 1; guard++) {
				const others = [...avoid, ...done];
				const block = others.find((r) => hits(b, r));
				if (!block) break;
				Object.assign(b, step(b, block, others, dir));
			}
			done.push(rect(b));
		}
	};
	// 2. downward sweep, 3. upward sweep for whatever left the bounds
	sweep(order, 1);
	if (out.some((b) => !inY(b.y, b.h))) sweep([...order].reverse(), -1);
	// never off the stage, even when there is not room for everything (then the overlap is the lesser evil)
	for (const b of out) Object.assign(b, clampToRect(b.x, b.y, b.w, b.h, bounds, 2));
	return out;
}
