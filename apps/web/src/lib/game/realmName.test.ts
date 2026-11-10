// Names by realm tier (decisions 2026-11-10): tiers 1–6 escalate, every colour stop is readable on ink (≥ 4.5:1),
// and a purchased chat aura never adds motion/glow/fill (the realm always outranks the purchase).
import fs from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';
import { auraAccent, clampTier, realmNameStyle, styleWeight } from './realmName';

const css = fs.readFileSync(path.resolve(__dirname, '../../app.css'), 'utf8');
const hexOf = (token: string) => css.match(new RegExp(`${token}:\\s*(#[0-9a-f]{6})`, 'i'))?.[1];
const lum = (hex: string) => {
	const [r, g, b] = [1, 3, 5].map((i) => {
		const c = parseInt(hex.slice(i, i + 2), 16) / 255;
		return c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
	}) as [number, number, number];
	return 0.2126 * r + 0.7152 * g + 0.0722 * b;
};
/** OKLab [L, a, b] of #rrggbb */
const oklab = (hex: string): [number, number, number] => {
	const [r, g, b] = [1, 3, 5].map((i) => {
		const c = parseInt(hex.slice(i, i + 2), 16) / 255;
		return c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
	}) as [number, number, number];
	const l = Math.cbrt(0.4122214708 * r + 0.5363325363 * g + 0.0514459929 * b);
	const m = Math.cbrt(0.2119034982 * r + 0.6806995451 * g + 0.1073969566 * b);
	const s = Math.cbrt(0.0883024619 * r + 0.2817188376 * g + 0.6299787005 * b);
	return [0.2104542553 * l + 0.793617785 * m - 0.0040720468 * s, 1.9779984951 * l - 2.428592205 * m + 0.4505937099 * s, 0.0259040371 * l + 0.7827717662 * m - 0.808675766 * s];
};
const dist = (a: number[], b: number[]) => Math.hypot(a[0]! - b[0]!, a[1]! - b[1]!, a[2]! - b[2]!);
const contrast = (a: string, b: string) => {
	const [x, y] = [lum(a), lum(b)].sort((p, q) => q - p) as [number, number];
	return (x + 0.05) / (y + 0.05);
};

describe('realm names', () => {
	it('clamps tiers to 1–6', () => {
		expect([clampTier(undefined), clampTier(0), clampTier(3), clampTier(9), clampTier(NaN)]).toEqual([1, 1, 3, 6, 1]);
	});

	it.each(['tien', 'vu'] as const)('%s: each tier is visibly stronger than the one below', (p) => {
		const styles = [1, 2, 3, 4, 5, 6].map((t) => realmNameStyle(t, p));
		expect(styles.map((s) => s.effect)).toEqual(['plain', 'glow', 'glow', 'shimmer', 'flow', 'prismatic']);
		for (let i = 1; i < styles.length; i++) {
			expect(styleWeight(styles[i]!), `tier ${i + 1}`).toBeGreaterThan(styleWeight(styles[i - 1]!));
			expect(styles[i]!.glow).toBeGreaterThanOrEqual(styles[i - 1]!.glow);
		}
		expect(styles.filter((s) => s.animated).map((s) => s.tier)).toEqual([4, 5, 6]);
		expect(styles.filter((s) => s.glints).map((s) => s.tier)).toEqual([6]);
	});

	it.each(['tien', 'vu'] as const)('%s: every colour stop exists and reads ≥ 4.5:1 on ink-900 and ink-950', (p) => {
		const inks = [hexOf('--color-ink-900')!, hexOf('--color-ink-950')!];
		for (let t = 1; t <= 6; t++)
			for (const tok of realmNameStyle(t, p).stops) {
				const hex = hexOf(tok);
				expect(hex, tok).toBeTruthy();
				for (const ink of inks) expect(contrast(hex!, ink), `${tok} on ${ink}`).toBeGreaterThanOrEqual(4.5);
			}
	});

	it.each(['tien', 'vu'] as const)('%s: adjacent tiers are clearly distinct in colour (OKLab ΔE ≥ 0.075)', (p) => {
		const base = [1, 2, 3, 4, 5, 6].map((t) => oklab(hexOf(realmNameStyle(t, p).stops[0]!)!));
		for (let i = 1; i < base.length; i++) expect(dist(base[i]!, base[i - 1]!), `tier ${i} → ${i + 1}`).toBeGreaterThanOrEqual(0.075);
		// and the top tier is the brightest of the path (most striking)
		const l = base.map((c) => c[0]);
		expect(Math.max(...l)).toBe(l[5]);
	});

	it('the Vu path uses its own tokens', () => {
		expect(realmNameStyle(5, 'vu').stops.every((s) => s.startsWith('--color-rname-vu-5'))).toBe(true);
	});

	it('fairness: a chat aura is a static accent only, weaker than the faintest realm effect', () => {
		for (const fx of ['fx-mac-huong', 'fx-bich-lam', 'fx-kim-sa', 'fx-tinh-ha', 'fx-linh-quang', 'fx-tu-quang', 'fx-unknown']) {
			const a = auraAccent(fx)!;
			expect(a.animated).toBe(false);
			expect(a.glow).toBe(0);
			for (const s of a.stops) expect(hexOf(s) ?? css.includes(s), s).toBeTruthy();
		}
		expect(auraAccent(undefined)).toBeNull();
		// the name itself is styled by the realm alone: same style with or without an aura
		expect(realmNameStyle(1)).toEqual(realmNameStyle(1));
	});
});
