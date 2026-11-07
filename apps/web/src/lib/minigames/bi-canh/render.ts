// Plain canvas-2D rendering of Bí Cảnh (no engine). Visual language: ink-wash (thủy mặc) on a sealed talisman floor —
// a faint trận đồ of concentric rings and the eight trigram ticks, tà khí as smoky ink blots (âm hồn grey-violet
// wisps, hỏa linh cinnabar sparks, thạch quỷ heavy stone-grey blots, tà tướng a large blot with a cinnabar seal), the
// cultivator a small jade-robed figure with a gold aura, the phi kiếm a short gold stroke, linh khí motes cyan dots.
// Colours come from the design tokens in app.css (read once via getComputedStyle). Everything here is display only.
import { SUB, type BiCanhGame } from './sim';

export interface Palette {
	bg: string;
	bgDeep: string;
	line: string;
	ink: string;
	jade: string;
	gold: string;
	goldSoft: string;
	cyan: string;
	violet: string;
	cinnabar: string;
	stone: string;
	mist: string;
}

export function readPalette(el: Element): Palette {
	const cs = getComputedStyle(el);
	const v = (name: string) => cs.getPropertyValue(`--color-${name}`).trim();
	return {
		bg: v('ink-900'),
		bgDeep: v('ink-950'),
		line: v('ink-700'),
		ink: v('ink-600'),
		jade: v('jade-300'),
		gold: v('gold-500'),
		goldSoft: v('gold-300'),
		cyan: v('spirit-cyan'),
		violet: v('spirit-violet'),
		cinnabar: v('cinnabar-300'),
		stone: v('mist-500'),
		mist: v('mist-100')
	};
}

export interface Burst {
	x: number; // css px
	y: number;
	life: number; // 1 → 0
	color: string;
}

export interface Fx {
	alpha: number; // tick interpolation 0..1 (display only)
	bursts: Burst[];
	hurt: number; // 1 → 0 red veil
	reducedMotion: boolean;
	timeMs: number;
}

/** Deterministic 0..1 jitter so blots look hand-drawn but never flicker. */
function jitter(a: number, b: number): number {
	let h = Math.imul((a * 73856093) ^ (b * 19349663), 2654435761) >>> 0;
	h ^= h >>> 15;
	return ((h >>> 0) % 1000) / 1000;
}

/** An irregular ink blot around (x, y). */
function blot(ctx: CanvasRenderingContext2D, id: number, x: number, y: number, r: number, color: string, wobble: number) {
	ctx.fillStyle = color;
	ctx.beginPath();
	const n = 9;
	for (let k = 0; k <= n; k++) {
		const a = (k / n) * Math.PI * 2;
		const rr = r * (0.78 + 0.32 * jitter(id, k) + 0.06 * Math.sin(wobble + k));
		const px = x + Math.cos(a) * rr;
		const py = y + Math.sin(a) * rr;
		if (k === 0) ctx.moveTo(px, py);
		else ctx.lineTo(px, py);
	}
	ctx.closePath();
	ctx.fill();
}

function floor(ctx: CanvasRenderingContext2D, w: number, h: number, pal: Palette) {
	ctx.fillStyle = pal.bgDeep;
	ctx.fillRect(0, 0, w, h);
	const g = ctx.createRadialGradient(w / 2, h / 2, 0, w / 2, h / 2, Math.max(w, h) * 0.7);
	g.addColorStop(0, pal.bg);
	g.addColorStop(1, pal.bgDeep);
	ctx.fillStyle = g;
	ctx.fillRect(0, 0, w, h);
	// Trận đồ: concentric rings and eight ticks (the sealed realm), very faint.
	ctx.save();
	ctx.globalAlpha = 0.35;
	ctx.strokeStyle = pal.line;
	ctx.lineWidth = 1;
	const cx = w / 2;
	const cy = h / 2;
	for (const k of [0.18, 0.32, 0.46]) {
		ctx.beginPath();
		ctx.arc(cx, cy, Math.min(w, h) * k, 0, Math.PI * 2);
		ctx.stroke();
	}
	for (let i = 0; i < 8; i++) {
		const a = (i / 8) * Math.PI * 2;
		const r0 = Math.min(w, h) * 0.46;
		ctx.beginPath();
		ctx.moveTo(cx + Math.cos(a) * r0, cy + Math.sin(a) * r0);
		ctx.lineTo(cx + Math.cos(a) * (r0 + 14), cy + Math.sin(a) * (r0 + 14));
		ctx.stroke();
	}
	ctx.restore();
	// Kết giới edge.
	ctx.strokeStyle = pal.gold;
	ctx.globalAlpha = 0.5;
	ctx.lineWidth = 2;
	ctx.strokeRect(1, 1, w - 2, h - 2);
	ctx.globalAlpha = 1;
}

