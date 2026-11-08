// Placeholder sounds rendered at runtime (lazy chunk): original, generated from a few oscillators and seeded noise —
// no binary assets, nothing copied. Each recipe is a list of partials (sine/triangle/square/noise) with a pitch glide,
// an attack/decay envelope and a start delay; render() mixes them into mono PCM and normalises the peak to −3 dBFS.
// Pure and deterministic (seeded noise), so tests can check every id renders, its length and its level.
// Real recordings replace these by dropping `<id>.ogg` (+ `.mp3`) into lib/assets/audio — the manifest prefers files.

export type Wave = 'sine' | 'triangle' | 'square' | 'noise';

export interface Voice {
	wave: Wave;
	/** start / end frequency (Hz; noise: low-pass cutoff) — exponential glide */
	f: number;
	f1?: number;
	/** seconds */
	delay?: number;
	attack?: number;
	/** exponential decay time constant (s) and hard length (s) */
	decay: number;
	len: number;
	gain: number;
	/** tremolo rate (Hz), depth 0..1 */
	trem?: [number, number];
}

export interface Recipe {
	partials: Voice[];
}

/** peak after normalisation: −3 dBFS */
export const PEAK = 0.707;

const bell = (f: number, len: number, gain = 1, delay = 0): Voice[] => [
	{ wave: 'sine', f, decay: len * 0.35, len, gain, delay, attack: 0.004 },
	{ wave: 'sine', f: f * 2.76, decay: len * 0.12, len: len * 0.6, gain: gain * 0.35, delay, attack: 0.002 },
	{ wave: 'sine', f: f * 5.4, decay: len * 0.05, len: len * 0.3, gain: gain * 0.15, delay, attack: 0.001 }
];
const clink = (f: number, gain = 1, delay = 0): Voice[] => [
	{ wave: 'triangle', f, decay: 0.06, len: 0.28, gain, delay, attack: 0.001 },
	{ wave: 'sine', f: f * 1.5, decay: 0.05, len: 0.2, gain: gain * 0.4, delay, attack: 0.001 }
];
const shimmer = (f: number, len: number, gain: number, delay = 0): Voice[] => [
	{ wave: 'sine', f, f1: f * 1.02, decay: len * 0.4, len, gain, delay, attack: len * 0.25, trem: [11, 0.5] },
	{ wave: 'sine', f: f * 1.5, decay: len * 0.3, len, gain: gain * 0.6, delay: delay + 0.05, attack: len * 0.25, trem: [13, 0.5] }
];

/** Every synth placeholder by sound id (ids and specs: lib/assets/audio/README.md). */
export const RECIPES: Record<string, Recipe> = {
	'ui.click': { partials: [{ wave: 'triangle', f: 1400, f1: 900, decay: 0.018, len: 0.08, gain: 1, attack: 0.001 }, { wave: 'noise', f: 3000, decay: 0.008, len: 0.03, gain: 0.25 }] },
	'ui.window_open': {
		partials: [
			{ wave: 'noise', f: 2500, decay: 0.09, len: 0.26, gain: 0.5, attack: 0.08 },
			{ wave: 'sine', f: 520, f1: 780, decay: 0.08, len: 0.24, gain: 0.6, attack: 0.03 }
		]
	},
	'ui.window_close': {
		partials: [
			{ wave: 'noise', f: 1800, decay: 0.06, len: 0.2, gain: 0.45, attack: 0.02 },
			{ wave: 'sine', f: 700, f1: 460, decay: 0.06, len: 0.18, gain: 0.55, attack: 0.01 }
		]
	},
	'ui.countdown': { partials: bell(880, 0.3, 1) },
	'ui.countdown_go': { partials: [...bell(1320, 0.45, 1), ...bell(1760, 0.4, 0.5, 0.02)] },
	'sfx.chat': { partials: [{ wave: 'sine', f: 1050, f1: 980, decay: 0.025, len: 0.1, gain: 1, attack: 0.002 }] },
	'sfx.settle': {
		partials: [
			{ wave: 'noise', f: 600, f1: 4000, decay: 0.5, len: 0.8, gain: 0.55, attack: 0.55 },
			{ wave: 'sine', f: 330, f1: 660, decay: 0.4, len: 0.7, gain: 0.35, attack: 0.5 },
			...bell(1318, 0.5, 0.8, 0.62)
		]
	},
	'sfx.item_pham': { partials: clink(1760) },
	'sfx.item_linh': { partials: [...clink(1760), ...shimmer(2093, 0.35, 0.35, 0.05)] },
	'sfx.item_huyen': { partials: [...clink(1976), ...shimmer(2349, 0.45, 0.45, 0.05)] },
	'sfx.item_dia': { partials: [...clink(1760), ...shimmer(2093, 0.55, 0.45, 0.05), { wave: 'sine', f: 110, decay: 0.3, len: 0.6, gain: 0.5, attack: 0.05 }] },
	'sfx.item_thien': { partials: [...bell(1046, 0.8, 0.9), ...shimmer(2093, 0.75, 0.45, 0.1)] },
	'sfx.item_tien': {
		partials: [
			{ wave: 'triangle', f: 523, decay: 0.6, len: 1.0, gain: 0.35, attack: 0.25, trem: [5, 0.2] },
			{ wave: 'triangle', f: 659, decay: 0.6, len: 1.0, gain: 0.3, attack: 0.3, trem: [5.5, 0.2] },
			{ wave: 'triangle', f: 784, decay: 0.6, len: 1.0, gain: 0.3, attack: 0.35, trem: [6, 0.2] },
			...bell(1568, 0.8, 0.7, 0.2)
		]
	},
	'sfx.stage_up': {
		partials: [
			{ wave: 'triangle', f: 392, decay: 0.25, len: 0.5, gain: 0.8, attack: 0.002 },
			{ wave: 'triangle', f: 523, decay: 0.25, len: 0.5, gain: 0.8, attack: 0.002, delay: 0.09 },
			{ wave: 'triangle', f: 659, decay: 0.3, len: 0.6, gain: 0.8, attack: 0.002, delay: 0.18 },
			...shimmer(1318, 0.5, 0.3, 0.25)
		]
	},
	'sfx.breakthrough': {
		partials: [
			{ wave: 'sine', f: 98, decay: 0.9, len: 2.0, gain: 1, attack: 0.02 },
			{ wave: 'sine', f: 196, f1: 190, decay: 0.7, len: 1.8, gain: 0.5, attack: 0.02, trem: [3, 0.3] },
			{ wave: 'sine', f: 271, decay: 0.5, len: 1.5, gain: 0.3, attack: 0.02 },
			{ wave: 'noise', f: 400, f1: 3000, decay: 0.6, len: 1.6, gain: 0.45, attack: 0.5, delay: 0.1 }
		]
	},
	'sfx.thunder': {
		partials: [
			{ wave: 'noise', f: 6000, f1: 1500, decay: 0.05, len: 0.25, gain: 1, attack: 0.001 },
			{ wave: 'noise', f: 300, f1: 120, decay: 0.45, len: 1.2, gain: 0.9, attack: 0.04, delay: 0.05 },
			{ wave: 'sine', f: 55, f1: 40, decay: 0.4, len: 1.1, gain: 0.5, attack: 0.03, delay: 0.05 }
		]
	},
	'sfx.fail': {
		partials: [
			{ wave: 'sine', f: 147, f1: 130, decay: 0.3, len: 0.8, gain: 0.8, attack: 0.01 },
			{ wave: 'square', f: 155, f1: 138, decay: 0.15, len: 0.5, gain: 0.12, attack: 0.01 },
			{ wave: 'noise', f: 5000, decay: 0.04, len: 0.2, gain: 0.35, delay: 0.03 }
		]
	},
	'sfx.insight': {
		partials: [
			...bell(784, 1.2, 0.9),
			{ wave: 'noise', f: 900, decay: 0.4, len: 1.0, gain: 0.25, attack: 0.35, delay: 0.15 }
		]
	},
	'sfx.announcement': {
		partials: [
			...bell(196, 2.0, 1),
			...bell(294, 1.6, 0.4, 0.01),
			{ wave: 'sine', f: 98, decay: 0.8, len: 2.0, gain: 0.35, attack: 0.02, trem: [2.5, 0.3] }
		]
	}
};

