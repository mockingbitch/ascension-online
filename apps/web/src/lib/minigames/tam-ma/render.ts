// Plain canvas-2D rendering of Vượt Tâm Ma (no engine). Visual language: ink-wash (thủy mặc) — every "tạp niệm"
// is a soft ink blot, blots of one piece bleed into each other, risen rows are grey smoke. Colours come from the
// design tokens in app.css (read once via getComputedStyle), never raw values.
import { GARBAGE, type TamMaGame } from './sim';

export interface Palette {
	bg: string;
	bgDeep: string;
	line: string;
	text: string;
	dim: string;
	garbage: string;
	garbageEdge: string;
	calm: string;
	danger: string;
	pieces: string[];
}

export function readPalette(el: Element): Palette {
	const cs = getComputedStyle(el);
	const v = (name: string) => cs.getPropertyValue(`--color-${name}`).trim();
	return {
		bg: v('ink-900'),
		bgDeep: v('ink-950'),
		line: v('ink-700'),
		text: v('mist-100'),
		dim: v('mist-500'),
		garbage: v('ink-600'),
		garbageEdge: v('mist-500'),
		calm: v('spirit-cyan'),
		danger: v('cinnabar-500'),
		// vọng, chấp, tham, sân, si, mạn — then repeat
		pieces: [v('jade-300'), v('spirit-cyan'), v('gold-300'), v('cinnabar-300'), v('spirit-violet'), v('mist-300'), v('jade-500'), v('gold-500')]
	};
}

/** Deterministic 0..1 jitter per cell so blots look hand-inked but never flicker. */
function jitter(x: number, y: number, k: number): number {
	let h = Math.imul(x * 73856093 ^ y * 19349663 ^ k * 83492791, 2654435761) >>> 0;
	h ^= h >>> 15;
	return (h % 1000) / 1000;
}

function blot(ctx: CanvasRenderingContext2D, cx: number, cy: number, r: number, color: string, alpha: number, seedX: number, seedY: number) {
	ctx.save();
	ctx.globalAlpha = alpha;
	const g = ctx.createRadialGradient(cx - r * 0.25, cy - r * 0.3, r * 0.1, cx, cy, r * 1.05);
	g.addColorStop(0, color);
	g.addColorStop(0.75, color);
	g.addColorStop(1, 'transparent');
	ctx.fillStyle = g;
	ctx.beginPath();
	// 7-lobed irregular circle: the "ink" edge.
	const lobes = 7;
	for (let i = 0; i <= lobes; i++) {
		const a = (i / lobes) * Math.PI * 2;
		const rr = r * (0.9 + 0.12 * jitter(seedX, seedY, i));
		const px = cx + Math.cos(a) * rr;
		const py = cy + Math.sin(a) * rr;
		if (i === 0) ctx.moveTo(px, py);
		else ctx.quadraticCurveTo(cx + Math.cos(a - Math.PI / lobes) * rr * 1.08, cy + Math.sin(a - Math.PI / lobes) * rr * 1.08, px, py);
	}
	ctx.fill();
	ctx.restore();
}

/** Bridge between two orthogonal neighbours of the same piece, so a piece reads as one ink stroke. */
function bridge(ctx: CanvasRenderingContext2D, x0: number, y0: number, x1: number, y1: number, cell: number, color: string, alpha: number) {
	ctx.save();
	ctx.globalAlpha = alpha * 0.9;
	ctx.fillStyle = color;
	const t = cell * 0.56;
	const ax = (x0 + 0.5) * cell;
	const ay = (y0 + 0.5) * cell;
	const bx = (x1 + 0.5) * cell;
	const by = (y1 + 0.5) * cell;
	if (ay === by) ctx.fillRect(Math.min(ax, bx), ay - t / 2, Math.abs(bx - ax), t);
	else ctx.fillRect(ax - t / 2, Math.min(ay, by), t, Math.abs(by - ay));
	ctx.restore();
}

function drawCells(ctx: CanvasRenderingContext2D, cells: [number, number][], cell: number, color: string, alpha: number, key: number) {
	const set = new Set(cells.map(([x, y]) => x * 64 + y));
	for (const [x, y] of cells) {
		if (set.has((x + 1) * 64 + y)) bridge(ctx, x, y, x + 1, y, cell, color, alpha);
		if (set.has(x * 64 + y + 1)) bridge(ctx, x, y, x, y + 1, cell, color, alpha);
	}
	for (const [x, y] of cells) blot(ctx, (x + 0.5) * cell, (y + 0.5) * cell, cell * 0.47, color, alpha, x + key * 13, y);
}

export interface FrameFx {
	/** 0..1 flash after a line clear (0 = none) */
	clearFlash: number;
	/** 0..1 shake after a rising row (0 = none) */
	rise: number;
	reducedMotion: boolean;
}

