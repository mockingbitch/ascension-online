// Vượt Tâm Ma — deterministic simulator, the TypeScript twin of services/game/internal/trial/tamma/tamma.go.
// It MUST stay bit-identical with Go: integer math only (uint32 via `>>> 0` and Math.imul), xorshift32, the same step
// order and the same FNV-1a state hash. The shared golden vectors in packages/contracts/golden/tam-ma-v2.json are
// checked by both sim.test.ts and the Go tests. The client only renders this and records inputs; the verdict always
// comes from the server's replay.
import type { TamMaParams } from '$lib/game/types';

/** Input codes (wire format of the input log — never renumber). */
export const Act = {
	Left: 1,
	Right: 2,
	RotateCW: 3,
	RotateCC: 4,
	SoftDrop: 5,
	HardDrop: 6,
	Calm: 7
} as const;
export type Action = (typeof Act)[keyof typeof Act];

/** Board value of a risen "tạp niệm" row cell; pieces use 1..pieces.length. */
export const GARBAGE = 15;

export type EndReason = 'cleared' | 'topped_out' | 'time_up' | 'gave_up' | 'ended_early';

export class XorShift32 {
	s: number;
	constructor(seed: number) {
		this.s = seed >>> 0 || 0x9e3779b9;
	}
	next(): number {
		let s = this.s;
		s = (s ^ (s << 13)) >>> 0;
		s = (s ^ (s >>> 17)) >>> 0;
		s = (s ^ (s << 5)) >>> 0;
		this.s = s;
		return s;
	}
	intN(n: number): number {
		return this.next() % n;
	}
}

const RISE_SEED_XOR = 0xa5a5a5a5;

interface Shape {
	cells: [number, number][];
	w: number;
	h: number;
}

function extent(cells: [number, number][]): { w: number; h: number } {
	let w = 0;
	let h = 0;
	for (const [x, y] of cells) {
		w = Math.max(w, x + 1);
		h = Math.max(h, y + 1);
	}
	return { w, h };
}

/** (x, y) → (h − 1 − y, x), keeping cell order. */
function rotateCW(s: Shape): Shape {
	return { w: s.h, h: s.w, cells: s.cells.map(([x, y]) => [s.h - 1 - y, x] as [number, number]) };
}

const KICKS: [number, number][] = [
	[0, 0],
	[-1, 0],
	[1, 0],
	[0, -1],
	[-2, 0],
	[2, 0]
];

/** Integer division truncating toward zero (Go semantics). */
const quo = (a: number, b: number) => (a / b) | 0;

export class TamMaGame {
	readonly p: TamMaParams;
	readonly shapes: Shape[][];
	board: Uint8Array;
	pieceRng: XorShift32;
	riseRng: XorShift32;
	bag: number[] = [];
	bagPos = 0;
	cur = 0;
	rot = 0;
	px = 0;
	py = 0;
	next = 0;
	tick = 0;
	gravCounter = 0;
	riseCounter = 0;
	calmLeft = 0;
	charge = 0;
	lines = 0;
	score = 0;
	locked = 0;
	/** tick of the previous lock (starts at −minLockGapTicks) */
	lastLock: number;
	/** telemetry: smallest gap between two player-driven locks (0 = none yet) */
	minGap = 0;
	over = false;
	won = false;
	reason: EndReason | '' = '';
	/** rows cleared by the last lock (render hint only; not part of the state hash) */
	lastCleared = 0;
	/** ticks at which a tạp niệm row rose (render hint only) */
	lastRiseTick = -1;

	constructor(p: TamMaParams, seed: number) {
		this.p = p;
		this.board = new Uint8Array(p.width * p.height);
		this.pieceRng = new XorShift32(seed);
		this.riseRng = new XorShift32((seed ^ RISE_SEED_XOR) >>> 0);
		this.lastLock = -p.minLockGapTicks;
		this.shapes = p.pieces.map((pc) => {
			const cells = pc.cells.map(([x, y]) => [x, y] as [number, number]);
			const r0: Shape = { cells, ...extent(cells) };
			const rs = [r0];
			for (let r = 1; r < 4; r++) rs.push(rotateCW(rs[r - 1]!));
			return rs;
		});
		this.cur = this.draw();
		this.next = this.draw();
		this.spawn();
	}

	shape(piece = this.cur, rot = this.rot): Shape {
		return this.shapes[piece]![rot]!;
	}

	private draw(): number {
		if (this.bagPos >= this.bag.length) {
			this.bag = [];
			this.p.pieces.forEach((pc, i) => {
				for (let k = 0; k < pc.weight; k++) this.bag.push(i);
			});
			for (let i = this.bag.length - 1; i > 0; i--) {
				const j = this.pieceRng.intN(i + 1);
				const t = this.bag[i]!;
				this.bag[i] = this.bag[j]!;
				this.bag[j] = t;
			}
			this.bagPos = 0;
		}
		return this.bag[this.bagPos++]!;
	}

