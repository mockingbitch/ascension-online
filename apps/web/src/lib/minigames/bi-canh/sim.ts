// Bí Cảnh — deterministic simulator, the TypeScript twin of services/game/internal/secretrealm/bicanh/bicanh.go.
// It MUST stay bit-identical with Go: integer math only (every product stays below 2^53 by the loader's bounds;
// divisions truncate like Go), xorshift32, the same step order and the same FNV-1a state hash. The shared golden
// vectors in packages/contracts/golden/bi-canh-v1.json are checked by both sim.test.ts and the Go tests. The client
// only renders this and records the movement log; the verdict and the loot always come from the server's replay.
import type { BiCanhParams } from '$lib/game/types';

/** Position sub-units per arena unit (rules constant). */
export const SUB = 16;
export const MAX_DIR = 8;

export type EndReason = 'survived' | 'fallen' | 'gave_up' | 'ended_early';

/** round(1024·cos(2πi/64)); sin(i) = COS64[(i + 48) % 64]. Shared verbatim with Go. */
const COS64 = [1024, 1019, 1004, 980, 946, 903, 851, 792, 724, 650, 569, 483, 392, 297, 200, 100, 0, -100, -200, -297, -392, -483, -569, -650,
	-724, -792, -851, -903, -946, -980, -1004, -1019, -1024, -1019, -1004, -980, -946, -903, -851, -792, -724, -650, -569, -483, -392, -297, -200,
	-100, 0, 100, 200, 297, 392, 483, 569, 650, 724, 792, 851, 903, 946, 980, 1004, 1019];

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

/** Octile length max(|dx|,|dy|) + min(|dx|,|dy|)/2. */
export function stepLen(dx: number, dy: number): number {
	let a = Math.abs(dx);
	let b = Math.abs(dy);
	if (a < b) [a, b] = [b, a];
	return a + quo(b, 2);
}

/** Step (≤ budget octile) from p toward t. Same as Go's toward. */
function toward(px: number, py: number, tx: number, ty: number, budget: number): [number, number] {
	const dx = tx - px;
	const dy = ty - py;
	const l = stepLen(dx, dy);
	if (l <= budget) return [dx, dy];
	return [quo(dx * budget, l), quo(dy * budget, l)];
}

/** Per-tick move of direction d (0..8) at straight speed s (sub-units). */
export function dirVec(d: number, s: number): [number, number] {
	const g = quo(s * 181, 256);
	switch (d) {
		case 1:
			return [0, -s];
		case 2:
			return [g, -g];
		case 3:
			return [s, 0];
		case 4:
			return [g, g];
		case 5:
			return [0, s];
		case 6:
			return [-g, g];
		case 7:
			return [-s, 0];
		case 8:
			return [-g, -g];
	}
	return [0, 0];
}

function within(ax: number, ay: number, bx: number, by: number, r: number): boolean {
	const dx = ax - bx;
	const dy = ay - by;
	return dx * dx + dy * dy <= r * r;
}

export interface Foe {
	id: number;
	type: number;
	x: number;
	y: number;
	hp: number;
	orbitHit: number;
}
export interface Shot {
	x: number;
	y: number;
	vx: number;
	vy: number;
	life: number;
}
export interface Mote {
	x: number;
	y: number;
	value: number;
	life: number;
}

/** Render hints only (not part of the state hash): what happened during the last step. */
export interface StepEvents {
	killed: { x: number; y: number; type: number }[];
	hurt: boolean;
	picked: number;
	levelUp: boolean;
	fired: boolean;
}

export class BiCanhGame {
	readonly p: BiCanhParams;
	rng: XorShift32;
	tick = 0;
	px: number;
	py: number;
	hp: number;
	invuln = 0;
	level = 0;
	qi = 0;
	score = 0;
	kills = 0;
	spawned = 0;
	nextId = 1;
	spawnCounter: number;
	fireCounter = 0;
	hits = 0;
	foes: Foe[] = [];
	shots: Shot[] = [];
	motes: Mote[] = [];
	over = false;
	won = false;
	reason: EndReason | '' = '';
	events: StepEvents = { killed: [], hurt: false, picked: 0, levelUp: false, fired: false };

	constructor(p: BiCanhParams, seed: number) {
		this.p = p;
		this.rng = new XorShift32(seed);
		this.px = quo(p.arena.width * SUB, 2);
		this.py = quo(p.arena.height * SUB, 2);
		this.hp = p.player.hp;
		this.spawnCounter = Math.max(0, p.spawn.startTicks - 20);
	}

