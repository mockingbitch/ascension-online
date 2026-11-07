// Hóa Phàm — deterministic simulator, the TypeScript twin of services/game/internal/trial/hoapham/hoapham.go.
// It MUST stay bit-identical with Go: integer math only (every product stays below 2^53 by the loader's bounds;
// divisions truncate like Go), xorshift32, the same step order and the same FNV-1a state hash. The shared golden
// vectors in packages/contracts/golden/hoa-pham-v1.json are checked by both sim.test.ts and the Go tests. The client
// only renders this and records strokes; the verdict always comes from the server's replay.
import type { HoaPhamParams } from '$lib/game/types';

/** Physics sub-units per grid unit (rules constant). */
export const SUB = 32;

export type EndReason = 'cleared' | 'lanterns_out' | 'time_up' | 'gave_up' | 'ended_early';

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

/** Integer division truncating toward zero (Go semantics). */
const quo = (a: number, b: number) => Math.trunc(a / b);

/** Octile step length: max(|dx|,|dy|) + min(|dx|,|dy|)/2. */
export function stepLen(dx: number, dy: number): number {
	let a = Math.abs(dx);
	let b = Math.abs(dy);
	if (a < b) [a, b] = [b, a];
	return a + quo(b, 2);
}

/** Move from p toward t by at most `budget` (octile); never overshoots. Same as Go's Toward. */
export function toward(px: number, py: number, tx: number, ty: number, budget: number): [number, number] {
	const dx = tx - px;
	const dy = ty - py;
	const l = stepLen(dx, dy);
	if (l <= budget) return [tx, ty];
	if (budget <= 0) return [px, py];
	return [px + quo(dx * budget, l), py + quo(dy * budget, l)];
}

function segHits(ax: number, ay: number, bx: number, by: number, cx: number, cy: number, r: number): boolean {
	const dx = bx - ax;
	const dy = by - ay;
	const wx = cx - ax;
	const wy = cy - ay;
	const t = wx * dx + wy * dy;
	const r2 = r * r;
	if (t <= 0) return wx * wx + wy * wy <= r2;
	const l2 = dx * dx + dy * dy;
	if (t >= l2) {
		const ex = cx - bx;
		const ey = cy - by;
		return ex * ex + ey * ey <= r2;
	}
	const cr = dx * wy - dy * wx;
	return cr * cr <= r2 * l2;
}

export interface Obj {
	id: number;
	type: number;
	x: number;
	y: number;
	vx: number;
	vy: number;
	hp: number;
	lastStroke: number;
}

/** Render hints only (not part of the state hash): what happened during the last step. */
export interface StepEvents {
	cut: { obj: Obj; dx: number; dy: number }[];
	hit: Obj[]; // a net torn but not severed
	lantern: Obj[];
	escaped: number;
}

export class HoaPhamGame {
	readonly p: HoaPhamParams;
	rng: XorShift32;
	objs: Obj[] = [];
	tick = 0;
	spawnCounter: number;
	nextId = 1;
	spawned = 0;
	weightSum = 0;
	score = 0;
	cuts = 0;
	escaped = 0;
	lanternsCut = 0;
	lanternsLeft: number;
	charge = 0;
	calmLeft = 0;
	pressed = false;
	px = 0;
	py = 0;
	strokeId = 0;
	strokeCuts = 0;
	bestCombo = 0;
	over = false;
	won = false;
	reason: EndReason | '' = '';
	events: StepEvents = { cut: [], hit: [], lantern: [], escaped: 0 };

	constructor(p: HoaPhamParams, seed: number) {
		this.p = p;
		this.rng = new XorShift32(seed);
		this.lanternsLeft = p.lanterns;
		for (const o of p.objects) this.weightSum += o.weight;
		this.spawnCounter = Math.max(0, p.spawn.startTicks - 12);
	}

	private end(reason: EndReason, won: boolean) {
		this.over = true;
		this.won = won;
		this.reason = reason;
	}

	spawnInterval(): number {
		const s = this.p.spawn;
		return Math.max(s.minTicks, s.startTicks - Math.floor(this.tick / s.stepEveryTicks));
	}