	collides(piece: number, rot: number, px: number, py: number): boolean {
		const { width: w, height: h } = this.p;
		for (const [cx, cy] of this.shape(piece, rot).cells) {
			const x = px + cx;
			const y = py + cy;
			if (x < 0 || x >= w || y < 0 || y >= h) return true;
			if (this.board[y * w + x] !== 0) return true;
		}
		return false;
	}

	private end(reason: EndReason, won: boolean) {
		this.over = true;
		this.won = won;
		this.reason = reason;
	}

	private spawn() {
		this.rot = 0;
		this.px = quo(this.p.width - this.shape(this.cur, 0).w, 2);
		this.py = 0;
		this.gravCounter = 0;
		if (this.collides(this.cur, 0, this.px, this.py)) this.end('topped_out', false);
	}

	private tryMove(dx: number, dy: number): boolean {
		if (this.collides(this.cur, this.rot, this.px + dx, this.py + dy)) return false;
		this.px += dx;
		this.py += dy;
		return true;
	}

	private tryRotate(dir: number) {
		const nr = (this.rot + dir + 4) % 4;
		const from = this.shape(this.cur, this.rot);
		const to = this.shape(this.cur, nr);
		const bx = this.px + quo(from.w - to.w, 2);
		const by = this.py + quo(from.h - to.h, 2);
		for (const [kx, ky] of KICKS) {
			if (!this.collides(this.cur, nr, bx + kx, by + ky)) {
				this.rot = nr;
				this.px = bx + kx;
				this.py = by + ky;
				return;
			}
		}
	}

	private lock() {
		const { width: w, height: h } = this.p;
		for (const [cx, cy] of this.shape().cells) this.board[(this.py + cy) * w + this.px + cx] = this.cur + 1;
		this.locked++;
		this.lastLock = this.tick;
		let cleared = 0;
		for (let y = h - 1; y >= 0; ) {
			let full = true;
			for (let x = 0; x < w; x++) {
				if (this.board[y * w + x] === 0) {
					full = false;
					break;
				}
			}
			if (!full) {
				y--;
				continue;
			}
			cleared++;
			this.board.copyWithin(w, 0, y * w); // rows 0..y-1 down by one
			this.board.fill(0, 0, w);
		}
		this.lastCleared = cleared;
		if (cleared > 0) {
			this.lines += cleared;
			this.score += this.p.lineScores[Math.min(cleared, this.p.lineScores.length - 1)]!;
			this.charge = Math.min(this.p.calm.chargeMax, this.charge + cleared * this.p.calm.chargePerLine);
		}
		this.cur = this.next;
		this.next = this.draw();
		this.spawn();
	}

	private rise() {
		const { width: w, height: h } = this.p;
		for (let x = 0; x < w; x++) {
			if (this.board[x] !== 0) {
				this.end('topped_out', false);
				return;
			}
		}
		this.board.copyWithin(0, w);
		const gap = this.riseRng.intN(w);
		for (let x = 0; x < w; x++) this.board[(h - 1) * w + x] = x === gap ? 0 : GARBAGE;
		this.lastRiseTick = this.tick;
		if (this.collides(this.cur, this.rot, this.px, this.py)) {
			if (this.collides(this.cur, this.rot, this.px, this.py - 1)) {
				this.end('topped_out', false);
				return;
			}
			this.py--;
		}
	}

	lockAllowed(): boolean {
		return this.tick - this.lastLock >= this.p.minLockGapTicks;
	}

	private playerLock() {
		const gap = this.tick - this.lastLock;
		if (this.locked > 0 && (this.minGap === 0 || gap < this.minGap)) this.minGap = gap;
		this.lock();
	}

	gravityTicks(): number {
		const g = this.p.gravity;
		return Math.max(g.minTicks, g.startTicks - Math.floor(this.tick / g.stepEveryTicks));
	}

	private apply(a: number) {
		switch (a) {
			case Act.Left:
				this.tryMove(-1, 0);
				break;
			case Act.Right:
				this.tryMove(1, 0);
				break;
			case Act.RotateCW:
				this.tryRotate(1);
				break;
			case Act.RotateCC:
				this.tryRotate(-1);
				break;
			case Act.SoftDrop:
				if (this.tryMove(0, 1)) this.gravCounter = 0;
				else if (this.lockAllowed()) this.playerLock();
				break;
			case Act.HardDrop:
				if (this.lockAllowed()) {
					while (this.tryMove(0, 1));
					this.playerLock();
				}
				break;
			case Act.Calm:
				if (this.charge >= this.p.calm.chargeMax && this.calmLeft === 0) {
					this.calmLeft = this.p.calm.durationTicks;
					this.charge = 0;
				}
				break;
		}
	}

