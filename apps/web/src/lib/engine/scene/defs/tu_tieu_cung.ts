// Tử Tiêu Cung (lecture hall) as scene data: the 16:9 hall painting covers the screen anchored centre-top, Hồng
// Quân sits on the dais, 6 rows of cushions (one điện = 60 by default, decisions 2026-11-10) fill from the aisle
// outwards, nearest the dais first; rows widen and sitters grow toward the viewer (depth scaling), normalised to the
// painting. HallView draws the sage, seats, bubbles and the qi stream from these anchors; the page only holds UI.
import type { Anchor, SceneDef } from '../types';

/** Hồng Quân's dais (particles start here), normalised. */
export const HALL_DAIS: Anchor = { x: 0.5, y: 0.24 };

/**
 * Cushion rows, nearest the dais first: y, horizontal extent [from, to], the sitter scale (depth: nearer the viewer =
 * bigger) and the row's share of the cushions (`weight` = its count at 60). Rows recede by perspective (gaps shrink
 * toward the dais) and stay inside the pillar lines of the painting.
 */
export const HALL_ROWS = [
	{ y: 0.405, from: 0.37, to: 0.63, scale: 0.54, weight: 8 },
	{ y: 0.47, from: 0.355, to: 0.645, scale: 0.61, weight: 9 },
	{ y: 0.54, from: 0.34, to: 0.66, scale: 0.69, weight: 10 },
	{ y: 0.615, from: 0.325, to: 0.675, scale: 0.77, weight: 10 },
	{ y: 0.695, from: 0.31, to: 0.69, scale: 0.86, weight: 11 },
	{ y: 0.78, from: 0.295, to: 0.705, scale: 0.95, weight: 12 }
] as const;

/** Cushions per điện (lectures.yaml hall.seats; the server's `capacity` wins when it differs). */
export const HALL_DEFAULT_SEATS = 60;

export interface SeatAnchor extends Anchor {
	seat: number;
	scale: number;
	row: number;
}

/** How many of `count` cushions each row holds: proportional to the row weights (largest remainder). Pure. */
export function rowCounts(count: number): number[] {
	const total = HALL_ROWS.reduce((a, r) => a + r.weight, 0);
	const exact = HALL_ROWS.map((r) => (count * r.weight) / total);
	const out = exact.map(Math.floor);
	const order = exact.map((e, i) => [e - Math.floor(e), i] as const).sort((a, b) => b[0] - a[0] || a[1] - b[1]);
	for (let k = 0, left = count - out.reduce((a, b) => a + b, 0); left > 0; k++, left--) out[order[k % order.length]![1]]! += 1;
	return out;
}

/**
 * Seat positions for `count` cushions (server `capacity`): row by row from the dais, each row filled from the centre
 * outwards, so the lowest seat numbers (the server seats newcomers on the lowest free one) sit centre-front.
 */
export function hallSeats(count: number = HALL_DEFAULT_SEATS, fit?: SeatFit | null): SeatAnchor[] {
	const counts = rowCounts(Math.max(0, Math.floor(count)));
	const out: SeatAnchor[] = [];
	const f = fitTransform(fit);
	counts.forEach((n, ri) => {
		const r = HALL_ROWS[ri]!;
		const centreOut = [...Array(n).keys()].sort((a, b) => Math.abs(a - (n - 1) / 2) - Math.abs(b - (n - 1) / 2) || a - b);
		for (const c of centreOut) {
			const x = r.from + ((r.to - r.from) * (c + 0.5)) / n;
			out.push({ seat: out.length, x: 0.5 + (x - 0.5) * f.kx, y: f.top + (r.y - FIRST_Y) * f.ky, scale: r.scale * f.ks, row: ri });
		}
	});
	return out;
}

/**
 * Where the seat block must fit when part of the painting is hidden (short landscape screens crop the bottom rows and
 * the subtitles / chat dock sit over the floor) — normalised painting coordinates of the free area. The view measures
 * it; missing sides are unconstrained.
 */