	private spawnWave() {
		const s = this.p.spawn;
		const p = this.p;
		const n = 1 + this.rng.intN(Math.min(s.waveMax, 1 + Math.floor(this.tick / s.waveGrowEveryTicks)));
		for (let k = 0; k < n; k++) {
			let pick = this.rng.intN(this.weightSum);
			let t = 0;
			for (let i = 0; i < p.objects.length; i++) {
				const w = p.objects[i]!.weight;
				if (pick < w) {
					t = i;
					break;
				}
				pick -= w;
			}
			const def = p.objects[t]!;
			const m = def.role === 'lantern' ? p.lanternMotion : p.knotMotion;
			const xu = s.margin + this.rng.intN(p.width - 2 * s.margin + 1);
			const vy = m.vySub.min + this.rng.intN(m.vySub.max - m.vySub.min + 1);
			let vx = this.rng.intN(m.vxMaxSub + 1);
			const side = this.rng.intN(2);
			if (xu * 3 < p.width) {
				// drift right
			} else if (xu * 3 > 2 * p.width) vx = -vx;
			else if (side === 1) vx = -vx;
			this.objs.push({ id: this.nextId, type: t, x: xu * SUB, y: (p.height + def.radius) * SUB, vx, vy: -vy, hp: def.hp, lastStroke: 0 });
			this.nextId++;
			this.spawned++;
		}
	}

	private cut(x: number, y: number) {
		const p = this.p;
		if (stepLen(x - this.px, y - this.py) < p.minCutStep) return;
		const ax = this.px * SUB;
		const ay = this.py * SUB;
		const bx = x * SUB;
		const by = y * SUB;
		const keep: Obj[] = [];
		for (const o of this.objs) {
			if (this.over || o.lastStroke === this.strokeId) {
				keep.push(o);
				continue;
			}
			const def = p.objects[o.type]!;
			if (!segHits(ax, ay, bx, by, o.x, o.y, def.radius * SUB)) {
				keep.push(o);
				continue;
			}
			o.lastStroke = this.strokeId;
			if (def.role === 'lantern') {
				this.lanternsCut++;
				this.lanternsLeft--;
				this.strokeCuts = 0;
				if (this.calmLeft === 0) this.charge = 0;
				this.events.lantern.push(o);
				if (this.lanternsLeft <= 0) this.end('lanterns_out', false);
				continue;
			}
			o.hp--;
			if (o.hp > 0) {
				this.events.hit.push(o);
				keep.push(o);
				continue;
			}
			this.strokeCuts++;
			this.cuts++;
			this.bestCombo = Math.max(this.bestCombo, this.strokeCuts);
			this.score += def.points + Math.min(p.combo.maxBonus, (this.strokeCuts - 1) * p.combo.stepBonus);
			this.events.cut.push({ obj: o, dx: x - this.px, dy: y - this.py });
			if (this.calmLeft === 0) {
				this.charge += p.calm.chargePerCut;
				if (this.charge >= p.calm.chargeMax) {
					this.charge = 0;
					this.calmLeft = p.calm.durationTicks;
				}
			}
		}
		this.objs = keep;
	}

	private world() {
		const p = this.p;
		const w = p.width * SUB;
		const h = p.height * SUB;
		const keep: Obj[] = [];
		for (const o of this.objs) {
			const def = p.objects[o.type]!;
			const r = def.radius * SUB;
			o.x += o.vx;
			o.y += o.vy;
			if (def.role === 'knot') o.vy += p.gravitySub;
			const gone = o.x + r < 0 || o.x - r > w || (o.vy > 0 && o.y - r > h) || (def.role === 'lantern' && o.y + r < 0);
			if (!gone) {
				keep.push(o);
				continue;
			}
			if (def.role === 'knot') {
				this.escaped++;
				this.events.escaped++;
				if (this.calmLeft === 0) this.charge = 0;
			}
		}
		this.objs = keep;
		this.spawnCounter++;
		if (this.spawnCounter >= this.spawnInterval()) {
			this.spawnCounter = 0;
			this.spawnWave();
		}
	}

