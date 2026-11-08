// Canvas2D particle system (spec §1.5): one preallocated pool (structure of arrays), no allocation per frame, a hard
// cap per effects quality and devicePixelRatio capped at 2. Driven by the shared loop (engine/loop.ts); display only.
// Two motions share the pool: ballistic (speed/angle/gravity/drag — bursts, petals, smoke) and path (a quadratic
// Bézier from → control → to over a fixed time, optional start delay — qi orbs flying to the HUD, motes converging).

export type Quality = 'full' | 'lite' | 'off';

/** Max live particles per quality (spec: ≤ 300 full, ≤ 80 lite). */
export const PARTICLE_CAP: Record<Quality, number> = { full: 300, lite: 80, off: 0 };
export const MAX_DPR = 2;

/** One emission recipe. Colours are CSS colour strings resolved from design tokens by the caller. */
export interface ParticlePreset {
	id: string;
	/** particles per spawn() at quality full; lite scales by `liteScale` */
	count: number;
	liteScale?: number;
	lifeMs: [number, number];
	/** initial speed px/s */
	speed: [number, number];
	/** emission cone: centre angle and spread (radians) */
	angle: number;
	spread: number;
	size: [number, number];
	/** px/s² (positive = down) */
	gravity: number;
	/** velocity multiplier per second (0..1, 1 = no drag) */
	drag: number;
	colors: string[];
	/** opacity over life: 'fade' = 1 → 0 (default), 'bell' = 0 → 1 → 0 (smoke, petals, orbs) */
	alpha?: 'fade' | 'bell';
	/** 'dot' (default) or 'petal' (a small ellipse turned along its velocity) */
	shape?: 'dot' | 'petal';
	/** size change per second (px/s; smoke grows) */
	grow?: number;
}

/** Path motion: from → (control) → to in `ms`, the i-th particle starting `i × stagger` ms later. */
export interface PathSpec {
	from: { x: number; y: number };
	to: { x: number; y: number };
	/** control point; omitted = an arc above the midpoint */
	ctrl?: { x: number; y: number };
	ms: number;
	stagger?: number;
	/** random offset (px) of each particle's start and control point */
	jitter?: number;
}

const enum Mode {
	Ballistic = 0,
	Path = 1
}
const ALPHA = { fade: 0, bell: 1 } as const;
const SHAPE = { dot: 0, petal: 1 } as const;

export type Rng = () => number;

export class ParticleSystem {
	readonly capacity: number;
	readonly caps: Record<Quality, number>;
	#quality: Quality;
	#n = 0;
	// pool, allocated once
	#x: Float32Array;
	#y: Float32Array;
	#vx: Float32Array;
	#vy: Float32Array;
	#life: Float32Array;
	#maxLife: Float32Array;
	#size: Float32Array;
	#gravity: Float32Array;
	#drag: Float32Array;
	#grow: Float32Array;
	#delay: Float32Array;
	// path: start, control, end
	#x0: Float32Array;
	#y0: Float32Array;
	#cx: Float32Array;
	#cy: Float32Array;
	#x1: Float32Array;
	#y1: Float32Array;
	#mode: Uint8Array;
	#alpha: Uint8Array;
	#shape: Uint8Array;
	#color: string[];
	#rng: Rng;
	#ctx: CanvasRenderingContext2D | null = null;
	#canvas: HTMLCanvasElement | null = null;
	#dpr = 1;
	#cssW = 0;
	#cssH = 0;
	#resolve: (color: string) => string = (c) => c;
	#resolved = new Map<string, string>();