	/** One tick: the tick's actions in order, then gravity and rising rows (both frozen while calm). */
	step(actions: readonly number[]) {
		if (this.over) return;
		this.lastCleared = 0;
		for (const a of actions) {
			if (this.over) break;
			this.apply(a);
		}
		if (!this.over) {
			if (this.calmLeft > 0) {
				this.calmLeft--;
			} else {
				this.gravCounter++;
				if (this.gravCounter >= this.gravityTicks()) {
					this.gravCounter = 0;
					if (!this.tryMove(0, 1)) this.lock();
				}
				if (!this.over) {
					this.riseCounter++;
					if (this.riseCounter >= this.p.riseEveryTicks) {
						this.riseCounter = 0;
						this.rise();
					}
				}
			}
		}
		this.tick++;
		if (!this.over && this.lines >= this.p.passLines && this.tick >= this.p.minPassTicks) this.end('cleared', true);
		if (!this.over && this.tick >= this.p.timeLimitTicks) this.end('time_up', false);
	}

	/** Row the falling piece would land on (ghost), display only. */
	ghostY(): number {
		let y = this.py;
		while (!this.collides(this.cur, this.rot, this.px, y + 1)) y++;
		return y;
	}

	/** FNV-1a 32 over the canonical state, each value as 4 little-endian bytes. Same order as Go's Hash. */
	hash(): number {
		let h = 2166136261;
		const put = (v: number) => {
			let u = v >>> 0;
			for (let i = 0; i < 4; i++) {
				h = (h ^ (u & 0xff)) >>> 0;
				h = Math.imul(h, 16777619) >>> 0;
				u >>>= 8;
			}
		};
		const vals = [this.tick, +this.over, +this.won, this.lines, this.score, this.charge, this.calmLeft, this.gravCounter,
			this.riseCounter, this.cur, this.rot, this.px, this.py, this.next, this.bagPos, this.bag.length];
		for (const v of vals) put(v);
		for (const v of this.bag) put(v);
		put(this.pieceRng.s);
		put(this.riseRng.s);
		put(this.locked);
		put(this.lastLock);
		put(this.minGap);
		for (const c of this.board) put(c);
		return h;
	}
}

export type Input = [tick: number, action: number];

/** Largest number of inputs inside any window of `window` consecutive ticks (same two-pointer as Go). */
export function maxWindowInputs(inputs: readonly Input[], window: number): number {
	let best = 0;
	let lo = 0;
	for (let hi = 0; hi < inputs.length; hi++) {
		while (inputs[hi]![0] - inputs[lo]![0] >= window) lo++;
		best = Math.max(best, hi - lo + 1);
	}
	return best;
}

/** How many more inputs may be logged at `tick` without breaking the window cap (client throttle). */
export function windowRoom(p: TamMaParams, inputs: readonly Input[], tick: number): number {
	let n = 0;
	for (let i = inputs.length - 1; i >= 0 && inputs[i]![0] > tick - p.inputWindowTicks; i--) n++;
	return Math.max(0, p.maxInputsPerWindow - n);
}

export interface ReplayResult {
	over: boolean;
	won: boolean;
	reason: EndReason;
	endTick: number;
	lines: number;
	score: number;
	pieces: number;
	stateHash: number;
	minLockGapTicks: number;
}

/** Same contract as Go's tamma.Replay (used by tests; the server is the only judge). */
export function replay(p: TamMaParams, seed: number, inputs: readonly Input[], endTick: number): ReplayResult {
	if (endTick <= 0 || endTick > p.timeLimitTicks) throw new Error('invalid input log: endTick');
	if (inputs.length > p.maxInputs) throw new Error('invalid input log: too many inputs');
	let prev = -1;
	for (const [t, a] of inputs) {
		if (t < 0 || t >= endTick || t < prev) throw new Error('invalid input log: order');
		if (a < Act.Left || a > Act.Calm) throw new Error('invalid input log: action');
		prev = t;
	}
	if (maxWindowInputs(inputs, p.inputWindowTicks) > p.maxInputsPerWindow) throw new Error('implausible input cadence');
	const g = new TamMaGame(p, seed);
	let i = 0;
	while (!g.over && g.tick < endTick) {
		const batch: number[] = [];
		while (i < inputs.length && inputs[i]![0] === g.tick) batch.push(inputs[i++]![1]);
		g.step(batch);
	}
	let reason: EndReason = g.reason || 'gave_up';
	if (!g.over) reason = 'gave_up';
	else if (g.tick < endTick) reason = 'ended_early';
	return {
		over: g.over,
		won: g.won,
		reason,
		endTick: g.tick,
		lines: g.lines,
		score: g.score,
		pieces: g.locked,
		stateHash: g.hash(),
		minLockGapTicks: g.minGap
	};
}