	/** One tick. down = pointer pressed this tick at (x, y) (grid units, inside the field). Same order as Go. */
	step(down: boolean, x = 0, y = 0) {
		if (this.over) return;
		this.events = { cut: [], hit: [], lantern: [], escaped: 0 };
		if (down) {
			if (this.pressed) this.cut(x, y);
			else {
				this.strokeId++;
				this.strokeCuts = 0;
			}
			this.px = x;
			this.py = y;
		}
		this.pressed = down;
		if (!this.over) {
			if (this.calmLeft === 0 || this.calmLeft % 2 === 0) this.world();
			if (this.calmLeft > 0) this.calmLeft--;
		}
		this.tick++;
		if (!this.over && this.score >= this.p.passScore && this.tick >= this.p.minPassTicks) this.end('cleared', true);
		if (!this.over && this.tick >= this.p.timeLimitTicks) this.end('time_up', false);
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
		const vals = [this.tick, +this.over, +this.won, this.score, this.cuts, this.escaped, this.lanternsCut, this.lanternsLeft, this.charge,
			this.calmLeft, this.spawnCounter, this.nextId, this.spawned, +this.pressed, this.px, this.py, this.strokeId, this.strokeCuts,
			this.bestCombo, this.rng.s, this.objs.length];
		for (const v of vals) put(v);
		for (const o of this.objs) {
			put(o.id);
			put(o.type);
			put(o.x);
			put(o.y);
			put(o.vx);
			put(o.vy);
			put(o.hp);
			put(o.lastStroke);
		}
		return h;
	}
}

/** One stroke on the wire: [start, x0, y0, dx1, dy1, …]. */
export type Stroke = number[];

const strokeEnd = (s: Stroke) => s[0]! + (s.length - 3) / 2;

/** Largest sum of v over events inside any window of `window` consecutive ticks (same two-pointer as Go). */
export function maxWindowSum(ev: readonly [tick: number, v: number][], window: number): number {
	let best = 0;
	let sum = 0;
	let lo = 0;
	for (let hi = 0; hi < ev.length; hi++) {
		sum += ev[hi]![1];
		while (ev[hi]![0] - ev[lo]![0] >= window) sum -= ev[lo++]![1];
		best = Math.max(best, sum);
	}
	return best;
}

/**
 * Recorder: turns raw pointer positions into the logged strokes under the game rules every honest client applies —
 * the brush follows the pointer ≤ maxStep per tick and ≤ maxWindowTravel per window, a new stroke waits for
 * minLiftTicks and the window's stroke cap. So an honest log never trips the server's plausibility checks.
 */
export class Recorder {
	readonly p: HoaPhamParams;
	strokes: Stroke[] = [];
	private travel: [number, number][] = [];
	private starts: number[] = [];
	private lastEnd = -(1 << 30);
	inStroke = false;
	x = 0;
	y = 0;

	constructor(p: HoaPhamParams) {
		this.p = p;
	}

	private clampField(x: number, y: number): [number, number] {
		return [Math.min(Math.max(Math.round(x), 0), this.p.width - 1), Math.min(Math.max(Math.round(y), 0), this.p.height - 1)];
	}

	/** Travel budget at tick (window cap and max step). */
	room(tick: number): number {
		let used = 0;
		for (let i = this.travel.length - 1; i >= 0 && this.travel[i]![0] > tick - this.p.inputWindowTicks; i--) used += this.travel[i]![1];
		return Math.min(this.p.maxStep, this.p.maxWindowTravel - used);
	}

	canStart(tick: number): boolean {
		if (this.strokes.length >= this.p.maxStrokes) return false;
		if (this.strokes.length > 0 && tick - this.lastEnd - 1 < this.p.minLiftTicks) return false;
		let n = 0;
		for (let i = this.starts.length - 1; i >= 0 && this.starts[i]! > tick - this.p.inputWindowTicks; i--) n++;
		return n < this.p.maxStrokesPerWindow;
	}

	/**
	 * Sample for this tick. pointer = null when not pressed, else the raw position in (fractional) grid units.
	 * Returns the logged point (or null) to feed into HoaPhamGame.step.
	 */
	sample(tick: number, pointer: { x: number; y: number } | null): [number, number] | null {
		if (!pointer) {
			if (this.inStroke) {
				this.inStroke = false;
				this.lastEnd = tick - 1;
			}
			return null;
		}
		const [tx, ty] = this.clampField(pointer.x, pointer.y);
		if (!this.inStroke) {
			if (!this.canStart(tick)) return null; // wait (pressed but the stroke has not begun yet)
			this.strokes.push([tick, tx, ty]);
			this.starts.push(tick);
			this.inStroke = true;
			this.x = tx;
			this.y = ty;
			return [tx, ty];
		}
		const [nx, ny] = toward(this.x, this.y, tx, ty, this.room(tick));
		const dx = nx - this.x;
		const dy = ny - this.y;
		this.strokes[this.strokes.length - 1]!.push(dx, dy);
		this.travel.push([tick, stepLen(dx, dy)]);
		this.x = nx;
		this.y = ny;
		return [nx, ny];
	}
}