	/** `caps` overrides the per-quality caps (a smaller ambient system for a scene); off is always 0. */
	constructor(quality: Quality = 'full', rng: Rng = Math.random, caps?: { full: number; lite: number }) {
		this.#quality = quality;
		this.#rng = rng;
		this.caps = caps ? { full: Math.min(caps.full, PARTICLE_CAP.full), lite: Math.min(caps.lite, PARTICLE_CAP.lite), off: 0 } : PARTICLE_CAP;
		const n = (this.capacity = this.caps.full);
		const f = () => new Float32Array(n);
		[this.#x, this.#y, this.#vx, this.#vy, this.#life, this.#maxLife, this.#size, this.#gravity, this.#drag, this.#grow, this.#delay] = [f(), f(), f(), f(), f(), f(), f(), f(), f(), f(), f()];
		[this.#x0, this.#y0, this.#cx, this.#cy, this.#x1, this.#y1] = [f(), f(), f(), f(), f(), f()];
		this.#mode = new Uint8Array(n);
		this.#alpha = new Uint8Array(n);
		this.#shape = new Uint8Array(n);
		this.#color = new Array<string>(n).fill('');
	}

	get quality() {
		return this.#quality;
	}
	get cap() {
		return this.caps[this.#quality];
	}
	get alive() {
		return this.#n;
	}
	get dpr() {
		return this.#dpr;
	}

	/** Lowering the quality drops the newest particles above the new cap. */
	setQuality(q: Quality) {
		this.#quality = q;
		if (this.#n > this.cap) this.#n = this.cap;
	}

	/**
	 * Bind a canvas (CSS size in px, device pixel ratio capped at MAX_DPR). `resolve` turns preset colours such as
	 * `var(--color-jade-300)` into canvas colours (design tokens stay the single source); results are cached.
	 */
	attach(canvas: HTMLCanvasElement, cssW: number, cssH: number, dpr: number, resolve?: (color: string) => string) {
		this.#canvas = canvas;
		if (resolve) {
			this.#resolve = resolve;
			this.#resolved.clear();
		}
		this.#ctx = canvas.getContext('2d');
		this.resize(cssW, cssH, dpr);
	}

	resize(cssW: number, cssH: number, dpr: number) {
		this.#dpr = Math.min(MAX_DPR, Math.max(1, dpr || 1));
		this.#cssW = cssW;
		this.#cssH = cssH;
		if (this.#canvas) {
			this.#canvas.width = Math.round(cssW * this.#dpr);
			this.#canvas.height = Math.round(cssH * this.#dpr);
		}
	}

	detach() {
		this.#canvas = null;
		this.#ctx = null;
	}

	/** Emit `preset` at (x, y) in CSS px. Returns how many were actually spawned (cap-limited). */
	spawn(preset: ParticlePreset, x: number, y: number, count?: number): number {
		const n = this.#room(preset, count);
		const r = this.#rng;
		for (let k = 0; k < n; k++) {
			const i = this.#init(preset);
			const a = preset.angle + (r() - 0.5) * preset.spread;
			const s = lerp(r, preset.speed);
			this.#mode[i] = Mode.Ballistic;
			this.#x[i] = x;
			this.#y[i] = y;
			this.#vx[i] = Math.cos(a) * s;
			this.#vy[i] = Math.sin(a) * s;
		}
		return n;
	}

	/**
	 * Emit `preset` particles that travel along a quadratic Bézier (`path`), each lasting `path.ms` after its stagger
	 * delay. Speed/angle/gravity of the preset are ignored; size, colours, alpha curve and shape apply.
	 */
	spawnPath(preset: ParticlePreset, path: PathSpec, count?: number): number {
		const n = this.#room(preset, count);
		const r = this.#rng;
		const j = path.jitter ?? 0;
		const cx = path.ctrl?.x ?? (path.from.x + path.to.x) / 2;
		const cy = path.ctrl?.y ?? Math.min(path.from.y, path.to.y) - Math.abs(path.to.x - path.from.x) * 0.35 - 40;
		for (let k = 0; k < n; k++) {
			const i = this.#init(preset);
			this.#mode[i] = Mode.Path;
			this.#x0[i] = this.#x[i] = path.from.x + (r() - 0.5) * j;
			this.#y0[i] = this.#y[i] = path.from.y + (r() - 0.5) * j;
			this.#cx[i] = cx + (r() - 0.5) * j * 3;
			this.#cy[i] = cy + (r() - 0.5) * j * 2;
			this.#x1[i] = path.to.x;
			this.#y1[i] = path.to.y;
			this.#maxLife[i] = this.#life[i] = path.ms;
			this.#delay[i] = k * (path.stagger ?? 0);
		}
		return n;
	}

	#room(preset: ParticlePreset, count?: number): number {
		const want = count ?? (this.#quality === 'lite' ? Math.ceil(preset.count * (preset.liteScale ?? 0.35)) : preset.count);
		return Math.max(0, Math.min(want, this.cap - this.#n));
	}

	/** Take the next pool slot and set the per-preset fields. */
	#init(preset: ParticlePreset): number {
		const r = this.#rng;
		const i = this.#n++;
		this.#maxLife[i] = this.#life[i] = lerp(r, preset.lifeMs);
		this.#size[i] = lerp(r, preset.size);
		this.#gravity[i] = preset.gravity;
		this.#drag[i] = preset.drag;
		this.#grow[i] = preset.grow ?? 0;
		this.#delay[i] = 0;
		this.#alpha[i] = ALPHA[preset.alpha ?? 'fade'];
		this.#shape[i] = SHAPE[preset.shape ?? 'dot'];
		this.#color[i] = this.#colorOf(preset.colors[Math.floor(r() * preset.colors.length)] ?? preset.colors[0] ?? '');
		return i;
	}

	/** Advance by dt ms; dead particles are swap-removed (no allocation). */
	update(dtMs: number) {
		const dt = dtMs / 1000;
		let i = 0;
		while (i < this.#n) {
			if (this.#delay[i]! > 0) {
				this.#delay[i]! -= dtMs;
				i++;
				continue;
			}
			this.#life[i]! -= dtMs;
			if (this.#life[i]! <= 0) {
				this.#swap(i, --this.#n);
				continue;
			}
			if (this.#size[i]! > 0) this.#size[i] = Math.max(0.1, this.#size[i]! + this.#grow[i]! * dt);
			if (this.#mode[i] === Mode.Path) {
				const t = easeInOut(1 - this.#life[i]! / this.#maxLife[i]!);
				const u = 1 - t;
				this.#x[i] = u * u * this.#x0[i]! + 2 * u * t * this.#cx[i]! + t * t * this.#x1[i]!;
				this.#y[i] = u * u * this.#y0[i]! + 2 * u * t * this.#cy[i]! + t * t * this.#y1[i]!;
			} else {
				const d = Math.pow(this.#drag[i]!, dt);
				this.#vx[i]! *= d;
				this.#vy[i] = this.#vy[i]! * d + this.#gravity[i]! * dt;
				this.#x[i]! += this.#vx[i]! * dt;
				this.#y[i]! += this.#vy[i]! * dt;
			}
			i++;
		}
	}

	render() {
		const ctx = this.#ctx;
		if (!ctx) return;
		ctx.setTransform(this.#dpr, 0, 0, this.#dpr, 0, 0);
		ctx.clearRect(0, 0, this.#cssW, this.#cssH);
		ctx.globalCompositeOperation = 'lighter';
		for (let i = 0; i < this.#n; i++) {
			if (this.#delay[i]! > 0) continue;
			const left = this.#life[i]! / this.#maxLife[i]!;
			ctx.globalAlpha = this.#alpha[i] === ALPHA.bell ? Math.min(1, (1 - left) * 5, left * 3) : Math.max(0, left);
			ctx.fillStyle = this.#color[i]!;
			ctx.beginPath();
			if (this.#shape[i] === SHAPE.petal) {
				const s = this.#size[i]!;
				ctx.ellipse(this.#x[i]!, this.#y[i]!, s, s * 0.5, Math.atan2(this.#vy[i]!, this.#vx[i]!) + left * 6, 0, Math.PI * 2);
			} else {
				ctx.arc(this.#x[i]!, this.#y[i]!, this.#size[i]!, 0, Math.PI * 2);
			}
			ctx.fill();
		}
		ctx.globalAlpha = 1;
		ctx.globalCompositeOperation = 'source-over';
	}

	/** Drop every particle and wipe the canvas (scene hidden, effects turned off). */
	clear() {
		this.#n = 0;
		this.#ctx?.setTransform(1, 0, 0, 1, 0, 0);
		this.#ctx?.clearRect(0, 0, this.#canvas?.width ?? 0, this.#canvas?.height ?? 0);
	}

	#colorOf(c: string): string {
		let v = this.#resolved.get(c);
		if (v === undefined) this.#resolved.set(c, (v = this.#resolve(c)));
		return v;
	}

	#swap(a: number, b: number) {
		if (a === b) return;
		this.#x[a] = this.#x[b]!;
		this.#y[a] = this.#y[b]!;
		this.#vx[a] = this.#vx[b]!;
		this.#vy[a] = this.#vy[b]!;
		this.#life[a] = this.#life[b]!;
		this.#maxLife[a] = this.#maxLife[b]!;
		this.#size[a] = this.#size[b]!;
		this.#gravity[a] = this.#gravity[b]!;
		this.#drag[a] = this.#drag[b]!;
		this.#grow[a] = this.#grow[b]!;
		this.#delay[a] = this.#delay[b]!;
		this.#x0[a] = this.#x0[b]!;
		this.#y0[a] = this.#y0[b]!;
		this.#cx[a] = this.#cx[b]!;
		this.#cy[a] = this.#cy[b]!;
		this.#x1[a] = this.#x1[b]!;
		this.#y1[a] = this.#y1[b]!;
		this.#mode[a] = this.#mode[b]!;
		this.#alpha[a] = this.#alpha[b]!;
		this.#shape[a] = this.#shape[b]!;
		this.#color[a] = this.#color[b]!;
	}
}

const lerp = (r: Rng, a: [number, number]) => a[0] + (a[1] - a[0]) * r();
const easeInOut = (t: number) => (t < 0.5 ? 2 * t * t : 1 - (-2 * t + 2) ** 2 / 2);