export function drawBoard(ctx: CanvasRenderingContext2D, g: TamMaGame, cell: number, pal: Palette, fx: FrameFx) {
	const { width: w, height: h } = g.p;
	const W = w * cell;
	const H = h * cell;
	ctx.save();
	ctx.clearRect(0, 0, W, H);
	if (!fx.reducedMotion && fx.rise > 0) ctx.translate(0, Math.round(fx.rise * cell * 0.15));

	// Paper: deep ink with a faint lighter centre and vertical brush lines.
	const bg = ctx.createLinearGradient(0, 0, 0, H);
	bg.addColorStop(0, pal.bgDeep);
	bg.addColorStop(1, pal.bg);
	ctx.fillStyle = bg;
	ctx.fillRect(0, 0, W, H);
	ctx.globalAlpha = 0.35;
	ctx.strokeStyle = pal.line;
	ctx.lineWidth = 1;
	for (let x = 1; x < w; x++) {
		ctx.beginPath();
		ctx.moveTo(x * cell + 0.5, 0);
		ctx.lineTo(x * cell + 0.5, H);
		ctx.stroke();
	}
	ctx.globalAlpha = 1;

	// Danger zone: the top two rows tint cinnabar when the stack reaches them.
	let topFilled = h;
	for (let i = 0; i < g.board.length; i++) {
		if (g.board[i] !== 0) {
			topFilled = Math.floor(i / w);
			break;
		}
	}
	if (topFilled <= 3) {
		const dz = ctx.createLinearGradient(0, 0, 0, cell * 3);
		dz.addColorStop(0, pal.danger);
		dz.addColorStop(1, 'transparent');
		ctx.globalAlpha = 0.28;
		ctx.fillStyle = dz;
		ctx.fillRect(0, 0, W, cell * 3);
		ctx.globalAlpha = 1;
	}

	// Locked cells, grouped by value (bridges only between equal neighbours).
	for (let y = 0; y < h; y++) {
		for (let x = 0; x < w; x++) {
			const v = g.board[y * w + x]!;
			if (v === 0) continue;
			if (v === GARBAGE) {
				ctx.save();
				ctx.globalAlpha = 0.85;
				ctx.fillStyle = pal.garbage;
				const r = cell * 0.42;
				ctx.beginPath();
				ctx.arc((x + 0.5) * cell, (y + 0.5) * cell, r, 0, Math.PI * 2);
				ctx.fill();
				ctx.globalAlpha = 0.35;
				ctx.strokeStyle = pal.garbageEdge;
				ctx.setLineDash([2, 3]);
				ctx.stroke();
				ctx.restore();
				continue;
			}
			const color = pal.pieces[(v - 1) % pal.pieces.length]!;
			if (x + 1 < w && g.board[y * w + x + 1] === v) bridge(ctx, x, y, x + 1, y, cell, color, 0.62);
			if (y + 1 < h && g.board[(y + 1) * w + x] === v) bridge(ctx, x, y, x, y + 1, cell, color, 0.62);
		}
	}
	for (let y = 0; y < h; y++) {
		for (let x = 0; x < w; x++) {
			const v = g.board[y * w + x]!;
			if (v === 0 || v === GARBAGE) continue;
			blot(ctx, (x + 0.5) * cell, (y + 0.5) * cell, cell * 0.47, pal.pieces[(v - 1) % pal.pieces.length]!, 0.72, x, y);
		}
	}

	if (!g.over) {
		const shape = g.shape();
		const color = pal.pieces[g.cur % pal.pieces.length]!;
		// Ghost: where the blot will land (dotted outline).
		const gy = g.ghostY();
		if (gy > g.py) {
			ctx.save();
			ctx.globalAlpha = 0.4;
			ctx.strokeStyle = color;
			ctx.setLineDash([3, 4]);
			ctx.lineWidth = 1.5;
			for (const [cx, cy] of shape.cells) {
				ctx.beginPath();
				ctx.arc((g.px + cx + 0.5) * cell, (gy + cy + 0.5) * cell, cell * 0.38, 0, Math.PI * 2);
				ctx.stroke();
			}
			ctx.restore();
		}
		ctx.save();
		ctx.shadowColor = color;
		ctx.shadowBlur = fx.reducedMotion ? 0 : cell * 0.5;
		ctx.translate(g.px * cell, g.py * cell);
		drawCells(ctx, shape.cells, cell, color, 0.95, g.cur);
		ctx.restore();
	}

	// Calm: a cyan veil over the board while gravity is frozen.
	if (g.calmLeft > 0) {
		ctx.save();
		ctx.globalAlpha = 0.12 + (fx.reducedMotion ? 0 : 0.06 * Math.sin(g.tick / 6));
		ctx.fillStyle = pal.calm;
		ctx.fillRect(0, 0, W, H);
		ctx.restore();
	}
	if (fx.clearFlash > 0 && !fx.reducedMotion) {
		ctx.save();
		ctx.globalAlpha = 0.25 * fx.clearFlash;
		ctx.fillStyle = pal.text;
		ctx.fillRect(0, 0, W, H);
		ctx.restore();
	}
	ctx.restore();
}

/** Small preview of a piece (the "next" blot), centred in a size×size box. */
export function drawPiece(ctx: CanvasRenderingContext2D, g: TamMaGame, piece: number, size: number, pal: Palette) {
	ctx.clearRect(0, 0, size, size);
	const s = g.shape(piece, 0);
	const cell = Math.floor(size / Math.max(3, s.w, s.h + 0.5));
	ctx.save();
	ctx.translate(Math.floor((size - s.w * cell) / 2), Math.floor((size - s.h * cell) / 2));
	drawCells(ctx, s.cells, cell, pal.pieces[piece % pal.pieces.length]!, 0.9, piece);
	ctx.restore();
}