/** Seeded PRNG (mulberry32): the same noise every render. */
function rng(seed: number) {
	let a = seed >>> 0;
	return () => {
		a = (a + 0x6d2b79f5) >>> 0;
		let t = a;
		t = Math.imul(t ^ (t >>> 15), t | 1);
		t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
		return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
	};
}

/** Total length (s) of a recipe. */
export const recipeLength = (r: Recipe) => Math.max(...r.partials.map((p) => (p.delay ?? 0) + p.len));

/** Mix a recipe into mono PCM at `rate`, peak-normalised to PEAK, with 5 ms fade-out at the end. */
export function render(r: Recipe, rate: number, seed = 1): Float32Array {
	const n = Math.max(1, Math.ceil(recipeLength(r) * rate));
	const out = new Float32Array(n);
	const rand = rng(seed);
	for (const p of r.partials) {
		const start = Math.round((p.delay ?? 0) * rate);
		const len = Math.min(n - start, Math.round(p.len * rate));
		const attack = Math.max(1, Math.round((p.attack ?? 0.003) * rate));
		const f1 = p.f1 ?? p.f;
		let phase = 0;
		let lp = 0;
		for (let i = 0; i < len; i++) {
			const t = i / rate;
			const k = i / Math.max(1, len - 1);
			const f = p.f * Math.pow(f1 / p.f, k);
			const env = (i < attack ? i / attack : 1) * Math.exp(-Math.max(0, t - attack / rate) / p.decay) * (1 - k * k * k);
			const trem = p.trem ? 1 - p.trem[1] * 0.5 * (1 + Math.sin(2 * Math.PI * p.trem[0] * t)) : 1;
			let s: number;
			if (p.wave === 'noise') {
				// one-pole low-pass of white noise, cutoff gliding f → f1
				const a = 1 - Math.exp((-2 * Math.PI * f) / rate);
				lp += a * (rand() * 2 - 1 - lp);
				s = lp * 2;
			} else {
				phase += f / rate;
				phase -= Math.floor(phase);
				s =
					p.wave === 'sine'
						? Math.sin(2 * Math.PI * phase)
						: p.wave === 'triangle'
							? 1 - 4 * Math.abs(phase - 0.5)
							: phase < 0.5
								? 1
								: -1;
			}
			out[start + i]! += s * env * trem * p.gain;
		}
	}
	let peak = 0;
	for (let i = 0; i < n; i++) peak = Math.max(peak, Math.abs(out[i]!));
	const scale = peak > 0 ? PEAK / peak : 0;
	const fade = Math.min(n, Math.round(0.005 * rate));
	for (let i = 0; i < n; i++) out[i]! *= scale * (i >= n - fade ? (n - 1 - i) / fade : 1);
	return out;
}

/** Render the placeholder for `id` (null when no recipe). */
export async function renderSynth(id: string, rate: number): Promise<Float32Array | null> {
	const r = RECIPES[id];
	return r ? render(r, rate) : null;
}
