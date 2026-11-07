// Plain canvas-2D rendering of Hóa Phàm (no engine). Visual language: ink-wash (thủy mặc). "Trần duyên" knots are
// tangles of red fate-thread, pale lotus fibre or a violet karmic net; "chân tâm" lanterns are warm paper lamps. The
// brush is a tapered calligraphy stroke that dries out, not a glowing blade trail. Colours come from the design
// tokens in app.css (read once via getComputedStyle), never raw values. Everything here is display only.
import { SUB, type HoaPhamGame, type Obj } from './sim';

export interface Palette {
	bg: string;
	bgDeep: string;
	line: string;
	ink: string;
	brush: string;
	brushEdge: string;
	thread: string;
	fibre: string;
	net: string;
	lantern: string;
	lanternCore: string;
	calm: string;
	danger: string;
}

export function readPalette(el: Element): Palette {
	const cs = getComputedStyle(el);
	const v = (name: string) => cs.getPropertyValue(`--color-${name}`).trim();
	return {
		bg: v('ink-900'),
		bgDeep: v('ink-950'),
		line: v('ink-700'),
		ink: v('mist-500'),
		brush: v('mist-100'),
		brushEdge: v('jade-300'),
		thread: v('cinnabar-300'),
		fibre: v('jade-300'),
		net: v('spirit-violet'),
		lantern: v('gold-500'),
		lanternCore: v('gold-300'),
		calm: v('spirit-cyan'),
		danger: v('cinnabar-500')
	};
}

/** Deterministic 0..1 jitter so tangles look hand-drawn but never flicker. */
function jitter(a: number, b: number): number {
	let h = Math.imul((a * 73856093) ^ (b * 19349663), 2654435761) >>> 0;
	h ^= h >>> 15;
	return ((h >>> 0) % 1000) / 1000;
}

/** A tangle of `loops` irregular loops of thread around (0,0), radius r. */
function tangle(ctx: CanvasRenderingContext2D, id: number, r: number, loops: number, color: string, width: number) {
	ctx.strokeStyle = color;
	ctx.lineWidth = width;
	ctx.lineCap = 'round';
	for (let k = 0; k < loops; k++) {
		const a = jitter(id, k) * Math.PI;
		const rx = r * (0.55 + 0.4 * jitter(id, k + 7));
		const ry = r * (0.3 + 0.35 * jitter(id, k + 13));
		ctx.beginPath();
		ctx.ellipse((jitter(id, k + 3) - 0.5) * r * 0.3, (jitter(id, k + 5) - 0.5) * r * 0.3, rx, ry, a, 0, Math.PI * 2);
		ctx.stroke();
	}
	// loose ends
	ctx.beginPath();
	const e = jitter(id, 99) * Math.PI * 2;
	ctx.moveTo(Math.cos(e) * r * 0.7, Math.sin(e) * r * 0.7);
	ctx.quadraticCurveTo(Math.cos(e + 0.5) * r * 1.2, Math.sin(e + 0.5) * r * 1.2, Math.cos(e + 0.2) * r * 1.35, Math.sin(e + 0.2) * r * 1.35);
	ctx.stroke();
}

function net(ctx: CanvasRenderingContext2D, r: number, color: string, torn: boolean, scale: number) {
	ctx.strokeStyle = color;
	ctx.lineWidth = Math.max(1, scale * 0.6);
	ctx.beginPath();
	ctx.arc(0, 0, r, 0, Math.PI * 2);
	ctx.stroke();
	ctx.save();
	ctx.beginPath();
	ctx.arc(0, 0, r, 0, Math.PI * 2);
	ctx.clip();
	ctx.globalAlpha *= 0.8;
	const step = r / 2.5;
	for (let d = -r; d <= r; d += step) {
		if (torn && Math.abs(d) < step) continue; // the first stroke left a gash
		ctx.beginPath();
		ctx.moveTo(d - r, -r);
		ctx.lineTo(d + r, r);
		ctx.moveTo(d + r, -r);
		ctx.lineTo(d - r, r);
		ctx.stroke();
	}
	ctx.restore();
}

