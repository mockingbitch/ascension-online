// Effect presets and pure mappings (spec §4). Colours are design-token references, resolved once per colour by
// ParticleSystem.attach(…, resolve). Counts scale with log10 of SERVER values and are capped — presentation only,
// nothing here feeds back into game numbers.
import type { ParticlePreset, Quality } from './particles';

/** Small upward burst of linh khí motes. */
export const qiBurst: ParticlePreset = {
	id: 'qi_burst',
	count: 24,
	liteScale: 0.35,
	lifeMs: [600, 900],
	speed: [40, 140],
	angle: -Math.PI / 2,
	spread: Math.PI * 0.9,
	size: [1.5, 3.5],
	gravity: -20,
	drag: 0.4,
	colors: ['var(--color-jade-300)', 'var(--color-spirit-cyan)']
};

/** Qi orb flying along a path (Thu nhận → HUD, ngộ đạo → HUD). Speed fields unused for path particles. */
export const qiOrb: ParticlePreset = {
	id: 'qi_orb',
	count: 16,
	lifeMs: [700, 700],
	speed: [0, 0],
	angle: 0,
	spread: 0,
	size: [3.5, 6],
	gravity: 0,
	drag: 1,
	alpha: 'bell',
	colors: ['var(--color-jade-300)', 'var(--color-spirit-cyan)', 'var(--color-gold-300)']
};

/** Sparks where the orbs land / a reward lands. */
export const landSpark: ParticlePreset = {
	id: 'land_spark',
	count: 14,
	liteScale: 0.4,
	lifeMs: [300, 550],
	speed: [60, 180],
	angle: 0,
	spread: Math.PI * 2,
	size: [1, 2.4],
	gravity: 0,
	drag: 0.15,
	colors: ['var(--color-jade-300)', 'var(--color-gold-300)']
};

/** Small faction firework (works level up). */
export const firework: ParticlePreset = {
	id: 'firework',
	count: 48,
	liteScale: 0.3,
	lifeMs: [700, 1200],
	speed: [90, 260],
	angle: 0,
	spread: Math.PI * 2,
	size: [1.2, 2.8],
	gravity: 120,
	drag: 0.35,
	colors: ['var(--color-gold-300)', 'var(--color-cinnabar-300)', 'var(--color-jade-300)', 'var(--color-spirit-violet)']
};

/** Rising incense / seat-appearing smoke: big soft dots that grow and fade in-out. */
export const smoke: ParticlePreset = {
	id: 'smoke',
	count: 6,
	liteScale: 0.5,
	lifeMs: [1400, 2400],
	speed: [8, 22],
	angle: -Math.PI / 2,
	spread: 0.6,
	size: [2, 4],
	gravity: -6,
	drag: 0.7,
	grow: 5,
	alpha: 'bell',
	colors: ['var(--color-mist-500)', 'var(--color-mist-300)']
};

/** Puff when a listener's seat appears in the hall. */
export const smokePuff: ParticlePreset = { ...smoke, id: 'smoke_puff', count: 10, lifeMs: [500, 900], speed: [10, 40], spread: Math.PI * 2, gravity: -10, grow: 14 };

/** Peach petals drifting down at Đào Hoa Đình. */
export const petal: ParticlePreset = {
	id: 'petal',
	count: 1,
	lifeMs: [4000, 6500],
	speed: [10, 26],
	angle: Math.PI * 0.35,
	spread: 0.8,
	size: [2, 3.4],
	gravity: 6,
	drag: 0.9,
	alpha: 'bell',
	shape: 'petal',
	colors: ['var(--color-cinnabar-300)', 'var(--color-gold-300)']
};

/** Linh khí mote converging on the đan điền / drifting from Hồng Quân to the player (path particle). */
export const mote: ParticlePreset = {
	id: 'mote',
	count: 1,
	lifeMs: [2600, 3400],
	speed: [0, 0],
	angle: 0,
	spread: 0,
	size: [1.2, 2.4],
	gravity: 0,
	drag: 1,
	alpha: 'bell',
	colors: ['var(--color-spirit-cyan)', 'var(--color-jade-300)']
};

/**
 * Motes/orbs shown for a settle (spec §4.5): ∝ log10 of the SERVER-reported earnedMilli, 8..24 at full, fewer at lite,
 * none when off or nothing was earned. Presentation only — never feeds back into game numbers.
 */
export function motesForEarned(earnedMilli: number, quality: Quality): number {
	if (quality === 'off' || !(earnedMilli > 0)) return 0;
	const n = Math.min(24, Math.max(8, Math.round(4 * Math.log10(earnedMilli))));
	return quality === 'lite' ? Math.ceil(n / 3) : n;
}

/** Flight time of the orbs to the HUD (600–900 ms, longer for bigger amounts; lite = 600). */
export function orbFlightMs(earnedMilli: number, quality: Quality): number {
	if (quality === 'off' || !(earnedMilli > 0)) return 0;
	if (quality === 'lite') return 600;
	return Math.round(Math.min(900, Math.max(600, 450 + 60 * Math.log10(earnedMilli))));
}

// ── count-up (HUD numbers) ──

/** Count-up length for a change of `delta` (log-scaled 420–900 ms; lite 300; off 0 = jump). */
export function countUpMs(delta: number, quality: Quality): number {
	if (quality === 'off' || delta === 0 || !Number.isFinite(delta)) return 0;
	if (quality === 'lite') return 300;
	return Math.round(Math.min(900, Math.max(420, 300 + 100 * Math.log10(Math.abs(delta) + 1))));
}

/** Displayed value `elapsed` ms into a count-up from → to (ease-out cubic, integers, exact at the end). */
export function countUpAt(from: number, to: number, elapsedMs: number, durationMs: number): number {
	if (durationMs <= 0 || elapsedMs >= durationMs) return to;
	const t = Math.max(0, elapsedMs) / durationMs;
	const e = 1 - (1 - t) ** 3;
	return Math.round(from + (to - from) * e);
}

// ── rewards ──

const RARITY = ['pham', 'linh', 'huyen', 'dia', 'thien', 'tien'] as const;

/** Rarities from huyền up get the aura + rotating rays when they fly to the bag. */
export function isRare(rarity: string | undefined): boolean {
	return RARITY.indexOf(rarity as (typeof RARITY)[number]) >= 2;
}

/** How many reward icons fly (one per line, at most 4 full / 2 lite / 0 off). */
export function rewardFlights(lines: number, quality: Quality): number {
	if (quality === 'off' || lines <= 0) return 0;
	return Math.min(lines, quality === 'lite' ? 2 : 4);
}