	private end(reason: EndReason, won: boolean) {
		this.over = true;
		this.won = won;
		this.reason = reason;
	}

	private spawnInterval(): number {
		const s = this.p.spawn;
		return Math.max(s.minTicks, s.startTicks - Math.floor(this.tick / s.stepEveryTicks));
	}

	private place(t: number) {
		const def = this.p.enemies[t]!;
		const w = this.p.arena.width;
		const h = this.p.arena.height;
		const side = this.rng.intN(4);
		let xu: number;
		let yu: number;
		switch (side) {
			case 0:
				xu = def.radius + this.rng.intN(w - 2 * def.radius + 1);
				yu = def.radius;
				break;
			case 1:
				xu = w - def.radius;
				yu = def.radius + this.rng.intN(h - 2 * def.radius + 1);
				break;
			case 2:
				xu = def.radius + this.rng.intN(w - 2 * def.radius + 1);
				yu = h - def.radius;
				break;
			default:
				xu = def.radius;
				yu = def.radius + this.rng.intN(h - 2 * def.radius + 1);
		}
		let x = xu * SUB;
		let y = yu * SUB;
		if (within(x, y, this.px, this.py, this.p.spawn.safeRadius * SUB)) {
			x = w * SUB - x;
			y = h * SUB - y;
		}
		this.foes.push({ id: this.nextId, type: t, x, y, hp: def.hp, orbitHit: -1 });
		this.nextId++;
		this.spawned++;
	}

	private spawnWave() {
		const s = this.p.spawn;
		const n = 1 + this.rng.intN(Math.min(s.waveMax, 1 + Math.floor(this.tick / s.waveGrowEveryTicks)));
		let sum = 0;
		for (const e of this.p.enemies) if (e.fromTick <= this.tick) sum += e.weight;
		for (let k = 0; k < n; k++) {
			if (this.foes.length >= s.maxEnemies) return;
			let pick = this.rng.intN(sum);
			let t = 0;
			for (let i = 0; i < this.p.enemies.length; i++) {
				const e = this.p.enemies[i]!;
				if (e.fromTick > this.tick || e.weight === 0) continue;
				if (pick < e.weight) {
					t = i;
					break;
				}
				pick -= e.weight;
			}
			this.place(t);
		}
	}

	private damage(f: Foe, d: number) {
		if (f.hp <= 0) return;
		f.hp -= d;
		if (f.hp <= 0) {
			const def = this.p.enemies[f.type]!;
			this.kills++;
			this.score += def.points;
			if (def.points > 0 && this.motes.length < this.p.orbs.max) this.motes.push({ x: f.x, y: f.y, value: def.points, life: this.p.orbs.lifeTicks });
			this.events.killed.push({ x: f.x, y: f.y, type: f.type });
		}
	}