function lantern(ctx: CanvasRenderingContext2D, r: number, pal: Palette, flicker: number) {
	const glow = ctx.createRadialGradient(0, 0, r * 0.2, 0, 0, r * 2.1);
	glow.addColorStop(0, pal.lanternCore);
	glow.addColorStop(1, 'transparent');
	ctx.globalAlpha = 0.35 + 0.1 * flicker;
	ctx.fillStyle = glow;
	ctx.beginPath();
	ctx.arc(0, 0, r * 2.1, 0, Math.PI * 2);
	ctx.fill();
	ctx.globalAlpha = 1;
	// paper body: a soft barrel with ribs
	ctx.fillStyle = pal.lantern;
	ctx.beginPath();
	ctx.ellipse(0, 0, r * 0.78, r, 0, 0, Math.PI * 2);
	ctx.fill();
	ctx.strokeStyle = pal.bgDeep;
	ctx.globalAlpha = 0.35;
	ctx.lineWidth = Math.max(1, r * 0.08);
	// ribs: two inner arcs and the centre line
	ctx.beginPath();
	ctx.ellipse(0, 0, r * 0.4, r, 0, 0, Math.PI * 2);
	ctx.moveTo(0, -r);
	ctx.lineTo(0, r);
	ctx.stroke();
	ctx.globalAlpha = 1;
	ctx.fillStyle = pal.lanternCore;
	ctx.beginPath();
	ctx.ellipse(0, r * 0.15, r * 0.22, r * 0.34 + flicker * r * 0.05, 0, 0, Math.PI * 2);
	ctx.fill();
	// caps
	ctx.fillStyle = pal.bgDeep;
	ctx.fillRect(-r * 0.4, -r * 1.05, r * 0.8, r * 0.16);
	ctx.fillRect(-r * 0.4, r * 0.89, r * 0.8, r * 0.16);
}

export interface Piece {
	type: number;
	id: number;
	x: number; // px
	y: number;
	vx: number; // px per ms
	vy: number;
	rot: number;
	half: -1 | 1;
	life: number; // 1 → 0
}

export interface TrailPoint {
	x: number; // px
	y: number;
	age: number; // ms
	stroke: number;
}

export interface FrameFx {
	/** 0..1 interpolation between the last two ticks (display only) */
	alpha: number;
	pieces: Piece[];
	trail: TrailPoint[];
	/** 0..1 cinnabar flash after a lantern was cut */
	hurt: number;
	reducedMotion: boolean;
	timeMs: number;
}

function drawObj(ctx: CanvasRenderingContext2D, g: HoaPhamGame, o: Obj | Piece, x: number, y: number, rot: number, scale: number, pal: Palette, fx: FrameFx, half = 0) {
	const def = g.p.objects[o.type]!;
	const r = def.radius * scale;
	ctx.save();
	ctx.translate(x, y);
	if (def.role === 'lantern') {
		lantern(ctx, r, pal, fx.reducedMotion ? 0 : Math.sin(fx.timeMs / 140 + o.id) * 0.5 + 0.5);
		ctx.restore();
		return;
	}
	ctx.rotate(rot);
	if (half !== 0) {
		ctx.beginPath();
		ctx.rect(-r * 2, half < 0 ? -r * 2 : 0, r * 4, r * 2);
		ctx.clip();
	}
	if (def.id === 'luoi' || def.hp > 1) {
		net(ctx, r, pal.net, 'hp' in o && o.hp < def.hp, scale);
	} else if (def.points >= 2) {
		tangle(ctx, o.id, r, 3, pal.fibre, Math.max(1.2, scale * 0.7));
	} else {
		tangle(ctx, o.id, r, 4, pal.thread, Math.max(1.5, scale * 0.9));
	}
	ctx.restore();
}