export interface ReplayResult {
	over: boolean;
	won: boolean;
	reason: EndReason;
	endTick: number;
	cuts: number;
	score: number;
	spawned: number;
	lanternsCut: number;
	escaped: number;
	bestCombo: number;
	stateHash: number;
}

/** Same contract as Go's hoapham.Replay (used by tests; the server is the only judge). Throws on invalid/inhuman logs. */
export function replay(p: HoaPhamParams, seed: number, strokes: readonly Stroke[], endTick: number): ReplayResult {
	if (endTick <= 0 || endTick > p.timeLimitTicks) throw new Error('invalid stroke log: endTick');
	if (strokes.length > p.maxStrokes) throw new Error('invalid stroke log: too many strokes');
	const travel: [number, number][] = [];
	const starts: [number, number][] = [];
	let prevEnd = -(2 ** 40);
	let maxStep = 0;
	let minLift = 0;
	for (const s of strokes) {
		if (s.length < 3 || s.length % 2 === 0) throw new Error('invalid stroke log: shape');
		// Range-check the start first (Go: a huge start would overflow start + points).
		if (s[0]! < 0 || s[0]! >= endTick || s[0]! <= prevEnd || strokeEnd(s) >= endTick) throw new Error('invalid stroke log: order');
		if (prevEnd >= 0) {
			const gap = s[0]! - prevEnd - 1;
			if (gap < 1) throw new Error('invalid stroke log: touching');
			if (minLift === 0 || gap < minLift) minLift = gap;
		}
		let x = s[1]!;
		let y = s[2]!;
		if (x < 0 || x >= p.width || y < 0 || y >= p.height) throw new Error('invalid stroke log: outside');
		starts.push([s[0]!, 1]);
		for (let j = 3; j < s.length; j += 2) {
			const dx = s[j]!;
			const dy = s[j + 1]!;
			if (Math.abs(dx) > 65536 || Math.abs(dy) > 65536) throw new Error('invalid stroke log: delta');
			x += dx;
			y += dy;
			if (x < 0 || x >= p.width || y < 0 || y >= p.height) throw new Error('invalid stroke log: outside');
			const l = stepLen(dx, dy);
			maxStep = Math.max(maxStep, l);
			travel.push([s[0]! + (j - 1) / 2, l]);
		}
		prevEnd = strokeEnd(s);
	}
	if (maxStep > p.maxStep || maxWindowSum(travel, p.inputWindowTicks) > p.maxWindowTravel ||
		maxWindowSum(starts, p.inputWindowTicks) > p.maxStrokesPerWindow || (minLift > 0 && minLift < p.minLiftTicks)) {
		throw new Error('implausible pointer input');
	}
	const g = new HoaPhamGame(p, seed);
	let si = 0;
	let j = 0;
	let x = 0;
	let y = 0;
	while (!g.over && g.tick < endTick) {
		let down = false;
		const s = strokes[si];
		if (s) {
			if (g.tick === s[0]) {
				x = s[1]!;
				y = s[2]!;
				j = 3;
				down = true;
			} else if (g.tick > s[0]! && j < s.length) {
				x += s[j]!;
				y += s[j + 1]!;
				j += 2;
				down = true;
			}
			if (down && j >= s.length) {
				si++;
				j = 0;
			}
		}
		g.step(down, x, y);
	}
	let reason: EndReason = g.reason || 'gave_up';
	if (!g.over) reason = 'gave_up';
	else if (g.tick < endTick) reason = 'ended_early';
	return {
		over: g.over,
		won: g.won,
		reason,
		endTick: g.tick,
		cuts: g.cuts,
		score: g.score,
		spawned: g.spawned,
		lanternsCut: g.lanternsCut,
		escaped: g.escaped,
		bestCombo: g.bestCombo,
		stateHash: g.hash()
	};
}