/** Draws the whole field. scale = css px per arena unit. */
export function drawField(ctx: CanvasRenderingContext2D, g: BiCanhGame, scale: number, pal: Palette, fx: Fx) {
	const p = g.p;
	const w = p.arena.width * scale;
	const h = p.arena.height * scale;
	const k = scale / SUB;
	floor(ctx, w, h, pal);
	const t = fx.reducedMotion ? 0 : fx.timeMs / 300;

	// Linh khí motes.
	for (const m of g.motes) {
		const r = Math.max(2.5, (2 + m.value) * scale * 0.9);
		ctx.globalAlpha = m.life < 60 ? 0.35 + 0.65 * (m.life / 60) : 1;
		ctx.fillStyle = pal.cyan;
		ctx.beginPath();
		ctx.arc(m.x * k, m.y * k, r, 0, Math.PI * 2);
		ctx.fill();
		if (!fx.reducedMotion) {
			ctx.globalAlpha *= 0.3;
			ctx.beginPath();
			ctx.arc(m.x * k, m.y * k, r * 2.2, 0, Math.PI * 2);
			ctx.fill();
		}
	}
	ctx.globalAlpha = 1;

	// Tà khí.
	for (const f of g.foes) {
		const def = p.enemies[f.type]!;
		const r = Math.max(5, def.radius * scale); // readable on a phone; the hitbox is def.radius
		const x = f.x * k;
		const y = f.y * k;
		let color = pal.violet;
		if (def.id === 'hoa_linh') color = pal.cinnabar;
		else if (def.id === 'thach_quy') color = pal.stone;
		else if (def.atTicks?.length) color = pal.ink;
		ctx.globalAlpha = 0.9;
		blot(ctx, f.id, x, y, r, color, t + f.id);
		if (def.atTicks?.length) {
			// Tà tướng: cinnabar seal and a health ring.
			ctx.strokeStyle = pal.cinnabar;
			ctx.lineWidth = 2;
			ctx.strokeRect(x - r * 0.35, y - r * 0.35, r * 0.7, r * 0.7);
			ctx.beginPath();
			ctx.arc(x, y, r + 4, -Math.PI / 2, -Math.PI / 2 + (Math.PI * 2 * f.hp) / def.hp);
			ctx.stroke();
		} else {
			// Two pale eyes so a blot reads as a spirit.
			ctx.fillStyle = pal.mist;
			ctx.globalAlpha = 0.7;
			ctx.fillRect(x - r * 0.35, y - r * 0.15, Math.max(1, r * 0.18), Math.max(1, r * 0.18));
			ctx.fillRect(x + r * 0.17, y - r * 0.15, Math.max(1, r * 0.18), Math.max(1, r * 0.18));
		}
	}
	ctx.globalAlpha = 1;

	// Phi kiếm: a short gold stroke along its flight.
	ctx.strokeStyle = pal.goldSoft;
	ctx.lineCap = 'round';
	ctx.lineWidth = Math.max(2, p.sword.radius * scale * 0.8);
	for (const s of g.shots) {
		const x = s.x * k;
		const y = s.y * k;
		const len = Math.hypot(s.vx, s.vy) || 1;
		const ux = (s.vx / len) * Math.max(6, 9 * scale);
		const uy = (s.vy / len) * Math.max(6, 9 * scale);
		ctx.beginPath();
		ctx.moveTo(x - ux, y - uy);
		ctx.lineTo(x + ux * 0.4, y + uy * 0.4);
		ctx.stroke();
	}

	// Kiếm trận blades.
	for (const [bx, by] of g.blades()) {
		ctx.fillStyle = pal.gold;
		ctx.beginPath();
		ctx.arc(bx * k, by * k, Math.max(3.5, p.orbit.bladeRadius * scale), 0, Math.PI * 2);
		ctx.fill();
	}

	// The cultivator: gold aura, jade robe, ink head.
	const px = g.px * k;
	const py = g.py * k;
	const pr = Math.max(7, p.player.radius * scale);
	const blink = g.invuln > 0 && !fx.reducedMotion && Math.floor(fx.timeMs / 90) % 2 === 0;
	ctx.globalAlpha = blink ? 0.35 : 1;
	const aura = ctx.createRadialGradient(px, py, pr * 0.4, px, py, pr * 2.4);
	aura.addColorStop(0, pal.goldSoft);
	aura.addColorStop(1, 'transparent');
	ctx.fillStyle = aura;
	ctx.globalAlpha *= 0.35;
	ctx.beginPath();
	ctx.arc(px, py, pr * 2.4, 0, Math.PI * 2);
	ctx.fill();
	ctx.globalAlpha = blink ? 0.35 : 1;
	ctx.fillStyle = pal.jade;
	ctx.beginPath();
	ctx.moveTo(px, py - pr * 0.4);
	ctx.lineTo(px + pr, py + pr);
	ctx.lineTo(px - pr, py + pr);
	ctx.closePath();
	ctx.fill();
	ctx.fillStyle = pal.mist;
	ctx.beginPath();
	ctx.arc(px, py - pr * 0.65, pr * 0.42, 0, Math.PI * 2);
	ctx.fill();
	ctx.globalAlpha = 1;

	// Bursts (defeated spirits dissolve into ink).
	for (const b of fx.bursts) {
		ctx.globalAlpha = b.life * 0.6;
		ctx.strokeStyle = b.color;
		ctx.lineWidth = 2;
		ctx.beginPath();
		ctx.arc(b.x, b.y, (1 - b.life) * 16 * scale + 3, 0, Math.PI * 2);
		ctx.stroke();
	}
	ctx.globalAlpha = 1;

	if (fx.hurt > 0) {
		ctx.fillStyle = pal.cinnabar;
		ctx.globalAlpha = fx.hurt * 0.25;
		ctx.fillRect(0, 0, w, h);
		ctx.globalAlpha = 1;
	}
}