export function drawField(ctx: CanvasRenderingContext2D, g: HoaPhamGame, scale: number, pal: Palette, fx: FrameFx) {
	const W = g.p.width * scale;
	const H = g.p.height * scale;
	ctx.save();
	ctx.clearRect(0, 0, W, H);
	const bg = ctx.createLinearGradient(0, 0, 0, H);
	bg.addColorStop(0, pal.bgDeep);
	bg.addColorStop(1, pal.bg);
	ctx.fillStyle = bg;
	ctx.fillRect(0, 0, W, H);
	// Mist bands and a pond line at the bottom: where the knots rise from.
	ctx.globalAlpha = 0.25;
	ctx.strokeStyle = pal.line;
	ctx.lineWidth = 1;
	for (let k = 1; k <= 4; k++) {
		const y = (H * k) / 5 + Math.sin(k * 1.7) * scale * 3;
		ctx.beginPath();
		ctx.moveTo(0, y);
		ctx.bezierCurveTo(W * 0.3, y - scale * 4, W * 0.6, y + scale * 4, W, y);
		ctx.stroke();
	}
	ctx.globalAlpha = 0.6;
	ctx.strokeStyle = pal.ink;
	ctx.beginPath();
	ctx.moveTo(0, H - scale * 2);
	ctx.bezierCurveTo(W * 0.35, H - scale * 5, W * 0.65, H + scale, W, H - scale * 3);
	ctx.stroke();
	ctx.globalAlpha = 1;

	if (g.calmLeft > 0) {
		const v = ctx.createRadialGradient(W / 2, H / 2, Math.min(W, H) * 0.3, W / 2, H / 2, Math.max(W, H) * 0.75);
		v.addColorStop(0, 'transparent');
		v.addColorStop(1, pal.calm);
		ctx.globalAlpha = 0.18;
		ctx.fillStyle = v;
		ctx.fillRect(0, 0, W, H);
		ctx.globalAlpha = 1;
	}

	const a = fx.reducedMotion ? 0 : fx.alpha;
	const k = scale / SUB;
	for (const o of g.objs) {
		const x = (o.x + o.vx * a) * k;
		const y = (o.y + o.vy * a) * k;
		const rot = fx.reducedMotion ? 0 : ((g.tick + a) * 0.02 + o.id) % (Math.PI * 2);
		drawObj(ctx, g, o, x, y, rot, scale, pal, fx);
	}
	for (const pc of fx.pieces) {
		ctx.save();
		ctx.globalAlpha = Math.max(0, pc.life);
		drawObj(ctx, g, pc, pc.x, pc.y, pc.rot, scale, pal, fx, pc.half);
		ctx.restore();
	}

	// Brush: a tapered ink stroke per stroke id, wide where fresh, thin and dry where old.
	const maxAge = fx.reducedMotion ? 120 : 260;
	const pts = fx.trail.filter((t) => t.age < maxAge);
	for (let i = 1; i < pts.length; i++) {
		const p0 = pts[i - 1]!;
		const p1 = pts[i]!;
		if (p0.stroke !== p1.stroke) continue;
		const life = 1 - p1.age / maxAge;
		ctx.globalAlpha = Math.max(0, life) * 0.9;
		ctx.strokeStyle = pal.brushEdge;
		ctx.lineCap = 'round';
		ctx.lineWidth = Math.max(1, scale * 2.6 * life);
		ctx.beginPath();
		ctx.moveTo(p0.x, p0.y);
		ctx.lineTo(p1.x, p1.y);
		ctx.stroke();
		ctx.strokeStyle = pal.brush;
		ctx.lineWidth = Math.max(0.8, scale * 1.2 * life);
		ctx.stroke();
	}
	ctx.globalAlpha = 1;

	if (fx.hurt > 0) {
		ctx.globalAlpha = fx.hurt * (fx.reducedMotion ? 0.25 : 0.4);
		ctx.strokeStyle = pal.danger;
		ctx.lineWidth = scale * 3;
		ctx.strokeRect(0, 0, W, H);
		ctx.globalAlpha = 1;
	}
	ctx.restore();
}