	/** Advance one tick with direction dir (0..8). Same order as Go. */
	step(dir: number) {
		this.events = { killed: [], hurt: false, picked: 0, levelUp: false, fired: false };
		if (this.over) return;
		const p = this.p;
		// 1. move
		const [vx, vy] = dirVec(dir, p.player.speedSub);
		const r = p.player.radius * SUB;
		this.px = Math.min(Math.max(this.px + vx, r), p.arena.width * SUB - r);
		this.py = Math.min(Math.max(this.py + vy, r), p.arena.height * SUB - r);
		// 2. phi kiếm
		const lv = p.levels[this.level]!;
		if (this.fireCounter < lv.fireEvery) this.fireCounter++;
		if (this.fireCounter >= lv.fireEvery && this.shots.length < p.maxSwords) {
			let best = -1;
			let bestD = p.sword.range * SUB * (p.sword.range * SUB);
			for (let i = 0; i < this.foes.length; i++) {
				const f = this.foes[i]!;
				const dx = f.x - this.px;
				const dy = f.y - this.py;
				const d = dx * dx + dy * dy;
				if (d <= bestD && (best < 0 || d < bestD)) {
					best = i;
					bestD = d;
				}
			}
			if (best >= 0) {
				const f = this.foes[best]!;
				let [sx, sy] = toward(this.px, this.py, f.x, f.y, p.sword.speedSub);
				if (sx === 0 && sy === 0) sy = -p.sword.speedSub;
				const l = stepLen(sx, sy);
				if (l < p.sword.speedSub) {
					sx = quo(sx * p.sword.speedSub, l);
					sy = quo(sy * p.sword.speedSub, l);
				}
				this.shots.push({ x: this.px, y: this.py, vx: sx, vy: sy, life: p.sword.lifeTicks });
				this.fireCounter = 0;
				this.events.fired = true;
			}
		}
		// 3. swords fly
		const keepS: Shot[] = [];
		for (const s of this.shots) {
			s.x += s.vx;
			s.y += s.vy;
			s.life--;
			let hit = false;
			for (const f of this.foes) {
				if (f.hp > 0 && within(s.x, s.y, f.x, f.y, (p.enemies[f.type]!.radius + p.sword.radius) * SUB)) {
					this.damage(f, p.sword.damage);
					hit = true;
					break;
				}
			}
			const out = s.x < 0 || s.y < 0 || s.x > p.arena.width * SUB || s.y > p.arena.height * SUB;
			if (!hit && !out && s.life > 0) keepS.push(s);
		}
		this.shots = keepS;
		// 4. kiếm trận
		const n = lv.orbit;
		if (n > 0) {
			const base = (this.tick * p.orbit.step) % 64;
			for (let b = 0; b < n; b++) {
				const idx = (base + quo(b * 64, n)) % 64;
				const bx = this.px + quo(p.orbit.radius * SUB * COS64[idx]!, 1024);
				const by = this.py + quo(p.orbit.radius * SUB * COS64[(idx + 48) % 64]!, 1024);
				for (const f of this.foes) {
					if (f.hp <= 0 || (f.orbitHit >= 0 && this.tick - f.orbitHit < p.orbit.hitCooldownTicks)) continue;
					if (within(bx, by, f.x, f.y, (p.enemies[f.type]!.radius + p.orbit.bladeRadius) * SUB)) {
						f.orbitHit = this.tick;
						this.damage(f, p.orbit.damage);
					}
				}
			}
		}
		this.foes = this.foes.filter((f) => f.hp > 0);
		// 5. spirits close in; a touch outside invulnerability costs one hộ thể
		const keepF: Foe[] = [];
		for (const f of this.foes) {
			const def = p.enemies[f.type]!;
			const [dx, dy] = toward(f.x, f.y, this.px, this.py, def.speedSub);
			f.x += dx;
			f.y += dy;
			if (this.invuln === 0 && !this.over && within(f.x, f.y, this.px, this.py, (def.radius + p.player.radius) * SUB)) {
				this.hp--;
				this.hits++;
				this.invuln = p.player.invulnTicks;
				this.events.hurt = true;
				if (this.hp <= 0) this.end('fallen', false);
				continue;
			}
			keepF.push(f);
		}
		this.foes = keepF;
		// 6. motes
		const keepM: Mote[] = [];
		for (const m of this.motes) {
			if (within(m.x, m.y, this.px, this.py, p.orbs.pickupRadius * SUB)) {
				this.qi += m.value;
				this.score += m.value;
				this.events.picked += m.value;
				continue;
			}
			if (within(m.x, m.y, this.px, this.py, p.orbs.magnetRadius * SUB)) {
				const [dx, dy] = toward(m.x, m.y, this.px, this.py, p.orbs.magnetSpeedSub);
				m.x += dx;
				m.y += dy;
			}
			m.life--;
			if (m.life > 0) keepM.push(m);
		}
		this.motes = keepM;
		// 7. kiếm ý
		while (this.level + 1 < p.levels.length && this.qi >= p.levels[this.level + 1]!.qi) {
			this.level++;
			this.events.levelUp = true;
		}
		// 8. spawns
		if (!this.over) {
			for (let i = 0; i < p.enemies.length; i++) {
				for (const at of p.enemies[i]!.atTicks ?? []) if (at === this.tick && this.foes.length < p.spawn.maxEnemies) this.place(i);
			}
			this.spawnCounter++;
			if (this.spawnCounter >= this.spawnInterval()) {
				this.spawnCounter = 0;
				this.spawnWave();
			}
		}
		// 9. timers
		if (this.invuln > 0) this.invuln--;
		this.tick++;
		if (!this.over && this.tick >= p.timeLimitTicks) this.end('survived', true);
	}

	/** Blade positions (sub-units) for rendering, same formula as the sim (uses the tick about to be simulated). */
	blades(): [number, number][] {
		const n = this.p.levels[this.level]!.orbit;
		const out: [number, number][] = [];
		const base = (this.tick * this.p.orbit.step) % 64;
		for (let b = 0; b < n; b++) {
			const idx = (base + quo(b * 64, n)) % 64;
			out.push([this.px + quo(this.p.orbit.radius * SUB * COS64[idx]!, 1024), this.py + quo(this.p.orbit.radius * SUB * COS64[(idx + 48) % 64]!, 1024)]);
		}
		return out;
	}

