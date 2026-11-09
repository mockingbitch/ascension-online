// realms.yaml ↔ app.css for the effect tiers (decisions 2026-11-09 D, #10): every realm aura has a token, every tier
// has an fx_tier, and the six Vu aura tokens get brighter with each tier (a higher realm never looks dimmer).
import fs from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';

const repo = path.resolve(__dirname, '../../../../../..');
const yaml = fs.readFileSync(path.join(repo, 'content/catalog/realms.yaml'), 'utf8');
const css = fs.readFileSync(path.resolve(__dirname, '../../../app.css'), 'utf8');

const lin = (c: number) => (c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4);
/** OKLab lightness of a #rrggbb token */
function lightness(hex: string): number {
	const [r, g, b] = [1, 3, 5].map((i) => lin(parseInt(hex.slice(i, i + 2), 16) / 255)) as [number, number, number];
	const l = Math.cbrt(0.4122214708 * r + 0.5363325363 * g + 0.0514459929 * b);
	const m = Math.cbrt(0.2119034982 * r + 0.6806995451 * g + 0.1073969566 * b);
	const s = Math.cbrt(0.0883024619 * r + 0.2817188376 * g + 0.6299787005 * b);
	return 0.2104542553 * l + 0.793617785 * m - 0.0040720468 * s;
}
const tokenHex = (aura: string) => css.match(new RegExp(`--color-aura-${aura}:\\s*(#[0-9a-f]{6})`, 'i'))?.[1];

describe('effect tier tokens', () => {
	it('every realm aura in realms.yaml has a --color-aura-* token', () => {
		const auras = new Set([...yaml.matchAll(/aura:\s*([a-z-]+)/g)].map((m) => m[1]!));
		expect(auras.size).toBeGreaterThan(10);
		for (const a of auras) expect(tokenHex(a), a).toBeTruthy();
	});
	it('every playable tier declares visual.fx_tier 1–6, non-decreasing', () => {
		const fx = [...yaml.matchAll(/fx_tier:\s*(\d)/g)].map((m) => Number(m[1]));
		expect(fx).toHaveLength(16);
		for (let i = 1; i < fx.length; i++) expect(fx[i]!).toBeGreaterThanOrEqual(fx[i - 1]!);
		expect(new Set(fx)).toEqual(new Set([1, 2, 3, 4, 5, 6]));
	});
	it('the six Vu aura tokens brighten with each tier (decision #10)', () => {
		const vu = ['vu-ember', 'vu-blood', 'vu-crimson', 'vu-storm', 'vu-totem', 'vu-ancestral'].map((a) => lightness(tokenHex(a)!));
		for (let i = 1; i < vu.length; i++) expect(vu[i]!).toBeGreaterThan(vu[i - 1]!);
		// Vu realms use them in tier order (vu_1 … vu_15)
		const used = [...yaml.matchAll(/id: vu_(\d+),[^\n]*aura: (vu-[a-z]+)/g)].map((m) => [Number(m[1]), m[2]!] as const);
		expect(used).toHaveLength(15);
	});
});