export interface SeatFit {
	/** highest allowed feet y of the first row (below the header overlay) */
	top?: number;
	/** lowest allowed feet y */
	bottom?: number;
	/** leftmost / rightmost allowed cushion centre x */
	left?: number;
	right?: number;
}

const FIRST_Y: number = HALL_ROWS[0].y;
const LAST_Y: number = HALL_ROWS[HALL_ROWS.length - 1]!.y;
const MIN_X = Math.min(...HALL_ROWS.map((r) => r.from));
const MAX_X = Math.max(...HALL_ROWS.map((r) => r.to));
/** the first row may move up to here (still clear of the dais and Hồng Quân) when the block must shrink */
const FIT_TOP = 0.34;

/**
 * The squeeze for a fit (pure): rows keep their order and spacing ratios but span [top, bottom]; columns move toward
 * the aisle; sitters shrink with the tighter of the two (never below 55 %). No fit (or room enough) = identity.
 */
export function fitTransform(fit?: SeatFit | null): { top: number; ky: number; kx: number; ks: number; room: number } {
	const bottom = Math.min(LAST_Y, fit?.bottom ?? LAST_Y);
	let top = FIRST_Y;
	if (bottom < LAST_Y) top = Math.min(FIRST_Y, Math.max(FIT_TOP, bottom - (LAST_Y - FIRST_Y)));
	if (fit?.top !== undefined) top = Math.max(top, Math.min(fit.top, bottom - 0.05));
	const kyRaw = Math.min(1, (bottom - top) / (LAST_Y - FIRST_Y));
	let kxRaw = 1;
	if (fit?.left !== undefined && fit.left > MIN_X) kxRaw = Math.min(kxRaw, (0.5 - fit.left) / (0.5 - MIN_X));
	if (fit?.right !== undefined && fit.right < MAX_X) kxRaw = Math.min(kxRaw, (fit.right - 0.5) / (MAX_X - 0.5));
	const ky = Math.max(0.3, kyRaw);
	const kx = Math.max(0.3, kxRaw);
	const ks = Math.max(0.55, Math.min(1, ky, kx));
	// room: how much of the block's natural size the fit leaves (unclamped; compare two fits by it)
	return { top, ky, kx, ks, room: Math.min(kyRaw, kxRaw) };
}

export const tuTieuCung: SceneDef = {
	id: 'tu_tieu_cung',
	title: 'Tử Tiêu Cung',
	// .art { width: max(100cqw, 177.78cqh); left: 50%; top: 0 } = cover, focus centre-top
	frame: { aspect: 16 / 9, fit: 'cover', focus: { x: 0.5, y: 0 } },
	layers: [{ id: 'hall', art: 'scenes/tu-tieu-cung', depth: 0 }],
	anchors: {
		// z = height above the floor for the 3D renderer (render3d/hall.ts: dais top 0.9; the qi source sits on the sage's
		// lap at the same depth); 2D ignores it
		dais: { ...HALL_DAIS, z: 1.46 },
		// the sage image is bottom-aligned at 73% from the bottom of the painting → feet at y = 0.27
		hong_quan: { x: 0.5, y: 0.27, z: 0.9 },
		subtitle: { x: 0.5, y: 0.86 },
		...Object.fromEntries(hallSeats().map((s) => [`seat.${s.seat}`, { x: s.x, y: s.y }]))
	},
	hotspots: [{ id: 'notebook', anchor: 'dais', label: 'Sổ Đạo Ngộ', radius: 0.08, action: { kind: 'window', window: 'notebook' } }],
	camera: [{ id: 'hall', x: 0.5, y: 0.5, zoom: 1 }],
	ambient: [
		{ kind: 'light_rays', anchor: 'dais' },
		{ kind: 'qi_motes', anchor: 'dais', density: 9 }
	],
	music: 'bgm.tu_tieu_cung',
	ambience: 'amb.hall'
};