	/** FNV-1a (32-bit) over the canonical state, each value as 4 little-endian bytes (same order as Go). */
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
		for (const v of [this.tick, +this.over, +this.won, this.px, this.py, this.hp, this.invuln, this.level, this.qi, this.score, this.kills,
			this.spawned, this.nextId, this.spawnCounter, this.fireCounter, this.hits, this.rng.s, this.foes.length, this.shots.length,
			this.motes.length]) put(v);
		for (const f of this.foes) for (const v of [f.id, f.type, f.x, f.y, f.hp, f.orbitHit]) put(v);
		for (const s of this.shots) for (const v of [s.x, s.y, s.vx, s.vy, s.life]) put(v);
		for (const m of this.motes) for (const v of [m.x, m.y, m.value, m.life]) put(v);
		return h;
	}
}

/** One RLE entry on the wire: [dir, ticks]. */
export type Run = [number, number];

/**
 * Recorder: turns the wanted direction of each tick into the canonical RLE log under the cadence rule every honest
 * client applies — a change that would exceed maxChangesPerWindow inside inputWindowTicks is deferred (the previous
 * direction is kept). So an honest log never trips the server's plausibility check. Same rule as Go's bicanhtest.
 */
export class Recorder {
	readonly p: BiCanhParams;
	runs: Run[] = [];
	private changes: number[] = [];
	private tick = 0;

	constructor(p: BiCanhParams) {
		this.p = p;
	}

	sample(want: number): number {
		const t = this.tick++;
		if (this.runs.length === 0) {
			this.runs.push([want, 1]);
			return want;
		}
		const last = this.runs[this.runs.length - 1]!;
		if (want !== last[0] && this.runs.length < this.p.maxRuns) {
			let n = 0;
			for (let i = this.changes.length - 1; i >= 0 && this.changes[i]! > t - this.p.inputWindowTicks; i--) n++;
			if (n < this.p.maxChangesPerWindow) {
				this.changes.push(t);
				this.runs.push([want, 1]);
				return want;
			}
		}
		last[1]++;
		return last[0];
	}
}

export interface ReplayResult {
	over: boolean;
	won: boolean;
	reason: EndReason;
	endTick: number;
	kills: number;
	score: number;
	qi: number;
	level: number;
	hpLeft: number;
	spawned: number;
	stateHash: number;
}

/** Same contract as Go's bicanh.Replay (tests only; the server is the only judge). Throws on invalid/inhuman logs. */
export function replay(p: BiCanhParams, seed: number, runs: readonly Run[], endTick: number): ReplayResult {
	if (endTick <= 0 || endTick > p.timeLimitTicks) throw new Error('invalid move log: endTick');
	if (runs.length === 0 || runs.length > p.maxRuns) throw new Error('invalid move log: runs');
	let total = 0;
	const changes: number[] = [];
	runs.forEach(([d, n], i) => {
		if (!Number.isInteger(d) || !Number.isInteger(n) || d < 0 || d > MAX_DIR || n < 1 || n > p.timeLimitTicks) throw new Error('invalid move log: range');
		if (i > 0) {
			if (d === runs[i - 1]![0]) throw new Error('invalid move log: repeat');
			changes.push(total);
		}
		total += n;
		if (total > endTick) throw new Error('invalid move log: long');
	});
	if (total !== endTick) throw new Error('invalid move log: length');
	let worst = 0;
	let lo = 0;
	for (let hi = 0; hi < changes.length; hi++) {
		while (changes[hi]! - changes[lo]! >= p.inputWindowTicks) lo++;
		worst = Math.max(worst, hi - lo + 1);
	}
	if (worst > p.maxChangesPerWindow) throw new Error('implausible movement input');
	const g = new BiCanhGame(p, seed);
	for (const [d, n] of runs) {
		for (let k = 0; k < n && !g.over; k++) g.step(d);
		if (g.over) break;
	}
	let reason: EndReason = g.reason || 'gave_up';
	if (!g.over) reason = 'gave_up';
	else if (g.tick < endTick) reason = 'ended_early';
	return { over: g.over, won: g.won, reason, endTick: g.tick, kills: g.kills, score: g.score, qi: g.qi, level: g.level, hpLeft: g.hp,
		spawned: g.spawned, stateHash: g.hash() };
}
