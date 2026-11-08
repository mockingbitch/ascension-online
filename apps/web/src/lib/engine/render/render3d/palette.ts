// Design tokens → sRGB triples for 3D materials. Colours come only from `app.css` tokens (read at mount
// through the cascade, like the canvas particles); mixes are done here so no component holds a raw colour. Pure parts
// (parse / mix) are unit-tested; `readPalette` takes the resolver so tests can feed computed colours.

/** sRGB 0..1 */
export type Rgb = readonly [number, number, number];

/** Tokens the 3D hall uses (`--color-<name>`). */
export const PALETTE_TOKENS = [
	'ink-950',
	'ink-900',
	'ink-850',
	'ink-800',
	'ink-700',
	'mist-100',
	'mist-300',
	'gold-300',
	'gold-500',
	'gold-700',
	'jade-300',
	'jade-700',
	'cinnabar-500',
	'spirit-violet',
	'spirit-cyan'
] as const;

export type PaletteToken = (typeof PALETTE_TOKENS)[number];
export type Palette = Record<PaletteToken, Rgb>;

const clamp01 = (n: number) => Math.min(1, Math.max(0, n));

/** Parse a computed CSS colour: `rgb(r, g, b)`, `rgba(…)`, `rgb(r g b / a)`, `color(srgb r g b)` or `#rrggbb`. */
export function parseCssColor(css: string): Rgb | null {
	const s = css.trim().toLowerCase();
	let m = /^#([0-9a-f]{6})$/.exec(s);
	if (m) {
		const n = parseInt(m[1]!, 16);
		return [((n >> 16) & 255) / 255, ((n >> 8) & 255) / 255, (n & 255) / 255];
	}
	m = /^rgba?\(\s*([\d.]+)[\s,]+([\d.]+)[\s,]+([\d.]+)/.exec(s);
	if (m) return [clamp01(+m[1]! / 255), clamp01(+m[2]! / 255), clamp01(+m[3]! / 255)];
	m = /^color\(srgb\s+([\d.]+)\s+([\d.]+)\s+([\d.]+)/.exec(s);
	if (m) return [clamp01(+m[1]!), clamp01(+m[2]!), clamp01(+m[3]!)];
	return null;
}

/** `a` mixed with `b` (`t` = share of b), in sRGB. */
export function mix(a: Rgb, b: Rgb, t: number): Rgb {
	return [a[0] + (b[0] - a[0]) * t, a[1] + (b[1] - a[1]) * t, a[2] + (b[2] - a[2]) * t];
}

/** Every hall token through `resolve` (e.g. fx/colors tokenResolver). A token that fails to parse is a bug in the
 * tokens, not something to paper over: it throws (mount turns it into a 2D fallback). */
export function readPalette(resolve: (css: string) => string): Palette {
	const out = {} as Record<PaletteToken, Rgb>;
	for (const t of PALETTE_TOKENS) {
		const rgb = parseCssColor(resolve(`var(--color-${t})`));
		if (!rgb) throw new Error(`token --color-${t} did not resolve to a colour`);
		out[t] = rgb;
	}
	return out;
}
