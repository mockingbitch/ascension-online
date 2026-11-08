// Tử Tiêu Cung (lecture hall) as scene data: the 16:9 hall painting covers the screen anchored centre-top, Hồng
// Quân sits on the dais, 4 rows of cushions fill from the aisle outwards (normalised to the painting). HallView draws
// the sage, seats, bubbles and the qi stream from these anchors; the page only holds UI (subtitles, ngộ đạo, chat).
import type { Anchor, SceneDef } from '../types';

/** Hồng Quân's dais (particles start here), normalised. */
export const HALL_DAIS: Anchor = { x: 0.5, y: 0.24 };

/** Cushion rows, nearest the dais first: y, horizontal extent [from, to] and the sitter scale (normalised). */
export const HALL_ROWS = [
	{ y: 0.43, from: 0.35, to: 0.65, scale: 0.62 },
	{ y: 0.52, from: 0.33, to: 0.67, scale: 0.72 },
	{ y: 0.62, from: 0.31, to: 0.69, scale: 0.83 },
	{ y: 0.72, from: 0.29, to: 0.71, scale: 0.94 }
] as const;

export const HALL_DEFAULT_SEATS = 36;

export interface SeatAnchor extends Anchor {
	seat: number;
	scale: number;
}

/** Seat positions for `count` seats (server `seats`), filled from the centre outwards — same rule as the page. */
export function hallSeats(count: number = HALL_DEFAULT_SEATS): SeatAnchor[] {
	const perRow = Math.ceil(count / HALL_ROWS.length);
	const centreOut = [...Array(perRow).keys()].sort((a, b) => Math.abs(a - (perRow - 1) / 2) - Math.abs(b - (perRow - 1) / 2) || a - b);
	return Array.from({ length: count }, (_, i) => {
		const r = HALL_ROWS[Math.min(HALL_ROWS.length - 1, Math.floor(i / perRow))]!;
		const x = r.from + ((r.to - r.from) * (centreOut[i % perRow]! + 0.5)) / perRow;
		return { seat: i, x, y: r.y, scale: r.scale };
	});
}

export const tuTieuCung: SceneDef = {
	id: 'tu_tieu_cung',
	title: 'Tử Tiêu Cung',
	// .art { width: max(100cqw, 177.78cqh); left: 50%; top: 0 } = cover, focus centre-top
	frame: { aspect: 16 / 9, fit: 'cover', focus: { x: 0.5, y: 0 } },
	layers: [{ id: 'hall', art: 'scenes/tu-tieu-cung', depth: 0 }],
	anchors: {
		dais: HALL_DAIS,
		// the sage image is bottom-aligned at 73% from the bottom of the painting → feet at y = 0.27
		hong_quan: { x: 0.5, y: 0.27 },
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
